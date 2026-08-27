package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/adoptioncampaigns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancebundles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
)

func registerAdoptionCampaignsHTTP(mux *http.ServeMux, store *adoptioncampaigns.Store, releaseStore releaseStore, bundles *provenancebundles.Store, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/adoption-campaigns"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if adoptionCampaignError(w, e) {
			return
		}
		items = resolveAdoptionCampaigns(items, releaseStore, string(repo.ID))
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in adoptioncampaigns.VersionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		if !attestedRelease(string(repo.ID), in, releaseStore, bundles) {
			writeJSON(w, 422, map[string]string{"error": "release_is_not_exactly_attested"})
			return
		}
		c, e := store.Create(string(repo.ID), a.UserID, in)
		if adoptionCampaignError(w, e) {
			return
		}
		all, _ := store.List(string(repo.ID))
		resolved := resolveAdoptionCampaigns(all, releaseStore, string(repo.ID))
		for _, x := range resolved {
			if x.ID == c.ID {
				c = x
			}
		}
		w.Header().Set("Location", base+"/"+c.ID)
		writeJSON(w, 201, c)
	})
	mux.HandleFunc("GET "+base+"/{campaign}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		c, e := store.Get(string(repo.ID), r.PathValue("campaign"))
		if adoptionCampaignError(w, e) {
			return
		}
		all, _ := store.List(string(repo.ID))
		for _, x := range resolveAdoptionCampaigns(all, releaseStore, string(repo.ID)) {
			if x.ID == c.ID {
				c = x
			}
		}
		writeJSON(w, 200, c)
	})
	mux.HandleFunc("POST "+base+"/{campaign}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			adoptioncampaigns.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		if !attestedRelease(string(repo.ID), in.VersionInput, releaseStore, bundles) {
			writeJSON(w, 422, map[string]string{"error": "release_is_not_exactly_attested"})
			return
		}
		c, e := store.Revise(string(repo.ID), r.PathValue("campaign"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if adoptionCampaignError(w, e) {
			return
		}
		all, _ := store.List(string(repo.ID))
		for _, x := range resolveAdoptionCampaigns(all, releaseStore, string(repo.ID)) {
			if x.ID == c.ID {
				c = x
			}
		}
		writeJSON(w, 201, c)
	})
}

func attestedRelease(repo string, in adoptioncampaigns.VersionInput, rs releaseStore, bs *provenancebundles.Store) bool {
	r, e := rs.Get(repo, in.ReleaseID)
	if e != nil || r.Version != in.ReleaseVersion || r.CommitID != in.ReleaseRevision {
		return false
	}
	b, e := bs.Get(repo, in.BundleID)
	return e == nil && b.ReleaseID == r.ID && b.ReleaseVersion == r.Version && b.Revision == r.CommitID && b.Verification.PayloadSHA256 == in.BundleDigest && bs.Verify(b)
}
func resolveAdoptionCampaigns(items []adoptioncampaigns.Campaign, rs releaseStore, repo string) []adoptioncampaigns.Campaign {
	rels, _ := rs.List(repo)
	latest := ""
	if len(rels) > 0 {
		latest = rels[0].ID
	}
	for i := range items {
		v := items[i].Versions[len(items[i].Versions)-1]
		items[i].Findings = adoptioncampaigns.IntrinsicFindings(items[i])
		if latest != "" && v.ReleaseID != latest {
			items[i].Findings = append(items[i].Findings, adoptioncampaigns.Finding{Kind: "superseded_release", Detail: "a newer repository release exists", OwnerID: v.AuthorID, Reference: v.ReleaseID, Blocking: true})
		}
		if time.Now().UTC().After(v.Deadline) {
			items[i].Findings = append(items[i].Findings, adoptioncampaigns.Finding{Kind: "deadline_passed", Detail: "campaign deadline passed without a later outcome", OwnerID: v.AuthorID, Reference: v.Deadline.Format(time.RFC3339), Blocking: true})
		}
		for j := range items {
			if i == j {
				continue
			}
			ov := items[j].Versions[len(items[j].Versions)-1]
			if v.ReleaseID == ov.ReleaseID {
				continue
			}
			for _, a := range v.Audiences {
				for _, b := range ov.Audiences {
					if a.ID == b.ID {
						items[i].Findings = append(items[i].Findings, adoptioncampaigns.Finding{Kind: "conflicting_campaign", Detail: "the same audience is targeted by another release campaign", OwnerID: ov.AuthorID, Reference: items[j].ID, Blocking: true})
					}
				}
			}
		}
	}
	return items
}
func adoptionCampaignError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, adoptioncampaigns.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "adoption_campaign_not_found"})
	case errors.Is(e, adoptioncampaigns.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_adoption_campaign"})
	case errors.Is(e, adoptioncampaigns.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "adoption_campaign_changed"})
	case errors.Is(e, releases.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "release_not_found"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
