package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAPIContractPublicAPIAndAuthorization(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "service", Visibility: repositories.Public})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := apicontracts.New(t.TempDir())
	mux := http.NewServeMux()
	registerAPIContractsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/api-contracts"
	in := apicontracts.Input{Name: "Events", Version: "1.0.0", Description: "Read events", SourceRevision: "reviewed", DefinitionPath: "api/openapi.json", DefinitionFormat: "openapi", DefinitionValid: true, ValidationSummary: "valid", Operations: []apicontracts.Operation{{ID: "list", Method: "GET", Path: "/events", Summary: "List", Authentication: []string{"token"}, ResponseSchema: "Event", ErrorCodes: []string{"denied"}}}, Schemas: []apicontracts.Schema{{Name: "Event", Kind: "object"}}, Errors: []apicontracts.APIError{{Code: "denied", HTTPStatus: 403, Meaning: "scope denied"}}, Authentication: []apicontracts.Authentication{{ID: "token", Kind: "bearer", Description: "scoped token"}}, Environments: []apicontracts.Environment{{Name: "prod", BaseURL: "https://api.test", Availability: "available"}}, OwnerIDs: []string{"owner"}, Stability: "stable", SupportPolicy: "12 months", Compatibility: apicontracts.Compatibility{Promise: "semver"}, Links: []apicontracts.Link{{Kind: "source", ResourceID: "reviewed", Label: "source", Status: "current"}, {Kind: "release", ResourceID: "r1", Label: "v1", Status: "unreleased"}, {Kind: "documentation", ResourceID: "d1", Label: "guide", Status: "stale"}, {Kind: "data_use", ResourceID: "p1", Label: "privacy", Status: "current"}}, ChangeReason: "publish"}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created apicontracts.Contract
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	if created.Versions[0].AuthorID != "owner" || len(created.Gaps) != 2 {
		t.Fatalf("unexpected publication: %#v", created)
	}
	var list struct {
		Items []apicontracts.Contract `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatalf("public contract unavailable: %#v", list)
	}
}
