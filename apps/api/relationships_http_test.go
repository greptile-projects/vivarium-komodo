package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRelationshipGraphReportsExactResolvedAndStaleEvidence(t *testing.T) {
	objects, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), objects)
	provider, _ := catalog.Create("provider-owner", repositories.Metadata{Name: "contracts", Visibility: repositories.Public})
	consumer, _ := catalog.Create("consumer-owner", repositories.Metadata{Name: "checkout", Visibility: repositories.Public})
	releaseStore, _ := releases.New(t.TempDir())
	providerRelease, _ := releaseStore.Create(releases.CreateParams{RepositoryID: string(provider.ID), Version: "1.2.0", Notes: "contract", CommitID: "provider-commit", CreatedByID: "provider-owner"})
	consumerRelease, _ := releaseStore.Create(releases.CreateParams{RepositoryID: string(consumer.ID), Version: "2.0.0", Notes: "consumer", CommitID: "consumer-commit", CreatedByID: "consumer-owner"})
	relationshipStore, _ := relationships.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	providerToken := issueAccess(t, credentials, "provider-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	consumerToken := issueAccess(t, credentials, "consumer-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	proposalStore, _ := proposals.New(t.TempDir())
	pullStore, _ := pullrequests.New(t.TempDir())
	registerRelationshipsHTTP(mux, relationshipStore, releaseStore, deploymentStore, catalog, proposalStore, pullStore, credentials)
	post := func(path, token string, body map[string]string) int {
		data, _ := json.Marshal(body)
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != 201 {
			t.Fatalf("POST %s = %d %s", path, response.Code, response.Body.String())
		}
		return response.Code
	}
	post("/repositories/"+string(provider.ID)+"/interfaces", providerToken, map[string]string{"name": "payments", "version": "1.2.0", "release_id": providerRelease.ID, "schema_path": "api/openapi.yaml"})
	post("/repositories/"+string(consumer.ID)+"/dependencies", consumerToken, map[string]string{"provider_repository_id": string(provider.ID), "interface_name": "payments", "constraint": "^1.0.0", "release_id": consumerRelease.ID})
	read := func() map[string]any {
		request := httptest.NewRequest(http.MethodGet, "/repositories/"+string(provider.ID)+"/relationships", nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("GET = %d %s", response.Code, response.Body.String())
		}
		var result map[string]any
		_ = json.Unmarshal(response.Body.Bytes(), &result)
		return result
	}
	result := read()
	summary := result["summary"].(map[string]any)
	if summary["resolved"] != float64(1) || summary["repositories"] != float64(2) {
		t.Fatalf("resolved graph = %#v", result)
	}
	_, _ = releaseStore.Create(releases.CreateParams{RepositoryID: string(consumer.ID), Version: "2.1.0", Notes: "moved without declaration", CommitID: "new-consumer-commit", CreatedByID: "consumer-owner"})
	result = read()
	summary = result["summary"].(map[string]any)
	if summary["stale"] != float64(1) {
		t.Fatalf("stale graph = %#v", result)
	}
}
