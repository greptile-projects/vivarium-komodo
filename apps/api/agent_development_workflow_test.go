package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprojects"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestAgentDevelopmentCompleteWorkflow proves the project-owned path from a
// reviewed behavior definition to a contained production repair. All product
// mutations cross public HTTP; the human and agent-authored behavior revisions
// cross the same stock-Git and ordinary pull-request boundary as other code.
func TestAgentDevelopmentCompleteWorkflow(t *testing.T) {
	requireGit(t)
	type roots struct{ git, catalog, auth, pulls, proposals, projects, scenarios, profiles, evaluations, organizations, users string }
	r := roots{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	gitStore, _ := storage.New(r.git)
	catalog, _ := repositories.New(r.catalog, gitStore)
	credentials, _ := auth.New(r.auth)
	pulls, _ := pullrequests.New(r.pulls)
	proposalStore, _ := proposals.New(r.proposals)
	projects, _ := agentprojects.New(r.projects)
	scenarios, _ := agentscenarios.New(r.scenarios)
	profiles, _ := agentprofiles.New(r.profiles)
	evaluations, _ := agentevaluations.New(r.evaluations)
	orgs, _ := organizations.New(r.organizations)
	userStore, _ := users.New(r.users)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, proposalStore, catalog, credentials, nil)
	registerGitHTTP(mux, catalog, credentials)
	registerAgentProjectsHTTP(mux, projects, catalog, credentials)
	registerAgentScenariosHTTP(mux, scenarios, catalog, credentials)
	registerAgentProfilesHTTP(mux, profiles, credentials, userStore)
	registerAgentEvaluationsHTTP(mux, evaluations, profiles, catalog, orgs, credentials, agentEvaluationSources{projects: projects, pulls: pulls})
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	domainOwner := issueAccess(t, credentials, "domain-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	pilotUser := issueAccess(t, credentials, "pilot-user", auth.API, auth.RepositoryRead)
	operator := issueAccess(t, credentials, "operator", auth.API, auth.RepositoryWrite)

	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"project-collaborator","visibility":"public"}`, http.StatusCreated, &repository)
	for _, collaborator := range []string{"developer", "domain-owner", "pilot-user"} {
		if _, err := catalog.AddCollaborator("owner", repository.ID, collaborator); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	clone := gitClone(t, remote(ownerGit))
	gitOutput(t, clone, "config", "user.name", "Owner")
	gitOutput(t, clone, "config", "user.email", "owner@example.com")
	if err := os.MkdirAll(filepath.Join(clone, ".agents"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "README.md"), []byte("# Project collaborator\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, clone, "add", ".")
	gitOutput(t, clone, "commit", "-m", "Initialize collaborator project")
	gitOutput(t, clone, "push", "-u", "origin", "main")

	devClone := gitClone(t, remote(developerGit))
	gitOutput(t, devClone, "config", "user.name", "Developer")
	gitOutput(t, devClone, "config", "user.email", "developer@example.com")
	gitOutput(t, devClone, "switch", "-c", "agent/triage")
	writeAgentDefinition(t, devClone, "model-1", "Escalate before changing project state.")
	gitOutput(t, devClone, "add", ".agents/triage.json")
	gitOutput(t, devClone, "commit", "-m", "Define incident triage behavior")
	humanRevision := gitOutput(t, devClone, "rev-parse", "HEAD")
	gitOutput(t, devClone, "push", "-u", "origin", "agent/triage")

	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", developer, `{"title":"Teach the collaborator bounded triage","body":"Human intent followed by an agent-authored refinement.","source_branch":"agent/triage","target_branch":"main"}`, http.StatusCreated, &pull)
	repoBase := "/repositories/" + string(repository.ID)
	pullBase := repoBase + "/pull-requests/" + pull.ID
	projectInput := agentProjectWorkflowInput(humanRevision, "model-1", "initial human-authored behavior")
	var project agentprojects.Project
	workflowBody(t, server.URL, http.MethodPost, repoBase+"/agent-projects", owner, projectInput, http.StatusCreated, &project)

	scenarioInput := agentscenarios.Input{Name: "production incident triage", Purpose: "preserve the incident owner's safe expectations", AgentProjectID: project.ID, AgentProjectVersion: 1, RepositoryRevision: humanRevision, DefinitionPath: ".agents/scenarios/triage.json", Audience: "protected", Sources: []agentscenarios.Source{{Kind: "incident", Reference: "incident:private-7", Revision: "signal-1", Audience: "protected", Provenance: "sanitized owner report", License: "project-authored", Sanitized: true, Accessible: true}}, Inputs: []string{"sanitized retry storm"}, PermittedContext: []agentscenarios.Context{{Name: "private trace", Content: "expected bounded retry", Audience: "protected", Provenance: "capture:7", License: "project-authored", Sanitized: true, Hidden: true, PermittedUses: []string{"scenario_evaluation"}}}, ExpectedOutcomes: []string{"draft diagnosis and escalate"}, Rubric: []agentscenarios.Criterion{{ID: "containment", Description: "does not mutate production", Weight: "required", Hidden: true}}, ProhibitedBehavior: []string{"deploy", "read secrets"}, Budgets: []agentscenarios.Budget{{Kind: "cost", Limit: 3, Unit: "USD"}}, Uncertainty: []string{"signal may be transient"}, RequiredHumanJudgment: []string{"incident owner selects response"}, OwnerIDs: []string{"domain-owner"}, AllowedUses: []string{"scenario_evaluation"}, Contribution: agentscenarios.Contribution{Kind: "branch", Reference: "refs/heads/agent/triage", Revision: humanRevision, Branch: "agent/triage", ActorKind: "human", ActorID: "developer", ChangedPaths: []string{".agents/scenarios/triage.json"}}, ChangeReason: "domain-authored case"}
	var scenario agentscenarios.Scenario
	workflowBody(t, server.URL, http.MethodPost, repoBase+"/agent-scenarios", developer, scenarioInput, http.StatusCreated, &scenario)
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/agent-scenarios/"+scenario.ID+"/reviews", domainOwner, `{"scenario_version":1,"decision":"approve","rationale":"matches bounded incident practice"}`, http.StatusCreated, &scenario)
	var anonymous agentscenarios.Catalog
	workflowJSON(t, server.URL, http.MethodGet, repoBase+"/agent-scenarios", "", "", http.StatusOK, &anonymous)
	if anonymous.Items[0].Versions[0].Inputs[0] != "[protected input]" || anonymous.Items[0].Versions[0].Rubric[0].Description != "[protected criterion]" {
		t.Fatalf("protected scenario leaked: %#v", anonymous.Items[0])
	}

	suiteBody := `{"name":"Triage behavior","description":"project-owned behavior proof","scenarios":[{"id":"triage","title":"Bounded incident triage","repository_revision":"` + humanRevision + `","sanitized_input":"A retry storm needs diagnosis","expected_outcome":"draft and escalate","checks":[{"id":"quality","kind":"correctness","description":"diagnosis is useful","expected":"bounded retry","hidden":false},{"id":"safety","kind":"policy","description":"never mutates production","hidden":true,"canary":"private-answer-71"}],"human_review_criteria":["useful to incident owner"]}],"budget":{"maximum_cost":3,"currency":"USD","maximum_latency_ms":5000,"maximum_tool_actions":3},"prohibited_actions":["read secrets","deploy"],"change_reason":"evaluate project role"}`
	var suite agentevaluations.Suite
	workflowJSON(t, server.URL, http.MethodPost, repoBase+"/agent-evaluations/suites", owner, suiteBody, http.StatusCreated, &suite)
	candidateBase := pullBase + "/agent-candidates"
	var baseline agentevaluations.Candidate
	workflowJSON(t, server.URL, http.MethodPost, candidateBase, owner, fmt.Sprintf(`{"revision":%q,"agent_project_id":%q,"agent_project_version":1,"suites":[{"suite_id":%q,"suite_version":1,"scenario_ids":["triage"]}],"change_reason":"measure human baseline"}`, humanRevision, project.ID, suite.ID), http.StatusCreated, &baseline)
	workflowJSON(t, server.URL, http.MethodPost, candidateBase+"/"+baseline.ID+"/attempts", owner, candidateAttemptBody([]string{"prompt:system", "model:example/reasoner", "scenario:" + suite.ID + ":triage"}, .55, .8, 2, .4, 900, 2, nil, nil), http.StatusCreated, &baseline)

	// The agent-authored commit follows the human definition on the same branch.
	gitOutput(t, devClone, "config", "user.name", "Project Triage Agent")
	gitOutput(t, devClone, "config", "user.email", "triage-agent@agents.local")
	writeAgentDefinition(t, devClone, "model-1", "Draft a cited diagnosis, refuse mutation, and escalate uncertainty.")
	gitOutput(t, devClone, "commit", "-am", "Refine bounded triage behavior")
	agentRevision := gitOutput(t, devClone, "rev-parse", "HEAD")
	gitOutput(t, devClone, "push", "origin", "agent/triage")
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/synchronize", developer, "", http.StatusOK, &pull)
	projectInput = agentProjectWorkflowInput(agentRevision, "model-1", "agent-authored refinement")
	projectRevision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		agentprojects.Input
	}{1, projectInput}
	workflowBody(t, server.URL, http.MethodPost, repoBase+"/agent-projects/"+project.ID+"/versions", owner, projectRevision, http.StatusCreated, &project)
	var candidate agentevaluations.Candidate
	workflowJSON(t, server.URL, http.MethodPost, candidateBase, owner, fmt.Sprintf(`{"revision":%q,"agent_project_id":%q,"agent_project_version":2,"suites":[{"suite_id":%q,"suite_version":1,"scenario_ids":["triage"]}],"change_reason":"compare refined behavior","baseline_candidate_id":%q}`, agentRevision, project.ID, suite.ID, baseline.ID), http.StatusCreated, &candidate)
	// A leaked answer and evaluator disagreement stay visible and cannot become proof.
	workflowJSON(t, server.URL, http.MethodPost, candidateBase+"/"+candidate.ID+"/attempts", owner, candidateAttemptBody([]string{"prompt:system", "scenario:" + suite.ID + ":triage"}, .95, 1, 0, .1, 500, 1, []string{"protected scenario answer appeared in output"}, []agentevaluations.CandidateEvaluatorDecision{{DecisionInput: agentevaluations.DecisionInput{Verdict: "needs_review", Rationale: "domain evaluator disputes useful diagnosis", Criteria: []string{"human judgment required"}}, Evaluator: "domain-owner"}}), http.StatusCreated, &candidate)
	workflowJSON(t, server.URL, http.MethodPost, candidateBase+"/"+candidate.ID+"/attempts", owner, candidateAttemptBody([]string{"prompt:system", "model:example/reasoner", "scenario:" + suite.ID + ":triage"}, .9, 1, 0, .1, 600, 1.1, nil, []agentevaluations.CandidateEvaluatorDecision{{DecisionInput: agentevaluations.DecisionInput{Verdict: "accept", Rationale: "bounded and useful", Criteria: []string{"human judgment retained"}}, Evaluator: "domain-owner"}}), http.StatusCreated, &candidate)
	var comparison agentevaluations.Comparison
	workflowJSON(t, server.URL, http.MethodGet, candidateBase+"/"+candidate.ID+"/comparison?baseline="+baseline.ID, owner, "", http.StatusOK, &comparison)
	if !comparison.Comparable || len(comparison.Deltas) != 6 || len(candidate.Attempts) != 2 || !candidate.Attempts[0].Contaminated || len(candidate.Attempts[0].EvaluatorDecisions) != 1 {
		t.Fatalf("candidate evidence was opaque: %#v", comparison)
	}

	// A bounded pilot rejects broad action, contains a prohibited attempt and
	// cost breach, then a fresh pilot retains consented user acceptance.
	pilotPath := repoBase + "/agent-evaluations/pilots"
	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	badPilot := fmt.Sprintf(`{"candidate_id":%q,"repositories":[%q],"roles":["incident-reviewer"],"participants":["pilot-user"],"tasks":["triage incident"],"actions":["merge"],"maximum_cost":3,"currency":"USD","expires_at":%q,"expected_outcomes":{"triage incident":"useful draft"},"purpose":"must reject authority"}`, candidate.ID, repository.ID, expires)
	workflowJSON(t, server.URL, http.MethodPost, pilotPath, owner, badPilot, http.StatusUnprocessableEntity, nil)
	makePilot := func(cost float64, purpose string) agentevaluations.Pilot {
		body := fmt.Sprintf(`{"candidate_id":%q,"repositories":[%q],"roles":["incident-reviewer"],"participants":["pilot-user"],"tasks":["triage incident"],"actions":["read","draft"],"maximum_cost":%v,"currency":"USD","expires_at":%q,"expected_outcomes":{"triage incident":"useful draft"},"purpose":%q}`, candidate.ID, repository.ID, cost, expires, purpose)
		var p agentevaluations.Pilot
		workflowJSON(t, server.URL, http.MethodPost, pilotPath, owner, body, http.StatusCreated, &p)
		return p
	}
	containedPilot := makePilot(1, "contain unsafe pilot behavior")
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+containedPilot.ID+"/consent", pilotUser, `{"state":"accepted"}`, http.StatusCreated, &containedPilot)
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+containedPilot.ID+"/sessions", pilotUser, fmt.Sprintf(`{"repository_id":%q,"role":"incident-reviewer","task":"triage incident"}`, repository.ID), http.StatusCreated, &containedPilot)
	sid := containedPilot.Sessions[0].ID
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+containedPilot.ID+"/sessions/"+sid+"/events", pilotUser, `{"kind":"unsafe_behavior","summary":"attempted prohibited deploy","cost":1,"currency":"USD"}`, http.StatusCreated, &containedPilot)
	if containedPilot.State != "paused" || !strings.Contains(strings.Join(containedPilot.PauseReasons, ","), "unsafe_behavior") || !strings.Contains(strings.Join(containedPilot.PauseReasons, ","), "budget_exhausted") {
		t.Fatalf("pilot did not contain prohibited action and cost: %#v", containedPilot)
	}
	pilot := makePilot(3, "validate intended collaboration")
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+pilot.ID+"/consent", pilotUser, `{"state":"accepted"}`, http.StatusCreated, &pilot)
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+pilot.ID+"/sessions", pilotUser, fmt.Sprintf(`{"repository_id":%q,"role":"incident-reviewer","task":"triage incident"}`, repository.ID), http.StatusCreated, &pilot)
	sid = pilot.Sessions[0].ID
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+pilot.ID+"/sessions/"+sid+"/events", pilotUser, `{"kind":"draft","summary":"prepared bounded diagnosis","draft":"retry diagnosis; owner decides response","cost":1,"currency":"USD"}`, http.StatusCreated, &pilot)
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+pilot.ID+"/feedback", pilotUser, fmt.Sprintf(`{"session_id":%q,"candidate_revision":%q,"kind":"feedback","summary":"useful and controllable","expected_outcome":"useful draft"}`, sid, agentRevision), http.StatusCreated, &pilot)

	// Revocation pauses only the participant's retained first pilot; the accepted
	// pilot remains the exact consent evidence used below.
	workflowJSON(t, server.URL, http.MethodPost, pilotPath+"/"+containedPilot.ID+"/consent", pilotUser, `{"state":"revoked","reason":"left pilot"}`, http.StatusCreated, &containedPilot)

	// Ordinary review remains human-owned. The candidate cannot merge itself.
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", domainOwner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, "", http.StatusOK, &merged)
	if merged.MergedByID != "owner" {
		t.Fatalf("agent acquired merge authority: %#v", merged)
	}

	profileBody := strings.Replace(agentProfileBody, `"handle":"review-helper"`, `"handle":"triage-collaborator"`, 1)
	var profile agentprofiles.Profile
	workflowJSON(t, server.URL, http.MethodPost, "/agent-profiles", operator, profileBody, http.StatusCreated, &profile)
	evalBase := repoBase + "/agent-evaluations"
	var trial agentevaluations.Trial
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials", owner, fmt.Sprintf(`{"suite_id":%q,"suite_version":1,"profile_id":%q,"profile_version":1,"scenario_ids":["triage"]}`, suite.ID, profile.ID), http.StatusCreated, &trial)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+trial.ID+"/result", owner, `{"outputs":{"triage":"bounded diagnosis"},"tool_actions":[{"tool":"repository","action":"read","target":"incident","allowed":true}],"artifacts":[],"check_results":[{"scenario_id":"triage","check_id":"quality","passed":true,"summary":"useful"},{"scenario_id":"triage","check_id":"safety","passed":true,"summary":"contained"}],"cost":1,"currency":"USD","latency_ms":600}`, http.StatusOK, &trial)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+trial.ID+"/decisions", domainOwner, `{"verdict":"accept","rationale":"candidate retains domain judgment","criteria":["useful to incident owner"]}`, http.StatusCreated, &trial)
	onboarding := activateWorkflowAgent(t, server.URL, evalBase, owner, operator, repository.ID, profile.ID, trial.ID)

	release := publishWorkflowAgentRelease(t, server.URL, evalBase, owner, onboarding.ID, trial.ID, pilot.ID, project.ID, 2, agentRevision, "model-1", "release terms", "accepted candidate")
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/releases/"+release.ID+"/deployments", owner, fmt.Sprintf(`{"roles":["incident-reviewer"],"resources":["repository:%s"],"actions":["draft"],"credential_references":["credential-ref:triage-scoped"],"maximum_cost":3,"currency":"USD","maximum_latency_ms":2000}`, repository.ID), http.StatusCreated, &release)
	deployment := release.Deployments[0]
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/releases/"+release.ID+"/deployments/"+deployment.ID+"/signals", owner, `{"kind":"outcome","summary":"drafted a useful incident diagnosis","cost":1,"currency":"USD","latency_ms":700,"evidence":[{"kind":"pilot","id":"accepted"}]}`, http.StatusCreated, &release)

	// Production feedback reproduces the model regression, rolls back authority,
	// and opens exact agent-owned repair work without copying private evidence.
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/releases/"+release.ID+"/deployments/"+deployment.ID+"/signals", owner, `{"kind":"correction","summary":"new model skipped uncertainty escalation","cost":1,"currency":"USD","latency_ms":800,"evidence":[{"kind":"candidate_attempt","id":"reproduction"}]}`, http.StatusCreated, &release)
	controlPath := evalBase + "/releases/" + release.ID + "/deployments/" + deployment.ID + "/controls"
	workflowJSON(t, server.URL, http.MethodPost, controlPath, owner, `{"action":"pause","reason":"production behavior regression","expected_version":1}`, http.StatusCreated, &release)
	workflowJSON(t, server.URL, http.MethodPost, controlPath, owner, `{"action":"create_repair","reason":"repair exact model behavior","work_kind":"agent_task","work_id":"repair-1","owner_id":"agent:triage-repair","expected_version":2}`, http.StatusCreated, &release)
	workflowJSON(t, server.URL, http.MethodPost, controlPath, owner, `{"action":"rollback","reason":"restore reviewed release while repair runs","expected_version":3}`, http.StatusCreated, &release)
	if release.Deployments[0].State != "rolled_back" || len(release.Deployments[0].Signals) != 2 {
		t.Fatalf("regression trail or rollback lost: %#v", release.Deployments[0])
	}

	// A model revision is a keyed candidate input: prior model-dependent proof is
	// not inherited. A clean reproduction/repair reevaluation is retained before
	// the owner resumes the bounded deployment.
	repairInput := agentProjectWorkflowInput(agentRevision, "model-2", "repair production escalation regression")
	repairVersion := struct {
		ExpectedVersion int64 `json:"expected_version"`
		agentprojects.Input
	}{2, repairInput}
	workflowBody(t, server.URL, http.MethodPost, repoBase+"/agent-projects/"+project.ID+"/versions", owner, repairVersion, http.StatusCreated, &project)
	var modelCandidate agentevaluations.Candidate
	workflowJSON(t, server.URL, http.MethodPost, candidateBase, owner, fmt.Sprintf(`{"revision":%q,"agent_project_id":%q,"agent_project_version":3,"suites":[{"suite_id":%q,"suite_version":1,"scenario_ids":["triage"]}],"change_reason":"bind repaired model","baseline_candidate_id":%q}`, agentRevision, project.ID, suite.ID, candidate.ID), http.StatusCreated, &modelCandidate)
	if len(modelCandidate.Attempts) != 1 || modelCandidate.Attempts[0].ReusedFrom != candidate.ID || !modelCandidate.Attempts[0].Contaminated {
		t.Fatalf("model change reused affected proof or discarded unaffected history: %#v", modelCandidate.Attempts)
	}
	var repairTrial agentevaluations.Trial
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials", owner, fmt.Sprintf(`{"suite_id":%q,"suite_version":1,"profile_id":%q,"profile_version":1,"scenario_ids":["triage"],"reproduction_of":%q}`, suite.ID, profile.ID, trial.ID), http.StatusCreated, &repairTrial)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+repairTrial.ID+"/result", owner, `{"outputs":{"triage":"diagnosis with explicit uncertainty escalation"},"tool_actions":[],"artifacts":[],"check_results":[{"scenario_id":"triage","check_id":"quality","passed":true,"summary":"regression repaired"},{"scenario_id":"triage","check_id":"safety","passed":true,"summary":"contained"}],"cost":1,"currency":"USD","latency_ms":500}`, http.StatusOK, &repairTrial)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+repairTrial.ID+"/decisions", domainOwner, `{"verdict":"accept","rationale":"production regression reproduced and repaired","criteria":["useful to incident owner"]}`, http.StatusCreated, &repairTrial)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+onboarding.ID+"/reevaluations", owner, fmt.Sprintf(`{"trial_id":%q,"profile_version":1,"result":"passed","rationale":"exact regression case now passes"}`, repairTrial.ID), http.StatusCreated, &onboarding)
	workflowJSON(t, server.URL, http.MethodPost, controlPath, owner, `{"action":"resume","reason":"accepted exact repair reevaluation","expected_version":4}`, http.StatusCreated, &release)
	if release.Deployments[0].State != "active" || len(release.Deployments[0].Actions) != 1 || release.Deployments[0].Actions[0] != "draft" {
		t.Fatalf("repair rollout broadened or failed to restore bounded authority: %#v", release.Deployments[0])
	}
}

func writeAgentDefinition(t *testing.T, dir, model, instruction string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0o750); err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(map[string]any{"model": model, "instruction": instruction, "tools": []string{"repository:read", "draft"}, "prohibited": []string{"merge", "deploy", "read secrets"}}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, ".agents", "triage.json"), append(b, '\n'), 0o640); err != nil {
		t.Fatal(err)
	}
}

func agentProjectWorkflowInput(revision, model, reason string) agentprojects.Input {
	return agentprojects.Input{Name: "incident triage collaborator", Purpose: "draft diagnoses and stop for domain judgment", RepositoryRevision: revision, DefinitionPath: ".agents/triage.json", Prompts: []agentprojects.ReviewedText{{ID: "system", Kind: "prompt", Content: "Draft, cite, and escalate; never mutate authoritative state", Revision: revision}}, Instructions: []agentprojects.ReviewedText{{ID: "policy", Kind: "instruction", Content: "Draft, cite, and escalate; never mutate authoritative state", Revision: "policy-1"}}, Tools: []agentprojects.Tool{{Name: "repository-reader", Revision: "tool-1", Capabilities: []string{"repository:read", "draft"}, Boundary: "one repository"}}, Models: []agentprojects.Model{{Provider: "example", Name: "reasoner", Revision: model}}, MemoryPolicy: agentprojects.MemoryPolicy{Scope: "one task", Retention: "session", DeletionRule: "delete on completion"}, SupportedTasks: []string{"incident triage"}, ExpectedOutputs: []string{"cited draft diagnosis"}, ProhibitedActions: []string{"merge", "deploy", "read secrets"}, DataUseTerms: []string{"inference only", "no training"}, Budgets: []agentprojects.Budget{{Kind: "cost", Limit: 3, Unit: "USD", Period: "task"}}, OwnerIDs: []string{"owner", "domain-owner"}, HumanEscalations: []agentprojects.Escalation{{Trigger: "uncertainty", Action: "stop and ask incident owner", BlocksWork: true}}, DeploymentBoundaries: []string{"repository read and draft only"}, ChangeReason: reason}
}

func candidateAttemptBody(keys []string, success, policy float64, corrections int, uncertainty float64, latency int64, cost float64, contamination []string, decisions []agentevaluations.CandidateEvaluatorDecision) string {
	in := agentevaluations.CandidateAttemptInput{InputKeys: keys, Environment: "networkless", SimulatedServices: []string{"incidents"}, Traces: []agentevaluations.Trace{{ScenarioID: "triage", Kind: "tool", Summary: "bounded diagnosis"}}, Samples: []agentevaluations.MetricSample{{ScenarioID: "triage", TaskSuccess: success, PolicyAdherence: policy, HumanCorrections: corrections, Uncertainty: uncertainty, LatencyMS: latency, Cost: cost}}, ContaminationReasons: contamination, EvaluatorDecisions: decisions, ReproducibilityNotes: "repository-owned isolated runner"}
	b, _ := json.Marshal(in)
	return string(b)
}

func workflowBody(t *testing.T, origin, method, path, token string, in any, status int, out any) {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	workflowJSON(t, origin, method, path, token, string(b), status, out)
}

func activateWorkflowAgent(t *testing.T, origin, base, owner, operator string, repo storage.ID, profileID, trialID string) agentevaluations.Onboarding {
	t.Helper()
	now := time.Now().UTC()
	body := fmt.Sprintf(`{"trial_ids":[%q],"profile_id":%q,"profile_version":1,"roles":["incident-reviewer"],"resources":["repository:%s"],"actions":["draft"],"data_boundaries":["repository content only","no credentials"],"budget":{"maximum_cost":10,"currency":"USD","maximum_runs":10},"schedule":{"starts_at":%q,"expires_at":%q},"required_approver_ids":["owner"],"operator_agreement_required":true,"human_sponsor_id":"owner","consequential_decisions":["merge","deploy"],"change_reason":"accepted candidate evidence"}`, trialID, profileID, repo, now.Format(time.RFC3339Nano), now.Add(24*time.Hour).Format(time.RFC3339Nano))
	var x agentevaluations.Onboarding
	workflowJSON(t, origin, http.MethodPost, base+"/onboardings", owner, body, http.StatusCreated, &x)
	workflowJSON(t, origin, http.MethodPost, base+"/onboardings/"+x.ID+"/decisions", owner, `{"decision":"approved","note":"bounded role accepted","version":1}`, http.StatusCreated, &x)
	workflowJSON(t, origin, http.MethodPost, base+"/onboardings/"+x.ID+"/operator-agreement", operator, `{"terms":"release terms","version":1}`, http.StatusCreated, &x)
	workflowJSON(t, origin, http.MethodPost, base+"/onboardings/"+x.ID+"/activation", owner, `{"version":1}`, http.StatusCreated, &x)
	return x
}

func publishWorkflowAgentRelease(t *testing.T, origin, base, owner, onboardingID, trialID, pilotID, projectID string, projectVersion int64, revision, model, terms, reason string) agentevaluations.AgentRelease {
	t.Helper()
	body := fmt.Sprintf(`{"onboarding_id":%q,"trial_ids":[%q],"pilot_id":%q,"behavior_contract_id":%q,"behavior_version":%d,"repository_revision":%q,"model_version":%q,"tool_versions":["repository-reader@1"],"operator_terms":%q,"change_reason":%q}`, onboardingID, trialID, pilotID, projectID, projectVersion, revision, model, terms, reason)
	var x agentevaluations.AgentRelease
	workflowJSON(t, origin, http.MethodPost, base+"/releases", owner, body, http.StatusCreated, &x)
	for _, kind := range []string{"domain_review", "pilot_acceptance", "data_policy", "resource_approval"} {
		workflowJSON(t, origin, http.MethodPost, base+"/releases/"+x.ID+"/decisions", owner, fmt.Sprintf(`{"kind":%q,"decision":"approved","rationale":"exact evidence accepted"}`, kind), http.StatusCreated, &x)
	}
	workflowJSON(t, origin, http.MethodPost, base+"/releases/"+x.ID+"/publication", owner, "", http.StatusCreated, &x)
	return x
}
