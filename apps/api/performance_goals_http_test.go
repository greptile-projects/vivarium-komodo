package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPerformanceGoalContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	owner, collab := "owner", "collab"
	repo, _ := catalog.Create(owner, repositories.Metadata{Name: "speed", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator(owner, repo.ID, collab)
	ownerToken := issueAccess(t, credentials, owner, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	collabToken := issueAccess(t, credentials, collab, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := performancegoals.New(t.TempDir())
	mux := http.NewServeMux()
	registerPerformanceGoalsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repo.ID) + "/performance-goals"
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"subject_kind":"api","subject_id":"GET /search","title":"Keep search predictably fast","workloads":["100 concurrent repository searches"],"metrics":[{"id":"p95","name":"response latency","unit":"ms","direction":"lower","baseline":420,"target":{"maximum":250},"budget":300,"environment_digest":"linux-amd64","baseline_measured_at":"` + old + `","baseline_source":"preview pv1"}],"correctness_constraints":["Results remain complete and permission filtered"],"supported_environments":[{"name":"Production Linux","os":"linux","architecture":"amd64","digest":"linux-amd64"}],"owner_ids":["` + owner + `","` + collab + `"],"links":[{"kind":"issue","resource_id":"issue-1"},{"kind":"decision","resource_id":"decision-1"}],"baseline_max_age_days":30,"change_reason":"Agree before optimizing"}`
	var goal performancegoals.Goal
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):], ownerToken, body, 201, &goal)
	if goal.CurrentVersion != 1 || goal.Statuses[0].State != "missing_measurement" {
		t.Fatalf("unexpected goal: %+v", goal)
	}
	measurement := `{"version":1,"metric_id":"p95","value":230,"environment_digest":"mac-arm64","source":"benchmark run 44"}`
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):]+"/"+goal.ID+"/measurements", collabToken, measurement, 201, &goal)
	if goal.Statuses[0].State != "incomparable_environment" || goal.Measurements[0].ActorID != collab {
		t.Fatalf("measurement must remain explicitly incomparable and attributable: %+v", goal)
	}
	var listed struct {
		Items []performancegoals.Goal `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base[len(server.URL):], collabToken, "", 200, &listed)
	if len(listed.Items) != 1 {
		t.Fatal("goal not listed")
	}
	workflowJSON(t, server.URL, http.MethodPost, base[len(server.URL):]+"/"+goal.ID+"/versions", ownerToken, body[:1]+`"expected_version":0,`+body[1:], http.StatusConflict, nil)
}
