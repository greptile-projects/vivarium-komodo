package relationships

import "testing"

func TestVersionConstraints(t *testing.T) {
	cases := []struct {
		version, constraint string
		want                bool
	}{
		{"1.4.2", "^1.2.0", true}, {"2.0.0", "^1.2.0", false},
		{"1.4.9", "~1.4.0", true}, {"1.5.0", "~1.4.0", false},
		{"2.0.0", ">=1.9.0", true}, {"1.2.3", "1.2.3", true}, {"9.0.0", "*", true},
	}
	for _, test := range cases {
		if got := Satisfies(test.version, test.constraint); got != test.want {
			t.Errorf("Satisfies(%q,%q)=%v", test.version, test.constraint, got)
		}
	}
}

func TestStoreRetainsPublicationsAndDeclarations(t *testing.T) {
	store, _ := New(t.TempDir())
	pub, err := store.Publish(Interface{RepositoryID: "provider", Name: "payments", Version: "1.2.0", CommitID: "abc", ReleaseID: "release", PublishedByID: "owner"})
	if err != nil || pub.ID == "" {
		t.Fatalf("publish = %#v %v", pub, err)
	}
	if _, err = store.Publish(Interface{RepositoryID: "provider", Name: "payments", Version: "1.2.0", CommitID: "def", ReleaseID: "other", PublishedByID: "owner"}); err != ErrConflict {
		t.Fatalf("duplicate = %v", err)
	}
	dep, err := store.Declare(Dependency{RepositoryID: "consumer", CommitID: "def", ProviderRepositoryID: "provider", InterfaceName: "payments", Constraint: "^1.0.0", DeclaredByID: "consumer-owner"})
	if err != nil || dep.ID == "" {
		t.Fatalf("declare = %#v %v", dep, err)
	}
	interfaces, _ := store.Interfaces()
	dependencies, _ := store.Dependencies()
	if len(interfaces) != 1 || len(dependencies) != 1 {
		t.Fatalf("retained %d %d", len(interfaces), len(dependencies))
	}
}

func TestEvolutionPlanRetainsContractAgreementAndReadOnlyAnalysis(t *testing.T) {
	store, _ := New(t.TempDir())
	predecessor := Interface{ID: "published", RepositoryID: "provider", Name: "payments", Version: "1.0.0", CommitID: "old", ReleaseID: "release", SchemaPath: "api.json"}
	plan, err := store.CreateEvolution(EvolutionPlan{RepositoryID: "provider", InterfaceName: "payments", SourceKind: "pull_request", SourceID: "pr", CandidateCommitID: "new", CandidateSchemaPath: "api.json", CandidateSchemaSHA256: "new-hash", Predecessor: predecessor, PredecessorSchemaSHA256: "old-hash", AffectedConsumers: []AffectedConsumer{{RepositoryID: "checkout", OwnerID: "consumer-owner"}}, CreatedByID: "provider-owner"})
	if err != nil || plan.ID == "" {
		t.Fatalf("create = %#v %v", plan, err)
	}
	plan, err = store.UpdateEvolution(plan.ID, "provider-owner", EvolutionUpdate{Strategy: "dual publish, migrate, retire", Changes: []CompatibilityChange{{Classification: "breaking", Area: "request", Summary: "required field", Rationale: "explicit routing"}}, Steps: []MigrationStep{{ID: "provider", OwnerID: "provider-owner", Summary: "publish both shapes"}, {OwnerID: "consumer-owner", Summary: "migrate checkout", DependsOn: "provider"}}, Exceptions: []EvolutionException{{ConsumerID: "legacy", Reason: "retire next quarter"}}})
	if err != nil || len(plan.Changes) != 1 || len(plan.Steps) != 2 {
		t.Fatalf("contract = %#v %v", plan, err)
	}
	plan, err = store.AcknowledgeEvolution(plan.ID, "consumer-owner", "acknowledge", "window works", []string{"checkout"})
	if err != nil || len(plan.Acknowledgements) != 1 {
		t.Fatalf("ack = %#v %v", plan, err)
	}
	plan, token, err := store.StartEvolutionAnalysis(plan.ID, "provider-owner", "codex", "inspect selected snapshots", []string{"provider", "checkout"})
	if err != nil || token == "" || plan.Analyses[0].CredentialDigest == "" {
		t.Fatalf("analysis = %#v %v", plan, err)
	}
	context, analysis, err := store.EvolutionAnalysisContext(token)
	if err != nil || context.Analyses[0].CredentialDigest != "" || analysis.CredentialDigest != "" {
		t.Fatalf("context leaked credential = %#v %#v %v", context, analysis, err)
	}
	plan, err = store.AddEvolutionFinding(token, "risk", "checkout assumes the removed field", "dynamic clients were not sampled", []string{"checkout"})
	if err != nil || len(plan.Findings) != 1 || plan.Findings[0].ActorID != "agent:codex" {
		t.Fatalf("finding = %#v %v", plan, err)
	}
	if _, err = store.AddEvolutionFinding(token, "risk", "out of scope", "", []string{"unknown"}); err != ErrInvalid {
		t.Fatalf("out-of-scope finding = %v", err)
	}
}

func TestEvolutionMigrationTasksCoordinateIndependentRepositoryWork(t *testing.T) {
	store, _ := New(t.TempDir())
	plan, _ := store.CreateEvolution(EvolutionPlan{RepositoryID: "provider", InterfaceName: "payments", SourceKind: "pull_request", SourceID: "pr", CandidateCommitID: "candidate", CandidateSchemaPath: "api.json", Predecessor: Interface{ID: "prior"}, AffectedConsumers: []AffectedConsumer{{RepositoryID: "consumer", OwnerID: "consumer-owner"}}, CreatedByID: "provider-owner"})
	plan, err := store.CreateMigrationTask(plan.ID, "provider-owner", MigrationTaskInput{RepositoryID: "consumer-fork", TargetRepositoryID: "consumer", TargetVersion: "2.0.0", Title: "Adopt payments v2", Outcome: "Checkout uses v2", CompletionCriteria: []string{"contract tests pass"}})
	if err != nil || len(plan.Tasks) != 1 || !plan.Tasks[0].Ready {
		t.Fatalf("create task = %#v, %v", plan, err)
	}
	task := plan.Tasks[0]
	plan, err = store.AssignMigrationTask(plan.ID, task.ID, "consumer-owner", "", "human", "consumer-dev", "migrate checkout", "base")
	if err != nil || plan.Tasks[0].Assignment.AssignedByID != "consumer-owner" {
		t.Fatalf("consumer assignment = %#v, %v", plan.Tasks[0], err)
	}
	assignment := plan.Tasks[0].Assignment
	plan, err = store.StartMigrationTask(plan.ID, task.ID, "consumer-dev", assignment.ID, "consumer-fork", "migration/payments-v2", "base", "session")
	if err != nil || plan.Tasks[0].Work.RepositoryID != "consumer-fork" || plan.Tasks[0].Status != "in_progress" {
		t.Fatalf("start = %#v, %v", plan.Tasks[0], err)
	}
	plan, err = store.SynchronizeMigrationTask(plan.ID, task.ID, "consumer-dev", "head", "pull", true)
	if err != nil || plan.Tasks[0].Status != "completed" || plan.Tasks[0].Work.PullRequestID != "pull" {
		t.Fatalf("complete = %#v, %v", plan.Tasks[0], err)
	}
}
