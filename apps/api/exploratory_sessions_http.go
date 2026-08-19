package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/exploratorysessions"
)

func registerExploratorySessionsHTTP(mux *http.ServeMux, s *exploratorysessions.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/exploratory-sessions"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		permission := auth.RepositoryRead
		if write {
			permission = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, permission, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Catalog(repo)
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in exploratorysessions.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Create(repo, actor, in)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{session}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("session"))
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/timeline", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.EventInput
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Append(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.EventInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.FindingInput
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.AddFinding(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.FindingInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("PATCH "+base+"/{session}/findings/{finding}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.FindingUpdate
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateFinding(repo, r.PathValue("session"), r.PathValue("finding"), actor, in.ExpectedRevision, in.FindingUpdate)
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.ControlInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Control(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.ControlInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/candidate-revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.CandidateUpdate
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateCandidate(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.CandidateUpdate)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func explorationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, exploratorysessions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "exploratory_session_not_found"})
	case errors.Is(e, exploratorysessions.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "exploratory_session_changed_or_inactive"})
	case errors.Is(e, exploratorysessions.ErrScope):
		writeJSON(w, 403, map[string]string{"error": "exploratory_session_scope_exceeded"})
	case errors.Is(e, exploratorysessions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_exploratory_session"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
