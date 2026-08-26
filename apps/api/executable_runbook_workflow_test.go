package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookexecutions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestExecutableRunbookWorkflow is the black-box boundary for turning reviewed
// operational knowledge into rehearsal proof, a revision-exact live response,
// bounded human-agent execution, shift handoff, evaluated recovery, and a
// reviewed procedure correction. All mutations cross the public HTTP API.
func TestExecutableRunbookWorkflow(t *testing.T) {
	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	books, _ := runbooks.New(t.TempDir())
	rehearsals, _ := runbookrehearsals.New(t.TempDir())
	executions, _ := runbookexecutions.New(t.TempDir())
	mux := http.NewServeMux()
	registerRunbooksHTTP(mux, books, repos, credentials)
	registerRunbookRehearsalsHTTP(mux, rehearsals, books, repos, credentials)
	registerRunbookExecutionsHTTP(mux, executions, books, rehearsals, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	repo, _ := repos.Create("owner", repositories.Metadata{Name: "checkout-operations", Visibility: repositories.Private})
	actors := []string{"operator", "approver", "next-shift", "agent"}
	tokens := map[string]string{"owner": issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)}
	for _, actor := range actors {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	root := "/repositories/" + string(repo.ID)
	now := time.Now().UTC()

	var book runbooks.Runbook
	workflowValue(t, server.URL, http.MethodPost, root+"/runbooks", tokens["owner"], executableRunbookInput("service-v7", "initial reviewed procedure"), http.StatusCreated, &book)
	if len(book.Findings) != 0 || book.CurrentVersion != 1 || len(book.AuthorityPreview) != 4 {
		t.Fatalf("reviewed procedure was not publishable: %#v", book)
	}

	var rehearsal runbookrehearsals.Rehearsal
	rehearsalURL := root + "/runbooks/" + book.ID + "/rehearsals"
	workflowValue(t, server.URL, http.MethodPost, rehearsalURL, tokens["operator"], executableRehearsalInput(1), http.StatusCreated, &rehearsal)
	appendAttempt := func(actor, kind string, complete bool) {
		steps := executableRehearsalSteps(now)
		outcomes := []string{"diagnosis supported", "mitigation contained", "service recovered"}
		manual := []string{}
		if !complete {
			steps[2]["status"] = "failed"
			outcomes = outcomes[:2]
			manual = []string{"operator had to discover the verification window"}
		}
		workflowValue(t, server.URL, http.MethodPost, rehearsalURL+"/"+rehearsal.ID+"/attempts", tokens[actor], map[string]any{
			"expected_revision": rehearsal.Revision, "scenario_id": "degraded-checkout", "actor_kind": kind,
			"environment_revision": "sandbox-v4", "started_at": now.Add(-5 * time.Minute), "ended_at": now.Add(-time.Minute), "input_digest": "sha256:degraded-v7",
			"permissions": []map[string]any{{"capability": "telemetry:read", "resource_id": "checkout-sandbox", "granted": true, "authority_reference": "sandbox-policy-v4"}, {"capability": "deployment:rollback", "resource_id": "checkout-sandbox", "granted": true, "authority_reference": "sandbox-approval-81"}},
			"branches":    []map[string]any{{"step_id": "decide", "question": "rollback release v7?", "decision": "rollback", "actor_id": actor, "rationale": "synthetic errors correlate with release v7"}},
			"steps":       steps, "achieved_outcomes": outcomes, "manual_gaps": manual, "cost": 1.25, "currency": "USD",
		}, http.StatusCreated, &rehearsal)
	}
	appendAttempt("operator", "human", false)
	if rehearsal.Ready || rehearsal.Attempts[0].Proof {
		t.Fatal("manual operator gap was incorrectly accepted as rehearsal proof")
	}
	appendAttempt("agent", "agent", true)
	if !rehearsal.Ready || rehearsal.Attempts[1].ActorID != "agent" || len(rehearsal.NonAuthority) == 0 {
		t.Fatalf("bounded human-agent rehearsal did not produce current proof: %#v", rehearsal)
	}

	launch := executableLaunchInput(book, rehearsal, now)
	// A failed precondition remains a contained launch, not an executable one.
	blockedInput := cloneWorkflowMap(launch)
	blockedInput["idempotency_key"] = "alert-v7:failed-precondition"
	blockedInput["preconditions"] = []map[string]any{{"id": "impact-confirmed", "satisfied": false, "detail": "signal window has not been corroborated"}}
	var blocked runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions", tokens["operator"], blockedInput, http.StatusUnprocessableEntity, &blocked)
	if blocked.State != "blocked" || !executableBlocker(blocked, "precondition_failed") {
		t.Fatalf("failed precondition was not retained: %#v", blocked)
	}

	var execution runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions", tokens["operator"], launch, http.StatusCreated, &execution)
	var duplicate runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions", tokens["operator"], launch, http.StatusCreated, &duplicate)
	if duplicate.ID != execution.ID || execution.RunbookVersion != 1 || execution.Origin.Revision != "alert-revision-17" {
		t.Fatalf("duplicate or revision-exact alert context was lost: %#v %#v", execution, duplicate)
	}

	control := func(actor, action, step, key string, extra map[string]any, status int) {
		body := map[string]any{"expected_revision": execution.Revision, "idempotency_key": key, "action": action, "step_id": step}
		for k, v := range extra {
			body[k] = v
		}
		workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions/"+execution.ID+"/controls", tokens[actor], body, status, &execution)
	}
	control("approver", "join", "", "join-approver", map[string]any{"body": "independent production approver"}, http.StatusOK)
	control("next-shift", "join", "", "join-next", map[string]any{"body": "accepted frozen alert and procedure context"}, http.StatusOK)
	control("operator", "delegate", "diagnose", "delegate-analysis", map[string]any{"target_id": "agent", "mode": "analyze"}, http.StatusOK)
	control("agent", "perform", "diagnose", "unsafe-agent-perform", map[string]any{"health": "degraded", "evidence": []string{"agent:suggestion"}}, http.StatusForbidden)
	control("operator", "approve", "diagnose", "self-approval-denied", map[string]any{"body": "attempt to self-approve"}, http.StatusForbidden)
	control("approver", "approve", "diagnose", "approve-diagnosis", map[string]any{"body": "bounded telemetry read approved"}, http.StatusOK)
	control("operator", "perform", "diagnose", "perform-diagnosis", map[string]any{"body": "retry amplification begins at release v7", "health": "degraded", "evidence": []string{"alert:17", "trace:sanitized-44"}, "cost": 2.5}, http.StatusOK)
	control("approver", "approve", "decide", "approve-decision", map[string]any{"body": "human rollback decision approved"}, http.StatusOK)
	control("operator", "perform", "decide", "perform-decision", map[string]any{"body": "choose bounded rollback", "health": "degraded", "evidence": []string{"decision:rollback-v6"}}, http.StatusOK)
	control("approver", "approve", "mitigate", "approve-mitigation", map[string]any{"body": "ordinary deployment approval 93"}, http.StatusOK)
	control("operator", "perform", "mitigate", "failed-rollback", map[string]any{"body": "rollback command was interrupted after unhealthy canary", "health": "unhealthy", "evidence": []string{"deployment:93", "canary:unhealthy"}, "cost": 3.75}, http.StatusOK)
	if execution.State != "paused" || execution.RollbackState != "required" || execution.Credentials[len(execution.Credentials)-1].RevokedAt == nil {
		t.Fatalf("failed rollback or credential revocation escaped containment: %#v", execution)
	}
	control("operator", "discuss", "mitigate", "record-interruption", map[string]any{"body": "interrupted step retained; authorized operator completed rollback from current state"}, http.StatusOK)
	control("operator", "resume", "", "resume-current", map[string]any{"body": "current deployment evidence supports recovery verification"}, http.StatusOK)
	control("operator", "handoff", "", "handoff-shift", map[string]any{"target_id": "next-shift", "body": "unchanged alert, release, approvals, failed rollback, costs, and evidence accepted"}, http.StatusOK)
	control("approver", "approve", "verify", "approve-verification", map[string]any{"body": "independent recovery query approved"}, http.StatusOK)
	control("next-shift", "perform", "verify", "perform-verification", map[string]any{"body": "errors recovered but the documented observation window was too short", "health": "healthy", "evidence": []string{"metric:recovered", "users:recovered"}, "cost": 0.5}, http.StatusOK)
	if execution.State != "completed" || execution.ControllerID != "next-shift" || execution.Cost != 6.75 {
		t.Fatalf("handoff and bounded recovery trail incomplete: %#v", execution)
	}

	criteria := []map[string]any{
		{"kind": "health", "status": "met", "evidence": []string{"metric:recovered"}},
		{"kind": "containment", "status": "met", "evidence": []string{"canary:unhealthy", "execution:paused"}},
		{"kind": "recovery", "status": "unmet", "evidence": []string{"metric:short-window"}, "detail": "procedure omitted the full recovery observation window"},
		{"kind": "communication", "status": "met", "evidence": []string{"handoff:next-shift"}},
		{"kind": "rollback", "status": "met", "evidence": []string{"deployment:93"}},
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions/"+execution.ID+"/evaluation", tokens["next-shift"], map[string]any{
		"expected_revision": execution.Revision, "idempotency_key": "evaluation-17", "disposition": "completed", "criteria": criteria,
		"deviations": []string{"rollback was interrupted and resumed manually"}, "manual_work": []string{"operator supplied the missing recovery window"}, "failed_steps": []string{"mitigate"},
		"access_gaps": []string{}, "agent_corrections": []string{"analyze-only suggestion was prevented from performing mitigation"},
		"participant_feedback": []map[string]any{{"participant_id": "next-shift", "body": "frozen context made the handoff reviewable"}},
	}, http.StatusOK, &execution)
	if execution.Evaluation == nil || execution.Evaluation.OutcomeProven || len(execution.Evaluation.Findings) < 4 {
		t.Fatalf("unmet recovery and correction evidence disappeared: %#v", execution.Evaluation)
	}

	finding := execution.Evaluation.Findings[0]
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions/"+execution.ID+"/learning", tokens["owner"], map[string]any{
		"expected_revision": execution.Revision, "idempotency_key": "link-code-fix", "action": "link_improvement", "finding_id": finding.ID,
		"improvement_kind": "code", "improvement_reference": "pull-request:checkout-recovery-window@reviewed", "improvement_owner": "agent",
	}, http.StatusOK, &execution)

	// The reviewed update changes both procedure and referenced implementation.
	var revised runbooks.Runbook
	revisionInput := executableRunbookInput("service-v7", "reviewed code and recovery-window correction")
	revisionInput["expected_version"] = int64(1)
	workflowValue(t, server.URL, http.MethodPost, root+"/runbooks/"+book.ID+"/versions", tokens["owner"], revisionInput, http.StatusCreated, &revised)

	// Version one is now stale and cannot be launched even with its old proof.
	stale := executableLaunchInput(book, rehearsal, now)
	stale["idempotency_key"] = "alert-v18:stale-v1"
	stale["origin"] = map[string]any{"kind": "alert", "resource_id": "checkout-alert-18", "revision": "alert-revision-18", "timeline_reference": "/response-alerts/checkout-alert-18#timeline", "audience": "participants"}
	var staleExecution runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions", tokens["next-shift"], stale, http.StatusUnprocessableEntity, &staleExecution)
	if !executableBlocker(staleExecution, "stale_or_missing_rehearsal") {
		t.Fatalf("stale procedure was executable: %#v", staleExecution)
	}

	var fresh runbookrehearsals.Rehearsal
	workflowValue(t, server.URL, http.MethodPost, rehearsalURL, tokens["operator"], executableRehearsalInput(2), http.StatusCreated, &fresh)
	steps := executableRehearsalSteps(now)
	workflowValue(t, server.URL, http.MethodPost, rehearsalURL+"/"+fresh.ID+"/attempts", tokens["agent"], map[string]any{
		"expected_revision": fresh.Revision, "scenario_id": "degraded-checkout", "actor_kind": "agent", "environment_revision": "sandbox-v4", "started_at": now.Add(-4 * time.Minute), "ended_at": now, "input_digest": "sha256:degraded-v7",
		"permissions": []map[string]any{{"capability": "telemetry:read", "resource_id": "checkout-sandbox", "granted": true, "authority_reference": "sandbox-policy-v4"}}, "steps": steps,
		"achieved_outcomes": []string{"diagnosis supported", "mitigation contained", "service recovered"}, "manual_gaps": []string{}, "cost": 1, "currency": "USD",
	}, http.StatusCreated, &fresh)
	if !fresh.Ready {
		t.Fatalf("corrected revision lacks fresh passing proof: %#v", fresh)
	}
	for _, learning := range []map[string]any{
		{"idempotency_key": "record-reviewed-v2", "action": "record_revision", "reviewed_runbook_version": int64(2)},
		{"idempotency_key": "record-fresh-v2", "action": "record_fresh_rehearsal", "fresh_rehearsal_id": fresh.ID, "fresh_rehearsal_revision": fresh.Revision},
	} {
		learning["expected_revision"] = execution.Revision
		workflowValue(t, server.URL, http.MethodPost, root+"/runbook-executions/"+execution.ID+"/learning", tokens["owner"], learning, http.StatusOK, &execution)
	}
	if execution.ReviewedRunbookVersion != 2 || execution.FreshRehearsalID != fresh.ID || len(execution.NonAuthority) == 0 {
		t.Fatalf("continuous improvement trail is incomplete: %#v", execution)
	}
}

