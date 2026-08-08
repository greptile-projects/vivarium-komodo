package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestReleaseDeliveryAndRecoveryWorkflow proves that merged collaboration can
// be built, promoted, diagnosed, rolled back, repaired by an agent, reviewed,
// and delivered again using only public HTTP and stock Git workflow surfaces.
func TestReleaseDeliveryAndRecoveryWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the release delivery workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	builds, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	runner := checkruns.NewRunner(builds, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, builds)
	registerReleasesHTTP(mux, releaseStore, builds, runner, pulls, catalog, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, builds, catalog, credentials, nil, sessions, pulls)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil, runner)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"continuous-delivery","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}

	ownerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, ownerClone, "config", "user.name", "Maintainer")
	gitOutput(t, ownerClone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, ownerClone, "README.md", "# Continuous delivery\n")
	writeWorkflowFile(t, ownerClone, ".komodo/releases.json", `{"version":1,"builds":[{"name":"package","command":"mkdir -p dist; cp health.txt dist/app","artifacts":["dist/app"]}]}`)
	writeWorkflowFile(t, ownerClone, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"rollout","health":[{"name":"service-ready","command":"grep -qx healthy \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	writeWorkflowFile(t, ownerClone, "health.txt", "healthy\n")
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Initialize releasable service")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")

	developerClone := gitClone(t, remote(developerGit))
	gitOutput(t, developerClone, "config", "user.name", "Human Developer")
	gitOutput(t, developerClone, "config", "user.email", "developer@example.com")
	gitOutput(t, developerClone, "switch", "-c", "delivery/human-feature")
	writeWorkflowFile(t, developerClone, "FEATURE.md", "Human-authored delivery intent.\n")
	gitOutput(t, developerClone, "add", "FEATURE.md")
	gitOutput(t, developerClone, "commit", "-m", "Add human feature")
	gitOutput(t, developerClone, "push", "-u", "origin", "delivery/human-feature")
	human := deliveryPull(t, server.URL, repository.ID, developer, maintainer, "Ship human feature", "delivery/human-feature")

	var v1 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.0.0","commit_id":"`+human.MergeCommitID+`","notes":"Known-good human collaboration."}`, http.StatusCreated, &v1)
	v1Build, v1Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v1.ID, maintainer)
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/environments", maintainer, `{"name":"production","position":1,"command":"printf 'deploy %s\\n' \"$KOMODO_ARTIFACT_SHA256\"","configuration":{"REGION":"primary"},"secrets":{"DEPLOY_TOKEN":"protected-value"},"required_approvals":1,"concurrency":1}`, http.StatusCreated, &environment)
	v1Deployment := promoteAndApprove(t, server.URL, repository.ID, developer, maintainer, environment.ID, v1.ID, v1Build.ID, v1Artifact.ID)
	if waitForDeployment(t, server.URL, repository.ID, v1Deployment.ID, maintainer, "succeeded").ArtifactSHA256 != v1Artifact.SHA256 {
		t.Fatal("known-good deployment lost immutable artifact identity")
	}

	gitOutput(t, developerClone, "switch", "main")
	gitOutput(t, developerClone, "pull", "--ff-only")
	gitOutput(t, developerClone, "switch", "-c", "delivery/regression")
	writeWorkflowFile(t, developerClone, "health.txt", "unhealthy\n")
	gitOutput(t, developerClone, "commit", "-am", "Introduce rollout regression")
	gitOutput(t, developerClone, "push", "-u", "origin", "delivery/regression")
	bad := deliveryPull(t, server.URL, repository.ID, developer, maintainer, "Ship risky follow-up", "delivery/regression")
	var v2 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.1.0","commit_id":"`+bad.MergeCommitID+`","prior_release_id":"`+v1.ID+`","notes":"Follow-up with retained contributor attribution."}`, http.StatusCreated, &v2)
	if len(v2.PullRequests) != 1 || v2.PullRequests[0].ID != bad.ID || len(v2.ContributorIDs) != 1 || v2.ContributorIDs[0] != "developer" {
		t.Fatalf("release attribution is incomplete: %#v", v2)
	}
	v2Build, v2Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v2.ID, maintainer)
	failedStart := promoteAndApprove(t, server.URL, repository.ID, developer, maintainer, environment.ID, v2.ID, v2Build.ID, v2Artifact.ID)
	failed := waitForDeployment(t, server.URL, repository.ID, failedStart.ID, maintainer, "failed")
	if !deliveryEvent(failed.Events, "health.completed", "failed") {
		t.Fatalf("failed rollout retained no health evidence: %#v", failed.Events)
	}

	var rollback struct {
		Deployment  deployments.Deployment `json:"deployment"`
		KnownGoodID string                 `json:"known_good_deployment_id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+failed.ID+"/recovery", developer, `{"action":"rollback"}`, http.StatusCreated, &rollback)
	if rollback.KnownGoodID != v1Deployment.ID || rollback.Deployment.RecoveryOfID != failed.ID {
		t.Fatalf("rollback provenance is incomplete: %#v", rollback)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+rollback.Deployment.ID+"/approvals", maintainer, `{}`, http.StatusOK, nil)
	waitForDeployment(t, server.URL, repository.ID, rollback.Deployment.ID, maintainer, "succeeded")

	var repair struct {
		Pull       pullrequests.PullRequest `json:"pull_request"`
		Session    changesessions.Session   `json:"session"`
		Run        changesessions.Run       `json:"run"`
		Credential struct {
			Token  string `json:"token"`
			Branch string `json:"branch"`
		} `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+failed.ID+"/recovery", developer, `{"action":"repair","instructions":"Restore the repository health signal without deployment authority.","context_paths":["health.txt"]}`, http.StatusCreated, &repair)
	if !repair.Pull.Draft || repair.Session.DeploymentFailure == nil || repair.Session.DeploymentFailure.DeploymentID != failed.ID {
		t.Fatalf("repair did not retain redacted failure context: %#v", repair)
	}
	agentClone := gitClone(t, remote(repair.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	agentBranch := strings.TrimPrefix(repair.Credential.Branch, "refs/heads/")
	gitOutput(t, agentClone, "switch", agentBranch)
	writeWorkflowFile(t, agentClone, "health.txt", "healthy\n")
	gitOutput(t, agentClone, "commit", "-am", "Repair rollout health")
	repairedRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", agentBranch)
	runBase := "/repositories/" + repository.ID + "/pull-requests/" + repair.Pull.ID + "/change-sessions/" + repair.Session.ID + "/runs/" + repair.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", repair.Credential.Token, `{"type":"run.started","metadata":{"status":"Repairing the failed rollout"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", repair.Credential.Token, `{"summary":"Restored the health contract.","checks":[],"concerns":[]}`, http.StatusCreated, nil)
	pullBase := "/repositories/" + repository.ID + "/pull-requests/" + repair.Pull.ID
	var reviewable pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/request-review", developer, `{}`, http.StatusOK, &reviewable)
	if reviewable.Draft || reviewable.SourceCommitID != repairedRevision {
		t.Fatalf("agent repair did not enter review at its published revision: %#v", reviewable)
	}
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var mergedRepair pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", maintainer, `{}`, http.StatusOK, &mergedRepair)

	var v3 releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", maintainer, `{"version":"v1.1.1","commit_id":"`+mergedRepair.MergeCommitID+`","prior_release_id":"`+v2.ID+`","notes":"Agent repair reviewed and delivered."}`, http.StatusCreated, &v3)
	v3Build, v3Artifact := waitForReleaseArtifact(t, server.URL, repository.ID, v3.ID, maintainer)
	corrected := promoteAndApprove(t, server.URL, repository.ID, developer, maintainer, environment.ID, v3.ID, v3Build.ID, v3Artifact.ID)
	corrected = waitForDeployment(t, server.URL, repository.ID, corrected.ID, maintainer, "succeeded")
	if corrected.SourceCommitID != mergedRepair.MergeCommitID || corrected.RecoveryOfID != "" || v3.PriorReleaseID != v2.ID {
		t.Fatalf("corrected delivery trail is incomplete: release=%#v deployment=%#v", v3, corrected)
	}
}

