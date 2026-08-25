package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacitymodels"
	"net/http"
)

type capacityModelStore interface {
	Create(string, string, capacitymodels.Input) (capacitymodels.Model, error)
	Challenge(string, string, string, capacitymodels.ChallengeInput) (capacitymodels.Model, error)
	Get(string, string) (capacitymodels.Model, error)
	List(string) ([]capacitymodels.Model, error)
}

func registerCapacityModelsHTTP(mux *http.ServeMux, store capacityModelStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/capacity-models"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if capacityModelError(w, e) {
			return
		}
		for i := range items {
			items[i] = capacitymodels.Resolve(items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in capacitymodels.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		m, e := store.Create(string(repo.ID), a.UserID, in)
		if capacityModelError(w, e) {
			return
		}
		writeJSON(w, 201, capacitymodels.Resolve(m))
	})
	mux.HandleFunc("GET "+base+"/{model}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		m, e := store.Get(string(repo.ID), r.PathValue("model"))
		if capacityModelError(w, e) {
			return
		}
		writeJSON(w, 200, capacitymodels.Resolve(m))
	})
	mux.HandleFunc("POST "+base+"/{model}/challenges", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in capacitymodels.ChallengeInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		m, e := store.Challenge(string(repo.ID), r.PathValue("model"), a.UserID, in)
		if capacityModelError(w, e) {
			return
		}
		writeJSON(w, 201, capacitymodels.Resolve(m))
	})
}
func capacityModelError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capacitymodels.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capacity_model_not_found"})
	case errors.Is(e, capacitymodels.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capacity_model"})
	case errors.Is(e, capacitymodels.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capacity_model_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
