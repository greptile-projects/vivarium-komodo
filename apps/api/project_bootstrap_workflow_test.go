package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
)

// TestProjectBootstrapWorkflow is the black-box boundary from an accepted
// pre-repository direction to an inspectable, atomic collaboration boundary.
func TestProjectBootstrapWorkflow(t *testing.T) {
	credentials, _ := auth.New(t.TempDir())
	incubators, _ := projectincubators.New(t.TempDir())
	boundaries, _ := projectboundaries.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectBoundariesHTTP(mux, boundaries, incubators, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "founder", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	inc, err := incubators.Create("founder", projectincubators.Input{Title: "Shared compiler", Audience: "small language teams", Problem: "tooling is fragmented", DesiredOutcome: "safe shared delivery", SuccessMeasures: []string{"first contribution in one day"}, SponsorIDs: []string{"founder"}, DecisionRights: []string{"founder accepts direction"}, Visibility: "public"}, projectincubators.Source{Kind: "idea", Status: "accessible"})
	if err != nil {
		t.Fatal(err)
	}
	inc, err = incubators.AddAlternative(inc.ID, "founder", projectincubators.Alternative{Title: "Go service", ProductBoundary: "API and CLI", Architecture: "modular service", Interfaces: []string{"HTTP"}, Licenses: []string{"Apache-2.0"}, OperatingCosts: []string{"USD 13 monthly"}, SecurityRisks: []string{"untrusted input"}, DataRisks: []string{"source metadata"}, BuildOrAdopt: "build workflow"})
	if err != nil {
		t.Fatal(err)
	}
	inc, err = incubators.AcceptAlternative(inc.ID, inc.Alternatives[0].ID, "founder")
	if err != nil {
		t.Fatal(err)
	}

	kinds := []string{"organization", "repository", "team", "package", "agent_role", "contributor_pathway", "documentation", "environment", "review_policy", "security_policy", "privacy_policy", "quality_policy", "release_policy"}
	resources := make([]projectboundaries.Resource, 0, len(kinds))
	for _, kind := range kinds {
		resources = append(resources, projectboundaries.Resource{Kind: kind, Mode: "create", Name: "compiler-" + kind, OwnerIDs: []string{"founder"}, Access: []projectboundaries.Access{{SubjectID: "contributors", Role: "read", Source: "accepted direction"}}, MonthlyCost: 1, Generated: []projectboundaries.GeneratedContent{{Path: kind + ".md", Template: "baseline-v1", Source: "incubator " + inc.ID + " alternative " + inc.AcceptedAlternativeID, ApprovedByIDs: []string{"founder"}}}, Policies: []projectboundaries.Policy{{Kind: kind, Source: "platform baseline v1", Summary: "owner review and least privilege"}}})
	}
	input := projectboundaries.Input{IncubatorID: inc.ID, AlternativeID: inc.AcceptedAlternativeID, Title: "Shared compiler", Visibility: "public", OwnerIDs: []string{"founder"}, Resources: resources, RecurringCostLimit: 15}
	var boundary projectboundaries.Boundary
	body, _ := json.Marshal(input)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries", owner, string(body), http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", owner, `{"revision":1}`, http.StatusConflict, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/approvals", owner, `{"revision":1,"decision":"approved","reason":"ownership, access, cost, generated content, and policy accepted"}`, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", owner, `{"revision":1}`, http.StatusCreated, &boundary)
	if boundary.State != "active" || len(boundary.Attempts) != 1 {
		t.Fatalf("not atomically active: %#v", boundary)
	}
	workflowJSON(t, server.URL, http.MethodGet, "/project-boundaries/"+boundary.ID, "", "", http.StatusOK, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/rollback", owner, `{"revision":1,"reason":"retry with corrected defaults"}`, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", owner, `{"revision":1}`, http.StatusCreated, &boundary)
	if len(boundary.Attempts) != 3 {
		t.Fatalf("attempt history lost: %#v", boundary.Attempts)
	}
}
