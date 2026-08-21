package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectdeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectreadiness"
)

// TestProjectReadinessWorkflow is the black-box boundary from a proven first slice to an evidence-backed, deliberately scoped launch.
func TestProjectReadinessWorkflow(t *testing.T) {
	credentials, _ := auth.New(t.TempDir())
	incubators, _ := projectincubators.New(t.TempDir())
	boundaries, _ := projectboundaries.New(t.TempDir())
	deliveries, _ := projectdeliveries.New(t.TempDir())
	readiness, _ := projectreadiness.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectReadinessHTTP(mux, readiness, incubators, boundaries, deliveries, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	inc, _ := incubators.Create("maintainer", projectincubators.Input{Title: "Shared compiler", Audience: "language teams", Problem: "fragmented tools", DesiredOutcome: "safe compilation", SuccessMeasures: []string{"compile sample"}, SponsorIDs: []string{"maintainer"}, DecisionRights: []string{"maintainer launches"}, Visibility: "public"}, projectincubators.Source{Kind: "idea", Status: "accessible"})
	inc, _ = incubators.AddAlternative(inc.ID, "maintainer", projectincubators.Alternative{Title: "service", ProductBoundary: "API", Architecture: "service", Interfaces: []string{"HTTP"}, Licenses: []string{"Apache-2.0"}, OperatingCosts: []string{"$10"}, SecurityRisks: []string{"inputs"}, DataRisks: []string{"metadata"}, BuildOrAdopt: "build"})
	inc, _ = incubators.AcceptAlternative(inc.ID, inc.Alternatives[0].ID, "maintainer")
	resources := []projectboundaries.Resource{}
	for _, k := range []string{"organization", "repository", "team", "package", "agent_role", "contributor_pathway", "documentation", "environment", "review_policy", "security_policy", "privacy_policy", "quality_policy", "release_policy"} {
		resources = append(resources, projectboundaries.Resource{Kind: k, Mode: "create", Name: k, OwnerIDs: []string{"maintainer"}})
	}
	b, _ := boundaries.Create("maintainer", projectboundaries.Input{IncubatorID: inc.ID, AlternativeID: inc.AcceptedAlternativeID, Title: "compiler", Visibility: "public", OwnerIDs: []string{"maintainer"}, Resources: resources, RecurringCostLimit: 20})
	b, _ = boundaries.Decide(b.ID, "maintainer", "approved", "ready", 1)
	b, _ = boundaries.Activate(b.ID, "maintainer", 1)
	steps := []projectdeliveries.Step{}
	for i, k := range []string{"code", "tests", "documentation", "infrastructure", "interface"} {
		steps = append(steps, projectdeliveries.Step{ID: "s-" + k, Order: i + 1, Kind: k, Title: k, OwnerID: "maintainer", AcceptanceCriteria: []string{"reviewed"}})
	}
	d, _ := deliveries.Create("maintainer", projectdeliveries.Input{IncubatorID: inc.ID, BoundaryID: b.ID, BoundaryRevision: b.Revision, AlternativeID: inc.AcceptedAlternativeID, Journey: "compile", SuccessCriteria: []string{"works"}, CostLimit: 20, Steps: steps, Team: []projectdeliveries.Member{{ID: "m", Kind: "human", SubjectID: "maintainer", Role: "lead", Scope: "slice", ExpiresAt: time.Now().Add(time.Hour)}, {ID: "r", Kind: "human", SubjectID: "reviewer", Role: "reviewer", Scope: "review", ExpiresAt: time.Now().Add(time.Hour)}}})
	for _, s := range steps {
		d, _ = deliveries.Workspace(d.ID, "maintainer", projectdeliveries.Workspace{StepID: s.ID, RepositoryHandle: b.Resources[1].Handle, BaseRevision: "base", DefinitionDigest: "sha256:workspace", Commands: []string{"test"}})
		w := d.Workspaces[len(d.Workspaces)-1]
		d, _ = deliveries.Pull(d.ID, "maintainer", projectdeliveries.PullRequest{StepID: s.ID, WorkspaceID: w.ID, RepositoryHandle: w.RepositoryHandle, Revision: "revision-" + s.ID, Kind: s.Kind, URL: "https://example.test/pull"})
		p := d.PullRequests[len(d.PullRequests)-1]
		d, _ = deliveries.CheckReview(d.ID, p.ID, "maintainer", "check", p.Revision, "passed", "checks")
		d, _ = deliveries.CheckReview(d.ID, p.ID, "reviewer", "review", p.Revision, "approved", "reviewed")
	}
	pullIDs := []string{}
	for _, p := range d.PullRequests {
		pullIDs = append(pullIDs, p.ID)
	}
	d, _ = deliveries.Preview(d.ID, "maintainer", projectdeliveries.Preview{Revision: "slice-1", PullIDs: pullIDs, URL: "https://preview.test", Journey: "compile", InvitedUserIDs: []string{"target-user"}})
	d, _ = deliveries.Evidence(d.ID, d.Previews[0].ID, "target-user", "passed", "compiled", "sha256:preview", "slice-1")
	if len(d.Blockers) > 0 {
		t.Fatalf("fixture unexpectedly blocked: %v", d.Blockers)
	}
	owners := map[string][]string{}
	for _, c := range projectreadiness.RequiredCategories {
		owners[c] = []string{"maintainer"}
	}
	in := projectreadiness.Input{IncubatorID: inc.ID, BoundaryID: b.ID, BoundaryRevision: b.Revision, AlternativeID: inc.AcceptedAlternativeID, DeliveryID: d.ID, DeliveryRevision: d.Revision, LaunchRevision: "release-1", DeclaredScope: "public", RequiredOwners: owners}
	raw, _ := json.Marshal(in)
	var r projectreadiness.Readiness
	workflowJSON(t, server.URL, http.MethodPost, "/project-readiness", owner, string(raw), 201, &r)
	for _, c := range projectreadiness.RequiredCategories {
		e := projectreadiness.Evidence{Category: c, Reference: "https://example.test/evidence/" + c, Digest: "sha256:" + c, Summary: "current evidence", Outcome: "passed", SafeDefaults: true, SupportedPromises: true, UserValidated: true}
		if c == "ownership" {
			e.MaintainerIDs = []string{"maintainer"}
		}
		body, _ := json.Marshal(map[string]any{"revision": r.Revision, "evidence": e})
		workflowJSON(t, server.URL, http.MethodPost, "/project-readiness/"+r.ID+"/evidence", owner, string(body), 201, &r)
	}
	// Prototype debt needs a bounded exception, follow-up, and narrower launch; every other owner accepts exact evidence.
	for _, c := range projectreadiness.RequiredCategories {
		e := r.Evidence[len(r.Evidence)-1]
		for _, candidate := range r.Evidence {
			if candidate.Category == c {
				e = candidate
			}
		}
		decision := projectreadiness.Decision{Category: c, EvidenceDigest: e.Digest, Decision: "accepted", Reason: "evidence is current"}
		if c == "prototype_debt" {
			e.Outcome = "failed"
			body, _ := json.Marshal(map[string]any{"revision": r.Revision, "evidence": e})
			workflowJSON(t, server.URL, http.MethodPost, "/project-readiness/"+r.ID+"/evidence", owner, string(body), 201, &r)
			decision = projectreadiness.Decision{Category: c, EvidenceDigest: e.Digest, Decision: "exception", Reason: "bounded beta debt", NarrowedScope: "invited beta", ExpiresAt: time.Now().Add(7 * 24 * time.Hour), FollowUpWork: "task:retire-prototype"}
		}
		body, _ := json.Marshal(map[string]any{"revision": r.Revision, "decision": decision})
		workflowJSON(t, server.URL, http.MethodPost, "/project-readiness/"+r.ID+"/decisions", owner, string(body), 201, &r)
	}
	if !r.Ready || r.EffectiveScope != "invited beta" || len(r.Blockers) != 0 || r.AuthorityGranted {
		t.Fatalf("readiness did not safely narrow launch: %#v", r)
	}
}
