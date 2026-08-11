package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestContributionGuidanceWorkflow proves that private-fork work can receive
// attributable upstream mentoring and bounded agent help without making the
// mentor a fork participant or transferring decision ownership.
func TestContributionGuidanceWorkflow(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	opportunities, _ := contributionopportunities.New(t.TempDir())
	pathways, _ := contributorpathways.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	organizationStore, _ := organizations.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	runner := workspaces.NewRunner(workspaceStore, catalog)

	organization, _ := organizationStore.Create("maintainer", "welcoming-team", "Welcoming team", "")
	_, agent, err := organizationStore.RegisterAgent(organization.ID, "maintainer", organizations.Agent{Slug: "guide", Name: "Guide agent", Capabilities: []string{"workspace:edit"}, OperatorIDs: []string{"maintainer"}, Visibility: "internal"})
	if err != nil {
		t.Fatal(err)
	}
	upstream, _ := catalog.Create("maintainer", repositories.Metadata{Name: "welcoming-project", Visibility: repositories.Public})
	upstream, err = catalog.TransferOwner(upstream.ID, "user", "maintainer", "organization", organization.ID, "maintainer")
	if err != nil {
		t.Fatal(err)
	}
	fork, _ := catalog.Create("newcomer", repositories.Metadata{Name: "welcoming-project-contribution", Visibility: repositories.Private})
	mentorToken := issueAccess(t, credentials, "mentor", auth.API, auth.RepositoryRead)
	maintainerToken := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorToken := issueAccess(t, credentials, "newcomer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentToken := issueAccess(t, credentials, agent.ID, auth.API, auth.RepositoryRead)

	revision := "1111111111111111111111111111111111111111"
	opportunity, err := opportunities.Publish(string(upstream.ID), "maintainer", "Clarify setup", revision, "open", true, contributionopportunities.Input{Source: contributionopportunities.Source{Kind: "issue", ResourceID: "issue-1"}, RequiredSkills: []string{"Go"}, Interests: []string{"onboarding"}, ExpectedOutcome: "Improve setup", Scope: []string{"README.md"}, Risk: "low", MentorIDs: []string{"mentor"}, Assistance: "human_or_agent"})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := opportunities.Claim(string(upstream.ID), opportunity.ID, "newcomer", "I can help", 24)
	if err != nil {
		t.Fatal(err)
	}
	definition := workspaces.Definition{Version: 1, Resources: workspaces.ResourceLimits{CPUSeconds: 30, MemoryMB: 256, DiskMB: 256, SetupTimeoutSeconds: 30}}
	workspace, err := workspaceStore.Create(string(fork.ID), revision, "newcomer", workspaces.SourceContext{Type: "contribution_opportunity", ID: opportunity.ID, UpstreamRepositoryID: string(upstream.ID)}, workspaces.Access{RepositoryID: string(fork.ID), ActorID: "newcomer", Permission: "fork:write"}, definition, "definition")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(workspaceStore.Environment(workspace.ID), 0750); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(workspaceStore.Environment(workspace.ID), "README.md"), []byte("draft\n"), 0640); err != nil {
		t.Fatal(err)
	}
	workspace, err = workspaceStore.Finish(workspace.ID, true, "ready")
	if err != nil {
		t.Fatal(err)
	}
	collaboration, err := opportunities.StartCollaboration(string(upstream.ID), opportunity.ID, claim.ID, string(fork.ID), workspace.ID, "newcomer", revision, 4)
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerContributionOpportunitiesHTTP(mux, opportunities, catalog, credentials, issueStore, proposalStore, organizationStore, pathways, workspaceStore, runner)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(upstream.ID) + "/contribution-opportunities/" + opportunity.ID + "/collaboration"
	workflowJSON(t, server.URL, http.MethodPost, base+"/presence", contributorToken, `{"surface":"thread"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/presence", mentorToken, `{"surface":"checkpoint"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, base+"/mentor-availability", mentorToken, `{"state":"available","note":"Responding for the next four hours."}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/events", contributorToken, `{"kind":"question","message":"Does this preserve the setup scope?"}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/events", contributorToken, `{"kind":"checkpoint_request","message":"Please review README.md before I continue.","paths":["README.md"]}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/events", mentorToken, `{"kind":"advice","message":"Keep the command exact; the contributor still owns the choice.","decision_owner_id":"newcomer"}`, http.StatusCreated, nil)

	var controlled contributionopportunities.Collaboration
	workflowJSON(t, server.URL, http.MethodPost, base+"/agent-controls", contributorToken, `{"agent_id":"`+agent.ID+`","mode":"edit","paths":["README.md"]}`, http.StatusCreated, &controlled)
	control := controlled.Controls[len(controlled.Controls)-1]
	workflowJSON(t, server.URL, http.MethodPost, base+"/agent-actions", agentToken, `{"control_id":"`+control.ID+`","kind":"edit","path":"README.md","content":"guided draft\n","summary":"Applied only the requested wording change."}`, http.StatusCreated, &controlled)
	if data, _ := os.ReadFile(filepath.Join(workspaceStore.Environment(workspace.ID), "README.md")); string(data) != "guided draft\n" {
		t.Fatalf("agent edit = %q", data)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/agent-controls/"+control.ID+"/interventions", contributorToken, `{"action":"revoke","version":1}`, http.StatusOK, &controlled)
	workflowJSON(t, server.URL, http.MethodPost, base+"/agent-actions", agentToken, `{"control_id":"`+control.ID+`","kind":"edit","path":"README.md","content":"unauthorized\n","summary":"should fail"}`, http.StatusConflict, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/events", mentorToken, `{"kind":"handoff","message":"I am unavailable; maintainer should take the next review."}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/transition", maintainerToken, `{"state":"reassignment_required","reason":"Mentor availability changed; assign a new reviewer."}`, http.StatusOK, &controlled)
	if controlled.State != "reassignment_required" || controlled.ResponseExpectedBy.Sub(collaboration.CreatedAt) != 4*time.Hour || controlled.Events[len(controlled.Events)-1].DecisionOwnerID != "newcomer" {
		t.Fatalf("retained guidance = %#v", controlled)
	}
	if member, _ := catalog.IsCollaborator(fork.ID, "mentor"); member {
		t.Fatal("mentoring silently granted private-fork access")
	}
}
