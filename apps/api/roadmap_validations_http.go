package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapvalidations"
)

func registerRoadmapValidationsHTTP(mux *http.ServeMux, s *roadmapvalidations.Store, roadmaps *productroadmaps.Store, feedback *productfeedback.Store, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/product-roadmaps/{roadmap}/validations"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		permission := auth.RepositoryRead
		if write {
			permission = auth.RepositoryWrite
		}
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, permission, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), actor.UserID, true
	}
	resolve := func(repo, roadmapID string, in roadmapvalidations.Input) (int64, string, int64, bool) {
		r, e := roadmaps.Get(repo, roadmapID)
		if e != nil {
			return 0, "", 0, false
		}
		for _, o := range r.Versions[len(r.Versions)-1].Outcomes {
			if o.ID == in.OutcomeID {
				return r.CurrentVersion, o.OpportunityID, o.OpportunityVersion, true
			}
		}
		return 0, "", 0, false
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.List(repo)
		if validationError(w, e) {
			return
		}
		out := []roadmapvalidations.Validation{}
		for _, x := range v {
			if x.RoadmapID == r.PathValue("roadmap") {
				out = append(out, x)
			}
		}
		writeJSON(w, 200, map[string]any{"items": publicValidations(out)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in roadmapvalidations.Input
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		rv, opp, ov, yes := resolve(repo, r.PathValue("roadmap"), in)
		if !yes {
			writeJSON(w, 422, map[string]string{"error": "invalid_roadmap_outcome"})
			return
		}
		v, e := s.Create(repo, r.PathValue("roadmap"), rv, opp, ov, a, in)
		if validationError(w, e) {
			return
		}
		writeJSON(w, 201, publicValidation(v))
	})
	mux.HandleFunc("GET "+base+"/{validation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("validation"))
		if validationError(w, e) {
			return
		}
		writeJSON(w, 200, publicValidation(v))
	})
	mux.HandleFunc("POST "+base+"/{validation}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			roadmapvalidations.Input
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Revise(repo, r.PathValue("validation"), a, in.ExpectedVersion, in.Input)
		if validationError(w, e) {
			return
		}
		writeJSON(w, 201, publicValidation(v))
	})
	mux.HandleFunc("POST "+base+"/{validation}/invitations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ParticipantID      string `json:"participant_id"`
			FeedbackID         string `json:"feedback_id"`
			AccessibilityNeeds string `json:"accessibility_needs"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		f, e := feedback.Get(repo, in.FeedbackID)
		if e != nil || f.ReporterID != in.ParticipantID || !f.Consent.Research || f.Consent.WithdrawnAt != nil {
			writeJSON(w, 422, map[string]string{"error": "participant_research_consent_required"})
			return
		}
		v, token, e := s.Invite(repo, r.PathValue("validation"), a, in.ParticipantID, in.FeedbackID, in.AccessibilityNeeds)
		if validationError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"validation": publicValidation(v), "participant_credential": map[string]any{"token": token, "repository_access": false, "activity_only": true}})
	})
	mux.HandleFunc("POST "+base+"/{validation}/assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in roadmapvalidations.Assessment
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Assess(repo, r.PathValue("validation"), a, in)
		if validationError(w, e) {
			return
		}
		writeJSON(w, 201, publicValidation(v))
	})
	mux.HandleFunc("GET /roadmap-validation-participant/context", func(w http.ResponseWriter, r *http.Request) {
		v, i, e := s.Participant(participantToken(r))
		if validationError(w, e) {
			return
		}
		i.TokenDigest = ""
		writeJSON(w, 200, map[string]any{"validation": publicValidation(v), "invitation": i, "authority": map[string]bool{"repository": false, "activity": true}})
	})
	mux.HandleFunc("POST /roadmap-validation-participant/findings", func(w http.ResponseWriter, r *http.Request) {
		var in roadmapvalidations.Finding
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Find(participantToken(r), in)
		if validationError(w, e) {
			return
		}
		writeJSON(w, 201, publicValidation(v))
	})
}

func publicValidation(v roadmapvalidations.Validation) roadmapvalidations.Validation {
	for i := range v.Invitations {
		v.Invitations[i].TokenDigest = ""
	}
	return v
}
func publicValidations(v []roadmapvalidations.Validation) []roadmapvalidations.Validation {
	for i := range v {
		v[i] = publicValidation(v[i])
	}
	return v
}
func participantToken(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func validationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, roadmapvalidations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "roadmap_validation_not_found"})
	case errors.Is(e, roadmapvalidations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "roadmap_validation_version_conflict"})
	case errors.Is(e, roadmapvalidations.ErrUnauthorized):
		writeJSON(w, 401, map[string]string{"error": "participant_credential_invalid"})
	case errors.Is(e, roadmapvalidations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_roadmap_validation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
