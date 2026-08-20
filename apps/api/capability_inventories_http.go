package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
	"net/http"
)

func registerCapabilityInventoriesHTTP(mux *http.ServeMux, s *capabilityinventories.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/capability-inventories"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID))
		if !capabilityInventoryError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in capabilityinventories.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !capabilityInventoryError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{inventory}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("inventory"))
		if !capabilityInventoryError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{inventory}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			capabilityinventories.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("inventory"), a.UserID, in.ExpectedVersion, in.Input)
		if !capabilityInventoryError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func capabilityInventoryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capabilityinventories.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capability_inventory_not_found"})
	case errors.Is(e, capabilityinventories.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capability_inventory_changed_or_conflicting"})
	case errors.Is(e, capabilityinventories.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capability_inventory"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
