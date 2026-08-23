package workflowdefinitions

import (
	"errors"
	"testing"
	"time"
)

func executableDefinition() Input {
	in := definition()
	in.MaximumConcurrency = 1
	in.RateLimit = RateLimit{MaximumInvocations: 2, WindowSeconds: 60}
	in.Steps = append(in.Steps, Step{ID: "publish", Name: "Publish draft", Needs: []string{"draft"}, Inputs: map[string]string{"proposal": "steps.draft.proposal"}, Invocation: Invocation{Kind: "platform_action", Reference: "proposal.create", Revision: "v2", Accessible: true, OwnerIDs: []string{"owner"}, Capabilities: []string{"proposal:create"}}, Retry: Retry{MaximumAttempts: 1}, TimeoutSeconds: 60, MaximumCost: 2, CompletionCriteria: []string{"proposal created"}})
	in.MaximumCost = 5
	return in
}

func invokeAt(workflow string, now time.Time) InvokeInput {
	return InvokeInput{IdempotencyKey: "event-1", WorkflowVersion: 1, Event: TriggeringEvent{ID: "issue-7", Type: "repository_event", Name: "issue.accepted", Revision: "event-revision-1", OccurredAt: now.Add(-time.Second)}, Actor: ExecutionActor{Kind: "human", ID: "owner"}, Inputs: map[string]any{"issue": "7"}, PermittedResources: []ResourceRevision{{Kind: "approved_agent", Reference: "repair-agent", Revision: "v1"}, {Kind: "platform_action", Reference: "proposal.create", Revision: "v2"}}, Policy: PolicyDecision{Repository: "allowed", Organization: "allowed", Agent: "allowed", Embargo: "allowed", Environment: "allowed", Approval: "allowed"}}
}

