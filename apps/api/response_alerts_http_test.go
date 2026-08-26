package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestResponseAlertWorkspacePublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("alice", repositories.Metadata{Name: "response-workspace", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("alice", repo.ID, "owner")
	alice := issueAccess(t, credentials, "alice", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	alerts, _ := responsealerts.New(t.TempDir())
	policies, _ := responsepolicies.New(t.TempDir())
	rotations, _ := responserotations.New(t.TempDir())
	incidentRecords, _ := incidents.New(t.TempDir())
	now := time.Now().UTC()
	policy := responsepolicies.Policy{ID: "policy", CurrentVersion: 1, Versions: []responsepolicies.Version{{Number: 1, Input: responsepolicies.Input{Coverage: []responsepolicies.Coverage{{ID: "critical-api", ResourceKind: "service", ResourceID: "api", SignalClass: "reliability", Severity: "critical", TeamID: "ops", Target: responsepolicies.Target{AcknowledgeMinutes: 5}}}}}}}
	rotation := responserotations.Rotation{ID: "rotation", Revision: 1, Input: responserotations.Input{PolicyID: "policy", PolicyVersion: 1, TeamID: "ops"}, CurrentShift: &responserotations.ShiftView{ResponderID: "alice"}}
	alert, _ := alerts.Create(string(repo.ID), "monitor", responsealerts.Input{Signal: responsealerts.Signal{SignalClass: "reliability", Severity: "critical", ResourceKind: "service", ResourceID: "api", Revision: "release-7", ObservedAt: now, CorrelationKey: "api:errors", Summary: "errors affect users"}}, policy, []responserotations.Rotation{rotation})
	mux := http.NewServeMux()
	registerResponseAlertsHTTP(mux, alerts, policies, rotations, incidentRecords, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/response-alerts/" + alert.ID
	var opened responsealerts.Alert
	workflowJSON(t, server.URL, http.MethodPost, base+"/workspace", alice, `{"expected_revision":1,"context":[{"kind":"release","resource_id":"release-7","revision":"commit-7","permitted":true,"audience":"participants"},{"kind":"runbook","resource_id":"rb-api","revision":"v3","permitted":true,"audience":"participants"}]}`, 201, &opened)
	workflowJSON(t, server.URL, http.MethodPost, base+"/workspace/actions", alice, `{"expected_revision":2,"kind":"invite","detail":"invite service owner","assignee_id":"owner"}`, 201, &opened)
	workflowJSON(t, server.URL, http.MethodPost, base+"/workspace/diagnostics", owner, `{"expected_revision":3,"name":"inspect lag","command_reference":"rb-api#lag","context_references":["rb-api"],"approved_by_id":"alice","sanitized_output":"lag=42s"}`, 201, &opened)
	var delegated struct {
		Alert      responsealerts.Alert `json:"alert"`
		Credential string               `json:"credential"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/workspace/agents", alice, `{"expected_revision":4,"agent":"triage@v1","mandate":"compare current release","context_references":["release-7"]}`, 201, &delegated)
	if delegated.Credential == "" || delegated.Alert.Workspace.AgentInvestigations[0].CredentialDigest != "" {
		t.Fatalf("worker credential absent or digest exposed: %+v", delegated)
	}
	workflowJSON(t, server.URL, http.MethodGet, "/response-alert-investigations/context", delegated.Credential, "", 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/response-alert-investigations/records", delegated.Credential, `{"kind":"finding","body":"retry behavior changed","evidence_references":["release-7"]}`, 201, nil)
	var promoted struct {
		Alert    responsealerts.Alert `json:"alert"`
		Incident incidents.Incident   `json:"incident"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/workspace/incident", alice, `{"expected_revision":6,"title":"API unavailable","summary":"confirmed impact","severity":"critical","roles":{"commander":"alice"},"affected":[{"repository_id":"`+string(repo.ID)+`","environment_id":"production"}]}`, 201, &promoted)
	if promoted.Alert.Workspace.IncidentID != promoted.Incident.ID || len(promoted.Alert.Events) < 5 || len(promoted.Alert.Workspace.AgentInvestigations[0].Records) != 1 {
		t.Fatalf("promotion lost response context: %+v", promoted)
	}
}
