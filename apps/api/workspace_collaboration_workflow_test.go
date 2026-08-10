package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestContextToMergedWorkspaceWorkflow proves that planned work can move
// through one shared human-agent environment into ordinary verified review and
// integration while the transient runtime and durable evidence have separate
// lifetimes. Every workflow action crosses a public HTTP or stock Git boundary.
func TestContextToMergedWorkspaceWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bwrap is required for the workspace collaboration workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	incidentStore, _ := incidents.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	organizationStore, _ := organizations.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	workspaceRunner := workspaces.NewRunner(workspaceStore, catalog)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: catalog, checks: checks, starter: checkRunner, proposals: plans}
	checkRunner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil, pulls, checkRunner)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks, queue)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, sessions, nil)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, catalog, credentials, plans, pulls, incidentStore, organizationStore, checkRunner)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	peer := issueAccess(t, credentials, "peer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	organization, err := organizationStore.Create("maintainer", "distributed-team", "Distributed team", "")
	if err != nil {
		t.Fatal(err)
	}
	organization, err = organizationStore.Invite(organization.ID, "maintainer", "peer")
	if err != nil {
		t.Fatal(err)
	}
	organization, err = organizationStore.Accept(organization.ID, "peer")
	if err != nil {
		t.Fatal(err)
	}
	_, agent, err := organizationStore.RegisterAgent(organization.ID, "maintainer", organizations.Agent{Slug: "pair-agent", Name: "Pair agent", Capabilities: []string{"workspace:edit", "workspace:execute"}, OperatorIDs: []string{"peer"}, Visibility: "internal"})
	if err != nil {
		t.Fatal(err)
	}

	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"shared-workspace","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err = catalog.TransferOwner(storage.ID(repository.ID), "user", "maintainer", "organization", organization.ID, "maintainer"); err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "peer"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["workspace"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/integration-queue", maintainer, `{"branch":"main","enabled":true,"concurrency":1,"failure_behavior":"pause"}`, http.StatusOK, nil)

	remoteURL, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
	remoteURL.User = url.UserPassword("git", maintainerGit)
	clone := gitClone(t, remoteURL.String())
	gitOutput(t, clone, "config", "user.name", "Maintainer")
	gitOutput(t, clone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, clone, "README.md", "# Shared workspace\n")
	writeWorkflowFile(t, clone, ".komodo/workspaces.json", `{"version":1,"tools":[{"name":"sh","version":"system"}],"dependencies":["repository snapshot"],"setup":["mkdir -p public && printf ready > .workspace-ready"],"ports":[{"number":3000,"label":"preview","path":"public"}],"resources":{"cpu_seconds":30,"memory_mb":256,"disk_mb":256,"setup_timeout_seconds":30}}`)
	writeWorkflowFile(t, clone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"workspace","command":"test \"$(cat feature.txt)\" = \"human and peer and agent\"","timeout_seconds":30}]}`)
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Define shared development environment")
	baseRevision := gitOutput(t, clone, "rev-parse", "HEAD")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	proposalBase := "/repositories/" + repository.ID + "/proposals"
	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, proposalBase, developer, `{"title":"Build together","body":"Develop and verify the feature in one reproducible environment."}`, http.StatusCreated, &proposal)
	planBase := proposalBase + "/" + proposal.ID + "/plan"
	var task proposals.Task
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks", maintainer, `{"title":"Implement shared feature","outcome":"feature.txt records the jointly developed result."}`, http.StatusCreated, &task)
	workflowJSON(t, server.URL, http.MethodPut, planBase+"/tasks/"+task.ID+"/assignment", maintainer, `{"kind":"human","assignee_id":"developer","mandate":"Pair with the team and publish verified work.","base_revision":"`+baseRevision+`"}`, http.StatusOK, &task)

	workspaceBase := "/repositories/" + repository.ID + "/workspaces"
	var workspace workspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase, developer, `{"revision":"`+baseRevision+`","source_context":{"type":"proposal_task","id":"`+task.ID+`","parent_id":"`+proposal.ID+`"}}`, http.StatusCreated, &workspace)
	workspace = waitForReadyWorkspace(t, server.URL, workspaceBase+"/"+workspace.ID, developer)
	itemBase := workspaceBase + "/" + workspace.ID
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/presence", developer, `{"surface":"files","path":"feature.txt"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/presence", peer, `{"surface":"discussion"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, itemBase+"/files", developer, `{"path":"feature.txt","content":"human"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/messages", peer, `{"message":"I will add the pairing handoff before the agent runs it."}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, itemBase+"/files", peer, `{"path":"feature.txt","content":"human and peer"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/controls", developer, `{"subject_id":"`+agent.ID+`","subject_kind":"approved_agent","mode":"execute","scopes":["files","terminal","preview"]}`, http.StatusOK, &workspace)
	grant := workspace.Controls[len(workspace.Controls)-1]
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/controls/"+grant.ID+"/interventions", peer, `{"action":"guide","message":"Append only the approved agent marker.","version":1}`, http.StatusOK, &workspace)
	workflowJSON(t, server.URL, http.MethodPut, itemBase+"/files", peer, `{"path":"feature.txt","content":"human and peer and agent"}`, http.StatusOK, nil)
	var command struct {
		Result workspaces.CommandResult `json:"result"`
	}
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/commands", peer, `{"command":"test \"$(cat feature.txt)\" = \"human and peer and agent\"","timeout_seconds":10}`, http.StatusOK, &command)

	var checkpoint workspaces.Checkpoint
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/checkpoints", peer, `{"summary":"Joint implementation passes locally","paths":["feature.txt"],"reproducibility":{"dependencies":["repository snapshot"],"commands":["test feature.txt"]}}`, http.StatusCreated, &checkpoint)
	workflowJSON(t, server.URL, http.MethodPut, itemBase+"/files", developer, `{"path":"feature.txt","content":"unfinished overwrite"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/suspend", developer, `{}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodGet, itemBase, peer, "", http.StatusOK, &workspace)
	if workspace.State != workspaces.Suspended || workspace.Context.ID != task.ID {
		t.Fatalf("suspended workspace lost context: %#v", workspace)
	}
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/resume", peer, `{}`, http.StatusOK, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/checkpoints/"+checkpoint.ID+"/restore", developer, `{}`, http.StatusConflict, nil)
	workflowJSON(t, server.URL, http.MethodPut, itemBase+"/files", developer, `{"path":"feature.txt","deleted":true}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/checkpoints/"+checkpoint.ID+"/restore", developer, `{}`, http.StatusOK, &workspace)

	var publication struct {
		Workspace workspaces.Workspace     `json:"workspace"`
		Pull      pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/checkpoints/"+checkpoint.ID+"/publication", developer, `{"branch":"work/shared-feature","target_branch":"main","title":"Implement shared feature","message":"Jointly authored in the reproducible workspace.","create_pull_request":true}`, http.StatusCreated, &publication)
	if publication.Pull.TaskID != task.ID || publication.Pull.WorkspaceID != workspace.ID || publication.Pull.CheckpointID != checkpoint.ID {
		t.Fatalf("publication lost intent or workspace provenance: %#v", publication.Pull)
	}
	landOrchestratedPull(t, server.URL, repository.ID, maintainer, publication.Pull)

	expires := time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339Nano)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/expiry", maintainer, `{"expires_at":"`+expires+`"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, itemBase+"/stop", maintainer, `{"reason":"merged work no longer needs compute","expire":true}`, http.StatusOK, &workspace)
	if workspace.State != workspaces.Expired || len(workspace.Checkpoints) != 1 || workspace.Checkpoints[0].Publication == nil {
		t.Fatalf("expiry discarded durable evidence: %#v", workspace)
	}
	var retained pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/pull-requests/"+publication.Pull.ID, peer, "", http.StatusOK, &retained)
	if retained.Status != pullrequests.Merged || retained.WorkspaceID != workspace.ID || retained.MergeCommitID == "" {
		t.Fatalf("merged trail is incomplete: %#v", retained)
	}
	var reconciled proposals.Plan
	workflowJSON(t, server.URL, http.MethodGet, planBase, peer, "", http.StatusOK, &reconciled)
	mergedTask := orchestrationTask(t, reconciled, task.ID)
	if mergedTask.Status != proposals.TaskMerged || len(mergedTask.Contributions) != 1 || mergedTask.Contributions[0].PullRequestID != retained.ID {
		t.Fatalf("merged workspace contribution did not reconcile its plan: %#v", mergedTask)
	}
	verified := gitClone(t, remoteURL.String())
	assertFile(t, filepath.Join(verified, "feature.txt"), "human and peer and agent", 0)
	if _, err := os.Stat(workspaceStore.Environment(workspace.ID)); !os.IsNotExist(err) {
		t.Fatalf("expired runtime still exists: %v", err)
	}
	if !strings.Contains(retained.Body, workspace.ID) {
		t.Fatalf("pull request does not explain workspace origin: %q", retained.Body)
	}
}

func waitForReadyWorkspace(t *testing.T, origin, path, token string) workspaces.Workspace {
	t.Helper()
	var item workspaces.Workspace
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		workflowJSON(t, origin, http.MethodGet, path, token, "", http.StatusOK, &item)
		if item.State == workspaces.Ready {
			return item
		}
		if item.State == workspaces.Failed {
			t.Fatalf("workspace setup failed: %#v", item.Events)
		}
	}
	t.Fatalf("workspace did not become ready: %#v", item)
	return item
}
