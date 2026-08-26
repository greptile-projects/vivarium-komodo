package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookexecutions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRunbookExecutionPublicLaunchContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "operations", Visibility: repositories.Private})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	books, _ := runbooks.New(t.TempDir())
	rehearsals, _ := runbookrehearsals.New(t.TempDir())
	executions, _ := runbookexecutions.New(t.TempDir())
	book, err := books.Create(string(repo.ID), "owner", runbooks.Input{Name: "Restore API", Purpose: "guided recovery", Scope: runbooks.Scope{Kind: "service", ResourceID: "api", Revision: "service-v7", OwnerID: "owner"}, Preconditions: []runbooks.Precondition{{ID: "impact", Description: "confirm impact", Evidence: "metrics", OwnerID: "owner", Safe: true}}, Steps: []runbooks.Step{{ID: "inspect", Kind: "diagnostic", Title: "inspect", Purpose: "locate failure", Preconditions: []string{"impact"}, ExpectedEvidence: []string{"errors"}, OwnerIDs: []string{"owner"}, RequiredSkills: []string{"diagnosis"}}}, RollbackCriteria: []string{"health worsens"}, OwnerIDs: []string{"owner"}, RequiredSkills: []string{"diagnosis"}, EscalationPaths: []runbooks.Escalation{{Condition: "blocked", OwnerID: "owner", RequiredSkills: []string{"diagnosis"}, AudienceIDs: []string{"owners"}, Action: "escalate"}}, ChangeReason: "publish"})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerRunbookExecutionsHTTP(mux, executions, books, rehearsals, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	var recommendations struct {
		Items              []runbookexecutions.Candidate `json:"items"`
		AutomaticSelection bool                          `json:"automatic_selection"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runbook-executions/recommendations", token, `{"origin":{"kind":"alert","resource_id":"a1","revision":"1","timeline_reference":"/alerts/a1","audience":"participants"},"resource_kinds":["service"],"resource_ids":["api"],"required_skills":["diagnosis"]}`, 200, &recommendations)
	if recommendations.AutomaticSelection || len(recommendations.Items) != 1 || recommendations.Items[0].Score != 7 || recommendations.Items[0].Eligible {
		t.Fatalf("unsafe recommendation: %#v", recommendations)
	}
	now := time.Now().UTC()
	body := map[string]any{"idempotency_key": "a1:restore", "runbook_id": book.ID, "runbook_version": 1, "origin": map[string]any{"kind": "alert", "resource_id": "a1", "revision": "1", "timeline_reference": "/alerts/a1#timeline", "audience": "participants"}, "affected_resources": []string{"service:api"}, "signal_window": map[string]any{"started_at": now.Add(-time.Minute), "ended_at": now}, "context": []map[string]any{{"kind": "release", "resource_id": "release-7", "revision": "commit-7", "permitted": true, "audience": "participants", "accessible": true}}, "preconditions": []map[string]any{{"id": "impact", "satisfied": true, "evidence_reference": "metric:errors"}}, "access": []map[string]any{{"capability": "telemetry:read", "resource_id": "api", "granted": true, "authority_reference": "policy:on-call"}}, "match_explanation": []string{"exact resource match"}}
	var launched runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runbook-executions", token, body, 422, &launched)
	if launched.State != "blocked" || launched.Origin.TimelineReference != "/alerts/a1#timeline" || len(launched.Blockers) != 1 {
		t.Fatalf("launch context lost: %#v", launched)
	}
	if len(launched.ActivePath) != 1 || launched.ActivePath[0].ID != "inspect" || len(launched.Steps) != 1 || launched.ControllerID != "owner" || launched.PredictedNextAction == "" {
		t.Fatalf("live procedure was not frozen: %#v", launched)
	}
	var retried runbookexecutions.Execution
	workflowValue(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runbook-executions", token, body, 422, &retried)
	if retried.ID != launched.ID {
		t.Fatalf("retry created duplicate: %s != %s", retried.ID, launched.ID)
	}
}
