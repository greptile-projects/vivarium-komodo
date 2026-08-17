package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAgentEvaluationsHTTP(m *http.ServeMux, s *agentevaluations.Store, profiles *agentprofiles.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/agent-evaluations"
	m.HandleFunc("GET "+base+"/suites", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.ListSuites(string(repo.ID))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	m.HandleFunc("POST "+base+"/suites", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.SuiteInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/suites/{suite}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetSuite(string(repo.ID), r.PathValue("suite"), false)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/suites/{suite}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.SuiteInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("suite"), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/trials", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		xs, e := s.ListTrials(string(repo.ID))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	m.HandleFunc("POST "+base+"/trials", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.TrialInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		profile, e := profiles.Get(in.ProfileID)
		if e != nil || in.ProfileVersion > profile.CurrentVersion {
			agentEvaluationError(w, agentevaluations.ErrInvalid)
			return
		}
		x, e := s.Start(string(repo.ID), a.UserID, in)
		if !agentEvaluationError(w, e) {
			w.Header().Set("Location", "/repositories/"+string(repo.ID)+"/agent-evaluations/trials/"+x.ID)
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/trials/{trial}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		x, e := s.GetTrial(string(repo.ID), r.PathValue("trial"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/trials/{trial}/result", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.ResultInput
		if !readJSON(w, r, &in, 4<<20) {
			return
		}
		x, e := s.Complete(string(repo.ID), r.PathValue("trial"), in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/trials/{trial}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.DecisionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Decide(string(repo.ID), r.PathValue("trial"), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func agentEvaluationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentevaluations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_evaluation_not_found"})
	case errors.Is(e, agentevaluations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "stale_agent_evaluation_version"})
	case errors.Is(e, agentevaluations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_evaluation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
