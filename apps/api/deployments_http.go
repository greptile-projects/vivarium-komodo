package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type deploymentStore interface {
	PutEnvironment(string, string, string, deployments.EnvironmentInput) (deployments.Environment, error)
	ListEnvironments(string) ([]deployments.Environment, error)
	GetEnvironment(string, string) (deployments.Environment, error)
	Secrets(string, string) (map[string]string, error)
	Create(deployments.CreateDeployment) (deployments.Deployment, error)
	Approve(string, string, string) (deployments.Deployment, error)
	Start(string, string) (deployments.Deployment, error)
	Log(string, string, string, string) error
	Complete(string, string, bool, string) (deployments.Deployment, error)
	Stage(string, string, string, string, string, string, string) (deployments.Deployment, error)
	Control(string, string, string, string, string) (deployments.Deployment, error)
	GetDeployment(string, string) (deployments.Deployment, error)
	ListDeployments(string) ([]deployments.Deployment, error)
}

type deploymentPackageSafety interface {
	List(string) ([]dependencyinventory.Inventory, error)
	GetByID(string) (packagecatalog.Version, error)
	GetConsumerPolicy(string) (packagecatalog.ConsumerPolicy, error)
	HasActiveException(string, string) bool
}

func registerDeploymentsHTTP(mux *http.ServeMux, store deploymentStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore, activity activityStore, sessions changeSessionStore, pulls pullRequestStore, safety ...deploymentPackageSafety) {
	var packageSafety deploymentPackageSafety
	if len(safety) > 0 {
		packageSafety = safety[0]
	}
	mux.HandleFunc("GET /repositories/{repository}/environments", listEnvironments(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/environments", putEnvironment(store, repositories, credentials, false))
	mux.HandleFunc("PUT /repositories/{repository}/environments/{environment}", putEnvironment(store, repositories, credentials, true))
	mux.HandleFunc("GET /repositories/{repository}/deployments", listDeployments(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/deployments/{deployment}", getDeployment(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/deployments", createDeployment(store, releaseStore, builds, repositories, credentials, activity, packageSafety))
	mux.HandleFunc("POST /repositories/{repository}/deployments/{deployment}/approvals", approveDeployment(store, repositories, credentials, builds, activity))
	mux.HandleFunc("POST /repositories/{repository}/deployments/{deployment}/control", controlDeployment(store, repositories, credentials, activity))
	mux.HandleFunc("POST /repositories/{repository}/deployments/{deployment}/recovery", recoverDeployment(store, builds, repositories, credentials, activity, sessions, pulls))
}

func recoverDeployment(store deploymentStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore, activity activityStore, sessions changeSessionStore, pulls pullRequestStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repo.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		failed, err := store.GetDeployment(string(repo.ID), r.PathValue("deployment"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if failed.State != "failed" {
			writeJSON(w, 409, map[string]string{"error": "deployment_not_unhealthy"})
			return
		}
		var input struct {
			Action       string   `json:"action"`
			Instructions string   `json:"instructions"`
			ContextPaths []string `json:"context_paths"`
		}
		if !readJSON(w, r, &input, 32<<10) {
			return
		}
		switch input.Action {
		case "rollback":
			items, listErr := store.ListDeployments(string(repo.ID))
			if listErr != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			var known *deployments.Deployment
			for i := range items {
				candidate := &items[i]
				if candidate.EnvironmentID == failed.EnvironmentID && candidate.State == "succeeded" && candidate.CreatedAt.Before(failed.CreatedAt) {
					known = candidate
					break
				}
			}
			if known == nil {
				writeJSON(w, 409, map[string]string{"error": "no_known_good_deployment"})
				return
			}
			item, createErr := store.Create(deployments.CreateDeployment{RepositoryID: string(repo.ID), EnvironmentID: known.EnvironmentID, ReleaseID: known.ReleaseID, BuildRunID: known.BuildRunID, ArtifactID: known.ArtifactID, ArtifactPath: known.ArtifactPath, ArtifactSHA256: known.ArtifactSHA256, SourceCommitID: known.SourceCommitID, ActorID: actor.UserID, RecoveryOfID: failed.ID, RecoveryAction: "rollback"})
			if createErr != nil {
				writeJSON(w, 409, map[string]string{"error": "recovery_not_accepted"})
				return
			}
			_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, Type: "deployment.rollback_started", Resource: activities.Resource{Type: "deployment", ID: item.ID}, Metadata: map[string]string{"failed_deployment_id": failed.ID, "known_good_deployment_id": known.ID, "release_id": item.ReleaseID, "environment_id": item.EnvironmentID}})
			if item.State == "queued" {
				go runDeployment(store, builds, repositories, activity, item)
			}
			writeJSON(w, http.StatusCreated, map[string]any{"deployment": item, "known_good_deployment_id": known.ID})
			return
		case "repair":
			if strings.TrimSpace(input.Instructions) == "" {
				input.Instructions = "Diagnose the unhealthy deployment evidence and prepare a reviewed code repair. Do not operate the deployment environment."
			}
			if len(input.Instructions) > 10000 || len(input.ContextPaths) > 50 {
				writeJSON(w, 422, map[string]string{"error": "invalid_recovery"})
				return
			}
			for _, contextPath := range input.ContextPaths {
				if contextPath == "" || strings.HasPrefix(contextPath, "/") || strings.Contains(contextPath, "..") || len(contextPath) > 500 {
					writeJSON(w, 422, map[string]string{"error": "invalid_recovery"})
					return
				}
			}
			opened, openErr := repositories.Open(repo.ID)
			if openErr != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			defaultRef, refErr := opened.DefaultBranch()
			if refErr != nil {
				writeJSON(w, 409, map[string]string{"error": "default_branch_unavailable"})
				return
			}
			targetBranch := strings.TrimPrefix(string(defaultRef), "refs/heads/")
			target, _, targetOK := branchTip(opened, targetBranch)
			if !targetOK {
				writeJSON(w, 409, map[string]string{"error": "default_branch_unavailable"})
				return
			}
			branch := "codex/recovery-" + failed.ID[:12]
			refName := storage.ReferenceName("refs/heads/" + branch)
			if err = opened.CreateReference(storage.Reference{Name: refName, ObjectID: storage.ObjectID(failed.SourceCommitID)}); err != nil {
				writeJSON(w, 409, map[string]string{"error": "recovery_branch_unavailable"})
				return
			}
			pull, err := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), AuthorID: actor.UserID, Title: "Repair unhealthy deployment", Body: "Diagnose deployment " + failed.ID + " for release " + failed.ReleaseID + ". Changes must pass ordinary review and integration before a new release.", SourceBranch: branch, TargetBranch: targetBranch, SourceCommitID: failed.SourceCommitID, TargetCommitID: string(target), Draft: true})
			if err != nil {
				_ = opened.DeleteReference(refName)
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			evidence := &changesessions.DeploymentFailure{DeploymentID: failed.ID, ReleaseID: failed.ReleaseID, EnvironmentID: failed.EnvironmentID, BuildRunID: failed.BuildRunID, ArtifactID: failed.ArtifactID, ArtifactPath: failed.ArtifactPath, ArtifactSHA256: failed.ArtifactSHA256, SourceCommitID: failed.SourceCommitID, State: failed.State, CurrentStage: failed.CurrentStage}
			for _, event := range failed.Events {
				evidence.Events = append(evidence.Events, changesessions.DeploymentEvidenceEvent{Sequence: event.Sequence, Type: event.Type, Stream: event.Stream, Message: event.Message, Stage: event.Stage, Signal: event.Signal, Outcome: event.Outcome, CreatedAt: event.CreatedAt})
			}
			session, err := sessions.CreateWithDeploymentFailure(string(repo.ID), pull.ID, actor.UserID, failed.SourceCommitID, evidence)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			issued, err := credentials.IssueRepositoryGit(actor.UserID, "Deployment repair "+session.ID, string(repo.ID), string(refName), 24*time.Hour)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			run, err := sessions.Delegate(string(repo.ID), pull.ID, session.ID, changesessions.DelegateParams{InitiatorID: actor.UserID, Agent: "codex", Instructions: input.Instructions, RevisionID: failed.SourceCommitID, ContextPaths: input.ContextPaths, WorkingBranch: branch, CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt})
			if err != nil {
				_, _ = credentials.Revoke(actor.UserID, issued.ID)
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, Type: "deployment.repair_started", Resource: activities.Resource{Type: "pull_request", ID: pull.ID}, Metadata: map[string]string{"deployment_id": failed.ID, "release_id": failed.ReleaseID, "session_id": session.ID, "run_id": run.ID}})
			writeJSON(w, http.StatusCreated, map[string]any{"pull_request": pull, "session": session, "run": run, "credential": map[string]any{"token": issued.Token, "username": "agent", "expires_at": issued.ExpiresAt, "repository_id": string(repo.ID), "branch": string(refName)}})
			return
		default:
			writeJSON(w, 422, map[string]string{"error": "invalid_recovery"})
		}
	}
}

