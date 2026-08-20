package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprojects"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"net/http"
)

func registerAgentProjectsHTTP(mux *http.ServeMux, s *agentprojects.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/agent-projects"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !agentProjectError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentprojects.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !agentProjectError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{project}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("project"))
		if !agentProjectError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{project}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			agentprojects.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("project"), a.UserID, in.ExpectedVersion, in.Input)
		if !agentProjectError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func agentProjectError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentprojects.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_project_not_found"})
	case errors.Is(e, agentprojects.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "agent_project_changed_or_conflicting"})
	case errors.Is(e, agentprojects.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_project"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
