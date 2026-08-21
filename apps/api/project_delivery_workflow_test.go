package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectdeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
)

// TestProjectDeliveryWorkflow is the black-box boundary from an active project
// boundary to a target-user-proven, ordinarily reviewed first product slice.
func TestProjectDeliveryWorkflow(t *testing.T) {
	credentials, _ := auth.New(t.TempDir())
	incubators, _ := projectincubators.New(t.TempDir())
	boundaries, _ := projectboundaries.New(t.TempDir())
	deliveries, _ := projectdeliveries.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectDeliveriesHTTP(mux, deliveries, incubators, boundaries, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "founder", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reviewer := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	user := issueAccess(t, credentials, "target-user", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	inc, _ := incubators.Create("founder", projectincubators.Input{Title: "Shared compiler", Audience: "language teams", Problem: "fragmented tools", DesiredOutcome: "safe compilation", SuccessMeasures: []string{"compile a sample"}, SponsorIDs: []string{"founder"}, DecisionRights: []string{"founder accepts"}, Visibility: "public"}, projectincubators.Source{Kind: "idea", Status: "accessible"})
	inc, _ = incubators.AddAlternative(inc.ID, "founder", projectincubators.Alternative{Title: "Go service", ProductBoundary: "API", Architecture: "service", Interfaces: []string{"HTTP"}, Licenses: []string{"Apache-2.0"}, OperatingCosts: []string{"$10"}, SecurityRisks: []string{"input"}, DataRisks: []string{"metadata"}, BuildOrAdopt: "build"})
	inc, _ = incubators.AcceptAlternative(inc.ID, inc.Alternatives[0].ID, "founder")
	kinds := []string{"organization", "repository", "team", "package", "agent_role", "contributor_pathway", "documentation", "environment", "review_policy", "security_policy", "privacy_policy", "quality_policy", "release_policy"}
	resources := []projectboundaries.Resource{}
	for _, k := range kinds {
		resources = append(resources, projectboundaries.Resource{Kind: k, Mode: "create", Name: k, OwnerIDs: []string{"founder"}})
	}
	b, _ := boundaries.Create("founder", projectboundaries.Input{IncubatorID: inc.ID, AlternativeID: inc.AcceptedAlternativeID, Title: "compiler", Visibility: "public", OwnerIDs: []string{"founder"}, Resources: resources, RecurringCostLimit: 10})
	b, _ = boundaries.Decide(b.ID, "founder", "approved", "ready", 1)
	b, _ = boundaries.Activate(b.ID, "founder", 1)
	exp := time.Now().UTC().Add(time.Hour)
	steps := []projectdeliveries.Step{}
	for i, k := range []string{"code", "tests", "documentation", "infrastructure", "interface"} {
		deps := []string{}
		if i > 0 {
			deps = []string{steps[i-1].ID}
		}
		steps = append(steps, projectdeliveries.Step{ID: "step-" + k, Order: i + 1, Kind: k, Title: "Deliver " + k, OwnerID: "agent:compiler", DependsOnIDs: deps, AcceptanceCriteria: []string{"ordinary review passes"}})
	}
	in := projectdeliveries.Input{IncubatorID: inc.ID, BoundaryID: b.ID, BoundaryRevision: 1, AlternativeID: inc.AcceptedAlternativeID, Journey: "compile a sample safely", SuccessCriteria: []string{"target user compiles sample"}, CostLimit: 20, Steps: steps, Team: []projectdeliveries.Member{{ID: "m1", Kind: "human", SubjectID: "founder", Role: "lead", Scope: "plan and review", ExpiresAt: exp}, {ID: "m2", Kind: "human", SubjectID: "reviewer", Role: "reviewer", Scope: "review only", ExpiresAt: exp}, {ID: "m3", Kind: "agent", SubjectID: "agent:compiler", Role: "builder", Scope: "listed steps only", ExpiresAt: exp}}}
	buf, _ := json.Marshal(in)
	var d projectdeliveries.Delivery
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries", owner, string(buf), 201, &d)
	for i, s := range steps {
		wsBody := fmt.Sprintf(`{"step_id":%q,"repository_handle":%q,"base_revision":"base-1","definition_digest":"sha256:workspace","commands":["go test ./..."]}`, s.ID, b.Resources[1].Handle)
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/workspaces", owner, wsBody, 201, &d)
		ws := d.Workspaces[len(d.Workspaces)-1]
		pullBody := fmt.Sprintf(`{"step_id":%q,"workspace_id":%q,"repository_handle":%q,"revision":%q,"kind":%q,"url":%q}`, s.ID, ws.ID, ws.RepositoryHandle, fmt.Sprintf("rev-%d", i), s.Kind, "https://example.test/pulls/"+s.ID)
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/pull-requests", owner, pullBody, 201, &d)
		p := d.PullRequests[len(d.PullRequests)-1]
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/pull-requests/"+p.ID+"/checks", owner, fmt.Sprintf(`{"revision":%q,"outcome":"passed","name":"ordinary checks"}`, p.Revision), 201, &d)
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/pull-requests/"+p.ID+"/reviews", reviewer, fmt.Sprintf(`{"revision":%q,"decision":"approved","body":"ordinary review"}`, p.Revision), 201, &d)
	}
	pulls := []string{}
	for _, p := range d.PullRequests {
		pulls = append(pulls, p.ID)
	}
	preview, _ := json.Marshal(projectdeliveries.Preview{Revision: "slice-rev-1", PullIDs: pulls, URL: "https://preview.example.test", Journey: in.Journey, InvitedUserIDs: []string{"target-user"}})
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/previews", owner, string(preview), 201, &d)
	p := d.Previews[0]
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/previews/"+p.ID+"/evidence", user, `{"revision":"wrong","outcome":"passed","observation":"worked"}`, 409, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/previews/"+p.ID+"/evidence", user, `{"revision":"slice-rev-1","outcome":"passed","observation":"compiled the sample","artifact":"sha256:evidence"}`, 201, &d)
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+d.ID+"/activity", owner, `{"kind":"handoff","from_id":"founder","to_id":"agent:compiler","step_id":"step-code","detail":"accepted bounded implementation context","cost":3.5,"revision":"rev-0"}`, 201, &d)
	if len(d.Blockers) != 0 || d.TotalCost != 3.5 || d.AuthorityGranted {
		t.Fatalf("slice not proven without implicit authority: %#v", d)
	}
}
