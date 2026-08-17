package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/supportquestions"
)

type supportSources struct {
	releases interface {
		Get(string, string) (releases.Release, error)
	}
	packages interface {
		Get(string, string) (packagecatalog.Version, error)
	}
	docs interface {
		Get(string, string) (docscollections.Collection, error)
	}
	issues interface {
		List(string) ([]issues.Issue, error)
	}
}

func registerSupportQuestionsHTTP(mux *http.ServeMux, s *supportquestions.Store, repos proposalRepositoryStore, credentials authStore, src supportSources, runner *supportquestions.VerificationRunner) {
	base := "/repositories/{repository}/support-questions"
	access := func(w http.ResponseWriter, r *http.Request, required bool) (repositories.Repository, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, required)
		return repo, a.UserID, ok
	}
	participant := func(repo repositories.Repository, actor string) bool {
		if actor == repo.OwnerID {
			return true
		}
		ok, _ := repos.IsCollaborator(repo.ID, actor)
		return ok
	}
	visible := func(repo repositories.Repository, v supportquestions.Question, actor string) bool {
		return v.Audience == "public" && repo.Visibility == repositories.Public || actor == v.AuthorID || participant(repo, actor)
	}
	project := func(repo repositories.Repository, v supportquestions.Question, actor string) supportquestions.Question {
		maintainer := actor == v.AuthorID || participant(repo, actor)
		if !maintainer {
			v.Contact.Value = ""
		}
		for i := range v.Evidence {
			if v.Evidence[i].Visibility == "maintainers" && !maintainer {
				v.Evidence[i].Content = ""
			}
		}
		for i := range v.Solutions {
			notices := []supportquestions.SolutionNotification{}
			for _, notice := range v.Solutions[i].Notifications {
				if notice.Recipient == actor {
					notices = append(notices, notice)
				}
			}
			v.Solutions[i].Notifications = notices
		}
		return v
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if supportError(w, e) {
			return
		}
		out := []supportquestions.Question{}
		for _, v := range items {
			if visible(repo, v, a) {
				v = project(repo, v, a)
				v.Discussion = nil
				v.History = nil
				for i := range v.Evidence {
					v.Evidence[i].Content = ""
				}
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in supportquestions.Input
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		if !validSupportSubject(string(repo.ID), in.Subject, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_support_subject"})
			return
		}
		v, e := s.Create(string(repo.ID), a, in)
		if supportError(w, e) {
			return
		}
		related := supportSuggestions(repos, repo, a, v, s, src.issues)
		v, _ = s.SetRelated(string(repo.ID), v.ID, related)
		writeJSON(w, 201, project(repo, v, a))
	})
	mux.HandleFunc("GET "+base+"/suggestions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		stub := supportquestions.Question{Title: q, Question: q, Goal: q}
		writeJSON(w, 200, map[string]any{"items": supportSuggestions(repos, repo, a, stub, s, src.issues)})
	})
	mux.HandleFunc("GET "+base+"/solutions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		items, e := s.Solutions(string(repo.ID), r.URL.Query().Get("q"), actor, actor == "" || !participant(repo, actor))
		if supportError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("GET "+base+"/{question}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, v, a) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, project(repo, v, a))
	})
	mux.HandleFunc("POST "+base+"/{question}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		v, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, v, a) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e = s.Comment(string(repo.ID), v.ID, a, in.Body)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 201, project(repo, v, a))
	})
	mux.HandleFunc("POST "+base+"/{question}/answers", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, a) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in supportquestions.AnswerInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !validateGuidanceCitations(repo, a, q, in, src, s, repos) {
			writeJSON(w, 422, map[string]string{"error": "inaccessible_or_invalid_guidance_evidence"})
			return
		}
		v, e := s.ReviseAnswer(string(repo.ID), q.ID, a, in)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 201, project(repo, v, a))
	})
	mux.HandleFunc("POST "+base+"/{question}/answers/{answer}/feedback", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, a) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			RevisionID string `json:"revision_id"`
			ClaimID    string `json:"claim_id"`
			Kind       string `json:"kind"`
			Body       string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Feedback(string(repo.ID), q.ID, r.PathValue("answer"), in.RevisionID, in.ClaimID, a, in.Kind, in.Body)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 201, project(repo, v, a))
	})
	mux.HandleFunc("POST "+base+"/{question}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in supportquestions.VerificationInputRequest
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		attempt, e := s.CreateVerification(q, actor, "", in)
		if supportError(w, e) {
			return
		}
		runner.Start(attempt)
		writeJSON(w, 201, supportVerificationProjection(attempt, q, nil))
	})
	mux.HandleFunc("GET "+base+"/{question}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, e := s.ListVerifications(string(repo.ID), q.ID)
		if supportError(w, e) {
			return
		}
		out := make([]map[string]any, 0, len(items))
		var latest *supportquestions.VerificationAttempt
		if len(items) > 0 {
			latest = &items[len(items)-1]
		}
		for _, item := range items {
			out = append(out, supportVerificationProjection(item, q, latest))
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	})
	mux.HandleFunc("GET "+base+"/{question}/verifications/{verification}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item, e := s.GetVerification(string(repo.ID), q.ID, r.PathValue("verification"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, _ := s.ListVerifications(string(repo.ID), q.ID)
		var latest *supportquestions.VerificationAttempt
		if len(items) > 0 {
			latest = &items[len(items)-1]
		}
		writeJSON(w, 200, supportVerificationProjection(item, q, latest))
	})
	mux.HandleFunc("POST "+base+"/{question}/verifications/{verification}/reruns", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		prior, e := s.GetVerification(string(repo.ID), q.ID, r.PathValue("verification"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		in := supportquestions.VerificationInputRequest{AnswerID: prior.AnswerID, AnswerRevisionID: prior.AnswerRevisionID, SourceRevision: prior.SourceRevision, SoftwareVersion: prior.SoftwareVersion, Environment: prior.Environment, Dependencies: prior.Dependencies, Inputs: prior.Inputs, ArtifactPaths: prior.ArtifactPaths, CostUnits: prior.CostUnits}
		attempt, e := s.CreateVerification(q, actor, prior.ID, in)
		if supportError(w, e) {
			return
		}
		runner.Start(attempt)
		writeJSON(w, 201, supportVerificationProjection(attempt, q, &attempt))
	})
	mux.HandleFunc("POST "+base+"/{question}/solutions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if actor != q.AuthorID && !participant(repo, actor) {
			writeJSON(w, 403, map[string]string{"error": "support_participant_required"})
			return
		}
		var in supportquestions.ResolutionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		verification, e := s.GetVerification(string(repo.ID), q.ID, in.VerificationID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "passed_verification_required"})
			return
		}
		if !validSolutionLinks(string(repo.ID), in.Links, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_solution_link"})
			return
		}
		q, e = s.Resolve(string(repo.ID), q.ID, actor, in, verification)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 201, project(repo, q, actor))
	})
	mux.HandleFunc("POST "+base+"/{question}/solutions/{solution}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		if !participant(repo, actor) {
			writeJSON(w, 403, map[string]string{"error": "repository_maintainer_required"})
			return
		}
		q, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, q, actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Type             string `json:"type"`
			Reason           string `json:"reason"`
			TargetQuestionID string `json:"target_question_id"`
			TargetSolutionID string `json:"target_solution_id"`
			Version          string `json:"version"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		if in.Type == "merge" {
			target, er := s.Get(string(repo.ID), in.TargetQuestionID)
			found := false
			if er == nil {
				for _, x := range target.Solutions {
					found = found || x.ID == in.TargetSolutionID && x.Status != "archived" && x.Status != "merged"
				}
			}
			if !found {
				writeJSON(w, 422, map[string]string{"error": "invalid_merge_target"})
				return
			}
		}
		q, e = s.SolutionEvent(string(repo.ID), q.ID, r.PathValue("solution"), actor, in.Type, in.Reason, in.TargetQuestionID, in.TargetSolutionID, in.Version)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 201, project(repo, q, actor))
	})
	mux.HandleFunc("PATCH "+base+"/{question}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		v, e := s.Get(string(repo.ID), r.PathValue("question"))
		if e != nil || !visible(repo, v, a) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if a != v.AuthorID && !participant(repo, a) {
			writeJSON(w, 403, map[string]string{"error": "support_participant_required"})
			return
		}
		var in struct {
			Status string `json:"status"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		v, e = s.Status(string(repo.ID), v.ID, a, in.Status)
		if supportError(w, e) {
			return
		}
		writeJSON(w, 200, project(repo, v, a))
	})
}

