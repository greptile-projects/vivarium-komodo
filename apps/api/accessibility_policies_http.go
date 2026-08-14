package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
)

func registerAccessibilityPoliciesHTTP(mux *http.ServeMux, s *accessibilitypolicies.Store, repos proposalRepositoryStore, c authStore, commitments accessibilityCommitmentStore, previewStore *previews.Store, assessments *accessibilityassessments.Store, runs readinessCheckStore, pulls pullRequestStore) {
	mux.HandleFunc("POST /repositories/{repository}/accessibility-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in accessibilitypolicies.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		commitment, e := commitments.Get(string(repo.ID), in.CommitmentID)
		if e != nil || in.CommitmentVersion < 1 || in.CommitmentVersion > int64(len(commitment.Versions)) {
			writeJSON(w, 422, map[string]string{"error": "exact_accessibility_commitment_required"})
			return
		}
		p, e := s.Create(string(repo.ID), actor.UserID, in)
		if errors.Is(e, accessibilitypolicies.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_delivery_policy"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /repositories/{repository}/accessibility-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/accessibility-acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID   string `json:"policy_id"`
			PreviewID  string `json:"preview_id"`
			ScenarioID string `json:"scenario_id"`
			Role       string `json:"role"`
			Decision   string `json:"decision"`
			Rationale  string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 16000) {
			return
		}
		p, invite, e := previewStore.Authorize(string(repo.ID), r.PathValue("pull"), in.PreviewID, actor.UserID)
		if e != nil || invite.Role != in.Role {
			writeJSON(w, 404, map[string]string{"error": "preview_invitation_required"})
			return
		}
		a, e := s.Acknowledge(string(repo.ID), r.PathValue("pull"), in.PolicyID, in.PreviewID, p.Revision, in.ScenarioID, in.Role, in.Decision, in.Rationale, actor.UserID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_acknowledgement"})
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/accessibility-overrides", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			AcknowledgementID string                         `json:"acknowledgement_id"`
			Rationale         string                         `json:"rationale"`
			FollowUp          accessibilitypolicies.FollowUp `json:"follow_up"`
		}
		if !readJSON(w, r, &in, 16000) {
			return
		}
		o, e := s.Override(string(repo.ID), r.PathValue("pull"), in.AcknowledgementID, actor.UserID, in.Rationale, in.FollowUp)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "rejected_acknowledgement_and_follow_up_required"})
			return
		}
		writeJSON(w, 201, o)
	})
	mux.HandleFunc("POST /repositories/{repository}/releases/accessibility-readiness", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in struct {
			PullRequestID string   `json:"pull_request_id"`
			Revision      string   `json:"revision"`
			TargetBranch  string   `json:"target_branch"`
			Paths         []string `json:"paths"`
			Journeys      []string `json:"journeys"`
			RiskClasses   []string `json:"risk_classes"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		pull, e := pulls.Get(string(repo.ID), in.PullRequestID)
		if e != nil || pull.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_release_candidate_required"})
			return
		}
		aa, e := assessments.List(string(repo.ID), in.PullRequestID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		rr := []checkruns.Run{}
		if runs != nil {
			rr, e = runs.List(string(repo.ID), in.PullRequestID)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		a, e := s.Assess(string(repo.ID), in.PullRequestID, in.Revision, in.TargetBranch, in.Paths, in.Journeys, in.RiskClasses, accessibilitypolicies.Evidence{Assessments: aa, Runs: rr})
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, a)
	})
}
