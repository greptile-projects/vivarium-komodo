package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunbookPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "operations", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("owner", repo.ID, "writer")
	token := issueAccess(t, credentials, "writer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := runbooks.New(t.TempDir())
	mux := http.NewServeMux()
	registerRunbooksHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	body := `{"name":"Restore API","purpose":"Explain safe diagnosis and rollback","scope":{"kind":"service","resource_id":"api","revision":"service-v7","owner_id":"owner"},"preconditions":[{"id":"impact","description":"confirm impact","evidence":"dashboard window","owner_id":"responder","safe":true}],"steps":[{"id":"inspect","kind":"diagnostic","title":"Inspect errors","purpose":"locate failure","precondition_ids":["impact"],"references":[{"kind":"command","resource_id":"metrics-query","revision":"sha256:1","detail":"read errors","accessible":false,"reviewed":true,"secret_bearing":false,"owner_id":"operators"}],"expected_evidence":["sanitized errors"],"required_authority":["telemetry:read"],"owner_ids":[],"required_skills":["diagnosis"],"rollback_criteria":[]}],"rollback_criteria":["error rate rises"],"owner_ids":["owner"],"required_skills":["diagnosis"],"escalation_paths":[{"condition":"evidence unavailable","owner_id":"owner","required_skills":["incident-command"],"audience_ids":["operators"],"action":"escalate"}],"policy_references":[{"kind":"security","resource_id":"prod-policy","revision":"v4","accessible":true,"conflicting":true,"owner_id":"security"}],"change_reason":"publish procedure"}`
	var x runbooks.Runbook
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runbooks", token, body, 201, &x)
	if x.CurrentVersion != 1 || len(x.Findings) != 3 || x.AuthorityPreview[0].Granted || len(x.NonAuthority) == 0 {
		t.Fatalf("public projection lost review state: %#v", x)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repo.ID)+"/runbooks/"+x.ID+"/versions", token, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
}
