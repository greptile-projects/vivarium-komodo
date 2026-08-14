package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func accessibilityBody(owner, standardVersion, exceptionExpiry string) string {
	return `{"title":"Accessible pull request review","scopes":[{"kind":"journey","resource_id":"pull-review","name":"Review a proposed change"},{"kind":"component","resource_id":"diff","name":"Diff viewer"}],"standards":[{"id":"wcag","name":"WCAG","version":"` + standardVersion + `","level":"AA"}],"assistive_technologies":[{"id":"nvda-firefox","name":"NVDA","version":"2026.1","platform":"Firefox on Windows"},{"id":"voiceover-safari","name":"VoiceOver","platform":"Safari on macOS"}],"target_audiences":["blind contributors","keyboard-only reviewers"],"required_scenarios":[{"id":"review-diff","name":"Navigate and comment on a changed line","scope_ids":["journey:pull-review","component:diff"],"standard_ids":["wcag"],"assistive_technology_ids":["nvda-firefox","voiceover-safari"]}],"severity_policy":[{"severity":"critical","definition":"Cannot complete review","review_effect":"block_review","resolution_target_days":1},{"severity":"high","definition":"Material barrier with workaround","review_effect":"block_merge","resolution_target_days":7}],"owner_ids":["` + owner + `"],"exceptions":[{"id":"voiceover-gap","scenario_ids":["review-diff"],"reason":"Upstream browser defect","approved_by":"` + owner + `","expires_at":"` + exceptionExpiry + `"}],"links":[{"kind":"roadmap_outcome","resource_id":"roadmap-1"},{"kind":"documentation","resource_id":"docs-1"},{"kind":"preview","resource_id":"preview-1"},{"kind":"release_policy","resource_id":"stable"}],"change_reason":"Make review expectations testable"}`
}

func TestAccessibilityCommitmentContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	owner, reader := "owner", "reader"
	repo, _ := catalog.Create(owner, repositories.Metadata{Name: "inclusive", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator(owner, repo.ID, reader)
	ownerToken := issueAccess(t, credentials, owner, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	readerToken := issueAccess(t, credentials, reader, auth.API, auth.RepositoryRead)
	store, _ := accessibilitycommitments.New(t.TempDir())
	mux := http.NewServeMux()
	registerAccessibilityCommitmentsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/accessibility-commitments"
	expires := time.Now().UTC().Add(14 * 24 * time.Hour).Format(time.RFC3339)
	var c accessibilitycommitments.Commitment
	workflowJSON(t, server.URL, http.MethodPost, base, ownerToken, accessibilityBody(owner, "2.2", expires), 201, &c)
	if c.CurrentVersion != 1 {
		t.Fatalf("unexpected commitment: %+v", c)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+c.ID+"/coverage", ownerToken, `{"version":1,"scenario_id":"review-diff","assistive_technology_id":"nvda-firefox","status":"passed","revision":"abc123","evidence":"axe and manual screen-reader transcript"}`, 201, &c)
	if len(c.Blockers) != 1 || c.Blockers[0].Kind != "expiring_exception" || c.Coverage[0].ActorID != owner {
		t.Fatalf("coverage or exception derivation lost: %+v", c)
	}
	var listed struct {
		Items []accessibilitycommitments.Commitment `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, readerToken, "", 200, &listed)
	if len(listed.Items) != 1 || listed.Items[0].Versions[0].AuthorID != owner {
		t.Fatal("reader cannot inspect attributable contract")
	}
	revisionBody := accessibilityBody(owner, "2.2", expires)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+c.ID+"/versions", ownerToken, revisionBody[:1]+`"expected_version":0,`+revisionBody[1:], http.StatusConflict, nil)
}

func TestAccessibilityCommitmentsExposeConflictsAndUnsupportedEnvironments(t *testing.T) {
	store, _ := accessibilitycommitments.New(t.TempDir())
	expiry := time.Now().UTC().Add(90 * 24 * time.Hour).Format(time.RFC3339) // create through JSON to keep the fixture aligned with the public shape
	decode := func(version string) accessibilitycommitments.VersionInput {
		var in accessibilitycommitments.VersionInput
		if err := json.Unmarshal([]byte(accessibilityBody("owner", version, expiry)), &in); err != nil {
			t.Fatal(err)
		}
		return in
	}
	a, _ := store.Create("repo", "owner", decode("2.2"))
	_, _ = store.Create("repo", "owner", decode("2.1"))
	a, _ = store.RecordCoverage("repo", a.ID, "tester", accessibilitycommitments.CoverageInput{Version: 1, ScenarioID: "review-diff", AssistiveTechnologyID: "nvda-firefox", Status: "unsupported", Evidence: "browser/accessibility tree mismatch"})
	items, _ := store.List("repo")
	kinds := map[string]bool{}
	for _, c := range items {
		for _, b := range c.Blockers {
			kinds[b.Kind] = true
		}
	}
	if !kinds["conflicting_requirement"] || !kinds["unsupported_environment"] {
		t.Fatalf("missing explicit blockers: %+v", items)
	}
}
