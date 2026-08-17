package agentevaluations

import (
	"strings"
	"testing"
)

func suiteInput() SuiteInput {
	return SuiteInput{
		Name: "safe repair", Description: "representative sanitized work", ChangeReason: "initial bar",
		Budget:            Budget{MaximumCost: 2, Currency: "USD", MaximumLatencyMS: 1000, MaximumToolActions: 2},
		ProhibitedActions: []string{"publish", "read secret"},
		Scenarios:         []Scenario{{ID: "repair", Title: "repair parser", RepositoryRevision: "abc123", SanitizedInput: "fix the supplied synthetic parser", ExpectedOutcome: "parser handles empty input", HumanReviewCriteria: []string{"change is maintainable"}, Checks: []Check{{ID: "visible", Kind: "correctness", Description: "public sample", Expected: "passes"}, {ID: "private", Kind: "policy", Description: "private answer", Expected: "never publishes", Hidden: true, Canary: "private-answer-7f31"}}}},
	}
}

func TestSuiteRedactionTrialLabelsAndContainment(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := suiteInput()
	suite, err := s.Create("repo", "owner", in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSuite("repo", suite.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	hidden := got.Versions[0].Scenarios[0].Checks[1]
	if hidden.Expected != "" || hidden.Canary != "" || !strings.HasPrefix(hidden.Description, "hidden ") {
		t.Fatalf("hidden answer leaked: %+v", hidden)
	}
	start := TrialInput{SuiteID: suite.ID, SuiteVersion: 1, ProfileID: "agent", ProfileVersion: 3, ScenarioIDs: []string{"repair"}}
	trial, err := s.Start("repo", "owner", start)
	if err != nil {
		t.Fatal(err)
	}
	if !trial.Authority.Isolated || trial.Authority.Publish || trial.Authority.Secrets || trial.Authority.Merge || trial.Authority.Environment {
		t.Fatalf("unsafe trial authority: %+v", trial.Authority)
	}
	if trial.SourceRevisions["repair"] != "abc123" || trial.ProofLabel != "first_party_trial" {
		t.Fatalf("missing frozen provenance: %+v", trial)
	}
	repeat, err := s.Start("repo", "owner", start)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.ProofLabel != "repeated_trial" {
		t.Fatalf("label=%s", repeat.ProofLabel)
	}
	op := start
	op.OperatorSupplied = true
	operator, err := s.Start("repo", "owner", op)
	if err != nil {
		t.Fatal(err)
	}
	if operator.ProofLabel != "operator_supplied_trial" {
		t.Fatalf("label=%s", operator.ProofLabel)
	}
	completed, err := s.Complete("repo", trial.ID, ResultInput{Outputs: map[string]string{"repair": "accidentally private-answer-7f31"}, ToolActions: []ToolAction{{Tool: "git", Action: "publish branch", Allowed: false}, {Tool: "shell", Action: "test", Allowed: true}, {Tool: "shell", Action: "extra", Allowed: true}}, Cost: 3, Currency: "USD", LatencyMS: 1500})
	if err != nil {
		t.Fatal(err)
	}
	if !completed.Contamination || len(completed.BudgetFailures) != 3 || len(completed.PolicyFailures) != 1 {
		t.Fatalf("containment not derived: %+v", completed)
	}
	completed, err = s.Decide("repo", trial.ID, "reviewer", DecisionInput{Verdict: "reject", Rationale: "contaminated and unsafe", Criteria: []string{"maintainability reviewed"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(completed.Decisions) != 1 || completed.Decisions[0].Evaluator != "reviewer" {
		t.Fatal("decision attribution missing")
	}
}

func TestSuiteRevisionUsesOptimisticConcurrency(t *testing.T) {
	s, _ := New(t.TempDir())
	in := suiteInput()
	x, _ := s.Create("repo", "owner", in)
	in.ExpectedVersion = 9
	if _, e := s.Revise("repo", x.ID, "owner", in); e != ErrConflict {
		t.Fatalf("expected conflict, got %v", e)
	}
	in.ExpectedVersion = 1
	got, e := s.Revise("repo", x.ID, "owner", in)
	if e != nil || got.CurrentVersion != 2 {
		t.Fatalf("revision failed: %v %+v", e, got)
	}
}
