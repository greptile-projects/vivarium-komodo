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

func TestFindingDecisionWorkVerificationAndRevisionTransition(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 25, 21, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p := reviewplans.Version{Number: 1, Revision: "base", Input: reviewplans.Input{Areas: []reviewplans.Area{{ID: "code", Paths: []string{"a.go"}}}}}
	r := reviewrouting.Routing{PlanVersion: 1, Revision: "base", Assignments: []reviewrouting.Assignment{{ID: "review", AreaID: "code", ParticipantID: "reviewer", Kind: "human", State: "accepted", PlanVersion: 1, Revision: "base"}}}
	x, _ := s.Open("repo", "pull", p, r)
	c := Citation{Kind: "diff", Reference: "a.go#L1", Revision: "base", Summary: "unsafe default", Accessible: true, Audience: "repository"}
	x, _ = s.AddFinding("repo", "pull", "reviewer", r, "review", "unsafe default", "high", "concern", []Citation{c}, nil, x.Version)
	finding := x.Findings[0].ID
	x, err := s.Decide("repo", "pull", "owner", finding, "accepted", "repair this candidate", "reviewer still requests a regression", "", true, nil, nil, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.LinkWork("repo", "pull", "contributor", finding, "commit", "commit:repair", "repair", "replace the unsafe default", x.Version)
	if err != nil {
		t.Fatal(err)
	}
	p = reviewplans.Version{Number: 2, Revision: "repair"}
	x, err = s.Transition("repo", "pull", "contributor", p, []FindingApplicability{{FindingID: finding, State: "addressed", Reason: "commit:repair changes the cited path"}}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	proof := Citation{Kind: "check", Reference: "check:safe-default", Revision: "repair", Summary: "targeted regression passed", Accessible: true, Audience: "repository"}
	x, err = s.Verify("repo", "pull", "reviewer", finding, "reproduction", "scenario:safe-default", "base", "repair", "passed", "fails on base and is contained on repair", []Citation{proof}, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	if x.Transitions[0].Findings[0].State != "addressed" || x.Verifications[0].Outcome != "passed" || x.Decisions[0].Dissent == "" || x.WorkLinks[0].Kind != "commit" {
		t.Fatalf("resolution trail incomplete: %#v", x)
	}

	expires := now.Add(time.Hour)
	if _, err = s.Decide("repo", "pull", "contributor", finding, "exception", "temporarily retain behavior", "", "", false, []string{"linux"}, &expires, x.Version); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-owner exception accepted: %v", err)
	}
}
