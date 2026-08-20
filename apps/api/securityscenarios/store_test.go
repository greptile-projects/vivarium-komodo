package securityscenarios

import "testing"

func scenarioInput() Input {
	return Input{Name: "Callback cannot reach metadata", ThreatModelID: "tm", ThreatModelRevision: "design-v2", AbusePathID: "ssrf", SourceRevision: "candidate", DefinitionPath: ".komodo/security-checks.json", AttackerPreconditions: []string{"attacker can configure callback"}, Capabilities: []Capability{{Name: "send callback URL", Boundary: "synthetic HTTP fixture only"}}, Fixtures: []Fixture{{ID: "metadata", Description: "fake metadata service", Generator: "go run fixtures/metadata.go", Synthetic: true}}, Actions: []Action{{ID: "request", Description: "submit link-local callback", Command: "go test ./security -run TestSSRF"}}, Containment: []Criterion{{ID: "deny", Description: "request is denied", Observable: "HTTP 422"}}, Detection: []Criterion{{ID: "audit", Description: "attempt is audited", Observable: "ssrf_denied event"}}, Recovery: []Criterion{{ID: "healthy", Description: "worker remains healthy", Observable: "health check passes"}}, OwnerIDs: []string{"security-owner"}, ChangeReason: "make agreed path reproducible"}
}

func TestReviewedScenarioRetainsSafeAndUnsafeEvidence(t *testing.T) {
	s, _ := New(t.TempDir())
	x, e := s.Create("repo", "agent", scenarioInput())
	if e != nil || x.Approved {
		t.Fatalf("create: %#v %v", x, e)
	}
	if _, e = s.Review("repo", x.ID, "agent", "approve", "looks good", 1); e != ErrInvalid {
		t.Fatal("non-owner reviewed scenario")
	}
	x, e = s.Review("repo", x.ID, "security-owner", "approve", "bounded and representative", 1)
	if e != nil || !x.Approved {
		t.Fatalf("review: %#v %v", x, e)
	}
	good := AttemptInput{ScenarioVersion: 1, TargetKind: "workspace", PullRequestID: "pull", Revision: "candidate", Isolation: "ephemeral", Network: "none", Status: "passed", Commands: []string{"go test ./security -run TestSSRF"}, Logs: []string{"request denied [redacted]"}, Traces: []string{"submit -> policy -> deny"}, Artifacts: []Artifact{{Name: "trace", Digest: "sha256:abc", MediaType: "application/json", Size: 10, Sanitized: true}}, Coverage: Coverage{ContainmentIDs: []string{"deny"}, DetectionIDs: []string{"audit"}, RecoveryIDs: []string{"healthy"}}, Cost: 0.02, Currency: "USD", Provenance: []string{"pull@candidate", "security-checks.json@blob"}}
	x, e = s.AddAttempt("repo", x.ID, "scoped-agent", good)
	if e != nil || len(x.Attempts) != 1 || !x.Attempts[0].Current {
		t.Fatalf("safe attempt: %#v %v", x, e)
	}
	unsafe := good
	unsafe.Status = "unsafe"
	unsafe.DestructiveEffects = true
	unsafe.Blockers = []string{"destructive effect cannot be isolated"}
	if _, e = s.AddAttempt("repo", x.ID, "scoped-agent", unsafe); e != nil {
		t.Fatalf("unsafe result should remain explicit: %v", e)
	}
	unsafe.Status = "passed"
	if _, e = s.AddAttempt("repo", x.ID, "scoped-agent", unsafe); e != ErrInvalid {
		t.Fatal("unsafe attempt reported passing")
	}
}

func TestScenarioRejectsSensitiveFixtureAndUnreportedDependency(t *testing.T) {
	s, _ := New(t.TempDir())
	in := scenarioInput()
	in.Fixtures[0].ContainsProductionData = true
	if _, e := s.Create("repo", "owner", in); e != ErrInvalid {
		t.Fatal("production fixture accepted")
	}
	in = scenarioInput()
	x, _ := s.Create("repo", "owner", in)
	a := AttemptInput{ScenarioVersion: 1, TargetKind: "workspace", PullRequestID: "pull", Revision: "candidate", Isolation: "ephemeral", Network: "none", Status: "blocked", Currency: "USD", Provenance: []string{"candidate"}, InaccessibleDependencies: []string{"provider"}}
	if _, e := s.AddAttempt("repo", x.ID, "owner", a); e != ErrInvalid {
		t.Fatal("silent inaccessible dependency accepted")
	}
	a.Blockers = []string{"provider unavailable"}
	if _, e := s.AddAttempt("repo", x.ID, "owner", a); e != nil {
		t.Fatalf("explicit blocker rejected: %v", e)
	}
}
