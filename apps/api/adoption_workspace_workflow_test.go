package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/adoptionworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestAdoptionWorkspaceWorkflow is the black-box boundary for the complete
// evaluation-to-upstream-improvement loop. It crosses the public HTTP handlers
// used by /adoptions and retains stock Git objects from the independently owned
// provider and consumer repositories as exact delivery and contribution proof.
func TestAdoptionWorkspaceWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	providerRepo, _ := repos.Create("provider", repositories.Metadata{Name: "compiler-kit", Visibility: repositories.Public})
	consumerRepo, _ := repos.Create("adopter", repositories.Metadata{Name: "consumer-product", Visibility: repositories.Private})
	providerBase := adoptionCommit(t, repos, providerRepo.ID, "Provider Maintainer", "compiler kit v2.1.0", "")
	providerRepair := adoptionCommit(t, repos, providerRepo.ID, "Fit Agent", "compatible module defaults", string(providerBase))
	consumerBase := adoptionCommit(t, repos, consumerRepo.ID, "Adopter", "temporary compatibility adapter", "")
	consumerUpdate := adoptionCommit(t, repos, consumerRepo.ID, "Adopter", "use upstream compiler fix", string(consumerBase))
	if providerBase == providerRepair || consumerBase == consumerUpdate {
		t.Fatal("stock Git did not retain distinct provider and consumer revisions")
	}
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
	plan := `{"trial_id":"` + trial + `","selected_version":"v2.1.0","selected_revision":"commit-good","architecture":"consumer invokes the isolated compiler through a local adapter","configuration_ownership":[{"decision":"adapter defaults","owner_id":"adopter","side":"consumer"},{"decision":"compiler flags","owner_id":"provider","side":"provider"}],"update_policy":"monthly review; exact versions require consumer approval","support_policy":"consumer triages first; provider owns confirmed compiler defects","service_boundaries":[{"name":"local adapter","description":"consumer-owned invocation and fallback","owner_id":"adopter"},{"name":"compiler","description":"provider-owned compile behavior","owner_id":"provider"}],"data_boundaries":[{"name":"source input","description":"remains in the consumer networkless environment","owner_id":"adopter"}],"required_exceptions":[{"description":"permanent access to the provider benchmark corpus","owner_id":"adopter","resolution":"denied"}],"exit_strategy":"retain the adapter interface and replace the compiler within one release","unresolved_gaps":[{"description":"long-term support response time is not evidenced","owner_id":"provider"}],"recurring_cost":"USD 40/month consumer budget; no provider commitment","compatibility_promises":[{"promise":"provider maintains compile/v2 through v2.x","owner_id":"provider"}],"work":[{"key":"adapter","scope":"consumer_repository","target":"consumer/compiler-adapter","owner_kind":"agent","owner_id":"agent:integration","depends_on":[],"acceptance_criteria":["ordinary review and journey checks pass"]},{"key":"environment","scope":"environment","target":"consumer development environment","owner_kind":"human","owner_id":"adopter","depends_on":["adapter"],"acceptance_criteria":["networkless configuration is approved"]},{"key":"docs","scope":"documentation","target":"consumer integration guide","owner_kind":"human","owner_id":"adopter","depends_on":["environment"],"acceptance_criteria":["support and exit ownership are documented"]},{"key":"upstream","scope":"upstream_fork","target":"provider/compiler compatibility fork","owner_kind":"human","owner_id":"provider","depends_on":["adapter"],"acceptance_criteria":["provider decides whether to accept the compatibility change"]}]}`
	workflowJSON(t, server.URL, http.MethodPost, planPath, agent, plan, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, planPath, maintainer, strings.Replace(plan, `"trial_id":"`+trial+`"`, `"trial_id":"missing"`, 1), http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, planPath, maintainer, plan, http.StatusCreated, &workspace)
	if len(workspace.Candidates[0].IntegrationPlans) != 1 || workspace.Candidates[0].IntegrationPlans[0].AuthorityGranted || len(workspace.Candidates[0].IntegrationPlans[0].Preview.EffectiveAccess) != 4 || len(workspace.Candidates[0].IntegrationPlans[0].Preview.Blockers) != 2 {
		t.Fatalf("integration agreement did not retain ownership, blockers, or authority boundaries: %#v", workspace.Candidates[0].IntegrationPlans)
	}
	integrationPlan := workspace.Candidates[0].IntegrationPlans[0].ID
	deliveryPath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/deliveries"
	delivery := `{"plan_id":"` + integrationPlan + `","provider_revision":"commit-good","consumer_repository":"consumer/product","consumer_pull_request":"pull:consumer/product#81","consumer_revision":"consumer-commit-81","pinned_dependencies":["compiler-kit@2.1.0"],"changes":[{"kind":"dependency","path":"go.mod"},{"kind":"integration","path":"compiler/adapter.go"},{"kind":"configuration","path":"config/compiler.yaml"},{"kind":"infrastructure","path":"infra/compiler.tf"},{"kind":"test","path":"compiler/adapter_test.go"},{"kind":"documentation","path":"docs/compiler.md"}],"evidence":[{"kind":"provider_attestation","reference":"attestation:release-7","revision":"consumer-commit-81","status":"passed","approved_by_id":"provider"},{"kind":"approval","reference":"approval:consumer-owner","revision":"consumer-commit-81","status":"passed","approved_by_id":"adopter"},{"kind":"review","reference":"review:pull-81","revision":"consumer-commit-81","status":"passed","approved_by_id":"consumer-reviewer"},{"kind":"policy","reference":"policy:assessment-81","revision":"consumer-commit-81","status":"passed"},{"kind":"rehearsal","reference":"rehearsal:compiler-81","revision":"consumer-commit-81","status":"passed"},{"kind":"release","reference":"release:consumer-v9","revision":"consumer-commit-81","status":"passed"},{"kind":"support_readiness","reference":"support:on-call-81","revision":"consumer-commit-81","status":"passed","approved_by_id":"adopter"},{"kind":"user_acceptance","reference":"feedback:target-user-81","revision":"consumer-commit-81","status":"passed","approved_by_id":"target-user"}],"rollout":[{"name":"canary","environment":"consumer-staging","release_revision":"consumer-commit-81","status":"passed","health":"healthy","cost":4,"currency":"USD","evidence_reference":"deployment:canary-81"},{"name":"production","environment":"consumer-production","release_revision":"consumer-commit-81","status":"passed","health":"healthy","cost":40,"currency":"USD","evidence_reference":"deployment:production-81"}]}`
	delivery = strings.ReplaceAll(delivery, "consumer-commit-81", string(consumerBase))
	workflowJSON(t, server.URL, http.MethodPost, deliveryPath, agent, delivery, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, deliveryPath, owner, strings.Replace(delivery, `"provider_revision":"commit-good"`, `"provider_revision":"commit-next"`, 1), http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, deliveryPath, owner, delivery, http.StatusCreated, &workspace)
	delivered := workspace.Candidates[0].Deliveries[0]
	if delivered.Status != "active" || delivered.AuthorityGranted || len(delivered.Rollout) != 2 {
		t.Fatalf("delivery did not retain exact governed rollout: %#v", delivered)
	}
	observationPath := deliveryPath + "/" + delivered.ID + "/observations"
	workflowJSON(t, server.URL, http.MethodPost, observationPath, maintainer, fmt.Sprintf(`{"consumer_revision":%q,"kind":"rollout","status":"failed","summary":"canary compiler requests regressed and the provider attestation became unavailable","evidence_reference":"deployment:failed-canary-81"}`, consumerBase), http.StatusCreated, &workspace)
	if workspace.Candidates[0].Deliveries[0].Status != "paused" {
		t.Fatalf("revoked access did not pause adoption")
	}
	workflowJSON(t, server.URL, http.MethodPost, observationPath, agent, fmt.Sprintf(`{"consumer_revision":%q,"kind":"rollout","status":"restored","summary":"agent chose rollback","evidence_reference":"deployment:rollback-81"}`, consumerBase), http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, observationPath, owner, fmt.Sprintf(`{"consumer_revision":%q,"kind":"rollout","status":"restored","summary":"consumer owner restored the attested prior release","evidence_reference":"deployment:rollback-81"}`, consumerBase), http.StatusCreated, &workspace)
	if workspace.Candidates[0].Deliveries[0].Status != "active" || len(workspace.Candidates[0].Deliveries[0].Observations) != 2 {
		t.Fatalf("owner-controlled restoration was not retained: %#v", workspace.Candidates[0].Deliveries[0])
	}
	sharePath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/upstream-shares"
	workflowJSON(t, server.URL, http.MethodPost, sharePath, owner, `{"kind":"compatibility_evidence","summary":"Module defaults fail on the adopter's synthetic fixture","redacted_details":["Run the public synthetic fixture with the default module mode","The compatible mode succeeds without consumer data"],"evidence_references":["attempt:`+failedAttempt+`","attempt:`+passingAttempt+`"],"visibility":"public"}`, http.StatusCreated, &workspace)
	share := workspace.Candidates[0].UpstreamShares[0].ID
	workflowJSON(t, server.URL, http.MethodPost, sharePath, owner, `{"kind":"trial_finding","summary":"unsafe redaction","redacted_details":["token: consumer-private-value"],"evidence_references":["attempt:`+failedAttempt+`"],"visibility":"public"}`, http.StatusUnprocessableEntity, nil)
	workflowJSON(t, server.URL, http.MethodPost, sharePath+"/"+share+"/consent", owner, `{"decision":"accepted"}`, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, sharePath+"/"+share+"/consent", maintainer, `{"decision":"accepted"}`, http.StatusCreated, &workspace)
	contributionPath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/upstream-contributions"
	workflowJSON(t, server.URL, http.MethodPost, contributionPath, agent, fmt.Sprintf(`{"share_ids":[%q],"kind":"fork_pull_request","repository_id":"federated:provider/compiler","resource_id":"pull:provider/compiler#19","revision":%q,"author_kind":"agent","author_id":"agent:fit-reader","contributor_guidance":"pathway:compiler-v4","review_reference":"review:19","checks_reference":"checks:19","security_reference":"security:19","local_resolution":"retain the consumer compatibility flag until an accepted release is verified"}`, share, providerRepair), http.StatusCreated, &workspace)
	contribution := workspace.Candidates[0].Contributions[0].ID
	workflowJSON(t, server.URL, http.MethodPost, contributionPath+"/"+contribution+"/decision", agent, `{"decision":"accepted","reason":"agent chose acceptance","release":"v2.1.1","revision":"repair-commit-19"}`, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, contributionPath+"/"+contribution+"/decision", maintainer, fmt.Sprintf(`{"decision":"accepted","reason":"ordinary review, checks, and security policy passed","release":"v2.1.1","revision":%q}`, providerRepair), http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, contributionPath, owner, `{"share_ids":["`+share+`"],"kind":"local_pull_request","repository_id":"consumer/product","resource_id":"pull:consumer/product#93","revision":"local-doc-fix","author_kind":"human","author_id":"adopter","contributor_guidance":"consumer contribution policy","review_reference":"review:93","checks_reference":"checks:93","security_reference":"security:93","local_resolution":"ship the corrected consumer guide while upstream declines it"}`, http.StatusCreated, &workspace)
	rejected := workspace.Candidates[0].Contributions[1].ID
	workflowJSON(t, server.URL, http.MethodPost, contributionPath+"/"+rejected+"/decision", maintainer, `{"decision":"rejected","reason":"provider documentation describes a different supported workflow"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, contributionPath, owner, `{"share_ids":["`+share+`"],"kind":"federated_pull_request","repository_id":"federated:provider/compiler","resource_id":"federated-proposal:44","revision":"federated-fix-44","author_kind":"human","author_id":"adopter","contributor_guidance":"federated pathway v4","review_reference":"pending:provider","checks_reference":"checks:consumer-44","security_reference":"security:consumer-44","local_resolution":"keep the bounded fork and compatibility flag during provider outage"}`, http.StatusCreated, &workspace)
	unavailable := workspace.Candidates[0].Contributions[2].ID
	workflowJSON(t, server.URL, http.MethodPost, contributionPath+"/"+unavailable+"/decision", owner, `{"decision":"provider_unavailable","reason":"the trusted provider peer is currently unreachable"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, sharePath, owner, `{"kind":"usage_outcome","summary":"A consumer rollout revealed a non-public usage pattern","redacted_details":["Aggregate outcome only; consumer identities are withheld"],"evidence_references":["delivery:`+delivered.ID+`"],"visibility":"embargoed","embargo_until":"2099-01-01T00:00:00Z"}`, http.StatusCreated, &workspace)
	embargoedShare := workspace.Candidates[0].UpstreamShares[1].ID
	workflowJSON(t, server.URL, http.MethodPost, sharePath+"/"+embargoedShare+"/consent", maintainer, `{"decision":"accepted"}`, http.StatusCreated, &workspace)
	updateEvidence := `[{"kind":"provider_attestation","reference":"attestation:release-2.1.1","revision":"consumer-commit-92","status":"passed","approved_by_id":"provider"},{"kind":"approval","reference":"approval:update-92","revision":"consumer-commit-92","status":"passed","approved_by_id":"adopter"},{"kind":"review","reference":"review:update-92","revision":"consumer-commit-92","status":"passed"},{"kind":"policy","reference":"policy:update-92","revision":"consumer-commit-92","status":"passed"},{"kind":"rehearsal","reference":"rehearsal:update-92","revision":"consumer-commit-92","status":"passed"},{"kind":"release","reference":"release:consumer-v10","revision":"consumer-commit-92","status":"passed"},{"kind":"support_readiness","reference":"support:update-92","revision":"consumer-commit-92","status":"passed"},{"kind":"user_acceptance","reference":"feedback:update-92","revision":"consumer-commit-92","status":"passed","approved_by_id":"target-user"}]`
	updateEvidence = strings.ReplaceAll(updateEvidence, "consumer-commit-92", string(consumerUpdate))
	updatePath := "/adoption-workspaces/" + workspace.ID + "/candidates/" + candidate + "/verified-updates"
	updateBody := fmt.Sprintf(`{"contribution_id":%q,"delivery_id":%q,"from_consumer_revision":%q,"to_consumer_revision":%q,"accepted_release":"v2.1.1","provider_revision":%q,"replaced_patches":["temporary module compatibility flag"],"evidence":%s}`, contribution, delivered.ID, consumerBase, consumerUpdate, providerRepair, updateEvidence)
	workflowJSON(t, server.URL, http.MethodPost, updatePath, maintainer, updateBody, http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, updatePath, owner, updateBody, http.StatusCreated, &workspace)
	if len(workspace.Candidates[0].VerifiedUpdates) != 1 || workspace.Candidates[0].VerifiedUpdates[0].Status != "verified" || workspace.Candidates[0].Contributions[0].AuthorityGranted {
		t.Fatalf("upstream acceptance did not retain the verified consumer-controlled update: %#v", workspace.Candidates[0])
	}
	workflowJSON(t, server.URL, http.MethodPost, trialPath, maintainer, `{"name":"Provider-only benchmark","source":{"kind":"revision","reference":"git:compiler","revision":"commit-good"},"packages":["compiler-kit"],"data":[{"kind":"permitted","description":"provider benchmark corpus","reference":"provider-permission:4"}],"journey_ids":["compile a sample project"],"policies":["provider lab only"],"setup":["create isolated benchmark workspace"],"commands":["compiler-kit benchmark"],"budget":"USD 5","evidence_audience":"provider"}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates", maintainer, `{"project":"Compiler Kit","provider_repository":"federated:provider/compiler","version":"v2.2.0-rc1","revision":"commit-next"}`, http.StatusCreated, &workspace)
	regressionCandidate := workspace.Candidates[1].ID
	regressionTrials := "/adoption-workspaces/" + workspace.ID + "/candidates/" + regressionCandidate + "/trials"
	workflowJSON(t, server.URL, http.MethodPost, regressionTrials, agent, `{"name":"Version regression check","source":{"kind":"revision","reference":"git:compiler","revision":"commit-next","attestation":"release-candidate:2.2"},"packages":["compiler-kit@2.2.0-rc1"],"data":[{"kind":"synthetic","description":"generated sample project"}],"journey_ids":["compile a sample project"],"policies":["networkless execution"],"setup":["create an isolated temporary directory"],"commands":["compiler-kit synthetic-project"],"budget":"USD 2","evidence_audience":"public"}`, http.StatusCreated, &workspace)
	regressionTrial := workspace.Candidates[1].Trials[0].ID
	workflowJSON(t, server.URL, http.MethodPost, regressionTrials+"/"+regressionTrial+"/attempts", agent, `{"environment":"ubuntu-24.04 ephemeral","source_revision":"commit-next","commands":["compiler-kit synthetic-project"],"checks":[{"name":"compile journey","status":"failed","summary":"v2.2 reintroduced the module import failure"}],"measurements":[{"name":"duration","value":13,"unit":"seconds"}],"cost":0.1,"currency":"USD","findings":[{"kind":"version_regression","summary":"the accepted v2.1.1 behavior regressed"}],"artifacts":["sha256:regression"]}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodPost, "/adoption-workspaces/"+workspace.ID+"/candidates", maintainer, `{"project":"Fast Compiler Demo","provider_repository":"federated:other/fast-compiler","version":"v0.8.0","revision":"unsuitable-commit"}`, http.StatusCreated, &workspace)
	unsuitableCandidate := workspace.Candidates[2].ID
	unsuitableTrials := "/adoption-workspaces/" + workspace.ID + "/candidates/" + unsuitableCandidate + "/trials"
	workflowJSON(t, server.URL, http.MethodPost, unsuitableTrials, owner, `{"name":"Unsuitable candidate isolation check","source":{"kind":"release","reference":"package:fast-compiler@0.8.0","revision":"unsuitable-commit","attestation":"release:fast-0.8"},"packages":["fast-compiler@0.8.0"],"data":[{"kind":"synthetic","description":"generated sample project"}],"journey_ids":["compile a sample project"],"policies":["no source retention"],"setup":["create an isolated temporary directory"],"commands":["fast-compiler synthetic-project"],"budget":"USD 2","evidence_audience":"public"}`, http.StatusCreated, &workspace)
	unsuitableTrial := workspace.Candidates[2].Trials[0].ID
	workflowJSON(t, server.URL, http.MethodPost, unsuitableTrials+"/"+unsuitableTrial+"/attempts", owner, `{"environment":"ubuntu-24.04 ephemeral","source_revision":"unsuitable-commit","commands":["fast-compiler synthetic-project"],"checks":[{"name":"source retention","status":"failed","summary":"candidate retains source beyond the trial"}],"measurements":[{"name":"retention","value":24,"unit":"hours"}],"cost":0.2,"currency":"USD","findings":[{"kind":"unsuitable","summary":"violates the no-retention constraint"}],"artifacts":["sha256:retention-trace"]}`, http.StatusCreated, &workspace)
	workflowJSON(t, server.URL, http.MethodGet, "/adoption-workspaces/"+workspace.ID, agent, "", http.StatusOK, &workspace)
	if workspace.AuthorityGranted || workspace.Candidates[0].Coverage["capability"] != "supported" || workspace.Candidates[0].Coverage["security"] != "stale" || workspace.Candidates[0].Evidence[2].ProofOfFit || workspace.Candidates[0].Evidence[2].Reference != "" {
		t.Fatalf("fit projection presented a gap as proof or granted authority: %#v", workspace.Candidates[0])
	}
	if len(workspace.Candidates) != 3 || workspace.Candidates[0].Trials[0].Status != "passed" || !workspace.Candidates[0].Trials[0].Attempts[1].Reproducible || len(workspace.Candidates[0].Trials[0].Feedback) != 1 || workspace.Candidates[1].Trials[0].Status != "failed" || workspace.Candidates[2].Trials[0].Status != "failed" {
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
	if publicView.Candidates[0].UpstreamShares[0].Consent != "accepted" || publicView.Candidates[0].UpstreamShares[1].Consent != "inaccessible" || publicView.Candidates[0].UpstreamShares[1].Summary != "" || publicView.Candidates[0].Contributions[1].Decision != "rejected" || publicView.Candidates[0].Contributions[2].Decision != "provider_unavailable" {
		t.Fatalf("upstream projection leaked an embargo or lost safe local outcomes: %#v", publicView.Candidates[0])
	}
}

func adoptionCommit(t *testing.T, repos *repositories.Store, repository storage.ID, author, content, parent string) storage.ObjectID {
	t.Helper()
	opened, err := repos.Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := opened.WriteObject(storage.BlobObject, []byte(content+"\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "adoption.txt", blob))
	parentLine := ""
	if parent != "" {
		parentLine = "parent " + parent + "\n"
	}
	commit, err := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+parentLine+"author "+author+" <actor@example.test> 1 +0000\ncommitter "+author+" <actor@example.test> 1 +0000\n\nsoftware adoption\n"))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}
