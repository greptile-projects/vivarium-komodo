package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestIncidentPublicAPIStartsFromFailedHealthSignal(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	deploymentStore, _ := deployments.New(t.TempDir())
	environment, _ := deploymentStore.PutEnvironment(string(repository.ID), "", "owner", deployments.EnvironmentInput{Name: "Production", Position: 1, Command: "deploy", Concurrency: 1})
	deployment, _ := deploymentStore.Create(deployments.CreateDeployment{RepositoryID: string(repository.ID), EnvironmentID: environment.ID, ReleaseID: "release", ActorID: "owner"})
	deployment, _ = deploymentStore.Start(string(repository.ID), deployment.ID)
	deployment, _ = deploymentStore.Stage(string(repository.ID), deployment.ID, "canary", "health.completed", "availability", "failed", "probe failed")
	incidentStore, _ := incidents.New(t.TempDir())
	mux := http.NewServeMux()
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, catalog, credentials)
	body, _ := json.Marshal(map[string]any{"title": "Availability loss", "summary": "Canary is unavailable", "severity": "critical", "roles": map[string]string{"commander": "owner"}, "affected": []map[string]string{{"repository_id": string(repository.ID), "environment_id": environment.ID}}, "source_signal": map[string]any{"repository_id": string(repository.ID), "deployment_id": deployment.ID, "event_sequence": deployment.Events[len(deployment.Events)-1].Sequence}})
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/incidents", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("declare = %d %s", response.Code, response.Body.String())
	}
	var item incidents.Incident
	_ = json.Unmarshal(response.Body.Bytes(), &item)
	if item.SourceSignal == nil || item.SourceSignal.Signal != "availability" || item.SourceSignal.Stage != "canary" || item.Timeline[0].ActorID != "owner" {
		t.Fatalf("incident = %#v", item)
	}
	update, _ := json.Marshal(map[string]string{"audience": "public", "message": "Responders are mitigating impact."})
	request = httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/incidents/"+item.ID+"/updates", bytes.NewReader(update))
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("update = %d %s", response.Code, response.Body.String())
	}
}
