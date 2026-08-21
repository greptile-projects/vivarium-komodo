package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectboundaries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectdeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectlife"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectreadiness"
)

// TestProjectIncubationCompleteWorkflow proves that the separately governed
// incubation resources retain one attributable chain from need to stewardship.
// The lifecycle transitions cross the same public HTTP handlers used by the web
// application; direct store calls compact repeated evidence assembly already
// covered by the focused component workflows.
func TestProjectIncubationCompleteWorkflow(t *testing.T) {
	credentials, _ := auth.New(t.TempDir())
	incubators, _ := projectincubators.New(t.TempDir())
	boundaries, _ := projectboundaries.New(t.TempDir())
	deliveries, _ := projectdeliveries.New(t.TempDir())
	readiness, _ := projectreadiness.New(t.TempDir())
	life, _ := projectlife.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectBoundariesHTTP(mux, boundaries, incubators, credentials)
	registerProjectDeliveriesHTTP(mux, deliveries, incubators, boundaries, credentials)
	registerProjectReadinessHTTP(mux, readiness, incubators, boundaries, deliveries, credentials)
	registerProjectLifeHTTP(mux, life, readiness, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	founder := issueAccess(t, credentials, "founder", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	expert := issueAccess(t, credentials, "domain-expert", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent:onboarding-7", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	target := issueAccess(t, credentials, "target-user", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	need := projectincubators.Input{Title: "Shared compiler", Audience: "small language teams", Problem: "tooling is fragmented", DesiredOutcome: "compile safely in one command", Constraints: []string{"no source retention"}, SuccessMeasures: []string{"first compile in five minutes"}, SponsorIDs: []string{"founder"}, DecisionRights: []string{"founder accepts direction"}, Visibility: "public"}
	inc, err := incubators.Create("founder", need, projectincubators.Source{Kind: "idea", Status: "accessible"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := incubators.Create("domain-expert", need, projectincubators.Source{Kind: "idea", Status: "accessible"})
	if err != nil || len(duplicate.DuplicateIDs) != 1 {
		t.Fatalf("duplicate need was not retained symmetrically: %#v, %v", duplicate, err)
	}
	inc, err = incubators.Invite(inc.ID, "founder", projectincubators.Participant{Kind: "human", UserID: "domain-expert", Role: "compiler domain expert"})
	if err != nil {
		t.Fatal(err)
	}
	inc, err = incubators.Consent(inc.ID, inc.Participants[len(inc.Participants)-1].ID, "domain-expert", "accepted")
	if err != nil {
		t.Fatal(err)
	}
	inc, err = incubators.Invite(inc.ID, "founder", projectincubators.Participant{Kind: "agent", AgentIdentity: "agent:onboarding-7", Role: "bounded researcher", OnboardingID: "onboarding-7", OnboardingScopeKind: "organization", OnboardingScopeID: "tools"})
	if err != nil {
		t.Fatal(err)
	}

	rejected := projectincubators.Alternative{Title: "browser prototype", ProductBoundary: "browser only", Architecture: "single page prototype", Interfaces: []string{"web"}, Licenses: []string{"MIT"}, OperatingCosts: []string{"USD 2/month"}, SecurityRisks: []string{"untrusted input"}, DataRisks: []string{"local source"}, BuildOrAdopt: "adopt prototype"}
	inc, _ = incubators.AddAlternative(inc.ID, "domain-expert", rejected)
	inc, _ = incubators.AddExperiment(inc.ID, "agent:onboarding-7", projectincubators.Experiment{AlternativeID: inc.Alternatives[0].ID, Question: "does the prototype preserve compiler isolation?", Method: []string{"run synthetic source"}, Inputs: []string{"fixture:v1"}, Commands: []string{"prototype test --network=none"}, SuccessCriteria: []string{"no host access"}, Budget: "USD 2", SafetyBoundary: "networkless synthetic workspace"})
	inc, _ = incubators.AddAttempt(inc.ID, inc.Experiments[0].ID, "agent:onboarding-7", projectincubators.ExperimentAttempt{InputDigest: "sha256:prototype-v1", Outcome: "failed", Measurements: map[string]string{"isolation": "failed", "cost": "USD 1.80"}, Artifacts: []string{"sha256:trace"}, Notes: "prototype escaped its declared boundary"})
	chosen := projectincubators.Alternative{Title: "isolated Go service", ProductBoundary: "API and CLI", Architecture: "networkless compiler workers", Interfaces: []string{"HTTP", "CLI"}, Dependencies: []string{"Go runtime"}, Licenses: []string{"Apache-2.0"}, OperatingCosts: []string{"USD 13/month"}, SecurityRisks: []string{"untrusted source"}, DataRisks: []string{"ephemeral metadata"}, BuildOrAdopt: "build bounded service", SupersedesID: inc.Alternatives[0].ID}
	inc, _ = incubators.AddAlternative(inc.ID, "domain-expert", chosen)
	inc, _ = incubators.AddExperiment(inc.ID, "agent:onboarding-7", projectincubators.Experiment{AlternativeID: inc.Alternatives[1].ID, Question: "does the worker contain untrusted source?", Method: []string{"run synthetic source"}, Inputs: []string{"fixture:v2"}, Commands: []string{"go test ./..."}, SuccessCriteria: []string{"contained"}, Budget: "USD 3", SafetyBoundary: "networkless synthetic workspace"})
	inc, _ = incubators.AddAttempt(inc.ID, inc.Experiments[1].ID, "agent:onboarding-7", projectincubators.ExperimentAttempt{InputDigest: "sha256:worker-v2", Outcome: "passed", Measurements: map[string]string{"containment": "passed", "cost": "USD 2.40"}, Notes: "reproducible bounded result"})
	inc, _ = incubators.AcceptAlternative(inc.ID, inc.Alternatives[1].ID, "founder")

	kinds := []string{"organization", "repository", "team", "package", "agent_role", "contributor_pathway", "documentation", "environment", "review_policy", "security_policy", "privacy_policy", "quality_policy", "release_policy"}
	resources := make([]projectboundaries.Resource, 0, len(kinds))
	for _, kind := range kinds {
		resources = append(resources, projectboundaries.Resource{Kind: kind, Mode: "create", Name: "compiler-" + kind, OwnerIDs: []string{"founder"}, MonthlyCost: 1})
	}
	boundaryInput := projectboundaries.Input{IncubatorID: inc.ID, AlternativeID: inc.AcceptedAlternativeID, Title: "Shared compiler", Visibility: "public", OwnerIDs: []string{"founder"}, Resources: resources, RecurringCostLimit: 10}
	var boundary projectboundaries.Boundary
	workflowBody(t, server.URL, http.MethodPost, "/project-boundaries", founder, boundaryInput, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", founder, `{"revision":1}`, http.StatusConflict, nil) // budget breach and missing owner approval
	if !strings.Contains(strings.Join(boundary.ActivationBlockers, ","), "cost") {
		t.Fatalf("bootstrap budget blocker missing: %#v", boundary)
	}
	// A corrected manifest is a new revision-bound attempt; no partial handles leaked.
	boundaryInput.RecurringCostLimit = 15
	workflowBody(t, server.URL, http.MethodPost, "/project-boundaries", founder, boundaryInput, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/approvals", founder, `{"revision":1,"decision":"approved","reason":"resources and recurring cost accepted"}`, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", founder, `{"revision":1}`, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/rollback", founder, `{"revision":1,"reason":"partial external bootstrap signal; release created handles"}`, http.StatusCreated, &boundary)
	workflowJSON(t, server.URL, http.MethodPost, "/project-boundaries/"+boundary.ID+"/activation", founder, `{"revision":1}`, http.StatusCreated, &boundary)

	steps := []projectdeliveries.Step{}
	for i, kind := range []string{"code", "tests", "documentation", "infrastructure", "interface"} {
		steps = append(steps, projectdeliveries.Step{ID: "step-" + kind, Order: i + 1, Kind: kind, Title: kind, OwnerID: "founder", AcceptanceCriteria: []string{"reviewed at exact revision"}})
	}
	deliveryInput := projectdeliveries.Input{IncubatorID: inc.ID, BoundaryID: boundary.ID, BoundaryRevision: boundary.Revision, AlternativeID: inc.AcceptedAlternativeID, Journey: "compile synthetic source", SuccessCriteria: []string{"target user compiles"}, CostLimit: 8, Steps: steps, Team: []projectdeliveries.Member{{ID: "lead", Kind: "human", SubjectID: "founder", Role: "lead", Scope: "slice", ExpiresAt: time.Now().Add(time.Hour)}, {ID: "expert", Kind: "human", SubjectID: "domain-expert", Role: "reviewer", Scope: "review", ExpiresAt: time.Now().Add(time.Hour)}, {ID: "agent", Kind: "agent", SubjectID: "agent:onboarding-7", ParticipantID: inc.Participants[2].ID, Role: "implementer", Scope: "code and tests", ExpiresAt: time.Now().Add(time.Hour)}}}
	var delivery projectdeliveries.Delivery
	workflowBody(t, server.URL, http.MethodPost, "/project-deliveries", founder, deliveryInput, http.StatusCreated, &delivery)
	for i, step := range steps {
		actor := founder
		if i < 2 {
			actor = agent
		}
		workflowBody(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/workspaces", actor, projectdeliveries.Workspace{StepID: step.ID, RepositoryHandle: boundary.Resources[1].Handle, BaseRevision: "base-1", DefinitionDigest: "sha256:workspace", Commands: []string{"go test ./..."}}, http.StatusCreated, &delivery)
		workspace := delivery.Workspaces[len(delivery.Workspaces)-1]
		workflowBody(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/pull-requests", actor, projectdeliveries.PullRequest{StepID: step.ID, WorkspaceID: workspace.ID, RepositoryHandle: workspace.RepositoryHandle, Revision: "slice-" + step.Kind + "-1", Kind: step.Kind, URL: "https://example.test/pulls/" + step.Kind}, http.StatusCreated, &delivery)
		pull := delivery.PullRequests[len(delivery.PullRequests)-1]
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/pull-requests/"+pull.ID+"/checks", founder, `{"revision":"`+pull.Revision+`","outcome":"passed","name":"ordinary checks"}`, http.StatusCreated, &delivery)
		workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/pull-requests/"+pull.ID+"/reviews", expert, `{"revision":"`+pull.Revision+`","decision":"approved","body":"independent domain review"}`, http.StatusCreated, &delivery)
	}
	pullIDs := []string{}
	for _, p := range delivery.PullRequests {
		pullIDs = append(pullIDs, p.ID)
	}
	preview := projectdeliveries.Preview{Revision: "preview-1", PullIDs: pullIDs, URL: "https://preview.test/1", Journey: delivery.Journey, InvitedUserIDs: []string{"target-user"}}
	workflowBody(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/previews", founder, preview, http.StatusCreated, &delivery)
	previewID := delivery.Previews[0].ID
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/previews/"+previewID+"/evidence", target, `{"revision":"wrong-preview","outcome":"failed","observation":"could not compile"}`, http.StatusConflict, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/previews/"+previewID+"/evidence", target, `{"revision":"preview-1","outcome":"passed","observation":"compiled synthetic project","artifact":"sha256:user-session"}`, http.StatusCreated, &delivery)
	workflowBody(t, server.URL, http.MethodPost, "/project-deliveries/"+delivery.ID+"/activity", agent, projectdeliveries.Activity{Kind: "agent_action", Detail: "implemented isolated worker", Cost: 9, Revision: "slice-code-1"}, http.StatusCreated, &delivery)
	if !strings.Contains(strings.Join(delivery.Blockers, ","), "cost limit") {
		t.Fatalf("delivery budget breach was hidden: %#v", delivery)
	}
	// Retain the breached delivery, then prove a corrected attempt with its own exact revision.
	deliveryInput.CostLimit = 12
	delivery, err = deliveries.Create("founder", deliveryInput)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		delivery, _ = deliveries.Workspace(delivery.ID, "founder", projectdeliveries.Workspace{StepID: step.ID, RepositoryHandle: boundary.Resources[1].Handle, BaseRevision: "base-1", DefinitionDigest: "sha256:workspace", Commands: []string{"test"}})
		w := delivery.Workspaces[len(delivery.Workspaces)-1]
		delivery, _ = deliveries.Pull(delivery.ID, "founder", projectdeliveries.PullRequest{StepID: step.ID, WorkspaceID: w.ID, RepositoryHandle: w.RepositoryHandle, Revision: "ready-" + step.Kind, Kind: step.Kind, URL: "https://example.test/p/" + step.Kind})
		p := delivery.PullRequests[len(delivery.PullRequests)-1]
		delivery, _ = deliveries.CheckReview(delivery.ID, p.ID, "founder", "check", p.Revision, "passed", "checks")
		delivery, _ = deliveries.CheckReview(delivery.ID, p.ID, "domain-expert", "review", p.Revision, "approved", "review")
	}
	pullIDs = nil
	for _, p := range delivery.PullRequests {
		pullIDs = append(pullIDs, p.ID)
	}
	delivery, _ = deliveries.Preview(delivery.ID, "founder", projectdeliveries.Preview{Revision: "launch-1", PullIDs: pullIDs, URL: "https://preview.test/launch", Journey: delivery.Journey, InvitedUserIDs: []string{"target-user"}})
	delivery, _ = deliveries.Evidence(delivery.ID, delivery.Previews[0].ID, "target-user", "passed", "useful", "sha256:preview", "launch-1")

	owners := map[string][]string{}
	for _, c := range projectreadiness.RequiredCategories {
		owners[c] = []string{"founder"}
	}
	owners["security_privacy"] = []string{"domain-expert"}
	readyInput := projectreadiness.Input{IncubatorID: inc.ID, BoundaryID: boundary.ID, BoundaryRevision: boundary.Revision, AlternativeID: inc.AcceptedAlternativeID, DeliveryID: delivery.ID, DeliveryRevision: delivery.Revision, LaunchRevision: "release-1", DeclaredScope: "public", RequiredOwners: owners}
	var ready projectreadiness.Readiness
	workflowBody(t, server.URL, http.MethodPost, "/project-readiness", founder, readyInput, http.StatusCreated, &ready)
	for _, c := range projectreadiness.RequiredCategories {
		ownerToken := founder
		if c == "security_privacy" {
			ownerToken = expert
		}
		ev := projectreadiness.Evidence{Category: c, Reference: "https://evidence.test/" + c, Digest: "sha256:" + c, Summary: "passing launch evidence", Outcome: "passed", SafeDefaults: true, SupportedPromises: true, UserValidated: true}
		if c == "ownership" {
			ev.MaintainerIDs = []string{"founder"}
		}
		workflowBody(t, server.URL, http.MethodPost, "/project-readiness/"+ready.ID+"/evidence", ownerToken, map[string]any{"revision": ready.Revision, "evidence": ev}, http.StatusCreated, &ready)
	}
	for _, c := range projectreadiness.RequiredCategories {
		if c == "security_privacy" {
			continue
		}
		workflowBody(t, server.URL, http.MethodPost, "/project-readiness/"+ready.ID+"/decisions", founder, map[string]any{"revision": ready.Revision, "decision": projectreadiness.Decision{Category: c, EvidenceDigest: "sha256:" + c, Decision: "accepted", Reason: "current evidence accepted"}}, http.StatusCreated, &ready)
	}
	if ready.Ready || !strings.Contains(strings.Join(ready.Blockers, ","), "security_privacy") {
		t.Fatalf("unavailable owner did not block launch: %#v", ready)
	}
	workflowBody(t, server.URL, http.MethodPost, "/project-readiness/"+ready.ID+"/decisions", expert, map[string]any{"revision": ready.Revision, "decision": projectreadiness.Decision{Category: "security_privacy", EvidenceDigest: "sha256:security_privacy", Decision: "accepted", Reason: "owner is available and accepts current evidence"}}, http.StatusCreated, &ready)

	lifeInput := projectlife.Input{IncubatorID: inc.ID, AlternativeID: inc.AcceptedAlternativeID, BoundaryID: boundary.ID, BoundaryRevision: boundary.Revision, DeliveryID: delivery.ID, DeliveryRevision: delivery.Revision, ReadinessID: ready.ID, ReadinessRevision: ready.Revision, LaunchRevision: "release-1", Audience: "public", OwnerIDs: []string{"founder"}}
	var record projectlife.Record
	workflowBody(t, server.URL, http.MethodPost, "/project-life", founder, lifeInput, http.StatusCreated, &record)
	badPublication := projectlife.Publication{Kind: "release", Revision: "release-rollback", Reference: "https://releases.test/rollback", Digest: "sha256:bad", Audience: "public", Attestation: "failed launch health; rolled back"}
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/publications", founder, map[string]any{"revision": record.Revision, "publication": badPublication}, http.StatusUnprocessableEntity, nil)
	for _, p := range []projectlife.Publication{{Kind: "release", Revision: "release-1", Reference: "https://releases.test/1", Digest: "sha256:release", Audience: "public", Attestation: "owner-attested exact release"}, {Kind: "contributor_opportunity", Revision: "release-1", Reference: "https://docs.test/contributing", Digest: "sha256:contributing", Audience: "public", Attestation: "reviewed contributor pathway"}} {
		workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/publications", founder, map[string]any{"revision": record.Revision, "publication": p}, http.StatusCreated, &record)
	}
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/signals", founder, map[string]any{"revision": record.Revision, "signal": projectlife.Signal{Kind: "adoption", Measure: "weekly teams", Value: 7, Unit: "teams", EvidenceReference: "product:adoption-1", EvidenceDigest: "sha256:adoption", ObservedAt: time.Now()}}, http.StatusCreated, &record)
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/feedback", founder, map[string]any{"revision": record.Revision, "feedback": projectlife.Feedback{Audience: "public users", Summary: "add editor diagnostics", EvidenceReference: "feedback:42"}}, http.StatusCreated, &record)
	feedbackID := record.Feedback[0].ID
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/work", founder, map[string]any{"revision": record.Revision, "work": projectlife.Work{FeedbackID: feedbackID, Kind: "agent", OwnerID: "agent:onboarding-7", Title: "draft diagnostics proposal", Reference: "task:diagnostics"}}, http.StatusCreated, &record)
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/roadmap", founder, map[string]any{"revision": record.Revision, "roadmap": projectlife.RoadmapChange{FeedbackID: feedbackID, Revision: "roadmap-2", Summary: "prioritize editor diagnostics", EvidenceReferences: []string{"feedback:42", "product:adoption-1"}}}, http.StatusCreated, &record)
	disposition := projectlife.Disposition{State: "graduated", TargetReference: "organization:developer-tools", Reason: "real adoption and continuing owners", Obligations: []projectlife.Obligation{{Kind: "repository", ResourceReference: boundary.Resources[1].Handle, Resolution: "owned by developer-tools maintainers", EvidenceReference: "governance:graduation-1"}, {Kind: "support", ResourceReference: "support:compiler", Resolution: "founder rotation established", EvidenceReference: "support:rotation-1"}}}
	workflowBody(t, server.URL, http.MethodPost, "/project-life/"+record.ID+"/disposition", founder, map[string]any{"revision": record.Revision, "disposition": disposition}, http.StatusCreated, &record)
	if record.AuthorityGranted || len(record.Blockers) > 0 || record.Disposition.State != "graduated" || len(record.Work) != 1 || len(record.Roadmap) != 1 {
		b, _ := json.MarshalIndent(record, "", "  ")
		t.Fatalf("incubation trail did not reach contained stewardship: %s", b)
	}
}
