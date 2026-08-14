package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

type accessibilityCommitmentStore interface {
	Create(string, string, accessibilitycommitments.VersionInput) (accessibilitycommitments.Commitment, error)
	Revise(string, string, string, int64, accessibilitycommitments.VersionInput) (accessibilitycommitments.Commitment, error)
	RecordCoverage(string, string, string, accessibilitycommitments.CoverageInput) (accessibilitycommitments.Commitment, error)
	Get(string, string) (accessibilitycommitments.Commitment, error)
	List(string) ([]accessibilitycommitments.Commitment, error)
}

func registerAccessibilityCommitmentsHTTP(mux *http.ServeMux, store accessibilityCommitmentStore, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/accessibility-commitments"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if accessibilityCommitmentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in accessibilitycommitments.VersionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := store.Create(string(repo.ID), a.UserID, in)
		if accessibilityCommitmentError(w, e) {
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
		if accessibilityCommitmentError(w, e) {
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
			accessibilitycommitments.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		c, e := store.Revise(string(repo.ID), r.PathValue("commitment"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if accessibilityCommitmentError(w, e) {
			return
		}
		writeJSON(w, 201, c)
	})
	mux.HandleFunc("POST "+base+"/{commitment}/coverage", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in accessibilitycommitments.CoverageInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		c, e := store.RecordCoverage(string(repo.ID), r.PathValue("commitment"), a.UserID, in)
		if accessibilityCommitmentError(w, e) {
			return
		}
		c, e = store.Get(string(repo.ID), c.ID)
		if accessibilityCommitmentError(w, e) {
			return
		}
		writeJSON(w, 201, c)
	})
}
func accessibilityCommitmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, accessibilitycommitments.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "accessibility_commitment_not_found"})
	case errors.Is(e, accessibilitycommitments.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_commitment"})
	case errors.Is(e, accessibilitycommitments.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "accessibility_commitment_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
