package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityexpectations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestSecurityExpectationsPublicAPIAndDerivedGaps(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "identity", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := securityexpectations.New(t.TempDir())
	mux := http.NewServeMux()
	registerSecurityExpectationsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/security-expectations"
	in := securityexpectations.Input{
		Name: "Session protection", Description: "Protect authenticated sessions", ChangeReason: "initial model",
		Scopes: []securityexpectations.Scope{{Kind: "service", Reference: "identity"}, {Kind: "user_journey", Reference: "sign-in"}},
		Assets: []securityexpectations.Asset{{ID: "session", Name: "Session", Classification: "critical", Protection: "confidentiality and integrity"}},
		Actors: []securityexpectations.Actor{{ID: "attacker", Description: "internet attacker", Capabilities: []string{"arbitrary requests"}}},
		Boundaries: []securityexpectations.Boundary{
			{ID: "allow", From: "browser", To: "api", AssetIDs: []string{"session"}, Allowed: true, Rationale: "sign in"},
			{ID: "deny", From: "browser", To: "api", AssetIDs: []string{"session"}, Allowed: false, Rationale: "deny unauthenticated access"},
		},
		Controls:       []securityexpectations.Control{{ID: "integrity", Description: "validate session", Guarantee: "forgeries rejected", UnsupportedReason: "legacy endpoint cannot validate"}},
		AbuseCases:     []securityexpectations.AbuseCase{{ID: "theft", ActorID: "attacker", AssetIDs: []string{"session"}, BoundaryIDs: []string{"allow"}, Description: "steal session", Impact: "account takeover", Severity: "critical", ControlIDs: []string{"integrity"}}},
		SeverityPolicy: []securityexpectations.SeverityRule{{Severity: "critical", Response: "block release", ReleaseBlocking: true}},
		Exceptions:     []securityexpectations.Exception{{ID: "legacy", Subject: "integrity", Rationale: "replacement active", OwnerID: "security", ApprovedBy: "owner", ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}},
		Links:          []securityexpectations.Link{{Kind: "privacy", Reference: "account-data", Commitment: "session confidentiality"}, {Kind: "infrastructure", Reference: "edge", Commitment: "TLS termination"}, {Kind: "api", Reference: "auth-v1", Commitment: "reject forgery"}, {Kind: "quality", Reference: "sign-in", Commitment: "abuse regression"}, {Kind: "release", Reference: "identity", Commitment: "critical risks block"}, {Kind: "design", Reference: "sign-in", Commitment: "safe error states"}},
	}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created securityexpectations.Expectation
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	kinds := map[string]bool{}
	for _, gap := range created.Gaps {
		kinds[gap.Kind] = true
	}
	for _, want := range []string{"missing_owner", "contradictory_boundary", "unsupported_guarantee", "expiring_exception"} {
		if !kinds[want] {
			t.Errorf("missing %s: %#v", want, created.Gaps)
		}
	}
	var catalog securityexpectations.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("public catalog unavailable: %#v", catalog)
	}
	in.ChangeReason = "owners reviewed"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		securityexpectations.Input
	}{1, in}
	body, _ = json.Marshal(revision)
	var revised securityexpectations.Expectation
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 || revised.Versions[1].AuthorID != "owner" {
		t.Fatalf("version history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusConflict, nil)
}
