package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"net/http"
)

func registerResponsePoliciesHTTP(mux *http.ServeMux, s *responsepolicies.Store, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/response-policies"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if !responsePolicyError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsepolicies.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !responsePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{policy}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("policy"))
		if !responsePolicyError(w, e) {
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
			responsepolicies.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("policy"), a.UserID, in.ExpectedVersion, in.Input)
		if !responsePolicyError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func responsePolicyError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, responsepolicies.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "response_policy_not_found"})
	case errors.Is(e, responsepolicies.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_response_policy"})
	case errors.Is(e, responsepolicies.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "response_policy_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
