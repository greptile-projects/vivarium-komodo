package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResponsePolicyPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "response", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("owner", repo.ID, "collab")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	collab := issueAccess(t, credentials, "collab", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := responsepolicies.New(t.TempDir())
	mux := http.NewServeMux()
	registerResponsePoliciesHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/response-policies"
	expires := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	body := `{"name":"Production response","description":"Agree coverage before alerts","resources":[{"kind":"repository","id":"self","owner_team_ids":["maintainers"],"required":true},{"kind":"service","id":"api","owner_team_ids":["maintainers","operators"],"required":true},{"kind":"dependency","id":"database","owner_team_ids":["operators"],"required":true}],"teams":[{"id":"maintainers","name":"Maintainers","member_ids":["owner"],"skills":["code"],"available":true,"authority":["repository:write"]},{"id":"operators","name":"Operators","member_ids":[],"skills":["operations"],"available":false,"authority":["environment:observe"]}],"coverage":[{"id":"api-critical","resource_kind":"service","resource_id":"api","signal_class":"reliability","severity":"critical","team_id":"operators","required_skills":["operations","database"],"response_target":{"acknowledge_minutes":5,"engage_minutes":10,"update_minutes":15},"escalation_path":[{"after_minutes":10,"team_id":"maintainers","audience_ids":["status-page"],"action":"request human engagement"}],"communication_audience_ids":["service-owners","status-page"],"expected_actions":["assess user impact","follow authorized runbook"],"incident_criteria":["multi-region user impact"]}],"rule_references":[{"kind":"organization_membership","resource_id":"org-1","revision":"v3","required":true,"accessible":true,"owner_id":"owner"},{"kind":"privacy","resource_id":"privacy-1","revision":"v2","required":true,"accessible":false,"owner_id":"privacy-owner"}],"exceptions":[{"id":"skill-gap","coverage_id":"api-critical","rationale":"database responder is being recruited","owner_id":"operators","approved_by":"owner","expires_at":"` + expires + `"}],"owner_ids":["owner"],"change_reason":"establish response ownership"}`
	var p responsepolicies.Policy
	workflowJSON(t, server.URL, http.MethodPost, base, collab, body, 201, &p)
	if p.CurrentVersion != 1 || p.Versions[0].AuthorID != "collab" || len(p.Gaps) != 8 || len(p.NonAuthority) == 0 {
		t.Fatalf("policy lost coverage findings or boundaries: %+v", p)
	}
	var list struct {
		Items []responsepolicies.Policy `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, owner, "", 200, &list)
	if len(list.Items) != 1 {
		t.Fatalf("policy not listed: %+v", list)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/versions", owner, body[:1]+`"expected_version":0,`+body[1:], http.StatusConflict, nil)
}
