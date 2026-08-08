package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestParallelIntegrationQueueWorkflow proves the complete queue through its
// public HTTP and stock-Git surfaces. Independently reviewed human and agent
// changes enter together, every candidate is reverified after the target
// advances, a newly failing change is isolated, and compatible later work
// continues without a maintainer racing branch updates.
func TestParallelIntegrationQueueWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the integration queue workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	runs, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	runner := checkruns.NewRunner(runs, catalog)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: catalog, checks: runs, starter: runner}
	runner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, runs, queue)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil, runner)
	registerCheckRunsHTTP(mux, runs, runner, pulls, catalog, credentials, sessions, nil)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	contributor := issueAccess(t, credentials, "contributor", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorGit := issueAccess(t, credentials, "contributor", auth.Git, auth.GitRead, auth.GitWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"parallel-integration","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "contributor"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["integration"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/integration-queue", maintainer, `{"branch":"main","enabled":true,"concurrency":3,"failure_behavior":"remove"}`, http.StatusOK, nil)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	ownerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, ownerClone, "config", "user.name", "Maintainer")
	gitOutput(t, ownerClone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, ownerClone, "README.md", "# Parallel integration\n")
	// Each source passes alone. After the first change lands, the fragile
	// candidate fails only in the evolved target state, while the agent change
	// remains compatible. Sleeping gives all three entries time to queue before
	// the first candidate completes.
	writeWorkflowFile(t, ownerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"integration","command":"sleep 2; if [ -f human.txt ] && [ -f fragile.txt ]; then echo incompatible combination >&2; exit 1; fi","timeout_seconds":30}]}`)
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Initialize protected project")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")

	work := gitClone(t, remote(contributorGit))
	gitOutput(t, work, "config", "user.name", "Human Contributor")
	gitOutput(t, work, "config", "user.email", "contributor@example.com")
	makeBranch := func(branch, file, contents string) {
		gitOutput(t, work, "switch", "main")
		gitOutput(t, work, "switch", "-c", branch)
		writeWorkflowFile(t, work, file, contents)
		gitOutput(t, work, "add", file)
		gitOutput(t, work, "commit", "-m", "Prepare "+branch)
		gitOutput(t, work, "push", "-u", "origin", branch)
	}
	makeBranch("human-compatible", "human.txt", "human contribution\n")
	makeBranch("human-fragile", "fragile.txt", "independently valid\n")
	makeBranch("agent-compatible", "agent-plan.txt", "agent work requested\n")

	openPull := func(title, branch string) pullrequests.PullRequest {
		var pull pullrequests.PullRequest
		workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", contributor, `{"title":"`+title+`","source_branch":"`+branch+`","target_branch":"main"}`, http.StatusCreated, &pull)
		return pull
	}
	first := openPull("Human compatible change", "human-compatible")
	fragile := openPull("Human change sensitive to target", "human-fragile")
	agentPull := openPull("Agent compatible change", "agent-compatible")

	// Publish the third pull request through a credential-bound agent run so
	// the queue ultimately lands both human and agent-attributed work.
	agentBase := "/repositories/" + repository.ID + "/pull-requests/" + agentPull.ID
	var session changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, agentBase+"/change-sessions", contributor, `{}`, http.StatusCreated, &session)
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	delegateBody := `{"instructions":"Complete the requested compatible agent change.","revision_id":"` + agentPull.SourceCommitID + `","context_paths":["agent-plan.txt"],"working_branch":"agent-compatible","agent":"codex"}`
	workflowJSON(t, server.URL, http.MethodPost, agentBase+"/change-sessions/"+session.ID+"/runs", contributor, delegateBody, http.StatusCreated, &delegated)
	agentClone := gitClone(t, remote(delegated.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "agent-compatible")
	writeWorkflowFile(t, agentClone, "agent.txt", "agent contribution\n")
	gitOutput(t, agentClone, "add", "agent.txt")
	gitOutput(t, agentClone, "commit", "-m", "Complete agent contribution")
	agentRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "agent-compatible")
	var publication struct {
		Run  changesessions.Run       `json:"run"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	runBase := agentBase + "/change-sessions/" + session.ID + "/runs/" + delegated.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", delegated.Credential.Token, `{"type":"run.started","metadata":{"status":"Completing queued change"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", delegated.Credential.Token, `{"summary":"Completed the compatible queued change.","checks":["integration"],"concerns":[]}`, http.StatusCreated, &publication)
	if publication.Run.InitiatorID != "contributor" || publication.Run.Agent != "codex" || publication.Pull.SourceCommitID != agentRevision {
		t.Fatalf("agent publication lost attribution: %#v", publication)
	}
	agentPull = publication.Pull

	for _, pull := range []pullrequests.PullRequest{first, fragile, agentPull} {
		base := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID
		waitForWorkflowCheck(t, server.URL, base, maintainer, pull.SourceCommitID, checkruns.Succeeded)
		workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	}
	for _, pull := range []pullrequests.PullRequest{first, fragile, agentPull} {
		workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID+"/queue", maintainer, "", http.StatusCreated, nil)
	}

	entries := waitForQueueOutcomes(t, server.URL, repository.ID, maintainer, map[string]string{
		first.ID: "merged", fragile.ID: "removed", agentPull.ID: "merged",
	})
	assertQueueEvidence(t, entries[first.ID], "maintainer", "merged", "", 1)
	assertQueueEvidence(t, entries[fragile.ID], "maintainer", "removed", "checks_failed", 2)
	assertQueueEvidence(t, entries[agentPull.ID], "maintainer", "merged", "", 2)
	if !entries[fragile.ID].History[0].Checks.Satisfied || entries[fragile.ID].History[1].Checks.Satisfied {
		t.Fatalf("superseded success and evolved-target failure were not both retained: %#v", entries[fragile.ID].History)
	}

	var fragilePull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/pull-requests/"+fragile.ID, maintainer, "", http.StatusOK, &fragilePull)
	if fragilePull.Status != pullrequests.Open {
		t.Fatalf("isolated pull request was closed: %#v", fragilePull)
	}
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "human.txt"), "human contribution\n", 0)
	assertFile(t, filepath.Join(verified, "agent.txt"), "agent contribution\n", 0)
	if _, err := os.Stat(filepath.Join(verified, "fragile.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed queued change reached the target: %v", err)
	}
}