func executableRunbookInput(scopeRevision, reason string) map[string]any {
	ref := func(kind, id, revision, detail string) map[string]any {
		return map[string]any{"kind": kind, "resource_id": id, "revision": revision, "detail": detail, "accessible": true, "reviewed": true, "secret_bearing": false, "owner_id": "owner"}
	}
	pre := []string{"impact-confirmed"}
	return map[string]any{
		"name": "Recover degraded checkout", "purpose": "diagnose and safely roll back a degraded checkout release", "scope": map[string]any{"kind": "service", "resource_id": "checkout", "revision": scopeRevision, "owner_id": "owner"},
		"preconditions": []map[string]any{{"id": "impact-confirmed", "description": "confirm revision-exact user impact", "evidence": "alert signal window", "owner_id": "operator", "safe": true}},
		"steps": []map[string]any{
			{"id": "diagnose", "kind": "diagnostic", "title": "Diagnose retry amplification", "purpose": "identify the failing release", "precondition_ids": pre, "references": []map[string]any{ref("command", "checkout-errors", "sha256:query-v4", "sanitized telemetry query"), ref("agent", "ops-agent", "sha256:agent-v5", "approved analysis-only agent")}, "expected_evidence": []string{"revision-bound trace", "alert window"}, "required_authority": []string{"telemetry:read"}, "owner_ids": []string{"operator"}, "required_skills": []string{"diagnosis"}},
			{"id": "decide", "kind": "decision", "title": "Choose mitigation", "purpose": "require accountable human judgment", "precondition_ids": pre, "expected_evidence": []string{"approved decision"}, "decision": map[string]any{"question": "roll back release v7?", "options": []string{"rollback", "escalate"}, "human_required": true, "owner_id": "approver"}, "required_authority": []string{}, "owner_ids": []string{"approver"}, "required_skills": []string{"incident-command"}, "depends_on": []string{"diagnose"}},
			{"id": "mitigate", "kind": "action", "title": "Roll back release", "purpose": "perform bounded mitigation", "precondition_ids": pre, "references": []map[string]any{ref("workflow_component", "deployment-rollback", "sha256:component-v6", "reviewed rollback component")}, "expected_evidence": []string{"deployment receipt", "canary health"}, "required_authority": []string{"deployment:rollback"}, "owner_ids": []string{"operator"}, "required_skills": []string{"rollback"}, "depends_on": []string{"decide"}, "rollback_criteria": []string{"canary remains unhealthy"}},
			{"id": "verify", "kind": "diagnostic", "title": "Verify recovery", "purpose": "observe service and user recovery", "precondition_ids": pre, "references": []map[string]any{ref("documentation", "recovery-window", "git:reviewed-fix", "reviewed recovery observation window")}, "expected_evidence": []string{"health window", "user recovery"}, "required_authority": []string{"telemetry:read"}, "owner_ids": []string{"next-shift"}, "required_skills": []string{"verification"}, "depends_on": []string{"mitigate"}},
		},
		"rollback_criteria": []string{"canary health worsens"}, "health_criteria": []string{"error rate below one percent for fifteen minutes"}, "containment_criteria": []string{"unhealthy mitigation pauses execution"}, "recovery_criteria": []string{"affected users recover through a complete observation window"}, "communication_criteria": []string{"next shift accepts exact context"},
		"owner_ids": []string{"owner"}, "required_skills": []string{"diagnosis", "rollback", "verification"}, "escalation_paths": []map[string]any{{"condition": "rollback cannot restore health", "owner_id": "owner", "team_id": "checkout", "required_skills": []string{"incident-command"}, "audience_ids": []string{"service-owners", "users"}, "action": "escalate to incident"}},
		"policy_references": []map[string]any{{"kind": "production", "resource_id": "checkout", "revision": "policy-v9", "accessible": true, "conflicting": false, "owner_id": "owner"}}, "change_reason": reason,
	}
}

