package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func objectiveBody(owner, signal, calculation string, expiry time.Time) string {
	return `{"title":"Pull request availability","description":"Contributors can review and merge changes","scopes":[{"kind":"repository","name":"whole repository"},{"kind":"release","resource_id":"v2","name":"version 2"},{"kind":"environment","resource_id":"production","name":"production"}],"indicators":[{"id":"successful_requests","name":"Successful pull requests","description":"Pull pages complete without error","signal":"http.pull.success","signal_status":"` + signal + `","calculation":"` + calculation + `","unit":"percent","good_event":"status below 500","total_event":"all pull page requests"}],"measurement_windows":[{"id":"rolling-28d","kind":"rolling","duration":"28d"}],"targets":[{"indicator_id":"successful_requests","window_id":"rolling-28d","comparator":"gte","value":99.9,"error_budget_percent":0.1}],"journeys":[{"id":"review","name":"Review a change","behavior":"A collaborator can open and review a pull request","owner_ids":["` + owner + `"]}],"dependencies":[{"id":"database","name":"Primary database","kind":"service","required":true,"owner_ids":[]}],"severity_thresholds":[{"level":"warning","budget_consumed_percent":50,"response":"investigate within one day","owner_ids":["` + owner + `"]},{"level":"critical","budget_consumed_percent":100,"response":"contain affected delivery","owner_ids":["` + owner + `"]}],"owner_ids":["` + owner + `"],"commitment_links":[{"kind":"product","resource_id":"roadmap-v2","label":"Review journey","status":"linked"},{"kind":"performance","resource_id":"goal-api","label":"API latency","status":"linked"},{"kind":"accessibility","resource_id":"a11y-v1","label":"Pull journey","status":"linked"},{"kind":"privacy","resource_id":"privacy-v1","label":"Request telemetry","status":"missing"},{"kind":"release","resource_id":"release-policy","label":"Production release","status":"linked"}],"exceptions":[{"id":"database-owner","reason":"service ownership transition","approved_by":"` + owner + `","owner_id":"` + owner + `","expires_at":"` + expiry.Format(time.RFC3339) + `"}],"exception_policy":"Owner approval, rationale, responsible owner, and expiry are required","change_reason":"publish shared reliability terms"}`
}
func TestServiceObjectivePublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "reliable", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := serviceobjectives.New(t.TempDir())
	mux := http.NewServeMux()
	registerServiceObjectivesHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/service-objectives"
	var got serviceobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base, owner, objectiveBody("owner", "missing", "percentile", time.Now().UTC().Add(10*24*time.Hour)), 201, &got)
	var list struct {
		Items []serviceobjectives.Objective `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", 200, &list)
	k := map[string]bool{}
	for _, b := range list.Items[0].Blockers {
		k[b.Kind] = true
	}
	for _, want := range []string{"missing_signal", "unsupported_calculation", "missing_dependency_owner", "missing_commitment", "expiring_exception"} {
		if !k[want] {
			t.Fatalf("missing %s in %+v", want, list.Items[0].Blockers)
		}
	}
	if got.Versions[0].AuthorID != "owner" {
		t.Fatal("version is not attributable")
	}
	body := objectiveBody("owner", "available", "ratio", time.Now().UTC().Add(60*24*time.Hour))
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+got.ID+"/versions", owner, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
}
func TestServiceObjectiveConflictingTargets(t *testing.T) {
	s, _ := serviceobjectives.New(t.TempDir())
	in := serviceobjectives.VersionInput{Title: "Availability", Description: "Journey works", Scopes: []serviceobjectives.Scope{{Kind: "environment", ResourceID: "prod", Name: "Production"}}, Indicators: []serviceobjectives.Indicator{{ID: "ok", Name: "OK", Description: "requests succeed", Signal: "requests", SignalStatus: "available", Calculation: "ratio", Unit: "percent"}}, Windows: []serviceobjectives.Window{{ID: "month", Kind: "rolling", Duration: "30d"}}, Targets: []serviceobjectives.Target{{IndicatorID: "ok", WindowID: "month", Comparator: "gte", Value: 99, ErrorBudgetPercent: 1}}, Journeys: []serviceobjectives.Journey{{ID: "use", Name: "Use", Behavior: "user succeeds", OwnerIDs: []string{"owner"}}}, Severities: []serviceobjectives.Severity{{Level: "critical", BudgetConsumedPercent: 100, Response: "respond", OwnerIDs: []string{"owner"}}}, OwnerIDs: []string{"owner"}, ExceptionPolicy: "time bounded", ChangeReason: "publish"}
	_, _ = s.Create("repo", "owner", in)
	in.Targets[0].Value = 99.9
	_, _ = s.Create("repo", "owner", in)
	items, _ := s.List("repo")
	found := false
	for _, x := range items {
		for _, b := range x.Blockers {
			found = found || b.Kind == "conflicting_target"
		}
	}
	if !found {
		t.Fatalf("expected conflict: %+v", items)
	}
}
