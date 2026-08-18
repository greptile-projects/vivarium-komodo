package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDesignProposalPublicReviewBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	opened, _ := catalog.Open(repo.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, []byte{})
	commit, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor Owner <owner@example.test> 1 +0000\ncommitter Owner <owner@example.test> 1 +0000\n\nbase\n"))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "designer")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	designer := issueAccess(t, credentials, "designer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := designproposals.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	mux := http.NewServeMux()
	registerDesignProposalsHTTP(mux, store, catalog, credentials, plans, pulls)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/design-proposals"
	in := designproposals.Input{Title: "Review publish", Origin: designproposals.Origin{Kind: "feedback", ID: "feedback-1"}, UserGoal: "understand publication", Journeys: []designproposals.Journey{{Name: "publish", Steps: []string{"review", "confirm"}, Outcome: "published"}}, States: []designproposals.State{{Name: "review", Behavior: "show impact", Content: "Review"}}, Content: []string{"Review", "Publish"}, Constraints: []designproposals.Constraint{{Kind: "accessibility", Requirement: "keyboard complete"}}, Alternatives: []designproposals.Alternative{{Name: "instant", Tradeoff: "errors", Reason: "unsafe"}}, SuccessMeasures: []designproposals.Measure{{Name: "success", Target: "95%"}}, AffectedComponents: []string{"PublishDialog"}, ComponentContracts: []designproposals.ComponentContract{{Name: "PublishDialog", Contract: "summary, cancel, and publish actions with focus return"}}, Breakpoints: []designproposals.Breakpoint{{Name: "compact", MinimumWidth: 0, MaximumWidth: 639, Behavior: "stack actions below summary"}, {Name: "wide", MinimumWidth: 640, Behavior: "align actions beside summary"}}, Evidence: []designproposals.Evidence{{ID: "visible", Kind: "feedback", Reference: "feedback-1", Summary: "confusion", Audience: "repository"}, {ID: "private", Kind: "research", Reference: "interview", Summary: "restricted", Audience: "private_research"}}, Uncertainty: []string{"copy length"}, ChangeReason: "open review"}
	body, _ := json.Marshal(in)
	var p designproposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, base, designer, string(body), http.StatusCreated, &p)
	if p.PrivateEvidenceCount != 1 || len(p.Revisions[0].Evidence) != 1 {
		t.Fatalf("unsafe projection: %#v", p)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/participants", designer, `{"expected_version":1,"subject_id":"agent-1","kind":"agent","role":"reviewer","grounded_evidence_ids":["private"]}`, http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/participants", designer, `{"expected_version":1,"subject_id":"agent-1","kind":"agent","role":"reviewer","grounded_evidence_ids":["visible"]}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/artifacts", designer, `{"kind":"wireframe","title":"Review state","proposal_revision":1,"frames":[{"name":"review","description":"Wide and compact confirmation surface","format":"html","body":"<button>Publish</button>"}],"interactions":[{"trigger":"activate","action":"publish","result":"show success"}],"assets":[{"id":"publish-icon","name":"Publish icon","source":"design://publish/icon-v2","author_id":"designer","license":"CC-BY-4.0","transformations":["exported SVG","optimized paths"]}],"evidence_ids":["visible"],"uncertainty":["mobile layout"],"change_reason":"invite challenge"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/comments", owner, `{"subject_kind":"artifact","subject_id":"`+p.Artifacts[0].ID+`","subject_revision":1,"body":"Explain the irreversible effect","stance":"dissent","evidence_ids":["visible"],"uncertainty":"new users may not recognize publish"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/acknowledgements", designer, `{"expected_version":1,"owner_id":"owner"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/acknowledgements/"+p.Acknowledgements[0].ID+"/response", owner, `{"status":"acknowledged","rationale":"the dissent is explicitly retained for implementation review"}`, http.StatusOK, &p)
	var implementation struct {
		Design designproposals.Proposal `json:"design_proposal"`
		Work   proposals.Proposal       `json:"work_proposal"`
		Tasks  []proposals.Task         `json:"tasks"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/implementations", designer, `{"base_revision":"`+string(commit)+`","title":"Implement accepted publish experience","tasks":[{"title":"Build responsive publish surface","outcome":"All accepted states render without guessing","owner_kind":"agent","owner_id":"codex","acceptance_criteria":["wide and compact states match the accepted prototype"],"changed_paths":["apps/web/publish.tsx"],"rendered_surfaces":["publish-wide","publish-compact"]},{"title":"Verify content and keyboard behavior","outcome":"Human verifies the final experience","owner_kind":"human","owner_id":"owner","depends_on":[1],"acceptance_criteria":["keyboard publication and success state pass"],"changed_paths":["apps/web/publish.test.tsx"],"rendered_surfaces":["publish-keyboard"]}]}`, http.StatusCreated, &implementation)
	if len(implementation.Tasks) != 2 || implementation.Tasks[1].DependsOn[0] != implementation.Tasks[0].ID || implementation.Tasks[0].ReasoningContext == nil || implementation.Tasks[0].ReasoningContext.Design == nil || len(implementation.Tasks[0].ReasoningContext.Design.Assets) != 1 || len(implementation.Tasks[0].ReasoningContext.Design.Requirements) < 8 {
		t.Fatalf("implementation handoff lost context: %#v", implementation)
	}
	p = implementation.Design
	imp := p.Implementations[0]
	requirement := imp.Requirements[0].ID
	candidate, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), ProposalID: implementation.Work.ID, TaskID: implementation.Tasks[0].ID, AuthorID: "designer", Title: "Implement publish surface", SourceBranch: "design/publish", TargetBranch: "main", SourceCommitID: "candidate-commit", TargetCommitID: string(commit)})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/implementations/"+imp.ID+"/mappings", designer, `{"task_id":"`+implementation.Tasks[0].ID+`","pull_request_id":"`+candidate.ID+`","revision":"candidate-commit","changed_paths":["apps/web/publish.tsx"],"rendered_surfaces":["publish-wide","publish-compact"],"requirement_ids":["`+requirement+`"],"deviations":[{"requirement_id":"`+requirement+`","reason":"compact layout needs shorter sequencing","impact":"same outcome with one combined review step"}]}`, http.StatusCreated, &p)
	if p.Mappings[0].Deviations[0].Status != "pending" {
		t.Fatalf("deviation was not approval-gated: %#v", p.Mappings)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/implementation-mappings/"+p.Mappings[0].ID+"/deviations/"+requirement+"/approval", owner, `{}`, http.StatusOK, &p)
	if p.Mappings[0].Deviations[0].Status != "approved" || p.Mappings[0].Deviations[0].ApprovedByID != "owner" {
		t.Fatalf("approval not attributed: %#v", p.Mappings)
	}
	var list struct {
		Items []designproposals.Proposal `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 || len(list.Items[0].Comments) != 1 {
		t.Fatalf("public review unavailable: %#v", list)
	}
}
