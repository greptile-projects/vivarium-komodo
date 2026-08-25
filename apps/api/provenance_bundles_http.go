package main

import (
	"errors"
	"net/http"
	"path"
	"sort"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenanceassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancebundles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
)

func registerProvenanceBundlesHTTP(mux *http.ServeMux, store *provenancebundles.Store, releaseStore releaseStore, builds releaseBuildStore, graphs *provenancegraphs.Store, assessments *provenanceassessments.Store, packages packageStore, repos pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/releases/{release}/provenance-bundles", publishProvenanceBundle(store, releaseStore, builds, graphs, assessments, repos, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}/provenance-bundles", listReleaseProvenanceBundles(store, releaseStore, repos, credentials))
	mux.HandleFunc("POST /repositories/{repository}/provenance-bundles/{bundle}/observations", observeProvenanceBundle(store, repos, credentials))
	mux.HandleFunc("GET /provenance-bundles/{bundle}", publicProvenanceBundle(store))
	mux.HandleFunc("POST /provenance-bundles/{bundle}/verify", verifyProvenanceBundle(store))
	mux.HandleFunc("GET /provenance-bundles/{bundle}/compare/{other}", compareProvenanceBundles(store))
	mux.HandleFunc("GET /packages/{package}/provenance", packageProvenanceBundle(store, packages))
}

func publishProvenanceBundle(store *provenancebundles.Store, releasesStore releaseStore, builds releaseBuildStore, graphs *provenancegraphs.Store, assessments *provenanceassessments.Store, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		rel, e := releasesStore.Get(string(repo.ID), r.PathValue("release"))
		if errors.Is(e, releases.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		var in struct {
			Audience           string                          `json:"audience"`
			GraphID            string                          `json:"graph_id"`
			AssessmentID       string                          `json:"assessment_id"`
			Artifacts          []provenancebundles.Artifact    `json:"artifacts"`
			Components         []provenancebundles.Component   `json:"components"`
			Licenses           []string                        `json:"licenses"`
			Notices            []string                        `json:"notices"`
			SourceAttestations []provenancebundles.Attestation `json:"source_attestations"`
			BuildAttestations  []provenancebundles.Attestation `json:"build_attestations"`
			Omissions          []provenancebundles.Omission    `json:"omissions"`
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		gs, e := graphs.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		validGraph := false
		for _, g := range gs {
			if g.ID == in.GraphID && g.Revision == rel.CommitID {
				validGraph = true
				break
			}
		}
		if !validGraph {
			writeJSON(w, 422, map[string]string{"error": "graph_does_not_match_release"})
			return
		}
		a, e := assessments.Get(in.AssessmentID)
		if e != nil || a.RepositoryID != string(repo.ID) || a.GraphID != in.GraphID || a.Revision != rel.CommitID || a.CandidateKind != "release_candidate" || a.CandidateID != rel.Version {
			writeJSON(w, 422, map[string]string{"error": "assessment_does_not_match_release"})
			return
		}
		for i := range in.Artifacts {
			want := &in.Artifacts[i]
			artifact, file, e := builds.OpenArtifact(string(repo.ID), "release:"+rel.ID, want.BuildRunID, want.ID)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "artifact_does_not_match_release"})
				return
			}
			file.Close()
			if artifact.SHA256 != want.SHA256 || artifact.Size != want.Size || artifact.MediaType != want.MediaType {
				writeJSON(w, 422, map[string]string{"error": "artifact_does_not_match_release"})
				return
			}
			if want.Name == "" {
				want.Name = path.Base(artifact.Path)
			}
		}
		v, e := store.Publish(provenancebundles.PublishInput{RepositoryID: string(repo.ID), ReleaseID: rel.ID, ReleaseVersion: rel.Version, Revision: rel.CommitID, Audience: in.Audience, GraphID: in.GraphID, AssessmentID: a.ID, PolicyVersion: int(a.PolicyVersion), Artifacts: in.Artifacts, Components: in.Components, Licenses: in.Licenses, Notices: in.Notices, SourceAttestations: in.SourceAttestations, BuildAttestations: in.BuildAttestations, Omissions: in.Omissions, PublishedByID: actor.UserID})
		if errors.Is(e, provenancebundles.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "bundle_exists"})
		} else if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_provenance_bundle"})
		} else {
			w.Header().Set("Location", "/provenance-bundles/"+v.ID)
			writeJSON(w, 201, v)
		}
	}
}

