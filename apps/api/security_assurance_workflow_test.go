package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securitydelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityexpectations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
)

// TestProactiveSecurityAssuranceWorkflow is the black-box boundary for the
// design-to-sustained-defense loop. It uses the public security workspace APIs
// and stock Git object identities while retaining every unsafe, stale, failed,
// and superseded security decision.
func TestProactiveSecurityAssuranceWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	creds, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "privileged-runner", Visibility: repositories.Private})
	for _, id := range []string{"security", "platform", "agent"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, id)
	}
	owner := issueAccess(t, creds, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	security := issueAccess(t, creds, "security", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, creds, "agent", auth.API, auth.RepositoryRead)

	expectations, _ := securityexpectations.New(t.TempDir())
	models, _ := threatmodels.New(t.TempDir())
	scenarios, _ := securityscenarios.New(t.TempDir())
	delivery, _ := securitydelivery.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	mux := http.NewServeMux()
	registerSecurityExpectationsHTTP(mux, expectations, repos, creds)
	registerThreatModelsHTTP(mux, models, repos, creds, threatModelSources{pulls: pulls, plans: plans, scenarios: scenarios})
	registerSecurityScenariosHTTP(mux, scenarios, models, repos, creds, pulls, nil)
	registerSecurityDeliveryHTTP(mux, delivery, models, scenarios, repos, orgs, creds)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID)

	opened, _ := repos.Open(repo.ID)
	commit := func(parent storage.ObjectID, body, message, who string) storage.ObjectID {
		blob, _ := opened.WriteObject(storage.BlobObject, []byte(body))
		tree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "runner.txt", blob))
		p := ""
		if parent != "" {
			p = "parent " + string(parent) + "\n"
		}
		c, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+p+"author "+who+" <"+who+"@example.test> 1 +0000\ncommitter "+who+" <"+who+"@example.test> 1 +0000\n\n"+message+"\n"))
		return c
	}
	base := commit("", "runner trusts caller supplied project\n", "propose privileged runner", "platform")
	redesign := commit(base, "runner resolves project from signed job scope\n", "bind jobs to signed scope", "agent")
	repair := commit(redesign, "runner binds scope and rejects replayed jobs\n", "repair replay window", "agent")
	monitorRepair := commit(repair, "runner binds scope, rejects replay, and rotates audience keys\n", "restore audience control", "platform")
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), AuthorID: "platform", Title: "Add privileged runner", SourceBranch: "runner", TargetBranch: "main", SourceCommitID: string(base), TargetCommitID: string(base)})

	expectation := securityexpectations.Input{Name: "Privileged job isolation", Description: "A job may act only for its signed project scope", Scopes: []securityexpectations.Scope{{Kind: "service", Reference: "runner"}, {Kind: "user_journey", Reference: "privileged-job"}}, Assets: []securityexpectations.Asset{{ID: "token", Name: "job capability", Classification: "critical", Protection: "integrity", OwnerIDs: []string{"security"}}}, Actors: []securityexpectations.Actor{{ID: "contributor", Description: "repository contributor", Capabilities: []string{"submit job"}}}, Boundaries: []securityexpectations.Boundary{{ID: "queue-runner", From: "queue", To: "privileged runner", AssetIDs: []string{"token"}, Allowed: true, Authentication: "signed job scope", Rationale: "execute reviewed work"}}, Controls: []securityexpectations.Control{{ID: "scope", Description: "bind every job to repository and audience", Guarantee: "cross-project and replayed jobs are rejected", OwnerIDs: []string{"security"}, Supported: true}}, AbuseCases: []securityexpectations.AbuseCase{{ID: "confused-deputy", ActorID: "contributor", AssetIDs: []string{"token"}, BoundaryIDs: []string{"queue-runner"}, Description: "substitute another project", Impact: "privileged cross-project action", Severity: "critical", ControlIDs: []string{"scope"}, OwnerIDs: []string{"security"}}}, SeverityPolicy: []securityexpectations.SeverityRule{{Severity: "critical", Response: "block delivery", ReleaseBlocking: true}}, OwnerIDs: []string{"security", "platform"}, Links: []securityexpectations.Link{{Kind: "design", Reference: "runner-v1", Commitment: "scope is server derived"}, {Kind: "infrastructure", Reference: "isolated-workers", Commitment: "no production credentials"}, {Kind: "release", Reference: "runner", Commitment: "critical paths block"}}, ChangeReason: "agree protection before implementation"}
	var published securityexpectations.Expectation
	workflowValue(t, server.URL, http.MethodPost, root+"/security-expectations", owner, expectation, http.StatusCreated, &published)
	if len(published.Gaps) != 1 || published.Gaps[0].Kind != "missing_owner" {
		t.Fatalf("expected explicit boundary ownership gap: %#v", published.Gaps)
	}

	modelInput := func(revision string, title string) threatmodels.Input {
		return threatmodels.Input{Title: title, Summary: "Trace privilege from job submission to runner", Origin: threatmodels.Origin{Kind: "pull_request", Reference: pull.ID, Revision: revision}, Inputs: []threatmodels.InputBinding{{Kind: "code", Reference: "runner.txt", Revision: revision}, {Kind: "trust_boundary", Reference: published.ID, Revision: "1"}}, EntryPoints: []threatmodels.EntryPoint{{ID: "job", Description: "submitted job envelope", Privileges: []string{"select inputs"}, OwnerIDs: []string{"platform"}}}, DataFlows: []threatmodels.DataFlow{{ID: "dispatch", From: "queue", To: "runner", Data: []string{"project", "audience"}, Boundary: "queue-runner"}}, AttackerGoals: []threatmodels.AttackerGoal{{ID: "deputy", Actor: "contributor", Goal: "run as another project", Capability: "submit job", Impact: "cross-project write"}}, AbusePaths: []threatmodels.AbusePath{{ID: "substitute", GoalID: "deputy", EntryPointIDs: []string{"job"}, DataFlowIDs: []string{"dispatch"}, Steps: []string{"submit mismatched project", "reuse privileged audience"}, MitigationIDs: []string{"bind"}, ResidualRisk: "key audience may change after release", Severity: "critical", OwnerIDs: []string{"security", "platform"}}}, Mitigations: []threatmodels.Mitigation{{ID: "bind", Description: "derive project and audience from signed scope", Status: "designed", OwnerIDs: []string{"security"}}}, Alternatives: []threatmodels.Alternative{{ID: "caller-project", Description: "trust caller project field", SecurityEffect: "preserves confused deputy path", AbusePathIDs: []string{"substitute"}}, {ID: "signed-scope", Description: "server-derived signed scope", SecurityEffect: "removes caller substitution", AbusePathIDs: []string{"substitute"}}}, OwnerIDs: []string{"security", "platform"}, ResidualRisk: "audience configuration remains monitored"}
	}
	var design threatmodels.Model
	designInput := modelInput(string(base), "Privileged runner design")
	designInput.Mitigations[0].Status = "proposed"
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models", security, designInput, http.StatusCreated, &design)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings", agent, threatmodels.FindingInput{Kind: "finding", Body: "caller project reaches privileged dispatch", AbusePathIDs: []string{"substitute"}, AlternativeIDs: []string{"signed-scope"}, Citations: []threatmodels.Citation{{Kind: "code", Reference: "runner.txt", Revision: string(base), Detail: "project is caller supplied", Visibility: "repository"}}}, http.StatusCreated, &design)
	falsePositive := design.Findings[0].ID
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings/"+falsePositive+"/classification", security, map[string]any{"kind": "false_positive", "audience": "repository", "rationale": "this citation describes the already removed parser path"}, http.StatusCreated, &design)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings/"+falsePositive+"/resolution", security, map[string]any{"kind": "false_positive", "rationale": "wrong dispatch path cited"}, http.StatusCreated, &design)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings", agent, threatmodels.FindingInput{Kind: "finding", Body: "project substitution crosses the privileged boundary", AbusePathIDs: []string{"substitute"}, AlternativeIDs: []string{"signed-scope"}, Citations: []threatmodels.Citation{{Kind: "code", Reference: "runner.txt", Revision: string(base), Detail: "caller project reaches dispatch", Visibility: "repository"}}}, http.StatusCreated, &design)
	finding := design.Findings[1].ID
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings/"+finding+"/classification", security, map[string]any{"kind": "confirmed", "audience": "repository", "rationale": "synthetic evidence is safe for repair"}, http.StatusCreated, &design)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/acknowledgements", security, map[string]any{"decision": "request_changes", "rationale": "select the signed-scope alternative", "origin_revision": string(base)}, http.StatusCreated, &design)

	_, _ = pulls.SynchronizeSource(string(repo.ID), pull.ID, string(redesign))
	var stale threatmodels.Model
	workflowValue(t, server.URL, http.MethodGet, root+"/threat-models/"+design.ID, security, nil, http.StatusOK, &stale)
	if !stale.Stale || !stale.Acknowledgements[0].Stale {
		t.Fatalf("changed design did not stale analysis and acknowledgement: %#v", stale)
	}
	// The old finding remains attributable, but cannot open work against the changed candidate.
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+design.ID+"/findings/"+finding+"/delivery", security, map[string]any{"owner_kind": "agent", "owner_id": "agent", "candidate_revision": string(redesign), "abuse_path_ids": []string{"substitute"}, "permitted_evidence_references": []string{"runner.txt"}, "acceptance_criteria": []string{"base abuse fails", "repair contains all three domains"}}, http.StatusUnprocessableEntity, nil)

	// Rebind the redesigned candidate, then retain inaccessible and unsafe tests.
	var current threatmodels.Model
	currentInput := modelInput(string(redesign), "Redesigned privileged runner")
	currentInput.Mitigations[0].Status = "proposed"
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models", security, currentInput, http.StatusCreated, &current)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+current.ID+"/findings", agent, threatmodels.FindingInput{Kind: "finding", Body: "signed scope remains replayable until the worker records its nonce", AbusePathIDs: []string{"substitute"}, Citations: []threatmodels.Citation{{Kind: "code", Reference: "runner.txt", Revision: string(redesign), Detail: "scope has no replay record", Visibility: "repository"}}}, http.StatusCreated, &current)
	currentFinding := current.Findings[0].ID
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+current.ID+"/findings/"+currentFinding+"/classification", security, map[string]any{"kind": "confirmed", "audience": "repository", "rationale": "credential-free synthetic replay is permitted"}, http.StatusCreated, &current)
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+current.ID+"/findings/"+currentFinding+"/delivery", security, map[string]any{"owner_kind": "agent", "owner_id": "agent", "candidate_revision": string(redesign), "abuse_path_ids": []string{"substitute"}, "permitted_evidence_references": []string{"runner.txt"}, "acceptance_criteria": []string{"base demonstrates replay", "repair contains, detects, and recovers"}}, http.StatusCreated, nil)
	scenarioInput := securityScenario(current.ID, string(redesign))
	unsafe := scenarioInput
	unsafe.Fixtures[0].ContainsSecrets = true
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios", security, unsafe, http.StatusUnprocessableEntity, nil)
	scenarioInput = securityScenario(current.ID, string(redesign))
	var scenario securityscenarios.Scenario
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios", security, scenarioInput, http.StatusCreated, &scenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/reviews", security, map[string]any{"scenario_version": 1, "decision": "approve", "rationale": "networkless synthetic attack is bounded"}, http.StatusCreated, &scenario)
	blocked := securityAttempt(pull.ID, string(redesign), "blocked")
	blocked.InaccessibleDependencies = []string{"restricted identity provider evidence"}
	blocked.Blockers = []string{"identity provider evidence inaccessible to this agent"}
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/attempts", agent, blocked, http.StatusCreated, &scenario)
	bad := securityAttempt(pull.ID, string(redesign), "unsafe")
	bad.DestructiveEffects = true
	bad.Blockers = []string{"would mutate a shared project"}
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/attempts", agent, bad, http.StatusCreated, &scenario)
	failed := securityAttempt(pull.ID, string(redesign), "failed")
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/attempts", agent, failed, http.StatusCreated, &scenario)
	if len(scenario.Attempts) != 3 {
		t.Fatal("contained evidence trail was discarded")
	}

	// The first repair still fails. A distinct repair is then proven and reviewed.
	_, _ = pulls.SynchronizeSource(string(repo.ID), pull.ID, string(repair))
	repairScenarioInput := securityScenario(current.ID, string(repair))
	repairScenarioInput.ThreatModelRevision = string(redesign)
	repairScenarioInput.ChangeReason = "exercise the repaired candidate without rewriting base evidence"
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/versions", security, repairScenarioInput, http.StatusCreated, &scenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/reviews", security, map[string]any{"scenario_version": 2, "decision": "approve", "rationale": "same abuse path against the repaired candidate"}, http.StatusCreated, &scenario)
	firstRepair := securityAttempt(pull.ID, string(repair), "failed")
	firstRepair.ScenarioVersion = 2
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/attempts", agent, firstRepair, http.StatusCreated, &scenario)
	passed := securityAttempt(pull.ID, string(repair), "passed")
	passed.ScenarioVersion = 2
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+scenario.ID+"/attempts", security, passed, http.StatusCreated, &scenario)
	_, _ = pulls.PutReview(string(repo.ID), pull.ID, "owner", pullrequests.Approve, string(repair))
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models/"+current.ID+"/findings/"+currentFinding+"/verification", security, map[string]any{"pull_request_id": pull.ID, "design_change_references": []string{"design:signed-scope-with-nonce"}, "commit_ids": []string{string(repair)}, "review_id": "review:owner-approved", "security_scenario_id": scenario.ID, "base_attempt_id": scenario.Attempts[2].ID, "repair_attempt_id": scenario.Attempts[len(scenario.Attempts)-1].ID, "mitigation_coverage": []string{"bind"}}, http.StatusCreated, &current)

	// Delivery uses a fresh revision-bound assurance model and scenario.
	var assurance threatmodels.Model
	assuranceInput := modelInput(string(repair), "Repair assurance")
	assuranceInput.Mitigations[0].Status = "verified"
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models", security, assuranceInput, http.StatusCreated, &assurance)
	assuranceScenarioInput := securityScenario(assurance.ID, string(repair))
	var assuranceScenario securityscenarios.Scenario
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios", security, assuranceScenarioInput, http.StatusCreated, &assuranceScenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+assuranceScenario.ID+"/reviews", security, map[string]any{"scenario_version": 1, "decision": "approve", "rationale": "reviewed regression"}, http.StatusCreated, &assuranceScenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+assuranceScenario.ID+"/attempts", security, securityAttempt(pull.ID, string(repair), "passed"), http.StatusCreated, &assuranceScenario)
	var policy securitydelivery.Policy
	workflowValue(t, server.URL, http.MethodPost, root+"/security-delivery-policies", owner, securitydelivery.PolicyInput{Name: "Privileged runner delivery", Branches: []string{"main"}, Components: []string{"runner"}, Assets: []string{"job capability"}, RiskClasses: []string{"critical"}, RequiredThreatModels: []string{assurance.ID}, RequiredScenarios: []string{assuranceScenario.ID}, RequiredControlOwnerIDs: []string{"security"}, RequireResolvedFindings: true}, http.StatusCreated, &policy)
	// A release-blocking exception cannot be disguised as an invalid long-lived waiver.
	workflowValue(t, server.URL, http.MethodPost, root+"/security-delivery/exceptions", owner, map[string]any{"policy_id": policy.ID, "subject_kind": "pull_request", "subject_id": pull.ID, "revision": string(repair), "reason": "ship without owner review", "requirement_kinds": []string{"control_owner_acknowledgement"}, "expires_at": time.Now().Add(31 * 24 * time.Hour)}, http.StatusUnprocessableEntity, nil)

	for _, target := range []struct{ kind, id string }{{"pull_request", pull.ID}, {"integration_queue", "queue:main:42"}, {"release", "runner-v1"}, {"deployment", "staging:runner-v1"}} {
		workflowValue(t, server.URL, http.MethodPost, root+"/security-delivery/acknowledgements", security, map[string]any{"policy_id": policy.ID, "subject_kind": target.kind, "subject_id": target.id, "revision": string(repair), "decision": "accept", "rationale": "current three-domain evidence and ordinary review"}, http.StatusCreated, nil)
		var assessment securitydelivery.Assessment
		workflowValue(t, server.URL, http.MethodPost, root+"/security-delivery/assessments", security, map[string]any{"subject_kind": target.kind, "subject_id": target.id, "revision": string(repair), "branch": "main", "components": []string{"runner"}, "assets": []string{"job capability"}, "risk_classes": []string{"critical"}, "threat_model_ids": []string{assurance.ID}, "scenario_ids": []string{assuranceScenario.ID}}, http.StatusOK, &assessment)
		if !assessment.Ready {
			t.Fatalf("%s security gate blocked: %#v", target.kind, assessment.Requirements)
		}
	}

	var signal securitydelivery.Signal
	workflowValue(t, server.URL, http.MethodPost, root+"/security-signals", security, map[string]any{"deployment_id": "prod:runner-v1:canary", "release_id": "runner-v1", "revision": string(repair), "environment": "production-canary", "assumption": "issuer audience keys remain project scoped", "control_id": "scope", "outcome": "violated", "summary": "sanitized aggregate shows audience mismatch denials increasing", "input_keys": []string{"control:scope", "assumption:audience"}, "observed_at": time.Now(), "sanitized": true}, http.StatusCreated, &signal)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-signals/"+signal.ID+"/responses", owner, map[string]any{"kind": "repair", "resource_id": "private-repair:audience-rotation"}, http.StatusCreated, &signal)
	if signal.Response == nil || signal.Response.Kind != "repair" {
		t.Fatal("changed assumption did not open a private connected repair")
	}

	// The connected repair restores the control with new exact evidence; old
	// evidence remains present and cannot satisfy this revision.
	_, _ = pulls.SynchronizeSource(string(repo.ID), pull.ID, string(monitorRepair))
	var restored threatmodels.Model
	restoredModelInput := modelInput(string(monitorRepair), "Audience rotation repair")
	restoredModelInput.Mitigations[0].Status = "verified"
	workflowValue(t, server.URL, http.MethodPost, root+"/threat-models", security, restoredModelInput, http.StatusCreated, &restored)
	restoredInput := securityScenario(restored.ID, string(monitorRepair))
	var restoredScenario securityscenarios.Scenario
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios", security, restoredInput, http.StatusCreated, &restoredScenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+restoredScenario.ID+"/reviews", security, map[string]any{"scenario_version": 1, "decision": "approve", "rationale": "rotation and rollback are observable"}, http.StatusCreated, &restoredScenario)
	workflowValue(t, server.URL, http.MethodPost, root+"/security-scenarios/"+restoredScenario.ID+"/attempts", security, securityAttempt(pull.ID, string(monitorRepair), "passed"), http.StatusCreated, &restoredScenario)
	var restoredSignal securitydelivery.Signal
	workflowValue(t, server.URL, http.MethodPost, root+"/security-signals", security, map[string]any{"deployment_id": "prod:runner-v1:stage-2", "release_id": "runner-v1.0.1", "revision": string(monitorRepair), "environment": "production-stage-2", "assumption": "issuer audience keys remain project scoped", "control_id": "scope", "outcome": "satisfied", "summary": "sanitized mismatch rate returned to zero after rotation", "input_keys": []string{"control:scope", "assumption:audience"}, "observed_at": time.Now(), "sanitized": true}, http.StatusCreated, &restoredSignal)
	if restoredSignal.Violated || signal.Response.ResourceID == "" {
		t.Fatal("restored control lost its connected prior failure")
	}
}

