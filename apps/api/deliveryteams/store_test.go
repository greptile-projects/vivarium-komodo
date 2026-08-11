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

func TestGovernedTimelineAndVerifiableHandoff(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create("repo", "Handoff team", "lead", Outcome{Kind: "planned_outcome", Title: "Continue verified work"}, CharterInput{Outcome: "Ship", SuccessMeasures: []string{"verified"}, OperatingPrinciples: []string{"cite evidence"}, TotalBudget: Budget{Hours: 20}, DefaultEscalation: "lead"})
	if err != nil {
		t.Fatal(err)
	}
	for _, principal := range []string{"dev", "reviewer"} {
		v, err = s.Invite("repo", v.ID, "lead", v.Version, ParticipantInput{Kind: "human", PrincipalID: principal, Role: principal, Why: "needed", Responsibilities: []string{"continue work"}, Budget: Budget{Hours: 10}, RequestedActions: []string{"contents:read"}, Access: AccessPreview{Actions: []string{"contents:read"}}})
		if err != nil {
			t.Fatal(err)
		}
		v, err = s.Respond("repo", v.ID, v.Participants[len(v.Participants)-1].ID, principal, "accepted", v.Version)
		if err != nil {
			t.Fatal(err)
		}
	}
	revision := "1111111111111111111111111111111111111111"
	work := WorkStream{ID: "api", Title: "API", OwnerParticipantID: v.Participants[0].ID, Inputs: []string{"contract"}, ExpectedArtifacts: []string{"handler"}, AcceptanceCriteria: []string{"tests pass"}, RepositoryScope: []RepositoryScope{{RepositoryID: "repo", CommitID: revision, Paths: []string{"apps/api"}, RequiredActions: []string{"contents:read"}}}, IntegrationOrder: 1, Budget: Budget{Hours: 8}}
	v, err = s.ProposePlan("repo", v.ID, "dev", v.Participants[0].ID, v.Version, PlanInput{Streams: []WorkStream{work}, ChangeReason: "start"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Plan.Current.Status != "accepted" {
		t.Fatalf("plan = %#v", v.Plan.Current)
	}
	context := ExecutionContext{Kind: "workspace", ID: "workspace-1", RepositoryID: "repo", Revision: revision}
	v, err = s.AttachExecution("repo", v.ID, "dev", v.Participants[0].ID, v.Version, context, "api")
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.PublishTimeline("repo", v.ID, "dev", v.Participants[0].ID, v.Version, TimelineInput{StreamID: "api", Kind: "finding", Summary: "The handler needs an explicit conflict guard.", Context: context, Citations: []Citation{{RepositoryID: "repo", Revision: revision, Path: "apps/api/handler.go", ResourceKind: "blob", ResourceID: "blob-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	entryID := v.Timeline[0].ID
	v, err = s.RequestHandoff("repo", v.ID, "dev", v.Participants[0].ID, v.Version, HandoffInput{StreamID: "api", ToParticipantID: v.Participants[1].ID, InputEntryIDs: []string{entryID}, Context: context, AcceptanceCriteria: []string{"reproduce the conflict"}, ResidualUncertainty: []string{"behavior on an empty branch is unknown"}})
	if err != nil {
		t.Fatal(err)
	}
	h := v.Handoffs[0]
	if h.InputRevisions[0] != revision || h.ResidualUncertainty[0] == "" || h.Status != "requested" {
		t.Fatalf("handoff = %#v", h)
	}
	v, err = s.AcceptHandoff("repo", v.ID, h.ID, "reviewer", v.Participants[1].ID, "I verified the cited revision and can reproduce it.", v.Version)
	if err != nil {
		t.Fatal(err)
	}
	if v.Handoffs[0].Status != "accepted" || v.Handoffs[0].Acceptance.ActorID != "reviewer" {
		t.Fatalf("acceptance = %#v", v.Handoffs[0])
	}
	blocked, err := s.Reconcile("repo", v.ID, "dev", v.Version, IntegrationInput{Contributions: []Contribution{{StreamID: "api", SourceBranch: "team/api", SourceCommitID: revision, TargetBranch: "main", TargetCommitID: revision, Title: "Deliver API", Summary: "Conflict-safe handler", EvidenceEntryIDs: []string{entryID}, HandoffIDs: []string{h.ID}, Criteria: []Criterion{{Criterion: "tests pass", Status: "unmet"}}, Conflict: true}}})
	if err != nil || blocked.Integrations[0].Status != "blocked" || len(blocked.Integrations[0].Blockers) != 2 {
		t.Fatalf("blocked reconciliation = %v %#v", err, blocked.Integrations)
	}
	v, err = s.Reconcile("repo", v.ID, "dev", blocked.Version, IntegrationInput{Contributions: []Contribution{{StreamID: "api", SourceBranch: "team/api", SourceCommitID: revision, TargetBranch: "main", TargetCommitID: revision, Title: "Deliver API", Summary: "Conflict-safe handler", EvidenceEntryIDs: []string{entryID}, HandoffIDs: []string{h.ID}, Criteria: []Criterion{{Criterion: "tests pass", Status: "met", EvidenceEntryIDs: []string{entryID}}}, ResidualRisks: []string{"empty branches remain unverified"}}}})
	if err != nil || v.Integrations[1].Status != "ready" {
		t.Fatalf("ready reconciliation = %v %#v", err, v.Integrations)
	}
	v, err = s.LinkPullRequest("repo", v.ID, v.Integrations[1].ID, "api", "dev", "pull-1", v.Version)
	if err != nil || v.Integrations[1].Contributions[0].PullRequestID != "pull-1" || v.Integrations[1].Contributions[0].PublishedByID != "dev" {
		t.Fatalf("published contribution = %v %#v", err, v.Integrations[1])
	}
	_, err = s.PublishTimeline("repo", v.ID, "dev", v.Participants[0].ID, v.Version, TimelineInput{StreamID: "api", Kind: "artifact", Summary: "secret", Context: context, Citations: []Citation{{RepositoryID: "repo", Revision: revision, Path: ".env", ResourceKind: "blob", ResourceID: "secret"}}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-scope evidence should fail, got %v", err)
	}
	_, err = s.PublishTimeline("repo", v.ID, "dev", v.Participants[0].ID, v.Version, TimelineInput{StreamID: "api", Kind: "artifact", Summary: "other repository", Context: context, Citations: []Citation{{RepositoryID: "private-repo", Revision: revision, Path: "apps/api/handler.go", ResourceKind: "blob", ResourceID: "private"}}})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("inaccessible evidence should fail closed, got %v", err)
	}
}

func TestRunningTeamSurfacesInterventionAndBoundedRecovery(t *testing.T) {
	s, _ := New(t.TempDir())
	v, err := s.Create("repo", "Running team", "lead", Outcome{Kind: "planned_outcome", Title: "Recover safely"}, CharterInput{Outcome: "Ship", SuccessMeasures: []string{"verified"}, OperatingPrinciples: []string{"bounded recovery"}, TotalBudget: Budget{Hours: 10, AgentRuns: 2}, DefaultEscalation: "lead"})
	if err != nil {
		t.Fatal(err)
	}
	for _, principal := range []string{"agent-one", "backup"} {
		kind := "human"
		if principal == "agent-one" {
			kind = "agent"
		}
		v, err = s.Invite("repo", v.ID, "lead", v.Version, ParticipantInput{Kind: kind, PrincipalID: principal, Role: principal, Why: "operate stream", Responsibilities: []string{"deliver"}, Budget: Budget{Hours: 8, AgentRuns: 2}, RequestedActions: []string{"contents:read"}, Access: AccessPreview{Actions: []string{"contents:read"}}})
		if err != nil {
			t.Fatal(err)
		}
		v, err = s.Respond("repo", v.ID, v.Participants[len(v.Participants)-1].ID, "lead", "accepted", v.Version)
		if err != nil {
			t.Fatal(err)
		}
	}
	revision := "1111111111111111111111111111111111111111"
	work := WorkStream{ID: "api", Title: "API", OwnerParticipantID: v.Participants[0].ID, Inputs: []string{"contract"}, ExpectedArtifacts: []string{"patch"}, AcceptanceCriteria: []string{"tests"}, RepositoryScope: []RepositoryScope{{RepositoryID: "repo", CommitID: revision, Paths: []string{"apps/api"}, RequiredActions: []string{"contents:read"}}}, IntegrationOrder: 1, Budget: Budget{Hours: 4, AgentRuns: 1}}
	v, err = s.ProposePlan("repo", v.ID, "lead", "", v.Version, PlanInput{Streams: []WorkStream{work}, ChangeReason: "execute"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.AcceptPlan("repo", v.ID, v.Participants[0].ID, "operator", v.Version, v.Plan.Current.Version)
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.ReportStream("repo", v.ID, "api", "operator", v.Participants[0].ID, v.Version, StreamStatusInput{ReportedRevision: revision, Status: "failed", ActiveAction: "run checks", Question: "Is the contract still current?", PredictedNextAction: "retry checks", ResourceUse: ResourceUse{Hours: 2, AgentRuns: 1}, AccessState: "active", OutputState: "clean"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Runtime.Status != "attention_required" {
		t.Fatalf("runtime response = %#v", v.Runtime)
	}
	v, err = s.Get("repo", v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Runtime.Status != "attention_required" || v.Runtime.Blockers[0].Kind != "failed_run" || v.Runtime.Blockers[0].Escalates {
		t.Fatalf("runtime = %#v", v.Runtime)
	}
	v, err = s.Control("repo", v.ID, "lead", v.Version, ControlInput{StreamID: "api", Action: "resume", Instruction: "Use the one bounded recovery attempt."})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.ReportStream("repo", v.ID, "api", "operator", v.Participants[0].ID, v.Version, StreamStatusInput{ReportedRevision: revision, Status: "failed", ActiveAction: "rerun checks", ResourceUse: ResourceUse{Hours: 5, AgentRuns: 2}, AccessState: "revoked", OutputState: "conflicting"})
	if err != nil {
		t.Fatal(err)
	}
	v, _ = s.Get("repo", v.ID)
	kinds := map[string]bool{}
	for _, b := range v.Runtime.Blockers {
		kinds[b.Kind] = true
	}
	for _, kind := range []string{"recovery_exhausted", "budget_exhausted", "access_revoked", "conflicting_output"} {
		if !kinds[kind] {
			t.Fatalf("missing %s in %#v", kind, v.Runtime.Blockers)
		}
	}
	beforeOwner := v.Plan.Current.Streams[0].OwnerParticipantID
	v, err = s.Control("repo", v.ID, "lead", v.Version, ControlInput{StreamID: "api", Action: "reassign", Instruction: "Preserve accepted output and continue within existing read scope.", TargetParticipantID: v.Participants[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if v.Plan.Current.Streams[0].OwnerParticipantID != beforeOwner || v.StreamRuns[0].OperationalOwnerID != v.Participants[1].ID || v.Controls[len(v.Controls)-1].ExpandsAuthority {
		t.Fatalf("reassignment rewrote authority: %#v", v)
	}
	v, err = s.ReportStream("repo", v.ID, "api", "backup", v.Participants[1].ID, v.Version, StreamStatusInput{ReportedRevision: revision, Status: "running", ActiveAction: "preserve and reconcile accepted output", ResourceUse: ResourceUse{Hours: 3, AgentRuns: 1}, AccessState: "active", OutputState: "clean"})
	if err != nil || v.StreamRuns[0].OperationalOwnerID != v.Participants[1].ID {
		t.Fatalf("operational reassignee could not report safely: %v %#v", err, v.StreamRuns)
	}
	v, err = s.Control("repo", v.ID, "lead", v.Version, ControlInput{Action: "pause", Instruction: "Pause the whole effort at a safe checkpoint."})
	if err != nil || v.StreamRuns[0].Status != "paused" {
		t.Fatalf("whole-team pause = %v %#v", err, v.StreamRuns)
	}
	if _, err = s.Control("repo", v.ID, "lead", v.Version, ControlInput{StreamID: "api", Action: "narrow", Instruction: "Limit recovery", NarrowedPaths: []string{"outside"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-scope narrowing = %v", err)
	}
}
