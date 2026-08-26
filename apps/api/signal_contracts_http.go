package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalcontracts"
	"net/http"
)

type signalContractStore interface {
	Create(string, string, signalcontracts.Input) (signalcontracts.Contract, error)
	Revise(string, string, string, int64, signalcontracts.Input) (signalcontracts.Contract, error)
	Challenge(string, string, string, bool, int64, string, string, []signalcontracts.Evidence) (signalcontracts.Contract, error)
	Get(string, string) (signalcontracts.Contract, error)
	List(string) ([]signalcontracts.Contract, error)
}

func registerSignalContractsHTTP(mux *http.ServeMux, store signalContractStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/signal-contracts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if signalContractError(w, e) {
			return
		}
		for i := range xs {
			xs[i] = signalcontracts.Resolve(xs[i])
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalcontracts.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		c, e := store.Create(string(repo.ID), a.UserID, in)
		if signalContractError(w, e) {
			return
		}
		writeJSON(w, 201, signalcontracts.Resolve(c))
	})
	mux.HandleFunc("GET "+base+"/{contract}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, e := store.Get(string(repo.ID), r.PathValue("contract"))
		if signalContractError(w, e) {
			return
		}
		writeJSON(w, 200, signalcontracts.Resolve(c))
	})
	mux.HandleFunc("POST "+base+"/{contract}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			signalcontracts.Input
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		c, e := store.Revise(string(repo.ID), r.PathValue("contract"), a.UserID, in.ExpectedVersion, in.Input)
		if signalContractError(w, e) {
			return
		}
		writeJSON(w, 201, signalcontracts.Resolve(c))
	})
	mux.HandleFunc("POST "+base+"/{contract}/challenges", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Version    int64                      `json:"version"`
			Agent      bool                       `json:"agent"`
			Assumption string                     `json:"assumption"`
			Position   string                     `json:"position"`
			Evidence   []signalcontracts.Evidence `json:"evidence"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := store.Challenge(string(repo.ID), r.PathValue("contract"), a.UserID, in.Agent, in.Version, in.Assumption, in.Position, in.Evidence)
		if signalContractError(w, e) {
			return
		}
		writeJSON(w, 201, signalcontracts.Resolve(c))
	})
}
func signalContractError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, signalcontracts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "signal_contract_not_found"})
	case errors.Is(e, signalcontracts.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_signal_contract"})
	case errors.Is(e, signalcontracts.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "signal_contract_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