func validSolutionLinks(repo string, links []supportquestions.SolutionLink, src supportSources) bool {
	for _, link := range links {
		switch link.Kind {
		case "documentation":
			_, e := src.docs.Get(repo, link.ResourceID)
			if e != nil {
				return false
			}
		case "package":
			_, e := src.packages.Get(repo, link.ResourceID)
			if e != nil {
				return false
			}
		case "release":
			_, e := src.releases.Get(repo, link.ResourceID)
			if e != nil {
				return false
			}
		case "contributor_guidance":
			if strings.TrimSpace(link.ResourceID) == "" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func supportVerificationProjection(a supportquestions.VerificationAttempt, q supportquestions.Question, latest *supportquestions.VerificationAttempt) map[string]any {
	reasons := []string{}
	currentAnswer, currentInstructions := false, false
	for _, answer := range q.Answers {
		if answer.ID == a.AnswerID && answer.CurrentID == a.AnswerRevisionID {
			currentAnswer = true
			for _, revision := range answer.Revisions {
				if revision.ID == a.AnswerRevisionID {
					currentInstructions = supportInstructionsDigest(revision.Instructions) == a.InstructionsDigest
				}
			}
		}
	}
	if !currentAnswer {
		reasons = append(reasons, "answer_revision_changed")
	}
	if !currentInstructions {
		reasons = append(reasons, "instructions_changed")
	}
	if q.SoftwareVersion != "" && q.SoftwareVersion != a.SoftwareVersion {
		reasons = append(reasons, "software_version_changed")
	}
	if q.Environment != "" && q.Environment != a.Environment.Name {
		reasons = append(reasons, "environment_changed")
	}
	if latest != nil && latest.ID != a.ID {
		if latest.SoftwareVersion != a.SoftwareVersion {
			reasons = append(reasons, "software_version_changed")
		}
		if latest.SourceRevision != a.SourceRevision {
			reasons = append(reasons, "source_revision_changed")
		}
		if latest.Environment.ImageDigest != a.Environment.ImageDigest {
			reasons = append(reasons, "environment_dependency_changed")
		}
		if !supportStringMapEqual(latest.Dependencies, a.Dependencies) {
			reasons = append(reasons, "dependencies_changed")
		}
		if !supportInputsEqual(latest.Inputs, a.Inputs) {
			reasons = append(reasons, "inputs_changed")
		}
	}
	return map[string]any{"attempt": a, "stale": len(reasons) > 0, "stale_reasons": reasons, "reusable_record_excludes_secrets_and_private_user_data": true, "authority": "verification grants no repository, credential, review, merge, environment, or operational authority"}
}
func supportInstructionsDigest(v []string) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func supportStringMapEqual(a, b map[string]string) bool { return reflect.DeepEqual(a, b) }
func supportInputsEqual(a, b []supportquestions.VerificationInput) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].SHA256 != b[i].SHA256 {
			return false
		}
	}
	return true
}

