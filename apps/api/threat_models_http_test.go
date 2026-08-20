package main

import (
	"encoding/json"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestThreatModelsRepositoryReaderCollaboration(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	creds, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "media", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, creds, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, creds, "reader", auth.API, auth.RepositoryRead)
	store, _ := threatmodels.New(t.TempDir())
	mux := http.NewServeMux()
	registerThreatModelsHTTP(mux, store, repos, creds, threatModelSources{})
	server := httptest.NewServer(mux)
	defer server.Close()
	in := threatmodels.Input{Title: "Webhook design", Summary: "Inspect callbacks", Origin: threatmodels.Origin{Kind: "api_evolution", Reference: "hooks", Revision: "v2"}, Inputs: []threatmodels.InputBinding{{Kind: "architecture", Reference: "callbacks", Revision: "a1"}}, EntryPoints: []threatmodels.EntryPoint{{ID: "hook", Description: "callback URL", Privileges: []string{"choose URL"}}}, AttackerGoals: []threatmodels.AttackerGoal{{ID: "ssrf", Actor: "writer", Goal: "reach metadata", Capability: "choose callback", Impact: "credential theft"}}, AbusePaths: []threatmodels.AbusePath{{ID: "metadata", GoalID: "ssrf", EntryPointIDs: []string{"hook"}, Steps: []string{"set metadata URL"}, ResidualRisk: "DNS rebinding", Severity: "high", OwnerIDs: []string{"owner"}}}, OwnerIDs: []string{"owner"}, ResidualRisk: "DNS rebinding remains"}
	b, _ := json.Marshal(in)
	base := "/repositories/" + string(repo.ID) + "/threat-models"
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(b), http.StatusCreated, &struct{}{})
	var list struct {
		Items []threatmodels.Model `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatal("model unavailable")
	}
	finding := `{"kind":"finding","body":"DNS can change after validation","abuse_path_ids":["metadata"],"citations":[{"kind":"api","reference":"hooks","revision":"v2","detail":"callback validation","visibility":"repository"}]}`
	var got threatmodels.Model
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+list.Items[0].ID+"/findings", reader, finding, http.StatusCreated, &got)
	if len(got.Findings) != 1 || got.Findings[0].AuthorID != "reader" {
		t.Fatalf("reader finding missing: %#v", got)
	}
	ack := `{"decision":"request_changes","rationale":"pin resolved addresses","origin_revision":"v2"}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+got.ID+"/acknowledgements", owner, ack, http.StatusCreated, &got)
	if len(got.Acknowledgements) != 1 {
		t.Fatal("acknowledgement missing")
	}
}
