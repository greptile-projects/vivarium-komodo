package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestProductExperimentWorkflow is the black-box boundary for the complete
// feedback-to-learned-product loop. Collaboration uses public HTTP, variant
// delivery uses stock Git and ordinary review/release/deployment policy, and
// experiment evidence never becomes code, audience, or operational authority.
func TestProductExperimentWorkflow(t *testing.T) {
	requireGit(t)
	if _, err := os.Stat("/usr/bin/bwrap"); err != nil {
		t.Skip("bwrap is required for the product experiment workflow")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	deploymentStore, _ := deployments.New(t.TempDir())
	experiments, _ := productexperiments.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, checks)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerDeploymentsHTTP(mux, deploymentStore, releaseStore, checks, catalog, credentials, nil, nil, pulls)
	registerProductExperimentsHTTP(mux, experiments, catalog, credentials, pulls, releaseStore, deploymentStore)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "product-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	human := issueAccess(t, credentials, "human-designer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "experiment-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "product-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "experiment-agent", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"guided-onboarding","visibility":"private"}`, http.StatusCreated, &repository)
	for _, actor := range []string{"human-designer", "experiment-agent"} {
		if _, err := catalog.AddCollaborator("product-owner", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}

	base := "/repositories/" + string(repository.ID) + "/product-experiments"
	var signal productexperiments.Signal
	workflowJSON(t, server.URL, http.MethodPost, base+"/signals", owner, `{"name":"repository activation","description":"A new owner completes setup","unit":"users","event":"repository.activated","properties":["variant","completed"],"permitted_audiences":["product_analytics"],"instrumented":true,"change_reason":"User feedback asks for clearer setup guidance"}`, http.StatusCreated, &signal)
	plan := `{"title":"Help new owners finish setup","source":{"kind":"issue","id":"feedback-confusing-first-run"},"hypothesis":"Guided setup increases activation without increasing support contacts","variants":[{"id":"control","name":"Current setup","control":true},{"id":"guided","name":"Guided setup"}],"target_audience":{"description":"Consenting new repository owners","eligibility":["repository created in prior seven days"],"exclusions":["staff","prior experiment participants"],"consent":"product_analytics","estimated_size":500},"measures":[{"id":"activation","name":"Activation","kind":"success","signal_id":"` + signal.ID + `","signal_version":1,"aggregation":"conversion rate","threshold":"at least +5%"},{"id":"support","name":"Support contacts","kind":"guardrail","signal_id":"` + signal.ID + `","signal_version":1,"aggregation":"contacts per owner","threshold":"no more than +2%"}],"minimum_evidence":"100 users per variant at 95% confidence","duration_hours":168,"owner_ids":["product-owner"],"participant_ids":["product-owner","human-designer","experiment-agent"],"stop_conditions":["support contacts increase more than 2%"],"assumptions":["activation event remains stable"],"overlap_keys":["onboarding:new-owner"],"change_reason":"Turn user feedback into a testable contract"}`
	var experiment productexperiments.Experiment
	workflowJSON(t, server.URL, http.MethodPost, base, owner, plan, http.StatusCreated, &experiment)
	for _, actor := range []string{owner, human, agent} {
		workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/approvals", actor, `{"decision":"approved","note":"The hypothesis, consent, and stop rule are explicit"}`, http.StatusCreated, &experiment)
	}

	ownerWork := gitClone(t, remote(ownerGit))
	gitOutput(t, ownerWork, "config", "user.name", "Human Designer")
	gitOutput(t, ownerWork, "config", "user.email", "human@example.com")
	writeWorkflowFile(t, ownerWork, "onboarding.txt", "current setup\n")
	writeWorkflowFile(t, ownerWork, ".komodo/checks.json", `{"version":1,"checks":[{"name":"experiment-contract","command":"grep -q 'assignment: stable' experiment.txt && grep -q 'fallback: current setup' experiment.txt"}]}`)
	writeWorkflowFile(t, ownerWork, ".komodo/releases.json", `{"version":1,"builds":[{"name":"web","command":"mkdir -p dist; cp onboarding.txt dist/onboarding.txt","artifacts":["dist/onboarding.txt"]}]}`)
	writeWorkflowFile(t, ownerWork, ".komodo/deployments.json", `{"version":1,"environments":[{"name":"production","stages":[{"name":"rollout","health":[{"name":"experience-readable","command":"test -s \"$KOMODO_ARTIFACT_PATH\""}]}]}]}`)
	gitOutput(t, ownerWork, "add", ".")
	gitOutput(t, ownerWork, "commit", "-m", "Capture current onboarding experience")
	gitOutput(t, ownerWork, "push", "-u", "origin", "main")

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Human Designer")
	gitOutput(t, agentWork, "config", "user.email", "human@example.com")
	gitOutput(t, agentWork, "switch", "-c", "experiment/guided-onboarding")
	writeWorkflowFile(t, agentWork, "onboarding.txt", "guided setup designed by human collaborator\n")
	gitOutput(t, agentWork, "add", "onboarding.txt")
	gitOutput(t, agentWork, "commit", "-m", "Design guided onboarding variant")
	gitOutput(t, agentWork, "config", "user.name", "Experiment Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.com")
	writeWorkflowFile(t, agentWork, "experiment.txt", "assignment: stable\nmetric: repository.activated variant completed\nisolation: mutually exclusive\nfallback: current setup\nremoval: delete flag and collection after decision\n")
	gitOutput(t, agentWork, "add", "experiment.txt")
	gitOutput(t, agentWork, "commit", "-m", "Instrument privacy-bounded experiment")
	candidate := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "experiment/guided-onboarding")
	for _, item := range []string{
		`{"kind":"workspace","owner_kind":"human","owner_id":"human-designer","variant_ids":["guided"],"resource_id":"workspace-design","revision":"` + candidate + `"}`,
		`{"kind":"session","owner_kind":"agent","owner_id":"experiment-agent","variant_ids":["control","guided"],"resource_id":"session-instrumentation","revision":"` + candidate + `"}`,
	} {
		workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/work-items", owner, item, http.StatusCreated, &experiment)
	}
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", agent, `{"title":"Human-designed, agent-instrumented onboarding experiment","body":"Implements the approved experiment with explicit fallback and removal.","source_branch":"experiment/guided-onboarding","target_branch":"main"}`, http.StatusCreated, &pull)
	waitForWorkflowCheck(t, server.URL, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID, agent, candidate, checkruns.Succeeded)
	implementation := `{"pull_request_id":"` + pull.ID + `","variant_ids":["control","guided"],"event_definitions":[{"signal_id":"` + signal.ID + `","signal_version":1,"event":"repository.activated","properties":["variant","completed"]}],"exposure_rules":["stable deterministic assignment after consent"],"privacy_classification":"consented product analytics","removal_plan":"delete variant flag and event collection after decision","check_names":{"assignment":"experiment-contract","metric_capture":"experiment-contract","variant_isolation":"experiment-contract","fallback":"experiment-contract"}}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/implementations", owner, implementation, http.StatusCreated, &experiment)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID+"/merge", owner, `{}`, http.StatusOK, &pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"Reviewed onboarding experiment."}`, http.StatusCreated, &release)
	build, artifact := waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, owner)
	var environment deployments.Environment
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/environments", owner, `{"name":"production","position":1,"command":"printf deployed","required_approvals":1,"concurrency":1}`, http.StatusCreated, &environment)
	started := promoteAndApprove(t, server.URL, string(repository.ID), human, owner, environment.ID, release.ID, build.ID, artifact.ID)
	deployed := waitForDeployment(t, server.URL, string(repository.ID), started.ID, owner, "succeeded")

	policy := `{"expected_plan_version":1,"release_id":"` + release.ID + `","variant_ids":["control","guided"],"mutual_exclusion_group":"onboarding","eligibility":{"consent_class":"product_analytics","regions":["US","EU"],"organization_ids":[],"required_attributes":["new_owner"],"excluded_attributes":["staff"]},"allocation":[{"variant_id":"control","basis_points":5000},{"variant_id":"guided","basis_points":5000}],"collection":[{"signal_id":"` + signal.ID + `","signal_version":1,"properties":["variant","completed"]}],"retention_days":30,"approver_ids":["product-owner","human-designer"],"change_reason":"Bound exposure to consent and minimal evidence"}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/audience-policies", owner, policy, http.StatusCreated, &experiment)
	for _, actor := range []string{owner, human} {
		workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/audience-policies/approval", actor, `{"decision":"approved","note":"Audience and collection are bounded"}`, http.StatusCreated, &experiment)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/audience-policies/assignments", owner, `{"subject":"user-42","region":"US","consent_classes":["product_analytics"],"attributes":["new_owner"]}`, http.StatusCreated, &experiment)
	launch := func() productexperiments.Run {
		body := `{"environment_id":"` + environment.ID + `","deployment_id":"` + deployed.ID + `","stages":[{"name":"canary","max_exposure":500,"allocation":[{"variant_id":"control","basis_points":5000},{"variant_id":"guided","basis_points":5000}]},{"name":"progressive","max_exposure":5000,"allocation":[{"variant_id":"control","basis_points":4000},{"variant_id":"guided","basis_points":6000}]}]}`
		workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs", owner, body, http.StatusCreated, &experiment)
		return experiment.Runs[len(experiment.Runs)-1]
	}
	failed := launch()
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs/"+failed.ID+"/observations", human, `{"exposure_by_variant":{"control":20,"guided":20},"measure_values":{"activation":0.40,"support":0.05},"uncertainty":{"activation":0.10,"support":0.02},"data_quality":"healthy","operational_health":"healthy","instrumentation_health":"healthy","consent_health":"valid","guardrail_breached":true,"cost_units":4,"evidence":["aggregate:canary-guardrail"]}`, http.StatusCreated, &experiment)
	retry := launch()
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs/"+retry.ID+"/observations", human, `{"exposure_by_variant":{"control":50,"guided":50},"measure_values":{"activation":0.48,"support":0.01},"uncertainty":{"activation":0.06,"support":0.01},"data_quality":"healthy","operational_health":"healthy","instrumentation_health":"healthy","consent_health":"valid","cost_units":8,"evidence":["aggregate:retry-canary"]}`, http.StatusCreated, &experiment)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs/"+retry.ID+"/stages/advance", owner, `{"reason":"Canary evidence is healthy and consent-valid"}`, http.StatusCreated, &experiment)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs/"+retry.ID+"/observations", human, `{"exposure_by_variant":{"control":120,"guided":180},"measure_values":{"activation":0.58,"support":0.01},"uncertainty":{"activation":0.03,"support":0.01},"data_quality":"healthy","operational_health":"healthy","instrumentation_health":"healthy","consent_health":"valid","cost_units":18,"evidence":["aggregate:retry-threshold"]}`, http.StatusCreated, &experiment)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs/"+retry.ID+"/controls", owner, `{"action":"stop","reason":"Declared evidence threshold reached"}`, http.StatusCreated, &experiment)
	observation := experiment.Runs[1].Observations[1]
	analysis := `{"run_id":"` + retry.ID + `","observation_id":"` + observation.ID + `","evidence_state":"threshold_reached","summary":"Guided setup improves activation with a healthy support guardrail","segment_effects":[{"segment":"new owners","variant_id":"guided","measure_id":"activation","effect":0.10,"uncertainty":0.03,"sample_size":300}],"exclusions":["staff","non-consenting owners"],"guardrails":[{"measure_id":"support","status":"passed","value":0.01,"uncertainty":0.01}],"interpretation":{"summary":"The retry clears the threshold after the unsafe attempt was contained","actor_kind":"agent","actor_id":"experiment-agent","evidence":["aggregate:retry-threshold"],"uncertainty":"Long-term retention is not measured"},"dissent":[{"actor_id":"human-designer","position":"Roll out progressively and retain the fallback","evidence":["segment:new-owners"]}],"aggregated_evidence":["aggregate:canary-guardrail","aggregate:retry-threshold"]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/analyses", agent, analysis, http.StatusCreated, &experiment)
	decision := `{"expected_version":0,"analysis_id":"` + experiment.Analyses[0].ID + `","outcome":"adopt_variant","adopted_variant_id":"guided","rationale":"The successful retry meets activation and guardrail thresholds","user_protections":["retain consent boundary","preserve fallback during rollout"],"tasks":[{"kind":"rollout","title":"Roll out guided setup and remove experiment machinery","owner_id":"product-owner","required_actions":["ship chosen experience","remove control flag","stop event collection"]}],"change_reason":"Acknowledge credible retry evidence and dissent"}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/decisions", owner, decision, http.StatusCreated, &experiment)
	d := experiment.Decisions[0]
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/decisions/"+d.ID+"/tasks/"+d.Tasks[0].ID+"/complete", owner, `{"pull_request_id":"`+pull.ID+`","release_id":"`+release.ID+`","deployment_id":"`+deployed.ID+`","evidence":["experiment-contract passed","guided experience deployed"]}`, http.StatusCreated, &experiment)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/decisions/"+d.ID+"/cleanup", owner, `{}`, http.StatusCreated, &experiment)
	if experiment.Runs[0].Status != "contained" || experiment.Runs[1].Status != "stopped" || experiment.Cleanup == nil || !experiment.Cleanup.CollectionStopped || len(experiment.WorkItems) != 2 || experiment.Implementations[0].SourceCommitID != candidate || experiment.Decisions[0].Tasks[0].DeploymentID != deployed.ID {
		t.Fatalf("experiment trail is incomplete: %#v", experiment)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+experiment.ID+"/runs", owner, `{}`, http.StatusConflict, nil)
}
