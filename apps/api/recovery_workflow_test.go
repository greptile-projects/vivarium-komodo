package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryresponses"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestContinuityRecoveryWorkflow is the black-box boundary for the complete
// commitment-to-restored-collaboration loop. It deliberately retains unsafe
// evidence and a failed rehearsal beside the reviewed, verified recovery path.
func TestContinuityRecoveryWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	objectives, _ := recoveryobjectives.New(t.TempDir())
	plans, _ := protectionplans.New(t.TempDir())
	exercises, _ := recoveryexercises.New(t.TempDir(), plans)
	investigations, _ := recoveryinvestigations.New(t.TempDir())
	improvements, _ := recoveryimprovements.New(t.TempDir())
	responses, _ := recoveryresponses.New(t.TempDir(), plans)
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, proposalStore, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, proposalStore, catalog, credentials, nil, runner, checks, nil)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerRecoveryObjectivesHTTP(mux, objectives, catalog, credentials)
	registerProtectionPlansHTTP(mux, plans, objectives, catalog, credentials)
	registerRecoveryExercisesHTTP(mux, exercises, catalog, credentials)
	registerRecoveryInvestigationsHTTP(mux, investigations, exercises, catalog, credentials)
	registerRecoveryImprovementsHTTP(mux, improvements, investigations, exercises, proposalStore, catalog, credentials)
	registerRecoveryResponsesHTTP(mux, responses, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "continuity-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reviewer := issueAccess(t, credentials, "recovery-reviewer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "continuity-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	var repo repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"survivable-collaboration","visibility":"private"}`, 201, &repo)
	for _, actor := range []string{"codex", "recovery-reviewer"} {
		if _, err := catalog.AddCollaborator("continuity-owner", repo.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repo.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Continuity Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.test")
	writeWorkflowFile(t, work, "recovery/restore.sh", "#!/bin/sh\nprintf 'restore_without_refs=true\\n'\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"continuity/reachable-history","command":"grep -q 'include_all_refs=true' recovery/restore.sh"}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Add regional recovery policy")
	baseRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	base := "/repositories/" + string(repo.ID)

	objectiveInput := recoveryobjectives.VersionInput{Title: "Collaboration survives regional loss", Description: "Restore service and retained Git decisions", OwnerIDs: []string{"continuity-owner"}, Resources: []recoveryobjectives.Resource{{ID: "git", Kind: "repository", Name: "Git history", UserCapability: "contributors clone reviewed history", OwnerIDs: []string{"continuity-owner"}, DependencyIDs: []string{"vault"}, AcceptableLoss: "0s", RestorationTime: "1h", Retention: "7y", Jurisdictions: []string{"EU"}, ValidationCriteria: []string{"all refs reachable"}, Feasibility: "achievable"}, {ID: "collaboration", Kind: "collaboration_records", ResourceID: "repository-events", Name: "reviews and decisions", UserCapability: "collaborators resume attributed work", OwnerIDs: []string{"continuity-owner"}, DependencyIDs: []string{"vault"}, AcceptableLoss: "5m", RestorationTime: "2h", Retention: "7y", Jurisdictions: []string{"EU"}, ValidationCriteria: []string{"pull and review trail reconciles"}, Feasibility: "achievable"}}, Dependencies: []recoveryobjectives.Dependency{{ID: "vault", Name: "regional recovery vault", Kind: "storage", OwnerIDs: []string{"continuity-owner"}, Protected: true, Protection: "vault-eu-v2"}}, ExceptionPolicy: "owner and reviewer approval", ChangeReason: "declare measurable continuity"}
	b, _ := json.Marshal(objectiveInput)
	var objective recoveryobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-objectives", owner, string(b), 201, &objective)
	planInput := protectionplans.VersionInput{ObjectiveID: objective.ID, ObjectiveVersion: 1, ResourceIDs: []string{"git", "collaboration"}, EnvironmentID: "production", Mode: "snapshot", Schedule: "hourly", MaximumAgeSeconds: 7200, Encryption: "AES-256-GCM", KeyReference: "kms:continuity", AccessScope: []string{"recovery-response"}, Destinations: []protectionplans.Destination{{ID: "vault", Kind: "object_store", Region: "eu", Jurisdiction: "EU", Authorized: true}}, Retention: "7y", ChecksumAlgorithm: "sha256", ValidationCriteria: []string{"refs and reviews reconcile"}, CostLimit: 25, Currency: "USD", ChangeReason: "protect collaboration state"}
	b, _ = json.Marshal(planInput)
	var plan protectionplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, base+"/protection-plans", owner, string(b), 201, &plan)
	now := time.Now().UTC().Truncate(time.Second)
	capture := func(key string, version int64, revision string, safe bool) protectionplans.CaptureInput {
		return protectionplans.CaptureInput{IdempotencyKey: key, PlanVersion: version, StartedAt: now.Add(-time.Minute), CapturedAt: now, Resources: []protectionplans.ManifestResource{{ResourceID: "git", SourceVersion: revision, Provenance: "main@" + revision, DependencyVersions: map[string]string{"vault": "v2"}, ObjectCount: 12, ByteCount: 4096, Checksum: "sha256:git", Complete: safe, SourceState: "committed"}, {ResourceID: "collaboration", SourceVersion: "events-42", Provenance: "events@42", DependencyVersions: map[string]string{"vault": "v2"}, ObjectCount: 20, ByteCount: 8192, Checksum: "sha256:events", Complete: true, SourceState: "committed"}}, Validation: protectionplans.Validation{CompletenessVerified: safe, ChecksumVerified: safe, DecryptionVerified: true, KeyAvailable: true, DestinationAuthorized: true, ValidatedAt: now, EvidenceDigest: "sha256:validation"}, Cost: 4}
	}
	b, _ = json.Marshal(capture("corrupt-provider-run", 1, baseRevision, false))
	workflowJSON(t, server.URL, http.MethodPost, base+"/protection-plans/"+plan.ID+"/captures", owner, string(b), 201, &plan)
	if plan.Captures[0].Recoverable {
		t.Fatal("corrupt evidence became a recovery point")
	}
	b, _ = json.Marshal(capture("verified-provider-run", 1, baseRevision, true))
	workflowJSON(t, server.URL, http.MethodPost, base+"/protection-plans/"+plan.ID+"/captures", owner, string(b), 201, &plan)

	launch := recoveryexercises.LaunchInput{IdempotencyKey: "regional-loss-1", Scenario: "regional loss", FailureModes: []string{"primary region destroyed", "vault dependency unavailable"}, PlanID: plan.ID, PlanVersion: 1, CaptureID: plan.LatestRecoverableCaptureID, Resources: []recoveryexercises.SelectedResource{{ResourceID: "git", SourceVersion: baseRevision, DependencyVersions: map[string]string{"vault": "v2"}}, {ResourceID: "collaboration", SourceVersion: "events-42", DependencyVersions: map[string]string{"vault": "v2"}}}, EnvironmentID: "isolated-region", IsolationKind: "ephemeral_networkless", MaximumDurationSeconds: 3600, MaximumCost: 12, Steps: []recoveryexercises.Step{{ID: "restore", Kind: "restore", ResourceID: "git", Command: "recovery/restore.sh", Expected: "all refs restored"}}, Checks: []recoveryexercises.Check{{ID: "history", Kind: "user_journey", Command: "git fsck --full", Expected: "history and reviews resolve", ObjectiveResourceIDs: []string{"git", "collaboration"}}}}
	exerciseResult := func(ex recoveryexercises.Exercise, passed bool) recoveryexercises.ResultInput {
		r := recoveryResult(now, ex, passed)
		r.StepResults[0].Command = "recovery/restore.sh"
		r.CheckResults[0].CheckID = "history"
		r.CheckResults[0].Command = "git fsck --full"
		return r
	}
	b, _ = json.Marshal(launch)
	var failed recoveryexercises.Exercise
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises", owner, string(b), 201, &failed)
	result := exerciseResult(failed, false)
	result.Summary = "vault recovered but restore omitted collaboration refs"
	b, _ = json.Marshal(result)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises/"+failed.ID+"/result", owner, string(b), 201, &failed)

	invInput := recoveryinvestigations.CreateInput{ExerciseID: failed.ID, ExerciseRevision: failed.Revision, Title: "Missing collaboration history", Question: "Why did regional restore fail?", ResourceIDs: []string{"git"}, Evidence: []recoveryinvestigations.Evidence{{Kind: "exercise", ResourceID: failed.ID, Revision: "2", Summary: "history check failed after dependency returned", Audience: "repository"}, {Kind: "code", ResourceID: string(repo.ID), Revision: baseRevision, Path: "recovery/restore.sh", Summary: "restore excludes non-branch refs", Audience: "repository", Uncertainty: "dependency outage timing did not corrupt the verified manifest"}}}
	b, _ = json.Marshal(invInput)
	var investigation recoveryinvestigations.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-investigations", agent, string(b), 201, &investigation)
	finding := recoveryinvestigations.Finding{Kind: "conclusion", Statement: "The restore policy omitted review refs; the recovered vault manifest remained intact.", Citations: []recoveryinvestigations.Citation{{EvidenceID: investigation.Evidence[0].ID}, {EvidenceID: investigation.Evidence[1].ID}}, Uncertainty: "regional dependency recovery time remains separately bounded", Verdict: "supported"}
	b, _ = json.Marshal(finding)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-investigations/"+investigation.ID+"/findings", agent, string(b), 201, &investigation)

	improveInput := recoveryimprovements.CreateInput{InvestigationID: investigation.ID, FindingID: investigation.Findings[0].ID, Title: "Restore every collaboration ref", BaseRevision: baseRevision, Tasks: []recoveryimprovements.TaskInput{{Title: "Include all refs", OwnerKind: "agent", OwnerID: "codex", AcceptanceCriteria: []string{"continuity check passes"}}, {Title: "Review restored history", OwnerKind: "human", OwnerID: "recovery-reviewer", DependsOn: []int{1}, AcceptanceCriteria: []string{"ordinary review approves exact revision"}}}}
	b, _ = json.Marshal(improveInput)
	var made struct {
		Improvement recoveryimprovements.Improvement `json:"improvement"`
		Proposal    proposals.Proposal               `json:"proposal"`
		Tasks       []string                         `json:"tasks"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-improvements", owner, string(b), 201, &made)
	var assigned proposals.Task
	workflowJSON(t, server.URL, http.MethodPut, base+"/proposals/"+made.Proposal.ID+"/plan/tasks/"+made.Tasks[0]+"/assignment", owner, `{"kind":"agent","assignee_id":"codex","mandate":"Change only the bounded restore policy and return through ordinary checks and review.","repository_id":"`+string(repo.ID)+`","base_revision":"`+baseRevision+`"}`, 200, &assigned)
	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Recovery Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.test")
	gitOutput(t, agentWork, "switch", "-c", "repair/all-collaboration-refs")
	writeWorkflowFile(t, agentWork, "recovery/restore.sh", "#!/bin/sh\nprintf 'include_all_refs=true cost_usd=0.31 handoff=recovery-reviewer\\n'\n")
	gitOutput(t, agentWork, "add", "recovery/restore.sh")
	gitOutput(t, agentWork, "commit", "-m", "Restore all collaboration refs")
	repairRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "repair/all-collaboration-refs")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", agent, `{"title":"Restore all collaboration refs","body":"Bounded agent repair with USD 0.31 cost and explicit reviewer handoff.","source_branch":"repair/all-collaboration-refs","target_branch":"main","proposal_id":"`+made.Proposal.ID+`","task_id":"`+made.Tasks[0]+`"}`, 201, &pull)
	pullBase := base + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, agent, repairRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, 200, &pull)
	link := recoveryimprovements.Link{Kind: "pull_request", ResourceID: pull.ID, Revision: pull.MergeCommitID, TaskID: made.Tasks[0], Summary: "reviewed agent repair; cost USD 0.31"}
	b, _ = json.Marshal(link)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-improvements/"+made.Improvement.ID+"/links", owner, string(b), 201, &made.Improvement)

	planInput.ChangeReason = "include every collaboration ref"
	revisionBody := struct {
		ExpectedVersion int64 `json:"expected_version"`
		protectionplans.VersionInput
	}{1, planInput}
	b, _ = json.Marshal(revisionBody)
	workflowJSON(t, server.URL, http.MethodPost, base+"/protection-plans/"+plan.ID+"/versions", owner, string(b), 201, &plan)
	b, _ = json.Marshal(capture("repaired-provider-run", 2, pull.MergeCommitID, true))
	workflowJSON(t, server.URL, http.MethodPost, base+"/protection-plans/"+plan.ID+"/captures", owner, string(b), 201, &plan)
	launch.IdempotencyKey, launch.PlanVersion, launch.CaptureID = "regional-loss-verification", 2, plan.LatestRecoverableCaptureID
	launch.Resources[0].SourceVersion = pull.MergeCommitID
	b, _ = json.Marshal(launch)
	var verified recoveryexercises.Exercise
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises", owner, string(b), 201, &verified)
	failedVerification := exerciseResult(verified, false)
	b, _ = json.Marshal(failedVerification)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises/"+verified.ID+"/result", owner, string(b), 201, &verified)
	b, _ = json.Marshal(map[string]string{"exercise_id": verified.ID})
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-improvements/"+made.Improvement.ID+"/verification", owner, string(b), 201, &made.Improvement)
	if made.Improvement.State != "verification_failed" {
		t.Fatalf("failed verification escaped: %+v", made.Improvement)
	}
	launch.IdempotencyKey = "regional-loss-verification-retry"
	b, _ = json.Marshal(launch)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises", owner, string(b), 201, &verified)
	passing := exerciseResult(verified, true)
	passing.CheckResults[0].AchievedObjectiveResourceIDs = []string{"git", "collaboration"}
	b, _ = json.Marshal(passing)
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-exercises/"+verified.ID+"/result", owner, string(b), 201, &verified)
	b, _ = json.Marshal(map[string]string{"exercise_id": verified.ID})
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-improvements/"+made.Improvement.ID+"/verification", owner, string(b), 201, &made.Improvement)
	if made.Improvement.State != "verified" {
		t.Fatalf("reviewed improvement not verified: %+v", made.Improvement)
	}

	activation := recoveryresponses.ActivationInput{IdempotencyKey: "incident-regional-loss", TriggerKind: "loss_event", TriggerID: "simulated-region-destroyed", LossConfirmed: true, Summary: "primary service and collaboration store destroyed", PlanID: plan.ID, PlanVersion: 2, CaptureID: plan.LatestRecoverableCaptureID, WorkspaceID: "incident-workspace", EnvironmentID: "production", EstimatedLoss: "under five minutes", ApproverIDs: []string{"recovery-reviewer"}, CommunicationChannels: []string{"status-page"}, RollbackOptions: []string{"return to maintenance mode"}, Steps: []recoveryresponses.Step{{ID: "restore", Title: "restore service and collaboration history", ResourceID: "git", ExecutorKind: "agent", ExecutorID: "codex", Command: "recovery/restore.sh", Expected: "validated refs and review history"}}}
	b, _ = json.Marshal(activation)
	var response recoveryresponses.Response
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-responses", owner, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.DecisionInput{ExpectedRevision: response.Revision, Kind: "approve", Rationale: "verified point satisfies declared objectives"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-responses/"+response.ID+"/approvals", reviewer, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.StepUpdate{ExpectedRevision: response.Revision, Status: "succeeded", Summary: "service and collaboration history restored", EvidenceDigest: "sha256:restored", KeyAvailable: true, ReplicaCurrent: true})
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-responses/"+response.ID+"/steps/restore", agent, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.ValidationInput{ExpectedRevision: response.Revision, Passed: true, EvidenceDigest: "sha256:user-journeys-and-git", Summary: "service journeys, Git history, reviews, and attribution reconcile"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/recovery-responses/"+response.ID+"/validations", reviewer, string(b), 201, &response)
	if response.State != "restored" || response.ActivatedBy != "continuity-owner" || len(response.Validations) != 1 {
		t.Fatalf("trusted return trail incomplete: %+v", response)
	}
}
