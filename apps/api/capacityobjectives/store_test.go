package capacityobjectives

import (
	"testing"
	"time"
)

func input(now time.Time) VersionInput {
	return VersionInput{SubjectKind: "api", SubjectID: "public", Title: "Launch demand", Description: "Serve roadmap adoption", Forecasts: []Forecast{{ID: "launch", Segment: "active teams", Demand: 1000, Unit: "requests/second", StartsAt: now, EndsAt: now.Add(time.Hour), Confidence: "estimated"}}, TrafficShapes: []TrafficShape{{Name: "launch", Pattern: "bursty", PeakMultiplier: 2, DurationMinutes: 30}}, ServiceLevels: []Commitment{{ID: "availability", Kind: "availability", Scope: "api", Operator: "at_least", Value: 99.9, Unit: "percent"}}, BottleneckThresholds: []Commitment{{ID: "pool-floor", Kind: "connections", Scope: "database", Operator: "at_least", Value: 200, Unit: "connections"}, {ID: "pool-cap", Kind: "connections", Scope: "database", Operator: "at_most", Value: 100, Unit: "connections"}}, DependencyLimits: []Limit{{Dependency: "database", Metric: "connections", Maximum: 100, Unit: "connections"}}, Regions: []string{"eu-west-1"}, OwnerIDs: []string{"owner"}, BudgetAmount: 5000, BudgetCurrency: "USD", LeadTimeDays: 45, Signals: []Signal{{Name: "request rate", Required: true, OwnerID: "operator"}}, Assumptions: []Assumption{{ID: "growth", Statement: "launch grows demand", OwnerID: "product", ExpiresAt: now.Add(7 * 24 * time.Hour)}}, SuccessCriteria: []string{"headroom remains above 30%"}, RollbackCriteria: []string{"error rate exceeds 1%"}, Links: []Link{{Kind: "product_roadmap", ResourceID: "roadmap-1"}, {Kind: "performance_goal", ResourceID: "goal-1"}}, ChangeReason: "agree launch boundary"}
}
func TestVersioningAndDerivedGaps(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.now = func() time.Time { return now }
	o, e := s.Create("repo", "author", input(now))
	if e != nil {
		t.Fatal(e)
	}
	o = Resolve(o, now)
	k := map[string]bool{}
	for _, g := range o.Gaps {
		k[g.Kind] = true
	}
	for _, want := range []string{"unsupported_forecast", "missing_signal", "conflicting_commitment", "expiring_assumption"} {
		if !k[want] {
			t.Fatalf("missing %s: %#v", want, o.Gaps)
		}
	}
	in := input(now)
	in.ChangeReason = "refresh"
	if _, e = s.Revise("repo", o.ID, "owner", 0, in); e != ErrConflict {
		t.Fatalf("want conflict, got %v", e)
	}
	o, e = s.Revise("repo", o.ID, "owner", 1, in)
	if e != nil || o.CurrentVersion != 2 {
		t.Fatalf("revision: %#v %v", o, e)
	}
}
