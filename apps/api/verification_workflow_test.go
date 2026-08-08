package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestVerificationRepairAndMergeWorkflow proves that verification, delegated
// repair, and merge form one public workflow. Human and agent publication use
// stock Git; every other transition and every piece of check evidence is read
// or written through the application HTTP contract.
func TestVerificationRepairAndMergeWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the verification workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	runs, _ := checkruns.New(t.TempDir())
	runner := checkruns.NewRunner(runs, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, runs)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil, runner)
	registerCheckRunsHTTP(mux, runs, runner, pulls, catalog, credentials, sessions, nil)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"verified-agent-loop","visibility":"private"}`, http.StatusCreated, &repository)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", maintainer, `{"branch":"main","checks":["guide"]}`, http.StatusOK, nil)

	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	maintainerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, maintainerClone, "config", "user.name", "Maintainer")
	gitOutput(t, maintainerClone, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, maintainerClone, "README.md", "# Verification workflow\n")
	gitOutput(t, maintainerClone, "add", "README.md")
	gitOutput(t, maintainerClone, "commit", "-m", "Initialize project")
	gitOutput(t, maintainerClone, "push", "-u", "origin", "main")

	developerClone := gitClone(t, remote(developerGit))
	gitOutput(t, developerClone, "config", "user.name", "Developer")
	gitOutput(t, developerClone, "config", "user.email", "developer@example.com")
	gitOutput(t, developerClone, "switch", "-c", "candidate/verified-guide")
	writeWorkflowFile(t, developerClone, "GUIDE.md", "# Guide\n\nDraft.\n")
	writeWorkflowFile(t, developerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"guide","command":"mkdir -p evidence; if grep -q verified GUIDE.md; then printf passed > evidence/guide.txt; else printf 'GUIDE.md must contain verified\\n' | tee evidence/guide.txt >&2; exit 1; fi","timeout_seconds":30,"artifacts":["evidence/guide.txt"]}]}`)
	gitOutput(t, developerClone, "add", "GUIDE.md", ".komodo/checks.json")
	gitOutput(t, developerClone, "commit", "-m", "Propose checked guide")
	failedRevision := gitOutput(t, developerClone, "rev-parse", "HEAD")
	gitOutput(t, developerClone, "push", "-u", "origin", "candidate/verified-guide")

	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", developer, `{"title":"Verify the guide","body":"Repair the repository-defined check before merge.","source_branch":"candidate/verified-guide","target_branch":"main"}`, http.StatusCreated, &pull)
	base := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID
	failed := waitForWorkflowCheck(t, server.URL, base, maintainer, failedRevision, checkruns.Failed)
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Fatalf("failed check outcome was not retained: %#v", failed)
	}
	var evidence struct {
		Items        []checkruns.Event `json:"items"`
		LastSequence int64             `json:"last_sequence"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/check-runs/"+failed.ID+"/events?after=0", maintainer, "", http.StatusOK, &evidence)
	if evidence.LastSequence < 5 || !workflowLogContains(evidence.Items, "must contain verified") {
		t.Fatalf("failed check evidence is incomplete: %#v", evidence)
	}
	failedArtifact := workflowArtifact(t, failed)
	assertWorkflowArtifact(t, server.URL, base, failed.ID, failedArtifact.ID, maintainer, "GUIDE.md must contain verified\n")

	var session changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/check-runs/"+failed.ID+"/change-session", developer, `{}`, http.StatusCreated, &session)
	if session.CheckFailure == nil || session.CheckFailure.RunID != failed.ID || len(session.CheckFailure.Artifacts) != 1 {
		t.Fatalf("repair session lost failed evidence: %#v", session)
	}
	run, workerToken := delegateVerificationRun(t, server.URL, base, session.ID, developer, failedRevision)
	runBase := base + "/change-sessions/" + session.ID + "/runs/" + run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", workerToken, `{"type":"run.started","metadata":{"status":"Repairing the failed guide check"}}`, http.StatusCreated, nil)

	agentClone := gitClone(t, remote(workerToken))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "candidate/verified-guide")
	writeWorkflowFile(t, agentClone, "GUIDE.md", "# Guide\n\nThis revision is verified.\n")
	gitOutput(t, agentClone, "commit", "-am", "Repair guide verification")
	passingRevision := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "candidate/verified-guide")
	var publication struct {
		Run  changesessions.Run       `json:"run"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", workerToken, `{"summary":"Repaired the failed guide check.","checks":["guide"],"concerns":[]}`, http.StatusCreated, &publication)
	if publication.Run.InitiatorID != "developer" || publication.Run.Publication == nil || publication.Pull.SourceCommitID != passingRevision {
		t.Fatalf("agent publication attribution is incomplete: %#v", publication)
	}
	passed := waitForWorkflowCheck(t, server.URL, base, maintainer, passingRevision, checkruns.Succeeded)

	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &readiness)
	if !readiness.Ready || !readiness.CanMerge || len(readiness.Checks.Requirements) != 1 || readiness.Checks.Requirements[0].RunID != passed.ID || readiness.Checks.CommitID != passingRevision {
		t.Fatalf("readiness did not select the repaired revision: %#v", readiness)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, "", http.StatusOK, &merged)
	if merged.Status != pullrequests.Merged || merged.MergedByID != "maintainer" {
		t.Fatalf("merge attribution: %#v", merged)
	}

	// Both attempts and the original diagnostic artifact remain public evidence
	// after merge; success never overwrites the failed revision's history.
	var retained struct {
		Items []checkruns.Run `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/check-runs", maintainer, "", http.StatusOK, &retained)
	if len(retained.Items) != 2 || retained.Items[0].ID != passed.ID || retained.Items[1].ID != failed.ID {
		t.Fatalf("check attempt history was not retained: %#v", retained.Items)
	}
	assertWorkflowArtifact(t, server.URL, base, failed.ID, failedArtifact.ID, maintainer, "GUIDE.md must contain verified\n")
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "GUIDE.md"), "# Guide\n\nThis revision is verified.\n", 0)
}

func waitForWorkflowCheck(t *testing.T, origin, base, actor, commit string, state checkruns.State) checkruns.Run {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var collection struct {
			Items []checkruns.Run `json:"items"`
		}
		workflowJSON(t, origin, http.MethodGet, base+"/check-runs", actor, "", http.StatusOK, &collection)
		for _, run := range collection.Items {
			if run.CommitID == commit && run.State == state {
				return run
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("check for %s did not reach %s", commit, state)
	return checkruns.Run{}
}

func delegateVerificationRun(t *testing.T, origin, base, sessionID, actor, revision string) (changesessions.Run, string) {
	t.Helper()
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	body := `{"instructions":"Use the captured check evidence to repair GUIDE.md.","revision_id":"` + revision + `","context_paths":["GUIDE.md",".komodo/checks.json"],"working_branch":"candidate/verified-guide","agent":"codex"}`
	workflowJSON(t, origin, http.MethodPost, base+"/change-sessions/"+sessionID+"/runs", actor, body, http.StatusCreated, &delegated)
	return delegated.Run, delegated.Credential.Token
}

func workflowArtifact(t *testing.T, run checkruns.Run) checkruns.Artifact {
	t.Helper()
	for _, event := range run.Events {
		if event.Artifact != nil {
			return *event.Artifact
		}
	}
	t.Fatal("check did not retain its declared artifact")
	return checkruns.Artifact{}
}

func assertWorkflowArtifact(t *testing.T, origin, base, runID, artifactID, actor, want string) {
	t.Helper()
	response := workflowRequest(t, origin, http.MethodGet, base+"/check-runs/"+runID+"/artifacts/"+artifactID, actor, "", nil)
	defer response.Body.Close()
	contents, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(contents) != want {
		t.Fatalf("artifact = %d %q, want 200 %q", response.StatusCode, contents, want)
	}
}

func workflowLogContains(events []checkruns.Event, fragment string) bool {
	for _, event := range events {
		if event.Type == "log" && strings.Contains(event.Message, fragment) {
			return true
		}
	}
	return false
}

func writeWorkflowFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}
