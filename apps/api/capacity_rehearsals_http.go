package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityrehearsals"
	"net/http"
)

type capacityRehearsalStore interface {
	Create(string, string, capacityrehearsals.Input) (capacityrehearsals.Rehearsal, error)
	AppendAttempt(string, string, string, capacityrehearsals.AttemptInput) (capacityrehearsals.Rehearsal, error)
	Get(string, string) (capacityrehearsals.Rehearsal, error)
	List(string) ([]capacityrehearsals.Rehearsal, error)
}

func registerCapacityRehearsalsHTTP(mux *http.ServeMux, store capacityRehearsalStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/capacity-rehearsals"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if rehearsalError(w, e) {
			return
		}
		for i := range xs {
			xs[i] = capacityrehearsals.Resolve(xs[i])
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in capacityrehearsals.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, e := store.Create(string(repo.ID), a.UserID, in)
		if rehearsalError(w, e) {
			return
		}
		writeJSON(w, 201, capacityrehearsals.Resolve(x))
	})
	mux.HandleFunc("GET "+base+"/{rehearsal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := store.Get(string(repo.ID), r.PathValue("rehearsal"))
		if rehearsalError(w, e) {
			return
		}
		writeJSON(w, 200, capacityrehearsals.Resolve(x))
	})
	mux.HandleFunc("POST "+base+"/{rehearsal}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in capacityrehearsals.AttemptInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, e := store.AppendAttempt(string(repo.ID), r.PathValue("rehearsal"), a.UserID, in)
		if rehearsalError(w, e) {
			return
		}
		writeJSON(w, 201, capacityrehearsals.Resolve(x))
	})
}
func rehearsalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capacityrehearsals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capacity_rehearsal_not_found"})
	case errors.Is(e, capacityrehearsals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capacity_rehearsal"})
	case errors.Is(e, capacityrehearsals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capacity_rehearsal_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
