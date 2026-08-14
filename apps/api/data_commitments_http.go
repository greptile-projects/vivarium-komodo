package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
)

type dataCommitmentStore interface {
	Create(string, string, datacommitments.VersionInput) (datacommitments.Commitment, error)
	Revise(string, string, string, int64, datacommitments.VersionInput) (datacommitments.Commitment, error)
	Get(string, string) (datacommitments.Commitment, error)
	List(string) ([]datacommitments.Commitment, error)
}

func registerDataCommitmentsHTTP(mux *http.ServeMux, store dataCommitmentStore, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/data-commitments"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if dataCommitmentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in datacommitments.VersionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := store.Create(string(repo.ID), a.UserID, in)
		if dataCommitmentError(w, e) {
			return
		}
		writeJSON(w, 201, c)
	})
	mux.HandleFunc("GET "+base+"/{commitment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, e := store.Get(string(repo.ID), r.PathValue("commitment"))
		if dataCommitmentError(w, e) {
			return
		}
		writeJSON(w, 200, c)
	})
	mux.HandleFunc("POST "+base+"/{commitment}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			datacommitments.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := store.Revise(string(repo.ID), r.PathValue("commitment"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if dataCommitmentError(w, e) {
			return
		}
		writeJSON(w, 201, c)
	})
}

func dataCommitmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, datacommitments.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "data_commitment_not_found"})
	case errors.Is(e, datacommitments.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_data_commitment"})
	case errors.Is(e, datacommitments.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "data_commitment_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
