package debuggingworkspaces

import (
	"testing"
	"time"
)

func TestWorkspaceRetainsExactContextGapsAudienceAndAttribution(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	v, err := s.Create("repo", "responder", CreateInput{
		Title: "Intermittent checkout failure", Origin: Origin{Kind: "incident", ResourceID: "inc-7", Revision: "event-3", Summary: "Checkout sometimes returns 502"}, ReleaseID: "release-12", ReleaseRevision: "commit-a", Environment: "production", Window: TimeWindow{Start: start, End: start.Add(time.Hour)}, UserJourney: "checkout", OwnerIDs: []string{"service-owner"}, Severity: "high", SourceRevision: "commit-a",
		Bindings:          []Binding{{Kind: "package", ResourceID: "api", Revision: "pkg-4", Status: "available"}, {Kind: "configuration", ResourceID: "checkout-config", Revision: "config-8", Status: "available"}, {Kind: "infrastructure", Status: "unavailable", Reason: "provider observation is restricted"}},
		PermittedEvidence: []EvidencePermission{{Kind: "trace", ResourceID: "trace-redacted", Audience: "participants", Access: "permitted"}, {Kind: "logs", Audience: "repository", Access: "unavailable", Reason: "collection has not been approved"}}, Audience: "participants", ParticipantIDs: []string{"service-owner"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.UnavailableContext) != 1 || len(v.Authority) != 0 || len(v.History) != 1 {
		t.Fatalf("missing safe derived context: %+v", v)
	}
	if _, err = s.AddHypothesis("repo", v.ID, "stranger", Hypothesis{Summary: "cache race", Status: "proposed"}); err != ErrInvalid {
		t.Fatalf("non-participant wrote hypothesis: %v", err)
	}
	v, err = s.AddHypothesis("repo", v.ID, "service-owner", Hypothesis{Summary: "cache race after configuration reload", Status: "proposed", Uncertainty: "trace is sampled"})
	if err != nil || len(v.Hypotheses) != 1 || v.Hypotheses[0].ActorID != "service-owner" || len(v.History) != 2 {
		t.Fatalf("hypothesis attribution missing: %+v %v", v, err)
	}
	got, err := s.Get("repo", v.ID)
	if err != nil || got.SourceRevision != "commit-a" || got.Bindings[0].Revision != "pkg-4" {
		t.Fatalf("durable revision context missing: %+v %v", got, err)
	}
}

func TestWorkspaceRequiresEveryRevisionBoundaryOrExplicitGap(t *testing.T) {
	s, _ := New(t.TempDir())
	start := time.Now().UTC()
	_, err := s.Create("repo", "owner", CreateInput{Title: "failure", Origin: Origin{Kind: "trace", ResourceID: "t", Summary: "failure"}, ReleaseID: "r", ReleaseRevision: "c", Environment: "prod", Window: TimeWindow{Start: start, End: start.Add(time.Minute)}, UserJourney: "login", OwnerIDs: []string{"owner"}, Severity: "critical", SourceRevision: "c", Bindings: []Binding{{Kind: "package", ResourceID: "p", Revision: "1", Status: "available"}, {Kind: "configuration", Status: "unavailable", Reason: "denied"}}, PermittedEvidence: []EvidencePermission{{Kind: "trace", Audience: "repository", Access: "permitted"}}, Audience: "repository"})
	if err != ErrInvalid {
		t.Fatalf("missing infrastructure boundary accepted: %v", err)
	}
}
