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

func TestConfirmedFindingDeliveryVerificationAndExplicitResolution(t *testing.T) {
	s, _ := New(t.TempDir())
	x, _ := s.Create("repo", "lead", input(time.Now().UTC()))
	x, _ = s.Append("repo", x.ID, "agent-1", x.Revision, EventInput{Kind: "observation", CharterID: "retry", Route: "/checkout", BehaviorIDs: []string{"one-order"}, Observation: "two orders", AgentAction: true})
	eid := x.Events[0].ID
	x, _ = s.AddFinding("repo", x.ID, "agent-1", x.Revision, FindingInput{CharterID: "retry", Title: "duplicate", Description: "two orders", EventIDs: []string{eid}, ReproductionSteps: []string{"retry"}})
	fid := x.Findings[0].ID
	x, _ = s.UpdateFinding("repo", x.ID, fid, "lead", x.Revision, FindingUpdate{Classification: "defect", Reproduction: "reproduced", Rationale: "stable"})
	x, err := s.LinkDelivery("repo", x.ID, fid, "lead", x.Revision, DeliveryLinkInput{IssueID: "issue", ProposalID: "proposal", TaskID: "task", OwnerKind: "agent", OwnerID: "agent-1", AcceptanceCriteria: []string{"one order"}, PermittedEventIDs: []string{eid}, MinimizedReproduction: []string{"retry once"}})
	if err != nil || x.Findings[0].Delivery.BaseRevision != "commit-a" {
		t.Fatalf("delivery = %#v, %v", x, err)
	}
	if _, err = s.VerifyDelivery("repo", x.ID, fid, "lead", x.Revision, VerificationInput{PullRequestID: "pull", BaseRevision: "wrong", RepairRevision: "fixed", FailingEvidenceID: "fail", PassingEvidenceID: "pass", ReviewID: "review", QualityPlanID: "quality", QualityPlanVersion: 1, ScenarioID: "scenario", ScenarioVersion: 1}); !errors.Is(err, ErrConflict) {
		t.Fatalf("wrong base = %v", err)
	}
	x, err = s.VerifyDelivery("repo", x.ID, fid, "lead", x.Revision, VerificationInput{PullRequestID: "pull", BaseRevision: "commit-a", RepairRevision: "commit-b", FailingEvidenceID: "base-run", PassingEvidenceID: "repair-run", ReviewID: "review", QualityPlanID: "quality", QualityPlanVersion: 2, ScenarioID: "scenario", ScenarioVersion: 1})
	if err != nil || x.Findings[0].Status != "resolved" || x.Findings[0].Delivery.ScenarioID != "scenario" {
		t.Fatalf("verification = %#v, %v", x, err)
	}

	y, _ := s.Create("repo", "lead", input(time.Now().UTC()))
	y, _ = s.Append("repo", y.ID, "agent-1", y.Revision, EventInput{Kind: "observation", CharterID: "retry", Route: "/checkout", Observation: "timing varied", AgentAction: true})
	y, _ = s.AddFinding("repo", y.ID, "agent-1", y.Revision, FindingInput{CharterID: "retry", Title: "timing", Description: "varied", EventIDs: []string{y.Events[0].ID}, ReproductionSteps: []string{"repeat"}})
	y, err = s.ResolveWithoutDelivery("repo", y.ID, y.Findings[0].ID, "lead", y.Revision, ResolutionInput{Kind: "flaky", Rationale: "one in ten", FollowUp: "quarantine with expiry and rerun budget"})
	if err != nil || y.Findings[0].Resolution.Kind != "flaky" {
		t.Fatalf("resolution = %#v, %v", y, err)
	}
}
