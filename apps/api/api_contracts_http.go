package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAPIContractsHTTP(mux *http.ServeMux, s *apicontracts.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/api-contracts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if apiContractError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in apicontracts.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if apiContractError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{contract}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("contract"))
		if apiContractError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{contract}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			apicontracts.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("contract"), a.UserID, in.ExpectedVersion, in.Input)
		if apiContractError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{contract}/compare", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		from, e1 := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		to, e2 := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if e1 != nil || e2 != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_api_contract_comparison"})
			return
		}
		x, e := s.Compare(string(repo.ID), r.PathValue("contract"), from, to)
		if apiContractError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
}
func apiContractError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, apicontracts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "api_contract_not_found"})
	case errors.Is(e, apicontracts.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "api_contract_changed"})
	case errors.Is(e, apicontracts.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_api_contract"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
