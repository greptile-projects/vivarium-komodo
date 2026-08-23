package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowcomponents"
)

func registerWorkflowComponentsHTTP(mux *http.ServeMux, s *workflowcomponents.Store, repos dataFlowRepositories, credentials authStore) {
	components := "/repositories/{repository}/workflow-components"
	installations := "/repositories/{repository}/workflow-component-installations"
	mux.HandleFunc("GET "+components, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Components(string(repo.ID))
		if !workflowComponentError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+components, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowcomponents.PublishInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Publish(string(repo.ID), a.UserID, in)
		if !workflowComponentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+components+"/{component}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetComponent(string(repo.ID), r.PathValue("component"))
		if !workflowComponentError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("GET "+installations, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Installations(string(repo.ID))
		if !workflowComponentError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+installations, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowcomponents.InstallInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Install(string(repo.ID), a.UserID, in)
		if !workflowComponentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+installations+"/{installation}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			workflowcomponents.InstallInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("installation"), a.UserID, in.ExpectedRevision, in.InstallInput)
		if !workflowComponentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func workflowComponentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, workflowcomponents.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "workflow_component_not_found"})
	case errors.Is(e, workflowcomponents.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "workflow_component_changed"})
	case errors.Is(e, workflowcomponents.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_workflow_component"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