func waitForQueueOutcomes(t *testing.T, origin, repositoryID, actor string, want map[string]string) map[string]integrationQueueEntryResponse {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		var collection struct {
			Items []integrationQueueEntryResponse `json:"items"`
		}
		workflowJSON(t, origin, http.MethodGet, "/repositories/"+repositoryID+"/integration-queue/entries?branch=main", actor, "", http.StatusOK, &collection)
		found := map[string]integrationQueueEntryResponse{}
		for _, entry := range collection.Items {
			found[entry.PullRequestID] = entry
		}
		complete := true
		for pullID, state := range want {
			complete = complete && found[pullID].State == state
		}
		if complete {
			return found
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("integration queue did not reach the expected outcomes")
	return nil
}

func assertQueueEvidence(t *testing.T, entry integrationQueueEntryResponse, enqueuedBy, state, reason string, generations int) {
	t.Helper()
	if entry.EnqueuedByID != enqueuedBy || entry.State != state || entry.Reason != reason || entry.CompletedAt == nil || len(entry.History) != generations {
		t.Fatalf("queue evidence is incomplete: %#v", entry)
	}
	if len(entry.Events) < 2 || entry.Events[0].Action != "enqueued" || entry.Events[0].ActorID != enqueuedBy || entry.Events[len(entry.Events)-1].ActorID != "integration-queue" {
		t.Fatalf("queue decision attribution is incomplete: %#v", entry.Events)
	}
}
