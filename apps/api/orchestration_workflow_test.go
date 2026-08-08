package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestIdeaToIntegratedChangeWorkflow proves that a discussed idea can become
// dependent human and agent work, reviewed contributions, and safely ordered
// integration using only public HTTP, worker credentials, and stock Git.
func TestIdeaToIntegratedChangeWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the orchestration workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	runs, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	runner := checkruns.NewRunner(runs, catalog)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: catalog, checks: runs, starter: runner, proposals: plans}
	runner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil, pulls, runner)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, runner, runs, queue)
	registerCheckRunsHTTP(mux, runs, runner, pulls, catalog, credentials, sessions, nil)
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
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"orchestrated-delivery","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["delivery"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/integration-queue", maintainer, `{"branch":"main","enabled":true,"concurrency":1,"failure_behavior":"pause"}`, http.StatusOK, nil)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	ownerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, ownerClone, "config", "user.name", "Maintainer")
	gitOutput(t, ownerClone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, ownerClone, "README.md", "# Orchestrated delivery\n")
	writeWorkflowFile(t, ownerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"delivery","command":"test -f README.md","timeout_seconds":30}]}`)
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Initialize delivery")
	baseRevision := gitOutput(t, ownerClone, "rev-parse", "HEAD")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")

	proposalBase := "/repositories/" + repository.ID + "/proposals"
	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, proposalBase, developer, `{"title":"Ship a two-part guide","body":"Add a human-authored foundation, then an agent-authored example."}`, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, proposalBase+"/"+proposal.ID+"/comments", maintainer, `{"body":"Land the foundation first and verify both changes."}`, http.StatusCreated, nil)
	planBase := proposalBase + "/" + proposal.ID + "/plan"
	var human proposals.Task
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks", maintainer, `{"title":"Write foundation","outcome":"FOUNDATION.md documents the agreed workflow."}`, http.StatusCreated, &human)
	var agent proposals.Task
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks", maintainer, `{"title":"Add example","outcome":"EXAMPLE.md builds on the merged foundation.","depends_on":["`+human.ID+`"]}`, http.StatusCreated, &agent)
	assign := func(taskID, body string) proposals.Task {
		var task proposals.Task
		workflowJSON(t, server.URL, http.MethodPut, planBase+"/tasks/"+taskID+"/assignment", maintainer, body, http.StatusOK, &task)
		return task
	}
	human = assign(human.ID, `{"kind":"human","assignee_id":"developer","mandate":"Write the agreed foundation.","base_revision":"`+baseRevision+`"}`)

	work := gitClone(t, remote(developerGit))
	gitOutput(t, work, "config", "user.name", "Human Developer")
	gitOutput(t, work, "config", "user.email", "developer@example.com")
	gitOutput(t, work, "switch", "-c", "work/foundation")
	writeWorkflowFile(t, work, "FOUNDATION.md", "# Foundation\n\nHuman-owned context.\n")
	gitOutput(t, work, "add", "FOUNDATION.md")
	gitOutput(t, work, "commit", "-m", "Write foundation")
	gitOutput(t, work, "push", "-u", "origin", "work/foundation")
	var humanPublication struct {
		Task proposals.Task           `json:"task"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, planBase+"/tasks/"+human.ID+"/contributions", developer, `{"expected_assignment_id":"`+human.Assignment.ID+`","title":"Write foundation","source_branch":"work/foundation","target_branch":"main"}`, http.StatusCreated, &humanPublication)
	landOrchestratedPull(t, server.URL, repository.ID, maintainer, humanPublication.Pull)

	var plan proposals.Plan
	workflowJSON(t, server.URL, http.MethodGet, planBase, maintainer, "", http.StatusOK, &plan)
	agent = orchestrationTask(t, plan, agent.ID)
	if !agent.Ready || len(agent.BlockedBy) != 0 {
		t.Fatalf("dependent task did not become ready after queued merge: %#v", agent)
	}
	mergedBase := orchestrationTask(t, plan, human.ID).Contributions[0].SourceCommitID
	agent = assign(agent.ID, `{"kind":"agent","assignee_id":"codex","mandate":"Add the example using the merged foundation.","base_revision":"`+mergedBase+`"}`)
	var started struct {
		Task       proposals.Task                 `json:"task"`
		Session    changesessions.Session         `json:"session"`
		Run        changesessions.Run             `json:"run"`
		Credential struct{ Token, Branch string } `json:"credential"`
	}
	taskBase := planBase + "/tasks/" + agent.ID
	workflowJSON(t, server.URL, http.MethodPost, taskBase+"/change-sessions", maintainer, `{"expected_assignment_id":"`+agent.Assignment.ID+`"}`, http.StatusCreated, &started)
	runBase := taskBase + "/change-sessions/" + started.Session.ID + "/runs/" + started.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/interventions", developer, `{"type":"guidance","message":"Keep the example concise."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", started.Credential.Token, `{"type":"run.started","metadata":{"status":"Following collaborator guidance"}}`, http.StatusCreated, nil)
	agentClone := gitClone(t, remote(started.Credential.Token))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	agentBranch := strings.TrimPrefix(started.Credential.Branch, "refs/heads/")
	gitOutput(t, agentClone, "switch", agentBranch)
	assertFile(t, filepath.Join(agentClone, "FOUNDATION.md"), "# Foundation\n\nHuman-owned context.\n", 0)
	writeWorkflowFile(t, agentClone, "EXAMPLE.md", "# Example\n\nAgent-owned follow-up.\n")
	gitOutput(t, agentClone, "add", "EXAMPLE.md")
	gitOutput(t, agentClone, "commit", "-m", "Add example")
	agentRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", agentBranch)
	var agentPublication struct {
		Task proposals.Task           `json:"task"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, taskBase+"/contributions", started.Credential.Token, `{"expected_assignment_id":"`+agent.Assignment.ID+`","session_id":"`+started.Session.ID+`","title":"Add example","target_branch":"main"}`, http.StatusCreated, &agentPublication)
	if agentPublication.Pull.SourceCommitID != agentRevision || agentPublication.Pull.ChangeSessionID != started.Session.ID {
		t.Fatalf("agent contribution lost execution provenance: %#v", agentPublication)
	}
	landOrchestratedPull(t, server.URL, repository.ID, maintainer, agentPublication.Pull)

	workflowJSON(t, server.URL, http.MethodGet, planBase, maintainer, "", http.StatusOK, &plan)
	if orchestrationTask(t, plan, human.ID).Status != proposals.TaskMerged || orchestrationTask(t, plan, agent.ID).Status != proposals.TaskMerged {
		t.Fatalf("integrated plan is incomplete: %#v", plan.Tasks)
	}
	var restored changesessions.Session
	workflowJSON(t, server.URL, http.MethodGet, taskBase+"/change-sessions/"+started.Session.ID, maintainer, "", http.StatusOK, &restored)
	if restored.InitiatorID != "maintainer" || len(restored.Events) < 4 || restored.ContributionPullRequestID != agentPublication.Pull.ID {
		t.Fatalf("session attribution is incomplete: %#v", restored)
	}
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "FOUNDATION.md"), "# Foundation\n\nHuman-owned context.\n", 0)
	assertFile(t, filepath.Join(verified, "EXAMPLE.md"), "# Example\n\nAgent-owned follow-up.\n", 0)
}

func landOrchestratedPull(t *testing.T, origin, repositoryID, maintainer string, pull pullrequests.PullRequest) {
	t.Helper()
	base := "/repositories/" + repositoryID + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, origin, base, maintainer, pull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, origin, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, origin, http.MethodPost, base+"/queue", maintainer, "", http.StatusCreated, nil)
	waitForQueueOutcomes(t, origin, repositoryID, maintainer, map[string]string{pull.ID: "merged"})
}

func orchestrationTask(t *testing.T, plan proposals.Plan, id string) proposals.Task {
	t.Helper()
	for _, task := range plan.Tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s missing from plan", id)
	return proposals.Task{}
}