func deliveryPull(t *testing.T, origin, repository, author, maintainer, title, branch string) pullrequests.PullRequest {
	t.Helper()
	var pull pullrequests.PullRequest
	workflowJSON(t, origin, http.MethodPost, "/repositories/"+repository+"/pull-requests", author, `{"title":"`+title+`","source_branch":"`+branch+`","target_branch":"main"}`, http.StatusCreated, &pull)
	base := "/repositories/" + repository + "/pull-requests/" + pull.ID
	workflowJSON(t, origin, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, origin, http.MethodPost, base+"/merge", maintainer, `{}`, http.StatusOK, &pull)
	return pull
}

func waitForReleaseArtifact(t *testing.T, origin, repository, release, actor string) (checkruns.Run, checkruns.Artifact) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var attestation releaseAttestation
		workflowJSON(t, origin, http.MethodGet, "/repositories/"+repository+"/releases/"+release+"/attestation", actor, "", http.StatusOK, &attestation)
		if attestation.Verified {
			for _, run := range attestation.Attempts {
				for _, event := range run.Events {
					if event.Artifact != nil {
						return run, *event.Artifact
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("release did not produce a verified artifact")
	return checkruns.Run{}, checkruns.Artifact{}
}

func promoteAndApprove(t *testing.T, origin, repository, initiator, approver, environment, release, run, artifact string) deployments.Deployment {
	t.Helper()
	var item deployments.Deployment
	body := `{"environment_id":"` + environment + `","release_id":"` + release + `","build_run_id":"` + run + `","artifact_id":"` + artifact + `"}`
	workflowJSON(t, origin, http.MethodPost, "/repositories/"+repository+"/deployments", initiator, body, http.StatusCreated, &item)
	workflowJSON(t, origin, http.MethodPost, "/repositories/"+repository+"/deployments/"+item.ID+"/approvals", approver, `{}`, http.StatusOK, &item)
	return item
}

func waitForDeployment(t *testing.T, origin, repository, deployment, actor, state string) deployments.Deployment {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var item deployments.Deployment
		workflowJSON(t, origin, http.MethodGet, "/repositories/"+repository+"/deployments/"+deployment, actor, "", http.StatusOK, &item)
		if item.State == state {
			return item
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("deployment %s did not reach %s", deployment, state)
	return deployments.Deployment{}
}

func deliveryEvent(events []deployments.Event, kind, outcome string) bool {
	for _, event := range events {
		if event.Type == kind && event.Outcome == outcome {
			return true
		}
	}
	return false
}
