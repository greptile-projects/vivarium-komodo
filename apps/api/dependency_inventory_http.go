package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type dependencyInventoryStore interface {
	Create(dependencyinventory.CreateParams) (dependencyinventory.Inventory, error)
	List(string) ([]dependencyinventory.Inventory, error)
	Consumers(string) ([]dependencyinventory.Inventory, error)
}
type packageManifest struct {
	Version            int      `json:"version"`
	DirectDependencies []string `json:"direct_dependencies"`
}
type packageLock struct {
	Version  int             `json:"version"`
	Packages []lockedPackage `json:"packages"`
}
type lockedPackage struct {
	Identity         string   `json:"identity"`
	Version          string   `json:"version"`
	PackageVersionID string   `json:"package_version_id"`
	Dependencies     []string `json:"dependencies"`
}

func registerDependencyInventoryHTTP(mux *http.ServeMux, inventories dependencyInventoryStore, packages packageStore, releaseStore releaseStore, builds releaseBuildStore, deploymentStore deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/dependency-inventories", createDependencyInventory(inventories, packages, releaseStore, builds, deploymentStore, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/dependency-inventories", listDependencyInventories(inventories, repositories, credentials))
	mux.HandleFunc("GET /packages/{package}/consumers", packageConsumers(inventories, packages, repositories, credentials))
}

