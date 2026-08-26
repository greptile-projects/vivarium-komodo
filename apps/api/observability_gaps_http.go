package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/observabilitygaps"
	"net/http"
	"time"
)

type observabilityGapStore interface {
	Create(string, string, observabilitygaps.Input) (observabilitygaps.Gap, error)
	Revise(string, string, string, int64, observabilitygaps.Input) (observabilitygaps.Gap, error)
	Get(string, string) (observabilitygaps.Gap, error)
	List(string) ([]observabilitygaps.Gap, error)
}

func registerObservabilityGapsHTTP(mux *http.ServeMux, store observabilityGapStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/observability-gaps"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if observabilityGapError(w, e) {
			return
		}
		for i := range xs {
			xs[i] = observabilitygaps.Resolve(xs[i], time.Now().UTC())
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in observabilitygaps.Input
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		g, e := store.Create(string(repo.ID), a.UserID, in)
		if observabilityGapError(w, e) {
			return
		}
		writeJSON(w, 201, observabilitygaps.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("GET "+base+"/{gap}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		g, e := store.Get(string(repo.ID), r.PathValue("gap"))
		if observabilityGapError(w, e) {
			return
		}
		writeJSON(w, 200, observabilitygaps.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("POST "+base+"/{gap}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			observabilitygaps.Input
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		g, e := store.Revise(string(repo.ID), r.PathValue("gap"), a.UserID, in.ExpectedVersion, in.Input)
		if observabilityGapError(w, e) {
			return
		}
		writeJSON(w, 201, observabilitygaps.Resolve(g, time.Now().UTC()))
	})
}
func observabilityGapError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, observabilitygaps.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "observability_gap_not_found"})
	case errors.Is(e, observabilitygaps.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_observability_gap"})
	case errors.Is(e, observabilitygaps.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "observability_gap_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