func TestDurableExecutionSchedulesDependenciesAndContainsOutputs(t *testing.T) {
	now := time.Date(2026, 8, 23, 5, 0, 0, 0, time.UTC)
	root := t.TempDir()
	s, _ := New(root)
	s.now = func() time.Time { return now }
	w, _ := s.Create("repo", "owner", executableDefinition())
	_, _ = s.Activate("repo", w.ID, "owner", 1)
	x, err := s.Invoke("repo", w.ID, "owner", invokeAt(w.ID, now))
	if err != nil || x.State != "running" || x.WorkflowRepositoryRevision == "" || x.Steps[0].State != "ready" || x.Steps[1].State != "pending" {
		t.Fatalf("invoke: %#v %v", x, err)
	}
	duplicate, err := s.Invoke("repo", w.ID, "owner", invokeAt(w.ID, now))
	if err != nil || duplicate.ID != x.ID {
		t.Fatalf("duplicate event must return durable original: %#v %v", duplicate, err)
	}
	x, err = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-attempt-1", CredentialExpiresAt: now.Add(5 * time.Minute)})
	if err != nil || x.Steps[0].Credential == nil || x.Steps[0].Credential.SecretRetained || x.Steps[0].Credential.Subject == "owner" {
		t.Fatalf("scoped credential: %#v %v", x.Steps[0].Credential, err)
	}
	cred := x.Steps[0].Credential.Reference
	x, err = s.RecordResult("repo", w.ID, x.ID, "runner", "draft", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-attempt-1", CredentialReference: cred, State: "succeeded", Cost: 1, Outputs: map[string]OutputValue{"proposal": {Value: "proposal-9", Accessible: true}, "token": {Value: "access_token=leak", Accessible: true, Secret: true}}})
	if err != nil || x.Steps[1].State != "ready" || x.Steps[1].AvailableInputs["draft.proposal"].Value != "proposal-9" || len(x.Steps[0].Attempts[0].Outputs) != 1 || x.Steps[0].Credential.RevokedAt == nil {
		t.Fatalf("bounded output scheduling: %#v %v", x, err)
	}
	// A process restart reads the same exact execution and can continue it.
	restarted, _ := New(root)
	restarted.now = func() time.Time { return now.Add(time.Minute) }
	got, err := restarted.GetExecution("repo", w.ID, x.ID)
	if err != nil || got.Revision != x.Revision || got.Steps[1].State != "ready" {
		t.Fatalf("durable restart: %#v %v", got, err)
	}
}

func TestExecutionRetryLimitsPolicyAndDeterministicBlocks(t *testing.T) {
	now := time.Date(2026, 8, 23, 6, 0, 0, 0, time.UTC)
	s, _ := New(t.TempDir())
	s.now = func() time.Time { return now }
	w, _ := s.Create("repo", "owner", executableDefinition())
	_, _ = s.Activate("repo", w.ID, "owner", 1)
	denied := invokeAt(w.ID, now)
	denied.IdempotencyKey = "denied"
	denied.Event.ID = "denied-event"
	denied.Policy.Environment = "denied"
	blocked, err := s.Invoke("repo", w.ID, "owner", denied)
	if err != nil || blocked.State != "blocked" || blocked.Blockers[0] != "policy_denied" {
		t.Fatalf("policy decision must be durable: %#v %v", blocked, err)
	}
	x, _ := s.Invoke("repo", w.ID, "owner", invokeAt(w.ID, now))
	parallel := invokeAt(w.ID, now)
	parallel.IdempotencyKey, parallel.Event.ID = "event-2", "issue-8"
	if _, err = s.Invoke("repo", w.ID, "owner", parallel); !errors.Is(err, ErrBlocked) {
		t.Fatalf("concurrency must block, got %v", err)
	}
	x, _ = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "try-1", CredentialExpiresAt: now.Add(time.Minute)})
	x, _ = s.RecordResult("repo", w.ID, x.ID, "runner", "draft", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: "try-1", CredentialReference: x.Steps[0].Credential.Reference, State: "interrupted", Failure: "runner outage"})
	if x.Steps[0].State != "retry_ready" || x.State != "running" {
		t.Fatalf("interruption must be safely resumable: %#v", x)
	}
	x, _ = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "retry", StepID: "draft", Reason: "resume retained work"})
	x, _ = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "try-2", CredentialExpiresAt: now.Add(time.Minute)})
	x, _ = s.RecordResult("repo", w.ID, x.ID, "runner", "draft", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: "try-2", CredentialReference: x.Steps[0].Credential.Reference, State: "failed", Failure: "deterministic failure"})
	if x.State != "failed" || x.Steps[0].State != "failed" {
		t.Fatalf("retry exhaustion: %#v", x)
	}
	limited := invokeAt(w.ID, now)
	limited.IdempotencyKey, limited.Event.ID = "event-3", "issue-9"
	if _, err = s.Invoke("repo", w.ID, "owner", limited); !errors.Is(err, ErrBlocked) {
		t.Fatalf("rate limit must block, got %v", err)
	}
}

func TestStaleAccessAndCancellationRevokeStepIdentity(t *testing.T) {
	now := time.Date(2026, 8, 23, 7, 0, 0, 0, time.UTC)
	s, _ := New(t.TempDir())
	s.now = func() time.Time { return now }
	w, _ := s.Create("repo", "owner", executableDefinition())
	_, _ = s.Activate("repo", w.ID, "owner", 1)
	x, _ := s.Invoke("repo", w.ID, "owner", invokeAt(w.ID, now))
	x, _ = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "try", CredentialExpiresAt: now.Add(time.Minute)})
	x, _ = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "revoke_access", Reason: "agent permission revoked"})
	if x.State != "blocked" || x.Steps[0].Credential.RevokedAt == nil || x.Blockers[0] != "revoke_access" {
		t.Fatalf("revocation: %#v", x)
	}
	policy := PolicyDecision{Repository: "allowed", Organization: "allowed", Agent: "allowed", Embargo: "allowed", Environment: "allowed", Approval: "allowed"}
	x, _ = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "resume", Reason: "authority revalidated", Policy: &policy})
	x, _ = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "cancel", Reason: "event superseded"})
	if x.State != "cancelled" || x.CompletedAt == nil {
		t.Fatalf("cancel: %#v", x)
	}
}
