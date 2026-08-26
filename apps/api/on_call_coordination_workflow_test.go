package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responseoutcomes"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestOnCallCoordinationWorkflow is the black-box boundary for moving a released
// service signal through policy-pinned duty, bounded human-agent investigation,
// ordinary mitigation evidence, an accepted handoff, incident promotion, and
// reviewed learning. Every correction and containment action uses public HTTP.
func TestOnCallCoordinationWorkflow(t *testing.T) {
	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	policies, _ := responsepolicies.New(t.TempDir())
	rotations, _ := responserotations.New(t.TempDir())
	alerts, _ := responsealerts.New(t.TempDir())
	outcomes, _ := responseoutcomes.New(t.TempDir())
	incidentRecords, _ := incidents.New(t.TempDir())
	mux := http.NewServeMux()
	registerResponsePoliciesHTTP(mux, policies, repos, credentials)
	registerResponseRotationsHTTP(mux, rotations, repos, credentials)
	registerResponseAlertsHTTP(mux, alerts, policies, rotations, incidentRecords, repos, credentials)
	registerResponseOutcomesHTTP(mux, outcomes, alerts, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	repo, _ := repos.Create("owner", repositories.Metadata{Name: "released-checkout", Visibility: repositories.Private})
	actors := []string{"primary", "backup", "dependency-owner", "approver"}
	tokens := map[string]string{"owner": issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)}
	for _, actor := range actors {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	root := "/repositories/" + string(repo.ID)
	now := time.Now().UTC()

	var policy responsepolicies.Policy
	workflowValue(t, server.URL, http.MethodPost, root+"/response-policies", tokens["owner"], map[string]any{
		"name": "Checkout production response", "description": "Coverage for the released checkout service and its database dependency.",
		"resources": []map[string]any{{"kind": "service", "id": "checkout", "owner_team_ids": []string{"checkout-ops"}, "required": true}, {"kind": "dependency", "id": "ledger-db", "owner_team_ids": []string{"database"}, "required": true}},
		"teams": []map[string]any{
			{"id": "checkout-ops", "name": "Checkout operators", "member_ids": []string{"primary", "backup"}, "skills": []string{"service", "rollback"}, "available": true, "authority": []string{"observe production", "request approved deployment rollback"}},
			{"id": "database", "name": "Database owners", "member_ids": []string{"dependency-owner"}, "skills": []string{"database"}, "available": true, "authority": []string{"inspect dependency telemetry"}},
		},
		"coverage": []map[string]any{
			{"id": "checkout-high", "resource_kind": "service", "resource_id": "checkout", "signal_class": "reliability", "severity": "high", "team_id": "checkout-ops", "required_skills": []string{"service"}, "response_target": map[string]any{"acknowledge_minutes": 2, "engage_minutes": 5, "update_minutes": 10}, "escalation_path": []map[string]any{{"after_minutes": 5, "team_id": "checkout-ops", "audience_ids": []string{"service-owners"}, "action": "activate backup"}}, "communication_audience_ids": []string{"service-owners"}, "expected_actions": []string{"inspect released revision", "request authorized rollback"}, "incident_criteria": []string{"recurrence affects more than 100 users"}},
			{"id": "checkout-critical", "resource_kind": "service", "resource_id": "checkout", "signal_class": "reliability", "severity": "critical", "team_id": "checkout-ops", "required_skills": []string{"service", "rollback"}, "response_target": map[string]any{"acknowledge_minutes": 1, "engage_minutes": 2, "update_minutes": 5}, "escalation_path": []map[string]any{{"after_minutes": 2, "team_id": "checkout-ops", "audience_ids": []string{"users"}, "action": "create incident"}}, "communication_audience_ids": []string{"service-owners", "users"}, "expected_actions": []string{"create incident", "restore service"}, "incident_criteria": []string{"severe recurrence"}},
			{"id": "dependency-medium", "resource_kind": "dependency", "resource_id": "ledger-db", "signal_class": "dependency", "severity": "medium", "team_id": "database", "required_skills": []string{"database"}, "response_target": map[string]any{"acknowledge_minutes": 5, "engage_minutes": 10, "update_minutes": 15}, "communication_audience_ids": []string{"service-owners"}, "expected_actions": []string{"validate dependency evidence"}, "incident_criteria": []string{"confirmed user impact"}},
		},
		"rule_references": []map[string]any{{"kind": "service_ownership", "resource_id": "catalog:checkout", "revision": "owners-v7", "required": true, "accessible": true, "owner_id": "owner"}, {"kind": "access", "resource_id": "production", "revision": "environment-policy-v4", "required": true, "accessible": true, "owner_id": "owner"}},
		"owner_ids":       []string{"owner", "approver"}, "change_reason": "released checkout service needs accountable response",
	}, http.StatusCreated, &policy)

	var rotation responserotations.Rotation
	workflowValue(t, server.URL, http.MethodPost, root+"/response-rotations", tokens["owner"], map[string]any{
		"name": "Checkout follow-the-sun", "policy_id": policy.ID, "policy_version": policy.CurrentVersion, "team_id": "checkout-ops", "time_zone": "UTC", "handoff_minutes": 15, "workload_limit": 3,
		"participants": []map[string]any{
			{"user_id": "primary", "time_zone": "UTC", "qualifications": []string{"service", "rollback"}, "available": true, "member": true, "access": true, "workload": 1},
			{"user_id": "backup", "time_zone": "UTC", "qualifications": []string{"service", "rollback"}, "available": true, "member": true, "access": true, "workload": 0},
			{"user_id": "absent", "time_zone": "UTC", "qualifications": []string{"service"}, "available": false, "member": true, "access": true, "workload": 0},
			{"user_id": "revoked", "time_zone": "UTC", "qualifications": []string{"service"}, "available": true, "member": true, "access": false, "workload": 0},
		},
		"absence_rules": []map[string]any{{"kind": "unplanned", "notice_minutes": 0, "action": "activate first eligible backup"}},
		"shifts": []map[string]any{
			{"id": "current", "starts_at": now.Add(-time.Hour), "ends_at": now.Add(time.Hour), "primary_id": "primary", "backup_layers": [][]string{{"backup"}}, "required_qualifications": []string{"service", "rollback"}, "context_revision": "handoff-v7", "context_references": []string{"release:v7", "runbook:v3", "alert:active"}},
			{"id": "absent-shift", "starts_at": now.Add(2 * time.Hour), "ends_at": now.Add(3 * time.Hour), "primary_id": "absent", "backup_layers": [][]string{{"backup"}}, "required_qualifications": []string{"service"}, "context_revision": "handoff-v8", "context_references": []string{"release:v7", "runbook:v3"}},
			{"id": "revoked-shift", "starts_at": now.Add(3 * time.Hour), "ends_at": now.Add(4 * time.Hour), "primary_id": "revoked", "backup_layers": [][]string{{"backup"}}, "required_qualifications": []string{"service"}, "context_revision": "handoff-v9", "context_references": []string{"release:v7", "runbook:v3"}},
		}, "owner_ids": []string{"owner"},
	}, http.StatusCreated, &rotation)
	if !onCallGap(rotation, "unavailable_responder") || !onCallGap(rotation, "access_revoked") {
		t.Fatalf("absence or revoked access was hidden: %#v", rotation.Gaps)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/response-rotations/"+rotation.ID+"/events", tokens["primary"], map[string]any{"expected_revision": rotation.Revision, "shift_id": "current", "kind": "acknowledged", "detail": "inspected release, runbook, and active response context"}, http.StatusCreated, &rotation)

	createSignal := func(severity, class, resource, correlation, revision string, observed time.Time, users int, token string) responsealerts.Alert {
		var alert responsealerts.Alert
		workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts", token, map[string]any{"policy_id": policy.ID, "signal": map[string]any{"signal_class": class, "severity": severity, "resource_kind": map[string]string{"reliability": "service", "dependency": "dependency"}[class], "resource_id": resource, "revision": revision, "observed_at": observed, "correlation_key": correlation, "summary": "checkout evidence crossed its response threshold", "affected_resources": []string{"release:v7", resource}, "affected_user_count": users, "evidence": []map[string]any{{"kind": "metric", "reference": "otel:checkout-errors", "revision": revision, "accessible": true, "summary": "sanitized error window"}}, "uncertainty": "dependency contribution is noisy"}, "rate_limit_per_hour": 20}, http.StatusCreated, &alert)
		return alert
	}

	alert := createSignal("high", "reliability", "checkout", "checkout:errors:window-7", "signal-v7", now.Add(-3*time.Minute), 48, tokens["owner"])
	duplicate := createSignal("high", "reliability", "checkout", "checkout:errors:window-7", "signal-v7-duplicate", now, 48, tokens["owner"])
	if duplicate.ID != alert.ID || duplicate.DuplicateCount != 1 || duplicate.Revision != 2 {
		t.Fatalf("duplicate created another page: %#v %#v", alert, duplicate)
	}
	alert = duplicate
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/routing-attempts", tokens["owner"], map[string]any{"expected_revision": alert.Revision, "recipient_id": "primary", "channel": "inbox", "status": "failed", "reason": "transient inbox delivery failure", "policy_version": policy.CurrentVersion}, http.StatusCreated, &alert)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/routing-attempts", tokens["owner"], map[string]any{"expected_revision": alert.Revision, "recipient_id": "primary", "channel": "inbox", "status": "delivered", "reason": "retry reached actionable inbox", "policy_version": policy.CurrentVersion}, http.StatusCreated, &alert)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace", tokens["primary"], map[string]any{"expected_revision": alert.Revision, "context": []map[string]any{{"kind": "release", "resource_id": "release:v7", "revision": "commit-v7", "permitted": true, "audience": "participants"}, {"kind": "deployment", "resource_id": "deployment:rollback-v6-approved", "revision": "deployment-event-91", "permitted": true, "audience": "participants"}, {"kind": "dependency", "resource_id": "ledger-db", "revision": "telemetry-v12", "permitted": true, "audience": "participants"}, {"kind": "runbook", "resource_id": "runbook:v3", "revision": "blob-v3", "permitted": true, "audience": "participants"}}}, http.StatusCreated, &alert)
	if alert.ResponseDeadline == nil || alert.Workspace == nil || !alert.Workspace.OpenedAt.After(*alert.ResponseDeadline) {
		t.Fatalf("missed acknowledgement target was not retained: %#v", alert)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/actions", tokens["primary"], map[string]any{"expected_revision": alert.Revision, "kind": "invite", "detail": "dependency owner validates the noisy database signal", "assignee_id": "dependency-owner"}, http.StatusCreated, &alert)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/diagnostics", tokens["dependency-owner"], map[string]any{"expected_revision": alert.Revision, "name": "compare dependency saturation", "command_reference": "runbook:v3#dependency", "context_references": []string{"ledger-db", "runbook:v3"}, "approved_by_id": "primary", "sanitized_output": "database saturation normal; retry amplification is in release:v7"}, http.StatusCreated, &alert)
	var delegated struct {
		Alert      responsealerts.Alert `json:"alert"`
		Credential string               `json:"credential"`
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/agents", tokens["primary"], map[string]any{"expected_revision": alert.Revision, "agent": "response-investigator@sha256:v4", "mandate": "compare the released retry path within a two-dollar budget", "context_references": []string{"release:v7", "ledger-db", "runbook:v3"}}, http.StatusCreated, &delegated)
	alert = delegated.Alert
	workflowValue(t, server.URL, http.MethodPost, "/response-alert-investigations/records", delegated.Credential, map[string]any{"kind": "finding", "body": "release:v7 retry fan-out explains the errors; database evidence is noisy, not proof", "evidence_references": []string{"release:v7", "ledger-db"}}, http.StatusCreated, nil)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/actions", delegated.Credential, map[string]any{"expected_revision": alert.Revision + 1, "kind": "resolve", "detail": "agent attempts production control"}, http.StatusUnauthorized, nil)
	workflowValue(t, server.URL, http.MethodGet, root+"/response-alerts/"+alert.ID, tokens["primary"], nil, http.StatusOK, &alert)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/actions", tokens["primary"], map[string]any{"expected_revision": alert.Revision, "kind": "observe", "detail": "authorized operator completed the independently approved rollback; health evidence is passing", "audience": "participants", "evidence_references": []string{"deployment:rollback-v6-approved", "deployment-event-91"}}, http.StatusCreated, &alert)

	workflowValue(t, server.URL, http.MethodPost, root+"/response-rotations/"+rotation.ID+"/transfers", tokens["primary"], map[string]any{"expected_revision": rotation.Revision, "shift_id": "current", "kind": "delegate", "recipient_id": "backup", "context_revision": "handoff-v7", "context_references": []string{"release:v7", "runbook:v3", "alert:active"}, "rationale": "continue verification after shift boundary with the unchanged context"}, http.StatusCreated, &rotation)
	transfer := rotation.Transfers[len(rotation.Transfers)-1]
	workflowValue(t, server.URL, http.MethodPost, root+"/response-rotations/"+rotation.ID+"/transfers/"+transfer.ID+"/accept", tokens["backup"], map[string]any{"expected_revision": rotation.Revision}, http.StatusOK, &rotation)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/actions", tokens["primary"], map[string]any{"expected_revision": alert.Revision, "kind": "reassign", "detail": "backup accepted unchanged release, runbook, signal, mitigation, and agent context", "assignee_id": "backup"}, http.StatusCreated, &alert)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+alert.ID+"/workspace/actions", tokens["backup"], map[string]any{"expected_revision": alert.Revision, "kind": "resolve", "detail": "passing rollback evidence held through the verification window"}, http.StatusCreated, &alert)

	// The noisy dependency page remains a visible false positive instead of proof.
	noisy := createSignal("medium", "dependency", "ledger-db", "ledger-db:noisy:12", "dependency-v12", now, 0, tokens["dependency-owner"])
	if noisy.Status != "delivery_failed" || !strings.Contains(strings.Join(noisy.Gaps, " "), "no current") {
		t.Fatalf("uncovered dependency delivery was hidden: %#v", noisy)
	}
	var noisyOutcome responseoutcomes.Outcome
	workflowValue(t, server.URL, http.MethodPost, root+"/response-outcomes", tokens["owner"], map[string]any{"alert_id": noisy.ID, "expected_alert_revision": noisy.Revision, "summary": "dependency telemetry was noisy", "resolution": "classified as a false positive after owner inspection", "audience": "repository", "evidence_references": []string{"ledger-db", "otel:checkout-errors"}, "responder_minutes": 8, "agent_cost": 0, "recovered_users": 0, "false_positive": true, "interruptions": 1, "owners": []string{"owner", "approver"}}, http.StatusCreated, &noisyOutcome)

	// A severe recurrence gets its own exact signal window and ordinary incident.
	recurrence := createSignal("critical", "reliability", "checkout", "checkout:errors:recurrence-8", "signal-v8", now, 240, tokens["owner"])
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+recurrence.ID+"/routing-attempts", tokens["owner"], map[string]any{"expected_revision": recurrence.Revision, "recipient_id": "backup", "channel": "inbox", "status": "delivered", "policy_version": policy.CurrentVersion}, http.StatusCreated, &recurrence)
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+recurrence.ID+"/workspace", tokens["backup"], map[string]any{"expected_revision": recurrence.Revision, "context": []map[string]any{{"kind": "release", "resource_id": "release:v7", "revision": "commit-v7", "permitted": true, "audience": "participants"}, {"kind": "runbook", "resource_id": "runbook:v3", "revision": "blob-v3", "permitted": true, "audience": "participants"}, {"kind": "alert", "resource_id": alert.ID, "revision": "resolved-alert-v1", "permitted": true, "audience": "participants"}}}, http.StatusCreated, &recurrence)
	var promoted struct {
		Alert    responsealerts.Alert `json:"alert"`
		Incident incidents.Incident   `json:"incident"`
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+recurrence.ID+"/workspace/incident", tokens["backup"], map[string]any{"expected_revision": recurrence.Revision, "title": "Checkout retry recurrence", "summary": "severe recurrence after the first bounded recovery", "severity": "critical", "roles": map[string]string{"commander": "owner", "operations": "backup"}, "affected": []map[string]any{{"repository_id": string(repo.ID), "environment_id": "production"}}}, http.StatusCreated, &promoted)
	recurrence = promoted.Alert
	workflowValue(t, server.URL, http.MethodPost, root+"/response-alerts/"+recurrence.ID+"/workspace/actions", tokens["backup"], map[string]any{"expected_revision": recurrence.Revision, "kind": "resolve", "detail": "incident owns continuing communication and recovery"}, http.StatusCreated, &recurrence)

	var outcome responseoutcomes.Outcome
	workflowValue(t, server.URL, http.MethodPost, root+"/response-outcomes", tokens["owner"], map[string]any{"alert_id": recurrence.ID, "expected_alert_revision": recurrence.Revision, "summary": "review the severe checkout recurrence", "resolution": "connected incident retained mitigation and user recovery", "audience": "repository", "user_outcome_consent": true, "user_outcome": "240 affected users recovered after approved mitigation", "evidence_references": []string{"incident:" + promoted.Incident.ID, "deployment:rollback-v6-approved", "release:v7", "runbook:v3"}, "responder_minutes": 44, "agent_cost": 7.5, "recovered_users": 240, "interruptions": 2, "unsafe_automation": true, "owners": []string{"owner", "approver"}}, http.StatusCreated, &outcome)
	if outcome.Metrics.IncidentCount != 1 || len(outcome.Controls) == 0 || outcome.Controls[0].Kind != "routing_paused" {
		t.Fatalf("incident, agent budget breach, or scoped containment was lost: %#v", outcome)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/response-outcomes/"+outcome.ID+"/reviews", tokens["owner"], map[string]any{"expected_revision": outcome.Revision, "decision": "confirmed", "rationale": "incident, deployment, signal, cost, and user evidence agree"}, http.StatusCreated, &outcome)
	for _, work := range []map[string]any{
		{"kind": "reliability", "owner_kind": "agent", "owner_id": "signal-agent", "resource_id": "task:checkout-signal-v8", "summary": "reduce retry-amplification noise with reviewed revision-exact signal logic"},
		{"kind": "documentation", "owner_kind": "human", "owner_id": "backup", "resource_id": "task:runbook-v4", "summary": "add approved rollback and dependency-validation steps to the runbook"},
	} {
		work["expected_revision"] = outcome.Revision
		workflowValue(t, server.URL, http.MethodPost, root+"/response-outcomes/"+outcome.ID+"/work", tokens["owner"], work, http.StatusCreated, &outcome)
	}
	if len(outcome.Reviews) != 1 || len(outcome.Work) != 2 || outcome.Work[0].OwnerKind != "agent" || outcome.Work[1].OwnerKind != "human" {
		t.Fatalf("reviewed signal and runbook improvement work was not retained: %#v", outcome)
	}
}

func onCallGap(rotation responserotations.Rotation, kind string) bool {
	for _, gap := range rotation.Gaps {
		if gap.Kind == kind {
			return true
		}
	}
	return false
}
