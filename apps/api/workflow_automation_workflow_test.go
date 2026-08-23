package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowcomponents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowdefinitions"
)

// TestCollaborativeWorkflowAutomation proves the complete accepted-signal to
// protected-delivery loop through public HTTP and stock Git. The workflow is a
// durable coordinator: every consequential platform action still passes its
// native permission, review, check, queue, release, and environment boundary.
func TestCollaborativeWorkflowAutomation(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the workflow automation boundary")
	}
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	definitions, _ := workflowdefinitions.New(t.TempDir())
	components, _ := workflowcomponents.New(t.TempDir())
	runner := checkruns.NewRunner(checks, repos)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: repos, checks: checks, starter: runner}
	runner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, repos, credentials)
	registerIssuesHTTP(mux, issueStore, releaseStore, repos, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, repos, credentials, nil, runner, checks, queue)
	registerCheckRunsHTTP(mux, checks, runner, pulls, repos, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, repos, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checks, repos, credentials, nil, nil, pulls)
	registerWorkflowDefinitionsHTTP(mux, definitions, repos, credentials)
	registerWorkflowComponentsHTTP(mux, components, repos, credentials)
	registerGitHTTP(mux, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reviewer := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentAPI := issueAccess(t, credentials, "repair-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "repair-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repository, consumer struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"automated-service","visibility":"private"}`, 201, &repository)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"narrow-consumer","visibility":"private"}`, 201, &consumer)
	for _, id := range []string{"reviewer", "repair-agent"} {
		if _, err := repos.AddCollaborator("owner", storage.ID(repository.ID), id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repos.AddCollaborator("owner", storage.ID(consumer.ID), "reviewer"); err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/required-checks", owner, `{"branch":"main","checks":["repair-contract"]}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/integration-queue", owner, `{"branch":"main","enabled":true,"concurrency":1,"failure_behavior":"remove"}`, 200, nil)

	remote := func(repo, token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + repo)
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(repository.ID, ownerGit))
	gitOutput(t, work, "config", "user.name", "Workflow Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.test")
	writeWorkflowFile(t, work, "behavior.txt", "broken\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"repair-contract","command":"grep -qx repaired behavior.txt","timeout_seconds":30}]}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"package","command":"mkdir -p dist; cp behavior.txt dist/service","artifacts":["dist/service"]}]}`)
	writeWorkflowFile(t, work, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"rollout","health":[{"name":"repair-present","command":"grep -qx repaired \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	writeWorkflowFile(t, work, ".project/workflows/accepted-repair.json", "reviewed workflow source\n")
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Initialize workflow governed service")
	definitionRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")

	var issue issues.Issue
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/issues", reviewer, `{"title":"Repair broken behavior","expected_behavior":"The service reports repaired","observed_behavior":"The service reports broken","severity":"high","environment":"production","reproduction_steps":["read behavior.txt"],"visibility":"repository"}`, 201, &issue)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/issues/"+issue.ID+"/triage", owner, fmt.Sprintf(`{"expected_version":%d,"classification":"bug","priority":"high","assignee_ids":["repair-agent"],"labels":["accepted","automation"]}`, issue.Version), 200, &issue)

	definition := automationDefinition(definitionRevision, ownerID("owner"), ownerID("reviewer"))
	b, _ := json.Marshal(definition)
	var workflow workflowdefinitions.Workflow
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/workflow-definitions", owner, string(b), 201, &workflow)
	wf := "/repositories/" + repository.ID + "/workflow-definitions/" + workflow.ID
	workflowJSON(t, server.URL, http.MethodPost, wf+"/candidate-decisions", reviewer, `{"version":1,"kind":"review","decision":"approved","rationale":"bounded repair and human delivery gates match repository policy"}`, 201, &workflow)
	workflowJSON(t, server.URL, http.MethodPost, wf+"/activation", owner, `{"version":1}`, 201, &workflow)

	now := time.Now().UTC()
	invoke := automationInvoke(issue.ID, now)
	b, _ = json.Marshal(invoke)
	var run, duplicate workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodPost, wf+"/executions", owner, string(b), 201, &run)
	workflowJSON(t, server.URL, http.MethodPost, wf+"/executions", owner, string(b), 201, &duplicate)
	if duplicate.ID != run.ID {
		t.Fatal("duplicate accepted event created a second run")
	}
	exec := wf + "/executions/" + run.ID

	// Stale optimistic state, an operator pause, credential revocation, engine
	// interruption, and an ordinary failed attempt all retain one recoverable step.
	staleDispatch := fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":"stale","credential_expires_at":%q}`, run.Revision-1, now.Add(time.Minute).Format(time.RFC3339Nano))
	workflowJSON(t, server.URL, http.MethodPost, exec+"/steps/repair/dispatch", owner, staleDispatch, 409, nil)
	run = workflowDispatch(t, server.URL, exec, owner, run, "repair", "repair-1")
	firstCredential := run.Steps[0].Credential.Reference
	run = workflowControl(t, server.URL, exec, owner, run, "pause", "", "collaborator redirects unsafe first attempt")
	badResult := workflowdefinitions.ResultInput{ExpectedRevision: run.Revision, IdempotencyKey: "repair-1", CredentialReference: firstCredential, State: "succeeded", Outputs: map[string]workflowdefinitions.OutputValue{"revision": {Value: "untrusted", Accessible: true}}, Cost: 1}
	b, _ = json.Marshal(badResult)
	workflowJSON(t, server.URL, http.MethodPost, exec+"/steps/repair/results", owner, string(b), 422, nil)
	run = workflowControl(t, server.URL, exec, owner, run, "resume", "", "resume with a fresh step credential")
	run = workflowDispatch(t, server.URL, exec, owner, run, "repair", "repair-2")
	run = workflowResult(t, server.URL, exec, owner, run, "repair", "repair-2", "interrupted", nil, 0, "engine restarted")
	run = workflowControl(t, server.URL, exec, owner, run, "retry", "repair", "restart from durable attempt history")
	run = workflowDispatch(t, server.URL, exec, owner, run, "repair", "repair-3")
	run = workflowResult(t, server.URL, exec, owner, run, "repair", "repair-3", "failed", nil, .2, "candidate did not satisfy the issue")
	run = workflowControl(t, server.URL, exec, owner, run, "retry", "repair", "repair the retained failure")
	run = workflowDispatch(t, server.URL, exec, owner, run, "repair", "repair-4")

	agentWork := gitClone(t, remote(repository.ID, agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Approved Repair Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.test")
	gitOutput(t, agentWork, "switch", "-c", "automation/repair")
	writeWorkflowFile(t, agentWork, "behavior.txt", "repaired\n")
	gitOutput(t, agentWork, "commit", "-am", "Repair accepted issue")
	repairRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "automation/repair")
	run = workflowResult(t, server.URL, exec, agentAPI, run, "repair", "repair-4", "succeeded", map[string]string{"revision": repairRevision}, 1, "")

	run = workflowDispatch(t, server.URL, exec, owner, run, "pull", "pull-1")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", agentAPI, `{"title":"Repair accepted issue","source_branch":"automation/repair","target_branch":"main"}`, 201, &pull)
	run = workflowResult(t, server.URL, exec, owner, run, "pull", "pull-1", "succeeded", map[string]string{"pull_request": pull.ID}, .2, "")

	waitForWorkflowCheck(t, server.URL, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID, owner, pull.SourceCommitID, checkruns.Succeeded)
	run = workflowControl(t, server.URL, exec, reviewer, run, "take_over", "review", "human reviews exact pull revision and required checks")
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID+"/reviews/me", reviewer, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	run = workflowResult(t, server.URL, exec, reviewer, run, "review", run.Steps[2].Attempts[len(run.Steps[2].Attempts)-1].IdempotencyKey, "succeeded", map[string]string{"reviewed_revision": pull.SourceCommitID}, 0, "")

	run = workflowDispatch(t, server.URL, exec, owner, run, "queue", "queue-1")
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID+"/queue", owner, "", 201, nil)
	waitForQueueOutcomes(t, server.URL, repository.ID, owner, map[string]string{pull.ID: "merged"})
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/pull-requests/"+pull.ID, owner, "", 200, &pull)
	run = workflowResult(t, server.URL, exec, owner, run, "queue", "queue-1", "succeeded", map[string]string{"merge_revision": pull.MergeCommitID}, .2, "")

	run = workflowDispatch(t, server.URL, exec, owner, run, "release", "release-1")
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/releases", owner, `{"version":"v1.0.1","commit_id":"`+pull.MergeCommitID+`","notes":"Workflow-delivered accepted repair"}`, 201, &release)
	build, artifact := waitForReleaseArtifact(t, server.URL, repository.ID, release.ID, owner)
	run = workflowResult(t, server.URL, exec, owner, run, "release", "release-1", "succeeded", map[string]string{"release": release.ID}, .5, "")

	// A rejected workflow approval cannot dispatch the deployment request. A
	// fresh request and independent approval do not bypass environment approval.
	run = workflowRequestApproval(t, server.URL, exec, owner, run, "deploy")
	denied := run.ActionApprovals[len(run.ActionApprovals)-1]
	run = workflowDecideApproval(t, server.URL, exec, reviewer, run, denied.ID, "rejected", "deployment window is not yet protected")
	workflowJSON(t, server.URL, http.MethodPost, exec+"/steps/deploy/dispatch", owner, fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":"deploy-denied","credential_expires_at":%q}`, run.Revision, time.Now().Add(time.Minute).Format(time.RFC3339Nano)), 429, nil)
	run = workflowRequestApproval(t, server.URL, exec, owner, run, "deploy")
	approved := run.ActionApprovals[len(run.ActionApprovals)-1]
	run = workflowDecideApproval(t, server.URL, exec, reviewer, run, approved.ID, "approved", "protected window and exact release are confirmed")
	run = workflowDispatch(t, server.URL, exec, owner, run, "deploy", "deploy-1")
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/environments", owner, `{"name":"production","position":1,"command":"printf deployed","required_approvals":1,"concurrency":1}`, 201, &environment)
	var deployment deployments.Deployment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments", agentAPI, `{"environment_id":"`+environment.ID+`","release_id":"`+release.ID+`","build_run_id":"`+build.ID+`","artifact_id":"`+artifact.ID+`"}`, 201, &deployment)
	if deployment.State != "pending" || len(deployment.Approvals) != 0 {
		t.Fatalf("protected deployment did not wait: %#v", deployment)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/deployments/"+deployment.ID+"/approvals", reviewer, `{}`, 200, &deployment)
	deployment = waitForDeployment(t, server.URL, repository.ID, deployment.ID, owner, "succeeded")
	run = workflowResult(t, server.URL, exec, owner, run, "deploy", "deploy-1", "succeeded", map[string]string{"deployment": deployment.ID}, .5, "")
	if run.State != "completed" || len(run.ActionReceipts) != 1 || run.Cost < 2.59 || run.Cost > 2.61 || run.WorkflowRepositoryRevision != definitionRevision {
		t.Fatalf("workflow trail is incomplete: %#v", run)
	}

	// A second accepted signal exceeds its step boundary without affecting the
	// delivered run; the blocked attempt retains its cost decision and revokes
	// the short-lived reference.
	breach := automationInvoke("issue-budget", time.Now().UTC())
	breach.IdempotencyKey, breach.Event.ID, breach.Event.Revision = "issue-budget", "issue-budget", "issue-budget-v1"
	b, _ = json.Marshal(breach)
	var limited workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodPost, wf+"/executions", owner, string(b), 201, &limited)
	limitedExec := wf + "/executions/" + limited.ID
	limited = workflowDispatch(t, server.URL, limitedExec, owner, limited, "repair", "budget-1")
	limited = workflowResult(t, server.URL, limitedExec, owner, limited, "repair", "budget-1", "succeeded", map[string]string{"revision": "too-expensive"}, 5, "")
	if limited.State != "blocked" || limited.Blockers[0] != "budget_exceeded" || limited.Steps[0].Credential.RevokedAt == nil {
		t.Fatalf("budget containment failed: %#v", limited)
	}

	// Publish the proven repair step and pin it in another repository with only
	// issue read and pull creation. An observed breaking upgrade appends history
	// and blocks adoption without rewriting the working pin or completed run.
	publish := workflowcomponents.PublishInput{Name: "bounded-repair", Version: "1.0.0", Summary: "Prepare a reviewed issue repair", PackageVersionID: "component-release-1", SourceRepositoryID: repository.ID, SourceRevision: definitionRevision, SourcePath: ".project/workflows/accepted-repair.json", ArtifactDigest: "sha256:" + strings.Repeat("a", 64), Attestation: "release and workflow run verified", Inputs: []workflowcomponents.Field{{Name: "issue", Type: "string", Required: true}}, Outputs: []workflowcomponents.Field{{Name: "revision", Type: "string", Required: true}}, RequestedCapabilities: []string{"issue:read", "pull:create"}, Compatibility: workflowcomponents.Compatibility{Engine: "komodo-workflows", MinimumVersion: "1.0.0"}, DataUse: workflowcomponents.DataUse{Classes: []string{"repository_metadata"}, Purposes: []string{"repair"}, Retention: "execution lifetime"}, Tests: []workflowcomponents.TestEvidence{{Name: "accepted issue delivery", Revision: definitionRevision, Status: "passed", Attestation: "workflow execution " + run.ID}}, Support: workflowcomponents.Support{Policy: "maintained major", Contact: "https://support.example.test"}, PublisherSubject: "user:owner@local", PublisherInstance: "local", Visibility: "repository"}
	b, _ = json.Marshal(publish)
	var component workflowcomponents.Component
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/workflow-components", owner, string(b), 201, &component)
	install := workflowcomponents.InstallInput{ComponentID: component.ID, PullRequestID: "consumer-pr-1", PullRevision: "consumer-revision-1", Configuration: map[string]any{"labels": []any{"accepted"}}, Permissions: []workflowcomponents.PermissionMapping{{Requested: "issue:read", LocalPermission: "issues:read", Resource: "repository:" + consumer.ID}, {Requested: "pull:create", LocalPermission: "pulls:create", Resource: "repository:" + consumer.ID}}, Health: workflowcomponents.Health{Publisher: "unchanged", Trust: "trusted", Peer: "available", Vulnerability: "clear", Compatibility: "compatible"}, Reason: "reuse proven repair with narrower authority"}
	b, _ = json.Marshal(install)
	var installation workflowcomponents.Installation
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+consumer.ID+"/workflow-component-installations", owner, string(b), 201, &installation)
	install.PullRequestID, install.PullRevision, install.Reason = "consumer-pr-2", "consumer-revision-2", "inspect incompatible upgrade without replacing retained evidence"
	install.Health = workflowcomponents.Health{Publisher: "changed", Trust: "trusted", Peer: "available", Vulnerability: "clear", Compatibility: "breaking"}
	revisionBody := struct {
		ExpectedRevision int64 `json:"expected_revision"`
		workflowcomponents.InstallInput
	}{installation.CurrentRevision, install}
	b, _ = json.Marshal(revisionBody)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+consumer.ID+"/workflow-component-installations/"+installation.ID+"/revisions", owner, string(b), 201, &installation)
	if installation.State != "attention_required" || len(installation.Revisions) != 2 || installation.Revisions[0].Component.SourceRevision != definitionRevision || run.State != "completed" {
		t.Fatalf("component reuse or immutable run containment failed: installation=%#v run=%#v", installation, run)
	}
}

