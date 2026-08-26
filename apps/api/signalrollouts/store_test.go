package signalrollouts

import (
	"errors"
	"testing"
	"time"
)

func input() Input {
	return Input{ContractID: "contract-1", ContractVersion: 2, PullRequestID: "pull-1", ImplementationRunID: "run-1", DeployedRevision: "commit-1", CollectorRevision: "collector-2", ControllerID: "controller", OperatorIDs: []string{"operator"}, PrivacyControls: []string{"field allowlist", "regional routing"}, Stages: []Stage{{ID: "canary", Name: "EU canary", EnvironmentID: "production", EnvironmentRevision: "policy-4", ServiceIDs: []string{"checkout"}, Audiences: []string{"internal"}, Regions: []string{"eu-west"}, TrafficPercent: 5}, {ID: "regional", Name: "EU users", EnvironmentID: "production", EnvironmentRevision: "policy-4", ServiceIDs: []string{"checkout"}, Audiences: []string{"users"}, Regions: []string{"eu-west"}, TrafficPercent: 25}}, MaxCardinality: 1000, MaxStorageBytes: 1000000, MaxQueryCost: 10, Currency: "USD"}
}
func good(rev int64) ObservationInput {
	now := time.Now().UTC()
	return ObservationInput{ExpectedRevision: rev, StageID: "canary", WindowStart: now.Add(-time.Hour), WindowEnd: now, EvidenceIDs: []string{"evidence:window-1"}, SignalHealth: "healthy", CoveragePercent: 99, LatencyMS: 50, MissingPercent: 1, SamplingBiasPercent: 2, Cardinality: 500, StorageBytes: 500000, QueryCost: 5, PipelineLossPercent: 1, CollectorStatus: "healthy", ServiceStatus: "healthy"}
}

func TestProgressiveRolloutContainsAndRecovers(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Create("repo", "operator", input())
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "paused" || len(r.Findings) != 1 {
		t.Fatalf("missing proof must pause: %#v", r)
	}
	o := good(r.Revision)
	r, e = s.Observe("repo", r.ID, "human", "operator", o)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Findings) != 0 {
		t.Fatalf("passing proof findings: %#v", r.Findings)
	}
	r, e = s.Control("repo", r.ID, "human", "operator", ControlInput{ExpectedRevision: r.Revision, Action: "resume", StageID: "canary", Rationale: "reviewed production window passes", EvidenceIDs: o.EvidenceIDs})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "active" || r.EffectiveTrafficPercent != 5 {
		t.Fatalf("resume: %#v", r)
	}
	bad := good(r.Revision)
	bad.UnexpectedSensitiveData = true
	bad.QueryCost = 20
	r, e = s.Observe("repo", r.ID, "human", "operator", bad)
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "rolled_back" || r.EffectiveTrafficPercent != 0 {
		t.Fatalf("privacy breach must rollback: %#v", r)
	}
	if len(r.Observations) != 2 || len(r.Findings) < 2 {
		t.Fatalf("evidence was silently lost: %#v", r)
	}
	_, e = s.Control("repo", r.ID, "human", "operator", ControlInput{ExpectedRevision: r.Revision, Action: "resume", StageID: "canary", Rationale: "ignore it", EvidenceIDs: []string{"bad"}})
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("unsafe resume = %v", e)
	}
}

func TestOnlyNamedHumansCanOperate(t *testing.T) {
	s, _ := New(t.TempDir())
	_, e := s.Create("repo", "outsider", input())
	if !errors.Is(e, ErrForbidden) {
		t.Fatalf("create = %v", e)
	}
	r, e := s.Create("repo", "operator", input())
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Observe("repo", r.ID, "agent", "operator", good(r.Revision))
	if !errors.Is(e, ErrInvalid) {
		t.Fatalf("agent observation = %v", e)
	}
	_, e = s.Control("repo", r.ID, "human", "outsider", ControlInput{ExpectedRevision: r.Revision, Action: "pause", StageID: "canary", Rationale: "review", EvidenceIDs: []string{"e"}})
	if !errors.Is(e, ErrForbidden) {
		t.Fatalf("outsider control = %v", e)
	}
}
