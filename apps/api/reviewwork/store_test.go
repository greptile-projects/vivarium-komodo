package reviewwork

import (
	"errors"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
)

func TestParallelReviewRetainsCoverageConflictAndAgentBoundary(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC) }
	plan := reviewplans.Version{Number: 1, Revision: "candidate", Input: reviewplans.Input{Areas: []reviewplans.Area{{ID: "security", Paths: []string{"auth.go"}, Questions: []string{"Are sessions revoked?"}, Evidence: []reviewplans.Evidence{{Kind: "check", Description: "session check", Reference: "check-1"}}}}}}
	routing := reviewrouting.Routing{PlanVersion: 1, Revision: "candidate", Assignments: []reviewrouting.Assignment{{ID: "human-work", AreaID: "security", ParticipantID: "reviewer", Kind: "human", State: "accepted", PlanVersion: 1, Revision: "candidate"}, {ID: "agent-work", AreaID: "security", ParticipantID: "agent", Kind: "agent", State: "accepted", PlanVersion: 1, Revision: "candidate"}}}
	x, err := s.Open("repo", "pull", plan, routing)
	if err != nil {
		t.Fatal(err)
	}
	fileID := "security:file:auth.go"
	x, err = s.RecordProgress("repo", "pull", "reviewer", routing, "human-work", "investigating", []string{fileID}, []string{"auth.go:session"}, nil, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.RecordProgress("repo", "pull", "agent", routing, "agent-work", "investigating", []string{fileID}, []string{"auth.go:logout"}, []string{"runtime behavior unobserved"}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Conflicts) != 1 {
		t.Fatalf("overlap not visible: %#v", x)
	}
	citation := Citation{Kind: "diff", Reference: "auth.go#L20", Revision: "candidate", Summary: "revocation removed", Accessible: true, Audience: "repository"}
	x, err = s.AddFinding("repo", "pull", "agent", routing, "agent-work", "revocation may be skipped", "high", "concern", []Citation{citation}, []string{"preview unavailable"}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	if x.Findings[0].Status != "proposed_by_agent" || x.Findings[0].Uncertainty[0] != "preview unavailable" {
		t.Fatalf("agent boundary lost: %#v", x.Findings[0])
	}
	private := citation
	private.Audience = "embargoed"
	if _, err = s.AddFinding("repo", "pull", "agent", routing, "agent-work", "private claim", "high", "concern", []Citation{private}, nil, x.Version); !errors.Is(err, ErrInvalid) {
		t.Fatalf("private evidence propagated: %v", err)
	}
	x, err = s.AddMessage("repo", "pull", "reviewer", "security", "challenge", "The cited branch is unreachable.", routing, "human-work", []string{x.Findings[0].ID}, []Citation{citation}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Handoff("repo", "pull", "agent", routing, "agent-work", "human-work", "human must verify the proposed finding", []string{fileID}, []string{x.Findings[0].ID}, []string{"preview unavailable"}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.AcceptHandoff("repo", "pull", "reviewer", x.Handoffs[0].ID, routing, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	if x.Handoffs[0].State != "accepted" || len(x.Discussion) != 1 {
		t.Fatalf("shared reasoning missing: %#v", x)
	}
}

func TestWorkspaceRejectsRevisionDrift(t *testing.T) {
	s, _ := New(t.TempDir())
	p := reviewplans.Version{Number: 1, Revision: "one", Input: reviewplans.Input{Areas: []reviewplans.Area{{ID: "code", Paths: []string{"a"}}}}}
	r := reviewrouting.Routing{PlanVersion: 1, Revision: "one"}
	if _, e := s.Open("r", "p", p, r); e != nil {
		t.Fatal(e)
	}
	p.Number = 2
	p.Revision = "two"
	if _, e := s.Open("r", "p", p, r); !errors.Is(e, ErrConflict) {
		t.Fatalf("drift accepted: %v", e)
	}
}
