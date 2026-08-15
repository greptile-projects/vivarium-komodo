package reliabilitypolicies

import "testing"

func TestDeliveryPolicyContainsExactAffectedWork(t *testing.T) {
	s, _ := New(t.TempDir())
	p, e := s.Create("repo", "maintainer", PolicyInput{Name: "Production margin", ObjectiveID: "objective", ObjectiveVersion: 2, Branches: []string{"main"}, Services: []string{"api"}, Environments: []string{"production"}, Journeys: []string{"review"}, RiskClasses: []string{"high"}, RequiredOwnerIDs: []string{"service-owner"}, Rules: []Rule{{Condition: "budget_threshold", Action: "slow", ThresholdPercent: 80}, {Condition: "budget_exhausted", Action: "block"}, {Condition: "regression", Action: "rollback"}, {Condition: "missing_evidence", Action: "pause"}, {Condition: "dependency_failure", Action: "block"}}, ChangeReason: "protect current users"})
	if e != nil {
		t.Fatal(e)
	}
	c := Context{Kind: "deployment", ResourceID: "deploy-1", Revision: "commit-a", Branch: "main", Service: "api", Environment: "production", Journeys: []string{"review"}, RiskClass: "high"}
	_, e = s.RecordImpact("repo", p.ID, "release-manager", ImpactInput{Context: c, Phase: "predicted", PredictedBudgetConsumedPercent: 105, Regression: true, EvidenceStatus: "stale", DependencyStatus: "failed", Summary: "canary predicts availability loss", Evidence: []string{"check:availability@commit-a"}})
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Assess("repo", c, []string{"exception:bounded-canary"})
	if e != nil {
		t.Fatal(e)
	}
	if a.Ready || len(a.Impacts) != 1 || len(a.ActiveExceptions) != 1 {
		t.Fatalf("unsafe delivery was not contained: %+v", a)
	}
	kinds := map[string]bool{}
	actions := map[string]bool{}
	for _, r := range a.Requirements {
		kinds[r.Kind] = true
		actions[r.Action] = true
	}
	for _, want := range []string{"budget_exhausted", "regression", "missing_evidence", "dependency_failure", "owner_acknowledgement"} {
		if !kinds[want] {
			t.Fatalf("missing %s: %+v", want, a.Requirements)
		}
	}
	for _, want := range []string{"block", "pause", "rollback"} {
		if !actions[want] {
			t.Fatalf("missing %s action", want)
		}
	}
	if _, e = s.Acknowledge("repo", p.ID, "release-manager", "acknowledged", "looks fine", c); e == nil {
		t.Fatal("non-owner acknowledgement accepted")
	}
	if _, e = s.Acknowledge("repo", p.ID, "service-owner", "acknowledged", "I accept the bounded response", c); e != nil {
		t.Fatal(e)
	}
	a, _ = s.Assess("repo", c, nil)
	for _, r := range a.Requirements {
		if r.Kind == "owner_acknowledgement" {
			t.Fatal("exact owner acknowledgement not applied")
		}
	}
}

func TestPolicySelectorAndMissingImpact(t *testing.T) {
	s, _ := New(t.TempDir())
	p, _ := s.Create("repo", "owner", PolicyInput{Name: "Queue gate", ObjectiveID: "o", ObjectiveVersion: 1, Branches: []string{"main"}, RequiredOwnerIDs: []string{"owner"}, Rules: []Rule{{Condition: "missing_evidence", Action: "block"}}, ChangeReason: "gate main"})
	_ = p
	a, _ := s.Assess("repo", Context{Kind: "integration_queue", ResourceID: "q", Revision: "c", Branch: "other"}, nil)
	if len(a.AppliedPolicyIDs) != 0 || !a.Ready {
		t.Fatalf("unmatched policy applied: %+v", a)
	}
	a, _ = s.Assess("repo", Context{Kind: "pull_request", ResourceID: "pr", Revision: "c", Branch: "main"}, nil)
	if a.Ready || len(a.Requirements) == 0 || a.Requirements[0].Kind != "missing_impact" {
		t.Fatalf("missing prediction did not block: %+v", a)
	}
}
