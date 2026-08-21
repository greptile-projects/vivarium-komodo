package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectlife"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectreadiness"
)

func registerProjectLifeHTTP(m *http.ServeMux, s *projectlife.Store, ready *projectreadiness.Store, c authStore) {
	fail := func(w http.ResponseWriter, e error) bool {
		if e == nil {
			return false
		}
		code, key := 500, "internal_error"
		switch {
		case errors.Is(e, projectlife.ErrNotFound):
			code, key = 404, "project_life_not_found"
		case errors.Is(e, projectlife.ErrForbidden):
			code, key = 403, "project_life_owner_required"
		case errors.Is(e, projectlife.ErrConflict):
			code, key = 409, "project_life_revision_conflict"
		case errors.Is(e, projectlife.ErrInvalid):
			code, key = 422, "invalid_project_life"
		}
		writeJSON(w, code, map[string]string{"error": key})
		return true
	}
	m.HandleFunc("GET /project-life", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, c, auth.RepositoryRead); !ok {
			return
		}
		v, e := s.List()
		if !fail(w, e) {
			writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
		}
	})
	m.HandleFunc("GET /project-life/{project}", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticateRequest(w, r, c, auth.RepositoryRead); !ok {
			return
		}
		v, e := s.Get(r.PathValue("project"))
		if !fail(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-life", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in projectlife.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		rr, e := ready.Get(in.ReadinessID)
		if e != nil || !rr.Ready || rr.Revision != in.ReadinessRevision || rr.IncubatorID != in.IncubatorID || rr.AlternativeID != in.AlternativeID || rr.BoundaryID != in.BoundaryID || rr.BoundaryRevision != in.BoundaryRevision || rr.DeliveryID != in.DeliveryID || rr.DeliveryRevision != in.DeliveryRevision || rr.LaunchRevision != in.LaunchRevision || rr.EffectiveScope != in.Audience {
			writeJSON(w, 409, map[string]string{"error": "current_ready_exact_launch_required"})
			return
		}
		v, e := s.Create(a.UserID, in)
		if !fail(w, e) {
			w.Header().Set("Location", "/project-life/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	type body struct {
		Revision    int64                     `json:"revision"`
		Publication projectlife.Publication   `json:"publication"`
		Signal      projectlife.Signal        `json:"signal"`
		Feedback    projectlife.Feedback      `json:"feedback"`
		Work        projectlife.Work          `json:"work"`
		Roadmap     projectlife.RoadmapChange `json:"roadmap"`
		Disposition projectlife.Disposition   `json:"disposition"`
	}
	mutate := func(path string, fn func(string, string, body) (projectlife.Record, error)) {
		m.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			a, ok := authenticateRequest(w, r, c, auth.RepositoryWrite)
			if !ok {
				return
			}
			var b body
			if !readJSON(w, r, &b, 1<<20) {
				return
			}
			v, e := fn(r.PathValue("project"), a.UserID, b)
			if !fail(w, e) {
				writeJSON(w, 201, v)
			}
		})
	}
	mutate("POST /project-life/{project}/publications", func(id, a string, b body) (projectlife.Record, error) {
		return s.Publish(id, a, b.Revision, b.Publication)
	})
	mutate("POST /project-life/{project}/signals", func(id, a string, b body) (projectlife.Record, error) { return s.Observe(id, a, b.Revision, b.Signal) })
	mutate("POST /project-life/{project}/feedback", func(id, a string, b body) (projectlife.Record, error) {
		return s.AddFeedback(id, a, b.Revision, b.Feedback)
	})
	mutate("POST /project-life/{project}/work", func(id, a string, b body) (projectlife.Record, error) { return s.AddWork(id, a, b.Revision, b.Work) })
	mutate("POST /project-life/{project}/roadmap", func(id, a string, b body) (projectlife.Record, error) {
		return s.ReviseRoadmap(id, a, b.Revision, b.Roadmap)
	})
	mutate("POST /project-life/{project}/disposition", func(id, a string, b body) (projectlife.Record, error) {
		return s.Decide(id, a, b.Revision, b.Disposition)
	})
}
