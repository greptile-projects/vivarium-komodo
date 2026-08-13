package productopportunities

import (
	"errors"
	"testing"
)

func sample() Input {
	return Input{Title: "Review friction recurs", Need: "Collaborators cannot tell why reviews wait", AffectedAudiences: []string{"first-time contributors", "maintainers"}, Severity: "high", Reach: "some", Confidence: "medium", ExpectedValue: "Shorter, more predictable review loops", Uncertainty: []string{"support evidence has an unknown sampling frame"}, Sources: []Source{{Kind: "feedback", ResourceID: "feedback-1", CapturedRevision: "11", Relevance: "A contributor describes a three-day wait", Position: "supporting"}, {Kind: "issue", ResourceID: "issue-1", CapturedRevision: "2", Relevance: "A maintainer reports a different root cause", Position: "contradicting"}, {Kind: "support_signal", ResourceID: "support-q3", CapturedRevision: "sha256:abc", Relevance: "One accessibility need occurs only once but has high impact", Position: "minority"}, {Kind: "usage_evidence", ResourceID: "query-7", CapturedRevision: "sha256:def", Relevance: "Repeated abandoned review visits", Position: "duplicate"}}, ChangeReason: "Initial evidence-backed synthesis"}
}

func TestVersionsPreserveContradictionsChallengesAndDetachment(t *testing.T) {
	s, _ := New(t.TempDir())
	v, e := s.Create("repo", "human", sample())
	if e != nil {
		t.Fatal(e)
	}
	in := sample()
	in.Confidence = "high"
	in.ChangeReason = "Classified additional evidence"
	v, e = s.Revise("repo", v.ID, "agent:reader", 1, in)
	if e != nil || v.CurrentVersion != 2 || len(v.Versions) != 2 {
		t.Fatalf("revise: %+v %v", v, e)
	}
	v, e = s.Note("repo", v.ID, "participant", "challenge", "issue", "issue-1", "This issue affects only archived releases")
	if e != nil || len(v.Notes) != 1 {
		t.Fatalf("challenge: %+v %v", v, e)
	}
	v, e = s.DetachFeedback("repo", v.ID, "feedback-1", "reporter")
	if e != nil || v.CurrentVersion != 3 || !v.Versions[2].Sources[0].Detached || v.Versions[1].Sources[0].Detached {
		t.Fatalf("detachment must append without rewriting history: %+v %v", v, e)
	}
	if v.OperationalAuthority {
		t.Fatal("synthesis granted authority")
	}
	_, e = s.Revise("repo", v.ID, "human", 1, in)
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("expected stale conflict: %v", e)
	}
}

func TestRequiresInspectableClassification(t *testing.T) {
	s, _ := New(t.TempDir())
	in := sample()
	in.Sources[0].Relevance = ""
	if _, e := s.Create("repo", "human", in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("opaque source accepted: %v", e)
	}
}
