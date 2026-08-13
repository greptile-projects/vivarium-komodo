package productexperiments

import "testing"

func plan(signal string) PlanInput {
	return PlanInput{Title: "Safer onboarding", Source: Source{Kind: "issue", ID: "issue-1"}, Hypothesis: "Guidance improves activation without increasing support load", Variants: []Variant{{ID: "control", Name: "Current", Control: true}, {ID: "guided", Name: "Guided"}}, Audience: Audience{Description: "New repository owners", Eligibility: []string{"created repository in 7 days"}, Exclusions: []string{"staff"}, Consent: "product_analytics", EstimatedSize: 500}, Measures: []Measure{{ID: "activation", Name: "Activation", Kind: "success", SignalID: signal, SignalVersion: 1, Aggregation: "conversion rate", Threshold: "+5%"}, {ID: "support", Name: "Support contacts", Kind: "guardrail", SignalID: signal, SignalVersion: 1, Aggregation: "count per user", Threshold: "no more than +2%"}}, MinimumEvidence: "100 users per variant and 95% confidence", DurationHours: 168, OwnerIDs: []string{"owner"}, ParticipantIDs: []string{"owner", "analyst"}, StopConditions: []string{"support contacts increase 2%"}, Assumptions: []string{"signal identity is stable"}, OverlapKeys: []string{"onboarding:new-owner"}, ChangeReason: "Agree before exposure"}
}
func TestPlanReadinessTracksVersionedSignalsApprovalsAndAssumptions(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, e := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Repository becomes active", Unit: "users", Event: "repository.activated", PermittedAudiences: []string{"product_analytics"}, Instrumented: true, ChangeReason: "approved telemetry"})
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.Create("repo", "owner", plan(sig.ID))
	if e != nil {
		t.Fatal(e)
	}
	if x.Ready || len(x.Blockers) != 2 {
		t.Fatalf("expected participant blockers: %+v", x.Blockers)
	}
	x, _ = s.Approve("repo", x.ID, "owner", "approved", "")
	x, _ = s.Approve("repo", x.ID, "analyst", "approved", "")
	if !x.Ready {
		t.Fatalf("approved plan not ready: %+v", x.Blockers)
	}
	x, _ = s.ChangeAssumption("repo", x.ID, "analyst", "signal identity is stable", "event renamed upstream")
	if x.Ready || x.Blockers[0].Kind != "changed_assumption" {
		t.Fatalf("changed assumption hidden: %+v", x.Blockers)
	}
	p := plan(sig.ID)
	p.Assumptions = []string{"new event is stable"}
	p.ChangeReason = "Address signal rename"
	x, e = s.Revise("repo", x.ID, "owner", 1, p)
	if e != nil {
		t.Fatal(e)
	}
	if x.Ready {
		t.Fatal("old approvals incorrectly approved revision")
	}
}
func TestOverlapAndSignalPolicyStayExplicit(t *testing.T) {
	s, _ := New(t.TempDir())
	sig, _ := s.CreateSignal("repo", "owner", SignalVersion{Name: "activation", Description: "Activation", Unit: "users", Event: "activated", PermittedAudiences: []string{"research_consent"}, Instrumented: false, ChangeReason: "draft"})
	first, _ := s.Create("repo", "owner", plan(sig.ID))
	second, _ := s.Create("repo", "owner", plan(sig.ID))
	second, _ = s.Get("repo", second.ID)
	kinds := map[string]bool{}
	for _, b := range second.Blockers {
		kinds[b.Kind] = true
	}
	if !kinds["missing_instrumentation"] || !kinds["ineligible_audience"] || !kinds["overlapping_experiment"] {
		t.Fatalf("missing derived blockers %+v (first %s)", second.Blockers, first.ID)
	}
}
