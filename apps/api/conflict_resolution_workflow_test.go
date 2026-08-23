package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestCollaborativeConflictResolutionWorkflow proves the full public-HTTP and
// stock-Git path from independently reviewed competing changes to one reviewed
// two-input result. Failed and rejected agent work, revoked access, stale
// inputs, repeated queue conflicts, exact verification, and final queue history
// remain visible instead of being replaced by a private rebase.
func TestCollaborativeConflictResolutionWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for conflict reconciliation workspaces")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	workspaceRunner := workspaces.NewRunner(workspaceStore, catalog)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: catalog, checks: checks, starter: checkRunner}
	checkRunner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, checkRunner, checks, queue)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, nil, nil)
	registerWorkspacesHTTP(mux, workspaceStore, workspaceRunner, catalog, credentials, nil, pulls, nil, checkRunner)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	alice := issueAccess(t, credentials, "alice", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	bob := issueAccess(t, credentials, "bob", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	bobGit := issueAccess(t, credentials, "bob", auth.Git, auth.GitRead, auth.GitWrite)
	agent := issueAccess(t, credentials, "resolution-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"conflict-resolution","visibility":"private"}`, http.StatusCreated, &repository)
	for _, id := range []string{"alice", "bob", "resolution-agent"} {
		if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), id); err != nil {
			t.Fatal(err)
		}
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", owner, `{"branch":"main","checks":["combined"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/integration-queue", owner, `{"branch":"main","enabled":true,"concurrency":1,"failure_behavior":"remove"}`, http.StatusOK, nil)

	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Maintainer")
	gitOutput(t, work, "config", "user.email", "maintainer@example.test")
	writeWorkflowFile(t, work, "contract.txt", "base\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"combined","command":"sleep 1; test \"$(cat contract.txt)\" = \"source\" || test \"$(cat contract.txt)\" = \"target\" || test \"$(cat contract.txt)\" = \"both\"","timeout_seconds":20}]}`)
	writeWorkflowFile(t, work, ".komodo/workspaces.json", `{"version":1,"tools":[{"name":"sh","version":"system"}],"dependencies":["repository snapshot"],"setup":["true"],"commands":[{"name":"contract compatibility","command":"test \"$(cat contract.txt)\" = \"both\""},{"name":"conflict acceptance","command":"test -f source.go && test -f target.go"}],"resources":{"cpu_seconds":20,"memory_mb":128,"disk_mb":128,"setup_timeout_seconds":20}}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Define the shared contract")
	gitOutput(t, work, "push", "-u", "origin", "main")

	// Alice and Bob branch independently from the same public base.
	gitOutput(t, work, "switch", "-c", "alice-target")
	writeWorkflowFile(t, work, "contract.txt", "target\n")
	writeWorkflowFile(t, work, "target.go", "package contract\nconst SharedIntent = \"target\"\n")
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Preserve target callers")
	gitOutput(t, work, "push", "-u", "origin", "alice-target")
	gitOutput(t, work, "switch", "main")
	gitOutput(t, work, "switch", "-c", "bob-source")
	writeWorkflowFile(t, work, "contract.txt", "source\n")
	writeWorkflowFile(t, work, "source.go", "package contract\nconst SharedIntent = \"source\"\n")
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Preserve source callers")
	gitOutput(t, work, "push", "-u", "origin", "bob-source")

	open := func(actor, title, body, branch string) pullrequests.PullRequest {
		var pull pullrequests.PullRequest
		payload, _ := json.Marshal(map[string]string{"title": title, "body": body, "source_branch": branch, "target_branch": "main"})
		workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", actor, string(payload), http.StatusCreated, &pull)
		return pull
	}
	alicePull := open(alice, "Keep target compatibility", "Acceptance: target callers continue to receive target behavior.", "alice-target")
	staleBobPull := open(bob, "Keep source compatibility", "Acceptance: source callers remain supported without discarding target setup.", "bob-source")
	repeatedBobPull := open(bob, "Keep source compatibility, retry", "Acceptance: repeat the same source intent through the queue without losing its first attempt.", "bob-source")
	aliceBase := "/repositories/" + repository.ID + "/pull-requests/" + alicePull.ID
	waitForWorkflowCheck(t, server.URL, aliceBase, owner, alicePull.SourceCommitID, checkruns.Succeeded)
	staleBobBase := "/repositories/" + repository.ID + "/pull-requests/" + staleBobPull.ID
	repeatedBobBase := "/repositories/" + repository.ID + "/pull-requests/" + repeatedBobPull.ID
	waitForWorkflowCheck(t, server.URL, staleBobBase, owner, staleBobPull.SourceCommitID, checkruns.Succeeded)
	waitForWorkflowCheck(t, server.URL, repeatedBobBase, owner, repeatedBobPull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, aliceBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, staleBobBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, repeatedBobBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, aliceBase+"/queue", owner, "", http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, staleBobBase+"/queue", owner, "", http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, repeatedBobBase+"/queue", owner, "", http.StatusCreated, nil)
	waitForQueueOutcomes(t, server.URL, repository.ID, owner, map[string]string{alicePull.ID: "merged", staleBobPull.ID: "removed", repeatedBobPull.ID: "removed"})
	writeWorkflowFile(t, work, "source-notes.txt", "clarify source acceptance\n")
	gitOutput(t, work, "add", "source-notes.txt")
	gitOutput(t, work, "commit", "-m", "Clarify source acceptance concurrently")
	gitOutput(t, work, "push", "origin", "bob-source")
	var stale conflictAnalysis
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/pull-requests/"+staleBobPull.ID+"/conflicts", bob, "", http.StatusOK, &stale)
	if !stale.Stale || !stale.Source.Revision.Stale || !stale.Target.Revision.Stale {
		t.Fatalf("concurrent source and target movement was not retained as stale: %#v", stale)
	}

	// Bob's already-published branch now conflicts with the reviewed queued
	// result. Both queue attempts remain retained before a fresh exact pull is
	// opened against the evolved target.
	bobPull := open(bob, "Keep source compatibility", "Acceptance: source callers remain supported without discarding target setup.", "bob-source")
	bobBase := "/repositories/" + repository.ID + "/pull-requests/" + bobPull.ID
	workflowJSON(t, server.URL, http.MethodPut, bobBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	var analysis conflictAnalysis
	workflowJSON(t, server.URL, http.MethodGet, bobBase+"/conflicts", bob, "", http.StatusOK, &analysis)
	hasText, hasSemantic := false, false
	for _, item := range analysis.Conflicts {
		hasText = hasText || item.Kind == "textual"
		hasSemantic = hasSemantic || item.Kind == "semantic"
	}
	if !analysis.Complete || analysis.Stale || !hasText || !hasSemantic || analysis.Source.Intent.OwnerID != "bob" {
		t.Fatalf("competing intent was not explained: %#v", analysis)
	}

	var workspace workspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, bobBase+"/conflicts/workspace", owner, "{}", http.StatusCreated, &workspace)
	workspace = waitForReadyWorkspace(t, server.URL, "/repositories/"+repository.ID+"/workspaces/"+workspace.ID, owner)
	workspaceBase := "/repositories/" + repository.ID + "/workspaces/" + workspace.ID
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/presence", bob, `{"surface":"files","path":"contract.txt"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/presence", owner, `{"surface":"discussion"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/controls", owner, `{"subject_id":"resolution-agent","subject_kind":"human","mode":"execute","scopes":["files","terminal"]}`, http.StatusOK, &workspace)
	grant := workspace.Controls[len(workspace.Controls)-1]
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/controls/"+grant.ID+"/interventions", owner, `{"action":"guide","message":"Propose a bounded combined contract; do not publish.","version":1}`, http.StatusOK, nil)

	// The agent's first suggestion is attributable, tested, and explicitly
	// rejected; it is not silently overwritten by the accepted result.
	workflowJSON(t, server.URL, http.MethodPut, workspaceBase+"/files", agent, `{"path":"contract.txt","content":"agent-only\n"}`, http.StatusOK, nil)
	resolutionBody := func(kind, summary, disposition, rationale, actorKind string) string {
		payload := map[string]any{"kind": kind, "summary": summary, "paths": []string{"contract.txt"}, "evidence": []map[string]string{{"kind": "conflict", "reference": "contract.txt", "revision": analysis.Source.Revision.CommitID, "path": "contract.txt"}}, "impacts": []map[string]string{{"kind": "acceptance_criterion", "outcome": "both source and target callers remain supported", "disposition": disposition, "rationale": rationale}}, "actor_kind": actorKind}
		encoded, _ := json.Marshal(payload)
		return string(encoded)
	}
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/resolutions", agent, resolutionBody("proposal", "Prefer only the source behavior", "changed", "agent hypothesis pending owner review", "agent"), http.StatusCreated, nil)
	var rejected workspaces.Checkpoint
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints", agent, `{"summary":"Rejected agent-only candidate","paths":["contract.txt"],"reproducibility":{"dependencies":["repository snapshot"],"commands":["test \"$(cat contract.txt)\" = both"]}}`, http.StatusCreated, &rejected)
	rejectedIDs := verificationCriterionIDs(rejected)
	failedAttempt := verificationAttemptBody(rejected, rejectedIDs, "failed", "combined contract check failed")
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints/"+rejected.ID+"/verification-attempts", agent, failedAttempt, http.StatusCreated, &rejected)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints/"+rejected.ID+"/verification-decisions", owner, verificationDecisionBody(rejected, rejectedIDs, "rejected", "The suggestion discards Alice's accepted intent."), http.StatusCreated, &rejected)
	if rejected.Verification.Status != "failed" || len(rejected.Verification.Attempts) != 1 || len(rejected.Verification.Decisions) != 1 {
		t.Fatalf("rejected failed candidate was not retained: %#v", rejected.Verification)
	}
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/resolutions", owner, resolutionBody("undone", "Reject the agent-only suggestion", "preserved", "both owners require a combined result", "human"), http.StatusCreated, nil)

	if err := catalog.RemoveCollaborator("maintainer", storage.ID(repository.ID), "resolution-agent"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, workspaceBase+"/files", agent, `{"path":"contract.txt","content":"unreviewed\n"}`, http.StatusNotFound, nil)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/controls/"+grant.ID+"/interventions", owner, `{"action":"revoke","message":"Agent participation ended after rejection.","version":2}`, http.StatusOK, nil)

	workflowJSON(t, server.URL, http.MethodPut, workspaceBase+"/files", bob, `{"path":"contract.txt","content":"both\n"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/resolutions", bob, resolutionBody("applied", "Combine both accepted behaviors", "preserved", "the exact combined checks exercise both owners' criteria", "human"), http.StatusCreated, nil)
	var accepted workspaces.Checkpoint
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints", bob, `{"summary":"Combined reviewed result","paths":["contract.txt","source.go"],"reproducibility":{"dependencies":["repository snapshot"],"commands":["test \"$(cat contract.txt)\" = both","test -f source.go && test -f target.go"]}}`, http.StatusCreated, &accepted)
	acceptedIDs := verificationCriterionIDs(accepted)
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints/"+accepted.ID+"/verification-attempts", bob, verificationAttemptBody(accepted, acceptedIDs, "passed", "both criteria passed"), http.StatusCreated, &accepted)
	for actor, rationale := range map[string]string{owner: "Target behavior and setup are retained.", bob: "Source behavior and authorship are retained."} {
		workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints/"+accepted.ID+"/verification-decisions", actor, verificationDecisionBody(accepted, acceptedIDs, "approved", rationale), http.StatusCreated, &accepted)
	}
	if accepted.Verification.Status != "passed" || len(accepted.Verification.Attempts) != 1 || len(accepted.Verification.Decisions) != 2 {
		t.Fatalf("accepted proof is incomplete: %#v", accepted.Verification)
	}

	var publication struct {
		Workspace   workspaces.Workspace     `json:"workspace"`
		Publication workspaces.Publication   `json:"publication"`
		Pull        pullrequests.PullRequest `json:"pull_request"`
	}
	publishBody := `{"mode":"resolution_pull_request","branch":"resolve/both-intents","target_branch":"main","title":"Integrate both contract changes","message":"Publish the attributed, verified reconciliation."}`
	workflowJSON(t, server.URL, http.MethodPost, workspaceBase+"/checkpoints/"+accepted.ID+"/publication", owner, publishBody, http.StatusCreated, &publication)
	if publication.Pull.OriginPullRequestID != bobPull.ID || publication.Pull.ResolutionContext == nil || len(publication.Publication.ResolutionIDs) < 3 || len(publication.Publication.ApprovalIDs) != 2 {
		t.Fatalf("publication lost reconciliation provenance: %#v", publication)
	}
	resolutionBase := "/repositories/" + repository.ID + "/pull-requests/" + publication.Pull.ID
	waitForWorkflowCheck(t, server.URL, resolutionBase, owner, publication.Pull.SourceCommitID, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, resolutionBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, resolutionBase+"/queue", owner, "", http.StatusCreated, nil)
	entries := waitForQueueOutcomes(t, server.URL, repository.ID, owner, map[string]string{publication.Pull.ID: "merged"})
	if entries[publication.Pull.ID].State != "merged" || len(entries[publication.Pull.ID].History) == 0 {
		t.Fatalf("final queue history is incomplete: %#v", entries[publication.Pull.ID])
	}

	verified := gitClone(t, remote(bobGit))
	assertFile(t, filepath.Join(verified, "contract.txt"), "both\n", 0)
	commit := gitOutput(t, verified, "show", "-s", "--format=%B", publication.Pull.SourceCommitID)
	for _, marker := range []string{"Workspace-ID:", "Verification-Candidate:", "Resolution-Source:", "Resolution-Target:", "Resolution-Approval:", "Resolution-Entry:"} {
		if !strings.Contains(commit, marker) {
			t.Fatalf("resolution commit lacks %s: %s", marker, commit)
		}
	}
	parents := strings.Fields(gitOutput(t, verified, "show", "-s", "--format=%P", publication.Pull.SourceCommitID))
	if len(parents) != 2 {
		t.Fatalf("resolution commit does not retain both frozen inputs: %v", parents)
	}
}

func verificationCriterionIDs(checkpoint workspaces.Checkpoint) []string {
	ids := make([]string, 0, len(checkpoint.Verification.Criteria))
	for _, criterion := range checkpoint.Verification.Criteria {
		ids = append(ids, criterion.ID)
	}
	return ids
}

func verificationAttemptBody(checkpoint workspaces.Checkpoint, ids []string, status, log string) string {
	payload := workspaces.VerificationAttempt{CriterionIDs: ids, Kind: "acceptance", InputRevisions: checkpoint.Verification.Inputs, Commands: []string{"test combined acceptance"}, Logs: []string{log}, Coverage: []string{"source", "target", "combined"}, Cost: 1, Currency: "test-units", Status: status}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func verificationDecisionBody(checkpoint workspaces.Checkpoint, ids []string, decision, rationale string) string {
	payload := workspaces.VerificationDecision{CriterionIDs: ids, InputRevisions: checkpoint.Verification.Inputs, Decision: decision, Rationale: rationale}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}
