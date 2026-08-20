package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityremovals"
)

func registerCapabilityRemovalsHTTP(mux *http.ServeMux, s *capabilityremovals.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/capability-removals"
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
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityremovals.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(repo, actor, in)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{removal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("removal"))
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{removal}/evidence", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			capabilityremovals.DeliveryEvidence
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddEvidence(repo, r.PathValue("removal"), actor, in.ExpectedRevision, in.DeliveryEvidence)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{removal}/signals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			capabilityremovals.SignalInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddSignal(repo, r.PathValue("removal"), actor, in.ExpectedRevision, in.SignalInput)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{removal}/consumers", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision  int64  `json:"expected_revision"`
			ConsumerID        string `json:"consumer_id"`
			EvidenceReference string `json:"evidence_reference"`
			Summary           string `json:"summary"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.DiscoverConsumer(repo, r.PathValue("removal"), actor, in.ExpectedRevision, in.ConsumerID, in.EvidenceReference, in.Summary)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{removal}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64  `json:"expected_revision"`
			Action           string `json:"action"`
			Rationale        string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Control(repo, r.PathValue("removal"), actor, in.ExpectedRevision, in.Action, in.Rationale)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{removal}/complete", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			capabilityremovals.CompletionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Complete(repo, r.PathValue("removal"), actor, in.ExpectedRevision, in.CompletionInput)
		if !capabilityRemovalError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func capabilityRemovalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capabilityremovals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capability_removal_not_found"})
	case errors.Is(e, capabilityremovals.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capability_removal_owner_required"})
	case errors.Is(e, capabilityremovals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capability_removal_conflict"})
	case errors.Is(e, capabilityremovals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capability_removal"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
