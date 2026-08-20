package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"net/http"
)

func registerAssuranceProgramsHTTP(mux *http.ServeMux, s *assuranceprograms.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/assurance-programs"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !assuranceProgramError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assuranceprograms.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !assuranceProgramError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{program}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("program"))
		if !assuranceProgramError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{program}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			assuranceprograms.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("program"), a.UserID, in.ExpectedVersion, in.Input)
		if !assuranceProgramError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func assuranceProgramError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, assuranceprograms.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "assurance_program_not_found"})
	case errors.Is(e, assuranceprograms.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "assurance_program_changed_or_conflicting"})
	case errors.Is(e, assuranceprograms.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_assurance_program"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
