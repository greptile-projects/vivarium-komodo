package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectdeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"net/http"
)

func registerProjectDeliveriesHTTP(m *http.ServeMux, s *projectdeliveries.Store, inc *projectincubators.Store, bounds *projectboundaries.Store, c authStore) {
	fail := func(w http.ResponseWriter, e error) bool {
		if e == nil {
			return false
		}
		code, key := 500, "internal_error"
		switch {
		case errors.Is(e, projectdeliveries.ErrNotFound):
			code, key = 404, "project_delivery_not_found"
		case errors.Is(e, projectdeliveries.ErrForbidden):
			code, key = 403, "project_delivery_participant_required"
		case errors.Is(e, projectdeliveries.ErrConflict):
			code, key = 409, "project_delivery_revision_conflict"
		case errors.Is(e, projectdeliveries.ErrInvalid):
			code, key = 422, "invalid_project_delivery"
		}
		writeJSON(w, code, map[string]string{"error": key})
		return true
	}
	authn := func(w http.ResponseWriter, r *http.Request) (auth.Grant, bool) {
		return authenticateRequest(w, r, c, auth.RepositoryWrite)
	}
	m.HandleFunc("GET /project-deliveries", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		all, e := s.List()
		items := []projectdeliveries.Delivery{}
		for _, v := range all {
			if _, visible := inc.Get(v.IncubatorID, a.UserID); visible == nil {
				items = append(items, v)
			}
		}
		if !fail(w, e) {
			writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
		}
	})
	m.HandleFunc("GET /project-deliveries/{delivery}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("delivery"))
		if e == nil {
			_, e = inc.Get(v.IncubatorID, a.UserID)
		}
		if !fail(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-deliveries", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in projectdeliveries.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		i, e := inc.Get(in.IncubatorID, a.UserID)
		if e != nil || i.AcceptedAlternativeID != in.AlternativeID {
			writeJSON(w, 422, map[string]string{"error": "accepted_incubator_direction_required"})
			return
		}
		b, e := bounds.Get(in.BoundaryID, a.UserID, false)
		if e != nil || b.State != "active" || b.IncubatorID != in.IncubatorID || b.AlternativeID != in.AlternativeID || b.Revision != in.BoundaryRevision {
			writeJSON(w, 409, map[string]string{"error": "active_exact_project_boundary_required"})
			return
		}
		v, e := s.Create(a.UserID, in)
		if !fail(w, e) {
			w.Header().Set("Location", "/project-deliveries/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/workspaces", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in projectdeliveries.Workspace
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Workspace(r.PathValue("delivery"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/pull-requests", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in projectdeliveries.PullRequest
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Pull(r.PathValue("delivery"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/pull-requests/{pull}/checks", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in struct {
			Revision string `json:"revision"`
			Outcome  string `json:"outcome"`
			Name     string `json:"name"`
		}
		if !readJSON(w, r, &in, 32768) {
			return
		}
		v, e := s.CheckReview(r.PathValue("delivery"), r.PathValue("pull"), a.UserID, "check", in.Revision, in.Outcome, in.Name)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/pull-requests/{pull}/reviews", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in struct {
			Revision string `json:"revision"`
			Decision string `json:"decision"`
			Body     string `json:"body"`
		}
		if !readJSON(w, r, &in, 32768) {
			return
		}
		v, e := s.CheckReview(r.PathValue("delivery"), r.PathValue("pull"), a.UserID, "review", in.Revision, in.Decision, in.Body)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/previews", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in projectdeliveries.Preview
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Preview(r.PathValue("delivery"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/previews/{preview}/evidence", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in struct {
			Revision    string `json:"revision"`
			Outcome     string `json:"outcome"`
			Observation string `json:"observation"`
			Artifact    string `json:"artifact"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Evidence(r.PathValue("delivery"), r.PathValue("preview"), a.UserID, in.Outcome, in.Observation, in.Artifact, in.Revision)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-deliveries/{delivery}/activity", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r)
		if !ok {
			return
		}
		var in projectdeliveries.Activity
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.ActivityEvent(r.PathValue("delivery"), a.UserID, in)
		if !fail(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
