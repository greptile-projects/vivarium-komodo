package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/exploratorysessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualitygates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualityplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/testscenarios"
)

// TestCollaborativeTestEngineeringWorkflow is the black-box boundary for the
// expectation-to-sustained-quality loop. It crosses the public APIs used by the
// quality workspace and uses stock Git objects for exact candidate authorship.
func TestCollaborativeTestEngineeringWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "cross-platform-checkout", Visibility: repositories.Public})
	for _, collaborator := range []string{"product", "designer", "tester", "agent"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, collaborator)
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	tester := issueAccess(t, credentials, "tester", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	plans, _ := qualityplans.New(t.TempDir())
	scenarios, _ := testscenarios.New(t.TempDir())
	sessions, _ := exploratorysessions.New(t.TempDir())
	gates, _ := qualitygates.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	work, _ := proposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerQualityPlansHTTP(mux, plans, repos, credentials)
	registerTestScenariosHTTP(mux, scenarios, repos, credentials)
	registerExploratorySessionsHTTP(mux, sessions, repos, credentials, issueStore, work)
	registerQualityGatesHTTP(mux, gates, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID)

	opened, _ := repos.Open(repo.ID)
	baseBlob, _ := opened.WriteObject(storage.BlobObject, []byte("checkout retries create one order\n"))
	baseTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "checkout.txt", baseBlob))
	baseCommit, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(baseTree)+"\nauthor Product <product@example.test> 1 +0000\ncommitter Product <product@example.test> 1 +0000\n\nproduct and design intent\n"))
	repairBlob, _ := opened.WriteObject(storage.BlobObject, []byte("checkout retries reuse the pending idempotency key\n"))
	repairTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "checkout.txt", repairBlob))
	repairCommit, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(repairTree)+"\nparent "+string(baseCommit)+"\nauthor Agent <agent@example.test> 2 +0000\ncommitter Agent <agent@example.test> 2 +0000\n\nrepair retry edge case and add regression\n"))

	planInput := qualityplans.Input{
		Name: "Cross-platform checkout", Description: "One completed order across the supported purchase journey",
		Scopes:             []qualityplans.Scope{{Kind: "journey", Reference: "checkout", Revision: string(baseCommit)}, {Kind: "interface", Reference: "design:checkout-v3", Revision: "3"}, {Kind: "environment", Reference: "chromium-and-webkit"}},
		Risks:              []qualityplans.Risk{{ID: "duplicate", Description: "retry creates two orders", Severity: "critical"}},
		Requirements:       []qualityplans.Requirement{{ID: "product", Kind: "decision", Reference: "decision:one-order", Revision: string(baseCommit), Rationale: "a purchase has one durable outcome"}, {ID: "design", Kind: "design", Reference: "design:checkout-v3", Revision: "3", Rationale: "retry remains recoverable"}},
		Behaviors:          []qualityplans.Behavior{{ID: "one-order", Subject: "checkout retry", Description: "retry a delayed confirmation", Expected: "exactly one order exists", RequirementIDs: []string{"product", "design"}, RiskIDs: []string{"duplicate"}, TestLevels: []string{"journey", "exploratory", "production"}, EnvironmentIDs: []string{"chromium", "webkit"}, OwnerIDs: []string{"owner"}, JudgeIDs: []string{"tester"}, Testable: true}},
		Environments:       []qualityplans.Environment{{ID: "chromium", Name: "Chromium", Description: "desktop preview", Supported: true}, {ID: "webkit", Name: "WebKit", Description: "mobile device lab", Supported: true}},
		RepresentativeData: []qualityplans.RepresentativeData{{ID: "shopper", Description: "generated shopper and cart", Source: "generator:checkout", PrivacyClassification: "internal", Synthetic: true}},
		CoverageGoals:      []qualityplans.CoverageGoal{{Subject: "checkout", Metric: "supported platforms", Target: 2, TestLevel: "journey"}}, OwnerIDs: []string{"owner"},
		ReleaseThresholds: []qualityplans.Threshold{{ID: "matrix", Subject: "checkout", Metric: "required environments passing", Operator: "eq", Value: 2, Required: true}},
		Evidence:          []qualityplans.Evidence{{ID: "intent-review", Kind: "review", Reference: "review:product-design", Revision: string(baseCommit), BehaviorIDs: []string{"one-order"}, Status: "passing", Manual: true, ObservedAt: time.Now(), AuthorID: "tester"}}, ChangeReason: "product, design, and test review agree on release behavior",
	}
	var plan qualityplans.Plan
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-plans", owner, planInput, http.StatusCreated, &plan)
	if plan.CurrentVersion != 1 || len(plan.Gaps) != 0 || plan.Versions[0].AuthorID != "owner" {
		t.Fatalf("reviewed intent was not retained: %#v", plan)
	}

	unsafe := scenarioInput(plan.ID, string(baseCommit), "pull:intent-review", "human")
	unsafe.Fixtures[0].ContainsProductionData = true
	workflowValue(t, server.URL, http.MethodPost, base+"/test-scenarios", tester, unsafe, http.StatusUnprocessableEntity, nil)
	scenarioDefinition := scenarioInput(plan.ID, string(baseCommit), "pull:intent-review", "human")
	var scenario testscenarios.Scenario
	workflowValue(t, server.URL, http.MethodPost, base+"/test-scenarios", tester, scenarioDefinition, http.StatusCreated, &scenario)
	if scenario.Versions[0].Sources[0].Kind != "design" || scenario.Versions[0].Sources[1].Kind != "journey" || scenario.Versions[0].Contribution.Reference != "pull:intent-review" {
		t.Fatalf("scenario lost reviewed product/design provenance: %#v", scenario)
	}

	expires := time.Now().Add(30 * time.Minute)
	sessionInput := exploratorysessions.Input{Title: "Explore checkout retry boundaries", OriginKind: "pull_request_preview", OriginReference: "preview:checkout", Candidate: exploratorysessions.Candidate{Kind: "pull_request", Reference: "pull:checkout", Revision: string(baseCommit)}, QualityPlanID: plan.ID, Access: exploratorysessions.Access{ExpiresAt: expires, Environment: "isolated-preview", Network: "preview", AllowedRoutes: []string{"/checkout", "/catalog"}, AllowedCommands: []string{"bun test"}}, TestData: exploratorysessions.TestData{Description: "generated shopper", PrivacyClassification: "internal", Synthetic: true}, Budget: exploratorysessions.Budget{MaxMinutes: 45, MaxCost: 5, MaxAgentActions: 5}, Participants: []exploratorysessions.Participant{{ID: "tester", Kind: "human", Approved: true, Role: "lead"}, {ID: "agent", Kind: "agent", Approved: true, Role: "tester"}}, Charters: []exploratorysessions.Charter{{ID: "catalog", Title: "Unaffected navigation", Risk: "stale observations mislead", RiskLevel: "medium", Mission: "inspect catalog transition", OwnerID: "tester", Routes: []string{"/catalog"}, BehaviorIDs: []string{"navigation"}}, {ID: "retry", Title: "Retry timing", Risk: "duplicate order", RiskLevel: "critical", Mission: "interrupt delayed confirmation", OwnerID: "agent", Routes: []string{"/checkout"}, BehaviorIDs: []string{"one-order"}}}, Uncertainty: "device scheduling differs"}
	var session exploratorysessions.Session
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions", tester, sessionInput, http.StatusCreated, &session)
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/timeline", tester, map[string]any{"expected_revision": session.Revision, "kind": "observation", "charter_id": "catalog", "route": "/catalog", "behavior_ids": []string{"navigation"}, "observation": "catalog remains reachable"}, http.StatusCreated, &session)
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/candidate-revisions", tester, map[string]any{"expected_revision": session.Revision, "revision": "preview-corrected", "affected_routes": []string{"/catalog"}}, http.StatusCreated, &session)
	if !session.Events[0].Stale {
		t.Fatal("changed preview did not mark the affected exploratory result stale")
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/timeline", agent, map[string]any{"expected_revision": session.Revision, "kind": "observation", "charter_id": "retry", "route": "/checkout/confirm", "behavior_ids": []string{"one-order"}, "inputs": []string{"delay confirmation", "retry once"}, "observation": "two durable orders", "agent_action": true, "cost": 0.2}, http.StatusCreated, &session)
	edgeEvent := session.Events[len(session.Events)-1].ID
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/findings", agent, map[string]any{"expected_revision": session.Revision, "charter_id": "retry", "title": "Delayed confirmation duplicates order", "description": "retry creates two durable orders", "event_ids": []string{edgeEvent}, "reproduction_steps": []string{"open generated cart", "delay confirmation", "retry once"}, "uncertainty": "20ms boundary"}, http.StatusCreated, &session)
	finding := session.Findings[0].ID
	workflowValue(t, server.URL, http.MethodPatch, base+"/exploratory-sessions/"+session.ID+"/findings/"+finding, agent, map[string]any{"expected_revision": session.Revision, "classification": "defect", "reproduction": "reproduced", "rationale": "three isolated attempts"}, http.StatusOK, &session)

	var delivery struct {
		Session   exploratorysessions.Session `json:"session"`
		Task      proposals.Task              `json:"task"`
		Authority map[string]bool             `json:"authority"`
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/findings/"+finding+"/delivery", tester, map[string]any{"expected_revision": session.Revision, "expected_behavior": "exactly one order exists", "severity": "critical", "owner_kind": "agent", "owner_id": "agent", "acceptance_criteria": []string{"base fails", "repair passes on both platforms"}, "permitted_event_ids": []string{edgeEvent}, "minimized_reproduction": []string{"delay confirmation", "retry once"}}, http.StatusCreated, &delivery)
	session = delivery.Session
	if delivery.Task.BaseRevision != "preview-corrected" || delivery.Authority["git"] || len(session.Findings[0].Delivery.MinimizedReproduction) != 2 {
		t.Fatalf("governed minimized repair context was lost: %#v", delivery)
	}

	regressionInput := scenarioInput(plan.ID, string(repairCommit), "pull:repair", "agent")
	regressionInput.Name = "Delayed confirmation regression"
	regressionInput.Sources = append(regressionInput.Sources, testscenarios.Source{ID: "reproduction", Kind: "reproduction", Reference: finding, Revision: "preview-corrected", Rationale: "minimized exploratory finding", Accessible: true})
	regressionInput.Contribution.Scope = []string{"tests/checkout/**"}
	regressionInput.ChangeReason = "agent-authored repair adds the minimized regression"
	var regression testscenarios.Scenario
	workflowValue(t, server.URL, http.MethodPost, base+"/test-scenarios", agent, regressionInput, http.StatusCreated, &regression)
	// The first repair cannot claim success without distinct passing evidence.
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/findings/"+finding+"/verification", tester, map[string]any{"expected_revision": session.Revision, "pull_request_id": "pull:repair", "base_revision": "preview-corrected", "repair_revision": string(repairCommit), "failing_evidence_id": "run:first-failed", "passing_evidence_id": "run:first-failed", "review_id": "review:owner", "quality_plan_id": plan.ID, "quality_plan_version": 1, "regression_scenario_id": regression.ID, "regression_scenario_version": 1}, http.StatusUnprocessableEntity, nil)
	workflowValue(t, server.URL, http.MethodPost, base+"/exploratory-sessions/"+session.ID+"/findings/"+finding+"/verification", tester, map[string]any{"expected_revision": session.Revision, "pull_request_id": "pull:repair", "base_revision": "preview-corrected", "repair_revision": string(repairCommit), "failing_evidence_id": "run:base-failed", "passing_evidence_id": "run:repair-passed", "review_id": "review:owner-approved", "quality_plan_id": plan.ID, "quality_plan_version": 1, "regression_scenario_id": regression.ID, "regression_scenario_version": 1}, http.StatusCreated, &session)

	requirements := []qualitygates.Requirement{{ID: "chromium", BehaviorID: "one-order", ScenarioID: regression.ID, Kind: "scenario", Environment: "preview", Journey: "checkout", RiskClass: "critical", Locale: "en-US", Platform: "chromium", OwnerID: "owner", Required: true}, {ID: "webkit", BehaviorID: "one-order", ScenarioID: regression.ID, Kind: "scenario", Environment: "device-lab", Journey: "checkout", RiskClass: "critical", Locale: "en-US", Platform: "webkit", OwnerID: "owner", Required: true}}
	var policy qualitygates.Policy
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/policies", owner, qualitygates.PolicyInput{Name: "Checkout delivery", PlanID: plan.ID, PlanVersion: 1, Selector: qualitygates.Selector{Branches: []string{"main"}, Journeys: []string{"checkout"}, Platforms: []string{"chromium", "webkit"}, Releases: []string{"v1.0.0"}}, Requirements: requirements, ChangeReason: "require every promised platform through review, merge, and release"}, http.StatusCreated, &policy)
	gate := openQualityGate(t, server.URL, base, owner, policy.ID, "pull_request", "pull:repair", string(repairCommit), "")
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/attempts", tester, attempt("chromium", "failed", "preview", "chromium", regression.CurrentVersion, "run:first-repair-failed"), http.StatusCreated, &gate)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/attempts", tester, attempt("chromium", "passed", "preview", "chromium", regression.CurrentVersion, "run:repair-chromium"), http.StatusCreated, &gate)
	if gate.Ready || gate.Matrix[1].Status != "missing" {
		t.Fatal("missing WebKit platform did not contain the candidate")
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/attempts", tester, map[string]any{"requirement_id": "webkit", "kind": "scenario", "status": "flaky", "scenario_version": 1, "environment": "device-lab", "locale": "en-US", "platform": "webkit", "input_paths": []string{"checkout.txt"}, "evidence": []string{"run:webkit-flake"}, "flake_reason": "device startup timing"}, http.StatusCreated, &gate)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/acknowledgements", owner, map[string]any{"requirement_id": "chromium", "decision": "accepted", "rationale": "reviewed repair"}, http.StatusCreated, &gate)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/acknowledgements", owner, map[string]any{"requirement_id": "webkit", "decision": "rejected", "rationale": "a flake is not release evidence"}, http.StatusCreated, &gate)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/overrides", owner, map[string]any{"requirement_ids": []string{"webkit"}, "rationale": "ship despite flake", "expires_at": time.Now().Add(time.Hour)}, http.StatusUnprocessableEntity, nil)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/attempts", tester, attempt("webkit", "passed", "device-lab", "webkit", regression.CurrentVersion, "run:webkit-repeatable"), http.StatusCreated, &gate)
	workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+gate.ID+"/acknowledgements", owner, map[string]any{"requirement_id": "webkit", "decision": "accepted", "rationale": "repeatable device-lab pass reviewed"}, http.StatusCreated, &gate)
	if !gate.Ready || gate.Matrix[1].Attempts[0].Status != "flaky" {
		t.Fatalf("corrected pull evidence did not retain the contained flake: %#v", gate)
	}

	for _, target := range []struct{ kind, ref, release string }{{"merge_queue", "queue:main:17", ""}, {"release", "v1.0.0", "v1.0.0"}} {
		candidate := openQualityGate(t, server.URL, base, owner, policy.ID, target.kind, target.ref, string(repairCommit), target.release)
		for _, proof := range []struct{ id, environment, platform string }{{"chromium", "preview", "chromium"}, {"webkit", "device-lab", "webkit"}} {
			workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+candidate.ID+"/attempts", tester, attempt(proof.id, "passed", proof.environment, proof.platform, 1, "run:"+target.kind+":"+proof.id), http.StatusCreated, &candidate)
			workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+candidate.ID+"/acknowledgements", owner, map[string]any{"requirement_id": proof.id, "decision": "accepted", "rationale": "exact revision, review, and platform evidence current"}, http.StatusCreated, &candidate)
		}
		if !candidate.Ready {
			t.Fatalf("%s gate did not converge: %#v", target.kind, candidate.Blockers)
		}
		if target.kind == "release" {
			for _, id := range []string{"chromium", "webkit"} {
				workflowValue(t, server.URL, http.MethodPost, base+"/quality-gates/candidates/"+candidate.ID+"/post-release-signals", tester, map[string]any{"requirement_id": id, "release_id": "v1.0.0", "status": "verified", "evidence": "production-sample:" + id}, http.StatusCreated, &candidate)
			}
			if len(candidate.Signals) != 2 || candidate.Matrix[0].Signals[0].ActorID != "tester" {
				t.Fatalf("post-release behavior evidence lost attribution: %#v", candidate.Signals)
			}
		}
	}
}

