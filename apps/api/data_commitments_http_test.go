package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func dataCommitmentBody(owner, purpose, retention, status, expiry string) string {
	return `{"title":"Product analytics boundaries","scopes":[{"kind":"repository","name":"whole product"},{"kind":"release","resource_id":"v2","name":"version 2"},{"kind":"extension","resource_id":"insights","name":"Insights"},{"kind":"experiment","resource_id":"onboarding","name":"Onboarding experiment"},{"kind":"environment","resource_id":"eu-production","name":"EU production"}],"data_uses":[{"id":"usage","name":"Pseudonymous feature usage","categories":["interaction event"],"purposes":["` + purpose + `"],"subjects":["signed-in contributors"],"collection":"client emits only named events after notice","processing":["aggregate counts","measure adoption"],"sharing":["processor: metrics.example"],"retention":"` + retention + `","residency":["EU"],"deletion":"delete event rows within 7 days of account deletion","consent":"explicit analytics opt-in","owner_ids":["` + owner + `"]}],"guarantees":[{"id":"eu-only","description":"raw events remain in the EU","status":"` + status + `","rationale":"processor backup location is being migrated"}],"owner_ids":["` + owner + `"],"links":[{"kind":"policy","url":"https://example.test/privacy","label":"Privacy policy"},{"kind":"notice","url":"https://example.test/analytics-notice","label":"Analytics notice"}],"exceptions":[{"id":"backup-migration","data_use_ids":["usage"],"guarantee_ids":["eu-only"],"reason":"complete EU backup migration","approved_by":"` + owner + `","expires_at":"` + expiry + `"}],"change_reason":"make permitted analytics use reviewable"}`
}

func TestDataCommitmentPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "privacy", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	ownerToken := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	readerToken := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := datacommitments.New(t.TempDir())
	mux := http.NewServeMux()
	registerDataCommitmentsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/data-commitments"
	expires := time.Now().UTC().Add(14 * 24 * time.Hour).Format(time.RFC3339)
	var c datacommitments.Commitment
	workflowJSON(t, server.URL, http.MethodPost, base, ownerToken, dataCommitmentBody("owner", "improve onboarding", "30 days", "partial", expires), 201, &c)
	if c.CurrentVersion != 1 {
		t.Fatalf("unexpected contract: %+v", c)
	}
	var list struct {
		Items []datacommitments.Commitment `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, readerToken, "", 200, &list)
	kinds := map[string]bool{}
	for _, b := range list.Items[0].Blockers {
		kinds[b.Kind] = true
	}
	if !kinds["unsupported_guarantee"] || !kinds["expiring_exception"] || list.Items[0].Versions[0].AuthorID != "owner" {
		t.Fatalf("missing attributable blockers: %+v", list.Items)
	}
	body := dataCommitmentBody("owner", "improve onboarding", "30 days", "supported", expires)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+c.ID+"/versions", ownerToken, body[:1]+`"expected_version":0,`+body[1:], 409, nil)
}

func TestDataCommitmentsExposeMissingOwnersAndConflicts(t *testing.T) {
	store, _ := datacommitments.New(t.TempDir())
	expiry := time.Now().UTC().Add(90 * 24 * time.Hour)
	input := func(purpose, retention string) datacommitments.VersionInput {
		return datacommitments.VersionInput{Title: "Use", Scopes: []datacommitments.Scope{{Kind: "environment", ResourceID: "prod", Name: "Production"}}, DataUses: []datacommitments.DataUse{{ID: "events", Name: "Events", Categories: []string{"usage"}, Purposes: []string{purpose}, Subjects: []string{"users"}, Collection: "application", Processing: []string{"count"}, Sharing: []string{"none"}, Retention: retention, Residency: []string{"EU"}, Deletion: "account deletion", Consent: "opt in"}}, Guarantees: []datacommitments.Guarantee{{ID: "g", Description: "EU only", Status: "supported"}}, Links: []datacommitments.Link{{Kind: "policy", URL: "https://example.test/p"}, {Kind: "notice", URL: "https://example.test/n"}}, Exceptions: []datacommitments.Exception{{ID: "e", DataUseIDs: []string{"events"}, Reason: "transition", ApprovedBy: "owner", ExpiresAt: expiry}}, ChangeReason: "publish"}
	}
	_, _ = store.Create("repo", "owner", input("improve", "30 days"))
	_, _ = store.Create("repo", "owner", input("advertising", "1 year"))
	items, _ := store.List("repo")
	k := map[string]bool{}
	for _, c := range items {
		for _, b := range c.Blockers {
			k[b.Kind] = true
		}
	}
	if !k["missing_ownership"] || !k["conflicting_commitment"] {
		t.Fatalf("missing blockers: %+v", items)
	}
}
