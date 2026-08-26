package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacitydeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacitymodels"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestCapacityPlanningWorkflow is the black-box boundary for moving an accepted
// product outcome through a challenged forecast, bounded comparison, separately
// authorized delivery work, and progressive production proof. Public HTTP and
// stock Git retain every failed prediction and containment action.
func TestCapacityPlanningWorkflow(t *testing.T) {
	requireGit(t)
	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	objectives, _ := capacityobjectives.New(t.TempDir())
	models, _ := capacitymodels.New(t.TempDir())
	rehearsals, _ := capacityrehearsals.New(t.TempDir())
	plans, _ := capacityplans.New(t.TempDir())
	deliveries, _ := capacitydeliveries.New(t.TempDir())
	mux := http.NewServeMux()
	registerCapacityObjectivesHTTP(mux, objectives, repos, credentials)
	registerCapacityModelsHTTP(mux, models, repos, credentials)
	registerCapacityRehearsalsHTTP(mux, rehearsals, repos, credentials)
	registerCapacityPlansHTTP(mux, plans, repos, credentials)
	registerCapacityDeliveriesHTTP(mux, deliveries, plans, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	repo, _ := repos.Create("owner", repositories.Metadata{Name: "capacity-loop", Visibility: repositories.Private})
	actors := []string{"product", "agent", "dependency-owner", "operator"}
	tokens := map[string]string{"owner": issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)}
	for _, actor := range actors {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	root := "/repositories/" + string(repo.ID)
	now := time.Now().UTC()

	var objective capacityobjectives.Objective
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-objectives", tokens["product"], map[string]any{
		"subject_kind": "user_journey", "subject_id": "roadmap:team-launch", "title": "Serve accepted team launch", "description": "Capacity must arrive before the accepted roadmap outcome.",
		"demand_forecasts":      []map[string]any{{"id": "accepted-launch", "segment": "active teams", "demand": 1800, "unit": "requests/second", "starts_at": now.Add(30 * 24 * time.Hour), "ends_at": now.Add(60 * 24 * time.Hour), "confidence": "estimated", "evidence": []string{"roadmap:accepted"}}},
		"traffic_shapes":        []map[string]any{{"name": "launch", "pattern": "bursty", "peak_multiplier": 2.0, "duration_minutes": 30}},
		"seasonality":           []map[string]any{{"name": "weekday", "schedule": "Mon-Fri", "multiplier": 1.2}},
		"service_levels":        []map[string]any{{"id": "latency", "kind": "latency", "scope": "api", "operator": "at_most", "value": 200, "unit": "ms", "source": "slo:api"}},
		"bottleneck_thresholds": []map[string]any{{"id": "pool", "kind": "connections", "scope": "database", "operator": "at_most", "value": 100, "unit": "connections"}},
		"dependency_limits":     []map[string]any{{"dependency": "database", "metric": "connections", "maximum": 100, "unit": "connections", "owner_id": "dependency-owner"}},
		"regions":               []string{"eu-west-1", "us-east-1"}, "owner_ids": []string{"product", "operator"}, "budget_amount": 5000, "budget_currency": "USD", "lead_time_days": 30,
		"signals":          []map[string]any{{"name": "request rate", "required": true, "owner_id": "operator"}},
		"assumptions":      []map[string]any{{"id": "growth", "statement": "accepted launch doubles traffic", "owner_id": "product", "expires_at": now.Add(14 * 24 * time.Hour), "evidence": "roadmap:accepted"}},
		"success_criteria": []string{"at least 25 percent usable headroom"}, "rollback_criteria": []string{"p95 exceeds 200ms"},
		"links": []map[string]string{{"kind": "product_roadmap", "resource_id": "roadmap:accepted"}, {"kind": "service_objective", "resource_id": "slo:api"}, {"kind": "funding", "resource_id": "budget:launch"}}, "change_reason": "accepted roadmap outcome",
	}, http.StatusCreated, &objective)

	modelInput := capacityWorkflowModelInput(objective.ID, objective.CurrentVersion, "release-base", now, "read_only_agent")
	var badModel capacitymodels.Model
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-models", tokens["agent"], modelInput, http.StatusCreated, &badModel)
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-models/"+badModel.ID+"/challenges", tokens["dependency-owner"], map[string]any{"expected_revision": 1, "conclusion_id": "pool", "body": "Failover traffic was counted twice; the dependency owner rejects this forecast.", "evidence_ids": []string{"usage"}, "author_kind": "human"}, http.StatusCreated, &badModel)
	correctedInput := capacityWorkflowModelInput(objective.ID, objective.CurrentVersion, "release-base", now, "read_only_agent")
	correctedInput["supersedes_model_id"] = badModel.ID
	correctedInput["method"] = "dependency-owner corrected segmented forecast"
	var model capacitymodels.Model
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-models", tokens["agent"], correctedInput, http.StatusCreated, &model)
	if model.SupersedesModelID != badModel.ID || len(badModel.Challenges) != 1 {
		t.Fatal("bad forecast was not retained and explicitly corrected")
	}

	rehearsalInput := map[string]any{
		"objective_id": objective.ID, "objective_version": objective.CurrentVersion, "model_id": model.ID, "model_revision": model.Revision, "title": "Compare launch scaling", "definition_path": ".komodo/capacity/launch.json", "definition_revision": "scenario-v2", "environment_id": "load-lab", "environment_revision": "policy-4", "environment_class": "isolated", "coordinated_load_key": "accepted-launch",
		"limits": map[string]any{"max_duration_seconds": 120, "max_virtual_users": 500, "max_requests_per_second": 2000, "max_cost": 50, "currency": "USD"},
		"candidates": []map[string]any{
			{"id": "vertical", "name": "larger nodes", "approach": "vertical", "release_id": "release-base", "release_revision": "app-base", "infrastructure_plan_id": "infra-vertical", "infrastructure_revision": "infra-v1", "schema_id": "schema", "schema_revision": "schema-v1", "dependency_configuration_id": "database", "dependency_revision": "db-v1"},
			{"id": "horizontal", "name": "cache and horizontal workers", "approach": "horizontal", "release_id": "release-candidate", "release_revision": "app-candidate", "infrastructure_plan_id": "infra-horizontal", "infrastructure_revision": "infra-v2", "schema_id": "schema", "schema_revision": "schema-v1", "dependency_configuration_id": "database", "dependency_revision": "db-v2"}},
		"scenarios": []map[string]any{{"id": "peak", "name": "launch peak and database latency", "kind": "load_and_failure", "workload_source": "synthetic", "demand": 1800, "demand_unit": "requests/second", "duration_seconds": 60, "failure": "database latency 500ms", "correctness_criteria": []string{"no lost writes"}}}, "owner_ids": []string{"operator"},
	}
	var rehearsal capacityrehearsals.Rehearsal
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-rehearsals", tokens["agent"], rehearsalInput, http.StatusCreated, &rehearsal)
	attempt := func(candidate, actorKind string, repetitions int, noise, throughput, cost float64) {
		workflowValue(t, server.URL, http.MethodPost, root+"/capacity-rehearsals/"+rehearsal.ID+"/attempts", tokens[map[string]string{"agent": "agent", "human": "operator"}[actorKind]], map[string]any{
			"expected_revision": rehearsal.Revision, "candidate_id": candidate, "scenario_id": "peak", "actor_kind": actorKind, "started_at": now.Add(-time.Minute), "ended_at": now, "environment_revision": "policy-4", "workload_digest": "sha256:synthetic", "repetitions": repetitions, "noise_percent": noise, "status": "completed",
			"metrics": map[string]any{"throughput": throughput, "throughput_unit": "requests/second", "latency_p95_ms": 150, "error_rate": .001, "saturation": 70, "saturation_unit": "percent", "recovery_seconds": 4, "correctness_passed": true, "resources": map[string]float64{"cpu": 70}, "cost": cost, "currency": "USD"}, "logs": []string{"sanitized:load"}, "artifact_digests": []string{"sha256:result"},
		}, http.StatusCreated, &rehearsal)
	}
	attempt("vertical", "agent", 1, 19, 1400, 18)
	attempt("horizontal", "human", 3, 3, 1900, 12)
	if rehearsal.Attempts[0].Proof || !rehearsal.Attempts[1].Proof {
		t.Fatal("noisy alternative was treated as proof or supported alternative was lost")
	}

	// The unavailable dependency owner remains an explicit gap on the first plan.
	draftInput := capacityWorkflowPlanInput(objective, model, rehearsal, now, "", "")
	var draft capacityplans.Plan
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-plans", tokens["operator"], draftInput, http.StatusCreated, &draft)
	if draft.Status != "draft" || !capacityWorkflowPlanGap(draft, "unresolved_dependency") {
		t.Fatal("unavailable dependency owner was hidden")
	}

	planInput := capacityWorkflowPlanInput(objective, model, rehearsal, now, "quota:approved", "reservation:approved")
	var plan capacityplans.Plan
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-plans", tokens["operator"], planInput, http.StatusCreated, &plan)

	checkout := t.TempDir()
	gitOutput(t, checkout, "init", "-b", "main")
	gitOutput(t, checkout, "config", "user.name", "Capacity Team")
	gitOutput(t, checkout, "config", "user.email", "capacity@example.test")
	writeWorkflowFile(t, checkout, "app.go", "package capacity\n\nconst Workers = 8 // agent-authored optimization\n")
	writeWorkflowFile(t, checkout, "infra.tf", "# human-authored protected scaling plan\nworkers = 8\n")
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Deliver reviewed capacity candidate")
	deliveryRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	opened, _ := repos.Open(repo.ID)
	gitOutput(t, checkout, "remote", "add", "platform", opened.GitDir())
	gitOutput(t, checkout, "push", "platform", "HEAD:refs/heads/main")

	for _, work := range []map[string]any{
		{"phase_id": "prepare", "kind": "pull_request", "resource_id": "pull:application", "revision": deliveryRevision, "owner_kind": "agent", "owner_id": "agent", "status": "completed", "gate_evidence": map[string]string{"review": "review:human", "checks": "check:passed"}},
		{"phase_id": "prepare", "kind": "infrastructure_plan", "resource_id": "infra-horizontal", "revision": "infra-v2", "owner_kind": "human", "owner_id": "operator", "status": "completed", "gate_evidence": map[string]string{"review": "infra-review", "environment": "protected-production"}},
	} {
		work["expected_revision"] = plan.Revision
		workflowValue(t, server.URL, http.MethodPost, root+"/capacity-plans/"+plan.ID+"/work", tokens["operator"], work, http.StatusCreated, &plan)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-plans/"+plan.ID+"/decisions/go", tokens["operator"], map[string]any{"expected_revision": plan.Revision, "outcome": "proceed", "evidence_ids": []string{"review:human", "check:passed", "infra-review"}, "rationale": "ordinary application and infrastructure gates passed"}, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, root+"/capacity-plans/"+plan.ID+"/approvals", tokens["operator"], map[string]any{"expected_revision": plan.Revision, "decision": "approved", "rationale": "approve coordination only; provider and environment authority remain separate"}, http.StatusCreated, &plan)
	if plan.Status != "approved" || len(plan.NonAuthority) == 0 {
		t.Fatalf("plan did not reach bounded approval: %#v", plan.Gaps)
	}

	deliveryBase := root + "/capacity-plans/" + plan.ID + "/deliveries"
	deliveryInput := capacityWorkflowDeliveryInput(plan, objective, model)
	stale := capacityWorkflowDeliveryInput(plan, objective, model)
	stale["plan_revision"] = plan.Revision - 1
	workflowValue(t, server.URL, http.MethodPost, deliveryBase, tokens["operator"], stale, http.StatusConflict, nil)
	var delivery capacitydeliveries.Delivery
	workflowValue(t, server.URL, http.MethodPost, deliveryBase, tokens["operator"], deliveryInput, http.StatusCreated, &delivery)
	deliveryURL := deliveryBase + "/" + delivery.ID

	observe := func(actor, actorKind string, changes map[string]any) {
		body := capacityWorkflowObservation(delivery.Revision, now, deliveryRevision)
		for key, value := range changes {
			body[key] = value
		}
		workflowValueHeaders(t, server.URL, http.MethodPost, deliveryURL+"/observations", tokens[actor], body, map[string]string{"X-Actor-Kind": actorKind}, http.StatusCreated, &delivery)
	}
	// A delegated agent may observe and contain, but its budget breach and a
	// reliability regression can never authorize more cloud capacity.
	observe("agent", "agent", map[string]any{"quota": "denied", "cost": 650.0, "reliability": "regressed"})
	if !capacityWorkflowDeliveryBlocker(delivery, "quota_denial") || !capacityWorkflowDeliveryBlocker(delivery, "budget_breach") || !capacityWorkflowDeliveryBlocker(delivery, "reliability_regression") || delivery.State != "paused" {
		t.Fatalf("unsafe delegated rollout was not contained: %#v", delivery.Blockers)
	}
	workflowValueHeaders(t, server.URL, http.MethodPost, deliveryURL+"/controls", tokens["agent"], map[string]any{"expected_revision": delivery.Revision, "action": "pause", "phase_id": "progressive", "rationale": "contain denied quota and regression", "evidence_ids": []string{"production:failed"}}, map[string]string{"X-Actor-Kind": "agent"}, http.StatusCreated, &delivery)
	workflowValueHeaders(t, server.URL, http.MethodPost, deliveryURL+"/controls", tokens["agent"], map[string]any{"expected_revision": delivery.Revision, "action": "rollback", "phase_id": "progressive", "rationale": "attempt unauthorized rollback", "evidence_ids": []string{"production:failed"}}, map[string]string{"X-Actor-Kind": "agent"}, http.StatusForbidden, nil)

	// Quota recovery exposes waste before a final right-sized observation proves
	// usable headroom, reliability, and cost against real load.
	observe("operator", "human", map[string]any{"reservation_utilization_percent": 20.0})
	if !capacityWorkflowDeliveryBlocker(delivery, "unused_reservation") || !strings.Contains(delivery.PredictedNextAction, "go") {
		t.Fatal("overprovisioned result did not force a decision revisit")
	}
	observe("operator", "human", nil)
	if len(delivery.Blockers) != 0 || !delivery.ObjectiveValidated || !delivery.ForecastValidated {
		t.Fatalf("right-sized production recovery did not verify capacity: %#v", delivery)
	}
	workflowValueHeaders(t, server.URL, http.MethodPost, deliveryURL+"/controls", tokens["operator"], map[string]any{"expected_revision": delivery.Revision, "action": "resume", "phase_id": "progressive", "rationale": "current production proof meets every bound", "evidence_ids": []string{"production:healthy"}}, map[string]string{"X-Actor-Kind": "human"}, http.StatusCreated, &delivery)
	if delivery.State != "active" || len(delivery.NonAuthority) == 0 || delivery.Observations[0].InfrastructureRevision != "infra-v2" {
		t.Fatal("final trail lost protected revisions, authority limits, or active recovery")
	}
}

