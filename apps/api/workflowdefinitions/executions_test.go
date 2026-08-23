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

func TestExecutionGraphRetainsEvidenceAndSafeCollaboratorInterventions(t *testing.T) {
	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	s, _ := New(t.TempDir())
	s.now = func() time.Time { return now }
	in := executableDefinition()
	in.Steps[1].Optional = true
	in.Steps = append(in.Steps, Step{ID: "review", Name: "Human review", Needs: []string{"publish"}, Invocation: Invocation{Kind: "manual", Reference: "owner-review", Revision: "v1", Accessible: true, OwnerIDs: []string{"owner"}}, Retry: Retry{MaximumAttempts: 1}, TimeoutSeconds: 300, CompletionCriteria: []string{"review recorded"}})
	w, err := s.Create("repo", "owner", in)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = s.Activate("repo", w.ID, "owner", 1)
	invocation := invokeAt(w.ID, now)
	invocation.PermittedResources = append(invocation.PermittedResources, ResourceRevision{Kind: "manual", Reference: "owner-review", Revision: "v1"})
	x, err := s.Invoke("repo", w.ID, "owner", invocation)
	if err != nil {
		t.Fatal(err)
	}
	x, _ = s.Control("repo", w.ID, x.ID, "maintainer", ControlInput{ExpectedRevision: x.Revision, Action: "pause", Reason: "inspect live coordination"})
	if x.State != "paused" || x.Events[len(x.Events)-1].ActorID != "maintainer" {
		t.Fatalf("attributable pause: %#v", x)
	}
	x, _ = s.Control("repo", w.ID, x.ID, "maintainer", ControlInput{ExpectedRevision: x.Revision, Action: "resume", Reason: "inspection complete"})
	x, _ = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-1", CredentialExpiresAt: now.Add(time.Minute)})
	credential := x.Steps[0].Credential.Reference
	x, err = s.RecordResult("repo", w.ID, x.ID, "runner", "draft", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-1", CredentialReference: credential, State: "waiting_input", RequestedInput: "Choose the public proposal title", Logs: []ExecutionLog{{Level: "info", Message: "Draft assembled without private terminal input"}}, Artifacts: []ExecutionArtifact{{Name: "private trace", Digest: "sha256:abc", MediaType: "text/plain", Accessible: false}}, AgentSession: &AgentSession{ID: "session-1", Revision: "agent-v1", State: "completed"}})
	if err != nil || x.State != "waiting" || !x.Steps[0].Attempts[0].Artifacts[0].Redacted || x.Steps[0].Attempts[0].Artifacts[0].Name != "restricted artifact" {
		t.Fatalf("safe wait evidence: %#v %v", x, err)
	}
	if _, err = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "provide_input", StepID: "draft", Reason: "must reject private terminal material", Inputs: map[string]any{"terminal_input": "access_token=hidden"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("credential-shaped requested input must be rejected, got %v", err)
	}
	x, err = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "provide_input", StepID: "draft", Reason: "owner selected title", Inputs: map[string]any{"title": "Bounded repair"}})
	if err != nil || x.Steps[0].State != "ready" || x.Steps[0].ProvidedInputs["title"] != "Bounded repair" {
		t.Fatalf("provided input: %#v %v", x, err)
	}
	x, _ = s.Dispatch("repo", w.ID, x.ID, "scheduler", "draft", DispatchInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-2", CredentialExpiresAt: now.Add(time.Minute)})
	x, _ = s.RecordResult("repo", w.ID, x.ID, "runner", "draft", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: "draft-2", CredentialReference: x.Steps[0].Credential.Reference, State: "succeeded", Outputs: map[string]OutputValue{"proposal": {Value: "proposal-1", Accessible: true}}})
	x, err = s.Control("repo", w.ID, x.ID, "owner", ControlInput{ExpectedRevision: x.Revision, Action: "skip", StepID: "publish", Reason: "optional publication prohibited by current policy"})
	if err != nil || x.Steps[1].State != "skipped" || x.Steps[2].State != "manual_ready" {
		t.Fatalf("optional skip: %#v %v", x, err)
	}
	x, err = s.Control("repo", w.ID, x.ID, "reviewer", ControlInput{ExpectedRevision: x.Revision, Action: "take_over", StepID: "review", Reason: "reviewer accepted declared manual work"})
	if err != nil || x.Steps[2].ManualActorID != "reviewer" {
		t.Fatalf("manual takeover: %#v %v", x, err)
	}
	x, err = s.RecordResult("repo", w.ID, x.ID, "reviewer", "review", ResultInput{ExpectedRevision: x.Revision, IdempotencyKey: x.Steps[2].Attempts[0].IdempotencyKey, State: "succeeded"})
	if err != nil || x.State != "completed" || len(x.PredictedNextActions) != 0 {
		t.Fatalf("manual completion: %#v %v", x, err)
	}
}