func validateGuidanceCitations(repo repositories.Repository, actor string, q supportquestions.Question, in supportquestions.AnswerInput, src supportSources, questions *supportquestions.Store, repos proposalRepositoryStore) bool {
	for _, claim := range in.Claims {
		for _, c := range claim.Citations {
			if c.Visibility != q.Audience || strings.TrimSpace(c.Revision) == "" {
				return false
			}
			switch c.Kind {
			case "source", "symbol":
				if c.ResourceID != "" && c.ResourceID != string(repo.ID) || strings.TrimSpace(c.Path) == "" || (c.Kind == "symbol" && strings.TrimSpace(c.Symbol) == "") || c.LineStart < 0 || c.LineEnd < c.LineStart {
					return false
				}
				opener, ok := repos.(interface {
					Open(storage.ID) (*storage.Repository, error)
				})
				if !ok {
					return false
				}
				opened, e := opener.Open(repo.ID)
				if e != nil {
					return false
				}
				commit, _, e := resolveRevision(opened, c.Revision)
				if e != nil {
					return false
				}
				if !sourceCitationExists(opened, commit, c.Path, c.LineStart, c.LineEnd) {
					return false
				}
				if c.Kind == "symbol" {
					sources, _, _, e := collectSources(opened, commit)
					if e != nil {
						return false
					}
					found := false
					for _, symbol := range buildSymbols(opened, commit, sources, "", c.Symbol) {
						if symbol.Name == c.Symbol && symbol.Definition.Path == c.Path {
							found = true
						}
					}
					if !found {
						return false
					}
				}
			case "documentation":
				v, e := src.docs.Get(string(repo.ID), c.ResourceID)
				found := false
				for _, version := range v.History {
					if c.Revision == strconv.FormatInt(version.Number, 10) {
						found = true
					}
				}
				if e != nil || !found {
					return false
				}
			case "package":
				v, e := src.packages.Get(string(repo.ID), c.ResourceID)
				if e != nil || v.RepositoryID != string(repo.ID) || !(c.Revision == v.SourceCommitID || c.Revision == v.Version || c.Revision == v.ID) {
					return false
				}
			case "release":
				v, e := src.releases.Get(string(repo.ID), c.ResourceID)
				if e != nil || v.RepositoryID != string(repo.ID) || !(c.Revision == v.CommitID || c.Revision == v.Version || c.Revision == v.ID) {
					return false
				}
			case "support_question":
				v, e := questions.Get(string(repo.ID), c.ResourceID)
				if e != nil || c.Revision != strconv.FormatInt(v.Version, 10) || !(v.Audience == "public" && repo.Visibility == repositories.Public || v.AuthorID == actor || supportParticipant(repos, repo, actor)) {
					return false
				}
			case "issue":
				items, e := src.issues.List(string(repo.ID))
				if e != nil {
					return false
				}
				found := false
				for _, v := range items {
					if v.ID == c.ResourceID && c.Revision == strconv.FormatInt(v.Version, 10) && issueVisible(repos, repo, v, actor) {
						found = true
					}
				}
				if !found {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

func sourceCitationExists(repo *storage.Repository, commit storage.ObjectID, path string, start, end int) bool {
	c, e := repo.ReadCommit(commit)
	if e != nil {
		return false
	}
	tree := c.Tree
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var oid storage.ObjectID
	for i, p := range parts {
		t, e := repo.ReadTree(tree)
		if e != nil {
			return false
		}
		found := false
		for _, x := range t.Entries {
			if x.Name == p {
				found = true
				oid = x.ObjectID
				if i < len(parts)-1 {
					tree = x.ObjectID
				}
				break
			}
		}
		if !found {
			return false
		}
	}
	o, e := repo.ReadObject(oid)
	if e != nil {
		return false
	}
	lines := strings.Count(string(o.Content), "\n") + 1
	return start == 0 && end == 0 || start >= 1 && end >= start && end <= lines
}

func validSupportSubject(repo string, s supportquestions.Subject, src supportSources) bool {
	switch s.Kind {
	case "repository":
		return s.ResourceID == "" || s.ResourceID == repo
	case "package":
		v, e := src.packages.Get(repo, s.ResourceID)
		return e == nil && v.RepositoryID == repo
	case "release":
		v, e := src.releases.Get(repo, s.ResourceID)
		return e == nil && v.RepositoryID == repo
	case "journey":
		_, e := src.docs.Get(repo, s.ResourceID)
		return e == nil
	case "api", "error":
		return strings.TrimSpace(s.ResourceID) != ""
	}
	return false
}
func supportSuggestions(repos proposalRepositoryStore, repo repositories.Repository, actor string, q supportquestions.Question, s *supportquestions.Store, is interface {
	List(string) ([]issues.Issue, error)
}) []supportquestions.Related {
	wanted := words(q.Title + " " + q.Question + " " + q.Goal)
	if len(wanted) == 0 {
		return []supportquestions.Related{}
	}
	type hit struct {
		v supportquestions.Related
		n int
	}
	hits := []hit{}
	questions, _ := s.List(string(repo.ID))
	for _, v := range questions {
		if v.ID == q.ID || !(v.Audience == "public" && repo.Visibility == repositories.Public || v.AuthorID == actor || supportParticipant(repos, repo, actor)) {
			continue
		}
		n := overlap(wanted, words(v.Title+" "+v.Question+" "+v.Goal))
		if n > 0 {
			hits = append(hits, hit{supportquestions.Related{Kind: "support_question", ResourceID: v.ID, Title: v.Title, Status: v.Status}, n})
		}
	}
	issuesList, _ := is.List(string(repo.ID))
	for _, v := range issuesList {
		if !issueVisible(repos, repo, v, actor) {
			continue
		}
		n := overlap(wanted, words(v.Title+" "+v.ObservedBehavior))
		if n > 0 {
			hits = append(hits, hit{supportquestions.Related{Kind: "issue", ResourceID: v.ID, Title: v.Title, Status: v.Status}, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].n == hits[j].n {
			return hits[i].v.Title < hits[j].v.Title
		}
		return hits[i].n > hits[j].n
	})
	out := []supportquestions.Related{}
	for i, v := range hits {
		if i == 5 {
			break
		}
		out = append(out, v.v)
	}
	return out
}
func supportParticipant(repos proposalRepositoryStore, repo repositories.Repository, actor string) bool {
	if actor == "" {
		return false
	}
	if actor == repo.OwnerID {
		return true
	}
	ok, _ := repos.IsCollaborator(repo.ID, actor)
	return ok
}
func overlap(a, b map[string]bool) int {
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	return n
}
func supportError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, supportquestions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, supportquestions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_support_question"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
