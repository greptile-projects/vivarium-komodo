package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"net/http"
)

func registerProjectBoundariesHTTP(m *http.ServeMux, s *projectboundaries.Store, inc *projectincubators.Store, c authStore) {
	fail := func(w http.ResponseWriter, e error) bool {
		switch {
		case e == nil:
			return false
		case errors.Is(e, projectboundaries.ErrNotFound):
			writeJSON(w, 404, map[string]string{"error": "project_boundary_not_found"})
		case errors.Is(e, projectboundaries.ErrForbidden):
			writeJSON(w, 403, map[string]string{"error": "project_boundary_owner_required"})
		case errors.Is(e, projectboundaries.ErrConflict):
			writeJSON(w, 409, map[string]string{"error": "project_boundary_conflict"})
		case errors.Is(e, projectboundaries.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_project_boundary"})
		default:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		}
		return true
	}
	m.HandleFunc("GET /project-boundaries", func(w http.ResponseWriter, r *http.Request) {
		items, e := s.ListPublic()
		if !fail(w, e) {
			writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
		}
	})
	m.HandleFunc("GET /project-boundaries/{project}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Get(r.PathValue("project"), "", true)
		if !fail(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-boundaries", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in projectboundaries.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		source, e := inc.Get(in.IncubatorID, a.UserID)
		if e != nil || source.AcceptedAlternativeID == "" || source.AcceptedAlternativeID != in.AlternativeID {
			writeJSON(w, 422, map[string]string{"error": "accepted_incubator_direction_required"})
			return
		}
		authorized := source.CreatedByID == a.UserID
		for _, x := range source.SponsorIDs {
			authorized = authorized || x == a.UserID
		}
		if !authorized {
			writeJSON(w, 403, map[string]string{"error": "incubator_owner_required"})
			return
		}
		v, e := s.Create(a.UserID, in)
		if !fail(w, e) {
			w.Header().Set("Location", "/project-boundaries/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-boundaries/{project}/approvals", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Revision int64  `json:"revision"`
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		v, e := s.Decide(r.PathValue("project"), a.UserID, in.Decision, in.Reason, in.Revision)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-boundaries/{project}/activation", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Revision int64 `json:"revision"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		v, e := s.Activate(r.PathValue("project"), a.UserID, in.Revision)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-boundaries/{project}/rollback", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Revision int64  `json:"revision"`
			Reason   string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		v, e := s.Rollback(r.PathValue("project"), a.UserID, in.Reason, in.Revision)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
