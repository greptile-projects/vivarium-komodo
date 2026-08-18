package runtimereplays

import "testing"

func scenario(t *testing.T, s *Store) Scenario {
	t.Helper()
	v, e := s.Create("repo", "debug", "alice", CreateInput{Revision: "abc", Name: "intermittent retry", Behavior: "second request loses state", Audience: "participants", ParticipantIDs: []string{"bob"}, EvidenceIDs: []string{"capture"}, StateKind: "synthetic", Inputs: []Input{{Name: "request.json", Kind: "synthetic", Value: `{"sequence":2}`, SourceEvidenceID: "capture", Transformation: "identifiers replaced and timing bucketed"}}, Commands: []string{"bun test replay.test.ts"}, Invariants: []Invariant{{Name: "retry-loses-state", Expectation: "response is 409"}}})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestReplayRequiresRepeatedUnblockedExactRevisionEvidence(t *testing.T) {
	s, _ := New(t.TempDir())
	v := scenario(t, s)
	in := AttemptInput{TargetKind: "workspace", TargetID: "isolated", Revision: "abc", Environment: map[string]string{"network": "disabled", "state": "synthetic"}, Commands: []string{"bun test replay.test.ts"}, Outputs: []string{"409"}, InvariantResults: map[string]bool{"retry-loses-state": true}, Cost: 0.04, ProductionDifferences: []string{"synthetic account"}}
	v, e := s.Attempt("repo", v.ID, "alice", in)
	if e != nil || v.Reproduced || v.RepeatedPassingAttempts != 1 {
		t.Fatalf("first = %#v %v", v, e)
	}
	v, e = s.Attempt("repo", v.ID, "bob", in)
	if e != nil || !v.Reproduced || v.Status != "reproduced" || v.RepeatedPassingAttempts != 2 {
		t.Fatalf("second = %#v %v", v, e)
	}
}
func TestReplayKeepsUnsafeStaleAndIrreducibleConditionsExplicit(t *testing.T) {
	s, _ := New(t.TempDir())
	v := scenario(t, s)
	v, e := s.Attempt("repo", v.ID, "alice", AttemptInput{TargetKind: "preview", TargetID: "p", Revision: "changed", Environment: map[string]string{"state": "synthetic"}, Commands: []string{"bun test replay.test.ts"}, InvariantResults: map[string]bool{"retry-loses-state": true}, Blockers: []string{"nondeterminism", "missing_dependency", "irreducible_production_condition"}})
	if e != nil {
		t.Fatal(e)
	}
	if v.Reproduced || v.Status != "blocked" || len(v.Blockers) != 4 {
		t.Fatalf("blocked = %#v", v)
	}
}
func TestReplayRejectsProtectedFixtureAndAllowsAttributableRefinement(t *testing.T) {
	s, _ := New(t.TempDir())
	_, e := s.Create("repo", "debug", "alice", CreateInput{Revision: "abc", Name: "bad", Behavior: "bad", Audience: "repository", EvidenceIDs: []string{"capture"}, StateKind: "privacy_preserving", Inputs: []Input{{Name: "fixture", Kind: "redacted", Value: "Authorization: Bearer live", SourceEvidenceID: "capture", Transformation: "none"}}, Commands: []string{"test"}, Invariants: []Invariant{{Name: "x", Expectation: "y"}}})
	if e != ErrInvalid {
		t.Fatalf("secret = %v", e)
	}
	v := scenario(t, s)
	v, e = s.Refine("repo", v.ID, "bob", "Minimized the timing buckets from six to two without copying captured values.")
	if e != nil || len(v.Refinements) != 1 || v.Refinements[0].ActorID != "bob" {
		t.Fatalf("refinement = %#v %v", v, e)
	}
}

func TestRepairVerificationCanReplayAgainstCandidateRevision(t *testing.T) {
	s, _ := New(t.TempDir())
	v := scenario(t, s)
	v, e := s.Attempt("repo", v.ID, "alice", AttemptInput{Mode: "repair_verification", TargetKind: "workspace", TargetID: "candidate", Revision: "fixed", Environment: map[string]string{"network": "disabled"}, Commands: []string{"go test ./..."}, InvariantResults: map[string]bool{"retry-loses-state": false}, Cost: .02})
	if e != nil || v.Attempts[0].Status != "not_reproduced" || len(v.Attempts[0].Blockers) != 0 || v.Attempts[0].Mode != "repair_verification" {
		t.Fatalf("candidate replay = %#v, %v", v, e)
	}
}
