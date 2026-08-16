package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
	"net/http"
)

func registerAgentProfilesHTTP(m *http.ServeMux, s *agentprofiles.Store, c authStore, u *users.Store) {
	m.HandleFunc("GET /agent-profiles", func(w http.ResponseWriter, r *http.Request) {
		x, e := s.List()
		if !agentProfileError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	m.HandleFunc("POST /agent-profiles", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Handle string `json:"handle"`
			agentprofiles.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(a.UserID, in.Handle, in.Input, func(h string) bool { _, e := u.FindByHandle(h); return errors.Is(e, users.ErrNotFound) })
		if !agentProfileError(w, e) {
			w.Header().Set("Location", "/agent-profiles/"+x.ID)
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET /agent-profiles/{profile}", func(w http.ResponseWriter, r *http.Request) {
		x, e := s.Get(r.PathValue("profile"))
		if !agentProfileError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST /agent-profiles/{profile}/versions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			agentprofiles.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(r.PathValue("profile"), a.UserID, in.ExpectedVersion, in.Input)
		if !agentProfileError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func agentProfileError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentprofiles.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_profile_not_found"})
	case errors.Is(e, agentprofiles.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_profile"})
	case errors.Is(e, agentprofiles.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "agent_profile_changed"})
	case errors.Is(e, agentprofiles.ErrIdentityTaken):
		writeJSON(w, 409, map[string]string{"error": "identity_taken"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