func listReleaseProvenanceBundles(store *provenancebundles.Store, releasesStore releaseStore, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		if _, e := releasesStore.Get(string(repo.ID), r.PathValue("release")); e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, e := store.FindRelease(string(repo.ID), r.PathValue("release"))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func publicProvenanceBundle(store *provenancebundles.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := store.Get("", r.PathValue("bundle"))
		if e != nil || v.Audience != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func verifyProvenanceBundle(store *provenancebundles.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := store.Get("", r.PathValue("bundle"))
		if e != nil || v.Audience != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"bundle_id": v.ID, "verified": store.Verify(v), "payload_sha256": v.Verification.PayloadSHA256, "artifact_sha256": artifactDigests(v)})
	}
}
func compareProvenanceBundles(store *provenancebundles.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, e1 := store.Get("", r.PathValue("bundle"))
		b, e2 := store.Get("", r.PathValue("other"))
		if e1 != nil || e2 != nil || a.Audience != "public" || b.Audience != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, compareBundles(a, b, store.Verify(a), store.Verify(b)))
	}
}
func observeProvenanceBundle(store *provenancebundles.Store, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in provenancebundles.Notice
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := store.Observe(string(repo.ID), r.PathValue("bundle"), actor.UserID, in)
		if errors.Is(e, provenancebundles.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
		} else if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_trust_observation"})
		} else {
			writeJSON(w, 201, v)
		}
	}
}
func packageProvenanceBundle(store *provenancebundles.Store, packages packageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, e := packages.GetByID(r.PathValue("package"))
		if e != nil || p.Visibility != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items, e := store.FindRelease(p.RepositoryID, p.ReleaseID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := []provenancebundles.Bundle{}
		for _, v := range items {
			if v.Audience != "public" {
				continue
			}
			for _, a := range v.Artifacts {
				if a.ID == p.ArtifactID && a.SHA256 == p.SHA256 {
					out = append(out, v)
					break
				}
			}
		}
		writeJSON(w, 200, map[string]any{"package": map[string]string{"id": p.ID, "identity": p.Identity, "version": p.Version, "sha256": p.SHA256}, "items": out, "total_count": len(out)})
	}
}
func artifactDigests(v provenancebundles.Bundle) map[string]string {
	o := map[string]string{}
	for _, a := range v.Artifacts {
		o[a.Name] = a.SHA256
	}
	return o
}
func compareBundles(a, b provenancebundles.Bundle, aVerified, bVerified bool) map[string]any {
	ac, bc := map[string]string{}, map[string]string{}
	for _, x := range a.Components {
		ac[x.Kind+":"+x.Name] = x.Version + "|" + x.SHA256 + "|" + x.License
	}
	for _, x := range b.Components {
		bc[x.Kind+":"+x.Name] = x.Version + "|" + x.SHA256 + "|" + x.License
	}
	added, removed, changed := []string{}, []string{}, []string{}
	for k, v := range bc {
		if x, ok := ac[k]; !ok {
			added = append(added, k)
		} else if x != v {
			changed = append(changed, k)
		}
	}
	for k := range ac {
		if _, ok := bc[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return map[string]any{"base_bundle_id": a.ID, "other_bundle_id": b.ID, "signatures_verified": map[string]bool{a.ID: aVerified, b.ID: bVerified}, "added_components": added, "removed_components": removed, "changed_components": changed, "base_trust_status": a.TrustStatus, "other_trust_status": b.TrustStatus, "base_omissions": a.Omissions, "other_omissions": b.Omissions}
}
