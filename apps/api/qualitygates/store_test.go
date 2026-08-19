package qualitygates

import (
	"testing"
	"time"
)

func TestRevisionExactMatrixInvalidatesOnlyAffectedEvidence(t *testing.T) {
	s, _ := New(t.TempDir())
	policy, err := s.CreatePolicy("repo", "owner", PolicyInput{Name: "Release confidence", PlanID: "plan", PlanVersion: 2, ChangeReason: "protect checkout", Selector: Selector{Branches: []string{"main"}, Journeys: []string{"checkout"}, Locales: []string{"en-US", "fr-FR"}}, Requirements: []Requirement{
		{ID: "chrome", BehaviorID: "pay", ScenarioID: "checkout", Kind: "scenario", Environment: "preview", Journey: "checkout", RiskClass: "critical", Locale: "en-US", Platform: "chromium", OwnerID: "owner", Required: true},
		{ID: "safari", BehaviorID: "pay", Kind: "exploratory", Environment: "device-lab", Journey: "checkout", RiskClass: "critical", Locale: "fr-FR", Platform: "safari", OwnerID: "owner", Required: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Open("repo", "maintainer", OpenInput{PolicyID: policy.ID, PolicyVersion: 1, Target: Target{Kind: "pull_request", Reference: "pr-7", Revision: "abc", Branch: "main"}})
	if err != nil {
		t.Fatal(err)
	}
	g, err = s.AddAttempt("repo", g.ID, "tester", AttemptInput{RequirementID: "chrome", Kind: "scenario", Status: "passed", ScenarioVersion: 3, Environment: "preview", Locale: "en-US", Platform: "chromium", InputPaths: []string{"checkout/pay.go"}, DependencyRevisions: []string{"payments@1"}, Evidence: []string{"artifact:one"}})
	if err != nil {
		t.Fatal(err)
	}
	g, _ = s.AddAttempt("repo", g.ID, "tester", AttemptInput{RequirementID: "safari", Kind: "exploratory", Status: "flaky", Environment: "device-lab", Locale: "fr-FR", Platform: "safari", InputPaths: []string{"checkout/view.tsx"}, Evidence: []string{"session:two"}, FlakeReason: "timing"})
	g, _ = s.Acknowledge("repo", g.ID, "owner", "chrome", "accepted", "exact candidate inspected")
	g, _ = s.Acknowledge("repo", g.ID, "owner", "safari", "accepted", "flake understood")
	if g.Ready {
		t.Fatal("flake must block")
	}
	g, err = s.Override("repo", g.ID, "release-owner", OverrideInput{RequirementIDs: []string{"safari"}, Rationale: "contained browser timing", FollowUpWorkID: "issue-88", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || !g.Ready {
		t.Fatalf("scoped current override should unblock: %v %#v", err, g.Blockers)
	}
	g, err = s.Revise("repo", g.ID, "maintainer", RevisionInput{ExpectedRevision: "abc", Revision: "def", ChangedPaths: []string{"checkout/pay.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if !g.Attempts[0].Stale || g.Attempts[1].Stale {
		t.Fatalf("only intersecting attempt should stale: %#v", g.Attempts)
	}
	if g.Ready {
		t.Fatal("acknowledgements and overrides are revision-bound")
	}
}

func TestPostReleaseReopenRequiresFollowUp(t *testing.T) {
	s, _ := New(t.TempDir())
	p, _ := s.CreatePolicy("repo", "owner", PolicyInput{Name: "Quality", PlanID: "plan", PlanVersion: 1, ChangeReason: "initial", Requirements: []Requirement{{ID: "r", BehaviorID: "b", ScenarioID: "s", Kind: "scenario", Environment: "prod", OwnerID: "owner", Required: true}}})
	g, _ := s.Open("repo", "owner", OpenInput{PolicyID: p.ID, PolicyVersion: 1, Target: Target{Kind: "release", Reference: "v1", Revision: "sha"}})
	if _, err := s.Signal("repo", g.ID, "tester", SignalInput{RequirementID: "r", ReleaseID: "v1", Status: "reopened", Evidence: "observation:1", Rationale: "sample failed"}); err != ErrInvalid {
		t.Fatalf("expected follow-up validation, got %v", err)
	}
	g, err := s.Signal("repo", g.ID, "tester", SignalInput{RequirementID: "r", ReleaseID: "v1", Status: "reopened", Evidence: "observation:1", Rationale: "sample failed", FollowUpWorkID: "issue-9"})
	if err != nil || g.Matrix[0].Status != "reopened" {
		t.Fatalf("risk should reopen: %v %#v", err, g.Matrix)
	}
}
