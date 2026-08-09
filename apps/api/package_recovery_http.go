package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	repositorycatalog "github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type recoveryInventoryStore interface {
	Get(string, string) (dependencyinventory.Inventory, error)
	List(string) ([]dependencyinventory.Inventory, error)
	Consumers(string) ([]dependencyinventory.Inventory, error)
}

type packageSafetyEnforcer struct {
	inventories recoveryInventoryStore
	packages    packageStore
}

func (s packageSafetyEnforcer) List(id string) ([]dependencyinventory.Inventory, error) {
	return s.inventories.List(id)
}
func (s packageSafetyEnforcer) GetByID(id string) (packagecatalog.Version, error) {
	return s.packages.GetByID(id)
}
func (s packageSafetyEnforcer) GetConsumerPolicy(id string) (packagecatalog.ConsumerPolicy, error) {
	return s.packages.GetConsumerPolicy(id)
}
func (s packageSafetyEnforcer) HasActiveException(repositoryID, packageID string) bool {
	return s.packages.HasActiveException(repositoryID, packageID)
}

func registerPackageRecoveryHTTP(mux *http.ServeMux, packages packageStore, inventories recoveryInventoryStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) {
	mux.HandleFunc("PUT /repositories/{repository}/packages/{package}/safety", setPackageSafety(packages, inventories, repositories, credentials, activity))
	mux.HandleFunc("GET /packages/{package}/exposure", packageExposure(packages, inventories, repositories, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/package-recovery-policy", putPackageRecoveryPolicy(packages, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/package-recovery-exceptions", createPackageRecoveryException(packages, inventories, repositories, credentials, activity))
	mux.HandleFunc("POST /repositories/{repository}/package-repairs", createPackageRepair(packages, inventories, proposalStore, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/package-repairs", listPackageRepairs(packages, repositories, credentials))
}

func setPackageSafety(store packageStore, inventories recoveryInventoryStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
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
			State                string `json:"state"`
			Reason               string `json:"reason"`
			ReplacementVersionID string `json:"replacement_version_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		item, err := store.SetSafety(string(repo.ID), r.PathValue("package"), in.State, in.Reason, in.ReplacementVersionID, actor.UserID)
		switch {
		case errors.Is(err, packagecatalog.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_safety_notice"})
			return
		case errors.Is(err, packagecatalog.ErrSafetyConflict):
			writeJSON(w, 409, map[string]string{"error": "safety_notice_unchanged"})
			return
		case errors.Is(err, packagecatalog.ErrNotFound):
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		consumers, _ := inventories.Consumers(item.ID)
		notified := map[string]bool{}
		for _, inv := range consumers {
			if notified[inv.RepositoryID] {
				continue
			}
			consumer, e := repositories.Inspect(storage.ID(inv.RepositoryID))
			if e != nil {
				continue
			}
			notified[inv.RepositoryID] = true
			_ = recordActivity(activity, activities.Input{RepositoryID: inv.RepositoryID, ActorID: actor.UserID, TargetUserID: consumer.OwnerID, Type: "package.exposure_detected", Resource: activities.Resource{Type: "package", ID: item.ID}, Metadata: map[string]string{"state": item.Lifecycle, "identity": item.Identity, "version": item.Version, "replacement_version_id": in.ReplacementVersionID}})
		}
		writeJSON(w, 200, map[string]any{"package": item, "notified_repository_count": len(notified)})
	}
}

type exposure struct {
	RepositoryID   string `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	InventoryID    string `json:"inventory_id"`
	CommitID       string `json:"commit_id"`
	DeploymentID   string `json:"deployment_id,omitempty"`
	Direct         bool   `json:"direct"`
	Remediation    string `json:"remediation"`
	RepairID       string `json:"repair_id,omitempty"`
}

func packageExposure(store packageStore, inventories recoveryInventoryStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := store.GetByID(r.PathValue("package"))
		if err != nil || item.Visibility != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		invs, _ := inventories.Consumers(item.ID)
		out := []exposure{}
		for _, inv := range invs {
			repo, e := repositories.Inspect(storage.ID(inv.RepositoryID))
			if e != nil || repo.Visibility != repositorycatalog.Public {
				continue
			}
			direct := false
			for _, x := range inv.Resolutions {
				if x.PackageVersionID == item.ID {
					direct = x.Direct
				}
			}
			state := "exposed"
			rid := ""
			repairs, _ := store.ListRepairs(inv.RepositoryID)
			for _, repair := range repairs {
				if repair.InventoryID == inv.ID && repair.PackageVersionID == item.ID {
					state = "repair_open"
					rid = repair.ID
					break
				}
			}
			out = append(out, exposure{string(repo.ID), repo.Name, inv.ID, inv.CommitID, inv.DeploymentID, direct, state, rid})
		}
		writeJSON(w, 200, map[string]any{"package": item, "items": out, "total_count": len(out)})
	}
}

func putPackageRecoveryPolicy(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
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
			BlockDeprecated bool `json:"block_deprecated"`
		}
		if !readJSON(w, r, &in, 2<<10) {
			return
		}
		p, err := store.PutConsumerPolicy(string(repo.ID), actor.UserID, in.BlockDeprecated)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_policy"})
			return
		}
		writeJSON(w, 200, p)
	}
}

