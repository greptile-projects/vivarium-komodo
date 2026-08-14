package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationdelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
)

type localizationDeliverySources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	verification interface {
		Get(string, string) (localizationverification.Assessment, error)
	}
	releases interface {
		Get(string, string) (releases.Release, error)
	}
	docs interface {
		Get(string, string) (docscollections.Collection, error)
	}
	proposals interface {
		Get(string, string) (proposals.Proposal, error)
		GetPlan(string, string) (proposals.Plan, error)
	}
}

func registerLocalizationDeliveryHTTP(mux *http.ServeMux, s *localizationdelivery.Store, repos proposalRepositoryStore, c authStore, src localizationDeliverySources) {
	access := func(w http.ResponseWriter, r *http.Request, scope auth.Scope) (string, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, scope, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), actor.UserID, true
	}
	mux.HandleFunc("POST /repositories/{repository}/localization-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in localizationdelivery.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.CreatePolicy(repo, actor, in)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET /repositories/{repository}/localization-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Policies(repo)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("PUT /repositories/{repository}/pull-requests/{pull}/locale-publication", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			Revision        string                                 `json:"revision"`
			ExpectedVersion int64                                  `json:"expected_version"`
			Locales         []localizationdelivery.CandidateLocale `json:"locales"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_locale_candidate_required"})
			return
		}
		v, e := s.SetCandidate(repo, p.ID, in.Revision, actor, in.ExpectedVersion, in.Locales)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull}/localization-readiness", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		var in struct {
			Revision     string   `json:"revision"`
			TargetBranch string   `json:"target_branch"`
			Paths        []string `json:"paths"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_locale_candidate_required"})
			return
		}
		var evidence *localizationverification.Assessment
		if a, er := src.verification.Get(repo, p.ID); er == nil {
			evidence = &a
		}
		v, e := s.Assess(repo, p.ID, in.Revision, in.TargetBranch, in.Paths, evidence)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("GET /repositories/{repository}/locale-publications", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Publications(repo)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("POST /repositories/{repository}/locale-publications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in localizationdelivery.PublicationInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		candidate, e := s.Candidate(repo, in.CandidatePullRequestID)
		pull, pe := src.pulls.Get(repo, in.CandidatePullRequestID)
		candidatePublishedRevision := pe == nil && pull.SourceCommitID == candidate.Revision && (pull.MergeCommitID == in.Revision || candidate.Revision == in.Revision)
		if e != nil || candidate.Version != in.CandidateVersion || !candidatePublishedRevision {
			writeJSON(w, 422, map[string]string{"error": "current_locale_candidate_required"})
			return
		}
		state := ""
		for _, l := range candidate.Locales {
			if l.LocaleID == in.LocaleID {
				state = l.State
				if in.FallbackLocale == "" {
					in.FallbackLocale = l.FallbackLocale
				}
			}
		}
		if in.State == "published" && state != "staged" || in.State == "withdrawn" && state != "withdrawn" {
			writeJSON(w, 422, map[string]string{"error": "locale_publication_state_mismatch"})
			return
		}
		valid := false
		if in.Kind == "application" {
			x, er := src.releases.Get(repo, in.ResourceID)
			valid = er == nil && x.CommitID == in.Revision && x.Version == in.Version
		} else if in.Kind == "documentation" {
			x, er := src.docs.Get(repo, in.ResourceID)
			if er == nil {
				for _, h := range x.History {
					for _, m := range h.Versions {
						valid = valid || m.SourceRevision == in.Revision
					}
				}
			}
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "exact_published_resource_required"})
			return
		}
		v, e := s.Publish(repo, actor, in)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET /repositories/{repository}/locale-findings", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Findings(repo)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("POST /repositories/{repository}/locale-findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead)
		if !ok {
			return
		}
		var in localizationdelivery.FindingInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Report(repo, actor, in)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST /repositories/{repository}/locale-findings/{finding}/validation", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			State     string `json:"state"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Validate(repo, r.PathValue("finding"), actor, in.State, in.Rationale)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST /repositories/{repository}/locale-findings/{finding}/repair", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in localizationdelivery.Repair
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		_, e := src.proposals.Get(repo, in.ProposalID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "ordinary_repair_proposal_required"})
			return
		}
		plan, e := src.proposals.GetPlan(repo, in.ProposalID)
		found := false
		if e == nil {
			for _, t := range plan.Tasks {
				found = found || t.ID == in.TaskID
			}
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "ordinary_repair_task_required"})
			return
		}
		v, e := s.LinkRepair(repo, r.PathValue("finding"), actor, in)
		if localizationDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}

func localizationDeliveryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, localizationdelivery.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "localization_delivery_not_found"})
	case errors.Is(e, localizationdelivery.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "localization_delivery_conflict"})
	case errors.Is(e, localizationdelivery.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "localization_delivery_invalid"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
