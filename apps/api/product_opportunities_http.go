package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
)

type opportunitySources struct {
	feedback    *productfeedback.Store
	issues      *issues.Store
	previews    *previews.Store
	experiments *productexperiments.Store
}

func registerProductOpportunitiesHTTP(mux *http.ServeMux, s *productopportunities.Store, repos proposalRepositoryStore, c authStore, src opportunitySources) {
	base := "/repositories/{repository}/product-opportunities"
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
	project := func(v productopportunities.Opportunity) productopportunities.Opportunity {
		if len(v.Versions) > 1 {
			v.Versions = v.Versions[len(v.Versions)-1:]
		}
		return v
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		vs, e := s.List(repo)
		if productOpportunityError(w, e) {
			return
		}
		for i := range vs {
			vs[i] = project(vs[i])
		}
		writeJSON(w, 200, map[string]any{"items": vs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productopportunities.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !validateOpportunitySources(repo, in.Sources, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_opportunity_source"})
			return
		}
		v, e := s.Create(repo, a, in)
		if productOpportunityError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET "+base+"/{opportunity}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("opportunity"))
		if productOpportunityError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{opportunity}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productopportunities.Input
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !validateOpportunitySources(repo, in.Sources, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_opportunity_source"})
			return
		}
		v, e := s.Revise(repo, r.PathValue("opportunity"), a, in.ExpectedVersion, in.Input)
		if productOpportunityError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{opportunity}/notes", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Kind       string `json:"kind"`
			SourceKind string `json:"source_kind"`
			ResourceID string `json:"resource_id"`
			Body       string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Note(repo, r.PathValue("opportunity"), a, in.Kind, in.SourceKind, in.ResourceID, in.Body)
		if productOpportunityError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{opportunity}/feedback/{feedback}/detach", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		f, e := src.feedback.Get(repo, r.PathValue("feedback"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if f.ReporterID != a {
			writeJSON(w, 403, map[string]string{"error": "feedback_reporter_required"})
			return
		}
		v, e := s.DetachFeedback(repo, r.PathValue("opportunity"), f.ID, a)
		if productOpportunityError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
}

func validateOpportunitySources(repo string, ss []productopportunities.Source, src opportunitySources) bool {
	for _, s := range ss {
		switch s.Kind {
		case "feedback":
			v, e := src.feedback.Get(repo, s.ResourceID)
			if e != nil || strconv.FormatInt(v.UpdatedAt.UnixNano(), 10) != s.CapturedRevision {
				return false
			}
		case "issue":
			v, e := src.issues.Get(repo, s.ResourceID)
			if e != nil || strconv.FormatInt(v.Version, 10) != s.CapturedRevision {
				return false
			}
		case "preview_finding":
			v, e := src.previews.GetByID(s.ResourceID)
			if e != nil || v.RepositoryID != repo {
				return false
			}
			found := false
			for _, f := range v.Findings {
				if f.ID == s.SubresourceID && f.Revision == s.CapturedRevision {
					found = true
				}
			}
			if !found {
				return false
			}
		case "experiment_outcome":
			v, e := src.experiments.Get(repo, s.ResourceID)
			if e != nil || len(v.Decisions) == 0 || strconv.FormatInt(v.Decisions[len(v.Decisions)-1].Version, 10) != s.CapturedRevision {
				return false
			}
		case "support_signal", "usage_evidence": // These are bounded external observations; the citation ID and revision remain explicit.
		default:
			return false
		}
	}
	return true
}
func productOpportunityError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, productopportunities.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, productopportunities.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "opportunity_version_conflict"})
	case errors.Is(e, productopportunities.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_product_opportunity"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
