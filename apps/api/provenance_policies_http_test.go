package main

import (
	"encoding/json"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRepositoryProvenancePolicyPublicAPI(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := provenancepolicies.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	mux := http.NewServeMux()
	registerProvenancePoliciesHTTP(mux, store, repos, orgs, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/provenance-policies"
	now := time.Now().UTC()
	in := provenancepolicies.Input{Name: "Accepted material", Description: "Rules before contribution", Rules: []provenancepolicies.MaterialRule{{Kind: "source", Origins: []string{"original"}, Licenses: []string{"unknown"}, Uses: []string{"public_distribution"}}}, DistributionContexts: []provenancepolicies.DistributionContext{{ID: "public", Audience: "public", Uses: []string{"public_distribution"}, Licenses: []string{"GPL-3.0"}, NoticeRequired: true}}, Links: []provenancepolicies.Link{{Kind: "contributor_pathway", Reference: "pathway:external"}, {Kind: "agent_contract", Reference: "agent:repair", Revision: "v2"}, {Kind: "package", Reference: "package:cli"}, {Kind: "release", Reference: "release:v1"}, {Kind: "contribution_boundary", Reference: "peer:upstream", Boundary: "federated"}}, Exceptions: []provenancepolicies.Exception{{ID: "temporary", MaterialKinds: []string{"source"}, ContextIDs: []string{"public"}, Rationale: "replace source", OwnerID: "legal", ApprovedBy: "owner", ExpiresAt: now.Add(7 * 24 * time.Hour)}}, ChangeReason: "initial"}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var made provenancepolicies.Policy
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &made)
	kinds := map[string]bool{}
	for _, b := range made.Blockers {
		kinds[b.Kind] = true
	}
	for _, want := range []string{"unknown_license", "conflicting_terms", "missing_owner", "expiring_exception"} {
		if !kinds[want] {
			t.Errorf("missing %s: %#v", want, made.Blockers)
		}
	}
	var catalog provenancepolicies.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("public policy unavailable: %#v", catalog)
	}
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		provenancepolicies.Input
	}{1, in}
	body, _ = json.Marshal(revision)
	var revised provenancepolicies.Policy
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/versions", owner, string(body), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 || revised.Versions[1].AuthorID != "owner" {
		t.Fatalf("version history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/versions", owner, string(body), http.StatusConflict, nil)
}