func ownerID(id string) string { return id }

func automationDefinition(revision, owner, reviewer string) workflowdefinitions.Input {
	step := func(id, name string, needs []string, kind, reference, capability string, outputs ...string) workflowdefinitions.Step {
		fields := make([]workflowdefinitions.Field, 0, len(outputs))
		for _, output := range outputs {
			fields = append(fields, workflowdefinitions.Field{Name: output, Type: "string", Required: true})
		}
		return workflowdefinitions.Step{ID: id, Name: name, Needs: needs, Outputs: fields, Invocation: workflowdefinitions.Invocation{Kind: kind, Reference: reference, Revision: "v1", Accessible: true, OwnerIDs: []string{owner}, Capabilities: []string{capability}}, Retry: workflowdefinitions.Retry{MaximumAttempts: 4}, TimeoutSeconds: 300, MaximumCost: 3, CompletionCriteria: []string{name + " retained"}}
	}
	steps := []workflowdefinitions.Step{
		step("repair", "Prepare bounded repair", nil, "approved_agent", "repair-agent", "contents:write", "revision"),
		step("pull", "Open pull request", []string{"repair"}, "platform_action", "pull.create", "pull:create", "pull_request"),
		step("review", "Human review and checks", []string{"pull"}, "manual", "pull.review", "pull:review", "reviewed_revision"),
		step("queue", "Enter merge queue", []string{"review"}, "platform_action", "merge_queue.enter", "pull:queue", "merge_revision"),
		step("release", "Build release", []string{"queue"}, "platform_action", "release.create", "release:create", "release"),
		step("deploy", "Request protected deployment", []string{"release"}, "platform_action", "deployment.request", "deployment:create", "deployment"),
	}
	steps[5].Invocation.ActionClass = "deployment"
	return workflowdefinitions.Input{Name: "Accepted issue delivery", Outcome: "Reviewed repair is released and deployed", RepositoryRevision: revision, DefinitionPath: ".project/workflows/accepted-repair.json", Triggers: []workflowdefinitions.Trigger{{ID: "accepted", Type: "repository_event", Event: "issue.accepted", Conditions: []string{"label=accepted"}, InputMappings: map[string]string{"issue": "event.id"}}}, Inputs: []workflowdefinitions.Field{{Name: "issue", Type: "string", Required: true}}, Steps: steps, Outputs: []workflowdefinitions.Field{{Name: "deployment", Type: "string", Required: true}}, MaximumCost: 10, Currency: "USD", MaximumConcurrency: 1, OwnerIDs: []string{owner}, CompletionCriteria: []string{"protected deployment succeeds"}, Governance: workflowdefinitions.Governance{RequiredReviewerIDs: []string{reviewer}, ActionRequirements: []workflowdefinitions.ActionRequirement{{ActionClass: "deployment", OwnerIDs: []string{reviewer}, MinimumApprovals: 1, SeparateFromAuthor: true, ApprovalTTLSeconds: 300}}}, ChangeReason: "automate accepted issue delivery with retained human gates"}
}

