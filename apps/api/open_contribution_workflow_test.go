package main

import (
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
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestOpenContributionWorkflow proves the newcomer path as one public contract:
// discovery and application actions use HTTP, while all code publication uses
// an unmodified Git client. The newcomer is never made an upstream member.
func TestOpenContributionWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, checks)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil, runner)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, sessions, nil)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	newcomer := issueAccess(t, credentials, "newcomer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	newcomerGit := issueAccess(t, credentials, "newcomer", auth.Git, auth.GitRead, auth.GitWrite)
	var upstream struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"open-project","description":"Welcomes outside contributors","visibility":"public"}`, http.StatusCreated, &upstream)
	remote := func(id, token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + id)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	ownerClone := gitClone(t, remote(upstream.ID, maintainerGit))
	configureWorkflowGit(t, ownerClone, "Maintainer", "maintainer@example.test")
	writeWorkflowFile(t, ownerClone, "README.md", "# Open project\n")
	writeWorkflowFile(t, ownerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"contribution","command":"grep -q trusted CONTRIBUTING.md"}]}`)
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Open the project")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+upstream.ID+"/required-checks", maintainer, `{"branch":"main","checks":["contribution"]}`, http.StatusOK, nil)

	var discovery struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/public?q=outside", "", "", http.StatusOK, &discovery)
	if len(discovery.Items) != 1 || discovery.Items[0].ID != upstream.ID {
		t.Fatalf("public discovery = %#v", discovery)
	}
	if member, _ := catalog.IsCollaborator(storage.ID(upstream.ID), "newcomer"); member {
		t.Fatal("newcomer unexpectedly received upstream membership")
	}
	var fork struct {
		ID, UpstreamID string
		OwnerID        string `json:"owner_id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+upstream.ID+"/forks", newcomer, `{"name":"open-project-fork","visibility":"public"}`, http.StatusCreated, &fork)

	// Upstream moves after the fork; the fork owner imports it through the
	// explicit fast-forward sync before beginning independent work.
	writeWorkflowFile(t, ownerClone, "NEWS.md", "Upstream release\n")
	gitOutput(t, ownerClone, "add", "NEWS.md")
	gitOutput(t, ownerClone, "commit", "-m", "Publish upstream release")
	gitOutput(t, ownerClone, "push")
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+fork.ID+"/sync", newcomer, `{"branch":"main"}`, http.StatusOK, nil)
	forkClone := gitClone(t, remote(fork.ID, newcomerGit))
	configureWorkflowGit(t, forkClone, "New Contributor", "newcomer@example.test")
	gitOutput(t, forkClone, "switch", "-c", "contribution")
	writeWorkflowFile(t, forkClone, "CONTRIBUTING.md", "draft\n")
	gitOutput(t, forkClone, "add", "CONTRIBUTING.md")
	gitOutput(t, forkClone, "commit", "-m", "Propose outside contribution")
	gitOutput(t, forkClone, "push", "-u", "origin", "contribution")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+upstream.ID+"/pull-requests", newcomer, `{"title":"Outside contribution","source_repository_id":"`+fork.ID+`","source_branch":"contribution","target_branch":"main"}`, http.StatusCreated, &pull)
	base := "/repositories/" + upstream.ID + "/pull-requests/" + pull.ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/comments", maintainer, `{"body":"Please let the agent make this trusted."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPut, base+"/maintainer-modification", newcomer, `{"allowed":true}`, http.StatusOK, nil)
	var session changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions", maintainer, `{}`, http.StatusCreated, &session)
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token        string `json:"token"`
			RepositoryID string `json:"repository_id"`
			Branch       string `json:"branch"`
		} `json:"credential"`
	}
	body := `{"instructions":"Make the contribution trusted.","revision_id":"` + pull.SourceCommitID + `","context_paths":["CONTRIBUTING.md"],"working_branch":"contribution","agent":"codex"}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions/"+session.ID+"/runs", maintainer, body, http.StatusCreated, &delegated)
	if delegated.Credential.RepositoryID != fork.ID || delegated.Credential.Branch != "refs/heads/contribution" {
		t.Fatalf("agent authority crossed fork boundary: %#v", delegated.Credential)
	}
	runBase := base + "/change-sessions/" + session.ID + "/runs/" + delegated.Run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", delegated.Credential.Token, `{"type":"run.started","metadata":{"status":"Applying maintainer guidance"}}`, http.StatusCreated, nil)
	agentClone := gitClone(t, remote(fork.ID, delegated.Credential.Token))
	configureWorkflowGit(t, agentClone, "Codex Agent", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "contribution")
	writeWorkflowFile(t, agentClone, "CONTRIBUTING.md", "trusted outside contribution\n")
	gitOutput(t, agentClone, "commit", "-am", "Complete trusted contribution")
	revision := strings.TrimSpace(gitOutput(t, agentClone, "rev-parse", "HEAD"))
	gitOutput(t, agentClone, "push", "origin", "contribution")
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", delegated.Credential.Token, `{"summary":"Completed maintainer guidance.","checks":["contribution"],"concerns":[]}`, http.StatusCreated, nil)
	passed := waitForWorkflowCheck(t, server.URL, base, maintainer, revision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var ready readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &ready)
	if !ready.Ready || ready.Checks.Requirements[0].RunID != passed.ID {
		t.Fatalf("outside contribution not ready: %#v", ready)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, "", http.StatusOK, &merged)
	if merged.AuthorID != "newcomer" || merged.MergedByID != "maintainer" || merged.SourceRepositoryID != fork.ID {
		t.Fatalf("durable attribution = %#v", merged)
	}
	upstreamGit, _ := catalog.Open(storage.ID(upstream.ID))
	object, _ := upstreamGit.ReadObject(storage.ObjectID(merged.MergeCommitID))
	if !strings.Contains(string(object.Content), "Source-Repository: "+fork.ID) || !strings.Contains(string(object.Content), "Source-Commit: "+revision) {
		t.Fatalf("merge provenance missing: %s", object.Content)
	}
	if member, _ := catalog.IsCollaborator(storage.ID(upstream.ID), "newcomer"); member {
		t.Fatal("merge silently granted upstream membership")
	}
	verified := gitClone(t, server.URL+"/repositories/"+upstream.ID)
	assertFile(t, filepath.Join(verified, "CONTRIBUTING.md"), "trusted outside contribution\n", 0)
}

func configureWorkflowGit(t *testing.T, root, name, email string) {
	t.Helper()
	gitOutput(t, root, "config", "user.name", name)
	gitOutput(t, root, "config", "user.email", email)
}
