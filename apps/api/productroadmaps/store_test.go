package productroadmaps

import (
	"errors"
	"testing"
	"time"
)

func TestRoadmapVersioningAndBlockers(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	o := Outcome{ID: "outcome-a", OpportunityID: "opp", OpportunityVersion: 2, Title: "Faster review", Decision: "accepted", Status: "planned", OwnerID: "maintainer", OwnerAvailable: false, TargetHorizon: now.Add(-time.Hour), SuccessMeasures: []string{"median review under one day"}, DependsOn: []string{"missing"}, CapacityUnits: 6, Sequence: 1, Rationale: "Broad, severe evidence aligns with reliability."}
	in := Input{Name: "Direction", Goals: []string{"shorten review"}, CapacityUnits: 5, Outcomes: []Outcome{o}, ChangeReason: "Initial prioritization"}
	v, e := s.Create("repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Versions[0].Blockers) != 4 {
		t.Fatalf("blockers = %#v", v.Versions[0].Blockers)
	}
	if _, e = s.Replan("repo", v.ID, "other", 0, in); !errors.Is(e, ErrConflict) {
		t.Fatalf("conflict = %v", e)
	}
	o.OwnerAvailable = true
	o.TargetHorizon = now.Add(24 * time.Hour)
	o.DependsOn = nil
	o.CapacityUnits = 5
	in.Outcomes = []Outcome{o}
	in.ChangeReason = "Owner accepted and capacity reconciled"
	v, e = s.Replan("repo", v.ID, "owner", 1, in)
	if e != nil || v.CurrentVersion != 2 || len(v.Versions[1].Blockers) != 0 {
		t.Fatalf("replan = %#v, %v", v, e)
	}
	v, e = s.Scenario("repo", v.ID, "codex", "agent", 2, "Defer until dependency lands", []Outcome{o})
	if e != nil || v.Scenarios[0].ResourceAuthority {
		t.Fatalf("scenario = %#v, %v", v.Scenarios, e)
	}
	v, e = s.Comment("repo", v.ID, "reader", "Capacity risk is still material", 2)
	if e != nil || len(v.Comments) != 1 {
		t.Fatalf("comment = %#v, %v", v, e)
	}
}

func TestAcceptedOutcomeRequiresPromise(t *testing.T) {
	s, _ := New(t.TempDir())
	_, e := s.Create("repo", "owner", Input{Name: "x", Goals: []string{"g"}, CapacityUnits: 1, ChangeReason: "x", Outcomes: []Outcome{{ID: "o", OpportunityID: "p", OpportunityVersion: 1, Title: "x", Decision: "accepted", Sequence: 1, Rationale: "x"}}})
	if !errors.Is(e, ErrInvalid) {
		t.Fatalf("error = %v", e)
	}
}
