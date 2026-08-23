package main

import (
	"encoding/json"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workflowdefinitions"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkflowDefinitionsPublicReviewAndOwnerActivation(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "automation", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "writer")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	writer := issueAccess(t, credentials, "writer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := workflowdefinitions.New(t.TempDir())
	mux := http.NewServeMux()
	registerWorkflowDefinitionsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/workflow-definitions"
	in := workflowdefinitions.Input{Name: "triage", Outcome: "accepted issue has a bounded proposal", RepositoryRevision: "abc123", DefinitionPath: ".project/workflows/triage.json", Triggers: []workflowdefinitions.Trigger{{ID: "issue", Type: "repository_event", Event: "issue.accepted"}}, Steps: []workflowdefinitions.Step{{ID: "draft", Name: "Draft", Invocation: workflowdefinitions.Invocation{Kind: "platform_action", Reference: "proposal.create", Revision: "v1", Accessible: true, OwnerIDs: []string{"owner"}, Capabilities: []string{"proposal:draft"}}, Retry: workflowdefinitions.Retry{MaximumAttempts: 1}, TimeoutSeconds: 60, MaximumCost: 1, CompletionCriteria: []string{"proposal exists"}}}, MaximumCost: 1, Currency: "USD", OwnerIDs: []string{"owner"}, CompletionCriteria: []string{"proposal exists"}, ChangeReason: "initial"}
	body, _ := json.Marshal(in)
	var created workflowdefinitions.Workflow
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(body), http.StatusCreated, &created)
	if created.EffectiveAuthority.GrantsAuthority || created.EventSubscriptions[0] != "issue.accepted" {
		t.Fatalf("preview %#v", created)
	}
	var catalog workflowdefinitions.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("catalog %#v", catalog)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/activation", writer, `{"version":1}`, http.StatusConflict, nil)
	var active workflowdefinitions.Workflow
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/activation", owner, `{"version":1}`, http.StatusCreated, &active)
	if active.State != "active" {
		t.Fatalf("activation %#v", active)
	}
}
