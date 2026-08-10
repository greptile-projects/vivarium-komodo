package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
)

type decisionStore interface {
	Create(string, string, string, decisions.Context, decisions.ScopeInput) (decisions.Decision, error)
	Get(string, string) (decisions.Decision, error)
	List(string, string, string) ([]decisions.Decision, error)
	Revise(string, string, string, string, decisions.ScopeInput) (decisions.Decision, error)
	Comment(string, string, string, string) (decisions.Decision, error)
}

func registerDecisionsHTTP(mux *http.ServeMux, s decisionStore, repos proposalRepositoryStore, c authStore) {
	base := "/repositories/{repository}/decisions"
	mux.HandleFunc("POST "+base, createDecision(s, repos, c))
	mux.HandleFunc("GET "+base, listDecisions(s, repos, c))
	mux.HandleFunc("GET "+base+"/{decision}", getDecision(s, repos, c))
	mux.HandleFunc("PATCH "+base+"/{decision}", reviseDecision(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/comments", commentDecision(s, repos, c))
}
func createDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title   string            `json:"title"`
			Context decisions.Context `json:"context"`
			decisions.ScopeInput
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if !decisionActors(repo.OwnerID, repo.CollaboratorIDs, in.ParticipantIDs, in.OwnerID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_participants"})
			return
		}
		v, e := s.Create(string(repo.ID), a.UserID, in.Title, in.Context, in.ScopeInput)
		writeDecision(w, v, e, 201)
	}
}
func listDecisions(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID), strings.TrimSpace(r.URL.Query().Get("context_kind")), strings.TrimSpace(r.URL.Query().Get("context_id")))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := items[:0]
		for _, v := range items {
			if a.UserID == "" || containsDecisionActor(v.Scope.ParticipantIDs, a.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func getDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("repository"), r.PathValue("decision"))
		if e != nil || (a.UserID != "" && !containsDecisionActor(v.Scope.ParticipantIDs, a.UserID)) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func reviseDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title string `json:"title"`
			decisions.ScopeInput
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if !decisionActors(repo.OwnerID, repo.CollaboratorIDs, in.ParticipantIDs, in.OwnerID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_participants"})
			return
		}
		v, e := s.Revise(string(repo.ID), r.PathValue("decision"), a.UserID, in.Title, in.ScopeInput)
		writeDecision(w, v, e, 200)
	}
}
func commentDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Comment(string(repo.ID), r.PathValue("decision"), a.UserID, in.Body)
		writeDecision(w, v, e, 201)
	}
}
func decisionActors(owner string, collabs, participants []string, decisionOwner string) bool {
	if len(participants) == 0 {
		return false
	}
	allowed := map[string]bool{owner: true}
	for _, x := range collabs {
		allowed[x] = true
	}
	for _, x := range participants {
		if !allowed[x] {
			return false
		}
	}
	return allowed[decisionOwner]
}
func containsDecisionActor(v []string, x string) bool {
	for _, a := range v {
		if a == x {
			return true
		}
	}
	return false
}
func writeDecision(w http.ResponseWriter, v decisions.Decision, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
	} else if errors.Is(e, decisions.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_decision"})
	} else if errors.Is(e, decisions.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
