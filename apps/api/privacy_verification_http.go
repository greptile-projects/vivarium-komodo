package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyverification"
)

func registerPrivacyVerificationHTTP(mux *http.ServeMux, s *privacyverification.Store, repos proposalRepositoryStore, c authStore, commitments interface {
	Get(string, string) (datacommitments.Commitment, error)
}, previewStore *previews.Store, runs readinessCheckStore, pulls pullRequestStore) {
	mux.HandleFunc("POST /repositories/{repository}/privacy-verification-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in privacyverification.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		rec, e := commitments.Get(string(repo.ID), in.CommitmentID)
		if e != nil || in.CommitmentVersion < 1 || in.CommitmentVersion > int64(len(rec.Versions)) {
			writeJSON(w, 422, map[string]string{"error": "exact_data_commitment_required"})
			return
		}
		p, e := s.Create(string(repo.ID), actor.UserID, in)
		if errors.Is(e, privacyverification.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_privacy_verification_policy"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /repositories/{repository}/privacy-verification-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		p, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": p, "total_count": len(p)})
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/privacy-verification-acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID  string `json:"policy_id"`
			PreviewID string `json:"preview_id"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := previewStore.Get(string(repo.ID), r.PathValue("pull"), in.PreviewID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "exact_preview_required"})
			return
		}
		a, e := s.Acknowledge(string(repo.ID), r.PathValue("pull"), in.PolicyID, in.PreviewID, p.Revision, in.Decision, in.Rationale, actor.UserID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "privacy_owner_acknowledgement_required"})
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/privacy-verification-exceptions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		var in struct {
			PolicyID   string                       `json:"policy_id"`
			CheckNames []string                     `json:"check_names"`
			Dimensions []string                     `json:"dimensions"`
			Reason     string                       `json:"reason"`
			FollowUp   privacyverification.FollowUp `json:"follow_up"`
			ExpiresAt  time.Time                    `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Except(string(repo.ID), pull.ID, in.PolicyID, pull.SourceCommitID, in.Reason, actor.UserID, in.CheckNames, in.Dimensions, in.FollowUp, in.ExpiresAt)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_privacy_verification_exception"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/releases/privacy-readiness", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in struct {
			PullRequestID string   `json:"pull_request_id"`
			Revision      string   `json:"revision"`
			TargetBranch  string   `json:"target_branch"`
			Paths         []string `json:"paths"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		pull, e := pulls.Get(string(repo.ID), in.PullRequestID)
		if e != nil || pull.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_release_candidate_required"})
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
		a, e := s.Assess(string(repo.ID), in.PullRequestID, in.Revision, in.TargetBranch, in.Paths, rr)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, a)
	})
}
