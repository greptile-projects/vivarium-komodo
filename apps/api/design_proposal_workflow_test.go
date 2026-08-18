package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDesignProposalPublicReviewBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "designer")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	designer := issueAccess(t, credentials, "designer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := designproposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerDesignProposalsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/design-proposals"
	in := designproposals.Input{Title: "Review publish", Origin: designproposals.Origin{Kind: "feedback", ID: "feedback-1"}, UserGoal: "understand publication", Journeys: []designproposals.Journey{{Name: "publish", Steps: []string{"review", "confirm"}, Outcome: "published"}}, States: []designproposals.State{{Name: "review", Behavior: "show impact", Content: "Review"}}, Content: []string{"Review", "Publish"}, Constraints: []designproposals.Constraint{{Kind: "accessibility", Requirement: "keyboard complete"}}, Alternatives: []designproposals.Alternative{{Name: "instant", Tradeoff: "errors", Reason: "unsafe"}}, SuccessMeasures: []designproposals.Measure{{Name: "success", Target: "95%"}}, AffectedComponents: []string{"PublishDialog"}, Evidence: []designproposals.Evidence{{ID: "visible", Kind: "feedback", Reference: "feedback-1", Summary: "confusion", Audience: "repository"}, {ID: "private", Kind: "research", Reference: "interview", Summary: "restricted", Audience: "private_research"}}, Uncertainty: []string{"copy length"}, ChangeReason: "open review"}
	body, _ := json.Marshal(in)
	var p designproposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, base, designer, string(body), http.StatusCreated, &p)
	if p.PrivateEvidenceCount != 1 || len(p.Revisions[0].Evidence) != 1 {
		t.Fatalf("unsafe projection: %#v", p)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/participants", designer, `{"expected_version":1,"subject_id":"agent-1","kind":"agent","role":"reviewer","grounded_evidence_ids":["private"]}`, http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/participants", designer, `{"expected_version":1,"subject_id":"agent-1","kind":"agent","role":"reviewer","grounded_evidence_ids":["visible"]}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/artifacts", designer, `{"kind":"wireframe","title":"Review state","proposal_revision":1,"frames":[{"name":"review","format":"html","body":"<button>Publish</button>"}],"interactions":[],"evidence_ids":["visible"],"uncertainty":["mobile layout"],"change_reason":"invite challenge"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/comments", owner, `{"subject_kind":"artifact","subject_id":"`+p.Artifacts[0].ID+`","subject_revision":1,"body":"Explain the irreversible effect","stance":"dissent","evidence_ids":["visible"],"uncertainty":"new users may not recognize publish"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/acknowledgements", designer, `{"expected_version":1,"owner_id":"owner"}`, http.StatusCreated, &p)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+p.ID+"/acknowledgements/"+p.Acknowledgements[0].ID+"/response", owner, `{"status":"changes_requested","rationale":"resolve the documented dissent"}`, http.StatusOK, &p)
	var list struct {
		Items []designproposals.Proposal `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 || len(list.Items[0].Comments) != 1 {
		t.Fatalf("public review unavailable: %#v", list)
	}
}
