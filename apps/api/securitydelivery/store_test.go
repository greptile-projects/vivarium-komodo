package securitydelivery

import (
	"testing"
	"time"
)

func TestExactAssessmentAndAffectedExceptions(t *testing.T) {
	s, _ := New(t.TempDir())
	p, e := s.CreatePolicy("repository:r", "owner", PolicyInput{Name: "privileged API", ScopeKind: "repository", ScopeID: "r", Branches: []string{"main"}, Components: []string{"api/auth"}, Assets: []string{"tokens"}, RiskClasses: []string{"critical"}, RequiredThreatModels: []string{"tm"}, RequiredScenarios: []string{"sc"}, RequiredControlOwnerIDs: []string{"security"}, RequireResolvedFindings: true})
	if e != nil {
		t.Fatal(e)
	}
	ev := []Evidence{{ThreatModelID: "tm", ThreatModelRevision: "v2", Current: true, ResidualRisk: "bounded replay window", ScenarioID: "sc", ScenarioVersion: 2, AttemptID: "a", AttemptRevision: "rev", AttemptStatus: "passed", Coverage: []string{"containment", "detection", "recovery"}}}
	a, _ := s.Assess([]string{"repository:r"}, "pull_request", "pr", "rev", "main", []string{"api/auth/login.go"}, []string{"tokens"}, []string{"critical"}, ev)
	if a.Ready || len(a.Requirements) != 1 {
		t.Fatalf("missing owner acknowledgement was not isolated: %#v", a)
	}
	if _, e = s.Acknowledge("repository:r", p.ID, "pull_request", "pr", "rev", "security", "accept", "controls match"); e != nil {
		t.Fatal(e)
	}
	a, _ = s.Assess([]string{"repository:r"}, "pull_request", "pr", "rev", "main", []string{"api/auth/login.go"}, []string{"tokens"}, []string{"critical"}, ev)
	if !a.Ready || len(a.ResidualRisk) != 1 {
		t.Fatalf("current evidence not ready: %#v", a)
	}
	stale := ev
	stale[0].AttemptRevision = "old"
	a, _ = s.Assess([]string{"repository:r"}, "pull_request", "pr", "rev", "main", []string{"api/auth/login.go"}, []string{"tokens"}, []string{"critical"}, stale)
	if a.Ready || a.Requirements[0].Kind != "scenario_result" {
		t.Fatalf("revision drift not detected: %#v", a)
	}
	_, _ = s.Except("repository:r", p.ID, "pull_request", "pr", "rev", "owner", "bounded rollout", []string{"scenario_result"}, time.Now().Add(time.Hour))
	a, _ = s.Assess([]string{"repository:r"}, "pull_request", "pr", "rev", "main", []string{"api/auth/login.go"}, []string{"tokens"}, []string{"critical"}, stale)
	if !a.Ready || a.Requirements[0].Blocking {
		t.Fatalf("scoped exception not applied: %#v", a)
	}
}
func TestSanitizedSignalOpensAttributedResponse(t *testing.T) {
	s, _ := New(t.TempDir())
	in := SignalInput{DeploymentID: "d", ReleaseID: "rel", Revision: "rev", Environment: "prod", Assumption: "issuer remains isolated", ControlID: "c", Outcome: "control_failed", Summary: "aggregate rejection rate crossed threshold", InputKeys: []string{"control:c"}, ObservedAt: time.Now()}
	if _, e := s.RecordSignal("repository:r", "monitor", in, false); e == nil {
		t.Fatal("unsanitized signal accepted")
	}
	x, e := s.RecordSignal("repository:r", "monitor", in, true)
	if e != nil || !x.Violated {
		t.Fatal(e)
	}
	x, e = s.OpenResponse("repository:r", x.ID, "owner", "private_incident", "inc-1")
	if e != nil || x.Response.OpenedByID != "owner" {
		t.Fatalf("response missing: %#v %v", x, e)
	}
}
