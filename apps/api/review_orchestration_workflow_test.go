package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewcompletion"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewwork"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestReviewOrchestrationWorkflow is the black-box boundary for turning one
// cross-cutting pull request into complete, current human-agent review. Stock
// Git and public HTTP retain qualification, parallel work, disagreement,
// reassignment, repair, selective staleness, checks, queueing, and final merge.
func TestReviewOrchestrationWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the review orchestration boundary")
	}
	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	queue, _ := integrationqueue.New(t.TempDir())
	plans, _ := reviewplans.New(t.TempDir())
	routing, _ := reviewrouting.New(t.TempDir())
	work, _ := reviewwork.New(t.TempDir())
	completion, _ := reviewcompletion.New(t.TempDir())
	completionSources := reviewCompletionSources{completion, plans, routing, work}
	runner := checkruns.NewRunner(checks, repos)
	coordinator := &integrationQueueCoordinator{queue: queue, pulls: pulls, repositories: repos, checks: checks, starter: runner}
	runner.SetCompletionHook(func(checkruns.Run) { go coordinator.reconcileAll(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.run(ctx)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, repos, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, repos, credentials, runner, checks, queue, completionSources)
	registerCheckRunsHTTP(mux, checks, runner, pulls, repos, credentials, nil, nil)
	registerReviewPlansHTTP(mux, plans, pulls, repos, credentials)
	registerReviewRoutingHTTP(mux, routing, plans, pulls, repos, credentials)
	registerReviewWorkHTTP(mux, work, plans, routing, pulls, repos, credentials)
	registerReviewCompletionHTTP(mux, completionSources, pulls, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	actors := []string{"owner", "contributor", "code-owner", "security-owner", "accessibility-owner", "consumer-owner", "backup-consumer", "review-agent", "overloaded", "recused"}
	tokens := map[string]string{}
	for _, actor := range actors {
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "review-orchestration", Visibility: repositories.Private})
	for _, actor := range actors[1:] {
		if _, err := repos.AddCollaborator("owner", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/required-checks", tokens["owner"], `{"branch":"main","checks":["cross-cutting"]}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/integration-queue", tokens["owner"], `{"branch":"main","enabled":true,"concurrency":1,"failure_behavior":"remove"}`, http.StatusOK, nil)

	checkout := t.TempDir()
	gitOutput(t, checkout, "init", "-b", "main")
	gitOutput(t, checkout, "config", "user.name", "Contributor")
	gitOutput(t, checkout, "config", "user.email", "contributor@example.test")
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(checkout, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	write("api.go", "package service\n\nconst API = \"v1\"\n")
	write("security.go", "package service\n\nconst Secure = true\n")
	write("ui.ts", "export const label = 'Submit'\n")
	write("consumer.md", "# Consumer contract v1\n")
	write(".komodo/checks.json", `{"version":1,"checks":[{"name":"cross-cutting","command":"grep -q 'verified' consumer.md","timeout_seconds":30}]}`)
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Establish reviewed service")
	baseRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	gitOutput(t, checkout, "switch", "-c", "feature/cross-cutting")
	write("api.go", "package service\n\nconst API = \"v2\"\n")
	write("security.go", "package service\n\nconst Secure = false // disputed candidate\n")
	write("ui.ts", "export const label = ''\n")
	write("consumer.md", "# Consumer contract pending\n")
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Propose cross-cutting API")
	firstRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	opened, _ := repos.Open(repository.ID)
	gitOutput(t, checkout, "remote", "add", "platform", opened.GitDir())
	gitOutput(t, checkout, "push", "platform", baseRevision+":refs/heads/main", firstRevision+":refs/heads/feature/cross-cutting")

	var pull pullrequests.PullRequest
	pullRoot := "/repositories/" + string(repository.ID) + "/pull-requests"
	workflowJSON(t, server.URL, http.MethodPost, pullRoot, tokens["contributor"], `{"title":"Evolve API, security, interface, and consumer contract","body":"One candidate needs coordinated specialist review.","source_branch":"feature/cross-cutting","target_branch":"main"}`, http.StatusCreated, &pull)
	base := pullRoot + "/" + pull.ID
	planBase, routeBase, workBase, completionBase := base+"/review-plans", base+"/review-routing", base+"/review-work", base+"/review-completion"

	areas := []reviewplans.Area{
		{ID: "code", Name: "Code", Expertise: []string{"go"}, Paths: []string{"api.go"}, OwnerIDs: []string{"code-owner"}, Questions: []string{"Is the API implementation correct?"}, Evidence: []reviewplans.Evidence{{Kind: "diff", Description: "Inspect API diff", Required: true}}, CompletionRules: []string{"current implementation inspected"}},
		{ID: "security", Name: "Security", Expertise: []string{"security"}, Paths: []string{"security.go"}, OwnerIDs: []string{"security-owner"}, Questions: []string{"Does the boundary fail closed?"}, Evidence: []reviewplans.Evidence{{Kind: "reproduction", Description: "Exercise boundary", Required: true}}, DependsOn: []string{"code"}, CompletionRules: []string{"repair reproduced"}},
		{ID: "accessibility", Name: "Accessibility", Expertise: []string{"accessibility"}, Paths: []string{"ui.ts"}, OwnerIDs: []string{"accessibility-owner"}, Questions: []string{"Does the control retain a name?"}, Evidence: []reviewplans.Evidence{{Kind: "interface-check", Description: "Inspect accessible name", Required: true}}, CompletionRules: []string{"keyboard and name checked"}},
		{ID: "consumer", Name: "Consumer", Expertise: []string{"consumer"}, Paths: []string{"consumer.md"}, OwnerIDs: []string{"consumer-owner"}, Questions: []string{"Can clients migrate?"}, Evidence: []reviewplans.Evidence{{Kind: "contract", Description: "Inspect consumer contract", Required: true}}, DependsOn: []string{"code"}, CompletionRules: []string{"consumer migration verified"}},
	}
	planInput := reviewplans.Input{Intent: "Ship one compatible, secure, accessible API evolution", Risk: "high", PolicyReferences: []string{"policy:protected-main"}, Commitments: []reviewplans.Context{{Kind: "decision", Reference: "decision:api-v2", Revision: baseRevision, Accessible: true, OwnerIDs: []string{"owner"}}}, Areas: areas, ChangeReason: "cross-cutting review before integration"}
	var published struct {
		Plan reviewplans.Plan `json:"plan"`
	}
	workflowValue(t, server.URL, http.MethodPost, planBase, tokens["contributor"], map[string]any{"expected_version": 0, "plan": planInput}, http.StatusCreated, &published)

	qualified := func(id, kind, expertise string) reviewrouting.Candidate {
		candidate := reviewrouting.Candidate{ParticipantID: id, Kind: kind, Expertise: []string{expertise}, Available: true, Capacity: 2, TeamResponsibility: true, Evidence: []reviewrouting.Evidence{{Kind: "team", Reference: "team:" + expertise, Summary: "current responsibility", Accessible: true}}}
		if kind == "agent" {
			candidate.AgentApproval = "approval:read-only"
			candidate.ApprovedCapabilities = []string{"repository:read", "finding:propose"}
		}
		return candidate
	}
	candidates := []reviewrouting.Candidate{
		qualified("code-owner", "human", "go"), qualified("security-owner", "human", "security"), qualified("accessibility-owner", "human", "accessibility"), qualified("consumer-owner", "human", "consumer"), qualified("backup-consumer", "human", "consumer"), qualified("review-agent", "agent", "security"),
		{ParticipantID: "overloaded", Kind: "human", Expertise: []string{"go"}, Available: true, CurrentLoad: 2, Capacity: 2, CodeOwnership: true},
		{ParticipantID: "recused", Kind: "human", Expertise: []string{"security"}, Available: true, Capacity: 1, CodeOwnership: true},
		{ParticipantID: "private-expert", Kind: "human", Expertise: []string{"security"}, Available: true, Capacity: 1, Evidence: []reviewrouting.Evidence{{Kind: "private-report", Reference: "hidden", Summary: "not shareable", Accessible: false}}},
	}
	var route reviewrouting.Routing
	workflowValue(t, server.URL, http.MethodPost, routeBase+"/suggestions", tokens["owner"], map[string]any{"candidates": candidates}, http.StatusOK, &route)
	if !workflowRoutingBlocker(route, "overloaded") || !workflowRoutingBlocker(route, "inaccessible_evidence: private-report") {
		t.Fatalf("qualification failures were hidden: %#v", route.Suggestions)
	}

	invite := func(area string, candidate reviewrouting.Candidate, replaces string) reviewrouting.Assignment {
		t.Helper()
		body := map[string]any{"candidate": candidate, "escalation": "owner", "reason": "qualified owner for " + area, "replaces": replaces}
		workflowValue(t, server.URL, http.MethodPost, routeBase+"/areas/"+area+"/invitations", tokens["owner"], body, http.StatusCreated, &route)
		assignment := route.Assignments[len(route.Assignments)-1]
		workflowValue(t, server.URL, http.MethodPost, routeBase+"/assignments/"+assignment.ID+"/transitions", tokens[candidate.ParticipantID], map[string]string{"state": "accepted", "reason": "accept bounded review"}, http.StatusOK, &route)
		return assignment
	}
	recused := invite("security", candidates[7], "")
	workflowValue(t, server.URL, http.MethodPost, routeBase+"/assignments/"+recused.ID+"/transitions", tokens["recused"], map[string]string{"state": "recused", "reason": "authored the affected boundary"}, http.StatusOK, &route)
	assignments := map[string]reviewrouting.Assignment{}
	for area, index := range map[string]int{"code": 0, "security": 1, "accessibility": 2, "consumer": 3} {
		assignments[area] = invite(area, candidates[index], map[string]string{"security": recused.ID}[area])
	}
	backup := invite("consumer", candidates[4], "")
	agentAssignment := invite("security", candidates[5], "")

	var workspace reviewwork.Workspace
	workflowJSON(t, server.URL, http.MethodGet, workBase, tokens["owner"], "", http.StatusOK, &workspace)
	areaItems := func(area string) []string {
		var ids []string
		for _, item := range workspace.Queue {
			if item.AreaID == area && item.Accessible {
				ids = append(ids, item.ID)
			}
		}
		return ids
	}
	for area, assignment := range assignments {
		workflowValue(t, server.URL, http.MethodPost, workBase+"/progress", tokens[assignment.ParticipantID], map[string]any{"expected_version": workspace.Version, "assignment_id": assignment.ID, "state": "complete", "queue_item_ids": areaItems(area), "coverage": []string{area + " exact diff and requirements"}}, http.StatusCreated, &workspace)
	}
	workflowValue(t, server.URL, http.MethodPost, workBase+"/progress", tokens["review-agent"], map[string]any{"expected_version": workspace.Version, "assignment_id": agentAssignment.ID, "state": "complete", "queue_item_ids": areaItems("security"), "coverage": []string{"static boundary analysis"}, "uncertainty": []string{"runtime behavior unobserved"}}, http.StatusCreated, &workspace)

	citation := func(path string) reviewwork.Citation {
		return reviewwork.Citation{Kind: "diff", Reference: path, Revision: firstRevision, Summary: "exact candidate evidence", Accessible: true, Audience: "repository"}
	}
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings", tokens["review-agent"], map[string]any{"expected_version": workspace.Version, "assignment_id": agentAssignment.ID, "summary": "security flag is intentionally unreachable", "severity": "high", "conclusion": "no_issue", "citations": []reviewwork.Citation{citation("security.go#L3")}, "uncertainty": []string{"runtime unverified"}}, http.StatusCreated, &workspace)
	agentFinding := workspace.Findings[len(workspace.Findings)-1]
	privateCitation := citation("security.go#private")
	privateCitation.Audience = "embargoed"
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings", tokens["review-agent"], map[string]any{"expected_version": workspace.Version, "assignment_id": agentAssignment.ID, "summary": "private claim", "severity": "high", "conclusion": "concern", "citations": []reviewwork.Citation{privateCitation}}, http.StatusUnprocessableEntity, nil)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/messages", tokens["security-owner"], map[string]any{"expected_version": workspace.Version, "assignment_id": assignments["security"].ID, "area_id": "security", "kind": "challenge", "body": "The reachable default contradicts the agent conclusion.", "finding_ids": []string{agentFinding.ID}, "citations": []reviewwork.Citation{citation("security.go#L3")}}, http.StatusCreated, &workspace)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+agentFinding.ID+"/decisions", tokens["owner"], map[string]any{"expected_version": workspace.Version, "classification": "challenged", "rationale": "human reproduction disputes static analysis", "dissent": "agent retains its cited interpretation"}, http.StatusCreated, &workspace)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings", tokens["security-owner"], map[string]any{"expected_version": workspace.Version, "assignment_id": assignments["security"].ID, "summary": "security boundary fails open", "severity": "critical", "conclusion": "concern", "citations": []reviewwork.Citation{citation("security.go#L3")}}, http.StatusCreated, &workspace)
	humanFinding := workspace.Findings[len(workspace.Findings)-1]
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+humanFinding.ID+"/decisions", tokens["owner"], map[string]any{"expected_version": workspace.Version, "classification": "accepted", "rationale": "repair the reproduced fail-open boundary"}, http.StatusCreated, &workspace)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+agentFinding.ID+"/decisions", tokens["owner"], map[string]any{"expected_version": workspace.Version, "classification": "superseded", "rationale": "human finding captures the verified reachable behavior", "related_finding_id": humanFinding.ID, "dissent": "original agent proposal remains retained"}, http.StatusCreated, &workspace)

	workflowValue(t, server.URL, http.MethodPost, workBase+"/handoffs", tokens["consumer-owner"], map[string]any{"expected_version": workspace.Version, "from_assignment_id": assignments["consumer"].ID, "to_assignment_id": backup.ID, "reason": "primary owner became unavailable", "queue_item_ids": areaItems("consumer"), "residual_uncertainty": []string{"migration example needs confirmation"}}, http.StatusCreated, &workspace)
	handoff := workspace.Handoffs[len(workspace.Handoffs)-1]
	workflowValue(t, server.URL, http.MethodPost, workBase+"/handoffs/"+handoff.ID+"/acceptance", tokens["backup-consumer"], map[string]any{"expected_version": workspace.Version}, http.StatusCreated, &workspace)
	workflowValue(t, server.URL, http.MethodPost, routeBase+"/assignments/"+assignments["consumer"].ID+"/transitions", tokens["consumer-owner"], map[string]string{"state": "unavailable", "reason": "incident duty"}, http.StatusOK, &route)
	workflowValue(t, server.URL, http.MethodPost, routeBase+"/assignments/"+agentAssignment.ID+"/transitions", tokens["owner"], map[string]string{"state": "revoked", "reason": "read-only agent approval expired"}, http.StatusOK, &route)

	workflowValue(t, server.URL, http.MethodPut, completionBase+"/requirements", tokens["owner"], map[string]any{"expected_version": 0, "area_ids": []string{"code", "security", "accessibility", "consumer"}}, http.StatusOK, new(reviewcompletion.View))
	var completionView reviewcompletion.View
	workflowValue(t, server.URL, http.MethodPost, completionBase+"/areas/code/acknowledgements", tokens["code-owner"], map[string]any{"expected_version": 1, "assignment_id": assignments["code"].ID, "decision": "approve", "rationale": "first revision code inspected"}, http.StatusOK, &completionView)
	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", tokens["owner"], `{"decision":"approve"}`, http.StatusOK, nil)

	write("security.go", "package service\n\nconst Secure = true // verified repair\n")
	write("ui.ts", "export const label = 'Submit changes'\n")
	write("consumer.md", "# Consumer contract v2 verified\n")
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Address coordinated review")
	repairRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	gitOutput(t, checkout, "push", "platform", "feature/cross-cutting")
	workflowJSON(t, server.URL, http.MethodPost, base+"/synchronize", tokens["contributor"], "{}", http.StatusOK, &pull)
	if pull.SourceCommitID != repairRevision {
		t.Fatalf("repair revision not synchronized: %#v", pull)
	}
	var reviews struct {
		Items []pullrequests.Review `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/reviews", tokens["owner"], "", http.StatusOK, &reviews)
	if len(reviews.Items) != 1 || reviews.Items[0].CommitID == repairRevision {
		t.Fatalf("ordinary approval did not become stale: %#v", reviews.Items)
	}

	planInput.Risk = "critical"
	planInput.ChangeReason = "reachable fail-open behavior raised candidate risk"
	workflowValue(t, server.URL, http.MethodPost, planBase, tokens["contributor"], map[string]any{"expected_version": 1, "plan": planInput}, http.StatusCreated, &published)
	workflowValue(t, server.URL, http.MethodPost, routeBase+"/suggestions", tokens["owner"], map[string]any{"candidates": candidates}, http.StatusOK, &route)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/revision-transitions", tokens["contributor"], map[string]any{"expected_version": workspace.Version, "findings": []reviewwork.FindingApplicability{{FindingID: agentFinding.ID, State: "addressed", Reason: "superseded analysis retained"}, {FindingID: humanFinding.ID, State: "addressed", Reason: "repair commit restores fail-closed default"}}}, http.StatusCreated, &workspace)
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+humanFinding.ID+"/work-links", tokens["contributor"], map[string]any{"expected_version": workspace.Version, "kind": "commit", "reference": repairRevision, "revision": repairRevision, "purpose": "restore secure default and consumer contract"}, http.StatusCreated, &workspace)

	// Re-evaluate and reassign every current area. The backup consumer owns the
	// handed-off area, and the revoked agent is deliberately not re-invited.
	currentAssignments := map[string]reviewrouting.Assignment{}
	for area, index := range map[string]int{"code": 0, "security": 1, "accessibility": 2, "consumer": 4} {
		currentAssignments[area] = invite(area, candidates[index], "")
	}
	failedProof := reviewwork.Citation{Kind: "check", Reference: "reproduction:repair-1", Revision: repairRevision, Summary: "first targeted repair attempt failed", Accessible: true, Audience: "repository"}
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+humanFinding.ID+"/verifications", tokens["security-owner"], map[string]any{"expected_version": workspace.Version, "kind": "reproduction", "reference": "scenario:fail-closed", "base_revision": firstRevision, "revision": repairRevision, "outcome": "failed", "summary": "first repair fixture used the obsolete consumer shape", "citations": []reviewwork.Citation{failedProof}}, http.StatusCreated, &workspace)
	passedProof := failedProof
	passedProof.Reference = "check:cross-cutting"
	passedProof.Summary = "current targeted repair and consumer contract pass"
	workflowValue(t, server.URL, http.MethodPost, workBase+"/findings/"+humanFinding.ID+"/verifications", tokens["security-owner"], map[string]any{"expected_version": workspace.Version, "kind": "check", "reference": "cross-cutting", "base_revision": firstRevision, "revision": repairRevision, "outcome": "passed", "summary": "repair is contained on the exact candidate", "citations": []reviewwork.Citation{passedProof}}, http.StatusCreated, &workspace)

	// Current humans re-inspect every queue item and acknowledge the changed-risk
	// digest. The old code acknowledgement remains visible as selectively stale.
	for area, assignment := range currentAssignments {
		workflowValue(t, server.URL, http.MethodPost, workBase+"/progress", tokens[assignment.ParticipantID], map[string]any{"expected_version": workspace.Version, "assignment_id": assignment.ID, "state": "complete", "queue_item_ids": areaItems(area), "coverage": []string{"repaired " + area + " evidence"}}, http.StatusCreated, &workspace)
	}
	for _, area := range []string{"code", "security", "accessibility", "consumer"} {
		assignment := currentAssignments[area]
		workflowValue(t, server.URL, http.MethodPost, completionBase+"/areas/"+area+"/acknowledgements", tokens[assignment.ParticipantID], map[string]any{"expected_version": completionView.Version, "assignment_id": assignment.ID, "decision": "approve", "rationale": "exact repaired candidate verified"}, http.StatusOK, &completionView)
	}
	if !completionView.Ready || len(completionView.Areas[0].StaleApprovals) != 1 {
		t.Fatalf("current completion and selective staleness missing: %#v", completionView)
	}
	waitForWorkflowCheck(t, server.URL, base, tokens["owner"], repairRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", tokens["owner"], `{"decision":"approve"}`, http.StatusOK, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", tokens["owner"], "", http.StatusOK, &readiness)
	if !readiness.Ready || readiness.ReviewCompletion == nil || !readiness.ReviewCompletion.Ready {
		t.Fatalf("exact candidate not ready: %#v", readiness)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/queue", tokens["owner"], "", http.StatusCreated, nil)
	entries := waitForQueueOutcomes(t, server.URL, string(repository.ID), tokens["owner"], map[string]string{pull.ID: "merged"})
	workflowJSON(t, server.URL, http.MethodGet, base, tokens["owner"], "", http.StatusOK, &pull)
	main, _ := opened.ReadReference("refs/heads/main")
	if pull.Status != pullrequests.Merged || pull.MergeCommitID == "" || string(main.ObjectID) != pull.MergeCommitID || len(entries[pull.ID].History) == 0 || workspace.Findings[0].ActorKind != "agent" || workspace.Verifications[0].Outcome != "failed" || workspace.Verifications[1].Outcome != "passed" {
		t.Fatalf("final review or integration history incomplete: pull=%#v entry=%#v work=%#v", pull, entries[pull.ID], workspace)
	}
}

func workflowRoutingBlocker(route reviewrouting.Routing, want string) bool {
	for _, suggestion := range route.Suggestions {
		for _, blocker := range suggestion.Blockers {
			if blocker == want {
				return true
			}
		}
	}
	return false
}
