package main

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
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

func registerSupportQuestionsHTTP(mux *http.ServeMux, s *supportquestions.Store, repos proposalRepositoryStore, credentials authStore, src supportSources) {
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
