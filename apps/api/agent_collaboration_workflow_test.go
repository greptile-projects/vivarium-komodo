package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestDeveloperAndAgentCompleteCollaborationWorkflow proves that delegation is
// one public workflow, rather than a collection of independently tested
// handlers. Developer actions use JSON HTTP, and agent code publication uses a
// stock Git client plus the credential-bound worker API. The application is
// restarted mid-run before the worker publishes its result.
func TestDeveloperAndAgentCompleteCollaborationWorkflow(t *testing.T) {
	requireGit(t)
	type roots struct{ git, catalog, auth, pulls, proposals, sessions, activities, users string }
	r := roots{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	start := func() *httptest.Server {
		gitStorage, _ := storage.New(r.git)
		catalog, _ := repositories.New(r.catalog, gitStorage)
		credentials, _ := auth.New(r.auth)
		pulls, _ := pullrequests.New(r.pulls)
		proposalStore, _ := proposals.New(r.proposals)
		sessions, _ := changesessions.New(r.sessions)
		userStore, _ := users.New(r.users)
		activityStore, _ := activities.New(r.activities, userStore)
		mux := http.NewServeMux()
		registerRepositoriesHTTP(mux, catalog, credentials)
		registerPullRequestsHTTP(mux, pulls, proposalStore, catalog, credentials, activityStore)
		registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, activityStore)
		registerGitHTTP(mux, catalog, credentials)
		return httptest.NewServer(mux)
	}
	credentials, _ := auth.New(r.auth)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)

	server := start()
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"agent-loop","visibility":"private"}`, http.StatusCreated, &repository)
	// Catalog membership is itself part of the public product, but contributor
	// invitation is covered by the human workflow; use the boundary here to keep
	// this proof focused on delegation.
	gitStorage, _ := storage.New(r.git)
	catalog, _ := repositories.New(r.catalog, gitStorage)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	maintainerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, maintainerClone, "config", "user.name", "Maintainer")
	gitOutput(t, maintainerClone, "config", "user.email", "maintainer@example.com")
	if err := os.WriteFile(filepath.Join(maintainerClone, "README.md"), []byte("# Agent loop\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, maintainerClone, "add", "README.md")
	gitOutput(t, maintainerClone, "commit", "-m", "Initialize project")
	gitOutput(t, maintainerClone, "push", "-u", "origin", "main")

	developerClone := gitClone(t, remote(developerGit))
	gitOutput(t, developerClone, "config", "user.name", "Developer")
	gitOutput(t, developerClone, "config", "user.email", "developer@example.com")
	gitOutput(t, developerClone, "switch", "-c", "candidate/agent-docs")
	if err := os.WriteFile(filepath.Join(developerClone, "GUIDE.md"), []byte("# Guide\n\nDraft.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, developerClone, "add", "GUIDE.md")
	gitOutput(t, developerClone, "commit", "-m", "Start guide")
	seedRevision := gitOutput(t, developerClone, "rev-parse", "HEAD")
	gitOutput(t, developerClone, "push", "-u", "origin", "candidate/agent-docs")

	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", developer, `{"title":"Finish the agent guide","body":"Delegate the final documentation pass.","source_branch":"candidate/agent-docs","target_branch":"main"}`, http.StatusCreated, &pull)
	base := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID

	// A failed attempt remains attributable and loses its branch credential.
	var failedSession changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions", developer, `{}`, http.StatusCreated, &failedSession)
	failedRun, failedToken := delegateWorkflowRun(t, server.URL, base, failedSession.ID, developer, seedRevision)
	workerEvents := base + "/change-sessions/" + failedSession.ID + "/runs/" + failedRun.ID + "/events"
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"run.started","metadata":{"status":"Inspecting context"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"run.failed","metadata":{"error":"worker interrupted"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"agent.message","metadata":{"summary":"should be rejected"}}`, http.StatusUnauthorized, nil)

	// A new session is redirected by a collaborator, then survives a complete
	// server/store reopen while its worker credential remains usable.
	var session changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions", developer, `{}`, http.StatusCreated, &session)
	run, workerToken := delegateWorkflowRun(t, server.URL, base, session.ID, developer, seedRevision)
	runBase := base + "/change-sessions/" + session.ID + "/runs/" + run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", workerToken, `{"type":"run.started","metadata":{"status":"Editing the guide"}}`, http.StatusCreated, nil)
	for _, body := range []string{`{"type":"guidance","message":"Include the exact API test command."}`, `{"type":"pause"}`, `{"type":"resume"}`} {
		workflowJSON(t, server.URL, http.MethodPost, runBase+"/interventions", maintainer, body, http.StatusCreated, nil)
	}
	server.Close()
	server = start()
	defer server.Close()
	base = "/repositories/" + repository.ID + "/pull-requests/" + pull.ID
	runBase = base + "/change-sessions/" + session.ID + "/runs/" + run.ID
	remote = func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("agent", token)
		return value.String()
	}
	agentClone := gitClone(t, remote(workerToken))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "candidate/agent-docs")
	if err := os.WriteFile(filepath.Join(agentClone, "GUIDE.md"), []byte("# Guide\n\nRun `cd apps/api && go test ./...` before review.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, agentClone, "commit", "-am", "Finish guide as delegated")
	agentCommit := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "candidate/agent-docs")
	var publication struct {
		Run  changesessions.Run       `json:"run"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", workerToken, `{"summary":"Completed the requested guide.","checks":["cd apps/api && go test ./..."],"concerns":[]}`, http.StatusCreated, &publication)
	if publication.Run.State != changesessions.Succeeded || publication.Run.Publication == nil || publication.Run.Publication.CommitIDs[0] != agentCommit || publication.Pull.SourceCommitID != agentCommit {
		t.Fatalf("publication did not connect worker push to review: %#v", publication)
	}

	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &readiness)
	if !readiness.Ready || !readiness.CanMerge {
		t.Fatalf("delegated change not ready: %#v", readiness)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, "", http.StatusOK, &merged)
	if merged.Status != pullrequests.Merged || merged.MergedByID != "maintainer" {
		t.Fatalf("merge attribution: %#v", merged)
	}
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "GUIDE.md"), "# Guide\n\nRun `cd apps/api && go test ./...` before review.\n", 0)

	var restored changesessions.Session
	workflowJSON(t, server.URL, http.MethodGet, base+"/change-sessions/"+session.ID, maintainer, "", http.StatusOK, &restored)
	if restored.Runs[0].InitiatorID != "developer" || restored.Runs[0].Agent != "codex" || restored.Runs[0].RevisionID != seedRevision || restored.Runs[0].CredentialRevokedAt == nil || len(restored.Events) < 7 {
		t.Fatalf("reconnected session lost state or attribution: %#v", restored)
	}
}

func delegateWorkflowRun(t *testing.T, origin, pullBase, sessionID, actor, revision string) (changesessions.Run, string) {
	t.Helper()
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	body := `{"instructions":"Finish the guide and preserve existing context.","revision_id":"` + revision + `","context_paths":["GUIDE.md"],"working_branch":"candidate/agent-docs","agent":"codex"}`
	workflowJSON(t, origin, http.MethodPost, pullBase+"/change-sessions/"+sessionID+"/runs", actor, body, http.StatusCreated, &delegated)
	return delegated.Run, delegated.Credential.Token
}
