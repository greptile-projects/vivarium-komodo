package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
)

func registerResponseAlertsHTTP(mux *http.ServeMux, s *responsealerts.Store, policies *responsepolicies.Store, rotations *responserotations.Store, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/response-alerts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.URL.Query().Get("recipient"))
		if !responseAlertError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID string `json:"policy_id"`
			responsealerts.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		p, e := policies.Get(string(repo.ID), in.PolicyID)
		if responseAlertDependencyError(w, e) {
			return
		}
		rs, e := rotations.List(string(repo.ID))
		if responseAlertDependencyError(w, e) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in.Input, p, rs)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{alert}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("alert"))
		if !responseAlertError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/routing-attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.AttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RecordAttempt(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func responseAlertDependencyError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, responsepolicies.ErrNotFound) {
		writeJSON(w, 422, map[string]string{"error": "active_response_policy_not_found"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
func responseAlertError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, responsealerts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "response_alert_not_found"})
	case errors.Is(e, responsealerts.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_response_alert"})
	case errors.Is(e, responsealerts.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "response_alert_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
