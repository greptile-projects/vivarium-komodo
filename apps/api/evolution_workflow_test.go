package main

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
)

// TestCrossRepositoryEvolutionWorkflow is the durable regression boundary for
// the proposal-to-ecosystem collaboration contract. Transport-specific tests
// exercise the same records through HTTP; this test keeps the complete story in
// one place so later changes cannot preserve each endpoint independently while
// breaking permissions, attribution, exact-candidate evidence, or recovery
// between them.
func TestCrossRepositoryEvolutionWorkflow(t *testing.T) {
	store, err := relationships.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	provider := "payments-provider"
	consumer := "independent-checkout"
	consumerFork := "checkout-contributor-fork"
	providerOwner := "provider-maintainer"
	consumerOwner := "consumer-maintainer"
	contributor := "consumer-contributor"

	publication, err := store.Publish(relationships.Interface{
		RepositoryID: provider, Name: "payments", Version: "1.0.0",
		CommitID: "provider-v1", ReleaseID: "provider-release-v1",
		SchemaPath: "api/payments.json", PublishedByID: providerOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	dependency, err := store.Declare(relationships.Dependency{
		RepositoryID: consumer, CommitID: "consumer-v1",
		ReleaseID: "consumer-release-v1", ProviderRepositoryID: provider,
		InterfaceName: "payments", Constraint: "^1.0.0", DeclaredByID: consumerOwner,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := store.CreateEvolution(relationships.EvolutionPlan{
		RepositoryID: provider, InterfaceName: "payments", SourceKind: "proposal",
		SourceID: "proposal-payments-v2", CandidateCommitID: "provider-candidate",
		CandidateSchemaPath: "api/payments.json", CandidateSchemaSHA256: "candidate-digest",
		Predecessor: publication, PredecessorSchemaSHA256: "released-digest",
		AffectedConsumers: []relationships.AffectedConsumer{{
			DependencyID: dependency.ID, RepositoryID: consumer, OwnerID: consumerOwner,
			CommitID: dependency.CommitID, Constraint: dependency.Constraint,
		}}, CreatedByID: providerOwner,
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.UpdateEvolution(plan.ID, providerOwner, relationships.EvolutionUpdate{
		Strategy: "publish compatible consumer first, then cut over provider",
		Changes:  []relationships.CompatibilityChange{{Classification: "breaking", Area: "charge request", Summary: "currency becomes required", Rationale: "remove ambiguous settlement currency"}},
		Steps:    []relationships.MigrationStep{{ID: "consumer-window", OwnerID: consumerOwner, Summary: "adopt the dual-shape client"}, {ID: "provider-cutover", OwnerID: providerOwner, Summary: "publish v2 after consumer adoption", DependsOn: "consumer-window"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, analysisToken, err := store.StartEvolutionAnalysis(plan.ID, providerOwner, "codex", "inspect exact provider and consumer snapshots", []string{provider, consumer})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.AddEvolutionFinding(analysisToken, "risk", "checkout omits currency when retrying a charge", "dynamic plugin consumers were not sampled", []string{consumer})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.AcknowledgeEvolution(plan.ID, consumerOwner, "acknowledge", "consumer-first window reserved", []string{consumer})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.AffectedConsumers) != 1 || plan.AffectedConsumers[0].DependencyID != dependency.ID || len(plan.Findings) != 1 || plan.Findings[0].ActorID != "agent:codex" || plan.Acknowledgements[0].ActorID != consumerOwner {
		t.Fatalf("discovery, agent, or owner attribution was lost: %#v", plan)
	}

	plan, err = store.CreateMigrationTask(plan.ID, providerOwner, relationships.MigrationTaskInput{
		RepositoryID: provider, TargetRepositoryID: provider, TargetVersion: "2.0.0",
		Title: "Implement payments v2", Outcome: "provider accepts explicit currency",
		CompletionCriteria: []string{"contract suite passes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	providerTask := plan.Tasks[0]
	plan, err = store.CreateMigrationTask(plan.ID, providerOwner, relationships.MigrationTaskInput{
		RepositoryID: consumerFork, TargetRepositoryID: consumer, TargetVersion: "2.0.0",
		Title: "Adopt payments v2", Outcome: "checkout sends explicit currency",
		CompletionCriteria: []string{"integration suite passes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	consumerTask := plan.Tasks[1]

	plan, err = store.AssignMigrationTask(plan.ID, providerTask.ID, providerOwner, "", "agent", "codex", "implement the provider candidate", "provider-base")
	if err != nil {
		t.Fatal(err)
	}
	providerAssignment := plan.Tasks[0].Assignment
	plan, err = store.StartMigrationTask(plan.ID, providerTask.ID, providerOwner, providerAssignment.ID, provider, "evolution/payments-v2", "provider-base", "provider-agent-session")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.SynchronizeMigrationTask(plan.ID, providerTask.ID, "agent:codex", "provider-head", "provider-pull", false)
	if err != nil {
		t.Fatal(err)
	}

	plan, err = store.AssignMigrationTask(plan.ID, consumerTask.ID, consumerOwner, "", "human", contributor, "migrate checkout in the contributor-owned fork", "consumer-base")
	if err != nil {
		t.Fatal(err)
	}
	consumerAssignment := plan.Tasks[1].Assignment
	plan, err = store.StartMigrationTask(plan.ID, consumerTask.ID, contributor, consumerAssignment.ID, consumerFork, "evolution/payments-v2", "consumer-base", "consumer-human-session")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.SynchronizeMigrationTask(plan.ID, consumerTask.ID, contributor, "consumer-head", "consumer-pull", false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tasks[0].Assignment.AssignedByID != providerOwner || plan.Tasks[0].Work.SessionID != "provider-agent-session" || plan.Tasks[1].Assignment.AssignedByID != consumerOwner || plan.Tasks[1].Work.RepositoryID != consumerFork || plan.Tasks[1].Work.StartedByID != contributor {
		t.Fatalf("independent work authority or attribution was lost: %#v", plan.Tasks)
	}

	revisions := []relationships.EvolutionRevision{
		{RepositoryID: provider, CommitID: "provider-head", TaskID: providerTask.ID, PullRequestID: "provider-pull"},
		{RepositoryID: consumer, CommitID: "consumer-head", TaskID: consumerTask.ID, PullRequestID: "consumer-pull", DependencyID: dependency.ID},
	}
	plan, verification, err := store.CreateEvolutionVerification(plan.ID, providerOwner, revisions)
	if err != nil {
		t.Fatal(err)
	}
	plan, err = store.AttachEvolutionVerificationRuns(plan.ID, verification.ID, []string{"contract-run", "integration-run"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Verifications[0].TriggeredByID != providerOwner || plan.Verifications[0].Revisions[1].CommitID != "consumer-head" || plan.Verifications[0].Revisions[1].DependencyID != dependency.ID {
		t.Fatalf("exact compatibility attestation was lost: %#v", plan.Verifications)
	}

	plan, err = store.ConfigureEvolutionRollout(plan.ID, providerOwner, verification.ID, []relationships.EvolutionRolloutPhaseInput{
		{Name: "Consumer adoption", Gates: []string{"contract", "integration"}, Steps: []relationships.EvolutionRolloutStep{{RepositoryID: consumer, Kind: "queue"}, {RepositoryID: consumer, Kind: "deployment"}}},
		{Name: "Provider cutover", Gates: []string{"integration"}, Steps: []relationships.EvolutionRolloutStep{{RepositoryID: provider, Kind: "queue"}, {RepositoryID: provider, Kind: "release"}, {RepositoryID: provider, Kind: "deployment"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	consumerPhase := plan.Rollout.Phases[0]
	plan, err = store.ApproveEvolutionRollout(plan.ID, consumerPhase.ID, consumer, consumerOwner, "approve", "consumer window owned here")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range consumerPhase.Steps {
		state := "succeeded"
		if step.Kind == "deployment" {
			state = "failed"
		}
		plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, consumerPhase.ID, step.ID, consumerOwner, "consumer-"+step.Kind, state, "derived platform evidence")
		if err != nil {
			t.Fatal(err)
		}
	}
	if plan.Rollout.State != "paused" || plan.Rollout.Phases[1].State != "pending" || plan.Rollout.Phases[0].Steps[0].State != "succeeded" {
		t.Fatalf("failure did not preserve safe progress and block cutover: %#v", plan.Rollout)
	}
	failedDeployment := plan.Rollout.Phases[0].Steps[1]
	plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, consumerPhase.ID, failedDeployment.ID, consumerOwner, "consumer-rollback", "rolled_back", "known-good consumer restored")
	if err != nil {
		t.Fatal(err)
	}
	providerPhase := plan.Rollout.Phases[1]
	plan, err = store.ApproveEvolutionRollout(plan.ID, providerPhase.ID, provider, providerOwner, "approve", "consumer is safe")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range providerPhase.Steps {
		plan, err = store.RecordEvolutionRolloutOutcome(plan.ID, providerPhase.ID, step.ID, providerOwner, "provider-"+step.Kind, "succeeded", "derived platform evidence")
		if err != nil {
			t.Fatal(err)
		}
	}
	consumerOutcomes := plan.Rollout.Phases[0].Outcomes
	if plan.Rollout.State != "completed" || len(consumerOutcomes) != 3 || consumerOutcomes[2].State != "rolled_back" || plan.Rollout.Phases[1].Approvals[0].ActorID != providerOwner {
		t.Fatalf("compatible rollout or recovery evidence was lost: %#v", plan.Rollout)
	}
}