func securityScenario(modelID, revision string) securityscenarios.Input {
	return securityscenarios.Input{Name: "Project substitution is contained", ThreatModelID: modelID, ThreatModelRevision: revision, AbusePathID: "substitute", SourceRevision: revision, DefinitionPath: ".komodo/security-checks.json", AttackerPreconditions: []string{"attacker can submit a synthetic job"}, Capabilities: []securityscenarios.Capability{{Name: "submit mismatched project", Boundary: "synthetic fixture in networkless workspace"}}, Fixtures: []securityscenarios.Fixture{{ID: "projects", Description: "two generated project identities", Generator: "go test ./security/fixtures", Synthetic: true}}, Actions: []securityscenarios.Action{{ID: "submit", Description: "submit cross-project and replayed jobs", Command: "go test ./security -run TestProjectScope"}}, Containment: []securityscenarios.Criterion{{ID: "deny", Description: "cross-project action is denied", Observable: "permission_denied"}}, Detection: []securityscenarios.Criterion{{ID: "audit", Description: "denial is attributed", Observable: "scope_mismatch event"}}, Recovery: []securityscenarios.Criterion{{ID: "healthy", Description: "valid project jobs continue", Observable: "health and valid job pass"}}, OwnerIDs: []string{"security"}, ChangeReason: "make the agreed abuse path reproducible"}
}

func securityAttempt(pull, revision, status string) securityscenarios.AttemptInput {
	return securityscenarios.AttemptInput{ScenarioVersion: 1, TargetKind: "workspace", PullRequestID: pull, Revision: revision, Isolation: "ephemeral", Network: "none", Status: status, Commands: []string{"go test ./security -run TestProjectScope"}, Logs: []string{"synthetic job [redacted]"}, Traces: []string{"queue -> scope -> deny"}, Artifacts: []securityscenarios.Artifact{{Name: "trace", Digest: "sha256:security", MediaType: "application/json", Size: 24, Sanitized: true}}, Coverage: securityscenarios.Coverage{ContainmentIDs: []string{"deny"}, DetectionIDs: []string{"audit"}, RecoveryIDs: []string{"healthy"}}, Cost: 0.03, Currency: "USD", Provenance: []string{"pull@" + revision, ".komodo/security-checks.json"}}
}
