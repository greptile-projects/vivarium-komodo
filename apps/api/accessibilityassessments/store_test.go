package accessibilityassessments

import "testing"

func TestAssessmentSeparatesEvidenceAndInvalidatesOnlyAffectedSources(t *testing.T) {
	s, _ := New(t.TempDir())
	a, err := s.Create("repo", "pull", "owner", Input{Revision: "one", Scenarios: []Scenario{{ID: "checkout", Name: "Complete checkout", Journey: "Open cart, review total, and pay", Audiences: []string{"keyboard users", "screen reader users"}, Evaluations: []string{"semantics", "keyboard", "contrast"}, Locations: []Location{{Path: "site/cart.html", BlobID: "cart-one"}, {Path: "site/theme.css", BlobID: "theme-one"}}, Digest: "scenario-one"}}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.AddAutomation("repo", "pull", a.ID, "runner", Automation{RunID: "run", Name: "axe-checkout", ScenarioIDs: []string{"checkout"}, Evaluations: []string{"semantics", "contrast"}, Status: "succeeded", RequiresHumanEvaluation: []string{"keyboard"}, Inputs: []Location{{Path: "site/cart.html", BlobID: "cart-one"}}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.AddFinding("repo", "pull", a.ID, "specialist", FindingInput{ScenarioID: "checkout", Evaluation: "keyboard", Result: "barrier", Severity: "high", Audiences: []string{"keyboard users"}, Locations: []Location{{Path: "site/cart.html", BlobID: "cart-one"}}, Summary: "Focus returns to the page start", Uncertainty: "Observed in Chromium only", RequiresHumanEvaluation: true, Citation: Citation{Kind: "preview", ResourceID: "preview"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Decide("repo", "pull", a.ID, a.Findings[0].ID, "owner", DecisionInput{Outcome: "confirmed", Rationale: "Reproduced with keyboard-only navigation"})
	if err != nil {
		t.Fatal(err)
	}
	Derive(&a, "two", map[string]string{"site/cart.html": "cart-one", "site/theme.css": "theme-two"})
	if !a.Stale || a.Automation[0].Stale || a.Findings[0].Stale || len(a.Gaps) != 0 || len(a.Findings[0].Decisions) != 1 {
		t.Fatalf("unrelated CSS change invalidated scoped evidence: %#v", a)
	}
	Derive(&a, "three", map[string]string{"site/cart.html": "cart-three", "site/theme.css": "theme-two"})
	if !a.Automation[0].Stale || !a.Findings[0].Stale || len(a.Gaps) != 3 {
		t.Fatalf("affected source change retained stale acceptance: %#v", a)
	}
}

func TestConfirmedFindingRetainsGovernedRepairAndDeliveryProgress(t *testing.T) {
	s, _ := New(t.TempDir())
	a, _ := s.Create("repo", "pull", "owner", Input{Revision: "base", CommitmentID: "commitment", CommitmentVersion: 2, Scenarios: []Scenario{{ID: "dialog", Name: "Use dialog", Journey: "Open and close", Audiences: []string{"keyboard users"}, Evaluations: []string{"focus"}, Locations: []Location{{Path: "dialog.go", BlobID: "old"}}, Digest: "digest"}}})
	a, _ = s.AddFinding("repo", "pull", a.ID, "specialist", FindingInput{ScenarioID: "dialog", Evaluation: "focus", Result: "barrier", Severity: "high", Audiences: []string{"keyboard users"}, Locations: []Location{{Path: "dialog.go", BlobID: "old"}}, Summary: "Focus is lost", Citation: Citation{Kind: "reproduction", ResourceID: "barrier", EvidenceIDs: []string{"tree"}}})
	fid := a.Findings[0].ID
	if _, _, err := s.CreateRepair("repo", "pull", a.ID, fid, "owner", Repair{Revision: "base", AcceptanceCriteria: []string{"Focus returns to the opener"}, CommitmentID: "commitment", CommitmentVersion: 2, ComponentGuidance: []string{"Use the shared dialog focus helper"}, OwnerKind: "agent", OwnerID: "codex", ProposalID: "proposal", TaskID: "task"}); err == nil {
		t.Fatal("unconfirmed finding created delivery work")
	}
	a, _ = s.Decide("repo", "pull", a.ID, fid, "owner", DecisionInput{Outcome: "confirmed", Rationale: "Reproduced from the retained accessibility tree"})
	a, repair, err := s.CreateRepair("repo", "pull", a.ID, fid, "owner", Repair{Revision: "base", AcceptanceCriteria: []string{"Focus returns to the opener"}, EvidenceIDs: []string{"tree"}, CommitmentID: "commitment", CommitmentVersion: 2, ComponentGuidance: []string{"Use the shared dialog focus helper"}, OwnerKind: "agent", OwnerID: "codex", ProposalID: "proposal", TaskID: "task", ChangeSessionID: "session"})
	if err != nil || repair.CreatedByID != "owner" || len(repair.Progress) != 1 {
		t.Fatalf("repair provenance missing: %#v, %v", repair, err)
	}
	a, err = s.AddRepairProgress("repo", "pull", a.ID, fid, repair.ID, "codex", "in_progress", "Implementing the focus handoff")
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.LinkRepairDelivery("repo", "pull", a.ID, fid, repair.ID, "owner", RepairDelivery{PullRequestID: "repair-pull", Revision: "candidate", PreviewID: "preview", DesignChanges: []string{"Return focus on close"}, CodeChanges: []string{"Use the dialog helper"}, InteractionTradeoffs: []string{"Delay close until focus target resolves"}, ContentTradeoffs: []string{"Keep the existing close label"}})
	if err != nil || a.Findings[0].Repair.Delivery.LinkedByID != "owner" || len(a.Findings[0].Repair.Progress) != 3 {
		t.Fatalf("delivery did not report to original finding: %#v, %v", a.Findings[0].Repair, err)
	}
}
