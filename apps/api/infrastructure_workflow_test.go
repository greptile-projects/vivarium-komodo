package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructureplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type infrastructureWorkflowEnvironments struct{ approvals int }

func (e infrastructureWorkflowEnvironments) ExecutionEnvironment(string, string) (int, bool) {
	return e.approvals, true
}

// TestInfrastructureEvolutionWorkflow is the black-box boundary for the full
// proposal-to-reconciled-infrastructure loop. Unsafe and failed attempts remain
// beside the exact reviewed plan, governed apply, verification, and repair.
func TestInfrastructureEvolutionWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	definitions, _ := infrastructurestate.New(t.TempDir())
	plans, _ := infrastructureplans.New(t.TempDir(), infrastructurePlanPulls{pulls}, definitions)
	plans.ConfigureExecutionAuthority(infrastructureWorkflowEnvironments{approvals: 1})
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, nil, nil, nil)
	registerInfrastructureStateHTTP(mux, definitions, catalog, credentials)
	registerInfrastructurePlansHTTP(mux, plans, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "platform-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	security := issueAccess(t, credentials, "security-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	service := issueAccess(t, credentials, "service-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "infra-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "platform-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "infra-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repo repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"reviewed-infrastructure","visibility":"private"}`, 201, &repo)
	for _, actor := range []string{"security-owner", "service-owner", "infra-agent"} {
		if _, err := catalog.AddCollaborator("platform-owner", repo.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repo.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Platform Owner")
	gitOutput(t, work, "config", "user.email", "platform@example.test")
	writeWorkflowFile(t, work, "app/service.go", "package app\n\nconst replicas = 3\n")
	writeWorkflowFile(t, work, "infra/service.json", `{"service":"api","replicas":3,"recovery":"regional"}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Run API on regional capacity")
	gitOutput(t, work, "push", "-u", "origin", "main")
	gitOutput(t, work, "switch", "-c", "infra/regional-api")
	writeWorkflowFile(t, work, "app/service.go", "package app\n\nconst replicas = 4\n")
	writeWorkflowFile(t, work, "infra/service.json", `{"service":"api","replicas":4,"recovery":"regional","identity":"workload-v2"}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Scale API and rotate workload identity")
	firstCandidate := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "infra/regional-api")
	base := "/repositories/" + string(repo.ID)
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", owner, `{"title":"Scale API with reviewed infrastructure","body":"Application and infrastructure evolve together.","source_branch":"infra/regional-api","target_branch":"main"}`, 201, &pull)
	pullBase := base + "/pull-requests/" + pull.ID

	definitionInput := infrastructurestate.VersionInput{Name: "production API", Description: "Reviewed service, network, and identity inventory", SourceRevision: firstCandidate, DefinitionPath: "infra/service.json", Format: "komodo-infrastructure-v1", OwnerIDs: []string{"platform-owner"}, Environments: []infrastructurestate.Environment{{ID: "production", Name: "Production", Tier: "production", Regions: []string{"eu-west"}, OwnerIDs: []string{"platform-owner"}}}, Resources: []infrastructurestate.Resource{{ID: "network", Kind: "network", Name: "service network", Provider: "cloud", ProviderResource: "network/main", OwnerIDs: []string{"security-owner"}, Environments: []string{"production"}, Constraints: []infrastructurestate.Constraint{{Kind: "security", Commitment: "private ingress"}, {Kind: "cost", Commitment: "under USD 20"}}}, {ID: "api", Kind: "service", Name: "API service", Provider: "cloud", ProviderResource: "service/api", OwnerIDs: []string{"service-owner"}, DependsOn: []string{"network"}, Environments: []string{"production"}, Configuration: []infrastructurestate.Boundary{{Name: "provider lease", Source: "governed environment reference", SecretBacked: true, Classification: "restricted"}}, Constraints: []infrastructurestate.Constraint{{Kind: "reliability", Commitment: "99.9 percent"}, {Kind: "privacy", Commitment: "EU processing"}, {Kind: "continuity", Commitment: "regional recovery"}}}}, ChangeReason: "review application and infrastructure together"}
	b, _ := json.Marshal(definitionInput)
	var definition infrastructurestate.Definition
	workflowJSON(t, server.URL, http.MethodPost, base+"/infrastructure-definitions", owner, string(b), 201, &definition)
	now := time.Now().UTC()
	initialObservation := infrastructurestate.ObservationInput{DefinitionVersion: 1, SourceRevision: firstCandidate, EnvironmentID: "production", Provider: "cloud", ProviderAccessible: true, EvidenceReference: "provider:before-change", ObservedAt: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour), Resources: []infrastructurestate.ObservedResource{{ResourceID: "network", ProviderResource: "network/main", Kind: "network", Status: "ready", ConfigurationState: "matching"}, {ResourceID: "api", ProviderResource: "service/api", Kind: "service", Status: "ready", ConfigurationState: "matching"}}, Summary: "sanitized pre-change state"}
	b, _ = json.Marshal(initialObservation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/infrastructure-definitions/"+definition.ID+"/observations", owner, string(b), 201, &definition)

	risks := []infrastructureplans.Risk{}
	for _, kind := range []string{"availability", "security", "privacy", "continuity", "cost", "data"} {
		risks = append(risks, infrastructureplans.Risk{Kind: kind, Level: "medium", Detail: kind + " effect is bounded and reviewed", Mitigation: "rehearse and verify"})
	}
	planInput := infrastructureplans.Input{Revision: firstCandidate, Definitions: []infrastructureplans.DefinitionRef{{ID: definition.ID, Version: 1, ObservationIDs: []string{definition.Observations[0].ID}}}, Changes: []infrastructureplans.Change{{ResourceID: "network", Action: "change", EnvironmentIDs: []string{"production"}, OwnerIDs: []string{"security-owner"}, Summary: "tighten regional routes", Risks: risks, RollbackLimit: "restore reviewed route table"}, {ResourceID: "api", Action: "replace", EnvironmentIDs: []string{"production"}, DependsOn: []string{"network"}, OwnerIDs: []string{"service-owner"}, Summary: "destructively replace runtime identity while retaining service", RollbackLimit: "traffic rollback ends after old identity retirement"}}, PolicyEffects: []infrastructureplans.PolicyEffect{{PolicyID: "production-change", Revision: "v4", Effect: "satisfy", Detail: "protected environment and owner review required"}}, Assumptions: []string{"regional capacity remains available"}, RollbackLimits: []string{"old identity cannot be restored after retirement"}}
	b, _ = json.Marshal(planInput)
	var stalePlan infrastructureplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/infrastructure-plans", owner, string(b), 201, &stalePlan)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/infrastructure-plans/"+stalePlan.ID+"/annotations", agent, `{"kind":"investigation","body":"Scoped agent found capacity and recovery checks are required.","evidence_reference":"analysis:regional-capacity","resource_ids":["network","api"]}`, 201, &stalePlan)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/infrastructure-plans/"+stalePlan.ID+"/invalidations", owner, `{"kind":"provider","reference":"provider:capacity-refresh"}`, 201, &stalePlan)
	if !stalePlan.Stale || len(stalePlan.Annotations) != 1 {
		t.Fatalf("stale analysis trail missing: %+v", stalePlan)
	}
	// An invalidated plan cannot silently gain rehearsal evidence.
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/infrastructure-plans/"+stalePlan.ID+"/rehearsals", owner, `{}`, 422, nil)

	planInput.Assumptions = []string{"refreshed provider capacity is available"}
	b, _ = json.Marshal(planInput)
	var plan infrastructureplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/infrastructure-plans", owner, string(b), 201, &plan)
	planPath := pullBase + "/infrastructure-plans/" + plan.ID
	for _, request := range []struct{ token, ownerID, resources string }{{security, "security-owner", `["network"]`}, {service, "service-owner", `["api"]`}} {
		workflowJSON(t, server.URL, http.MethodPost, planPath+"/acknowledgements", request.token, `{"owner_id":"`+request.ownerID+`","resource_ids":`+request.resources+`}`, 201, &plan)
	}
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/acknowledgements/"+plan.Acknowledgements[0].ID, security, `{"decision":"concern","rationale":"deny until isolation proves private ingress"}`, 200, &plan)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/acknowledgements", owner, `{"owner_id":"security-owner","resource_ids":["network"]}`, 201, &plan)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/acknowledgements/"+plan.Acknowledgements[2].ID, security, `{"decision":"acknowledged","rationale":"bounded rehearsal will prove ingress"}`, 200, &plan)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/acknowledgements/"+plan.Acknowledgements[1].ID, service, `{"decision":"acknowledged","rationale":"service replacement and rollback limit understood"}`, 200, &plan)

	checks := []infrastructureplans.RehearsalCheck{}
	for _, kind := range []string{"provisioning", "connectivity", "access_boundary", "policy", "service_journey", "failure_behavior", "cost_estimate", "teardown", "recovery"} {
		checks = append(checks, infrastructureplans.RehearsalCheck{ID: kind, Kind: kind, Command: "checks/" + kind, Expected: "bounded pass", ResourceIDs: []string{"network", "api"}})
	}
	rehearsalInput := infrastructureplans.RehearsalInput{Title: "isolated regional replacement", Environment: infrastructureplans.RehearsalEnvironment{ID: "ephemeral-42", Kind: "isolated", Regions: []string{"eu-test"}, NetworkBoundary: "deny production routes"}, Credential: infrastructureplans.CredentialBoundary{Reference: "lease:rehearsal-42", Provider: "cloud", Scope: []string{"network:ephemeral", "service:ephemeral"}, EnvironmentIDs: []string{"ephemeral-42"}, ExpiresAt: now.Add(time.Hour)}, State: infrastructureplans.StateBoundary{Kind: "synthetic", Reference: "fixture:privacy-safe"}, Resources: []infrastructureplans.RehearsalResource{{ResourceID: "network", Support: "supported"}, {ResourceID: "api", Support: "supported"}}, Checks: checks, MaximumDurationSeconds: 600, MaximumCost: 10, Currency: "USD"}
	b, _ = json.Marshal(rehearsalInput)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/rehearsals", owner, string(b), 201, &plan)
	attempt := func(teardown string) infrastructureplans.AttemptInput {
		results := []infrastructureplans.CheckResult{}
		for _, check := range checks {
			results = append(results, infrastructureplans.CheckResult{CheckID: check.ID, Status: "passed", Summary: "sanitized pass", ArtifactDigests: []string{"sha256:" + check.ID}, DurationMillis: 5})
		}
		return infrastructureplans.AttemptInput{RunnerAttestation: "runner:isolated", StartedAt: now, CompletedAt: now.Add(time.Minute), Results: results, ResourceGraph: []infrastructureplans.ResourceGraphEdge{{From: "network", To: "api"}}, AgentActions: []infrastructureplans.AgentAction{{AgentID: "infra-agent", Action: "analyze", ResourceID: "api", Summary: "verified recovery and privacy outcomes without provider authority"}}, EstimatedCost: 4, TeardownStatus: teardown, TeardownAttestation: "provider reports ephemeral cleanup state", RecoveryStatus: "passed", RecoveryAttestation: "synthetic service recovered"}
	}
	b, _ = json.Marshal(attempt("failed"))
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/rehearsals/"+plan.Rehearsals[0].ID+"/attempts", owner, string(b), 201, &plan)
	if plan.Rehearsals[0].Ready {
		t.Fatal("failed teardown became passing evidence")
	}
	b, _ = json.Marshal(attempt("passed"))
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/rehearsals/"+plan.Rehearsals[0].ID+"/attempts", owner, string(b), 201, &plan)

	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, 200, &pull)
	executionInput := infrastructureplans.ExecutionInput{EnvironmentID: "production", ControllerID: "platform-owner", Credential: infrastructureplans.ExecutionCredential{Reference: "lease:production-short", Provider: "cloud", Scopes: []string{"network:change", "service:replace"}, EnvironmentID: "production", ExpiresAt: time.Now().Add(20 * time.Millisecond)}, Budget: infrastructureplans.ExecutionBudget{MaximumCost: 12, Currency: "USD"}, Delegations: []infrastructureplans.StepDelegation{{StepID: "network", AgentID: "infra-agent", Actions: []string{"apply", "observe"}, ExpiresAt: time.Now().Add(10 * time.Millisecond)}}}
	b, _ = json.Marshal(executionInput)
	var executionPlan infrastructureplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/executions", owner, string(b), 201, &executionPlan)
	expired := executionPlan.Executions[0]
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/executions/"+expired.ID+"/approvals", security, `{}`, 200, &executionPlan)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/executions/"+expired.ID+"/control", owner, `{"action":"start","reason":"begin protected window"}`, 200, &executionPlan)
	time.Sleep(25 * time.Millisecond)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/executions/"+expired.ID+"/steps/network", agent, `{"state":"succeeded","provider_response":"route accepted","health":"healthy","cost":2,"next_action":"replace service","safety_point":true}`, 422, nil)

	executionInput.Credential.Reference = "lease:production-recovery"
	executionInput.Credential.ExpiresAt = time.Now().Add(4 * time.Hour)
	executionInput.Delegations[0].ExpiresAt = time.Now().Add(30 * time.Minute)
	b, _ = json.Marshal(executionInput)
	workflowJSON(t, server.URL, http.MethodPost, planPath+"/executions", owner, string(b), 201, &executionPlan)
	execution := executionPlan.Executions[1]
	execPath := planPath + "/executions/" + execution.ID
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/approvals", security, `{}`, 200, &executionPlan)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/control", owner, `{"action":"start","reason":"approved protected apply"}`, 200, &executionPlan)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/steps/network", agent, `{"state":"succeeded","provider_response":"regional route accepted","health":"healthy","cost":3,"next_action":"controller replaces service","safety_point":true}`, 200, &executionPlan)
	// The scoped agent cannot execute the unrelated replacement, and cost above
	// the declared ceiling cannot be retained as provider success.
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/steps/api", agent, `{"state":"succeeded","provider_response":"replacement accepted","health":"healthy","cost":4,"next_action":"verify","safety_point":true}`, 422, nil)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/steps/api", owner, `{"state":"succeeded","provider_response":"replacement accepted","health":"healthy","cost":15,"next_action":"verify","safety_point":true}`, 422, nil)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/steps/api", owner, `{"state":"failed","provider_response":"provider capacity unavailable","health":"unhealthy","cost":2,"blocker":"provider failure","next_action":"pause at old traffic route","safety_point":true}`, 200, &executionPlan)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/control", owner, `{"action":"resume","reason":"provider recovered; retry from retained partial apply"}`, 200, &executionPlan)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/steps/api", owner, `{"state":"succeeded","provider_response":"replacement accepted after retry","health":"healthy","cost":6,"blocker":"","next_action":"verify service outcomes","safety_point":true}`, 200, &executionPlan)
	execution = executionPlan.Executions[1]
	if execution.State != "succeeded" || len(execution.Events) < 7 || execution.Spent != 9 {
		t.Fatalf("partial apply recovery trail incomplete: %+v", execution)
	}

	postApply := initialObservation
	postApply.ObservedAt, postApply.ValidUntil = time.Now().Add(time.Second), time.Now().Add(time.Hour)
	postApply.EvidenceReference, postApply.Summary = "provider:post-apply", "released service and infrastructure observed"
	b, _ = json.Marshal(postApply)
	workflowJSON(t, server.URL, http.MethodPost, base+"/infrastructure-definitions/"+definition.ID+"/observations", owner, string(b), 201, &definition)
	postApplyID := definition.Observations[len(definition.Observations)-1].ID
	measures := []infrastructureplans.OutcomeMeasure{}
	for _, kind := range []string{"service", "security", "privacy", "cost", "continuity"} {
		measures = append(measures, infrastructureplans.OutcomeMeasure{Kind: kind, Status: "passed", EvidenceReference: "check:" + kind, Detail: kind + " commitment passed"})
	}
	verification := infrastructureplans.VerificationInput{ObservationIDs: []string{postApplyID}, Resources: []infrastructureplans.ResourceComparison{{ResourceID: "network", ObservationID: postApplyID, ExpectedAction: "change", ObservedState: "ready", Status: "matched", EvidenceReference: "provider:post-apply", Detail: "private routes match"}, {ResourceID: "api", ObservationID: postApplyID, ExpectedAction: "replace", ObservedState: "ready", Status: "matched", EvidenceReference: "provider:post-apply", Detail: "replacement serves traffic"}}, Measures: measures}
	b, _ = json.Marshal(verification)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/verifications", owner, string(b), 201, &executionPlan)

	drift := postApply
	drift.ObservedAt, drift.ValidUntil = time.Now().Add(2*time.Second), time.Now().Add(time.Hour)
	drift.EvidenceReference, drift.Summary = "provider:console-drift", "out-of-band replica change observed"
	drift.Resources[1].ConfigurationState = "drifted"
	b, _ = json.Marshal(drift)
	workflowJSON(t, server.URL, http.MethodPost, base+"/infrastructure-definitions/"+definition.ID+"/observations", owner, string(b), 201, &definition)
	driftID := definition.Observations[len(definition.Observations)-1].ID
	monitor := infrastructureplans.DriftInput{ObservationIDs: []string{driftID}, Findings: []infrastructureplans.DriftFinding{{Kind: "configuration_drift", ResourceID: "api", Status: "open", Cause: "out-of-band provider console change", Evidence: "provider:audit-drift", Detail: "replicas differ from reviewed intent"}}}
	b, _ = json.Marshal(monitor)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/monitoring", agent, string(b), 201, &executionPlan)
	assessment := executionPlan.Executions[1].Monitoring[0]

	repairWork := gitClone(t, remote(agentGit))
	gitOutput(t, repairWork, "config", "user.name", "Infrastructure Agent")
	gitOutput(t, repairWork, "config", "user.email", "agent@example.test")
	gitOutput(t, repairWork, "switch", "-c", "repair/reconcile-replicas")
	writeWorkflowFile(t, repairWork, "infra/reconcile.md", "Restore replicas to reviewed value 4 through protected execution.\n")
	gitOutput(t, repairWork, "add", "infra/reconcile.md")
	gitOutput(t, repairWork, "commit", "-m", "Document reviewed drift reconciliation")
	repairRevision := gitOutput(t, repairWork, "rev-parse", "HEAD")
	gitOutput(t, repairWork, "push", "-u", "origin", "repair/reconcile-replicas")
	var repairPull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", agent, `{"title":"Reconcile out-of-band replica drift","body":"Scoped agent repair returns provider state to reviewed intent.","source_branch":"repair/reconcile-replicas","target_branch":"main"}`, 201, &repairPull)
	repairBase := base + "/pull-requests/" + repairPull.ID
	workflowJSON(t, server.URL, http.MethodPut, repairBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, repairBase+"/merge", owner, `{}`, 200, &repairPull)
	action := infrastructureplans.DriftActionInput{Kind: "repair", OwnerKind: "agent", OwnerID: "infra-agent", Reference: "pull:" + repairPull.ID, SourceRevision: repairRevision, Rationale: "ordinary owner-reviewed repair restores declared replicas"}
	b, _ = json.Marshal(action)
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/monitoring/"+assessment.ID+"/actions", owner, string(b), 201, &executionPlan)

	reconciled := postApply
	reconciled.ObservedAt, reconciled.ValidUntil = time.Now().Add(3*time.Second), time.Now().Add(time.Hour)
	reconciled.EvidenceReference, reconciled.Summary = "provider:reconciled", "reviewed repair restored matching infrastructure"
	reconciled.Resources = append([]infrastructurestate.ObservedResource(nil), postApply.Resources...)
	reconciled.Resources[1].ConfigurationState = "matching"
	b, _ = json.Marshal(reconciled)
	workflowJSON(t, server.URL, http.MethodPost, base+"/infrastructure-definitions/"+definition.ID+"/observations", owner, string(b), 201, &definition)
	reconciledID := definition.Observations[len(definition.Observations)-1].ID
	b, _ = json.Marshal(infrastructureplans.DriftInput{ObservationIDs: []string{reconciledID}})
	workflowJSON(t, server.URL, http.MethodPost, execPath+"/monitoring", agent, string(b), 201, &executionPlan)
	execution = executionPlan.Executions[1]
	if !execution.Verifications[0].Converged || execution.Monitoring[0].State != "drifted" || len(execution.Monitoring[0].Actions) != 1 || execution.Monitoring[1].State != "matching" || repairPull.Status != pullrequests.Merged {
		t.Fatalf("intent-to-reconciliation trail incomplete: execution=%+v repair=%+v", execution, repairPull)
	}
}
