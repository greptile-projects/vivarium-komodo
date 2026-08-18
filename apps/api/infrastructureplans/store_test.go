package infrastructureplans

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

type fakePull struct {
	revision, merge string
	merged          bool
}

func (f *fakePull) CurrentRevision(string, string) (string, error) { return f.revision, nil }
func (f *fakePull) MergedRevision(string, string) (string, string, bool, error) {
	return f.revision, f.merge, f.merged, nil
}

type fakeEnvironments struct{ approvals int }

func (f fakeEnvironments) ExecutionEnvironment(string, string) (int, bool) { return f.approvals, true }

type fakeDefinitions struct {
	value infrastructurestate.Definition
}

func TestExecutionBindsMergedPlanEnvironmentAuthorityAndSafeAgentSteps(t *testing.T) {
	now := time.Now().UTC()
	pulls := &fakePull{revision: "candidate", merge: "merged", merged: true}
	defs := &fakeDefinitions{value: infrastructurestate.Definition{ID: "inventory", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}}}
	s, _ := New(t.TempDir(), pulls, defs)
	s.now = func() time.Time { return now }
	s.ConfigureExecutionAuthority(fakeEnvironments{approvals: 1})
	risks := []Risk{}
	for _, kind := range []string{"availability", "security", "privacy", "continuity", "cost", "data"} {
		risks = append(risks, Risk{Kind: kind, Level: "low", Detail: "bounded"})
	}
	p, err := s.Create("repo", "pull", "author", Input{Revision: "candidate", Definitions: []DefinitionRef{{ID: "inventory", Version: 1}}, Changes: []Change{{ResourceID: "network", Action: "change", EnvironmentIDs: []string{"production"}, OwnerIDs: []string{"owner"}, Summary: "update network", RollbackLimit: "restore route", Risks: risks}, {ResourceID: "service", Action: "change", EnvironmentIDs: []string{"production"}, OwnerIDs: []string{"owner"}, DependsOn: []string{"network"}, Summary: "update service", RollbackLimit: "restore service"}}, PolicyEffects: []PolicyEffect{{PolicyID: "environment", Revision: "1", Effect: "satisfy", Detail: "approved change window"}}, Assumptions: []string{"capacity exists"}, RollbackLimits: []string{"restore before contract"}})
	if err != nil {
		t.Fatal(err)
	}
	p, _ = s.Request("repo", "pull", p.ID, "reviewer", "owner", []string{"network", "service"})
	p, _ = s.Decide("repo", "pull", p.ID, p.Acknowledgements[0].ID, "owner", "acknowledged", "ready")
	checks := []RehearsalCheck{}
	for kind := range checkKinds {
		checks = append(checks, RehearsalCheck{ID: kind, Kind: kind, Command: "check", Expected: "pass", ResourceIDs: []string{"network", "service"}})
	}
	p, err = s.CreateRehearsal("repo", "pull", p.ID, "operator", RehearsalInput{Title: "safe rehearsal", Environment: RehearsalEnvironment{ID: "ephemeral", Kind: "isolated", NetworkBoundary: "isolated"}, Credential: CredentialBoundary{Reference: "lease:rehearsal", Provider: "cloud", Scope: []string{"read"}, EnvironmentIDs: []string{"ephemeral"}, ExpiresAt: now.Add(time.Hour)}, State: StateBoundary{Kind: "synthetic", Reference: "fixture"}, Resources: []RehearsalResource{{ResourceID: "network", Support: "supported"}, {ResourceID: "service", Support: "supported"}}, Checks: checks, MaximumDurationSeconds: 60, MaximumCost: 5, Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	results := []CheckResult{}
	for _, c := range checks {
		results = append(results, CheckResult{CheckID: c.ID, Status: "passed", Summary: "ok", DurationMillis: 1})
	}
	p, err = s.RecordRehearsalAttempt("repo", "pull", p.ID, p.Rehearsals[0].ID, "operator", AttemptInput{RunnerAttestation: "runner", StartedAt: now, CompletedAt: now.Add(time.Minute), Results: results, TeardownStatus: "passed", TeardownAttestation: "empty", RecoveryStatus: "passed", RecoveryAttestation: "restored"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.StartExecution("repo", "pull", p.ID, "owner", ExecutionInput{EnvironmentID: "production", ControllerID: "owner", Credential: ExecutionCredential{Reference: "lease:production", Provider: "cloud", Scopes: []string{"network:change", "service:change"}, EnvironmentID: "production", ExpiresAt: now.Add(time.Hour)}, Budget: ExecutionBudget{MaximumCost: 10, Currency: "USD"}, Delegations: []StepDelegation{{StepID: "network", AgentID: "agent", Actions: []string{"apply", "observe"}, ExpiresAt: now.Add(30 * time.Minute)}}})
	if err != nil {
		t.Fatalf("%v plan=%+v", err, p)
	}
	x := p.Executions[0]
	if x.MergedRevision != "merged" || x.State != "awaiting_approvals" || x.Steps[0].ResourceID != "network" {
		t.Fatalf("bad execution: %+v", x)
	}
	p, _ = s.ApproveExecution("repo", "pull", p.ID, x.ID, "approver")
	p, _ = s.ControlExecution("repo", "pull", p.ID, x.ID, "owner", "start", "begin approved window")
	p, err = s.UpdateExecutionStep("repo", "pull", p.ID, x.ID, "network", "agent", StepUpdate{State: "succeeded", ProviderResponse: "provider accepted change", Health: "healthy", Cost: 2, NextAction: "apply service", SafetyPoint: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateExecutionStep("repo", "pull", p.ID, x.ID, "service", "agent", StepUpdate{State: "succeeded", ProviderResponse: "provider accepted", Health: "healthy", Cost: 2, NextAction: "verify", SafetyPoint: true})
	if err != ErrInvalid {
		t.Fatalf("agent gained unrelated step authority: %v", err)
	}
	p, err = s.ControlExecution("repo", "pull", p.ID, x.ID, "owner", "pause", "inspect provider response")
	if err != nil || p.Executions[0].State != "paused" {
		t.Fatalf("safe pause failed: %v %+v", err, p.Executions[0])
	}
	p, _ = s.ControlExecution("repo", "pull", p.ID, x.ID, "owner", "resume", "continue after inspection")
	p, err = s.UpdateExecutionStep("repo", "pull", p.ID, x.ID, "service", "owner", StepUpdate{State: "succeeded", ProviderResponse: "provider accepted service change", Health: "healthy", Cost: 3, NextAction: "verify reviewed outcomes", SafetyPoint: true})
	if err != nil || p.Executions[0].State != "succeeded" {
		t.Fatalf("apply did not retain successful provider completion: %v %+v", err, p.Executions[0])
	}
	defs.value.Observations = []infrastructurestate.Observation{{ID: "post-apply", ObservationInput: infrastructurestate.ObservationInput{DefinitionVersion: 1, SourceRevision: "candidate", EnvironmentID: "production", Provider: "cloud", ProviderAccessible: true, EvidenceReference: "provider:snapshot-1", ObservedAt: now, ValidUntil: now.Add(time.Hour), Resources: []infrastructurestate.ObservedResource{{ResourceID: "network", ProviderResource: "network/main", Kind: "network", Status: "ready", ConfigurationState: "matching"}, {ResourceID: "service", ProviderResource: "service/api", Kind: "service", Status: "degraded", ConfigurationState: "drifted"}}}}}
	measures := []OutcomeMeasure{}
	for _, kind := range []string{"service", "security", "privacy", "cost", "continuity"} {
		status := "passed"
		if kind == "service" {
			status = "failed"
		}
		measures = append(measures, OutcomeMeasure{Kind: kind, Status: status, EvidenceReference: "check:" + kind, Detail: kind + " outcome checked"})
	}
	p, err = s.VerifyExecution("repo", "pull", p.ID, x.ID, "owner", VerificationInput{ObservationIDs: []string{"post-apply"}, Resources: []ResourceComparison{{ResourceID: "network", ObservationID: "post-apply", ExpectedAction: "change", ObservedState: "ready", Status: "matched", EvidenceReference: "provider:snapshot-1", Detail: "route matches"}, {ResourceID: "service", ObservationID: "post-apply", ExpectedAction: "change", ObservedState: "degraded", Status: "failed", EvidenceReference: "provider:snapshot-1", Detail: "health check failed"}}, Measures: measures})
	if err != nil || p.Executions[0].State != "diverged" || p.Executions[0].Verifications[0].Converged {
		t.Fatalf("failed verification claimed convergence: %v %+v", err, p.Executions[0])
	}
	measures[0].Status = "passed"
	defs.value.Observations = append(defs.value.Observations, infrastructurestate.Observation{ID: "post-recovery", ObservationInput: infrastructurestate.ObservationInput{DefinitionVersion: 1, SourceRevision: "candidate", EnvironmentID: "production", Provider: "cloud", ProviderAccessible: true, EvidenceReference: "provider:snapshot-2", ObservedAt: now.Add(2 * time.Minute), ValidUntil: now.Add(time.Hour), Resources: []infrastructurestate.ObservedResource{{ResourceID: "network", ProviderResource: "network/main", Kind: "network", Status: "ready", ConfigurationState: "matching"}, {ResourceID: "service", ProviderResource: "service/api", Kind: "service", Status: "ready", ConfigurationState: "matching"}}}})
	p, err = s.VerifyExecution("repo", "pull", p.ID, x.ID, "owner", VerificationInput{ObservationIDs: []string{"post-recovery"}, Resources: []ResourceComparison{{ResourceID: "network", ObservationID: "post-recovery", ExpectedAction: "change", ObservedState: "ready", Status: "matched", EvidenceReference: "provider:snapshot-2", Detail: "route matches"}, {ResourceID: "service", ObservationID: "post-recovery", ExpectedAction: "change", ObservedState: "ready", Status: "matched", EvidenceReference: "provider:snapshot-2", Detail: "service recovered"}}, Measures: measures})
	if err != nil || p.Executions[0].State != "verified" || !p.Executions[0].Verifications[1].Converged {
		t.Fatalf("passing evidence did not prove convergence: %v %+v", err, p.Executions[0])
	}
	p, err = s.MonitorExecution("repo", "pull", p.ID, x.ID, "reader", DriftInput{ObservationIDs: []string{"post-apply"}, Findings: []DriftFinding{{Kind: "configuration_drift", ResourceID: "service", Status: "open", Cause: "provider console change by external operator", Evidence: "provider:audit-2", Detail: "replica count differs"}}})
	if err != nil || p.Executions[0].State != "diverged" || p.Executions[0].Monitoring[0].State != "drifted" {
		t.Fatalf("drift was not retained: %v %+v", err, p.Executions[0])
	}
	a := p.Executions[0].Monitoring[0]
	p, err = s.OpenDriftAction("repo", "pull", p.ID, x.ID, a.ID, "owner", DriftActionInput{Kind: "adopt", OwnerKind: "human", OwnerID: "owner", Reference: "pull:adopt-replicas", SourceRevision: "review-candidate", Rationale: "review legitimate emergency capacity"})
	if err != nil || p.Executions[0].Monitoring[0].Actions[0].State != "pending_review" {
		t.Fatalf("ordinary-review adoption was not linked: %v %+v", err, p.Executions[0].Monitoring[0])
	}
}

func (f *fakeDefinitions) Get(string, string) (infrastructurestate.Definition, error) {
	return f.value, nil
}

func TestPlanOrdersChangesAndInvalidatesCollaboration(t *testing.T) {
	rev := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pulls := &fakePull{revision: rev}
	defs := &fakeDefinitions{value: infrastructurestate.Definition{ID: "inventory", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}, Observations: []infrastructurestate.Observation{{ID: "observation-1", ObservationInput: infrastructurestate.ObservationInput{EnvironmentID: "production", ObservedAt: time.Now()}}}}}
	s, err := New(t.TempDir(), pulls, defs)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Revision: rev, Definitions: []DefinitionRef{{ID: "inventory", Version: 1, ObservationIDs: []string{"observation-1"}}}, Changes: []Change{
		{ResourceID: "api", Action: "replace", EnvironmentIDs: []string{"production"}, DependsOn: []string{"database"}, OwnerIDs: []string{"service-owner"}, Summary: "replace runtime", RollbackLimit: "traffic rollback ends after contract", Risks: []Risk{{Kind: "availability", Level: "high", Detail: "restart"}, {Kind: "security", Level: "medium", Detail: "identity changes"}, {Kind: "privacy", Level: "medium", Detail: "processing boundary"}, {Kind: "continuity", Level: "high", Detail: "recovery window"}, {Kind: "cost", Level: "low", Detail: "overlap"}, {Kind: "data", Level: "critical", Detail: "schema compatibility"}}},
		{ResourceID: "database", Action: "change", EnvironmentIDs: []string{"production"}, OwnerIDs: []string{"data-owner"}, Summary: "expand storage", RollbackLimit: "allocated storage cannot shrink"},
	}, PolicyEffects: []PolicyEffect{{PolicyID: "security-policy", Revision: "v3", Effect: "exception_required", Detail: "replacement overlaps identities"}}, Assumptions: []string{"provider capacity remains available"}, RollbackLimits: []string{"no destructive action is authorized"}}
	p, err := s.Create("repo", "pull", "author", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DependencyOrder) != 2 || p.DependencyOrder[0] != "database" || p.Stale {
		t.Fatalf("unexpected plan: %+v", p)
	}
	p, err = s.Annotate("repo", "pull", p.ID, "reader", "investigation", "confirmed regional capacity", "evidence:capacity", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.Request("repo", "pull", p.ID, "reader", "service-owner", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	ack := p.Acknowledgements[0].ID
	p, err = s.Decide("repo", "pull", p.ID, ack, "service-owner", "acknowledged", "impact understood")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Acknowledgements[0].Current || len(p.Annotations) != 1 {
		t.Fatalf("collaboration missing: %+v", p)
	}
	pulls.revision = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	p, err = s.Get("repo", "pull", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Stale || p.Acknowledgements[0].Current || p.StaleReasons[0] != "source_revision_changed" {
		t.Fatalf("source change silently reused plan: %+v", p)
	}
}

func TestPlanRejectsCyclesAndUnboundObservations(t *testing.T) {
	p := &fakePull{revision: "r"}
	d := &fakeDefinitions{value: infrastructurestate.Definition{ID: "d", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}}}
	s, _ := New(t.TempDir(), p, d)
	_, err := s.Create("repo", "pull", "actor", Input{Revision: "r", Definitions: []DefinitionRef{{ID: "d", Version: 1, ObservationIDs: []string{"missing"}}}, Changes: []Change{{ResourceID: "a", Action: "create", EnvironmentIDs: []string{"test"}, DependsOn: []string{"b"}, Summary: "a", RollbackLimit: "a"}, {ResourceID: "b", Action: "create", EnvironmentIDs: []string{"test"}, DependsOn: []string{"a"}, Summary: "b", RollbackLimit: "b"}}})
	if err != ErrInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestRehearsalRetainsBoundedEvidenceAndNamesUntestableEffects(t *testing.T) {
	now := time.Now().UTC()
	pulls := &fakePull{revision: "revision"}
	defs := &fakeDefinitions{value: infrastructurestate.Definition{ID: "inventory", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}}}
	s, _ := New(t.TempDir(), pulls, defs)
	s.now = func() time.Time { return now }
	risks := []Risk{}
	for _, kind := range []string{"availability", "security", "privacy", "continuity", "cost", "data"} {
		risks = append(risks, Risk{Kind: kind, Level: "medium", Detail: kind + " is bounded"})
	}
	p, err := s.Create("repo", "pull", "author", Input{Revision: "revision", Definitions: []DefinitionRef{{ID: "inventory", Version: 1}}, Changes: []Change{{ResourceID: "network", Action: "create", EnvironmentIDs: []string{"test"}, Summary: "create isolated network", RollbackLimit: "remove network", Risks: risks}, {ResourceID: "database", Action: "destroy", EnvironmentIDs: []string{"production"}, Summary: "retire old database", RollbackLimit: "data deletion is irreversible"}}, PolicyEffects: []PolicyEffect{{PolicyID: "policy", Revision: "1", Effect: "satisfy", Detail: "ephemeral only"}}, Assumptions: []string{"capacity exists"}, RollbackLimits: []string{"destruction cannot be tested"}})
	if err != nil {
		t.Fatal(err)
	}
	checks := []RehearsalCheck{}
	for kind := range checkKinds {
		checks = append(checks, RehearsalCheck{ID: kind, Kind: kind, Command: "checks/" + kind, Expected: "bounded result", ResourceIDs: []string{"network"}})
	}
	p, err = s.CreateRehearsal("repo", "pull", p.ID, "operator", RehearsalInput{Title: "candidate rehearsal", Environment: RehearsalEnvironment{ID: "ephemeral-42", Kind: "isolated", Regions: []string{"test-1"}, NetworkBoundary: "deny production routes"}, Credential: CredentialBoundary{Reference: "lease:42", Provider: "example", Scope: []string{"network:create", "network:delete"}, EnvironmentIDs: []string{"ephemeral-42"}, ExpiresAt: now.Add(time.Hour)}, State: StateBoundary{Kind: "synthetic", Reference: "fixture:42"}, Resources: []RehearsalResource{{ResourceID: "network", Support: "supported"}, {ResourceID: "database", Support: "untestable_destructive", Reason: "irreversible deletion is not simulated"}}, Checks: checks, MaximumDurationSeconds: 600, MaximumCost: 10, Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	r := p.Rehearsals[0]
	if r.Ready || len(r.UntestableEffects) != 1 || r.Credential.SecretRetained {
		t.Fatalf("unsafe rehearsal projection: %+v", r)
	}
	results := []CheckResult{}
	for _, c := range checks {
		results = append(results, CheckResult{CheckID: c.ID, Status: "passed", Summary: "passed", SanitizedLog: "credential redacted", ArtifactDigests: []string{"sha256:" + c.ID}, DurationMillis: 4})
	}
	p, err = s.RecordRehearsalAttempt("repo", "pull", p.ID, r.ID, "operator", AttemptInput{RunnerAttestation: "runner:isolated-42", StartedAt: now, CompletedAt: now.Add(time.Minute), Results: results, ResourceGraph: []ResourceGraphEdge{{From: "network", To: "service"}}, AgentActions: []AgentAction{{AgentID: "agent-1", Action: "diagnose", ResourceID: "network", Summary: "verified denied production route"}}, EstimatedCost: 2.5, TeardownStatus: "passed", TeardownAttestation: "provider reports zero retained ephemeral resources", RecoveryStatus: "not_applicable", RecoveryAttestation: "no authoritative state changed"})
	if err != nil {
		t.Fatal(err)
	}
	r = p.Rehearsals[0]
	if !r.Attempts[0].Passed || r.Ready || len(r.Blockers) != 1 || r.Blockers[0] != "untestable_destructive_effects" {
		t.Fatalf("attempt evidence misrepresented: %+v", r)
	}
	pulls.revision = "changed"
	p, _ = s.Get("repo", "pull", p.ID)
	if p.Rehearsals[0].Current || p.Rehearsals[0].Blockers[0] != "plan_stale" {
		t.Fatalf("stale evidence remained current: %+v", p.Rehearsals[0])
	}
}
