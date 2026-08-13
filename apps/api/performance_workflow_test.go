package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	pi "github.com/greptile-projects/vivarium-komodo/apps/api/performanceinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestPerformanceEngineeringWorkflow is the black-box boundary for the complete
// production-concern-to-validated-improvement loop. Collaboration uses public
// HTTP, delivery uses stock Git and ordinary checks/review/release policy, and
// retained performance evidence never becomes repository or deployment authority.
func TestPerformanceEngineeringWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	goals, _ := performancegoals.New(t.TempDir())
	investigations, _ := pi.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, runner, checks, goals)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerPerformanceGoalsHTTP(mux, goals, catalog, releaseStore, credentials, pulls)
	registerPerformanceInvestigationsHTTP(mux, investigations, goals, catalog, credentials, plans)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "service-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	analyst := issueAccess(t, credentials, "analyst", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "perf-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "perf-agent", auth.Git, auth.GitRead, auth.GitWrite)
	ownerGit := issueAccess(t, credentials, "service-owner", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"user-search","visibility":"private"}`, http.StatusCreated, &repository)
	for _, actor := range []string{"analyst", "perf-agent"} {
		if _, err := catalog.AddCollaborator("service-owner", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	baselineWork := gitClone(t, remote(ownerGit))
	gitOutput(t, baselineWork, "config", "user.name", "Affected service owner")
	gitOutput(t, baselineWork, "config", "user.email", "owner@example.com")
	writeWorkflowFile(t, baselineWork, "api/search.go", "package search\nfunc Query(items []string) []string { return append([]string(nil), items...) }\n")
	writeWorkflowFile(t, baselineWork, ".komodo/checks.json", `{"version":1,"checks":[{"name":"performance-contract","command":"grep -q 'preserves permission filtering' api/search.go"}]}`)
	writeWorkflowFile(t, baselineWork, ".komodo/releases.json", `{"version":1,"builds":[{"name":"search-service","command":"mkdir -p dist; cp api/search.go dist/search.go","artifacts":["dist/search.go"]}]}`)
	gitOutput(t, baselineWork, "add", ".")
	gitOutput(t, baselineWork, "commit", "-m", "Capture reported slow search service")
	baselineRevision := gitOutput(t, baselineWork, "rev-parse", "HEAD")
	gitOutput(t, baselineWork, "push", "-u", "origin", "main")
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/required-checks", owner, `{"branch":"main","checks":["performance-contract"]}`, http.StatusOK, nil)

	goalBase := "/repositories/" + string(repository.ID) + "/performance-goals"
	goalBody := `{"subject_kind":"user_journey","subject_id":"repository-search","title":"Search results arrive before users abandon","workloads":["sanitized production query distribution from issue slow-search"],"metrics":[{"id":"p95","name":"response latency","unit":"ms","direction":"lower","baseline":420,"target":{"maximum":250},"budget":275,"environment_digest":"prod-search-v1","baseline_source":"production concern issue:slow-search"}],"correctness_constraints":["preserve permission filtering and result order"],"supported_environments":[{"name":"production search","digest":"prod-search-v1"}],"owner_ids":["service-owner","analyst"],"links":[{"kind":"issue","resource_id":"slow-search","label":"Users report search is too slow"}],"baseline_max_age_days":30,"change_reason":"Turn user impact into a measurable contract"}`
	var goal performancegoals.Goal
	workflowJSON(t, server.URL, http.MethodPost, goalBase, owner, goalBody, http.StatusCreated, &goal)
	trial := func(token, revision, samples, rerun string) performancegoals.Trial {
		body := `{"version":1,"benchmark":"production-search","definition_digest":"search-workload-v1","revision":"` + revision + `","environment":{"name":"production search","digest":"prod-search-v1"},"workload_source":"sanitized_production_capture","input_digests":["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"warmup_runs":3,"sampling_method":"wall_clock","samples":` + samples + `,"resource_profile":{"cpu_seconds":3.2,"peak_memory_mb":64},"evidence":[{"kind":"profile","name":"cpu.folded","sha256":"profile-v1"}],"cost":0.08,"rerun_of":"` + rerun + `"}`
		workflowJSON(t, server.URL, http.MethodPost, goalBase+"/"+goal.ID+"/trials", token, body, http.StatusCreated, &goal)
		return goal.Trials[len(goal.Trials)-1]
	}
	baseline := trial(analyst, baselineRevision, `[{"value":410},{"value":420},{"value":430}]`, "")
	invBase := "/repositories/" + string(repository.ID) + "/performance-investigations"
	var investigation pi.Investigation
	workflowJSON(t, server.URL, http.MethodPost, invBase, analyst, `{"goal_id":"`+goal.ID+`","goal_version":1,"title":"Search allocation profile","question":"Why do production-shaped searches exceed the latency budget?","owner_ids":["service-owner"],"evidence":[{"trial_id":"`+baseline.ID+`","revision":"`+baselineRevision+`","workload_source":"sanitized_production_capture","environment_digest":"prod-search-v1","visibility":"repository"}]}`, http.StatusCreated, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/participants", analyst, `{"user_id":"service-owner"}`, http.StatusOK, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/participants", analyst, `{"user_id":"perf-agent"}`, http.StatusOK, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/entries", agent, `{"kind":"flame_graph","title":"Repeated allocation dominates","body":"The copy accounts for most sampled CPU.","audience":"repository","flamegraph":"root;Query;append 78","uncertainty":"Confirm permission behavior before removing the copy.","citations":[{"kind":"profile","trial_id":"`+baseline.ID+`"},{"kind":"symbol","path":"api/search.go","symbol":"Query"}]}`, http.StatusCreated, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/entries", owner, `{"kind":"challenge","title":"Permission behavior is non-negotiable","body":"The optimization must retain result isolation.","audience":"repository","challenges":"`+investigation.Entries[0].ID+`","citations":[{"kind":"code","path":"api/search.go","symbol":"Query"}]}`, http.StatusCreated, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/entries", agent, `{"kind":"conclusion","title":"Reuse the filtered result buffer","body":"Reuse after filtering avoids the measured copy while preserving isolation.","audience":"repository","verdict":"supported","citations":[{"kind":"profile","trial_id":"`+baseline.ID+`"},{"kind":"code","path":"api/search.go","symbol":"Query"}]}`, http.StatusCreated, &investigation)
	conclusion := investigation.Entries[len(investigation.Entries)-1]
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+investigation.ID+"/changes", analyst, `{"owner_kind":"agent","owner_id":"perf-agent","diagnosis_entry_id":"`+conclusion.ID+`","baseline_trial_id":"`+baseline.ID+`","title":"Optimize production-shaped search","constraints":["preserve permission filtering"]}`, http.StatusCreated, &investigation)

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Performance Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.com")
	gitOutput(t, agentWork, "switch", "-c", "performance/search")
	writeWorkflowFile(t, agentWork, "api/search.go", "package search\n// preserves permission filtering before buffer reuse\nfunc Query(items []string) []string { return items }\n")
	gitOutput(t, agentWork, "add", "api/search.go")
	gitOutput(t, agentWork, "commit", "-m", "Reuse filtered search buffer")
	candidateRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "performance/search")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", agent, `{"title":"Agent-assisted search optimization","body":"Diagnosis `+conclusion.ID+`; task `+investigation.Changes[0].TaskID+`.","source_branch":"performance/search","target_branch":"main"}`, http.StatusCreated, &pull)
	pullBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, agent, candidateRevision, checkruns.Succeeded)

	noisy := trial(agent, candidateRevision, `[{"value":180},{"value":420},{"value":660}]`, "")
	compare := func(candidate performancegoals.Trial, failures []string) {
		failureJSON := "[]"
		if len(failures) > 0 {
			failureJSON = `[` + `"` + failures[0] + `"` + `]`
		}
		body := `{"version":1,"baseline_trial_id":"` + baseline.ID + `","candidate_trial_id":"` + candidate.ID + `","pull_request_id":"` + pull.ID + `","metric_id":"p95","correctness_checks":["permission fixture"],"correctness_failures":` + failureJSON + `,"affected_scenarios":["repository search"],"commands":["go test ./...","bench production-search"],"residual_risks":["traffic mix drift"]}`
		workflowJSON(t, server.URL, http.MethodPost, goalBase+"/"+goal.ID+"/comparisons", analyst, body, http.StatusCreated, &goal)
	}
	workflowJSON(t, server.URL, http.MethodPost, goalBase+"/"+goal.ID+"/delivery-policies", owner, `{"branch":"main","paths":["api/*"],"thresholds":[{"metric_id":"p95","maximum_percent_regression":5,"require_confidence":true}]}`, http.StatusCreated, &goal)
	compare(noisy, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, pullBase+"/readiness", owner, "", http.StatusOK, &readiness)
	if readiness.Performance[0].Status != "uncertain" {
		t.Fatalf("noisy benchmark was trusted: %#v", readiness.Performance)
	}
	incorrect := trial(agent, candidateRevision, `[{"value":205},{"value":210},{"value":215}]`, noisy.ID)
	compare(incorrect, []string{"permission filtering changed"})
	workflowJSON(t, server.URL, http.MethodGet, pullBase+"/readiness", owner, "", http.StatusOK, &readiness)
	if readiness.Performance[0].Status != "correctness_failed" {
		t.Fatalf("correctness regression was trusted: %#v", readiness.Performance)
	}
	stable := trial(agent, candidateRevision, `[{"value":215},{"value":220},{"value":225}]`, incorrect.ID)
	compare(stable, nil)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodGet, pullBase+"/readiness", owner, "", http.StatusOK, &readiness)
	if !readiness.Ready || readiness.Performance[0].Status != "satisfied" {
		t.Fatalf("reproducible improvement not ready: %#v", readiness)
	}
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, http.StatusOK, &pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v2.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"Agent optimization reviewed against exact performance evidence."}`, http.StatusCreated, &release)
	waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, owner)

	observation := func(stage string, value float64, health, uncertainty, action, kind, id string) {
		body := `{"goal_version":1,"comparison_id":"` + goal.Comparisons[len(goal.Comparisons)-1].ID + `","release_id":"` + release.ID + `","deployment_id":"deploy-search-v2","stage":"` + stage + `","revision":"` + pull.MergeCommitID + `","metric_id":"p95","value":` + formatFloat(value) + `,"environment_digest":"prod-search-v1","health":"` + health + `","assumptions":["representative traffic"],"uncertainty":"` + uncertainty + `","action":"` + action + `","linked_resource_kind":"` + kind + `","linked_resource_id":"` + id + `"}`
		workflowJSON(t, server.URL, http.MethodPost, goalBase+"/"+goal.ID+"/delivery-observations", owner, body, http.StatusCreated, &goal)
	}
	observation("canary", 310, "passing", "rollout target missed under cache warmup", "pause", "", "")
	observation("canary-retry", 235, "passing", "", "", "", "")
	observation("production", 225, "passing", "", "", "", "")
	if len(goal.Observations) != 3 || goal.Observations[0].Outcome != "regressed" || goal.Observations[0].Action != "pause" || goal.Observations[2].Outcome != "healthy" || goal.Observations[2].Revision != pull.MergeCommitID || goal.Measurements != nil {
		t.Fatalf("production containment or delivered trail is incomplete: %#v", goal)
	}
}

func formatFloat(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	return fmt.Sprintf("%g", value)
}