func capacityWorkflowModelInput(objectiveID string, objectiveVersion int64, release string, now time.Time, authorKind string) map[string]any {
	return map[string]any{"objective_id": objectiveID, "objective_version": objectiveVersion, "title": "Accepted launch forecast", "release_id": release, "release_revision": "app-base", "forecast_window": map[string]any{"starts_at": now.Add(-48 * time.Hour), "ends_at": now.Add(60 * 24 * time.Hour)}, "method": "segmented production trend", "author_kind": authorKind,
		"evidence":    []map[string]any{{"id": "usage", "kind": "usage", "resource_id": "metrics:requests", "revision": "otel-v4", "observation_window": map[string]any{"starts_at": now.Add(-48 * time.Hour), "ends_at": now.Add(-24 * time.Hour)}, "visibility": "repository", "summary": "sanitized request history", "sanitized": true, "instrumentation_version": "otel-v4"}},
		"assumptions": []map[string]any{{"id": "launch", "statement": "accepted launch increases active teams", "evidence_ids": []string{"usage"}, "owner_id": "product", "uncertainty": "plus or minus ten percent"}}, "workload_segments": []map[string]any{{"id": "interactive", "name": "interactive API", "demand": 1800, "unit": "requests/second", "evidence_ids": []string{"usage"}}}, "saturation_points": []map[string]any{{"id": "pool", "resource": "database pool", "metric": "connections", "capacity": 100, "unit": "connections", "saturates_at": now.Add(30 * 24 * time.Hour), "headroom_percent": 8, "evidence_ids": []string{"usage"}, "explanation": "requests per connection exceed measured limit"}}, "scenarios": []map[string]any{{"id": "planned", "name": "accepted launch", "demand_multiplier": 2, "probability": "medium", "saturation_ids": []string{"pool"}, "cost_curve": []map[string]any{{"demand": 1800, "cost": 4000, "currency": "USD", "period": "month"}}}}, "uncertainty": "launch timing may move", "provenance": []string{"analysis:sha256:forecast"}}
}

