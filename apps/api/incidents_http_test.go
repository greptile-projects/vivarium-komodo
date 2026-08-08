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
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
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
	releaseStore, _ := releases.New(t.TempDir())
	pullStore, _ := pullrequests.New(t.TempDir())
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, releaseStore, pullStore, catalog, credentials)
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

func TestPublicIncidentViewRedactsParticipantInvestigation(t *testing.T) {
	item := incidents.Incident{
		Followers: []string{"responder"}, Acknowledgements: []incidents.Acknowledgement{{ActorID: "responder"}},
		Evidence: []incidents.Evidence{{ID: "private", Audience: "participants"}, {ID: "public", Audience: "public"}},
		Findings: []incidents.Finding{{ID: "private", Audience: "participants"}, {ID: "public", Audience: "public"}},
		Timeline: []incidents.Event{{Sequence: 1, Type: "declared"}, {Sequence: 2, Type: "update", Audience: "participants"}, {Sequence: 3, Type: "update", Audience: "public"}},
	}
	visible := visibleIncident(item, false)
	if len(visible.Evidence) != 1 || visible.Evidence[0].ID != "public" || len(visible.Findings) != 1 || visible.Findings[0].ID != "public" || len(visible.Timeline) != 2 || visible.Followers != nil || visible.Acknowledgements != nil {
		t.Fatalf("public incident leaked participant state: %#v", visible)
	}
}

func TestResponderDelegatesReadOnlyIncidentInvestigation(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	opened, _ := catalog.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor Agent <agent@example.com> 0 +0000\ncommitter Agent <agent@example.com> 0 +0000\n\nrevision\n"))
	credentials, _ := auth.New(t.TempDir())
	ownerToken := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	deploymentStore, _ := deployments.New(t.TempDir())
	environment, _ := deploymentStore.PutEnvironment(string(repository.ID), "", "owner", deployments.EnvironmentInput{Name: "Production", Position: 1, Command: "deploy", Concurrency: 1})
	deployment, _ := deploymentStore.Create(deployments.CreateDeployment{RepositoryID: string(repository.ID), EnvironmentID: environment.ID, ReleaseID: "release", ActorID: "owner"})
	incidentStore, _ := incidents.New(t.TempDir())
	incident, _ := incidentStore.Create(incidents.CreateInput{RepositoryID: string(repository.ID), ActorID: "owner", Title: "Errors", Summary: "Elevated failures", Severity: "high", Roles: map[string]string{"commander": "owner"}, Affected: []incidents.AffectedEnvironment{{RepositoryID: string(repository.ID), EnvironmentID: environment.ID}}})
	incident, _ = incidentStore.AddEvidence(string(repository.ID), incident.ID, "owner", incidents.Evidence{Kind: "deployment", RepositoryID: string(repository.ID), ResourceID: deployment.ID, Title: "Rollout", Audience: "participants"})
	releaseStore, _ := releases.New(t.TempDir())
	pullStore, _ := pullrequests.New(t.TempDir())
	mux := http.NewServeMux()
	registerIncidentsHTTP(mux, incidentStore, deploymentStore, releaseStore, pullStore, catalog, credentials)
	body, _ := json.Marshal(map[string]any{"agent": "codex", "mandate": "Narrow the cause without changing production.", "evidence_ids": []string{incident.Evidence[0].ID}, "revisions": []map[string]string{{"repository_id": string(repository.ID), "commit_id": string(commit)}}, "operational_access": []map[string]string{{"repository_id": string(repository.ID), "kind": "deployment_logs", "resource_id": deployment.ID}}})
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/incidents/"+incident.ID+"/investigations", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+ownerToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("delegate = %d %s", response.Code, response.Body.String())
	}
	var created struct {
		WorkerCredential string             `json:"worker_credential"`
		Incident         incidents.Incident `json:"incident"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	if created.WorkerCredential == "" || created.Incident.Investigations[0].CredentialDigest != "" {
		t.Fatalf("credential response = %s", response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/incident-investigations/operational/"+deployment.ID, nil)
	request.Header.Set("Authorization", "Bearer "+created.WorkerCredential)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte("credential_digest")) {
		t.Fatalf("operational read = %d %s", response.Code, response.Body.String())
	}
	record, _ := json.Marshal(map[string]any{"type": "uncertainty", "message": "The timing matches but causality is unconfirmed.", "uncertainty": "low sample size", "evidence_ids": []string{incident.Evidence[0].ID}})
	request = httptest.NewRequest(http.MethodPost, "/incident-investigations/records", bytes.NewReader(record))
	request.Header.Set("Authorization", "Bearer "+created.WorkerCredential)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("record = %d %s", response.Code, response.Body.String())
	}
	restored, _ := incidentStore.Get(string(repository.ID), incident.ID)
	if restored.Timeline[len(restored.Timeline)-1].ActorID != "agent:codex" {
		t.Fatalf("timeline = %#v", restored.Timeline)
	}
	// The worker credential is intentionally not an auth.Store grant.
	request = httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/incidents/"+incident.ID+"/updates", bytes.NewReader([]byte(`{"audience":"participants","message":"mutate"}`)))
	request.Header.Set("Authorization", "Bearer "+created.WorkerCredential)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code == http.StatusCreated {
		t.Fatal("worker credential gained repository mutation authority")
	}
}
