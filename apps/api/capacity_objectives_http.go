package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityobjectives"
	"net/http"
	"time"
)

type capacityObjectiveStore interface {
	Create(string, string, capacityobjectives.VersionInput) (capacityobjectives.Objective, error)
	Revise(string, string, string, int64, capacityobjectives.VersionInput) (capacityobjectives.Objective, error)
	Get(string, string) (capacityobjectives.Objective, error)
	List(string) ([]capacityobjectives.Objective, error)
}

func registerCapacityObjectivesHTTP(mux *http.ServeMux, store capacityObjectiveStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/capacity-objectives"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if capacityObjectiveError(w, e) {
			return
		}
		for i := range items {
			items[i] = capacityobjectives.Resolve(items[i], time.Now().UTC())
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in capacityobjectives.VersionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		o, e := store.Create(string(repo.ID), a.UserID, in)
		if capacityObjectiveError(w, e) {
			return
		}
		writeJSON(w, 201, capacityobjectives.Resolve(o, time.Now().UTC()))
	})
	mux.HandleFunc("GET "+base+"/{objective}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		o, e := store.Get(string(repo.ID), r.PathValue("objective"))
		if capacityObjectiveError(w, e) {
			return
		}
		writeJSON(w, 200, capacityobjectives.Resolve(o, time.Now().UTC()))
	})
	mux.HandleFunc("POST "+base+"/{objective}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			capacityobjectives.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		o, e := store.Revise(string(repo.ID), r.PathValue("objective"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if capacityObjectiveError(w, e) {
			return
		}
		writeJSON(w, 201, capacityobjectives.Resolve(o, time.Now().UTC()))
	})
}
func capacityObjectiveError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capacityobjectives.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capacity_objective_not_found"})
	case errors.Is(e, capacityobjectives.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capacity_objective"})
	case errors.Is(e, capacityobjectives.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capacity_objective_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
