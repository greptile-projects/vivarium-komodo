package agentevaluations

import (
	"testing"
	"time"
)

func TestOnboardingRequiresCleanAcceptedEvidenceAndExplicitHumanBoundary(t *testing.T) {
	s, _ := New(t.TempDir())
	s.now = func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) }
	suite, _ := s.Create("repo", "owner", suiteInput())
	trial, _ := s.Start("repo", "owner", TrialInput{SuiteID: suite.ID, SuiteVersion: 1, ProfileID: "profile", ProfileVersion: 2, ScenarioIDs: []string{"repair"}})
	trial, _ = s.Complete("repo", trial.ID, ResultInput{Outputs: map[string]string{"repair": "done"}, ToolActions: []ToolAction{{Tool: "shell", Action: "test", Allowed: true}}, Cost: 1, Currency: "USD", LatencyMS: 20})
	trial, _ = s.Decide("repo", trial.ID, "reviewer", DecisionInput{Verdict: "accept", Rationale: "passed", Criteria: []string{"maintainable"}})
	in := OnboardingInput{TrialIDs: []string{trial.ID}, ProfileID: "profile", ProfileVersion: 2, Roles: []string{"contributor"}, Resources: []string{"repository:repo", "tasks", "sessions", "workspaces", "delivery-teams", "stewardship-mandates"}, Actions: []string{"branches:write", "checks:run"}, DataBoundaries: []string{"repository content only", "no secrets"}, Budget: OnboardingBudget{MaximumCost: 25, Currency: "USD", MaximumRuns: 10}, Schedule: OnboardingSchedule{StartsAt: s.now(), ExpiresAt: s.now().Add(24 * time.Hour)}, RequiredApproverIDs: []string{"owner", "security-owner"}, OperatorAgreementRequired: true, HumanSponsorID: "sponsor", ConsequentialDecisions: []string{"merge", "release", "spend", "governance"}, ChangeReason: "evaluation passed"}
	x, err := s.CreateOnboarding("repository", "repo", "owner", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Preview.Blockers) != 3 || x.Preview.Subject == "" {
		t.Fatalf("preview=%+v", x.Preview)
	}
	if _, e := s.ActivateOnboarding("repository", "repo", x.ID, "owner", 1); e != ErrConflict {
		t.Fatalf("activated without gates: %v", e)
	}
	x, _ = s.DecideOnboarding("repository", "repo", x.ID, "owner", "approved", "bounded", 1)
	x, _ = s.DecideOnboarding("repository", "repo", x.ID, "security-owner", "approved", "policy checked", 1)
	x, _ = s.AgreeOnboarding("repository", "repo", x.ID, "operator", "terms accepted", 1)
	x, err = s.ActivateOnboarding("repository", "repo", x.ID, "owner", 1)
	if err != nil || x.State != "active" || x.Identity == "" {
		t.Fatalf("activation: %v %+v", err, x)
	}
	in.ExpectedVersion = 1
	in.Actions = []string{"checks:run"}
	in.ChangeReason = "narrow upgrade"
	x, err = s.ReviseOnboarding("repository", "repo", x.ID, "owner", in)
	if err != nil || x.State != "pending_upgrade" || x.CurrentVersion != 2 || len(x.Decisions) != 2 {
		t.Fatalf("upgrade: %v %+v", err, x)
	}
	if len(x.Preview.Blockers) != 3 {
		t.Fatalf("new version reused approvals: %+v", x.Preview)
	}
	x, _ = s.RevokeOnboarding("repository", "repo", x.ID, "owner", "operator unavailable")
	if x.State != "revoked" || x.RevocationReason == "" {
		t.Fatal("revocation not retained")
	}
}
