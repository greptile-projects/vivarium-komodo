package agentevaluations

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func releaseFixture(t *testing.T) (*Store, AgentReleaseInput) {
	t.Helper()
	s, c, now := pilotFixture(t)
	suite, _ := s.Create("repo", "owner", suiteInput())
	trial, _ := s.Start("repo", "owner", TrialInput{SuiteID: suite.ID, SuiteVersion: 1, ProfileID: "profile", ProfileVersion: 2, ScenarioIDs: []string{"repair"}})
	trial, _ = s.Complete("repo", trial.ID, ResultInput{Outputs: map[string]string{"repair": "done"}, ToolActions: []ToolAction{{Tool: "shell", Action: "test", Allowed: true}}, Cost: 1, Currency: "USD", LatencyMS: 20})
	trial, _ = s.Decide("repo", trial.ID, "reviewer", DecisionInput{Verdict: "accept", Rationale: "passed", Criteria: []string{"safe"}})
	on, e := s.CreateOnboarding("repository", "repo", "owner", OnboardingInput{TrialIDs: []string{trial.ID}, ProfileID: "profile", ProfileVersion: 2, Roles: []string{"contributor"}, Resources: []string{"repository:repo", "tasks"}, Actions: []string{"draft", "checks:run"}, DataBoundaries: []string{"repository content only"}, Budget: OnboardingBudget{MaximumCost: 20, Currency: "USD", MaximumRuns: 10}, Schedule: OnboardingSchedule{StartsAt: now, ExpiresAt: now.Add(24 * time.Hour)}, RequiredApproverIDs: []string{"owner"}, OperatorAgreementRequired: true, HumanSponsorID: "sponsor", ConsequentialDecisions: []string{"merge"}, ChangeReason: "accepted evidence"})
	if e != nil {
		t.Fatal(e)
	}
	on, _ = s.DecideOnboarding("repository", "repo", on.ID, "owner", "approved", "bounded", 1)
	on, _ = s.AgreeOnboarding("repository", "repo", on.ID, "operator", "release terms", 1)
	on, e = s.ActivateOnboarding("repository", "repo", on.ID, "owner", 1)
	if e != nil {
		t.Fatal(e)
	}
	p := createPilotFixture(t, s, c, now)
	p, _ = s.SetPilotConsent("repo", p.ID, "user", "accepted", "")
	p, _ = s.StartPilotSession("repo", p.ID, "user", PilotSessionInput{RepositoryID: "repo", Role: "reviewer", Task: "triage issue"})
	p, e = s.RecordPilotFeedback("repo", p.ID, "user", PilotFeedbackInput{SessionID: p.Sessions[0].ID, CandidateRevision: p.CandidateRevision, Kind: "feedback", Summary: "accepted in bounded work", ExpectedOutcome: "useful draft"})
	if e != nil {
		t.Fatal(e)
	}
	return s, AgentReleaseInput{OnboardingID: on.ID, TrialIDs: []string{trial.ID}, PilotID: p.ID, BehaviorContractID: "agent-project", BehaviorVersion: 3, RepositoryRevision: "rev-1", ModelVersion: "model@2026-08", ToolVersions: []string{"shell@2", "git@1"}, OperatorTerms: "release terms", ChangeReason: "accepted candidate"}
}

func TestAgentReleaseGatesAttestationDeploymentAndRollback(t *testing.T) {
	s, in := releaseFixture(t)
	x, e := s.CreateRelease("repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Blockers) != 4 {
		t.Fatalf("missing release gates: %#v", x.Blockers)
	}
	if _, e = s.PublishRelease("repo", x.ID, "owner"); !errors.Is(e, ErrConflict) {
		t.Fatalf("published without gates: %v", e)
	}
	for _, kind := range releaseApprovalKinds {
		x, e = s.DecideRelease("repo", x.ID, kind+"-owner", kind, "approved", "current evidence reviewed")
		if e != nil {
			t.Fatal(e)
		}
	}
	x, e = s.PublishRelease("repo", x.ID, "owner")
	if e != nil || x.State != "attested" || !strings.HasPrefix(x.AttestationDigest, "sha256:") {
		t.Fatalf("publication: %v %#v", e, x)
	}
	dep := AgentDeploymentInput{Roles: []string{"contributor"}, Resources: []string{"repository:repo"}, Actions: []string{"draft"}, CredentialReferences: []string{"credential-ref:scoped"}, MaximumCost: 10, Currency: "USD", MaximumLatencyMS: 2000}
	x, e = s.DeployRelease("repo", x.ID, "owner", dep)
	if e != nil || len(x.Deployments) != 1 {
		t.Fatalf("deployment: %v %#v", e, x)
	}
	if _, e = s.DeployRelease("repo", x.ID, "owner", AgentDeploymentInput{Roles: dep.Roles, Resources: []string{"production"}, Actions: dep.Actions, CredentialReferences: dep.CredentialReferences, MaximumCost: 1, Currency: "USD", MaximumLatencyMS: 1}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("broad authority accepted: %v", e)
	}
}

func TestReleaseRegressionControlsRetainSignalsAndConnectedWork(t *testing.T) {
	s, in := releaseFixture(t)
	var e error
	x, _ := s.CreateRelease("repo", "owner", in)
	for _, k := range releaseApprovalKinds {
		x, _ = s.DecideRelease("repo", x.ID, k+"-owner", k, "approved", "reviewed")
	}
	x, _ = s.PublishRelease("repo", x.ID, "owner")
	x, _ = s.DeployRelease("repo", x.ID, "owner", AgentDeploymentInput{Roles: []string{"contributor"}, Resources: []string{"repository:repo"}, Actions: []string{"draft"}, CredentialReferences: []string{"credential-ref:scoped"}, MaximumCost: 10, Currency: "USD", MaximumLatencyMS: 2000})
	did := x.Deployments[0].ID
	x, e = s.RecordReleaseSignal("repo", x.ID, did, "monitor", ReleaseSignal{Kind: "safety", Summary: "prohibited action was contained", LatencyMS: 50, Evidence: []EvidenceReference{{Kind: "trial", ID: "reproduction"}}})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.ControlDeployment("repo", x.ID, did, "owner", "pause", "safety regression", "", "", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.ControlDeployment("repo", x.ID, did, "owner", "reopen_finding", "reproduce privately", "private_finding", "finding-7", "security-owner", 2)
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.ControlDeployment("repo", x.ID, did, "owner", "create_repair", "bounded repair", "agent_task", "task-9", "repair-agent", 3)
	if e != nil {
		t.Fatal(e)
	}
	if x.Deployments[0].State != "paused" || len(x.Deployments[0].Signals) != 1 || len(x.Deployments[0].Controls) != 3 {
		t.Fatalf("regression history lost: %#v", x.Deployments[0])
	}
}
