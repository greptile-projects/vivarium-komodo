package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/testscenarios"
)

func TestReusableTestScenarioAPITraceabilityAndFixtureSafety(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := testscenarios.New(t.TempDir())
	mux := http.NewServeMux()
	registerTestScenariosHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/test-scenarios"
	in := reusableScenarioInput()
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created testscenarios.Scenario
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	if created.CurrentVersion != 1 || len(created.Gaps) != 0 {
		t.Fatalf("scenario is not independently understandable: %#v", created)
	}
	v := created.Versions[0]
	if v.Sources[0].Revision != "issue-rev-4" || v.Contribution.ActorKind != "agent" || len(v.Generation.Assumptions) == 0 || !v.Fixtures[0].Synthetic {
		t.Fatalf("traceability, bounded agent provenance, or synthetic data lost: %#v", v)
	}
	var catalog testscenarios.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("public scenario catalog unavailable: %#v", catalog)
	}

	unsafe := reusableScenarioInput()
	unsafe.Name = "Copied customer"
	unsafe.Fixtures[0].Synthetic = false
	unsafe.Fixtures[0].ContainsProductionData = true
	body, _ = json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusUnprocessableEntity, nil)
	unsafe = reusableScenarioInput()
	unsafe.Name = "Hidden research"
	unsafe.Sources[0].Accessible = false
	body, _ = json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusUnprocessableEntity, nil)

	in.ChangeReason = "add cancellation case"
	in.Parameters[0].Values = append(in.Parameters[0].Values, "cancelled")
	revise := struct {
		ExpectedVersion int64 `json:"expected_version"`
		testscenarios.Input
	}{1, in}
	body, _ = json.Marshal(revise)
	var revised testscenarios.Scenario
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 {
		t.Fatalf("immutable revision history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusConflict, nil)
}

func reusableScenarioInput() testscenarios.Input {
	return testscenarios.Input{
		Name: "Checkout creates one order", Purpose: "Prove retries never duplicate an order", SourceRevision: "candidate-abc123", DefinitionPath: ".komodo/scenarios/checkout.json", QualityPlanID: "release-quality", QualityPlanVersion: 2, ChangeReason: "protect reported behavior",
		Sources:       []testscenarios.Source{{ID: "report", Kind: "issue", Reference: "issue-42", Revision: "issue-rev-4", Rationale: "reporter reproduced duplicate creation", Accessible: true}, {ID: "journey", Kind: "journey", Reference: "checkout", Revision: "journey-v3", Rationale: "accepted purchase journey", Accessible: true}},
		Parameters:    []testscenarios.Parameter{{Name: "payment_result", Type: "enum", Description: "sandbox gateway outcome", Values: []string{"accepted", "retry"}, Required: true}},
		Preconditions: []testscenarios.Step{{ID: "empty", Kind: "state", Description: "synthetic customer has no order"}},
		Actions:       []testscenarios.Step{{ID: "run", Kind: "command", Description: "execute the parameterized checkout scenario", Command: "bun test checkout.scenario.test.ts"}},
		Assertions:    []testscenarios.Assertion{{ID: "one", Description: "one durable order exists", Matcher: "count", Expected: "1"}, {ID: "receipt", Description: "receipt matches the order", Matcher: "invariant", Expected: "receipt.order_id == order.id"}},
		Fixtures:      []testscenarios.Fixture{{ID: "customer", Kind: "generator", Description: "deterministic boundary customer and cart", Source: "generator specification", SourceRevision: "fixture-v2", PrivacyClassification: "internal", Synthetic: true, Accessible: true, Transformations: []string{"generate names and addresses"}, Generator: "bun fixtures/generate-checkout.ts --seed 42"}},
		Environments:  []testscenarios.Environment{{ID: "isolated", Description: "credential-free checkout sandbox", Requirements: []string{"bun 1.x", "ephemeral sqlite"}, Network: "none"}},
		Contribution:  testscenarios.Contribution{Kind: "pull_request", Reference: "pull-18", Revision: "candidate-abc123", Branch: "agent/checkout-scenario", ChangedPaths: []string{".komodo/scenarios/checkout.json", "fixtures/generate-checkout.ts"}, Contributor: "agent:quality", ActorKind: "agent", Scope: []string{".komodo/scenarios/**", "fixtures/**"}},
		Generation:    testscenarios.Generation{Generated: true, Generator: "agent:quality profile-v3", Assumptions: []string{"payment sandbox is deterministic", "retry token remains stable"}, Provenance: []string{"issue-42@issue-rev-4", "journey:checkout@journey-v3"}}, Tags: []string{"checkout", "regression"},
	}
}
