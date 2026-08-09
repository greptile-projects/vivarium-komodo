package relationships

import "testing"

func TestEvolutionRolloutRequiresOwnersAndPausesWithoutStrandingCompletedSteps(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := store.CreateEvolution(EvolutionPlan{RepositoryID: "provider", InterfaceName: "payments", SourceKind: "pull_request", SourceID: "pr", CandidateCommitID: "candidate", CandidateSchemaPath: "api.json", Predecessor: Interface{ID: "prior"}, AffectedConsumers: []AffectedConsumer{{RepositoryID: "consumer", OwnerID: "consumer-owner"}}, CreatedByID: "provider-owner"})
	if err != nil {
		t.Fatal(err)
	}
	plan, verification, err := store.CreateEvolutionVerification(plan.ID, "provider-owner", []EvolutionRevision{{RepositoryID: "provider", CommitID: "p", TaskID: "pt", PullRequestID: "ppr"}, {RepositoryID: "consumer", CommitID: "c", TaskID: "ct", PullRequestID: "cpr"}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.ConfigureEvolutionRollout(plan.ID, "provider-owner", verification.ID, []EvolutionRolloutPhaseInput{
		{Name: "Consumers first", Gates: []string{"contract", "integration"}, Steps: []EvolutionRolloutStep{{RepositoryID: "consumer", Kind: "queue"}, {RepositoryID: "consumer", Kind: "release"}, {RepositoryID: "consumer", Kind: "deployment"}}},
		{Name: "Provider cutover", Gates: []string{"integration"}, Steps: []EvolutionRolloutStep{{RepositoryID: "provider", Kind: "queue"}, {RepositoryID: "provider", Kind: "deployment"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, second := plan.Rollout.Phases[0], plan.Rollout.Phases[1]
	if first.State != "pending" || second.State != "pending" {
		t.Fatalf("unexpected phases: %+v", plan.Rollout.Phases)
	}
	plan, err = store.ApproveEvolutionRollout(plan.ID, first.ID, "consumer", "consumer-owner", "approve", "window reserved")
	if err != nil || plan.Rollout.Phases[0].State != "ready" {
		t.Fatalf("approval: %v %+v", err, plan.Rollout)
	}
	for _, step := range plan.Rollout.Phases[0].Steps[:2] {
		plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, first.ID, step.ID, "consumer-owner", step.ID+"-resource", "succeeded", "retained evidence")
		if err != nil {
			t.Fatal(err)
		}
	}
	last := plan.Rollout.Phases[0].Steps[2]
	plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, first.ID, last.ID, "consumer-owner", "deployment", "failed", "health gate failed")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Rollout.State != "paused" || plan.Rollout.Phases[0].State != "paused" || plan.Rollout.Phases[1].State != "pending" || plan.Rollout.Phases[0].Steps[0].State != "succeeded" {
		t.Fatalf("unsafe failure state: %+v", plan.Rollout)
	}
	plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, first.ID, last.ID, "consumer-owner", "rollback-deployment", "rolled_back", "known good restored")
	if err != nil || plan.Rollout.Phases[0].State != "completed" {
		t.Fatalf("rollback: %v %+v", err, plan.Rollout)
	}
	plan, err = store.ApproveEvolutionRollout(plan.ID, second.ID, "provider", "provider-owner", "approve", "consumer safe")
	if err != nil || plan.Rollout.Phases[1].State != "ready" {
		t.Fatalf("next phase: %v %+v", err, plan.Rollout)
	}
}
