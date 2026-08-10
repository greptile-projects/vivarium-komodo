package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/questions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type questionStore interface {
	Create(questions.Conversation) (questions.Conversation, error)
	Get(string, string) (questions.Conversation, error)
	List(string) ([]questions.Conversation, error)
}
type questionCheckStore interface {
	List(string, string) ([]checkruns.Run, error)
}

func registerQuestionsHTTP(mux *http.ServeMux, conversations questionStore, repositories codeIntelligenceStore, credentials authStore, relationships codeRelationshipStore, checks questionCheckStore) {
	mux.HandleFunc("POST /repositories/{repository}/questions", createQuestion(conversations, repositories, credentials, relationships, checks))
	mux.HandleFunc("GET /repositories/{repository}/questions", listQuestions(conversations, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/questions/{conversation}", getQuestion(conversations, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/questions/{conversation}/events", streamQuestion(conversations, repositories, credentials))
}

type askRequest struct {
	Question string            `json:"question"`
	Revision string            `json:"revision"`
	Context  questions.Context `json:"context"`
}

func createQuestion(conversations questionStore, repositories codeIntelligenceStore, credentials authStore, relationships codeRelationshipStore, checks questionCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var input askRequest
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input) != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid_json"})
			return
		}
		input.Question = strings.TrimSpace(input.Question)
		input.Revision = strings.TrimSpace(input.Revision)
		if input.Context.Type == "" {
			input.Context.Type = "repository"
		}
		if input.Question == "" || len(input.Question) > 2000 || !validQuestionContext(input.Context) {
			writeJSON(w, 422, map[string]string{"error": "invalid_question"})
			return
		}
		opened, err := repositories.Open(item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitID, revision, err := resolveRevision(opened, input.Revision)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		claims := groundedClaims(opened, string(item.ID), commitID, input.Question, input.Context)
		deps, hidden, _ := readableCodeDependencies(repositories, relationships, item, actor.UserID, string(commitID))
		if len(deps) > 0 {
			d := deps[0]
			claims = append(claims, questions.Claim{ID: fmt.Sprintf("claim-%d", len(claims)+1), Text: fmt.Sprintf("This revision declares a dependency on %s through interface %s.", d.ProviderRepositoryID, d.InterfaceName), Mode: "evidence", Citations: []questions.Citation{{RepositoryID: string(item.ID), CommitID: string(commitID), Kind: "dependency", Path: d.EvidencePath, Label: d.ID}}})
		}
		if hidden > 0 {
			claims = append(claims, questions.Claim{ID: fmt.Sprintf("claim-%d", len(claims)+1), Text: "Some dependency evidence was omitted because it is not permitted for this collaborator.", Mode: "uncertainty", Uncertainty: "The explanation may be incomplete because unreadable repositories were excluded."})
		}
		if input.Context.Type == "pull_request" && input.Context.ID != "" && checks != nil {
			runs, _ := checks.List(string(item.ID), input.Context.ID)
			for i := len(runs) - 1; i >= 0; i-- {
				run := runs[i]
				if run.CommitID == string(commitID) {
					claims = append(claims, questions.Claim{ID: fmt.Sprintf("claim-%d", len(claims)+1), Text: fmt.Sprintf("Check %s is %s at this revision.", run.Definition.Name, run.State), Mode: "evidence", Citations: []questions.Citation{{RepositoryID: string(item.ID), CommitID: string(commitID), Kind: "check", Label: run.ID}}})
					break
				}
			}
		}
		if len(claims) == 0 {
			claims = append(claims, questions.Claim{ID: "claim-1", Text: "I could not locate enough permitted exact-revision evidence to answer confidently.", Mode: "uncertainty", Uncertainty: "Try naming a symbol or path, or choose a context with more specific evidence."})
		}
		answer := renderGroundedAnswer(claims)
		events := []questions.Event{{Type: "status", Text: "Collecting permitted evidence at " + string(commitID)}}
		for i := range claims {
			claim := claims[i]
			events = append(events, questions.Event{Type: "claim", Claim: &claim})
		}
		events = append(events, questions.Event{Type: "answer", Text: answer}, questions.Event{Type: "done"})
		conversation, err := conversations.Create(questions.Conversation{RepositoryID: string(item.ID), Revision: revision, CommitID: string(commitID), ActorID: actor.UserID, Question: input.Question, Context: input.Context, Answer: answer, Claims: claims, Events: events})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/repositories/%s/questions/%s", item.ID, conversation.ID))
		writeJSON(w, 201, conversation)
	}
}

func validQuestionContext(c questions.Context) bool {
	switch c.Type {
	case "repository", "file", "proposal", "task", "pull_request", "incident", "workspace":
	default:
		return false
	}
	return len(c.ID) <= 200 && len(c.Path) <= 1000 && !strings.Contains(c.Path, "\x00") && (c.Path == "" || (path.Clean(c.Path) == c.Path && !strings.HasPrefix(c.Path, "/") && !strings.HasPrefix(c.Path, "../")))
}

type evidenceLine struct {
	path, text, object string
	line, score        int
}