func workflowValue(t *testing.T, server, method, path, token string, body any, status int, out any) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, server, method, path, token, string(b), status, out)
}

func scenarioInput(planID, revision, contribution, actorKind string) testscenarios.Input {
	return testscenarios.Input{Name: "Checkout retry journey", Purpose: "prove delayed confirmation produces one order", SourceRevision: revision, DefinitionPath: "tests/checkout/retry.json", QualityPlanID: planID, QualityPlanVersion: 1, Sources: []testscenarios.Source{{ID: "design", Kind: "design", Reference: "design:checkout-v3", Revision: "3", Rationale: "retry stays recoverable", Accessible: true}, {ID: "journey", Kind: "journey", Reference: "decision:one-order", Revision: revision, Rationale: "one durable purchase", Accessible: true}}, Parameters: []testscenarios.Parameter{{Name: "platform", Type: "enum", Description: "supported browser engine", Values: []string{"chromium", "webkit"}, Required: true}}, Preconditions: []testscenarios.Step{{ID: "cart", Kind: "state", Description: "generated cart is open"}}, Actions: []testscenarios.Step{{ID: "submit", Kind: "interaction", Description: "submit then retry a delayed confirmation"}}, Assertions: []testscenarios.Assertion{{ID: "one", Description: "one order exists", Matcher: "count", Expected: "1"}}, Fixtures: []testscenarios.Fixture{{ID: "shopper", Kind: "generator", Description: "generated shopper and cart", Source: "generator:checkout", SourceRevision: "1", PrivacyClassification: "internal", Synthetic: true, Accessible: true, Generator: "checkout-fixture-v1"}}, Environments: []testscenarios.Environment{{ID: "preview", Description: "isolated browser preview", Requirements: []string{"chromium", "webkit"}, Network: "loopback"}}, Contribution: testscenarios.Contribution{Kind: "pull_request", Reference: contribution, Revision: revision, ChangedPaths: []string{"tests/checkout/retry.json"}, Contributor: map[bool]string{true: "agent", false: "tester"}[actorKind == "agent"], ActorKind: actorKind}, Generation: testscenarios.Generation{Generated: false, Provenance: []string{"product", "design"}}, Tags: []string{"checkout", "cross-platform"}, ChangeReason: "reviewed scenario derived from product and design intent"}
}

func attempt(id, status, environment, platform string, version int64, evidence string) map[string]any {
	return map[string]any{"requirement_id": id, "kind": "scenario", "status": status, "scenario_version": version, "environment": environment, "locale": "en-US", "platform": platform, "input_paths": []string{"checkout.txt", "tests/checkout/retry.json"}, "evidence": []string{evidence}}
}

func openQualityGate(t *testing.T, server, base, token, policyID, kind, ref, revision, release string) qualitygates.Gate {
	t.Helper()
	var gate qualitygates.Gate
	workflowValue(t, server, http.MethodPost, base+"/quality-gates/candidates", token, qualitygates.OpenInput{PolicyID: policyID, PolicyVersion: 1, Target: qualitygates.Target{Kind: kind, Reference: ref, Revision: revision, Branch: "main", Release: release}}, http.StatusCreated, &gate)
	return gate
}
