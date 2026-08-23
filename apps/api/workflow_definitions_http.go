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
	mux.HandleFunc("POST "+base+"/{workflow}/candidate-decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Version   int64  `json:"version"`
			Kind      string `json:"kind"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RecordCandidateDecision(string(repo.ID), r.PathValue("workflow"), a.UserID, in.Version, in.Kind, in.Decision, in.Rationale)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/simulations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.SimulationResult
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RecordSimulation(string(repo.ID), r.PathValue("workflow"), a.UserID, in)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/exceptions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.Exception
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddException(string(repo.ID), r.PathValue("workflow"), a.UserID, in)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/disable", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Disable(string(repo.ID), r.PathValue("workflow"), a.UserID, in.Reason)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/rollback", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Version int64  `json:"version"`
			Reason  string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Rollback(string(repo.ID), r.PathValue("workflow"), a.UserID, in.Version, in.Reason)
		if !workflowDefinitionError(w, e, &x) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{workflow}/executions", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Executions(string(repo.ID), r.PathValue("workflow"))
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.InvokeInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Invoke(string(repo.ID), r.PathValue("workflow"), a.UserID, in)
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{workflow}/executions/{execution}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetExecution(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"))
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions/{execution}/steps/{step}/dispatch", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.DispatchInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Dispatch(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"), a.UserID, r.PathValue("step"), in)
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions/{execution}/steps/{step}/approval-requests", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RequestActionApproval(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"), a.UserID, r.PathValue("step"), in.ExpectedRevision)
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions/{execution}/approval-requests/{approval}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64  `json:"expected_revision"`
			Decision         string `json:"decision"`
			Rationale        string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.DecideActionApproval(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"), r.PathValue("approval"), a.UserID, in.Decision, in.Rationale, in.ExpectedRevision)
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions/{execution}/steps/{step}/results", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.ResultInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RecordResult(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"), a.UserID, r.PathValue("step"), in)
		if !workflowDefinitionError(w, e, nil) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{workflow}/executions/{execution}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workflowdefinitions.ControlInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Control(string(repo.ID), r.PathValue("workflow"), r.PathValue("execution"), a.UserID, in)
		if !workflowDefinitionError(w, e, nil) {
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
		if x != nil {
			writeJSON(w, 422, map[string]any{"error": "workflow_activation_blocked", "diagnostics": x.Diagnostics})
		} else {
			writeJSON(w, 429, map[string]string{"error": "workflow_execution_limited", "recovery": "retry after a running execution or rate window completes"})
		}
	case errors.Is(e, workflowdefinitions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_workflow_definition"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
