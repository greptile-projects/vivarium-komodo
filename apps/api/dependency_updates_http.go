package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyupdates"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
)

type dependencyUpdateStore interface {
	PutPolicy(dependencyupdates.Policy) (dependencyupdates.Policy, error)
	ListPolicies(string) ([]dependencyupdates.Policy, error)
	Create(dependencyupdates.Update) (dependencyupdates.Update, error)
	Exists(string, string, string, string) (bool, error)
	List(string) ([]dependencyupdates.Update, error)
}
type dependencyUpdateInventoryStore interface {
	Get(string, string) (dependencyinventory.Inventory, error)
}

func registerDependencyUpdateHTTP(mux *http.ServeMux, updates dependencyUpdateStore, inventories dependencyUpdateInventoryStore, packages packageStore, releases releaseStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) {
	mux.HandleFunc("PUT /repositories/{repository}/dependency-update-policies/{identity...}", putDependencyUpdatePolicy(updates, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/dependency-update-policies", listDependencyUpdatePolicies(updates, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/dependency-updates", createDependencyUpdates(updates, inventories, packages, releases, proposalStore, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/dependency-updates", listDependencyUpdates(updates, repositories, credentials))
}

func putDependencyUpdatePolicy(store dependencyUpdateStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			TargetBranch string `json:"target_branch"`
			Allowed      string `json:"allowed"`
			Enabled      bool   `json:"enabled"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		opened, openErr := repositories.Open(repo.ID)
		if openErr != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, _, found := branchTip(opened, in.TargetBranch); !found {
			writeJSON(w, 422, map[string]string{"error": "invalid_target_branch"})
			return
		}
		identity := dependencyinventory.NormalizeIdentity(r.PathValue("identity"))
		p, err := store.PutPolicy(dependencyupdates.Policy{RepositoryID: string(repo.ID), Identity: identity, TargetBranch: in.TargetBranch, Allowed: in.Allowed, Enabled: in.Enabled, UpdatedByID: actor.UserID})
		if errors.Is(err, dependencyupdates.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_update_policy"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, p)
	}
}
func listDependencyUpdatePolicies(store dependencyUpdateStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.ListPolicies(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func listDependencyUpdates(store dependencyUpdateStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}

func createDependencyUpdates(store dependencyUpdateStore, inventories dependencyUpdateInventoryStore, packages packageStore, releaseStore releaseStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			InventoryID string `json:"inventory_id"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		inventory, err := inventories.Get(string(repo.ID), in.InventoryID)
		if errors.Is(err, dependencyinventory.ErrNotFound) {
			writeJSON(w, 422, map[string]string{"error": "invalid_inventory"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		policies, err := store.ListPolicies(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		policyByIdentity := map[string]dependencyupdates.Policy{}
		for _, p := range policies {
			if p.Enabled {
				policyByIdentity[p.Identity] = p
			}
		}
		var manifest packageManifest
		var lock packageLock
		manifestBody, err := relationshipBlob(repositories, string(repo.ID), inventory.CommitID, ".komodo/packages.json")
		if err == nil {
			err = json.Unmarshal(manifestBody, &manifest)
		}
		lockBody, lockErr := relationshipBlob(repositories, string(repo.ID), inventory.CommitID, ".komodo/packages.lock.json")
		if lockErr == nil {
			lockErr = json.Unmarshal(lockBody, &lock)
		}
		if err != nil || lockErr != nil || manifest.Version != 1 || lock.Version != 1 {
			writeJSON(w, 422, map[string]string{"error": "inventory_source_unavailable"})
			return
		}
		created := []dependencyupdates.Update{}
		for _, current := range inventory.Resolutions {
			policy, enabled := policyByIdentity[current.Identity]
			if !enabled || !current.Direct || current.Status != "resolved" {
				continue
			}
			candidates, _ := packages.Search(current.Identity)
			candidate, found := newestEligible(current, candidates, policy.Allowed, actor.UserID, repositories)
			if !found {
				continue
			}
			exists, checkErr := store.Exists(string(repo.ID), inventory.CommitID, current.Identity, candidate.ID)
			if checkErr != nil || exists {
				continue
			}
			proposed := lock
			proposed.Packages = append([]lockedPackage{}, lock.Packages...)
			for i := range proposed.Packages {
				if proposed.Packages[i].PackageVersionID == current.PackageVersionID {
					proposed.Packages[i].PackageVersionID = candidate.ID
					proposed.Packages[i].Version = candidate.Version
				}
			}
			newManifest, _ := json.MarshalIndent(manifest, "", "  ")
			newLock, _ := json.MarshalIndent(proposed, "", "  ")
			releaseNotes := ""
			if rel, e := releaseStore.Get(candidate.RepositoryID, candidate.ReleaseID); e == nil {
				releaseNotes = rel.Notes
			}
			compatibility := "same declared package dependencies"
			if len(candidate.Dependencies) > 0 {
				compatibility = "publisher declares dependencies; exact transitive lock resolution requires review"
			}
			paths := affectedDependencyPaths(inventory, current.PackageVersionID)
			title := fmt.Sprintf("Update %s to %s", current.Identity, candidate.Version)
			body := fmt.Sprintf("Package update detected by consumer policy.\n\nCurrent: `%s` (`%s`)\nCandidate: `%s` (`%s`)\nPublisher release: `%s`\nSource: `%s`\nArtifact SHA-256: `%s`\nCompatibility: %s\nAffected dependency paths:\n%s\n\nRelease notes:\n%s\n\nProposed `.komodo/packages.json`:\n```json\n%s\n```\n\nProposed `.komodo/packages.lock.json`:\n```json\n%s\n```", current.Version, current.PackageVersionID, candidate.Version, candidate.ID, candidate.ReleaseID, candidate.SourceCommitID, candidate.SHA256, compatibility, strings.Join(paths, "\n"), releaseNotes, newManifest, newLock)
			proposal, e := proposalStore.Create(string(repo.ID), actor.UserID, title, body)
			if e != nil {
				continue
			}
			task, e := proposalStore.CreateTask(string(repo.ID), proposal.ID, actor.UserID, proposals.TaskInput{Title: title, Outcome: "Commit the reviewed manifest and lock changes, resolve compatibility failures, and publish through ordinary pull-request review.", Position: 1, Status: proposals.TaskPlanned})
			if e != nil {
				continue
			}
			u, e := store.Create(dependencyupdates.Update{RepositoryID: string(repo.ID), InventoryID: inventory.ID, BaseCommitID: inventory.CommitID, TargetBranch: policy.TargetBranch, Identity: current.Identity, ProposalID: proposal.ID, TaskID: task.ID, Manifest: newManifest, Lock: newLock, Evidence: dependencyupdates.Evidence{CurrentPackageVersionID: current.PackageVersionID, CurrentVersion: current.Version, CandidatePackageVersionID: candidate.ID, CandidateVersion: candidate.Version, PublisherRepositoryID: candidate.RepositoryID, ReleaseID: candidate.ReleaseID, ReleaseNotes: releaseNotes, SourceCommitID: candidate.SourceCommitID, BuildRunID: candidate.Build.RunID, ArtifactSHA256: candidate.SHA256, Compatibility: compatibility, AffectedPaths: paths}, CreatedByID: actor.UserID})
			if e != nil {
				continue
			}
			created = append(created, u)
			_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, Type: "dependency_update.proposed", Resource: activities.Resource{Type: "proposal", ID: proposal.ID}, Metadata: map[string]string{"update_id": u.ID, "identity": u.Identity, "candidate_version": candidate.Version}})
		}
		writeJSON(w, 201, map[string]any{"items": created, "total_count": len(created)})
	}
}

func newestEligible(current dependencyinventory.Resolution, candidates []packagecatalog.Version, allowed, actorID string, repositories pullRequestRepositoryStore) (packagecatalog.Version, bool) {
	cv, ok := semver(current.Version)
	if !ok {
		return packagecatalog.Version{}, false
	}
	var best packagecatalog.Version
	var bv [3]int
	found := false
	for _, v := range candidates {
		if v.Identity != current.Identity || v.Lifecycle != "active" || !inventoryPackageReadable(v, actorID, repositories) {
			continue
		}
		vv, valid := semver(v.Version)
		if !valid || compareVersion(vv, cv) <= 0 || !allowedBump(cv, vv, allowed) {
			continue
		}
		if !found || compareVersion(vv, bv) > 0 {
			best, bv, found = v, vv, true
		}
	}
	return best, found
}
func semver(v string) ([3]int, bool) {
	var out [3]int
	core := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, e := strconv.Atoi(p)
		if e != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
func compareVersion(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func allowedBump(a, b [3]int, allowed string) bool {
	if b[0] != a[0] {
		return allowed == "major"
	}
	if b[1] != a[1] {
		return allowed == "minor" || allowed == "major"
	}
	return true
}
func affectedDependencyPaths(inv dependencyinventory.Inventory, target string) []string {
	byID := map[string]dependencyinventory.Resolution{}
	for _, r := range inv.Resolutions {
		byID[r.PackageVersionID] = r
	}
	out := []string{}
	var walk func(string, []string, map[string]bool)
	walk = func(id string, path []string, seen map[string]bool) {
		if seen[id] {
			return
		}
		seen[id] = true
		r, ok := byID[id]
		if !ok {
			return
		}
		next := append(path, r.Identity+"@"+r.Version)
		if id == target {
			out = append(out, "- "+strings.Join(next, " → "))
			return
		}
		for _, d := range r.Dependencies {
			walk(d, next, seen)
		}
	}
	for _, r := range inv.Resolutions {
		if r.Direct {
			walk(r.PackageVersionID, nil, map[string]bool{})
		}
	}
	sort.Strings(out)
	return out
}
