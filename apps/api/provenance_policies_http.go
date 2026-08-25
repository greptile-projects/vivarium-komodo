package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancepolicies"
)

func registerProvenancePoliciesHTTP(mux *http.ServeMux, s *provenancepolicies.Store, repos dataFlowRepositories, orgs *organizations.Store, credentials authStore) {
	registerRepositoryProvenancePolicies(mux, s, repos, credentials)
	base := "/organizations/{organization}/provenance-policies"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		scope := r.PathValue("organization")
		if !orgs.IsMember(scope, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		x, e := s.Catalog("organization", scope)
		if !provenancePolicyError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		scope := r.PathValue("organization")
		if !orgs.IsOwner(scope, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "organization_owner_required"})
			return
		}
		var in provenancepolicies.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create("organization", scope, a.UserID, in)
		if !provenancePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{policy}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		scope := r.PathValue("organization")
		if !orgs.IsMember(scope, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		x, e := s.Get("organization", scope, r.PathValue("policy"))
		if !provenancePolicyError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{policy}/versions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		scope := r.PathValue("organization")
		if !orgs.IsOwner(scope, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "organization_owner_required"})
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			provenancepolicies.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise("organization", scope, r.PathValue("policy"), a.UserID, in.ExpectedVersion, in.Input)
		if !provenancePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func registerRepositoryProvenancePolicies(mux *http.ServeMux, s *provenancepolicies.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/provenance-policies"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog("repository", string(repo.ID))
		if !provenancePolicyError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in provenancepolicies.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create("repository", string(repo.ID), a.UserID, in)
		if !provenancePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{policy}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get("repository", string(repo.ID), r.PathValue("policy"))
		if !provenancePolicyError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{policy}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			provenancepolicies.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise("repository", string(repo.ID), r.PathValue("policy"), a.UserID, in.ExpectedVersion, in.Input)
		if !provenancePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func provenancePolicyError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, provenancepolicies.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "provenance_policy_not_found"})
	case errors.Is(e, provenancepolicies.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "provenance_policy_changed_or_conflicting"})
	case errors.Is(e, provenancepolicies.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_provenance_policy"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
