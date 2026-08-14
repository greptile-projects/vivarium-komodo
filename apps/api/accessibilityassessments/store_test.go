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