func automationInvoke(issue string, now time.Time) workflowdefinitions.InvokeInput {
	resources := []workflowdefinitions.ResourceRevision{{Kind: "approved_agent", Reference: "repair-agent", Revision: "v1"}, {Kind: "platform_action", Reference: "pull.create", Revision: "v1"}, {Kind: "manual", Reference: "pull.review", Revision: "v1"}, {Kind: "platform_action", Reference: "merge_queue.enter", Revision: "v1"}, {Kind: "platform_action", Reference: "release.create", Revision: "v1"}, {Kind: "platform_action", Reference: "deployment.request", Revision: "v1"}}
	return workflowdefinitions.InvokeInput{IdempotencyKey: "accepted:" + issue, WorkflowVersion: 1, Event: workflowdefinitions.TriggeringEvent{ID: "accepted:" + issue, Type: "repository_event", Name: "issue.accepted", Revision: "issue:" + issue + ":triage-1", OccurredAt: now.Add(-time.Second)}, Actor: workflowdefinitions.ExecutionActor{Kind: "human", ID: "owner"}, Inputs: map[string]any{"issue": issue}, PermittedResources: resources, Policy: workflowdefinitions.PolicyDecision{Repository: "allowed", Organization: "allowed", Agent: "allowed", Embargo: "allowed", Environment: "allowed", Approval: "allowed"}}
}

