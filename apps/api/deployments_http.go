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

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
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
	GetDeployment(string, string) (deployments.Deployment, error)
	ListDeployments(string) ([]deployments.Deployment, error)
}

func registerDeploymentsHTTP(mux *http.ServeMux, store deploymentStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("GET /repositories/{repository}/environments", listEnvironments(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/environments", putEnvironment(store, repositories, credentials, false))
	mux.HandleFunc("PUT /repositories/{repository}/environments/{environment}", putEnvironment(store, repositories, credentials, true))
	mux.HandleFunc("GET /repositories/{repository}/deployments", listDeployments(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/deployments/{deployment}", getDeployment(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/deployments", createDeployment(store, releaseStore, builds, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/deployments/{deployment}/approvals", approveDeployment(store, repositories, credentials, builds))
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

func createDeployment(store deploymentStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
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
			go runDeployment(store, builds, item)
		}
		writeJSON(w, 201, item)
	}
}
func approveDeployment(store deploymentStore, repositories pullRequestRepositoryStore, credentials authStore, builds releaseBuildStore) http.HandlerFunc {
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
			go runDeployment(store, builds, item)
		}
		writeJSON(w, 200, item)
	}
}

func runDeployment(store deploymentStore, builds releaseBuildStore, item deployments.Deployment) {
	if _, e := store.Start(item.RepositoryID, item.ID); e != nil {
		return
	}
	env, e := store.GetEnvironment(item.RepositoryID, item.EnvironmentID)
	if e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "environment unavailable")
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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", env.Command)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "TMPDIR=" + dir, "KOMODO_ARTIFACT_PATH=" + artifactPath, "KOMODO_ARTIFACT_SHA256=" + item.ArtifactSHA256, "KOMODO_RELEASE_ID=" + item.ReleaseID, "KOMODO_SOURCE_COMMIT=" + item.SourceCommitID}
	for k, v := range env.Configuration {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	for k, v := range secrets {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if e = cmd.Start(); e != nil {
		store.Complete(item.RepositoryID, item.ID, false, "deployment command failed to start")
		return
	}
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
	e = cmd.Wait()
	<-done
	<-done
	message := "deployment completed"
	if ctx.Err() != nil {
		message = "deployment timed out"
	} else if e != nil {
		message = "deployment command failed"
	}
	store.Complete(item.RepositoryID, item.ID, e == nil && ctx.Err() == nil, message)
}
