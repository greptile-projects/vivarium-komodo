package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowdefinitions"
)

func TestWorkflowExecutionHTTPRetainsScopedRetryableRun(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "automation-run", Visibility: repositories.Public})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := workflowdefinitions.New(t.TempDir())
	in := workflowdefinitions.Input{Name: "triage", Outcome: "proposal ready", RepositoryRevision: "commit-definition", DefinitionPath: ".project/workflows/triage.json", Triggers: []workflowdefinitions.Trigger{{ID: "issue", Type: "repository_event", Event: "issue.accepted"}}, Inputs: []workflowdefinitions.Field{{Name: "issue", Type: "string", Required: true}}, Steps: []workflowdefinitions.Step{{ID: "draft", Name: "Draft", Outputs: []workflowdefinitions.Field{{Name: "proposal", Type: "string", Required: true}}, Invocation: workflowdefinitions.Invocation{Kind: "approved_agent", Reference: "triage-agent", Revision: "release-4", Accessible: true, OwnerIDs: []string{"owner"}, Capabilities: []string{"proposal:draft"}}, Retry: workflowdefinitions.Retry{MaximumAttempts: 2}, TimeoutSeconds: 60, MaximumCost: 2, CompletionCriteria: []string{"draft exists"}}}, MaximumCost: 2, Currency: "USD", MaximumConcurrency: 1, RateLimit: workflowdefinitions.RateLimit{MaximumInvocations: 3, WindowSeconds: 60}, OwnerIDs: []string{"owner"}, CompletionCriteria: []string{"proposal ready"}, ChangeReason: "execute reviewed automation"}
	w, _ := store.Create(string(repo.ID), "owner", in)
	_, _ = store.Activate(string(repo.ID), w.ID, "owner", 1)
	mux := http.NewServeMux()
	registerWorkflowDefinitionsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/workflow-definitions/" + w.ID + "/executions"
	now := time.Now().UTC()
	invoke := workflowdefinitions.InvokeInput{IdempotencyKey: "delivery:42", WorkflowVersion: 1, Event: workflowdefinitions.TriggeringEvent{ID: "event-42", Type: "repository_event", Name: "issue.accepted", Revision: "event-revision-42", OccurredAt: now.Add(-time.Second)}, Actor: workflowdefinitions.ExecutionActor{Kind: "human", ID: "owner"}, Inputs: map[string]any{"issue": "42"}, PermittedResources: []workflowdefinitions.ResourceRevision{{Kind: "approved_agent", Reference: "triage-agent", Revision: "release-4"}}, Policy: workflowdefinitions.PolicyDecision{Repository: "allowed", Organization: "allowed", Agent: "allowed", Embargo: "allowed", Environment: "allowed", Approval: "allowed"}}
	b, _ := json.Marshal(invoke)
	var run workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), http.StatusCreated, &run)
	var duplicate workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), http.StatusCreated, &duplicate)
	if duplicate.ID != run.ID {
		t.Fatalf("duplicate event created another execution: %#v", duplicate)
	}
	dispatch := fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":"attempt-1","credential_expires_at":%q}`, run.Revision, now.Add(30*time.Second).Format(time.RFC3339Nano))
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+run.ID+"/steps/draft/dispatch", owner, dispatch, http.StatusCreated, &run)
	result := workflowdefinitions.ResultInput{ExpectedRevision: run.Revision, IdempotencyKey: "attempt-1", CredentialReference: run.Steps[0].Credential.Reference, State: "succeeded", Outputs: map[string]workflowdefinitions.OutputValue{"proposal": {Value: "proposal-42", Accessible: true}, "private": {Value: "hidden", Secret: true}}, Cost: 1}
	b, _ = json.Marshal(result)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+run.ID+"/steps/draft/results", owner, string(b), http.StatusCreated, &run)
	if run.State != "completed" || run.Cost != 1 || len(run.Steps[0].Attempts[0].Outputs) != 1 || run.Steps[0].Credential.SecretRetained {
		t.Fatalf("completed run escaped bounds: %#v", run)
	}
	var public workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+run.ID, "", "", http.StatusOK, &public)
	if public.WorkflowVersion != 1 || public.Event.ID != "event-42" || public.Actor.ID != "owner" {
		t.Fatalf("public durable binding missing: %#v", public)
	}
}
