package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/propagationcampaigns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerPropagationCampaignsHTTP(mux *http.ServeMux, store *propagationcampaigns.Store, repos codeIntelligenceStore, credentials authStore) {
	base := "/repositories/{repository}/propagation-campaigns"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in propagationcampaigns.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		// A campaign may request work elsewhere, but its proof must be an exact,
		// locally readable source revision. The campaign itself grants no access.
		opened, err := repos.Open(repo.ID)
		if err != nil || in.Source.RepositoryID != string(repo.ID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_provenance"})
			return
		}
		for _, commit := range append(append([]string{}, in.Source.CommitIDs...), in.Source.Revision) {
			if _, err = opened.ReadCommit(storage.ObjectID(commit)); err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_source_provenance"})
				return
			}
		}
		x, err := store.Create(string(repo.ID), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, err := store.List(string(repo.ID))
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x, "total_count": len(x)})
	})
	mux.HandleFunc("GET "+base+"/{campaign}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, err := store.Get(string(repo.ID), r.PathValue("campaign"))
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in propagationcampaigns.AssessmentInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, err := store.Assess(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/assessments/{assessment}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in propagationcampaigns.FindingInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, err := store.AddFinding(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), r.PathValue("assessment"), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/assessments/{assessment}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, err := store.Acknowledge(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), r.PathValue("assessment"), actor.UserID, in.Decision, in.Rationale)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/contributions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in propagationcampaigns.ContributionInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, err := store.CreateContribution(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/equivalence-specifications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in propagationcampaigns.EquivalenceSpecificationInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, err := store.DefineEquivalence(string(repo.ID), r.PathValue("campaign"), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/equivalence-attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in propagationcampaigns.EquivalenceAttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, err := store.RecordEquivalenceAttempt(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), actor.UserID, in)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/targets/{target}/equivalence-attempts/{attempt}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, err := store.DecideEquivalence(string(repo.ID), r.PathValue("campaign"), r.PathValue("target"), r.PathValue("attempt"), actor.UserID, in.Decision, in.Rationale)
		if propagationCampaignError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
}

func propagationCampaignError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, propagationcampaigns.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "propagation_campaign_not_found"})
	case errors.Is(err, propagationcampaigns.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_propagation_campaign"})
	case errors.Is(err, propagationcampaigns.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "target_owner_required"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
