package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/observabilitygaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalcontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalimplementations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalrollouts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestObservabilityEngineeringWorkflow is the black-box boundary from an
// unanswered production question to bounded instrumentation, reproducible
// diagnosis, an ordinary reviewed repair, verified production outcome, and
// evidence-gated retirement. Stock Git and the public collaboration APIs retain
// every correction and containment decision without granting telemetry authority.
func TestObservabilityEngineeringWorkflow(t *testing.T) {
	requireGit(t)
	objects, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), objects)
	credentials, _ := auth.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	gaps, _ := observabilitygaps.New(t.TempDir())
	contracts, _ := signalcontracts.New(t.TempDir())
	implementations, _ := signalimplementations.New(t.TempDir())
	rollouts, _ := signalrollouts.New(t.TempDir())
	evaluations, _ := signalevaluations.New(t.TempDir())
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pulls, proposalStore, repos, credentials)
	registerObservabilityGapsHTTP(mux, gaps, repos, credentials)
	registerSignalContractsHTTP(mux, contracts, repos, credentials)
	registerSignalImplementationsHTTP(mux, implementations, repos, credentials, signalImplementationSources{contracts: contracts, pulls: pulls})
	registerSignalRolloutsHTTP(mux, rollouts, repos, credentials, signalRolloutSources{contracts: contracts, runs: implementations})
	registerSignalEvaluationsHTTP(mux, evaluations, gaps, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	repo, _ := repos.Create("owner", repositories.Metadata{Name: "observability-loop", Visibility: repositories.Private})
	actors := []string{"investigator", "signal-agent", "privacy-owner", "service-owner", "operator"}
	tokens := map[string]string{"owner": issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)}
	for _, actor := range actors {
		_, _ = repos.AddCollaborator("owner", repo.ID, actor)
		tokens[actor] = issueAccess(t, credentials, actor, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	}
	root := "/repositories/" + string(repo.ID)
	now := time.Now().UTC()

	checkout := t.TempDir()
	gitOutput(t, checkout, "init", "-b", "main")
	gitOutput(t, checkout, "config", "user.name", "Checkout Team")
	gitOutput(t, checkout, "config", "user.email", "checkout@example.test")
	writeWorkflowFile(t, checkout, "app/retry.go", "package app\n\nfunc retryClass(code int) string { if code == 429 { return \"capacity\" }; return \"provider\" }\n")
	writeWorkflowFile(t, checkout, "infra/collector.yaml", "sampling: 0.10\ncorrelation: none\n")
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Release opaque checkout retries")
	baseRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	opened, _ := repos.Open(repo.ID)
	gitOutput(t, checkout, "remote", "add", "platform", opened.GitDir())
	gitOutput(t, checkout, "push", "platform", "HEAD:refs/heads/main")

	var gap observabilitygaps.Gap
	workflowValue(t, server.URL, http.MethodPost, root+"/observability-gaps", tokens["investigator"], map[string]any{
		"origin":   map[string]any{"kind": "debugging_workspace", "resource_id": "production-investigation-42", "revision": "7"},
		"question": "Why do checkout retries strand some users?", "behavior": "retry recovery differs despite the same response code", "audience": []string{"responders", "checkout owners"}, "decision": "repair application classification or provider routing", "affected_services": []string{"checkout-api", "telemetry-pipeline"}, "affected_journeys": []string{"purchase"},
		"required_timeliness": map[string]any{"maximum_delay_seconds": 60, "decision_window": "active production investigation"},
		"current_evidence":    []map[string]any{{"id": "retry-rate", "kind": "metric", "source": "metric:retry", "semantics": "aggregate retry rate", "release_id": "release-7", "release_revision": baseRevision, "environment": "production", "environment_revision": "prod-19", "observed_at": now.Add(-time.Minute), "fresh_until": now.Add(time.Hour), "accessible": true, "owner_id": "operator"}},
		"missing_coverage":    []string{"no provider, release, journey, and trace correlation"}, "owner_ids": []string{"investigator", "service-owner", "privacy-owner"}, "success_criteria": []string{"identify the retry cause reproducibly", "verify repaired user recovery"}, "change_reason": "production evidence cannot answer the rollback decision",
	}, http.StatusCreated, &gap)

	unsafe := observabilityContractInput(baseRevision)
	unsafe["schema"] = []map[string]any{{"name": "retry_class", "type": "string", "description": "retry classification"}, {"name": "user_email", "type": "string", "description": "customer identity", "sensitive": true}}
	unsafe["dimensions"] = []map[string]any{{"name": "user_id", "source": "request.user", "bounded": false, "sensitive": true}}
	unsafe["correlation"] = []map[string]any{}
	unsafe["change_reason"] = "first design exposes identity and has no cardinality or correlation bound"
	var contract signalcontracts.Contract
	workflowValue(t, server.URL, http.MethodPost, root+"/signal-contracts", tokens["service-owner"], unsafe, http.StatusCreated, &contract)
	if !contract.Blocked || !observabilityContractFinding(contract, "sensitive_field") || !observabilityContractFinding(contract, "unbounded_dimension") {
		t.Fatalf("unsafe first contract was not blocked: %#v", contract.Findings)
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/signal-contracts/"+contract.ID+"/challenges", tokens["signal-agent"], map[string]any{"version": 1, "agent": true, "assumption": "aggregate response codes identify the cause", "position": "the existing schema conflicts with retry.kind and lacks trace correlation", "evidence": []map[string]any{{"uri": "source:app/retry.go", "revision": baseRevision, "claim": "the application emits a different classification and no trace key", "accessible": true}}}, http.StatusCreated, &contract)

	accepted := observabilityContractInput(baseRevision)
	accepted["change_reason"] = "owners accepted a redacted bounded schema with trace correlation"
	accepted["expected_version"] = contract.CurrentVersion
	workflowValue(t, server.URL, http.MethodPost, root+"/signal-contracts/"+contract.ID+"/versions", tokens["privacy-owner"], accepted, http.StatusCreated, &contract)
	if !contract.Complete || contract.Blocked || contract.CurrentVersion != 2 || len(contract.Challenges) != 1 {
		t.Fatalf("corrected owner-and-agent contract is not current: %#v", contract)
	}

	gitOutput(t, checkout, "switch", "-c", "instrument/retries")
	writeWorkflowFile(t, checkout, "app/retry.go", "package app\n\n// retryClass emits only a bounded class joined by trace_id.\nfunc retryClass(code int) string { if code == 429 { return \"capacity\" }; return \"provider\" }\n")
	writeWorkflowFile(t, checkout, "infra/collector.yaml", "sampling: 0.10\ncorrelation: trace_id\nredaction: allowlist\n")
	gitOutput(t, checkout, "add", ".")
	gitOutput(t, checkout, "commit", "-m", "Instrument bounded correlated retries")
	instrumentationRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	gitOutput(t, checkout, "push", "platform", "HEAD:refs/heads/instrument/retries")
	var instrumentationPull pullrequests.PullRequest
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests", tokens["service-owner"], map[string]any{"title": "Instrument bounded retry evidence", "body": "Application and infrastructure changes implement contract v2.", "source_branch": "instrument/retries", "target_branch": "main"}, http.StatusCreated, &instrumentationPull)

	var plan signalimplementations.Plan
	workflowValue(t, server.URL, http.MethodPost, root+"/signal-contracts/"+contract.ID+"/implementations", tokens["service-owner"], map[string]any{"contract_version": 2, "base_revision": baseRevision, "work": []map[string]any{{"kind": "pull_request", "owner_kind": "agent", "owner_id": "signal-agent", "repository_id": string(repo.ID), "resource_id": instrumentationPull.ID, "revision": instrumentationRevision, "permitted": true}, {"kind": "task", "owner_kind": "human", "owner_id": "operator", "repository_id": string(repo.ID), "resource_id": "infra:collector-change", "revision": instrumentationRevision, "permitted": true}}}, http.StatusCreated, &plan)

	badRun := observabilityTelemetryRun(plan, contract, instrumentationPull, instrumentationRevision, 75, false)
	var failed signalimplementations.Run
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests/"+instrumentationPull.ID+"/telemetry-checks/runs", tokens["signal-agent"], badRun, http.StatusCreated, &failed)
	if failed.Passed || failed.Cost != 75 || len(failed.Differences) == 0 {
		t.Fatalf("schema, correlation, redaction, or agent-cost failure escaped preview: %#v", failed)
	}
	var passing signalimplementations.Run
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests/"+instrumentationPull.ID+"/telemetry-checks/runs", tokens["service-owner"], observabilityTelemetryRun(plan, contract, instrumentationPull, instrumentationRevision, 1.25, true), http.StatusCreated, &passing)
	if !passing.Passed || passing.CandidateRevision != instrumentationRevision || passing.ConfigRevision != instrumentationRevision {
		t.Fatalf("revision-exact corrected preview did not pass: %#v", passing)
	}
	workflowValue(t, server.URL, http.MethodPut, root+"/pull-requests/"+instrumentationPull.ID+"/reviews/me", tokens["owner"], map[string]any{"decision": "approve"}, http.StatusOK, nil)
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests/"+instrumentationPull.ID+"/merge", tokens["owner"], map[string]any{}, http.StatusOK, &instrumentationPull)

	var rollout signalrollouts.Rollout
	rolloutBase := root + "/signal-contracts/" + contract.ID + "/rollouts"
	workflowValue(t, server.URL, http.MethodPost, rolloutBase, tokens["operator"], map[string]any{"contract_id": contract.ID, "contract_version": 2, "pull_request_id": instrumentationPull.ID, "implementation_run_id": passing.ID, "deployed_revision": instrumentationRevision, "collector_revision": "collector-v8", "controller_id": "progressive-controller", "operator_ids": []string{"operator"}, "privacy_controls": []string{"field allowlist", "regional retention policy"}, "stages": []map[string]any{{"id": "canary", "name": "checkout canary", "environment_id": "production", "environment_revision": "prod-policy-19", "service_ids": []string{"checkout-api"}, "audiences": []string{"responders"}, "regions": []string{"eu-west"}, "traffic_percent": 10}}, "max_cardinality": 100, "max_storage_bytes": 1000000, "max_query_cost": 10, "currency": "USD"}, http.StatusCreated, &rollout)
	observe := func(changes map[string]any) {
		body := observabilityRolloutObservation(rollout.Revision, now)
		for key, value := range changes {
			body[key] = value
		}
		workflowValue(t, server.URL, http.MethodPost, rolloutBase+"/"+rollout.ID+"/observations", tokens["operator"], body, http.StatusCreated, &rollout)
	}
	observe(map[string]any{"cardinality": 900, "sampling_bias_percent": 22})
	if rollout.State != "narrowed" || !observabilityRolloutFinding(rollout, "cardinality_breach") || !observabilityRolloutFinding(rollout, "sampling_skew") {
		t.Fatalf("cardinality and biased sample were not narrowed: %#v", rollout)
	}
	observe(map[string]any{"collector_status": "outage", "pipeline_loss_percent": 40})
	if rollout.State != "paused" || !observabilityRolloutFinding(rollout, "collector_outage") || !observabilityRolloutFinding(rollout, "pipeline_loss") {
		t.Fatalf("pipeline outage was not paused: %#v", rollout)
	}
	observe(nil)
	workflowValue(t, server.URL, http.MethodPost, rolloutBase+"/"+rollout.ID+"/controls", tokens["operator"], map[string]any{"expected_revision": rollout.Revision, "action": "resume", "stage_id": "canary", "rationale": "current evidence is bounded, unbiased, and healthy", "evidence_ids": []string{"window:healthy"}}, http.StatusCreated, &rollout)
	if rollout.State != "active" || len(rollout.Observations) != 3 {
		t.Fatalf("progressive signal did not recover with retained evidence: %#v", rollout)
	}

	evaluationBase := root + "/observability-gaps/" + gap.ID + "/signal-evaluations"
	var evaluation signalevaluations.Evaluation
	workflowValue(t, server.URL, http.MethodPost, evaluationBase, tokens["investigator"], observabilityEvaluationInput(gap, contract, rollout, instrumentationRevision, now), http.StatusCreated, &evaluation)
	addFinding := func(actor, kind, statement, criterion string) {
		headers := map[string]string{}
		if actor == "signal-agent" {
			headers["X-Actor-Kind"] = "read_only_agent"
		}
		workflowValueHeaders(t, server.URL, http.MethodPost, evaluationBase+"/"+evaluation.ID+"/findings", tokens[actor], map[string]any{"expected_revision": evaluation.Revision, "kind": kind, "statement": statement, "citation_ids": []string{"query-proof"}, "uncertainty": "bounded ten-percent canary", "reproduction": "rerun retry-class query at its pinned release and window", "criteria": map[string]string{"identify the retry cause reproducibly": criterion}}, headers, http.StatusCreated, &evaluation)
	}
	addFinding("investigator", "misleading", "provider latency appears to cause every retry", "failed")
	misleading := evaluation.Findings[0]
	workflowValue(t, server.URL, http.MethodPost, evaluationBase+"/"+evaluation.ID+"/resolutions", tokens["service-owner"], map[string]any{"expected_revision": evaluation.Revision, "finding_id": misleading.ID, "disposition": "repair_required", "repair_kind": "pull_request", "repair_id": "repair:retry-classification", "rationale": "correlated traces contradict the first aggregate conclusion"}, http.StatusCreated, &evaluation)
	addFinding("signal-agent", "supported", "application classification strands capacity retries before provider routing", "passed")
	supported := evaluation.Findings[len(evaluation.Findings)-1]
	for _, target := range []map[string]string{{"kind": "response_alert", "id": "alert:retry-capacity", "revision": "1"}, {"kind": "runbook", "id": "runbook:checkout-retry", "revision": "rehearsal-passed-v4"}} {
		workflowValue(t, server.URL, http.MethodPost, evaluationBase+"/"+evaluation.ID+"/resolutions", tokens["service-owner"], map[string]any{"expected_revision": evaluation.Revision, "finding_id": supported.ID, "disposition": "accepted", "target_kind": target["kind"], "target_id": target["id"], "target_revision": target["revision"], "rationale": "revision-exact evidence supports an actionable alert and rehearsed procedure"}, http.StatusCreated, &evaluation)
	}

	gitOutput(t, checkout, "fetch", "platform", "main")
	gitOutput(t, checkout, "switch", "-c", "repair/retry-classification", "FETCH_HEAD")
	writeWorkflowFile(t, checkout, "app/retry.go", "package app\n\n// routeRetry preserves capacity retries for governed recovery.\nfunc routeRetry(code int) string { if code == 429 { return \"recover-capacity\" }; return \"provider\" }\n")
	gitOutput(t, checkout, "add", "app/retry.go")
	gitOutput(t, checkout, "commit", "-m", "Repair capacity retry routing")
	repairRevision := gitOutput(t, checkout, "rev-parse", "HEAD")
	gitOutput(t, checkout, "push", "platform", "HEAD:refs/heads/repair/retry-classification")
	var repairPull pullrequests.PullRequest
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests", tokens["service-owner"], map[string]any{"title": "Repair capacity retry routing", "body": "Reviewed repair linked to reproducible signal evidence and runbook rehearsal.", "source_branch": "repair/retry-classification", "target_branch": "main"}, http.StatusCreated, &repairPull)
	workflowValue(t, server.URL, http.MethodPut, root+"/pull-requests/"+repairPull.ID+"/reviews/me", tokens["owner"], map[string]any{"decision": "approve"}, http.StatusOK, nil)
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests/"+repairPull.ID+"/merge", tokens["owner"], map[string]any{}, http.StatusOK, &repairPull)
	if repairPull.MergeCommitID == "" || repairRevision == instrumentationRevision {
		t.Fatal("ordinary reviewed repair did not retain exact Git provenance")
	}

	addFinding("investigator", "supported", "post-repair production window shows capacity retries recover", "passed")
	staleConsumers := []map[string]any{{"kind": "response_alert", "id": "alert:retry-capacity", "revision": "1", "owner_id": "operator", "impact": "migrate to the permanent service-level signal", "acknowledged": false}}
	workflowValue(t, server.URL, http.MethodPost, evaluationBase+"/"+evaluation.ID+"/lifecycle-decisions", tokens["service-owner"], map[string]any{"expected_revision": evaluation.Revision, "action": "remove", "signal_ids": []string{"retry-class"}, "rationale": "diagnostic question is answered", "policy_id": "telemetry-retention", "policy_revision": "9", "approved_by_id": "privacy-owner", "consumers": staleConsumers, "historical_meaning": "retry class under contract v2 and collector v8", "provenance_refs": []string{contract.ID + "@2", repairPull.MergeCommitID}}, http.StatusCreated, &evaluation)
	if evaluation.Lifecycles[len(evaluation.Lifecycles)-1].Applied || !strings.Contains(strings.Join(evaluation.Blockers, " "), "consumer") {
		t.Fatal("stale consumer and missing stop proof did not block retirement")
	}
	stopped := now.Add(time.Hour)
	workflowValue(t, server.URL, http.MethodPost, evaluationBase+"/"+evaluation.ID+"/lifecycle-decisions", tokens["service-owner"], map[string]any{"expected_revision": evaluation.Revision, "action": "remove", "signal_ids": []string{"retry-class"}, "rationale": "consumer migrated and diagnostic collection is verified stopped", "policy_id": "telemetry-retention", "policy_revision": "9", "approved_by_id": "privacy-owner", "consumers": []map[string]any{{"kind": "response_alert", "id": "alert:retry-capacity", "revision": "2", "owner_id": "operator", "impact": "now consumes permanent recovery SLI", "acknowledged": true}}, "historical_meaning": "retry class under contract v2 and collector v8", "provenance_refs": []string{contract.ID + "@2", repairPull.MergeCommitID}, "stop_evidence_ids": []string{"collector:v8:collection-stopped"}, "collection_stopped_at": stopped}, http.StatusCreated, &evaluation)
	if !evaluation.Lifecycles[len(evaluation.Lifecycles)-1].Applied || evaluation.CurrentSignalState["retry-class"] != "remove" || len(evaluation.NonAuthority) == 0 {
		t.Fatalf("verified diagnostic retirement lost its trail or authority boundary: %#v", evaluation)
	}
}

func observabilityContractInput(revision string) map[string]any {
	return map[string]any{
		"name": "checkout.retry.class", "signal_type": "event", "purpose": "distinguish application classification from provider retry failure",
		"schema": []map[string]any{{"name": "retry_class", "type": "string", "description": "bounded retry category", "classification": "operational"}, {"name": "trace_id", "type": "string", "description": "ephemeral trace correlation", "classification": "pseudonymous"}},
		"unit":   "retries", "dimensions": []map[string]any{{"name": "retry_class", "source": "app.retryClass", "bounded": true, "maximum_values": 4}},
		"sampling": map[string]any{"strategy": "trace_consistent", "rate": .10, "rationale": "bound hot-path overhead and preserve joins"}, "aggregation": map[string]any{"method": "count", "window_seconds": 60, "temporality": "delta"}, "correlation": []map[string]any{{"field": "trace_id", "target": "checkout trace", "propagation": "w3c"}},
		"retention_days": 7, "expected_volume": map[string]any{"events_per_second": 20, "bytes_per_event": 96, "peak_multiplier": 3}, "quality_thresholds": map[string]any{"maximum_delay_seconds": 60, "minimum_completeness": .98, "maximum_error_rate": .01},
		"owner_ids": []string{"service-owner", "privacy-owner"}, "consumers": []string{"response-alert", "checkout-runbook"}, "source_symbols": []map[string]any{{"path": "app/retry.go", "symbol": "retryClass", "revision": revision, "accessible": true}}, "service_boundaries": []map[string]any{{"service": "checkout-api", "boundary": "retry handler", "revision": revision, "owner_id": "service-owner"}},
		"collector": map[string]any{"kind": "otel_collector", "id": "checkout-collector", "revision": "v8", "supported": true}, "dependencies": []map[string]any{{"kind": "schema", "id": "retry-event", "revision": "v2", "supported": true}},
		"impact":       map[string]any{"privacy": "no direct user fields; trace IDs expire with the seven-day window", "security": "repository responders only", "residency": "regional routing", "performance": "under one millisecond p99", "cardinality": "four retry classes", "storage": "bounded to 1GB", "cost": "at most 20 USD monthly", "retention_days": 7, "residency_regions": []string{"eu-west"}, "monthly_storage_bytes": 1000000000, "monthly_cost": 20, "currency": "USD"},
		"alternatives": []map[string]any{{"id": "aggregate", "name": "aggregate retry count", "description": "cheaper but cannot answer causality", "sampling": map[string]any{"strategy": "all", "rate": 1, "rationale": "small aggregate"}, "aggregation": map[string]any{"method": "count", "window_seconds": 300, "temporality": "delta"}, "retention_days": 3, "expected_volume": map[string]any{"events_per_second": 1, "bytes_per_event": 32, "peak_multiplier": 2}, "impact": map[string]any{"monthly_storage_bytes": 10000000, "monthly_cost": 1, "currency": "USD"}, "evidence": []map[string]any{{"uri": "investigation:42", "revision": "7", "claim": "aggregate evidence produced the misleading first conclusion", "accessible": true}}}},
		"assumptions":  []string{"aggregate response codes identify the cause"},
	}
}

func observabilityTelemetryRun(plan signalimplementations.Plan, contract signalcontracts.Contract, pull pullrequests.PullRequest, revision string, cost float64, pass bool) map[string]any {
	checks := []string{"emission", "schema", "units", "correlation", "sampling", "redaction", "access_boundaries", "performance_overhead", "failure_behavior"}
	results := make([]map[string]any, 0, len(checks))
	for _, check := range checks {
		status := "passed"
		if !pass && (check == "schema" || check == "correlation" || check == "redaction") {
			status = "failed"
		}
		results = append(results, map[string]any{"check": check, "status": status, "summary": "bounded synthetic " + check + " evidence", "coverage": []string{"purchase retry", "collector failure"}, "evidence": []map[string]any{{"kind": "signal", "summary": "sanitized digest only", "digest": "sha256:0123456789abcdef" + check, "sanitized": true, "accessible": true}}})
	}
	differences := []map[string]any{}
	if !pass {
		differences = append(differences, map[string]any{"kind": "schema_conflict", "summary": "candidate used legacy retry.kind and omitted trace_id", "expected": "retry_class plus trace_id", "actual": "retry.kind without correlation"})
	}
	return map[string]any{"candidate_revision": revision, "plan_id": plan.ID, "contract_id": contract.ID, "contract_version": contract.CurrentVersion, "config_path": "infra/collector.yaml", "config_revision": revision, "synthetic_journey": "purchase retry after capacity response", "synthetic_failure": "collector unavailable and application retry classification", "results": results, "contract_differences": differences, "duration_ms": 400, "cost": cost, "currency": "USD", "authorship": []string{"signal-agent:application", "operator:infrastructure"}, "ordinary_policy_checks": []string{"human review", "privacy owner", "provenance"}}
}

func observabilityRolloutObservation(revision int64, now time.Time) map[string]any {
	return map[string]any{"expected_revision": revision, "stage_id": "canary", "window_start": now.Add(-time.Hour), "window_end": now, "evidence_ids": []string{"window:healthy"}, "signal_health": "healthy", "coverage_percent": 99, "latency_ms": 30, "missing_percent": 1, "sampling_bias_percent": 2, "cardinality": 4, "storage_bytes": 100000, "query_cost": 2, "pipeline_loss_percent": 1, "malformed_payloads": 0, "unexpected_sensitive_data": false, "collector_status": "healthy", "service_status": "healthy"}
}

func observabilityEvaluationInput(gap observabilitygaps.Gap, contract signalcontracts.Contract, rollout signalrollouts.Rollout, revision string, now time.Time) map[string]any {
	return map[string]any{"gap_version": gap.CurrentVersion, "title": "Correlate retry class with repaired user recovery", "signal_ids": []string{"retry-class"}, "signals": []map[string]any{{"id": "retry-class", "contract_id": contract.ID, "contract_version": contract.CurrentVersion, "rollout_id": rollout.ID, "revision": rollout.CollectorRevision, "kind": "event"}}, "queries": []map[string]any{{"id": "retry-cause", "expression": "count retries by retry_class joined on trace_id", "window_start": now.Add(-time.Hour), "window_end": now, "signal_ids": []string{"retry-class"}, "release_ids": []string{"release:instrumented"}, "deployment_ids": []string{"production:canary"}, "code_revisions": []string{revision}, "dependency_revisions": []string{"collector-v8"}, "journey_ids": []string{"purchase"}, "parameters": map[string]string{"region": "eu-west"}, "result_digest": "sha256:retry-cause"}}, "citations": []map[string]any{{"id": "query-proof", "query_id": "retry-cause", "source": "telemetry://checkout/retry-class", "revision": rollout.CollectorRevision, "digest": "sha256:query-proof", "accessible": true}}}
}

func observabilityContractFinding(c signalcontracts.Contract, kind string) bool {
	for _, f := range c.Findings {
		if f.Kind == kind {
			return true
		}
	}
	return false
}
func observabilityRolloutFinding(r signalrollouts.Rollout, kind string) bool {
	for _, f := range r.Findings {
		if f.Kind == kind {
			return true
		}
	}
	return false
}
