package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
)

func registerReliabilityPoliciesHTTP(mux *http.ServeMux, s *reliabilitypolicies.Store, objectives *serviceobjectives.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/reliability-delivery-policies"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in reliabilitypolicies.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		o, e := objectives.Get(string(repo.ID), in.ObjectiveID)
		if e != nil || in.ObjectiveVersion < 1 || in.ObjectiveVersion > o.CurrentVersion {
			writeJSON(w, 422, map[string]string{"error": "exact_service_objective_required"})
			return
		}
		p, e := s.Create(string(repo.ID), a.UserID, in)
		if errors.Is(e, reliabilitypolicies.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_reliability_delivery_policy"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs, "total_count": len(xs)})
	})
	mux.HandleFunc("POST "+base+"/{policy}/impacts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in reliabilitypolicies.ImpactInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := s.RecordImpact(string(repo.ID), r.PathValue("policy"), a.UserID, in)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_reliability_impact"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{policy}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Context   reliabilitypolicies.Context `json:"context"`
			Decision  string                      `json:"decision"`
			Rationale string                      `json:"rationale"`
		}
		if !readJSON(w, r, &in, 32000) {
			return
		}
		x, e := s.Acknowledge(string(repo.ID), r.PathValue("policy"), a.UserID, in.Decision, in.Rationale, in.Context)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "objective_owner_acknowledgement_required"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/assessment", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in struct {
			Context          reliabilitypolicies.Context `json:"context"`
			ActiveExceptions []string                    `json:"active_exceptions"`
		}
		if !readJSON(w, r, &in, 64000) {
			return
		}
		x, e := s.Assess(string(repo.ID), in.Context, in.ActiveExceptions)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_delivery_context"})
			return
		}
		writeJSON(w, 200, x)
	})
}