func executableRehearsalInput(version int64) map[string]any {
	return map[string]any{"runbook_version": version, "title": "Degraded checkout recovery", "environment_id": "checkout-sandbox", "environment_revision": "sandbox-v4", "environment_class": "isolated", "limits": map[string]any{"max_duration_seconds": 600, "max_cost": 10, "currency": "USD"}, "scenarios": []map[string]any{{"id": "degraded-checkout", "name": "retry amplification", "failure": "synthetic elevated checkout errors", "evidence_source": "synthetic", "input_digest": "sha256:degraded-v7", "expected_outcomes": []string{"diagnosis supported", "mitigation contained", "service recovered"}, "references": []map[string]any{{"kind": "service", "resource_id": "checkout", "revision": "service-v7"}, {"kind": "policy", "resource_id": "production", "revision": "policy-v9"}}}}, "owner_ids": []string{"operator", "owner"}}
}

func executableRehearsalSteps(now time.Time) []map[string]any {
	step := func(id string, destructive bool) map[string]any {
		x := map[string]any{"step_id": id, "status": "completed", "command": "reviewed:" + id, "output": "sanitized:" + id, "started_at": now.Add(-4 * time.Minute), "ended_at": now.Add(-3 * time.Minute), "artifact_digests": []string{"sha256:" + id}, "destructive": destructive}
		if destructive {
			x["destructive_handling"] = "simulated"
		}
		return x
	}
	return []map[string]any{step("diagnose", false), step("mitigate", true), step("verify", false)}
}

