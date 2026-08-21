package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/adoptionworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAdoptionWorkspacesHTTP(m *http.ServeMux, s *adoptionworkspaces.Store, c authStore) {
	fail := func(w http.ResponseWriter, e error) bool {
		if e == nil {
			return false
		}
		code, key := 500, "internal_error"
		switch {
		case errors.Is(e, adoptionworkspaces.ErrNotFound):
			code, key = 404, "adoption_workspace_not_found"
		case errors.Is(e, adoptionworkspaces.ErrForbidden):
			code, key = 403, "adoption_workspace_access_denied"
		case errors.Is(e, adoptionworkspaces.ErrInvalid):
			code, key = 422, "invalid_adoption_workspace"
		}
		writeJSON(w, code, map[string]string{"error": key})
		return true
	}
	m.HandleFunc("GET /adoption-workspaces", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.List(a.UserID)
		if !fail(w, e) {
			writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
		}
	})
	m.HandleFunc("POST /adoption-workspaces", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in adoptionworkspaces.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(a.UserID, in)
		if !fail(w, e) {
			w.Header().Set("Location", "/adoption-workspaces/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("GET /adoption-workspaces/{workspace}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("workspace"), a.UserID)
		if !fail(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /adoption-workspaces/{workspace}/participants", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in adoptionworkspaces.Participant
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Invite(r.PathValue("workspace"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /adoption-workspaces/{workspace}/participants/{participant}/consent", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Consent(r.PathValue("workspace"), r.PathValue("participant"), a.UserID, in.Decision)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /adoption-workspaces/{workspace}/candidates", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in adoptionworkspaces.Candidate
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddCandidate(r.PathValue("workspace"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /adoption-workspaces/{workspace}/candidates/{candidate}/evidence", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in adoptionworkspaces.Evidence
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddEvidence(r.PathValue("workspace"), r.PathValue("candidate"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
