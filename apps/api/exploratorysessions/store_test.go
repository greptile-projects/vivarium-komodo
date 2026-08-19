package exploratorysessions

import (
	"errors"
	"testing"
	"time"
)

func input(now time.Time) Input {
	return Input{Title: "Explore checkout retry", OriginKind: "pull_request_preview", OriginReference: "preview-1", Candidate: Candidate{Kind: "pull_request", Reference: "pull-7", Revision: "commit-a"}, QualityPlanID: "quality-1", Access: Access{ExpiresAt: now.Add(time.Hour), Environment: "preview-1", Network: "preview", AllowedRoutes: []string{"/checkout"}, AllowedCommands: []string{"bun test"}}, TestData: TestData{Description: "Generated shoppers", PrivacyClassification: "internal", Synthetic: true}, Budget: Budget{MaxMinutes: 60, MaxCost: 5, MaxAgentActions: 2}, Participants: []Participant{{ID: "lead", Kind: "human", Approved: true, Role: "lead"}, {ID: "agent-1", Kind: "agent", Approved: true, Role: "tester"}}, Charters: []Charter{{ID: "retry", Title: "Retry boundaries", Risk: "duplicate charge", RiskLevel: "critical", Mission: "Interrupt checkout at each boundary", OwnerID: "agent-1", Routes: []string{"/checkout"}, BehaviorIDs: []string{"one-order"}}}, Uncertainty: "Provider timing differs"}
}

func TestSessionTimelineScopeFindingsAndStaleness(t *testing.T) {
	now := time.Now().UTC()
	s, _ := New(t.TempDir())
	x, err := s.Create("repo", "lead", input(now))
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Append("repo", x.ID, "agent-1", x.Revision, EventInput{Kind: "observation", CharterID: "retry", Route: "/checkout/retry", BehaviorIDs: []string{"one-order"}, Inputs: []string{"synthetic decline"}, Observation: "second order appeared", Uncertainty: "one run", Artifacts: []Artifact{{Kind: "screenshot", Reference: "artifact:shot", SHA256: "abc", MediaType: "image/png", Sanitized: true}}, Cost: 1, AgentAction: true})
	if err != nil {
		t.Fatal(err)
	}
	eventID := x.Events[0].ID
	x, err = s.AddFinding("repo", x.ID, "agent-1", x.Revision, FindingInput{CharterID: "retry", Title: "Retry duplicates order", Description: "Two orders are created", EventIDs: []string{eventID}, ReproductionSteps: []string{"decline", "retry"}, Uncertainty: "timing sensitive"})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.UpdateFinding("repo", x.ID, x.Findings[0].ID, "lead", x.Revision, FindingUpdate{Status: "classified", Classification: "defect", Reproduction: "reproduced", Rationale: "repeated twice"})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.UpdateCandidate("repo", x.ID, "lead", x.Revision, CandidateUpdate{Revision: "commit-b", AffectedRoutes: []string{"/checkout"}})
	if err != nil {
		t.Fatal(err)
	}
	if !x.Events[0].Stale || !x.Findings[0].Stale {
		t.Fatalf("affected evidence not stale: %#v", x)
	}
}

func TestSessionContainsAgentAndBudgetAuthority(t *testing.T) {
	now := time.Now().UTC()
	s, _ := New(t.TempDir())
	in := input(now)
	in.Participants[1].Approved = false
	if _, err := s.Create("repo", "lead", in); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unapproved agent = %v", err)
	}
	in = input(now)
	x, _ := s.Create("repo", "lead", in)
	_, err := s.Append("repo", x.ID, "agent-1", x.Revision, EventInput{Kind: "command", CharterID: "retry", Route: "/checkout", Command: "curl production", AgentAction: true})
	if !errors.Is(err, ErrScope) {
		t.Fatalf("out-of-scope command = %v", err)
	}
	x, _ = s.Control("repo", x.ID, "lead", x.Revision, ControlInput{Action: "pause", Guidance: "inspect first trace"})
	if _, err = s.Append("repo", x.ID, "agent-1", x.Revision, EventInput{Kind: "note", CharterID: "retry", AgentAction: true}); !errors.Is(err, ErrConflict) {
		t.Fatalf("paused append = %v", err)
	}
}
