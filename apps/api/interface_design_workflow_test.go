package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designgovernance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designsystems"
	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestInterfaceDesignWorkflow is the black-box boundary for the complete
// feedback-to-shipped-interface collaboration loop. It intentionally crosses
// the public API seams between design history, ordinary implementation work,
// revision-exact evidence, and owner-governed correction.
func TestInterfaceDesignWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	for _, collaborator := range []string{"designer", "user", "agent"} {
		_, _ = repos.AddCollaborator("owner", repo.ID, collaborator)
	}
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	designer := issueAccess(t, credentials, "designer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	user := issueAccess(t, credentials, "user", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	designs, _ := designsystems.New(t.TempDir())
	proposalsStore, _ := designproposals.New(t.TempDir())
	work, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := interfacechecks.New(t.TempDir())
	governance, _ := designgovernance.New(t.TempDir())
	orgs, _ := organizations.New(t.TempDir())
	mux := http.NewServeMux()
	registerDesignSystemsHTTP(mux, designs, repos, credentials)
	registerDesignProposalsHTTP(mux, proposalsStore, repos, credentials, work, pulls)
	registerInterfaceChecksHTTP(mux, checks, repos, credentials, interfaceCheckSources{pulls: pulls, repositories: repos, designs: proposalsStore})
	registerDesignGovernanceHTTP(mux, governance, checks, repos, orgs, credentials, pulls)
	server := httptest.NewServer(mux)
	defer server.Close()
	repoBase := "/repositories/" + string(repo.ID)

	// The shared system changes its focus token and explicitly carries a
	// downstream consumer migration rather than silently changing the UI.
	systemInput := designsystems.Input{Name: "Core", Description: "Product interaction language", SourceRevision: "design-source-1", DefinitionPath: "design/core.json", ReleaseRevision: "release-1", Tokens: []designsystems.Token{{Name: "focus.ring", Category: "color", Value: "#0969da", Description: "Keyboard focus"}}, Components: []designsystems.Component{{Name: "PublishDialog", Purpose: "Confirm publication", Usage: "Review impact before publishing", Props: []string{"state"}}}, Patterns: []designsystems.Pattern{{Name: "Confirm", Trigger: "activate", Behavior: "publish once", Feedback: "announce success", Keyboard: "focus returns to trigger"}}, ContentRules: []designsystems.ContentRule{{Name: "Consequences", Guidance: "name the effect", Example: "Publish changes"}}, ResponsiveRules: []designsystems.ResponsiveRule{{Name: "compact", MinimumWidth: 0, MaximumWidth: 639, Behavior: "stack actions"}}, Themes: []designsystems.Theme{{Name: "light", Purpose: "default", TokenOverrides: map[string]string{}}}, Examples: []designsystems.Example{{Name: "Review", Subject: "PublishDialog", Markup: "<button>Publish changes</button>", Theme: "light", Locale: "en", Viewport: "compact"}}, Accessibility: []designsystems.Constraint{{Subject: "PublishDialog", Requirement: "visible focus and announced result"}}, Localization: []designsystems.Constraint{{Subject: "PublishDialog", Requirement: "supports 200% text expansion"}}, OwnerIDs: []string{"owner"}, Adoption: designsystems.AdoptionPolicy{Required: true, Consumers: []string{"web", "docs"}, ReviewCadence: "each release"}, Consumers: []designsystems.Consumer{{Name: "web", ImplementationRevision: "old-web", AdoptedVersion: 1, Status: "current"}}, Provenance: []designsystems.Provenance{{Kind: "research", Reference: "feedback-publish-confusion", Rationale: "reported journey need"}}, ChangeReason: "ground the experience need"}
	body, _ := json.Marshal(systemInput)
	var system designsystems.System
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-systems", owner, string(body), http.StatusCreated, &system)
	systemInput.SourceRevision = "design-source-2"
	systemInput.ReleaseRevision = "release-2"
	systemInput.Tokens[0].Value = "#8250df"
	systemInput.Consumers = []designsystems.Consumer{{Name: "web", ImplementationRevision: "candidate", AdoptedVersion: 2, Status: "current"}, {Name: "docs", ImplementationRevision: "docs-migration", AdoptedVersion: 2, Status: "planned"}}
	systemInput.ChangeReason = "increase focus contrast and migrate consumers"
	revisionBody, _ := json.Marshal(struct {
		ExpectedVersion int64 `json:"expected_version"`
		designsystems.Input
	}{1, systemInput})
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-systems/"+system.ID+"/versions", owner, string(revisionBody), http.StatusCreated, &system)
	if system.CurrentVersion != 2 || system.Versions[1].Consumers[1].Status != "planned" {
		t.Fatalf("token change lost downstream migration: %#v", system)
	}

	proposalInput := designproposals.Input{Title: "Understand publication", Origin: designproposals.Origin{Kind: "feedback", ID: "feedback-publish-confusion"}, UserGoal: "know what will happen before publishing", Journeys: []designproposals.Journey{{Name: "publish", Steps: []string{"review", "confirm", "success"}, Outcome: "published with confidence"}}, States: []designproposals.State{{Name: "review", Behavior: "show impact", Content: "Review changes"}, {Name: "error", Behavior: "retain input and explain recovery", Content: "Could not publish"}, {Name: "success", Behavior: "return focus and announce completion", Content: "Published"}}, Content: []string{"Review changes", "Publish changes", "Cancel"}, Constraints: []designproposals.Constraint{{Kind: "accessibility", Requirement: "keyboard and screen-reader complete"}, {Kind: "localization", Requirement: "German expanded copy fits compact layout"}}, Alternatives: []designproposals.Alternative{{Name: "instant publish", Tradeoff: "faster but error-prone", Reason: "agent-assisted alternative retained for comparison"}}, SuccessMeasures: []designproposals.Measure{{Name: "successful journey", Target: "95%"}}, AffectedComponents: []string{"PublishDialog"}, ComponentContracts: []designproposals.ComponentContract{{Name: "PublishDialog", Contract: "review, error, success, cancel, and focus return"}}, Breakpoints: []designproposals.Breakpoint{{Name: "compact", MinimumWidth: 0, MaximumWidth: 639, Behavior: "stack actions"}, {Name: "wide", MinimumWidth: 640, Behavior: "align actions"}}, Evidence: []designproposals.Evidence{{ID: "need", Kind: "feedback", Reference: "feedback-publish-confusion", Summary: "users cannot predict the effect", Audience: "repository"}}, Uncertainty: []string{"expanded translation length"}, ChangeReason: "compare human and agent alternatives"}
	body, _ = json.Marshal(proposalInput)
	var proposal designproposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals", designer, string(body), http.StatusCreated, &proposal)
	for _, invitation := range []string{`{"expected_version":1,"subject_id":"agent","kind":"agent","role":"reviewer","grounded_evidence_ids":["need"]}`, `{"expected_version":1,"subject_id":"designer","kind":"designer","role":"author","grounded_evidence_ids":["need"]}`, `{"expected_version":1,"subject_id":"user","kind":"user","role":"research_participant","grounded_evidence_ids":["need"]}`} {
		workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/participants", designer, invitation, http.StatusCreated, &proposal)
	}
	artifact := `{"kind":"interactive_prototype","title":"Responsive confirmation","proposal_revision":1,"frames":[{"name":"review","description":"compact and wide review","format":"html","body":"<button>Publish changes</button>"},{"name":"success","description":"announced completion","format":"html","body":"<p role=status>Published</p>"}],"interactions":[{"trigger":"keyboard activate","action":"publish","result":"announce success and return focus"}],"assets":[{"id":"publish","name":"Publish mark","source":"design://publish/v2","author_id":"designer","license":"CC-BY-4.0","transformations":["SVG export"]}],"evidence_ids":["need"],"uncertainty":["missing failure state"],"change_reason":"designer and invited-user review"}`
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/artifacts", designer, artifact, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/comments", agent, `{"subject_kind":"artifact","subject_id":"`+proposal.Artifacts[0].ID+`","subject_revision":1,"body":"Instant publication is faster but loses informed control","stance":"dissent","evidence_ids":["need"],"uncertainty":"error recovery"}`, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/comments", user, `{"subject_kind":"artifact","subject_id":"`+proposal.Artifacts[0].ID+`","subject_revision":1,"body":"I can predict the result and recover from error","stance":"support","evidence_ids":["need"]}`, http.StatusCreated, &proposal)
	correctedArtifact := `{"expected_version":1,"kind":"interactive_prototype","title":"Responsive confirmation with recovery","proposal_revision":1,"frames":[{"name":"review","description":"compact and wide review","format":"html","body":"<button>Publish changes</button>"},{"name":"error","description":"recoverable error retaining input","format":"html","body":"<p role=status>Could not publish</p>"},{"name":"success","description":"announced completion","format":"html","body":"<p role=status>Published</p>"}],"interactions":[{"trigger":"keyboard activate","action":"publish","result":"announce success or recoverable error and return focus"}],"assets":[{"id":"publish","name":"Publish mark","source":"design://publish/v2","author_id":"designer","license":"CC-BY-4.0","transformations":["SVG export"]}],"evidence_ids":["need"],"change_reason":"add the invited user's missing failure state"}`
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/artifacts/"+proposal.Artifacts[0].ID+"/revisions", designer, correctedArtifact, http.StatusCreated, &proposal)
	if proposal.Artifacts[0].CurrentVersion != 2 || len(proposal.Artifacts[0].Revisions[0].Frames) != 2 || len(proposal.Artifacts[0].Revisions[1].Frames) != 3 {
		t.Fatalf("missing state correction did not retain the stale prototype: %#v", proposal.Artifacts[0])
	}
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/acknowledgements", designer, `{"expected_version":1,"owner_id":"owner"}`, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-proposals/"+proposal.ID+"/acknowledgements/"+proposal.Acknowledgements[0].ID+"/response", owner, `{"status":"acknowledged","rationale":"responsive accessible prototype accepted"}`, http.StatusOK, &proposal)

	opened, _ := repos.Open(repo.ID)
	config := `{"schema_version":1,"specification":{"kind":"design_proposal","id":"` + proposal.ID + `","version":1},"cases":[{"name":"compact-de","journey":"publish","surface":"PublishDialog","context":{"viewport":"390x844","theme":"light","content_length":"expanded","locale":"de","interaction_state":"error","assistive_technology":"screen-reader"},"requirement_ids":["keyboard","localization"],"inputs":["apps/web/publish.tsx"]}]}`
	configBlob, _ := opened.WriteObject(storage.BlobObject, []byte(config))
	sourceBlob, _ := opened.WriteObject(storage.BlobObject, []byte("export const Publish = () => 'review error success'"))
	komodoTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "interface-checks.json", configBlob))
	webTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "publish.tsx", sourceBlob))
	appsTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("40000", "web", webTree))
	rootTree, _ := opened.WriteObject(storage.TreeObject, append(treeEntry("40000", ".komodo", komodoTree), treeEntry("40000", "apps", appsTree)...))
	candidate, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor Agent <agent@example.test> 1 +0000\ncommitter Agent <agent@example.test> 1 +0000\n\nimplement accepted design\n"))
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), AuthorID: "agent", Title: "Implement publish journey", SourceBranch: "design/publish", TargetBranch: "main", SourceCommitID: string(candidate), TargetCommitID: string(candidate)})

	var policy designgovernance.Policy
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/policies", owner, `{"name":"interface disciplines","target_branches":["main"],"components":["PublishDialog"],"required_roles":["design_owner","accessibility","localization","invited_user"]}`, http.StatusCreated, &policy)
	checkBase := repoBase + "/pull-requests/" + pull.ID + "/interface-checks"
	var run interfacechecks.Run
	workflowJSON(t, server.URL, http.MethodPost, checkBase+"/runs", agent, `{"revision":"`+string(candidate)+`","results":{"compact-de":{"status":"failed","summary":"focus ring differs and error state works","coverage":["visual","interaction","localization","accessibility"],"duration_ms":32,"differences":[{"id":"focus","kind":"visual","summary":"old focus token remains","requirement_ids":["keyboard"]}]}}}`, http.StatusCreated, &run)
	workflowJSON(t, server.URL, http.MethodPost, checkBase+"/"+run.ID+"/cases/compact-de/differences/focus/classification", designer, `{"classification":"regression","rationale":"candidate must use accepted high-contrast token"}`, http.StatusCreated, &run)
	for _, role := range policy.RequiredRoles {
		decision, rationale := "accepted", "exact prototype and candidate reviewed"
		if role == "design_owner" {
			decision, rationale = "rejected", "visual regression must be corrected"
		}
		workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/pull-requests/"+pull.ID+"/acceptances", owner, `{"policy_id":"`+policy.ID+`","revision":"`+string(candidate)+`","preview_id":"preview-1","role":"`+role+`","decision":"`+decision+`","rationale":"`+rationale+`"}`, http.StatusCreated, nil)
	}
	var blocked designgovernance.Assessment
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/release-readiness", owner, `{"pull_request_id":"`+pull.ID+`","revision":"`+string(candidate)+`","target_branch":"main","components":["PublishDialog"]}`, http.StatusOK, &blocked)
	if blocked.Ready {
		t.Fatal("rejected acceptance and visual regression did not block release")
	}

	// Correction is retained as connected work, then current evidence and every
	// discipline acceptance converge on the exact candidate.
	var repair designgovernance.Work
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/work", designer, `{"kind":"repair","release_id":"release-2","source_kind":"regression","source_id":"focus","affected_repository":"`+string(repo.ID)+`","owner_id":"agent","summary":"adopt the accepted focus token","acceptance_criteria":["visual, keyboard, localized, and screen-reader checks pass"]}`, http.StatusCreated, &repair)
	workflowJSON(t, server.URL, http.MethodPost, checkBase+"/"+run.ID+"/cases/compact-de/differences/focus/classification", designer, `{"classification":"false_positive","rationale":"corrected candidate digest matches design token v2"}`, http.StatusCreated, &run)
	workflowJSON(t, server.URL, http.MethodPost, checkBase+"/"+run.ID+"/cases/compact-de/approvals", user, `{"decision":"approved","note":"complete expanded German error journey works with keyboard and screen reader","difference_ids":["focus"]}`, http.StatusCreated, &run)
	for _, role := range policy.RequiredRoles {
		workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/pull-requests/"+pull.ID+"/acceptances", owner, `{"policy_id":"`+policy.ID+`","revision":"`+string(candidate)+`","preview_id":"preview-2","role":"`+role+`","decision":"accepted","rationale":"corrected exact candidate accepted"}`, http.StatusCreated, nil)
	}
	var ready designgovernance.Assessment
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/design-governance/release-readiness", owner, `{"pull_request_id":"`+pull.ID+`","revision":"`+string(candidate)+`","target_branch":"main","components":["PublishDialog"]}`, http.StatusOK, &ready)
	if !ready.Ready || repair.GrantsAuthority || len(run.Approvals) != 1 || len(proposal.Comments) != 2 {
		t.Fatalf("workflow did not converge without expanding authority: ready=%#v repair=%#v run=%#v", ready, repair, run)
	}
}