func capacityWorkflowPlanInput(o capacityobjectives.Objective, m capacitymodels.Model, r capacityrehearsals.Rehearsal, now time.Time, quotaApproval, reservationApproval string) map[string]any {
	return map[string]any{"objective_id": o.ID, "objective_version": o.CurrentVersion, "model_id": m.ID, "model_revision": m.Revision, "rehearsal_id": r.ID, "rehearsal_revision": r.Revision, "candidate_id": "horizontal", "title": "Deliver accepted launch capacity", "rationale": "bounded rehearsal supports horizontal scaling", "owner_ids": []string{"operator"}, "budget": 5000, "currency": "USD",
		"reservations": []map[string]any{{"id": "workers", "kind": "capacity", "resource_id": "worker-pool", "provider_id": "provider", "quantity": 8, "unit": "instances", "needed_by": now.Add(20 * 24 * time.Hour), "owner_id": "operator", "approval_id": reservationApproval}}, "dependencies": []map[string]any{{"id": "quota", "kind": "quota", "resource_id": "provider", "requirement": "eight worker instances", "owner_id": "dependency-owner", "needed_by": now.Add(14 * 24 * time.Hour), "approval_id": quotaApproval}},
		"phases": []map[string]any{{"id": "prepare", "name": "Prepare and review", "order": 1, "scope": []string{"application", "infrastructure", "observability"}, "owner_ids": []string{"operator", "agent"}, "budget": 1000, "currency": "USD", "reservation_ids": []string{"workers"}, "dependency_ids": []string{"quota"}, "gates": []map[string]any{{"kind": "review", "required": true}, {"kind": "checks", "required": true}}, "success_criteria": []string{"ordinary changes approved"}, "exit_criteria": []string{"revert application and infrastructure"}}}, "decision_points": []map[string]any{{"id": "go", "after_phase_id": "prepare", "question": "begin protected rollout?", "owner_id": "operator", "due_at": now.Add(21 * 24 * time.Hour), "options": []string{"proceed", "exit"}, "evidence_required": []string{"review", "checks", "quota"}}}, "exit_strategy": []string{"remove excess workers and retain baseline"}}
}

