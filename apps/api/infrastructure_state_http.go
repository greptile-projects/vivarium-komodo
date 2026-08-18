package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

func registerInfrastructureStateHTTP(mux *http.ServeMux, s *infrastructurestate.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/infrastructure-definitions"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if infrastructureStateError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in infrastructurestate.VersionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if infrastructureStateError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{definition}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("definition"))
		if infrastructureStateError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{definition}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			infrastructurestate.VersionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("definition"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if infrastructureStateError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{definition}/observations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in infrastructurestate.ObservationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Observe(string(repo.ID), r.PathValue("definition"), a.UserID, in)
		if infrastructureStateError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}

func infrastructureStateError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, infrastructurestate.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "infrastructure_definition_not_found"})
	case errors.Is(e, infrastructurestate.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_infrastructure_state"})
	case errors.Is(e, infrastructurestate.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "infrastructure_definition_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
