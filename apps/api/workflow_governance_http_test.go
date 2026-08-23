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

func TestWorkflowGovernanceKeepsConsequentialActionsHumanControlled(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "governed-automation", Visibility: repositories.Public})
	for _, id := range []string{"reviewer", "resource-owner"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, id)
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reviewer := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	resourceOwner := issueAccess(t, credentials, "resource-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := workflowdefinitions.New(t.TempDir())
	mux := http.NewServeMux()
	registerWorkflowDefinitionsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	in := workflowdefinitions.Input{Name: "release", Outcome: "reviewed release", RepositoryRevision: "pull-7", DefinitionPath: ".project/workflows/release.json", Triggers: []workflowdefinitions.Trigger{{ID: "merged", Type: "repository_event", Event: "pull_request.merged"}}, Steps: []workflowdefinitions.Step{{ID: "publish", Name: "Publish", Invocation: workflowdefinitions.Invocation{Kind: "platform_action", Reference: "release.create", Revision: "v2", Accessible: true, OwnerIDs: []string{"resource-owner"}, Capabilities: []string{"release:create"}, ActionClass: "release"}, Retry: workflowdefinitions.Retry{MaximumAttempts: 1}, TimeoutSeconds: 60, MaximumCost: 5, CompletionCriteria: []string{"receipt retained"}}}, MaximumCost: 5, Currency: "USD", OwnerIDs: []string{"owner"}, CompletionCriteria: []string{"release exists"}, Governance: workflowdefinitions.Governance{RequiredReviewerIDs: []string{"reviewer"}, RequiredOwnerIDs: []string{"owner"}, SimulationCases: []workflowdefinitions.SimulationCase{{ID: "merge", Event: "pull_request.merged", ExpectedEffects: []string{"create release"}, ExpectedPermissions: []string{"release:create"}, MaximumCost: 5}}, ActionRequirements: []workflowdefinitions.ActionRequirement{{ActionClass: "release", OwnerIDs: []string{"resource-owner"}, MinimumApprovals: 1, SeparateFromAuthor: true, ApprovalTTLSeconds: 300}}}, ChangeReason: "govern consequential release"}
	w, _ := store.Create(string(repo.ID), "owner", in)
	base := "/repositories/" + string(repo.ID) + "/workflow-definitions/" + w.ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/activation", owner, `{"version":1}`, http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/simulations", reviewer, `{"version":1,"case_id":"merge","passed":true,"effects":["create release"],"permissions":["release:create"],"cost":3}`, http.StatusCreated, &w)
	workflowJSON(t, server.URL, http.MethodPost, base+"/candidate-decisions", reviewer, `{"version":1,"kind":"review","decision":"approved","rationale":"simulation matches the pull candidate"}`, http.StatusCreated, &w)
	workflowJSON(t, server.URL, http.MethodPost, base+"/candidate-decisions", owner, `{"version":1,"kind":"owner_acknowledgement","decision":"acknowledged","rationale":"release ownership remains explicit"}`, http.StatusCreated, &w)
	workflowJSON(t, server.URL, http.MethodPost, base+"/exceptions", owner, `{"version":1,"scope":"spend above simulated amount","rationale":"bounded launch window","expires_at":"2030-01-01T00:00:00Z"}`, http.StatusCreated, &w)
	workflowJSON(t, server.URL, http.MethodPost, base+"/activation", owner, `{"version":1}`, http.StatusCreated, &w)
	now := time.Now().UTC()
	invoke := workflowdefinitions.InvokeInput{IdempotencyKey: "merge:7", WorkflowVersion: 1, Event: workflowdefinitions.TriggeringEvent{ID: "merge-7", Type: "repository_event", Name: "pull_request.merged", Revision: "pull-7", OccurredAt: now.Add(-time.Second)}, Actor: workflowdefinitions.ExecutionActor{Kind: "human", ID: "owner"}, Inputs: map[string]any{}, PermittedResources: []workflowdefinitions.ResourceRevision{{Kind: "platform_action", Reference: "release.create", Revision: "v2"}}, Policy: workflowdefinitions.PolicyDecision{Repository: "allowed", Organization: "allowed", Agent: "allowed", Embargo: "allowed", Environment: "allowed", Approval: "allowed"}}
	b, _ := json.Marshal(invoke)
	var run workflowdefinitions.Execution
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions", owner, string(b), http.StatusCreated, &run)
	dispatch := fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":"publish-1","credential_expires_at":%q}`, run.Revision, now.Add(30*time.Second).Format(time.RFC3339Nano))
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions/"+run.ID+"/steps/publish/dispatch", owner, dispatch, http.StatusTooManyRequests, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions/"+run.ID+"/steps/publish/approval-requests", owner, fmt.Sprintf(`{"expected_revision":%d}`, run.Revision), http.StatusCreated, &run)
	approval := run.ActionApprovals[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions/"+run.ID+"/approval-requests/"+approval.ID+"/decisions", resourceOwner, fmt.Sprintf(`{"expected_revision":%d,"decision":"approved","rationale":"resource owner approves this exact release"}`, run.Revision), http.StatusCreated, &run)
	dispatch = fmt.Sprintf(`{"expected_revision":%d,"idempotency_key":"publish-1","credential_expires_at":%q}`, run.Revision, time.Now().UTC().Add(30*time.Second).Format(time.RFC3339Nano))
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions/"+run.ID+"/steps/publish/dispatch", owner, dispatch, http.StatusCreated, &run)
	result := workflowdefinitions.ResultInput{ExpectedRevision: run.Revision, IdempotencyKey: "publish-1", CredentialReference: run.Steps[0].Credential.Reference, State: "succeeded", Outputs: map[string]workflowdefinitions.OutputValue{"release": {Value: "v1", Accessible: true, Digest: "sha256:release"}}, Cost: 3}
	b, _ = json.Marshal(result)
	workflowJSON(t, server.URL, http.MethodPost, base+"/executions/"+run.ID+"/steps/publish/results", owner, string(b), http.StatusCreated, &run)
	if run.State != "completed" || len(run.ActionReceipts) != 1 || run.ActionReceipts[0].ApprovalID != approval.ID {
		t.Fatalf("immutable action receipt missing: %#v", run)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/disable", owner, `{"reason":"anomalous release behavior"}`, http.StatusCreated, &w)
	if w.State != "draft" || w.DisabledAt == nil {
		t.Fatalf("emergency disable did not stop new effects: %#v", w)
	}
	in.RepositoryRevision, in.ChangeReason = "pull-8", "authority changed after anomaly"
	w, _ = store.Revise(string(repo.ID), w.ID, "owner", 1, in)
	workflowJSON(t, server.URL, http.MethodPost, base+"/rollback", owner, `{"version":1,"reason":"restore prior reviewed workflow as a new draft"}`, http.StatusCreated, &w)
	if w.CurrentVersion != 3 || w.State != "draft" || w.Versions[2].RepositoryRevision != "pull-7" {
		t.Fatalf("rollback did not preserve immutable version history: %#v", w)
	}
}
