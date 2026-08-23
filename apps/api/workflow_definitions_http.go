package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowdefinitions"
	"net/http"
)

func registerWorkflowDefinitionsHTTP(mux *http.ServeMux, s *workflowdefinitions.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/workflow-definitions"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{workflow}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("workflow"))
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			workflowdefinitions.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("workflow"), a.UserID, in.ExpectedVersion, in.Input)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/activation", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Version int64 `json:"version"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Activate(string(repo.ID), r.PathValue("workflow"), a.UserID, in.Version)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
}
func workflowDefinitionError(w http.ResponseWriter, e error, x *workflowdefinitions.Workflow) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, workflowdefinitions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "workflow_not_found"})
	case errors.Is(e, workflowdefinitions.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "workflow_changed_or_not_owned"})
	case errors.Is(e, workflowdefinitions.ErrBlocked):
		writeJSON(w, 422, map[string]any{"error": "workflow_activation_blocked", "diagnostics": x.Diagnostics})
	case errors.Is(e, workflowdefinitions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_workflow_definition"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
