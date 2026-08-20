package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
)

func registerCapabilityRetirementsHTTP(mux *http.ServeMux, s *capabilityretirements.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/capability-retirements"
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
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityretirements.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(repo, actor, in)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("plan"))
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			AuthorKind        string   `json:"author_kind"`
			Kind              string   `json:"kind"`
			Body              string   `json:"body"`
			EvidenceReference string   `json:"evidence_reference"`
			AudienceIDs       []string `json:"audience_ids"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Assess(repo, r.PathValue("plan"), actor, in.AuthorKind, in.Kind, in.Body, in.EvidenceReference, in.AudienceIDs)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Scope     string `json:"scope"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.DecideApproval(repo, r.PathValue("plan"), actor, in.Scope, in.Decision, in.Rationale)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/policy-decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Kind      string    `json:"kind"`
			Subject   string    `json:"subject"`
			Decision  string    `json:"decision"`
			Rationale string    `json:"rationale"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.AddPolicyDecision(repo, r.PathValue("plan"), actor, in.Kind, in.Subject, in.Decision, in.Rationale, in.ExpiresAt)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func capabilityRetirementError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capabilityretirements.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capability_retirement_not_found"})
	case errors.Is(e, capabilityretirements.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capability_retirement_owner_required"})
	case errors.Is(e, capabilityretirements.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capability_retirement_conflict"})
	case errors.Is(e, capabilityretirements.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capability_retirement"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
