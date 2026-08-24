package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/propagationcampaigns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPropagationCampaignPublicBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	campaigns, _ := propagationcampaigns.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "source", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "collaborator")
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nrepair\n", tree)))
	token := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPropagationCampaignsHTTP(mux, campaigns, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/propagation-campaigns"
	due := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	observed := time.Now().UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"title":"Propagate parser repair","intent":"Keep behavior equivalent","acceptance_criteria":["legacy input works"],"source":{"kind":"regression_correction","repository_id":%q,"resource_id":"correction-1","revision":%q,"commit_ids":[%q]},"targets":[{"id":"stable","repository_id":%q,"release_line":"v2","deadline":%q,"disposition":"pending","authority":{"owner_ids":["owner"],"access":"requested","basis":"target owner remains authoritative","observed_at":%q}},{"id":"peer","repository_reference":"https://peer.example/lib","release_line":"v1","deadline":%q,"depends_on":["stable"],"disposition":"inaccessible","disposition_reason":"peer unavailable","authority":{"access":"unknown","basis":"federated reference only","observed_at":%q}}],"completion_policy":{"mode":"all_supported","exception_requires_owner":true}}`, repository.ID, commit, commit, repository.ID, due, observed, due, observed)
	var campaign propagationcampaigns.Campaign
	workflowJSON(t, server.URL, http.MethodPost, base, token, body, 201, &campaign)
	if len(campaign.Blockers) != 1 || campaign.Blockers[0].Kind != "inaccessible" {
		t.Fatalf("explicit target lost: %#v", campaign)
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+campaign.ID, token, "", 200, &campaign)
	bad := fmt.Sprintf(`{"title":"bad","intent":"bad","acceptance_criteria":["x"],"source":{"kind":"policy_change","repository_id":%q,"resource_id":"p","revision":"missing","commit_ids":["missing"]},"targets":[]}`, repository.ID)
	workflowJSON(t, server.URL, http.MethodPost, base, token, bad, 422, nil)
}