func capacityWorkflowDeliveryInput(p capacityplans.Plan, o capacityobjectives.Objective, m capacitymodels.Model) map[string]any {
	return map[string]any{"plan_id": p.ID, "plan_revision": p.Revision, "objective_id": o.ID, "objective_version": o.CurrentVersion, "model_id": m.ID, "model_revision": m.Revision, "decision_revisit_id": "go", "phases": []map[string]any{{"id": "progressive", "plan_phase_id": "prepare", "name": "Protected progressive rollout", "environment_id": "production", "environment_revision": "environment-policy-v7", "controller_id": "autoscaler-v3", "operator_ids": []string{"operator"}, "delegated_agent_ids": []string{"agent"}, "target_capacity": 2000, "capacity_unit": "requests/second", "max_load": 1800, "min_headroom_percent": 25, "max_cost": 500, "currency": "USD"}}}
}

func capacityWorkflowObservation(revision int64, now time.Time, releaseRevision string) map[string]any {
	return map[string]any{"expected_revision": revision, "phase_id": "progressive", "release_revision": releaseRevision, "infrastructure_revision": "infra-v2", "schema_revision": "schema-v1", "dependency_configuration_revision": "db-v2", "evidence_window_start": now.Add(-time.Hour), "evidence_window_end": now, "production_evidence_ids": []string{"production:headroom", "production:cost"}, "allocated_capacity": 2100, "usable_capacity": 2000, "load": 1500, "forecast_load": 1500, "headroom_percent": 25, "scaling_lag_seconds": 20, "max_scaling_lag_seconds": 60, "regional_imbalance_percent": 3, "max_regional_imbalance_percent": 10, "service_levels": []map[string]any{{"name": "p95", "target": 200, "actual": 150, "unit": "ms", "met": true}}, "dependencies": []map[string]any{{"dependency_id": "database", "status": "healthy", "evidence_id": "dependency:healthy"}}, "correctness": "passed", "reliability": "healthy", "quota": "granted", "cost": 400, "reservation_utilization_percent": 80, "min_reservation_utilization_percent": 50}
}

func capacityWorkflowPlanGap(p capacityplans.Plan, kind string) bool {
	for _, g := range p.Gaps {
		if g.Kind == kind {
			return true
		}
	}
	return false
}
func capacityWorkflowDeliveryBlocker(d capacitydeliveries.Delivery, kind string) bool {
	for _, b := range d.Blockers {
		if b.Kind == kind {
			return true
		}
	}
	return false
}

func workflowValueHeaders(t *testing.T, server, method, path, token string, body any, headers map[string]string, status int, out any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, server+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		contents, _ := io.ReadAll(response.Body)
		t.Fatalf("%s %s = %d, want %d: %s", method, path, response.StatusCode, status, contents)
	}
	if out != nil && json.NewDecoder(response.Body).Decode(out) != nil {
		t.Fatalf("decode %s %s", method, path)
	}
}