func createDependencyInventory(store dependencyInventoryStore, packages packageStore, releaseStore releaseStore, builds releaseBuildStore, deploymentsStore deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			CommitID     string `json:"commit_id"`
			ReleaseID    string `json:"release_id"`
			BuildRunID   string `json:"build_run_id"`
			DeploymentID string `json:"deployment_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		manifestBody, e1 := relationshipBlob(repositories, string(repo.ID), in.CommitID, ".komodo/packages.json")
		lockBody, e2 := relationshipBlob(repositories, string(repo.ID), in.CommitID, ".komodo/packages.lock.json")
		if e1 != nil || e2 != nil {
			writeJSON(w, 422, map[string]string{"error": "dependency_manifest_missing"})
			return
		}
		var manifest packageManifest
		var lock packageLock
		if json.Unmarshal(manifestBody, &manifest) != nil || json.Unmarshal(lockBody, &lock) != nil || manifest.Version != 1 || lock.Version != 1 || len(lock.Packages) > 1000 {
			writeJSON(w, 422, map[string]string{"error": "invalid_dependency_manifest"})
			return
		}
		if !validInventoryEvidence(w, string(repo.ID), in.CommitID, in.ReleaseID, in.BuildRunID, in.DeploymentID, releaseStore, builds, deploymentsStore) {
			return
		}
		direct := map[string]bool{}
		for _, v := range manifest.DirectDependencies {
			v = dependencyinventory.NormalizeIdentity(v)
			if v == "" || direct[v] {
				writeJSON(w, 422, map[string]string{"error": "invalid_dependency_manifest"})
				return
			}
			direct[v] = true
		}
		res := make([]dependencyinventory.Resolution, 0, len(lock.Packages)+len(direct))
		gaps := []string{}
		seenIDs := map[string]bool{}
		seenIdentities := map[string]bool{}
		for _, p := range lock.Packages {
			p.Identity = dependencyinventory.NormalizeIdentity(p.Identity)
			item, e := packages.GetByID(p.PackageVersionID)
			status, reason := "resolved", ""
			if e != nil {
				status, reason = "unresolved", "package version is unavailable"
			} else if !inventoryPackageReadable(item, actor.UserID, repositories) {
				status, reason = "unresolved", "package version is unavailable"
			} else if item.Identity != p.Identity || item.Version != p.Version {
				status, reason = "stale", "lock identity or version does not match immutable package"
			}
			if p.Identity == "" || p.PackageVersionID == "" || seenIDs[p.PackageVersionID] {
				writeJSON(w, 422, map[string]string{"error": "invalid_dependency_lock"})
				return
			}
			seenIDs[p.PackageVersionID] = true
			seenIdentities[p.Identity] = true
			deps := append([]string{}, p.Dependencies...)
			sort.Strings(deps)
			resolution := dependencyinventory.Resolution{Identity: p.Identity, PackageVersionID: p.PackageVersionID, Version: p.Version, Direct: direct[p.Identity], Dependencies: deps, Status: status, Reason: reason}
			if status != "unresolved" {
				resolution.PackageRepositoryID, resolution.SourceCommitID, resolution.ReleaseID = item.RepositoryID, item.SourceCommitID, item.ReleaseID
				resolution.BuildRunID, resolution.ArtifactID, resolution.ArtifactSHA256 = item.Build.RunID, item.ArtifactID, item.SHA256
				resolution.License, resolution.SupportURL = item.License, item.SupportURL
			}
			res = append(res, resolution)
			if status != "resolved" {
				gaps = append(gaps, p.Identity+": "+reason)
			}
		}
		for _, v := range res {
			for _, id := range v.Dependencies {
				if !seenIDs[id] {
					gaps = append(gaps, v.Identity+": unresolved transitive dependency "+id)
				}
			}
		}
		for identity := range direct {
			if !seenIdentities[identity] {
				res = append(res, dependencyinventory.Resolution{Identity: identity, Direct: true, Dependencies: []string{}, Status: "unresolved", Reason: "direct dependency is absent from lock"})
				gaps = append(gaps, identity+": direct dependency is absent from lock")
			}
		}
		ms := sha256.Sum256(manifestBody)
		ls := sha256.Sum256(lockBody)
		item, e := store.Create(dependencyinventory.CreateParams{RepositoryID: string(repo.ID), CommitID: in.CommitID, ReleaseID: in.ReleaseID, BuildRunID: in.BuildRunID, DeploymentID: in.DeploymentID, ManifestSHA256: hex.EncodeToString(ms[:]), LockSHA256: hex.EncodeToString(ls[:]), CreatedByID: actor.UserID, Resolutions: res, ProvenanceGaps: gaps})
		if errors.Is(e, dependencyinventory.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "inventory_exists"})
		} else if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		} else {
			writeJSON(w, 201, item)
		}
	}
}

func inventoryPackageReadable(item packagecatalog.Version, actorID string, repositories pullRequestRepositoryStore) bool {
	if item.Visibility == "public" {
		return true
	}
	repository, err := repositories.Inspect(storage.ID(item.RepositoryID))
	if err != nil {
		return false
	}
	if repository.OwnerID == actorID {
		return true
	}
	allowed, _ := repositories.IsCollaborator(repository.ID, actorID)
	return allowed
}

func validInventoryEvidence(w http.ResponseWriter, repositoryID, commitID, releaseID, buildRunID, deploymentID string, releasesStore releaseStore, builds releaseBuildStore, deploymentsStore deploymentStore) bool {
	if buildRunID != "" && releaseID == "" {
		writeJSON(w, 422, map[string]string{"error": "build_requires_release"})
		return false
	}
	if releaseID != "" {
		v, e := releasesStore.Get(repositoryID, releaseID)
		if errors.Is(e, releases.ErrNotFound) || e == nil && v.CommitID != commitID {
			writeJSON(w, 422, map[string]string{"error": "invalid_release_evidence"})
			return false
		}
		if e != nil {
			return false
		}
		if buildRunID != "" {
			runs, e := builds.List(repositoryID, "release:"+releaseID)
			found := false
			for _, run := range runs {
				if run.ID == buildRunID && run.CommitID == commitID && string(run.State) == "succeeded" {
					found = true
				}
			}
			if e != nil || !found {
				writeJSON(w, 422, map[string]string{"error": "invalid_build_evidence"})
				return false
			}
		}
	}
	if deploymentID != "" {
		d, e := deploymentsStore.GetDeployment(repositoryID, deploymentID)
		if errors.Is(e, deployments.ErrNotFound) || e == nil && (d.SourceCommitID != commitID || d.State != "succeeded") {
			writeJSON(w, 422, map[string]string{"error": "invalid_deployment_evidence"})
			return false
		}
		if e != nil {
			return false
		}
	}
	return true
}
func listDependencyInventories(store dependencyInventoryStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}

type packageConsumer struct {
	RepositoryID   string   `json:"repository_id"`
	CommitID       string   `json:"commit_id"`
	ReleaseID      string   `json:"release_id,omitempty"`
	BuildRunID     string   `json:"build_run_id,omitempty"`
	DeploymentID   string   `json:"deployment_id,omitempty"`
	Direct         bool     `json:"direct"`
	Status         string   `json:"status"`
	ProvenanceGaps []string `json:"provenance_gaps"`
}

func visiblePackageConsumers(store dependencyInventoryStore, packageID string, repositories pullRequestRepositoryStore, actorID string) []packageConsumer {
	items, e := store.Consumers(packageID)
	if e != nil {
		return []packageConsumer{}
	}
	out := []packageConsumer{}
	for _, v := range items {
		repo, e := repositories.Inspect(storage.ID(v.RepositoryID))
		if e != nil {
			continue
		}
		allowed := repo.Visibility == "public" || repo.OwnerID == actorID
		if actorID != "" && !allowed {
			allowed, _ = repositories.IsCollaborator(repo.ID, actorID)
		}
		if !allowed {
			continue
		}
		direct := false
		status := "resolved"
		for _, x := range v.Resolutions {
			if x.PackageVersionID == packageID {
				direct = x.Direct
				status = x.Status
				break
			}
		}
		out = append(out, packageConsumer{RepositoryID: v.RepositoryID, CommitID: v.CommitID, ReleaseID: v.ReleaseID, BuildRunID: v.BuildRunID, DeploymentID: v.DeploymentID, Direct: direct, Status: status, ProvenanceGaps: v.ProvenanceGaps})
	}
	return out
}
func packageConsumers(store dependencyInventoryStore, packages packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, e := packages.GetByID(r.PathValue("package"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		actor, authenticated, ok := authenticateOptionalRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		actorID := ""
		if authenticated {
			actorID = actor.UserID
		}
		if item.Visibility != "public" {
			publisher, inspectErr := repositories.Inspect(storage.ID(item.RepositoryID))
			allowed := inspectErr == nil && publisher.OwnerID == actorID
			if inspectErr == nil && actorID != "" && !allowed {
				allowed, _ = repositories.IsCollaborator(publisher.ID, actorID)
			}
			if !allowed {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		items := visiblePackageConsumers(store, item.ID, repositories, actorID)
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
