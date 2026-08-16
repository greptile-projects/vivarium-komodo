package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryresponses"
)

func registerRecoveryResponsesHTTP(mux *http.ServeMux, s *recoveryresponses.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/recovery-responses"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if !recoveryResponseError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.ActivationInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Activate(string(repo.ID), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{response}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("response"))
		if !recoveryResponseError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{response}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.DecisionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Approve(string(repo.ID), r.PathValue("response"), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{response}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.DecisionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Decide(string(repo.ID), r.PathValue("response"), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{response}/steps/{step}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.StepUpdate
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateStep(string(repo.ID), r.PathValue("response"), r.PathValue("step"), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{response}/communications", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.CommunicationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Communicate(string(repo.ID), r.PathValue("response"), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{response}/validations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryresponses.ValidationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Validate(string(repo.ID), r.PathValue("response"), a.UserID, in)
		if !recoveryResponseError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func recoveryResponseError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, recoveryresponses.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "recovery_response_not_found"})
	case errors.Is(e, recoveryresponses.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_response"})
	case errors.Is(e, recoveryresponses.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "recovery_response_changed_or_blocked"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