func listEnvironments(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.ListEnvironments(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func putEnvironment(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore, update bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in deployments.EnvironmentInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		id := ""
		status := 201
		if update {
			id = r.PathValue("environment")
			status = 200
		}
		item, e := store.PutEnvironment(string(repo.ID), id, actor.UserID, in)
		if errors.Is(e, deployments.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_environment"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, status, item)
	}
}
func listDeployments(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.ListDeployments(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func getDeployment(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, e := store.GetDeployment(string(repo.ID), r.PathValue("deployment"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		after := int64(0)
		if raw := r.URL.Query().Get("after"); raw != "" {
			for _, c := range raw {
				if c < '0' || c > '9' {
					writeJSON(w, 422, map[string]string{"error": "invalid_after"})
					return
				}
				after = after*10 + int64(c-'0')
			}
		}
		if after > 0 {
			events := []deployments.Event{}
			for _, event := range item.Events {
				if event.Sequence > after {
					events = append(events, event)
				}
			}
			item.Events = events
		}
		writeJSON(w, 200, item)
	}
}

func createDeployment(store deploymentStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, safety ...deploymentPackageSafety) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repo.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			EnvironmentID string `json:"environment_id"`
			ReleaseID     string `json:"release_id"`
			BuildRunID    string `json:"build_run_id"`
			ArtifactID    string `json:"artifact_id"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		release, e := releaseStore.Get(string(repo.ID), in.ReleaseID)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "release_not_found"})
			return
		}
		if len(safety) > 0 && safety[0] != nil {
			inventories, _ := safety[0].List(string(repo.ID))
			policy, _ := safety[0].GetConsumerPolicy(string(repo.ID))
			for _, inventory := range inventories {
				if inventory.CommitID != release.CommitID {
					continue
				}
				for _, resolution := range inventory.Resolutions {
					version, versionErr := safety[0].GetByID(resolution.PackageVersionID)
					if versionErr != nil {
						continue
					}
					blocked := version.Lifecycle == "quarantined" || (version.Lifecycle == "deprecated" && policy.BlockDeprecated && !safety[0].HasActiveException(string(repo.ID), version.ID))
					if blocked {
						writeJSON(w, 409, map[string]string{"error": "unsafe_package_blocks_promotion", "package_version_id": version.ID, "lifecycle": version.Lifecycle})
						return
					}
				}
				break
			}
		}
		attempts, e := builds.List(string(repo.ID), "release:"+release.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		latest := map[string]checkruns.Run{}
		for i := len(attempts) - 1; i >= 0; i-- {
			latest[attempts[i].Definition.Name] = attempts[i]
		}
		verified := len(latest) > 0
		for _, run := range latest {
			if run.State != checkruns.Succeeded {
				verified = false
			}
		}
		if !verified {
			writeJSON(w, 409, map[string]string{"error": "release_not_verified"})
			return
		}
		run, e := builds.Get(string(repo.ID), "release:"+release.ID, in.BuildRunID)
		if e != nil || run.State != checkruns.Succeeded {
			writeJSON(w, 422, map[string]string{"error": "invalid_build_attempt"})
			return
		}
		artifact, file, e := builds.OpenArtifact(string(repo.ID), "release:"+release.ID, run.ID, in.ArtifactID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_artifact"})
			return
		}
		file.Close()
		environment, e := store.GetEnvironment(string(repo.ID), in.EnvironmentID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_environment"})
			return
		}
		if _, e = deploymentStages(repositories, string(repo.ID), release.CommitID, environment.Name); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_deployment_manifest"})
			return
		}
		item, e := store.Create(deployments.CreateDeployment{RepositoryID: string(repo.ID), EnvironmentID: in.EnvironmentID, ReleaseID: release.ID, BuildRunID: run.ID, ArtifactID: artifact.ID, ArtifactPath: artifact.Path, ArtifactSHA256: artifact.SHA256, SourceCommitID: release.CommitID, ActorID: actor.UserID})
		if errors.Is(e, deployments.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "environment_concurrency_reached"})
			return
		}
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_deployment"})
			return
		}
		if item.State == "queued" {
			go runDeployment(store, builds, repositories, activity, item)
		}
		writeJSON(w, 201, item)
	}
}
func approveDeployment(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore, builds releaseBuildStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repo.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item, e := store.GetDeployment(string(repo.ID), r.PathValue("deployment"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if item.InitiatedByID == actor.UserID {
			writeJSON(w, 409, map[string]string{"error": "initiator_cannot_approve"})
			return
		}
		item, e = store.Approve(string(repo.ID), item.ID, actor.UserID)
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "approval_not_accepted"})
			return
		}
		if item.State == "queued" {
			go runDeployment(store, builds, repositories, activity, item)
		}
		writeJSON(w, 200, item)
	}
}

func controlDeployment(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repo.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &input, 8192) {
			return
		}
		item, err := store.Control(string(repo.ID), r.PathValue("deployment"), actor.UserID, input.Action, input.Reason)
		if errors.Is(err, deployments.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_control"})
			return
		}
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "control_not_accepted"})
			return
		}
		eventType := "deployment." + input.Action
		target := item.InitiatedByID
		if target == actor.UserID {
			target = string(repo.OwnerID)
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repo.ID), ActorID: actor.UserID, Type: eventType, Resource: activities.Resource{Type: "deployment", ID: item.ID}, TargetUserID: target, Metadata: map[string]string{"state": item.State, "environment_id": item.EnvironmentID, "release_id": item.ReleaseID, "source_commit_id": item.SourceCommitID, "reason": input.Reason}})
		writeJSON(w, 200, item)
	}
}

func runDeployment(store deploymentStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, activity activityStore, item deployments.Deployment) {
	if _, e := store.Start(item.RepositoryID, item.ID); e != nil {
		return
	}
	env, e := store.GetEnvironment(item.RepositoryID, item.EnvironmentID)
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "environment unavailable")
		return
	}
	stages, e := deploymentStages(repositories, item.RepositoryID, item.SourceCommitID, env.Name)
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "invalid or missing .komodo/deployments.json rollout policy")
		return
	}
	secrets, e := store.Secrets(item.RepositoryID, item.EnvironmentID)
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "protected credentials unavailable")
		return
	}
	_, source, e := builds.OpenArtifact(item.RepositoryID, "release:"+item.ReleaseID, item.BuildRunID, item.ArtifactID)
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "artifact unavailable")
		return
	}
	defer source.Close()
	dir, e := os.MkdirTemp("", "komodo-deploy-")
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "workspace unavailable")
		return
	}
	defer os.RemoveAll(dir)
	artifactPath := filepath.Join(dir, filepath.Base(item.ArtifactPath))
	target, e := os.OpenFile(artifactPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if e == nil {
		_, e = io.Copy(target, source)
		target.Close()
	}
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "artifact materialization failed")
		return
	}
	baseEnv := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "TMPDIR=" + dir, "KOMODO_ARTIFACT_PATH=" + artifactPath, "KOMODO_ARTIFACT_SHA256=" + item.ArtifactSHA256, "KOMODO_RELEASE_ID=" + item.ReleaseID, "KOMODO_SOURCE_COMMIT=" + item.SourceCommitID}
	for k, v := range env.Configuration {
		baseEnv = append(baseEnv, k+"="+v)
	}
	for k, v := range secrets {
		baseEnv = append(baseEnv, k+"="+v)
	}
	if !runDeploymentCommand(store, item, dir, env.Command, baseEnv, secrets, 30*time.Minute) {
		completeDeploymentFailure(store, activity, item, "deployment command failed", "deployment.failed")
		return
	}
	for _, stage := range stages {
		if !waitForRollout(store, item) {
			return
		}
		store.Stage(item.RepositoryID, item.ID, stage.Name, "stage.started", "", "", "")
		stageEnv := append(baseEnv, "KOMODO_ROLLOUT_STAGE="+stage.Name)
		if stage.Command != "" && !runDeploymentCommand(store, item, dir, stage.Command, stageEnv, secrets, 10*time.Minute) {
			completeDeploymentFailure(store, activity, item, "rollout stage failed: "+stage.Name, "deployment.failed")
			return
		}
		healthy := true
		for _, signal := range stage.Health {
			if !waitForRollout(store, item) {
				return
			}
			store.Stage(item.RepositoryID, item.ID, stage.Name, "health.started", signal.Name, "", "")
			passed := runDeploymentCommand(store, item, dir, signal.Command, append(stageEnv, "KOMODO_HEALTH_SIGNAL="+signal.Name), secrets, time.Duration(signal.TimeoutSeconds)*time.Second)
			outcome := "passed"
			if !passed {
				outcome = "failed"
				healthy = false
			}
			store.Stage(item.RepositoryID, item.ID, stage.Name, "health.completed", signal.Name, outcome, "")
			if !healthy {
				break
			}
		}
		if !healthy {
			completeDeploymentFailure(store, activity, item, "health signal failed during "+stage.Name, "deployment.unhealthy")
			return
		}
		store.Stage(item.RepositoryID, item.ID, stage.Name, "stage.completed", "", "passed", "")
	}
	completed, _ := store.Complete(item.RepositoryID, item.ID, true, "rollout completed with healthy signals")
	recordDeploymentOutcome(activity, completed, "deployment.succeeded")
}

func runDeploymentCommand(store deploymentStore, item deployments.Deployment, dir, command string, environment []string, secrets map[string]string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = environment
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if e := cmd.Start(); e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "deployment command failed to start")
		return false
	}
	monitorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-monitorDone:
				return
			case <-ticker.C:
				current, getErr := store.GetDeployment(item.RepositoryID, item.ID)
				if getErr != nil || current.State == "canceled" || current.State == "failed" {
					cancel()
					return
				}
			}
		}
	}()
	done := make(chan struct{}, 2)
	capture := func(stream string, reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			for _, secret := range secrets {
				if secret != "" {
					line = strings.ReplaceAll(line, secret, "[REDACTED]")
				}
			}
			store.Log(item.RepositoryID, item.ID, stream, line)
		}
		done <- struct{}{}
	}
	go capture("stdout", stdout)
	go capture("stderr", stderr)
	err := cmd.Wait()
	close(monitorDone)
	<-done
	<-done
	return err == nil && ctx.Err() == nil
}

func waitForRollout(store deploymentStore, item deployments.Deployment) bool {
	for {
		current, err := store.GetDeployment(item.RepositoryID, item.ID)
		if err != nil {
			return false
		}
		switch current.State {
		case "running":
			return true
		case "paused":
			time.Sleep(200 * time.Millisecond)
		default:
			return false
		}
	}
}

func deploymentStages(repositories pullRequestRepositoryStore, repositoryID, commitID, environmentName string) ([]deployments.RolloutStage, error) {
	repository, err := repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return nil, err
	}
	commit, err := repository.ReadCommit(storage.ObjectID(commitID))
	if err != nil {
		return nil, err
	}
	entry, found, err := deploymentManifestEntry(repository, commit.Tree, []string{".komodo", "deployments.json"})
	if err != nil || !found || entry.Type != storage.BlobObject {
		return nil, deployments.ErrInvalid
	}
	object, err := repository.ReadObject(entry.ObjectID)
	if err != nil {
		return nil, err
	}
	return deployments.ParseManifest(object.Content, environmentName)
}

func deploymentManifestEntry(repository *storage.Repository, treeID storage.ObjectID, parts []string) (storage.TreeEntry, bool, error) {
	entries, err := repository.ReadTree(treeID)
	if err != nil {
		return storage.TreeEntry{}, false, err
	}
	for _, entry := range entries.Entries {
		if entry.Name != parts[0] {
			continue
		}
		if len(parts) == 1 {
			return entry, true, nil
		}
		if entry.Type != storage.TreeObject {
			return storage.TreeEntry{}, false, nil
		}
		return deploymentManifestEntry(repository, entry.ObjectID, parts[1:])
	}
	return storage.TreeEntry{}, false, nil
}

func recordDeploymentOutcome(activity activityStore, item deployments.Deployment, eventType string) {
	if activity == nil {
		return
	}
	_ = recordActivity(activity, activities.Input{RepositoryID: item.RepositoryID, ActorID: item.InitiatedByID, Type: eventType, Resource: activities.Resource{Type: "deployment", ID: item.ID}, Metadata: map[string]string{"state": item.State, "environment_id": item.EnvironmentID, "release_id": item.ReleaseID, "source_commit_id": item.SourceCommitID, "stage": item.CurrentStage}})
}

func completeDeploymentFailure(store deploymentStore, activity activityStore, item deployments.Deployment, message, eventType string) {
	current, err := store.GetDeployment(item.RepositoryID, item.ID)
	if err != nil || (current.State != "running" && current.State != "paused") {
		return
	}
	failed, err := store.Complete(item.RepositoryID, item.ID, false, message)
	if err == nil {
		recordDeploymentOutcome(activity, failed, eventType)
	}
}
