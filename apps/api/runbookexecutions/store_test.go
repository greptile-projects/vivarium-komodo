package runbookexecutions

import (
	"testing"
	"time"
)

func launch() LaunchInput {
	now := time.Now().UTC()
	return LaunchInput{IdempotencyKey: "alert-1:rb-1", RunbookID: "rb-1", RunbookVersion: 2, Origin: Origin{"alert", "alert-1", "7", "/alerts/alert-1#timeline", "participants"}, AffectedResources: []string{"service:api"}, SignalWindow: SignalWindow{now.Add(-time.Minute), now}, Context: []Context{{"release", "release-7", "commit-7", true, "participants", true}}, Preconditions: []Check{{"impact", true, "metric:errors", "confirmed"}}, Access: []Access{{"telemetry:read", "api", true, "policy:on-call"}}, MatchExplanation: []string{"exact service match"}, RehearsalID: "proof", RehearsalRevision: 3, RehearsalReady: true}
}
func TestLaunchDeduplicationAndBlocking(t *testing.T) {
	s, _ := New(t.TempDir())
	in := launch()
	x, e := s.Create("repo", "responder", in)
	if e != nil || x.State != "ready" {
		t.Fatalf("launch: %#v %v", x, e)
	}
	again, e := s.Create("repo", "responder", in)
	if e != nil || again.ID != x.ID {
		t.Fatalf("idempotency failed: %#v %v", again, e)
	}
	in.IdempotencyKey = "other"
	_, e = s.Create("repo", "responder", in)
	if e != ErrConflict {
		t.Fatalf("duplicate=%v", e)
	}
	in.IdempotencyKey = "blocked"
	in.Origin.ResourceID = "alert-2"
	in.RehearsalReady = false
	in.Context[0].Accessible = false
	in.Access[0].Granted = false
	blocked, e := s.Create("repo", "responder", in)
	if e != nil || blocked.State != "blocked" || len(blocked.Blockers) != 3 {
		t.Fatalf("blockers: %#v %v", blocked, e)
	}
}

func TestLiveProcedureEnforcesControlDelegationAndReceipts(t *testing.T) {
	s, _ := New(t.TempDir())
	in := launch()
	in.ActivePath = []ProcedureStep{
		{ID: "inspect", Kind: "diagnostic", ExpectedEvidence: []string{"errors"}, RequiredAuthority: []string{"telemetry:read"}},
		{ID: "rollback", Kind: "action", DependsOn: []string{"inspect"}, ExpectedEvidence: []string{"release"}, RequiredAuthority: []string{"deployment:rollback"}, RollbackCriteria: []string{"health worsens"}},
		{ID: "notify", Kind: "action", DependsOn: []string{"rollback"}, Optional: true, PolicyPermitsSkip: true},
	}
	x, err := s.Create("repo", "commander", in)
	if err != nil || x.State != "active" || x.PredictedNextAction == "" || len(x.Steps) != 3 {
		t.Fatalf("live launch: %#v %v", x, err)
	}
	x, err = s.Control("repo", x.ID, "reviewer", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "join-reviewer", Action: "join", ActorKind: "human"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Control("repo", x.ID, "agent-1", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "agent-before-delegation", Action: "perform", StepID: "inspect", Evidence: []string{"metric:window"}, Health: "degraded"}); err != ErrForbidden {
		t.Fatalf("undelegated agent=%v", err)
	}
	x, err = s.Control("repo", x.ID, "commander", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "delegate-analysis", Action: "delegate", StepID: "inspect", TargetID: "agent-1", Mode: "analyze"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Control("repo", x.ID, "agent-1", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "analysis-cannot-effect", Action: "perform", StepID: "inspect", Evidence: []string{"metric:window"}, Health: "degraded"}); err != ErrForbidden {
		t.Fatalf("analysis delegation=%v", err)
	}
	x, err = s.Control("repo", x.ID, "commander", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "delegate-execute", Action: "delegate", StepID: "inspect", TargetID: "agent-1", Mode: "execute"})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Control("repo", x.ID, "reviewer", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "approve-inspect", Action: "approve", StepID: "inspect", Body: "current evidence reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Control("repo", x.ID, "agent-1", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "perform-inspect", Action: "perform", StepID: "inspect", Evidence: []string{"metric:window"}, Health: "degraded", Cost: 1.25})
	if err != nil || len(x.Credentials) != 1 || x.Credentials[0].SecretRetained || len(x.ActionReceipts) != 5 || x.Cost != 1.25 {
		t.Fatalf("performed: %#v %v", x, err)
	}
	retry, err := s.Control("repo", x.ID, "agent-1", ControlInput{ExpectedRevision: 1, IdempotencyKey: "perform-inspect", Action: "perform", StepID: "inspect", Evidence: []string{"different"}, Health: "healthy"})
	if err != nil || retry.Revision != x.Revision || len(retry.ActionReceipts) != len(x.ActionReceipts) {
		t.Fatalf("retry repeated effect: %#v %v", retry, err)
	}
	if _, err = s.Control("repo", x.ID, "commander", ControlInput{ExpectedRevision: x.Revision - 1, IdempotencyKey: "stale", Action: "pause"}); err != ErrConflict {
		t.Fatalf("stale=%v", err)
	}
	x, err = s.Control("repo", x.ID, "reviewer", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "approve-rollback", Action: "approve", StepID: "rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Control("repo", x.ID, "reviewer", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "same-person-effect", Action: "perform", StepID: "rollback", Evidence: []string{"release:old"}, Health: "healthy"}); err != ErrForbidden {
		t.Fatalf("separation=%v", err)
	}
	x, err = s.Control("repo", x.ID, "commander", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "perform-rollback", Action: "perform", StepID: "rollback", Evidence: []string{"release:old"}, Health: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Control("repo", x.ID, "commander", ControlInput{ExpectedRevision: x.Revision, IdempotencyKey: "skip-notify", Action: "skip", StepID: "notify", Body: "policy allows omission"})
	if err != nil || x.State != "completed" || x.PredictedNextAction == "" {
		t.Fatalf("complete: %#v %v", x, err)
	}
}
