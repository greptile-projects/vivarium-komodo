package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAssuranceAssessmentsHTTP(mux *http.ServeMux, s *assuranceassessments.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/assurance-assessments"
	access := func(w http.ResponseWriter, r *http.Request, permission auth.Scope, require bool) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, permission, require)
		return string(repo.ID), a.UserID, ok
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(repo)
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assuranceassessments.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(repo, actor, in)
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{assessment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("assessment"))
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision  string                            `json:"expected_revision"`
			CandidateRevision string                            `json:"candidate_revision"`
			Inputs            []assuranceassessments.BoundInput `json:"inputs"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Rebind(repo, r.PathValue("assessment"), actor, in.ExpectedRevision, in.CandidateRevision, in.Inputs)
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/annotations", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in assuranceassessments.AnnotationInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, e := s.Annotate(repo, r.PathValue("assessment"), actor, in)
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ControlID string `json:"control_id"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := s.Decide(repo, r.PathValue("assessment"), actor, in.ControlID, in.Decision, in.Rationale)
		if !assuranceAssessmentError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}

func assuranceAssessmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, assuranceassessments.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "assurance_assessment_not_found"})
	case errors.Is(e, assuranceassessments.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "assurance_assessment_changed"})
	case errors.Is(e, assuranceassessments.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_assurance_assessment"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
