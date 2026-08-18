package infrastructureplans

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

type fakePull struct{ revision string }

func (f *fakePull) CurrentRevision(string, string) (string, error) { return f.revision, nil }

type fakeDefinitions struct {
	value infrastructurestate.Definition
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
