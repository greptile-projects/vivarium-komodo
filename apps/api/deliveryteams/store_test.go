package deliveryteams

import (
	"errors"
	"testing"
	"time"
)

func TestAttributableTeamFormation(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(48 * time.Hour)
	v, err := s.Create("repo", "Release team", "lead", Outcome{Kind: "decision", ID: "d1", Title: "Ship safely"}, CharterInput{Outcome: "Deliver the accepted choice", SuccessMeasures: []string{"checks pass"}, OperatingPrinciples: []string{"escalate uncertainty"}, TotalBudget: Budget{Hours: 20, AgentRuns: 2}, Deadline: &deadline, DefaultEscalation: "lead"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Invite("repo", v.ID, "lead", v.Version, ParticipantInput{Kind: "human", PrincipalID: "dev", Role: "implementer", Why: "owns the subsystem", Responsibilities: []string{"implementation"}, Budget: Budget{Hours: 8}, RequestedActions: []string{"contents:read"}, Access: AccessPreview{Actions: []string{"contents:read"}}})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Respond("repo", v.ID, v.Participants[0].ID, "dev", "accepted", v.Version)
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Revise("repo", v.ID, "dev", v.Version, CharterInput{Outcome: "Deliver the accepted choice", SuccessMeasures: []string{"checks pass", "owner approves"}, OperatingPrinciples: []string{"escalate uncertainty"}, TotalBudget: Budget{Hours: 20, AgentRuns: 2}, Deadline: &deadline, DefaultEscalation: "lead", ChangeReason: "add review boundary"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.CharterHistory) != 2 || v.Events[len(v.Events)-1].ActorID != "dev" {
		t.Fatalf("history was not attributed: %#v", v)
	}
	if _, err = s.Remove("repo", v.ID, v.Participants[0].ID, "lead", "scope changed", v.Version-1); !errors.Is(err, ErrConflict) {
		t.Fatalf("wanted concurrency conflict, got %v", err)
	}
}
