package capacityplans

import (
	"testing"
	"time"
)

func TestPlanAuthorityAndDelivery(t *testing.T) {
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	s.now = func() time.Time { return now }
	in := Input{ObjectiveID: "objective", ObjectiveVersion: 1, ModelID: "model", ModelRevision: 1, RehearsalID: "rehearsal", RehearsalRevision: 4, CandidateID: "horizontal", Title: "Scale API", Rationale: "rehearsal proof", OwnerIDs: []string{"eng", "finops"}, Budget: 10000, Currency: "USD", Reservations: []Reservation{{ID: "db", Kind: "capacity", ResourceID: "database", Quantity: 10, Unit: "nodes", NeededBy: now.Add(time.Hour), OwnerID: "infra"}}, Dependencies: []Dependency{{ID: "quota", Kind: "quota", ResourceID: "cloud", Requirement: "raise instance quota", OwnerID: "finops", NeededBy: now.Add(time.Hour)}}, Phases: []Phase{{ID: "prepare", Name: "Prepare", Order: 1, Scope: []string{"application", "schema", "observability"}, OwnerIDs: []string{"eng"}, Budget: 1000, Currency: "USD", ReservationIDs: []string{"db"}, DependencyIDs: []string{"quota"}, Gates: []Gate{{Kind: "review", Required: true}, {Kind: "checks", Required: true}}, SuccessCriteria: []string{"checks pass"}, ExitCriteria: []string{"rollback"}}}, DecisionPoints: []DecisionPoint{{ID: "go", AfterPhaseID: "prepare", Question: "proceed?", OwnerID: "eng", DueAt: now.Add(time.Hour), Options: []string{"proceed", "exit"}, EvidenceRequired: []string{"pull"}}}, ExitStrategy: []string{"remove reservation"}}
	p, e := s.Create("repo", "eng", in)
	if e != nil {
		t.Fatal(e)
	}
	if Resolve(p).Status != "draft" {
		t.Fatal("authority gaps must block")
	}
	if _, e = s.Approve("repo", p.ID, "stranger", ApprovalInput{ExpectedRevision: 1, Decision: "approved", Rationale: "ok"}); e != ErrForbidden {
		t.Fatalf("want forbidden: %v", e)
	}
	p, e = s.AddWork("repo", p.ID, "eng", WorkInput{ExpectedRevision: 1, PhaseID: "prepare", Kind: "pull_request", ResourceID: "pr-1", Revision: "abc", OwnerKind: "agent", OwnerID: "agent-1", Status: "planned", GateEvidence: map[string]string{"review": "review-1"}})
	if e != nil {
		t.Fatal(e)
	}
	if p.Work[0].CreatorID != "eng" || p.Revision != 2 {
		t.Fatal("work not retained")
	}
	p, e = s.Decide("repo", p.ID, "go", "eng", DecisionInput{ExpectedRevision: 2, Outcome: "proceed", EvidenceIDs: []string{"pr-1"}, Rationale: "gates passed"})
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Approve("repo", p.ID, "eng", ApprovalInput{ExpectedRevision: 3, Decision: "approved", Rationale: "engineering scope"})
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Approve("repo", p.ID, "finops", ApprovalInput{ExpectedRevision: 4, Decision: "approved", Rationale: "plan only"})
	if e != nil {
		t.Fatal(e)
	}
	r := Resolve(p)
	if r.Status != "draft" {
		t.Fatal("plan approval must not approve reservation or quota")
	}
}