func workflowDispatch(t *testing.T, origin, exec, actor string, run workflowdefinitions.Execution, step, key string) workflowdefinitions.Execution {
	t.Helper()
	body := fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":%q,"credential_expires_at":%q}`, run.Revision, key, time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano))
	workflowJSON(t, origin, http.MethodPost, exec+"/steps/"+step+"/dispatch", actor, body, 201, &run)
	return run
}
func workflowResult(t *testing.T, origin, exec, actor string, run workflowdefinitions.Execution, step, key, state string, outputs map[string]string, cost float64, failure string) workflowdefinitions.Execution {
	t.Helper()
	i := 0
	for n := range run.Steps {
		if run.Steps[n].ID == step {
			i = n
		}
	}
	values := map[string]workflowdefinitions.OutputValue{}
	for name, value := range outputs {
		values[name] = workflowdefinitions.OutputValue{Value: value, Accessible: true, Digest: "sha256:" + name}
	}
	in := workflowdefinitions.ResultInput{ExpectedRevision: run.Revision, IdempotencyKey: key, State: state, Outputs: values, Cost: cost, Failure: failure}
	if run.Steps[i].Credential != nil {
		in.CredentialReference = run.Steps[i].Credential.Reference
	}
	b, _ := json.Marshal(in)
	workflowJSON(t, origin, http.MethodPost, exec+"/steps/"+step+"/results", actor, string(b), 201, &run)
	return run
}
func workflowControl(t *testing.T, origin, exec, actor string, run workflowdefinitions.Execution, action, step, reason string) workflowdefinitions.Execution {
	t.Helper()
	in := workflowdefinitions.ControlInput{ExpectedRevision: run.Revision, Action: action, StepID: step, Reason: reason}
	b, _ := json.Marshal(in)
	workflowJSON(t, origin, http.MethodPost, exec+"/controls", actor, string(b), 201, &run)
	return run
}
func workflowRequestApproval(t *testing.T, origin, exec, actor string, run workflowdefinitions.Execution, step string) workflowdefinitions.Execution {
	t.Helper()
	workflowJSON(t, origin, http.MethodPost, exec+"/steps/"+step+"/approval-requests", actor, fmt.Sprintf(`{"expected_revision":%d}`, run.Revision), 201, &run)
	return run
}
func workflowDecideApproval(t *testing.T, origin, exec, actor string, run workflowdefinitions.Execution, approval, decision, rationale string) workflowdefinitions.Execution {
	t.Helper()
	body := fmt.Sprintf(`{"expected_revision":%d,"decision":%q,"rationale":%q}`, run.Revision, decision, rationale)
	workflowJSON(t, origin, http.MethodPost, exec+"/approval-requests/"+approval+"/decisions", actor, body, 201, &run)
	return run
}