func groundedClaims(repo *storage.Repository, repositoryID string, commitID storage.ObjectID, question string, context questions.Context) []questions.Claim {
	terms := questionTerms(question)
	lines := []evidenceLine{}
	commit, err := repo.ReadCommit(commitID)
	if err != nil {
		return nil
	}
	var walk func(storage.ObjectID, string)
	walk = func(treeID storage.ObjectID, prefix string) {
		tree, er := repo.ReadTree(treeID)
		if er != nil {
			return
		}
		for _, entry := range tree.Entries {
			p := entry.Name
			if prefix != "" {
				p = prefix + "/" + p
			}
			if entry.Type == storage.TreeObject {
				walk(entry.ObjectID, p)
				continue
			}
			if entry.Type != storage.BlobObject {
				continue
			}
			o, er := repo.ReadObject(entry.ObjectID)
			if er != nil || len(o.Content) > 1<<20 || bytes.IndexByte(o.Content, 0) >= 0 || !utf8.Valid(o.Content) {
				continue
			}
			for i, line := range strings.Split(string(o.Content), "\n") {
				score := 0
				lower := strings.ToLower(line + " " + p)
				for _, term := range terms {
					if strings.Contains(lower, term) {
						score++
					}
				}
				if context.Path != "" && p == context.Path {
					score += 3
				}
				if score > 0 && strings.TrimSpace(line) != "" {
					lines = append(lines, evidenceLine{p, strings.TrimSpace(line), string(entry.ObjectID), i + 1, score})
				}
			}
		}
	}
	walk(commit.Tree, "")
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].score == lines[j].score {
			if lines[i].path == lines[j].path {
				return lines[i].line < lines[j].line
			}
			return lines[i].path < lines[j].path
		}
		return lines[i].score > lines[j].score
	})
	claims := []questions.Claim{}
	seen := map[string]bool{}
	for _, e := range lines {
		if len(claims) >= 5 {
			break
		}
		key := e.path + ":" + fmt.Sprint(e.line)
		if seen[key] {
			continue
		}
		seen[key] = true
		text := e.text
		if len(text) > 240 {
			text = text[:240] + "…"
		}
		mode := "evidence"
		if e.score < 2 {
			mode = "inference"
			text = "A relevant location appears to be: " + text
		}
		claims = append(claims, questions.Claim{ID: fmt.Sprintf("claim-%d", len(claims)+1), Text: text, Mode: mode, Citations: []questions.Citation{{RepositoryID: repositoryID, CommitID: string(commitID), Kind: sourceKind(e.path), Path: e.path, LineStart: e.line, LineEnd: e.line, ObjectID: e.object}}})
	}
	if len(claims) < 5 {
		parts := bytes.SplitN(commit.Content, []byte("\n\n"), 2)
		if len(parts) == 2 && strings.TrimSpace(string(parts[1])) != "" {
			claims = append(claims, questions.Claim{ID: fmt.Sprintf("claim-%d", len(claims)+1), Text: "The pinned revision was introduced as: " + strings.TrimSpace(string(parts[1])), Mode: "evidence", Citations: []questions.Citation{{RepositoryID: repositoryID, CommitID: string(commitID), Kind: "history", Label: "commit message"}}})
		}
	}
	return claims
}

func questionTerms(q string) []string {
	stop := map[string]bool{"what": true, "where": true, "when": true, "which": true, "does": true, "this": true, "that": true, "with": true, "from": true, "have": true, "about": true, "would": true, "could": true, "should": true, "into": true, "your": true, "code": true}
	fields := strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' })
	out := []string{}
	for _, f := range fields {
		if len(f) >= 3 && !stop[f] {
			out = append(out, f)
		}
	}
	return out
}
func sourceKind(p string) string {
	lower := strings.ToLower(p)
	if strings.HasSuffix(lower, ".md") || strings.Contains(lower, "/docs/") || strings.HasPrefix(lower, "docs/") {
		return "documentation"
	}
	if strings.Contains(lower, "test") {
		return "test"
	}
	return "source"
}
func renderGroundedAnswer(claims []questions.Claim) string {
	parts := make([]string, 0, len(claims))
	for _, c := range claims {
		marker := ""
		if c.Mode == "inference" {
			marker = "Inference: "
		}
		if c.Mode == "uncertainty" {
			marker = "Uncertainty: "
		}
		cites := []string{}
		for _, x := range c.Citations {
			loc := x.Path
			if x.LineStart > 0 {
				loc += fmt.Sprintf(":%d", x.LineStart)
			}
			cites = append(cites, fmt.Sprintf("[%s@%s %s]", x.RepositoryID, x.CommitID, loc))
		}
		parts = append(parts, marker+c.Text+" "+strings.Join(cites, " "))
	}
	return strings.Join(parts, "\n\n")
}

func listQuestions(store questionStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(item.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func getQuestion(store questionStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, err := store.Get(string(item.ID), r.PathValue("conversation"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, c)
	}
}
func streamQuestion(store questionStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, err := store.Get(string(item.ID), r.PathValue("conversation"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		after := int64(0)
		fmt.Sscan(r.URL.Query().Get("after"), &after)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		for _, event := range c.Events {
			if event.Sequence <= after {
				continue
			}
			b, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, b)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
