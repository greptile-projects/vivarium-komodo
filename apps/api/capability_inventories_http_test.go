package main

import (
	"encoding/json"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCapabilityInventoryPublicAPIKeepsUncertainUseExplicit(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := capabilityinventories.New(t.TempDir())
	mux := http.NewServeMux()
	registerCapabilityInventoriesHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/capability-inventories"
	now := time.Now().UTC()
	in := capabilityinventories.Input{Name: "Legacy checkout", Description: "Everything implementing or consuming checkout v1", SourceRevision: "commit-a", DefinitionPath: ".komodo/capabilities/checkout.json", OwnerIDs: []string{"platform"}, ChangeReason: "inventory before deprecation", Elements: []capabilityinventories.Element{{ID: "route", Kind: "interface", Reference: "POST /checkout", Revision: "openapi-v1", OwnerIDs: []string{"api"}, Description: "legacy checkout route"}, {ID: "flag", Kind: "flag", Reference: "checkout_v1", Revision: "commit-a", Description: "runtime selection"}}, Environments: []capabilityinventories.Environment{{ID: "prod", Name: "Production", Revision: "release-7", OwnerIDs: []string{"operations"}}}, Consumers: []capabilityinventories.Consumer{{ID: "storefront", Kind: "application", Reference: "storefront", Revision: "commit-b", Status: "active", OwnerIDs: []string{"web"}, EnvironmentIDs: []string{"prod"}, ElementIDs: []string{"route"}, Discovery: "static", Audience: "repository"}, {ID: "plugins", Kind: "external", Reference: "third-party plugins", Status: "dynamic", ElementIDs: []string{"route", "flag"}, Discovery: "dynamic", Audience: "public", Detail: "plugins are loaded at runtime"}}, UsageEvidence: []capabilityinventories.UsageEvidence{{ID: "graph", Kind: "dependency_graph", Reference: "graph-7", Revision: "commit-a", ConsumerIDs: []string{"storefront"}, ElementIDs: []string{"route"}, EnvironmentIDs: []string{"prod"}, Status: "current", ObservedAt: now, AuthorID: "scanner"}, {ID: "telemetry", Kind: "telemetry", Reference: "restricted-dashboard", ConsumerIDs: []string{"plugins"}, ElementIDs: []string{"flag"}, EnvironmentIDs: []string{"prod"}, Status: "inaccessible", ObservedAt: now, AuthorID: "analyst", Detail: "reader lacks telemetry audience"}}, CompatibilityPromises: []capabilityinventories.CompatibilityPromise{{ID: "support", Scope: "public API v1", Revision: "policy-v3", ConsumerIDs: []string{"storefront", "plugins"}, OwnerIDs: []string{"api"}, Guarantee: "supported through release 9"}}}
	b, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(b), http.StatusUnauthorized, nil)
	var created capabilityinventories.Inventory
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), http.StatusCreated, &created)
	kinds := map[string]bool{}
	for _, g := range created.Gaps {
		kinds[g.Kind] = true
	}
	for _, want := range []string{"missing_owner", "missing_usage_evidence", "dynamic_consumer", "dynamic_discovery", "inaccessible_usage_evidence", "unverified_consumer_status"} {
		if !kinds[want] {
			t.Errorf("missing %s in %#v", want, created.Gaps)
		}
	}
	if created.RemovalReady {
		t.Fatal("uncertain dynamic use was treated as absent")
	}
	var list capabilityinventories.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatalf("public catalog unavailable: %#v", list)
	}
	in.ChangeReason = "repeat review"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		capabilityinventories.Input
	}{1, in}
	b, _ = json.Marshal(revision)
	var revised capabilityinventories.Inventory
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(b), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 {
		t.Fatalf("history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(b), http.StatusConflict, nil)
}
