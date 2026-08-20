package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAgentScenariosHTTP(m *http.ServeMux, s *agentscenarios.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/agent-scenarios"
	m.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if e != nil {
			agentScenarioError(w, e)
			return
		}
		for i := range x.Items {
			x.Items[i] = agentscenarios.Project(x.Items[i], a.UserID != "")
		}
		writeJSON(w, 200, x)
	})
	m.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentscenarios.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !agentScenarioError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{scenario}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("scenario"))
		if !agentScenarioError(w, e) {
			writeJSON(w, 200, agentscenarios.Project(x, a.UserID != ""))
		}
	})
	m.HandleFunc("POST "+base+"/{scenario}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			agentscenarios.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("scenario"), a.UserID, in.ExpectedVersion, in.Input)
		if !agentScenarioError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{scenario}/reviews", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ScenarioVersion int64  `json:"scenario_version"`
			ReviewerKind    string `json:"reviewer_kind"`
			Decision        string `json:"decision"`
			Rationale       string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if in.ReviewerKind == "" {
			in.ReviewerKind = "human"
		}
		x, e := s.Review(string(repo.ID), r.PathValue("scenario"), a.UserID, in.ReviewerKind, in.ScenarioVersion, in.Decision, in.Rationale)
		if !agentScenarioError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func agentScenarioError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentscenarios.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_scenario_not_found"})
	case errors.Is(e, agentscenarios.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "agent_scenario_changed_or_conflicting"})
	case errors.Is(e, agentscenarios.ErrUnsafeContext):
		writeJSON(w, 422, map[string]string{"error": "unsafe_agent_scenario_context"})
	case errors.Is(e, agentscenarios.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "agent_scenario_owner_or_scope_required"})
	case errors.Is(e, agentscenarios.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_scenario"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
