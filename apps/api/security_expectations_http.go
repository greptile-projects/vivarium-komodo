package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityexpectations"
	"net/http"
)

func registerSecurityExpectationsHTTP(mux *http.ServeMux, s *securityexpectations.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/security-expectations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !securityExpectationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in securityexpectations.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !securityExpectationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{expectation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("expectation"))
		if !securityExpectationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{expectation}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			securityexpectations.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("expectation"), a.UserID, in.ExpectedVersion, in.Input)
		if !securityExpectationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func securityExpectationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, securityexpectations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "security_expectation_not_found"})
	case errors.Is(e, securityexpectations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "security_expectation_changed_or_conflicting"})
	case errors.Is(e, securityexpectations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_security_expectation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
