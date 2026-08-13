package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
)

func registerProductRoadmapsHTTP(mux *http.ServeMux, s *productroadmaps.Store, opps *productopportunities.Store, repos proposalRepositoryStore, c authStore) {
	base := "/repositories/{repository}/product-roadmaps"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		p := auth.RepositoryRead
		if write {
			p = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, p, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	validate := func(repo string, in productroadmaps.Input) bool {
		for _, o := range in.Outcomes {
			v, e := opps.Get(repo, o.OpportunityID)
			if e != nil || v.CurrentVersion != o.OpportunityVersion {
				return false
			}
		}
		return true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.List(repo)
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productroadmaps.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !validate(repo, in) {
			writeJSON(w, 422, map[string]string{"error": "opportunity_version_not_current"})
			return
		}
		v, e := s.Create(repo, a, in)
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET "+base+"/{roadmap}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("roadmap"))
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{roadmap}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productroadmaps.Input
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !validate(repo, in.Input) {
			writeJSON(w, 422, map[string]string{"error": "opportunity_version_not_current"})
			return
		}
		v, e := s.Replan(repo, r.PathValue("roadmap"), a, in.ExpectedVersion, in.Input)
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{roadmap}/scenarios", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			BaseVersion int64                     `json:"base_version"`
			Summary     string                    `json:"summary"`
			AuthorKind  string                    `json:"author_kind"`
			Outcomes    []productroadmaps.Outcome `json:"outcomes"`
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		v, e := s.Scenario(repo, r.PathValue("roadmap"), a, in.AuthorKind, in.BaseVersion, in.Summary, in.Outcomes)
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{roadmap}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Version int64  `json:"version"`
			Body    string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Comment(repo, r.PathValue("roadmap"), a, in.Body, in.Version)
		if roadmapError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func roadmapError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, productroadmaps.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, productroadmaps.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "roadmap_version_conflict"})
	case errors.Is(e, productroadmaps.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_product_roadmap"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
