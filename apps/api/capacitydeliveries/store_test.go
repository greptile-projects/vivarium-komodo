package capacitydeliveries

import (
	"testing"
	"time"
)

func TestProductionProofAndDeterministicContainment(t *testing.T) {
	now := time.Date(2026, 8, 26, 2, 0, 0, 0, time.UTC)
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.now = func() time.Time { return now }
	in := Input{PlanID: "plan", PlanRevision: 4, ObjectiveID: "objective", ObjectiveVersion: 2, ModelID: "model", ModelRevision: 3, DecisionRevisitID: "decision", Phases: []Phase{{ID: "regional", PlanPhaseID: "rollout", Name: "Regional rollout", EnvironmentID: "production", EnvironmentRevision: "policy-7", ControllerID: "autoscaler", OperatorIDs: []string{"operator"}, DelegatedAgentIDs: []string{"agent"}, TargetCapacity: 200, CapacityUnit: "rps", MaxLoad: 160, MinHeadroomPercent: 20, MaxCost: 500, Currency: "USD"}}}
	d, e := s.Create("repo", "operator", in)
	if e != nil {
		t.Fatal(e)
	}
	if d.State != "paused" || d.Blockers[0].Kind != "missing_production_evidence" {
		t.Fatalf("missing evidence must contain: %#v", d)
	}
	if _, e = s.Control("repo", d.ID, "agent", "agent", ControlInput{ExpectedRevision: d.Revision, Action: "rollback", PhaseID: "regional", Rationale: "unsafe", EvidenceIDs: []string{"signal"}}); e != ErrForbidden {
		t.Fatalf("agent must be delegated and action bounded: %v", e)
	}
	o := ObservationInput{ExpectedRevision: d.Revision, PhaseID: "regional", ReleaseRevision: "commit-a", InfrastructureRevision: "infra-b", EvidenceWindowStart: now.Add(-time.Hour), EvidenceWindowEnd: now, ProductionEvidenceIDs: []string{"metrics-1"}, AllocatedCapacity: 220, UsableCapacity: 200, Load: 150, ForecastLoad: 150, HeadroomPercent: 25, ScalingLagSeconds: 20, MaxScalingLagSeconds: 60, RegionalImbalancePercent: 3, MaxRegionalImbalancePercent: 10, ServiceLevels: []ServiceLevel{{Name: "p99", Target: 200, Actual: 180, Unit: "ms", Met: true}}, Dependencies: []DependencyHealth{{DependencyID: "database", Status: "healthy", EvidenceID: "dep-1"}}, Correctness: "passed", Reliability: "healthy", Quota: "granted", Cost: 400, ReservationUtilizationPercent: 80, MinReservationUtilizationPercent: 50}
	d, e = s.Observe("repo", d.ID, "agent", "agent", o)
	if e != nil {
		t.Fatal(e)
	}
	if !d.ObjectiveValidated || !d.ForecastValidated || len(d.Blockers) != 0 {
		t.Fatalf("valid production proof not recognized: %#v", d)
	}
	o.ExpectedRevision = d.Revision
	o.Load = 230
	o.ForecastLoad = 150
	o.Cost = 700
	o.Quota = "denied"
	d, e = s.Observe("repo", d.ID, "human", "operator", o)
	if e != nil {
		t.Fatal(e)
	}
	if d.State != "paused" || d.Blockers[0].Kind != "quota_denial" || d.ObjectiveValidated {
		t.Fatalf("unsafe production evidence must contain deterministically: %#v", d)
	}
	if _, e = s.Control("repo", d.ID, "human", "operator", ControlInput{ExpectedRevision: d.Revision, Action: "resume", PhaseID: "regional", Rationale: "ignore", EvidenceIDs: []string{"metrics"}}); e != ErrConflict {
		t.Fatalf("resume must reject unresolved blockers: %v", e)
	}
}
