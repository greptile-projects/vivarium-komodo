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
	user := issueAccess(t, credentials, "target-user", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	var workspace adoptionworkspaces.Workspace
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces", owner, `{"title":"Adopt a shared compiler","outcome":"compile safely in one command","origin":{"kind":"incubator","resource_id":"inc_42","revision":"7"},"required_journeys":["compile a sample project"],"environments":[{"name":"developer laptops","platform":"linux","version":"ubuntu-24.04"}],"constraints":["no source retention"],"budget":"USD 100/month","owner_ids":["adopter"],"evaluation_criteria":[{"id":"isolation","description":"untrusted source remains isolated","required":true}],"visibility":"public"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants", owner, `{"kind":"human","subject_id":"provider","role":"provider_maintainer","evidence_access":"provider"}`, http.StatusCreated, &workspace)
	providerParticipant := workspace.Participants[len(workspace.Participants)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants/"+providerParticipant+"/consent", maintainer, `{"decision":"accepted"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants", owner, `{"kind":"agent","subject_id":"agent:fit-reader","role":"read_only_agent","evidence_access":"shared"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants", owner, `{"kind":"human","subject_id":"target-user","role":"affected_user","evidence_access":"shared"}`, http.StatusCreated, &workspace)
	userParticipant := workspace.Participants[len(workspace.Participants)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/participants/"+userParticipant+"/consent", user, `{"decision":"accepted"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates", maintainer, `{"project":"Compiler Kit","provider_repository":"federated:provider/compiler","version":"v2.1.0","revision":"commit-good"}`, http.StatusCreated, &workspace)
	candidate := workspace.Candidates[0].ID
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"capability","claim":"compiles the required fixture","reference":"attestation:compile-7","revision":"commit-good","visibility":"public","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"security","claim":"isolation review passed","reference":"assessment:old","revision":"commit-old","visibility":"shared","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"support","claim":"private support agreement exists","revision":"commit-good","visibility":"provider","availability":"unavailable"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", maintainer, `{"dimension":"provenance","claim":"release provenance is attested","reference":"attestation:private-provenance","revision":"commit-good","visibility":"provider","availability":"available"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates/"+candidate+"/evidence", agent, `{"dimension":"gap","claim":"agent recommendation","reference":"agent:private-demo","revision":"commit-good","visibility":"shared","availability":"available"}`, http.StatusForbidden, nil)
	trialPath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/trials"
	workflowJSON(t, server.URL, http.MethodPost, trialPath, agent, `{"name":"Linux compiler journey","source":{"kind":"release","reference":"package:compiler-kit@2.1.0","revision":"commit-good","attestation":"release-attestation:7"},"packages":["compiler-kit@2.1.0"],"apis":["compile/v2"],"data":[{"kind":"synthetic","description":"generated sample project"}],"journey_ids":["compile a sample project"],"policies":["no source retention","networkless execution"],"setup":["create an empty temporary directory"],"configuration":{"mode":"isolated"},"commands":["compiler-kit synthetic-project"],"budget":"USD 5","evidence_audience":"public"}`, http.StatusCreated, &workspace)
	trial := workspace.Candidates[0].Trials[0].ID
	workflowJSON(t, server.URL, http.MethodPost, trialPath, owner, `{"name":"credential leak","source":{"kind":"revision","reference":"git:compiler","revision":"commit-good"},"packages":["compiler-kit"],"journey_ids":["compile a sample project"],"policies":["isolated"],"commands":["run --token=private"],"budget":"USD 5","evidence_audience":"public"}`, http.StatusUnprocessableEntity, nil)
	attemptPath := trialPath + "/" + trial + "/attempts"
	workflowJSON(t, server.URL, http.MethodPost, attemptPath, agent, `{"environment":"ubuntu-24.04 ephemeral","source_revision":"commit-good","configuration":{"mode":"isolated"},"commands":["compiler-kit synthetic-project"],"integration_changes":["added synthetic compiler fixture"],"checks":[{"name":"compile journey","status":"failed","summary":"generated module import failed"}],"previews":[{"name":"compiler output","reference":"artifact:failed-output","status":"available"}],"measurements":[{"name":"duration","value":12.4,"unit":"seconds"}],"cost":0.12,"currency":"USD","findings":[{"kind":"compatibility","summary":"default module mode is incompatible","evidence_reference":"artifact:failed-output"}],"artifacts":["sha256:failed"]}`, http.StatusCreated, &workspace)
	failedAttempt := workspace.Candidates[0].Trials[0].Attempts[0].ID
	workflowJSON(t, server.URL, http.MethodPost, attemptPath, agent, `{"environment":"ubuntu-24.04 ephemeral","source_revision":"commit-good","configuration":{"mode":"isolated","module":"compatible"},"commands":["compiler-kit synthetic-project --module compatible"],"integration_changes":["configured compatible module mode"],"checks":[{"name":"compile journey","status":"passed","summary":"synthetic project compiled"}],"previews":[{"name":"compiler output","reference":"artifact:passing-output","status":"available"}],"measurements":[{"name":"duration","value":3.1,"unit":"seconds"}],"cost":0.14,"currency":"USD","findings":[],"artifacts":["sha256:passing"],"reproduction_of":"`+failedAttempt+`"}`, http.StatusCreated, &workspace)
	passingAttempt := workspace.Candidates[0].Trials[0].Attempts[1].ID
	workflowJSON(t, server.URL, http.MethodPost, trialPath+"/"+trial+"/feedback", agent, `{"attempt_id":"`+passingAttempt+`","journey_id":"compile a sample project","verdict":"meets","comment":"agent recommendation"}`, http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, trialPath+"/"+trial+"/feedback", user, `{"attempt_id":"`+passingAttempt+`","journey_id":"compile a sample project","verdict":"meets","comment":"reproduced without a private machine"}`, http.StatusCreated, &workspace)
	planPath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/integration-plans"
	plan := `{"trial_id":"` + trial + `","selected_version":"v2.1.0","selected_revision":"commit-good","architecture":"consumer invokes the isolated compiler through a local adapter","configuration_ownership":[{"decision":"adapter defaults","owner_id":"adopter","side":"consumer"},{"decision":"compiler flags","owner_id":"provider","side":"provider"}],"update_policy":"monthly review; exact versions require consumer approval","support_policy":"consumer triages first; provider owns confirmed compiler defects","service_boundaries":[{"name":"local adapter","description":"consumer-owned invocation and fallback","owner_id":"adopter"},{"name":"compiler","description":"provider-owned compile behavior","owner_id":"provider"}],"data_boundaries":[{"name":"source input","description":"remains in the consumer networkless environment","owner_id":"adopter"}],"required_exceptions":[{"description":"temporary module compatibility flag","owner_id":"adopter","resolution":"pending"}],"exit_strategy":"retain the adapter interface and replace the compiler within one release","unresolved_gaps":[{"description":"long-term support response time is not evidenced","owner_id":"provider"}],"recurring_cost":"USD 40/month consumer budget; no provider commitment","compatibility_promises":[{"promise":"provider maintains compile/v2 through v2.x","owner_id":"provider"}],"work":[{"key":"adapter","scope":"consumer_repository","target":"consumer/compiler-adapter","owner_kind":"agent","owner_id":"agent:integration","depends_on":[],"acceptance_criteria":["ordinary review and journey checks pass"]},{"key":"environment","scope":"environment","target":"consumer development environment","owner_kind":"human","owner_id":"adopter","depends_on":["adapter"],"acceptance_criteria":["networkless configuration is approved"]},{"key":"docs","scope":"documentation","target":"consumer integration guide","owner_kind":"human","owner_id":"adopter","depends_on":["environment"],"acceptance_criteria":["support and exit ownership are documented"]},{"key":"upstream","scope":"upstream_fork","target":"provider/compiler compatibility fork","owner_kind":"human","owner_id":"provider","depends_on":["adapter"],"acceptance_criteria":["provider decides whether to accept the compatibility change"]}]}`
	workflowJSON(t, server.URL, http.MethodPost, planPath, agent, plan, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, planPath, maintainer, strings.Replace(plan, `"trial_id":"`+trial+`"`, `"trial_id":"missing"`, 1), http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, planPath, maintainer, plan, http.StatusCreated, &workspace)
	if len(workspace.Candidates[0].IntegrationPlans) != 1 || workspace.Candidates[0].IntegrationPlans[0].AuthorityGranted || len(workspace.Candidates[0].IntegrationPlans[0].Preview.EffectiveAccess) != 4 || len(workspace.Candidates[0].IntegrationPlans[0].Preview.Blockers) != 2 {
		t.Fatalf("integration agreement did not retain ownership, blockers, or authority boundaries: %#v", workspace.Candidates[0].IntegrationPlans)
	}
	workflowJSON(t, server.URL, http.MethodPost, trialPath, maintainer, `{"name":"Provider-only benchmark","source":{"kind":"revision","reference":"git:compiler","revision":"commit-good"},"packages":["compiler-kit"],"data":[{"kind":"permitted","description":"provider benchmark corpus","reference":"provider-permission:4"}],"journey_ids":["compile a sample project"],"policies":["provider lab only"],"setup":["create isolated benchmark workspace"],"commands":["compiler-kit benchmark"],"budget":"USD 5","evidence_audience":"provider"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates", maintainer, `{"project":"Compiler Kit","provider_repository":"federated:provider/compiler","version":"v2.2.0-rc1","revision":"commit-next"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodGet, "/adoption-workspaces/"+workspace.ID, agent, "", http.StatusOK, &workspace)
	if workspace.AuthorityGranted || workspace.Candidates[0].Coverage["capability"] != "supported" || workspace.Candidates[0].Coverage["security"] != "stale" || workspace.Candidates[0].Evidence[2].ProofOfFit || workspace.Candidates[0].Evidence[2].Reference != "" {
		t.Fatalf("fit projection presented a gap as proof or granted authority: %#v", workspace.Candidates[0])
	}
	if len(workspace.Candidates) != 2 || workspace.Candidates[0].Trials[0].Status != "passed" || !workspace.Candidates[0].Trials[0].Attempts[1].Reproducible || len(workspace.Candidates[0].Trials[0].Feedback) != 1 {
		t.Fatalf("trial comparison did not retain reproducible evidence and human feedback: %#v", workspace.Candidates[0].Trials)
	}
	if !strings.Contains(strings.Join(workspace.Candidates[0].Blockers, " "), "no evidence") {
		t.Fatalf("missing comparison dimensions were hidden: %#v", workspace.Candidates[0].Blockers)
	}
	var publicView adoptionworkspaces.Workspace
	workflowJSON(t, server.URL, http.MethodGet, "/adoption-workspaces/"+workspace.ID, observer, "", http.StatusOK, &publicView)
	if publicView.Candidates[0].Evidence[3].Reference != "" || publicView.Candidates[0].Evidence[3].Status != "inaccessible" {
		t.Fatalf("provider evidence leaked through a public workspace: %#v", publicView.Candidates[0].Evidence[3])
	}
	if len(publicView.Candidates[0].Trials[0].Attempts) != 2 || publicView.Candidates[0].Trials[0].Attempts[0].Status != "failed" || publicView.Candidates[0].Trials[1].Status != "inaccessible" || len(publicView.Candidates[0].Trials[1].Attempts) != 0 {
		t.Fatalf("public trial surface hid failure or leaked scoped evidence: %#v", publicView.Candidates[0].Trials)
	}
}
