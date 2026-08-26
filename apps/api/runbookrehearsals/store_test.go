package runbookrehearsals

import (
	"testing"
	"time"
)

func sample() Input {
	return Input{
		RunbookID: "rb", RunbookVersion: 2, Title: "Dependency failure",
		EnvironmentID: "sandbox", EnvironmentRevision: "env-4", EnvironmentClass: "isolated",
		Limits: Limit{MaxDurationSeconds: 300, MaxCost: 5, Currency: "USD"}, OwnerIDs: []string{"operator"},
		Scenarios: []Scenario{{
			ID: "database", Name: "Database unavailable", Failure: "synthetic refused connections", EvidenceSource: "synthetic", InputDigest: "sha256:input",
			ExpectedOutcomes: []string{"traffic contained", "health restored"},
			References:       []BoundReference{{Kind: "service", ResourceID: "checkout", Revision: "service-v7"}, {Kind: "dependency", ResourceID: "database", Revision: "config-v3"}, {Kind: "runbook_step", ResourceID: "rollback", Revision: "step-v2"}},
		}},
	}
}
func attempt(rev int64) AttemptInput {
	start := time.Now().UTC()
	return AttemptInput{ExpectedRevision: rev, ScenarioID: "database", ActorKind: "agent", EnvironmentRevision: "env-4", StartedAt: start, EndedAt: start.Add(time.Minute), InputDigest: "sha256:input", Permissions: []Permission{{Capability: "telemetry:read", ResourceID: "sandbox", Granted: true, AuthorityReference: "policy:sandbox-v2"}}, Branches: []Branch{{StepID: "choose", Question: "rollback?", Decision: "yes", ActorID: "operator", Rationale: "synthetic health failed"}}, Steps: []StepResult{{StepID: "inspect", Status: "completed", Command: "approved:inspect@v2", Output: "connections refused", StartedAt: start, EndedAt: start.Add(10 * time.Second), ArtifactDigests: []string{"sha256:log"}}, {StepID: "rollback", Status: "completed", Command: "approved:rollback@v2 --simulate", Output: "simulation restored health", StartedAt: start.Add(10 * time.Second), EndedAt: start.Add(40 * time.Second), Destructive: true, DestructiveHandling: "simulated"}}, AchievedOutcomes: []string{"traffic contained", "health restored"}, Cost: 1.2, Currency: "USD"}
}

func TestProofAndSelectiveStaleness(t *testing.T) {
	s, _ := New(t.TempDir())
	r, e := s.Create("repo", "operator", sample())
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.AppendAttempt("repo", r.ID, "agent", attempt(1))
	if e != nil {
		t.Fatal(e)
	}
	resolved := Resolve(r)
	if !resolved.Ready || !resolved.Attempts[0].Proof {
		t.Fatalf("complete bounded attempt should prove readiness: %#v", resolved)
	}
	r, e = s.Observe("repo", r.ID, "operator", ObservationInput{ExpectedRevision: r.Revision, Kind: "dependency", ResourceID: "database", PreviousRevision: "config-v3", CurrentRevision: "config-v4", Detail: "failover policy changed"})
	if e != nil {
		t.Fatal(e)
	}
	resolved = Resolve(r)
	if resolved.Ready || !resolved.Attempts[0].Stale || resolved.Attempts[0].Proof {
		t.Fatalf("changed bound dependency should stale proof: %#v", resolved)
	}
}

func TestManualAndDestructiveGapsDoNotProve(t *testing.T) {
	s, _ := New(t.TempDir())
	r, _ := s.Create("repo", "operator", sample())
	a := attempt(1)
	a.ManualGaps = []string{"unknown database owner"}
	a.Steps[1].DestructiveHandling = "excluded"
	a.Steps[1].Status = "skipped"
	a.AchievedOutcomes = []string{"traffic contained"}
	r, e := s.AppendAttempt("repo", r.ID, "operator", a)
	if e != nil {
		t.Fatal(e)
	}
	x := Resolve(r)
	if x.Ready || x.Attempts[0].Proof || len(x.Attempts[0].Gaps) < 2 {
		t.Fatalf("manual and outcome gaps must remain visible: %#v", x)
	}
}
