package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectdeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectreadiness"
)

func registerProjectReadinessHTTP(m *http.ServeMux, s *projectreadiness.Store, inc *projectincubators.Store, bounds *projectboundaries.Store, deliveries *projectdeliveries.Store, c authStore) {
	fail := func(w http.ResponseWriter, e error) bool {
		if e == nil {
			return false
		}
		code, key := 500, "internal_error"
		switch {
		case errors.Is(e, projectreadiness.ErrNotFound):
			code, key = 404, "project_readiness_not_found"
		case errors.Is(e, projectreadiness.ErrForbidden):
			code, key = 403, "project_readiness_owner_required"
		case errors.Is(e, projectreadiness.ErrConflict):
			code, key = 409, "project_readiness_revision_conflict"
		case errors.Is(e, projectreadiness.ErrInvalid):
			code, key = 422, "invalid_project_readiness"
		}
		writeJSON(w, code, map[string]string{"error": key})
		return true
	}
	m.HandleFunc("GET /project-readiness", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		all, e := s.List()
		items := []projectreadiness.Readiness{}
		for _, v := range all {
			if _, visible := inc.Get(v.IncubatorID, a.UserID); visible == nil {
				items = append(items, v)
			}
		}
		if !fail(w, e) {
			writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
		}
	})
	m.HandleFunc("GET /project-readiness/{readiness}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("readiness"))
		if e == nil {
			_, e = inc.Get(v.IncubatorID, a.UserID)
		}
		if !fail(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-readiness", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in projectreadiness.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		i, e := inc.Get(in.IncubatorID, a.UserID)
		if e != nil || i.AcceptedAlternativeID != in.AlternativeID {
			writeJSON(w, 422, map[string]string{"error": "accepted_incubator_direction_required"})
			return
		}
		b, e := bounds.Get(in.BoundaryID, a.UserID, false)
		if e != nil || b.State != "active" || b.Revision != in.BoundaryRevision || b.IncubatorID != in.IncubatorID || b.AlternativeID != in.AlternativeID {
			writeJSON(w, 409, map[string]string{"error": "active_exact_project_boundary_required"})
			return
		}
		d, e := deliveries.Get(in.DeliveryID)
		if e != nil || d.Revision != in.DeliveryRevision || d.IncubatorID != in.IncubatorID || d.BoundaryID != in.BoundaryID || len(d.Blockers) > 0 {
			writeJSON(w, 409, map[string]string{"error": "proven_exact_project_delivery_required"})
			return
		}
		v, e := s.Create(a.UserID, in)
		if !fail(w, e) {
			w.Header().Set("Location", "/project-readiness/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	mutate := func(path string, evidence bool) {
		m.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
			if !ok {
				return
			}
			var body struct {
				Revision int64                     `json:"revision"`
				Evidence projectreadiness.Evidence `json:"evidence"`
				Decision projectreadiness.Decision `json:"decision"`
			}
			if !readJSON(w, r, &body, 1<<20) {
				return
			}
			var v projectreadiness.Readiness
			var e error
			if evidence {
				v, e = s.AddEvidence(r.PathValue("readiness"), a.UserID, body.Revision, body.Evidence)
			} else {
				v, e = s.Decide(r.PathValue("readiness"), a.UserID, body.Revision, body.Decision)
			}
			if !fail(w, e) {
				writeJSON(w, 201, v)
			}
		})
	}
	mutate("POST /project-readiness/{readiness}/evidence", true)
	mutate("POST /project-readiness/{readiness}/decisions", false)
}
