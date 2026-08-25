package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningoutcomes"
	"net/http"
)

func registerLearningOutcomesHTTP(mux *http.ServeMux, s *learningoutcomes.Store, repos contributorPathwayRepositories, c authStore) {
	base := "/repositories/{repository}/learning-pathways/{pathway}/outcomes"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, bool) {
		_, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, write)
		return a.UserID, ok
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := access(w, r, false); !ok {
			return
		}
		v, e := s.Get(r.PathValue("repository"), r.PathValue("pathway"))
		if errors.Is(e, learningoutcomes.ErrNotFound) {
			v = learningoutcomes.Record{RepositoryID: r.PathValue("repository"), PathwayID: r.PathValue("pathway"), Observations: []learningoutcomes.Observation{}, Findings: []learningoutcomes.Finding{}, Improvements: []learningoutcomes.Improvement{}, Revalidations: []learningoutcomes.Revalidation{}}
		} else if outcomeError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/observations", func(w http.ResponseWriter, r *http.Request) {
		a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in learningoutcomes.Observation
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Observe(r.PathValue("repository"), r.PathValue("pathway"), a, in)
		if outcomeError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/findings", func(w http.ResponseWriter, r *http.Request) {
		a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ActorKind string `json:"actor_kind"`
			learningoutcomes.Finding
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Find(r.PathValue("repository"), r.PathValue("pathway"), a, in.ActorKind, in.Finding)
		if outcomeError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/improvements", func(w http.ResponseWriter, r *http.Request) {
		a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in learningoutcomes.Improvement
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Improve(r.PathValue("repository"), r.PathValue("pathway"), a, in)
		if outcomeError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/revalidations", func(w http.ResponseWriter, r *http.Request) {
		a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in learningoutcomes.Revalidation
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Revalidate(r.PathValue("repository"), r.PathValue("pathway"), a, in)
		if outcomeError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func outcomeError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, learningoutcomes.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_learning_outcome"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