func executableLaunchInput(book runbooks.Runbook, rehearsal runbookrehearsals.Rehearsal, now time.Time) map[string]any {
	return map[string]any{"idempotency_key": "alert-v17:checkout-recovery", "runbook_id": book.ID, "runbook_version": int64(1), "origin": map[string]any{"kind": "alert", "resource_id": "checkout-alert-17", "revision": "alert-revision-17", "timeline_reference": "/response-alerts/checkout-alert-17#timeline", "audience": "participants"}, "affected_resources": []string{"service:checkout", "release:v7"}, "signal_window": map[string]any{"started_at": now.Add(-10 * time.Minute), "ended_at": now}, "context": []map[string]any{{"kind": "release", "resource_id": "checkout-release-v7", "revision": "commit-v7", "permitted": true, "audience": "participants", "accessible": true}, {"kind": "alert", "resource_id": "checkout-alert-17", "revision": "alert-revision-17", "permitted": true, "audience": "participants", "accessible": true}}, "preconditions": []map[string]any{{"id": "impact-confirmed", "satisfied": true, "evidence_reference": "alert:17"}}, "access": []map[string]any{{"capability": "telemetry:read", "resource_id": "checkout", "granted": true, "authority_reference": "on-call-policy-v9"}, {"capability": "deployment:rollback", "resource_id": "checkout", "granted": true, "authority_reference": "deployment-approval-93"}}, "match_explanation": []string{"exact service and required skills match"}, "rehearsal_id": rehearsal.ID, "rehearsal_revision": rehearsal.Revision}
}

func executableBlocker(x runbookexecutions.Execution, kind string) bool {
	for _, blocker := range x.Blockers {
		if blocker.Kind == kind {
			return true
		}
	}
	return false
}

func cloneWorkflowMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
