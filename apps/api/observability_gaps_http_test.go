package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/observabilitygaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestObservabilityGapPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "signals", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "collab")
	token := issueAccess(t, credentials, "collab", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := observabilitygaps.New(t.TempDir())
	mux := http.NewServeMux()
	registerObservabilityGapsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	now := time.Now().UTC()
	body := `{"origin":{"kind":"incident","resource_id":"inc-7","revision":"4"},"question":"Which release causes checkout retries?","behavior":"Retries rise only for some checkout journeys","audience":["responders","checkout-owner"],"decision":"whether to roll back release 7","affected_services":["checkout-api"],"affected_journeys":["purchase"],"required_timeliness":{"maximum_delay_seconds":60,"decision_window":"before mitigation"},"current_evidence":[{"id":"requests","kind":"metric","source":"metric:requests","semantics":"retry ratio by release","release_id":"release-7","release_revision":"abcdef","environment":"production","environment_revision":"prod-19","observed_at":"` + now.Add(-time.Minute).Format(time.RFC3339) + `","fresh_until":"` + now.Add(time.Hour).Format(time.RFC3339) + `","accessible":true,"owner_id":"telemetry"},{"id":"trace","kind":"trace","source":"trace:checkout","observed_at":"` + now.Add(-time.Hour).Format(time.RFC3339) + `","accessible":false,"owner_id":"platform"}],"missing_coverage":["No event connects retries to payment-provider responses"],"owner_ids":["collab","platform"],"success_criteria":["A responder can distinguish application from provider retries within 60 seconds"],"change_reason":"capture the blocked rollback decision"}`
	var gap observabilitygaps.Gap
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/observability-gaps", token, body, 201, &gap)
	if gap.CurrentVersion != 1 || gap.Versions[0].AuthorID != "collab" || len(gap.Findings) != 5 {
		t.Fatalf("lost explicit evidence limitations: %+v", gap.Findings)
	}
	var list struct {
		Items []observabilitygaps.Gap `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+string(repo.ID)+"/observability-gaps", token, "", 200, &list)
	if len(list.Items) != 1 || len(list.Items[0].NonAuthority) == 0 {
		t.Fatalf("gap unavailable or authoritative: %+v", list)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/observability-gaps/"+gap.ID+"/versions", token, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
}
