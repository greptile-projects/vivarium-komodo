package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func TestAccessibilityBarrierPrivacyAndExactReproduction(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "accessible", Visibility: repositories.Public})
	reporterToken := issueAccess(t, credentials, "reporter", auth.API, auth.RepositoryRead)
	viewerToken := issueAccess(t, credentials, "viewer", auth.API, auth.RepositoryRead)
	ownerToken := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ps, _ := previews.New(t.TempDir())
	p, _ := ps.Create(previews.Preview{RepositoryID: string(repo.ID), PullRequestID: "pull", Revision: "exact-revision", Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	ws, _ := workspaces.New(t.TempDir())
	store, _ := accessibilitybarriers.New(t.TempDir())
	mux := http.NewServeMux()
	registerAccessibilityBarriersHTTP(mux, store, catalog, credentials, accessibilityBarrierSources{previews: ps, workspaces: ws, repositories: catalog})
	server := httptest.NewServer(mux)
	defer server.Close()
	body := `{"context":{"kind":"preview","resource_id":"` + p.ID + `","path":"/settings","revision":"exact-revision"},"access_needs":"Operate every control by voice","expected_outcome":"Save without touch input","interaction_steps":["Open settings","Say click Save"],"environment":{"browser":"Chromium 140","device_class":"phone","assistive_technology":"Voice Access","sensitive_device_data":"private model and installed apps"},"identity_visibility":"maintainers","device_data_visibility":"maintainers","evidence":[{"kind":"input_trace","name":"commands.txt","media_type":"text/plain","content":"redacted command trace","visibility":"maintainers","redacted":true}]}`
	base := "/repositories/" + string(repo.ID) + "/accessibility-barriers"
	var barrier accessibilitybarriers.Barrier
	workflowJSON(t, server.URL, http.MethodPost, base, reporterToken, body, 201, &barrier)
	var listed struct {
		Items []accessibilitybarriers.Barrier `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, viewerToken, "", 200, &listed)
	if len(listed.Items) != 1 || listed.Items[0].ReporterID != "" || listed.Items[0].Environment.SensitiveDeviceData != "" || listed.Items[0].Evidence[0].Content != "" {
		t.Fatalf("restricted reporter evidence leaked: %+v", listed.Items)
	}
	attempt := `{"execution_kind":"preview","execution_id":"` + p.ID + `","revision":"wrong","environment":{"browser":"Chromium 140","device_class":"emulated phone","assistive_technology":"Voice Access"},"result":"unconfirmed","notes":"Could not confirm","evidence":[]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+barrier.ID+"/attempts", ownerToken, attempt, 422, nil)
	attempt = strings.Replace(attempt, "wrong", "exact-revision", 1)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+barrier.ID+"/attempts", ownerToken, attempt, 201, &barrier)
	if len(barrier.Attempts) != 1 || barrier.Attempts[0].Result != "unconfirmed" || barrier.Attempts[0].Revision != "exact-revision" {
		t.Fatalf("attempt not retained: %+v", barrier)
	}
}