func createPackageRecoveryException(store packageStore, inventories recoveryInventoryStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
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
			PackageVersionID string `json:"package_version_id"`
			Reason           string `json:"reason"`
			ExpiresInHours   int    `json:"expires_in_hours"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		found := false
		invs, _ := inventories.Consumers(in.PackageVersionID)
		for _, v := range invs {
			if v.RepositoryID == string(repo.ID) {
				found = true
			}
		}
		if !found || in.ExpiresInHours < 1 || in.ExpiresInHours > 24*30 {
			writeJSON(w, 422, map[string]string{"error": "invalid_exception"})
			return
		}
		v, err := store.CreateException(string(repo.ID), in.PackageVersionID, in.Reason, actor.UserID, time.Now().Add(time.Duration(in.ExpiresInHours)*time.Hour))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_exception"})
			return
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, Type: "package.exposure_excepted", Resource: activities.Resource{Type: "package", ID: in.PackageVersionID}, Metadata: map[string]string{"exception_id": v.ID, "reason": v.Reason}})
		writeJSON(w, 201, v)
	}
}

func createPackageRepair(store packageStore, inventories recoveryInventoryStore, plans proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			InventoryID      string `json:"inventory_id"`
			PackageVersionID string `json:"package_version_id"`
			OwnerType        string `json:"owner_type"`
			OwnerID          string `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		inv, err := inventories.Get(string(repo.ID), in.InventoryID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_inventory"})
			return
		}
		unsafe, err := store.GetByID(in.PackageVersionID)
		if err != nil || unsafe.Lifecycle == "active" || unsafe.Safety == nil {
			writeJSON(w, 422, map[string]string{"error": "package_not_unsafe"})
			return
		}
		present := false
		for _, x := range inv.Resolutions {
			if x.PackageVersionID == unsafe.ID {
				present = true
			}
		}
		if !present {
			writeJSON(w, 422, map[string]string{"error": "package_not_in_inventory"})
			return
		}
		if in.OwnerType == "" {
			in.OwnerType = "agent"
		}
		if in.OwnerType == "agent" {
			in.OwnerID = "codex"
		} else if in.OwnerType == "human" {
			participant := in.OwnerID == repo.OwnerID
			if !participant && in.OwnerID != "" {
				participant, _ = repositories.IsCollaborator(repo.ID, in.OwnerID)
			}
			if !participant {
				writeJSON(w, 422, map[string]string{"error": "invalid_repair_owner"})
				return
			}
		} else {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_owner"})
			return
		}
		existing, _ := store.ListRepairs(string(repo.ID))
		for _, repair := range existing {
			if repair.InventoryID == inv.ID && repair.PackageVersionID == unsafe.ID {
				writeJSON(w, 409, map[string]string{"error": "repair_exists"})
				return
			}
		}
		title := fmt.Sprintf("%s remediation: replace %s %s", strings.Title(unsafe.Lifecycle), unsafe.Identity, unsafe.Version)
		body := fmt.Sprintf("Priority ecosystem recovery for `%s` (`%s`).\n\nPublisher reason: %s\nSafe replacement: `%s`\nExposed inventory: `%s` at `%s`.\n\nThe consumer retains review, assignment, checks, and merge authority.", unsafe.ID, unsafe.Version, unsafe.Safety.Reason, unsafe.Safety.ReplacementVersionID, inv.ID, inv.CommitID)
		p, e := plans.Create(string(repo.ID), actor.UserID, title, body)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		task, e := plans.CreateTask(string(repo.ID), p.ID, actor.UserID, proposals.TaskInput{Title: title, Outcome: "Remove the unsafe resolution, adopt the safe replacement, and publish through ordinary review.", Position: 1, Status: proposals.TaskPlanned})
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ownerID := in.OwnerID
		assigned, e := plans.AssignTask(string(repo.ID), p.ID, task.ID, actor.UserID, "", proposals.AssignmentInput{Kind: proposals.AssigneeKind(in.OwnerType), AssigneeID: ownerID, Mandate: "Remediate only the captured unsafe dependency exposure.", RepositoryID: string(repo.ID), BaseRevision: inv.CommitID})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_owner"})
			return
		}
		v, e := store.CreateRepair(packagecatalog.Repair{RepositoryID: string(repo.ID), InventoryID: inv.ID, PackageVersionID: unsafe.ID, ReplacementVersionID: unsafe.Safety.ReplacementVersionID, ProposalID: p.ID, TaskID: assigned.ID, Priority: "urgent", OwnerType: in.OwnerType, OwnerID: ownerID, CreatedByID: actor.UserID})
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "repair_exists"})
			return
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, TargetUserID: ownerID, Type: "package.repair_opened", Resource: activities.Resource{Type: "proposal", ID: p.ID}, Metadata: map[string]string{"repair_id": v.ID, "package_version_id": unsafe.ID}})
		writeJSON(w, 201, v)
	}
}
func listPackageRepairs(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, err := store.ListRepairs(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
	}
}
