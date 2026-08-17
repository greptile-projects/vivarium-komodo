package agentevaluations

import (
	"testing"
	"time"
)

func activeOnboarding(t *testing.T, s *Store, trial Trial, suffix string) Onboarding {
	t.Helper()
	in := OnboardingInput{TrialIDs: []string{trial.ID}, ProfileID: "profile", ProfileVersion: 2, Roles: []string{"contributor"}, Resources: []string{"repository:repo", "tasks"}, Actions: []string{"branches:write", "checks:run"}, DataBoundaries: []string{"repository content only"}, Budget: OnboardingBudget{MaximumCost: 25, Currency: "USD", MaximumRuns: 10}, Schedule: OnboardingSchedule{StartsAt: s.now(), ExpiresAt: s.now().Add(48 * time.Hour)}, RequiredApproverIDs: []string{"owner"}, OperatorAgreementRequired: true, HumanSponsorID: "sponsor", ConsequentialDecisions: []string{"merge"}, ChangeReason: "evaluation passed " + suffix}
	x, e := s.CreateOnboarding("repository", "repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	x, _ = s.DecideOnboarding("repository", "repo", x.ID, "owner", "approved", "bounded", 1)
	x, _ = s.AgreeOnboarding("repository", "repo", x.ID, "operator", "accepted", 1)
	x, e = s.ActivateOnboarding("repository", "repo", x.ID, "owner", 1)
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestOperationalTrustContainsFailureAndPreservesReplacementHandoff(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	suite, _ := s.Create("repo", "owner", suiteInput())
	trial, _ := s.Start("repo", "owner", TrialInput{SuiteID: suite.ID, SuiteVersion: 1, ProfileID: "profile", ProfileVersion: 2, ScenarioIDs: []string{"repair"}})
	trial, _ = s.Complete("repo", trial.ID, ResultInput{Outputs: map[string]string{"repair": "done"}, ToolActions: []ToolAction{{Tool: "shell", Action: "test", Allowed: true}}, Cost: 1, Currency: "USD", LatencyMS: 20})
	trial, _ = s.Decide("repo", trial.ID, "reviewer", DecisionInput{Verdict: "accept", Rationale: "passed", Criteria: []string{"maintainable"}})
	x := activeOnboarding(t, s, trial, "primary")
	replacement := activeOnboarding(t, s, trial, "replacement")
	x, e := s.SetTrustPolicy("repository", "repo", x.ID, "owner", ReevaluationPolicy{IntervalDays: 30, RequiredSuiteID: suite.ID, SuspendOnFailure: true, MaximumVerificationFailureRate: .25, MaximumAverageCost: 5, Currency: "USD", ExpectedVersion: x.Trust.Version})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.RecordOutcome("repository", "repo", x.ID, "reviewer", OutcomeInput{Kind: "security_violation", WorkKind: "pull_request", WorkID: "pr-7", Summary: "Attempted a prohibited network action", Evidence: []EvidenceReference{{Kind: "check_run", ID: "check-9", Revision: "abc"}}, Cost: 8, Currency: "USD", ResponsivenessMS: 1200, OccurredAt: now})
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Trust.Notices) == 0 || x.Trust.Outcomes[0].RecordedBy != "reviewer" {
		t.Fatalf("missing attributable notice: %+v", x.Trust)
	}
	failing, _ := s.Start("repo", "owner", TrialInput{SuiteID: suite.ID, SuiteVersion: 1, ProfileID: "profile", ProfileVersion: 2, ScenarioIDs: []string{"repair"}})
	failing, _ = s.Complete("repo", failing.ID, ResultInput{Outputs: map[string]string{"repair": "wrong"}, ToolActions: []ToolAction{{Tool: "shell", Action: "test", Allowed: true}}, Cost: 2, Currency: "USD", LatencyMS: 30, Failure: "hidden verification failed"})
	x, e = s.RecordReevaluation("repository", "repo", x.ID, "owner", failing.ID, "failed", "correctness regressed", 2)
	if e != nil {
		t.Fatal(e)
	}
	if x.Trust.AuthorityStatus != "suspended" {
		t.Fatalf("failed reevaluation retained authority: %+v", x.Trust)
	}
	x, e = s.CreateHandoff("repository", "repo", x.ID, "owner", HandoffInput{WorkKind: "proposal_task", WorkID: "task-4", ReplacementOnboardingID: replacement.ID, Summary: "Transfer the active repair without private logs", Completed: []EvidenceReference{{Kind: "commit", ID: "abc"}}, Remaining: []string{"address review correction"}, VerificationCriteria: []string{"checks pass"}, ResidualRisks: []string{"migration edge case"}, ExpectedVersion: x.Trust.Version})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.AcceptHandoff("repository", "repo", x.ID, x.Trust.Handoffs[0].ID, "replacement-sponsor", "verified commit and remaining scope", x.Trust.Version)
	if e != nil {
		t.Fatal(e)
	}
	if x.Trust.Handoffs[0].State != "accepted" || x.Trust.Handoffs[0].ToIdentity != replacement.Identity {
		t.Fatalf("handoff not preserved: %+v", x.Trust.Handoffs)
	}
	x, e = s.ControlAuthority("repository", "repo", x.ID, "owner", "revoke", "replacement accepted", nil, nil, x.Trust.Version)
	if e != nil || x.Trust.AuthorityStatus != "revoked" || len(x.Trust.Outcomes) != 1 {
		t.Fatalf("revocation erased evidence: %v %+v", e, x)
	}
}
