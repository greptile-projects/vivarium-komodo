package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualitygates"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestQualityGatePublicAPIExposesExactCandidateMatrix(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := qualitygates.New(t.TempDir())
	mux := http.NewServeMux()
	registerQualityGatesHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/quality-gates"
	in := qualitygates.PolicyInput{Name: "Release", PlanID: "plan", PlanVersion: 1, ChangeReason: "protect journey", Selector: qualitygates.Selector{Branches: []string{"main"}}, Requirements: []qualitygates.Requirement{{ID: "checkout", BehaviorID: "checkout", ScenarioID: "scenario", Kind: "scenario", Environment: "preview", Locale: "en-US", Platform: "chromium", OwnerID: "owner", Required: true}}}
	body, _ := json.Marshal(in)
	var policy qualitygates.Policy
	workflowJSON(t, server.URL, http.MethodPost, base+"/policies", owner, string(body), http.StatusCreated, &policy)
	open, _ := json.Marshal(qualitygates.OpenInput{PolicyID: policy.ID, PolicyVersion: 1, Target: qualitygates.Target{Kind: "merge_queue", Reference: "queue-1", Revision: "sha-1", Branch: "main"}})
	var gate qualitygates.Gate
	workflowJSON(t, server.URL, http.MethodPost, base+"/candidates", owner, string(open), http.StatusCreated, &gate)
	if gate.Ready || len(gate.Matrix) != 1 || gate.Matrix[0].Gap != "no_current_attempt" {
		t.Fatalf("missing evidence was not exposed: %#v", gate)
	}
	var catalog qualitygates.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Policies) != 1 || len(catalog.Gates) != 1 || catalog.Gates[0].Target.Revision != "sha-1" {
		t.Fatalf("public exact matrix unavailable: %#v", catalog)
	}
}
