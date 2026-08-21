package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/adoptionworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

// TestAdoptionWorkspaceWorkflow is the public boundary for shared requirements,
// permission-aware participants, exact candidates, and inspectable fit evidence.
func TestAdoptionWorkspaceWorkflow(t *testing.T) {
	credentials, _ := auth.New(t.TempDir())
	store, _ := adoptionworkspaces.New(t.TempDir())
	mux := http.NewServeMux()
	registerAdoptionWorkspacesHTTP(mux, store, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	owner := issueAccess(t, credentials, "adopter", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainer := issueAccess(t, credentials, "provider", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent:fit-reader", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	observer := issueAccess(t, credentials, "public-observer", auth.API, auth.RepositoryRead)

	var workspace adoptionworkspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces", owner, `{"title":"Adopt a shared compiler","outcome":"compile safely in one command","origin":{"kind":"incubator","resource_id":"inc_42","revision":"7"},"required_journeys":["compile a sample project"],"environments":[{"name":"developer laptops","platform":"linux","version":"ubuntu-24.04"}],"constraints":["no source retention"],"budget":"USD 100/month","owner_ids":["adopter"],"evaluation_criteria":[{"id":"isolation","description":"untrusted source remains isolated","required":true}],"visibility":"public"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants", owner, `{"kind":"human","subject_id":"provider","role":"provider_maintainer","evidence_access":"provider"}`, http.StatusCreated, &workspace)
	providerParticipant := workspace.Participants[len(workspace.Participants)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants/"+providerParticipant+"/consent", maintainer, `{"decision":"accepted"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants", owner, `{"kind":"agent","subject_id":"agent:fit-reader","role":"read_only_agent","evidence_access":"shared"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates", maintainer, `{"project":"Compiler Kit","provider_repository":"federated:provider/compiler","version":"v2.1.0","revision":"commit-good"}`, http.StatusCreated, &workspace)
	candidate := workspace.Candidates[0].ID
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"capability","claim":"compiles the required fixture","reference":"attestation:compile-7","revision":"commit-good","visibility":"public","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"security","claim":"isolation review passed","reference":"assessment:old","revision":"commit-old","visibility":"shared","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"support","claim":"private support agreement exists","revision":"commit-good","visibility":"provider","availability":"unavailable"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"provenance","claim":"release provenance is attested","reference":"attestation:private-provenance","revision":"commit-good","visibility":"provider","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", agent, `{"dimension":"gap","claim":"agent recommendation","reference":"agent:private-demo","revision":"commit-good","visibility":"shared","availability":"available"}`, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodGet, "/adoption-workspaces/"+workspace.ID, agent, "", http.StatusOK, &workspace)
	if workspace.AuthorityGranted || workspace.Candidates[0].Coverage["capability"] != "supported" || workspace.Candidates[0].Coverage["security"] != "stale" || workspace.Candidates[0].Evidence[2].ProofOfFit || workspace.Candidates[0].Evidence[2].Reference != "" {
		t.Fatalf("fit projection presented a gap as proof or granted authority: %#v", workspace.Candidates[0])
	}
	if !strings.Contains(strings.Join(workspace.Candidates[0].Blockers, " "), "no evidence") {
		t.Fatalf("missing comparison dimensions were hidden: %#v", workspace.Candidates[0].Blockers)
	}
	var publicView adoptionworkspaces.Workspace
	workflowJSON(t, server.URL, http.MethodGet, "/adoption-workspaces/"+workspace.ID, observer, "", http.StatusOK, &publicView)
	if publicView.Candidates[0].Evidence[3].Reference != "" || publicView.Candidates[0].Evidence[3].Status != "inaccessible" {
		t.Fatalf("provider evidence leaked through a public workspace: %#v", publicView.Candidates[0].Evidence[3])
	}
}
