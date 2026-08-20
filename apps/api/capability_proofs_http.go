package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityproofs"
)

func registerCapabilityProofsHTTP(mux *http.ServeMux, s *capabilityproofs.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/capability-proofs"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, scope, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.List(repo)
		if !capabilityProofError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityproofs.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(repo, actor, in)
		if !capabilityProofError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{candidate}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("candidate"))
		if !capabilityProofError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{candidate}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityproofs.AttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddAttempt(repo, r.PathValue("candidate"), actor, in)
		if !capabilityProofError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{candidate}/usage", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in capabilityproofs.UsageInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddUsage(repo, r.PathValue("candidate"), actor, in)
		if !capabilityProofError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{candidate}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Acknowledge(repo, r.PathValue("candidate"), actor, in.Decision, in.Rationale)
		if !capabilityProofError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func capabilityProofError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capabilityproofs.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capability_proof_not_found"})
	case errors.Is(e, capabilityproofs.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capability_proof_owner_required"})
	case errors.Is(e, capabilityproofs.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capability_proof"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
