package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualitygates"
	"net/http"
)

func registerQualityGatesHTTP(mux *http.ServeMux, s *qualitygates.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/quality-gates"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		permission := auth.RepositoryRead
		if write {
			permission = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, permission, write)
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
		x, e := s.Catalog(repo)
		if !qualityGateError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/policies", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.PolicyInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreatePolicy(repo, a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/policies/{policy}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			qualitygates.PolicyInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RevisePolicy(repo, r.PathValue("policy"), a, in.ExpectedVersion, in.PolicyInput)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.OpenInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Open(repo, a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/candidates/{gate}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("gate"))
		if !qualityGateError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates/{gate}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.AttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddAttempt(repo, r.PathValue("gate"), a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates/{gate}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			RequirementID string `json:"requirement_id"`
			Decision      string `json:"decision"`
			Rationale     string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Acknowledge(repo, r.PathValue("gate"), a, in.RequirementID, in.Decision, in.Rationale)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates/{gate}/overrides", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.OverrideInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Override(repo, r.PathValue("gate"), a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates/{gate}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.RevisionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(repo, r.PathValue("gate"), a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/candidates/{gate}/post-release-signals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in qualitygates.SignalInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Signal(repo, r.PathValue("gate"), a, in)
		if !qualityGateError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func qualityGateError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, qualitygates.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "quality_gate_not_found"})
	case errors.Is(e, qualitygates.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "quality_gate_revision_conflict"})
	case errors.Is(e, qualitygates.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_quality_gate_evidence"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
