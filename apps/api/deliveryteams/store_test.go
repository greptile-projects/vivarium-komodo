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

func TestRevisionBoundParallelPlanExposesBlockersAndRequiresOwners(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create("repo", "Parallel team", "lead", Outcome{Kind: "planned_outcome", Title: "Ship one result"}, CharterInput{Outcome: "Ship", SuccessMeasures: []string{"accepted"}, OperatingPrinciples: []string{"surface conflicts"}, TotalBudget: Budget{Hours: 20}, DefaultEscalation: "lead"})
	if err != nil {
		t.Fatal(err)
	}
	for _, principal := range []string{"dev", "qa"} {
		v, err = s.Invite("repo", v.ID, "lead", v.Version, ParticipantInput{Kind: "human", PrincipalID: principal, Role: principal, Why: "specialist", Responsibilities: []string{principal}, Budget: Budget{Hours: 10}, RequestedActions: []string{"contents:read"}, Access: AccessPreview{Actions: []string{"contents:read"}}})
		if err != nil {
			t.Fatal(err)
		}
		v, err = s.Respond("repo", v.ID, v.Participants[len(v.Participants)-1].ID, principal, "accepted", v.Version)
		if err != nil {
			t.Fatal(err)
		}
	}
	commit := "1111111111111111111111111111111111111111"
	streams := []WorkStream{
		{ID: "api", Title: "API", OwnerParticipantID: v.Participants[0].ID, Inputs: []string{"contract"}, ExpectedArtifacts: []string{"handler"}, AcceptanceCriteria: []string{"tests pass"}, RepositoryScope: []RepositoryScope{{RepositoryID: "repo", CommitID: commit, Paths: []string{"src/shared"}, RequiredActions: []string{"contents:read"}}}, IntegrationOrder: 2, Budget: Budget{Hours: 8}},
		{ID: "web", Title: "Web", OwnerParticipantID: v.Participants[1].ID, Inputs: []string{"API"}, ExpectedArtifacts: []string{"screen"}, DependsOn: []string{"api"}, AcceptanceCriteria: []string{"usable"}, RepositoryScope: []RepositoryScope{{RepositoryID: "repo", CommitID: commit, Paths: []string{"src/shared/button"}, RequiredActions: []string{"candidate_branch:write"}}}, IntegrationOrder: 1, Budget: Budget{Hours: 8}, Assumptions: []Assumption{{ID: "api-shape", Statement: "API is stable", SourceStreamID: "api", SourceStreamRevision: 1}}},
	}
	v, err = s.ProposePlan("repo", v.ID, "dev", v.Participants[0].ID, v.Version, PlanInput{Streams: streams, ChangeReason: "initial decomposition"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Plan.Current.Status != "pending_acceptance" || len(v.Plan.Current.Blockers) != 3 {
		t.Fatalf("expected overlap, order, and access blockers: %#v", v.Plan.Current.Blockers)
	}
	streams[1].IntegrationOrder = 3
	streams[1].RepositoryScope[0].Paths = []string{"src/web"}
	streams[1].RepositoryScope[0].RequiredActions = []string{"contents:read"}
	v, err = s.ProposePlan("repo", v.ID, "dev", v.Participants[0].ID, v.Version, PlanInput{Streams: streams, ChangeReason: "resolve visible blockers"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Plan.Current.Blockers) != 0 || len(v.Plan.Current.Acceptances) != 1 {
		t.Fatalf("unexpected revised plan: %#v", v.Plan.Current)
	}
	v, err = s.AcceptPlan("repo", v.ID, v.Participants[1].ID, "qa", v.Version, v.Plan.Current.Version)
	if err != nil {
		t.Fatal(err)
	}
	if v.Plan.Current.Status != "accepted" || len(v.Plan.History) != 2 {
		t.Fatalf("owners did not accept immutable revision: %#v", v.Plan)
	}
	if _, err = s.AcceptPlan("repo", v.ID, v.Participants[1].ID, "qa", v.Version-1, v.Plan.Current.Version); !errors.Is(err, ErrConflict) {
		t.Fatalf("wanted stale conflict, got %v", err)
	}
}
