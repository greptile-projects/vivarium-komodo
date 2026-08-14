package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"net/http"
)

func registerServiceObjectivesHTTP(mux *http.ServeMux, s *serviceobjectives.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/service-objectives"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if serviceObjectiveError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in serviceobjectives.VersionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if serviceObjectiveError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{objective}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("objective"))
		if serviceObjectiveError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{objective}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			serviceobjectives.VersionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("objective"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if serviceObjectiveError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}
func serviceObjectiveError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, serviceobjectives.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "service_objective_not_found"})
	case errors.Is(e, serviceobjectives.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_service_objective"})
	case errors.Is(e, serviceobjectives.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "service_objective_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
