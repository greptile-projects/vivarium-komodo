package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentdiscovery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestDeveloperAndAgentCompleteCollaborationWorkflow proves that delegation is
// one public workflow, rather than a collection of independently tested
// handlers. Developer actions use JSON HTTP, and agent code publication uses a
// stock Git client plus the credential-bound worker API. The application is
// restarted mid-run before the worker publishes its result.
func TestDeveloperAndAgentCompleteCollaborationWorkflow(t *testing.T) {
	requireGit(t)
	type roots struct{ git, catalog, auth, pulls, proposals, sessions, activities, users, profiles, discovery, evaluations, organizations string }
	r := roots{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	start := func() *httptest.Server {
		gitStorage, _ := storage.New(r.git)
		catalog, _ := repositories.New(r.catalog, gitStorage)
		credentials, _ := auth.New(r.auth)
		pulls, _ := pullrequests.New(r.pulls)
		proposalStore, _ := proposals.New(r.proposals)
		sessions, _ := changesessions.New(r.sessions)
		userStore, _ := users.New(r.users)
		activityStore, _ := activities.New(r.activities, userStore)
		profiles, _ := agentprofiles.New(r.profiles)
		discovery, _ := agentdiscovery.New(r.discovery)
		evaluations, _ := agentevaluations.New(r.evaluations)
		orgs, _ := organizations.New(r.organizations)
		mux := http.NewServeMux()
		registerAgentProfilesHTTP(mux, profiles, credentials, userStore)
		registerAgentDiscoveryHTTP(mux, discovery, profiles, catalog, credentials)
		registerAgentEvaluationsHTTP(mux, evaluations, profiles, catalog, orgs, credentials)
		registerRepositoriesHTTP(mux, catalog, credentials)
		registerPullRequestsHTTP(mux, pulls, proposalStore, catalog, credentials, activityStore)
		registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, activityStore)
		registerGitHTTP(mux, catalog, credentials)
		return httptest.NewServer(mux)
	}
	credentials, _ := auth.New(r.auth)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	operator := issueAccess(t, credentials, "operator", auth.API, auth.RepositoryWrite)
	replacementOperator := issueAccess(t, credentials, "replacement-operator", auth.API, auth.RepositoryWrite)

	server := start()
	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer, `{"name":"agent-loop","visibility":"private"}`, http.StatusCreated, &repository)
	// Catalog membership is itself part of the public product, but contributor
	// invitation is covered by the human workflow; use the boundary here to keep
	// this proof focused on delegation.
	gitStorage, _ := storage.New(r.git)
	catalog, _ := repositories.New(r.catalog, gitStorage)
	if _, err := catalog.AddCollaborator("maintainer", storage.ID(repository.ID), "developer"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	maintainerClone := gitClone(t, remote(maintainerGit))
	gitOutput(t, maintainerClone, "config", "user.name", "Maintainer")
	gitOutput(t, maintainerClone, "config", "user.email", "maintainer@example.com")
	if err := os.WriteFile(filepath.Join(maintainerClone, "README.md"), []byte("# Agent loop\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, maintainerClone, "add", "README.md")
	gitOutput(t, maintainerClone, "commit", "-m", "Initialize project")
	gitOutput(t, maintainerClone, "push", "-u", "origin", "main")

	developerClone := gitClone(t, remote(developerGit))
	gitOutput(t, developerClone, "config", "user.name", "Developer")
	gitOutput(t, developerClone, "config", "user.email", "developer@example.com")
	gitOutput(t, developerClone, "switch", "-c", "candidate/agent-docs")
	if err := os.WriteFile(filepath.Join(developerClone, "GUIDE.md"), []byte("# Guide\n\nDraft.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, developerClone, "add", "GUIDE.md")
	gitOutput(t, developerClone, "commit", "-m", "Start guide")
	seedRevision := gitOutput(t, developerClone, "rev-parse", "HEAD")
	gitOutput(t, developerClone, "push", "-u", "origin", "candidate/agent-docs")

	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", developer, `{"title":"Finish the agent guide","body":"Delegate the final documentation pass.","source_branch":"candidate/agent-docs","target_branch":"main"}`, http.StatusCreated, &pull)
	base := "/repositories/" + repository.ID + "/pull-requests/" + pull.ID

	// Selection and adoption use public, independently owned profile evidence.
	// The resulting identities are deliberately not maintainer identities; their
	// only code path below is the ordinary branch-scoped session credential.
	var primary, replacement agentprofiles.Profile
	workflowJSON(t, server.URL, http.MethodPost, "/agent-profiles", operator, agentProfileBody, http.StatusCreated, &primary)
	replacementBody := strings.Replace(agentProfileBody, `"handle":"review-helper"`, `"handle":"docs-specialist"`, 1)
	replacementBody = strings.Replace(replacementBody, `"display_name":"Review Helper"`, `"display_name":"Docs Specialist"`, 1)
	workflowJSON(t, server.URL, http.MethodPost, "/agent-profiles", replacementOperator, replacementBody, http.StatusCreated, &replacement)
	searchBody := `{"context_type":"team_role","context_id":"docs-stream","public_summary":"Finish a reviewed guide","audience":"repository","workflow":"code review","required_permissions":["contents:read","discussion:write"],"allowed_boundaries":["EU"],"required_policy_terms":["requested review","never used for training","deleted within 24 hours"],"comparable_tags":["documentation"],"maximum_cost":0.03,"currency":"USD","availability_terms":["weekdays"]}`
	var search agentdiscovery.Search
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/agent-discovery/searches", developer, searchBody, http.StatusCreated, &search)
	if len(search.Matches) != 2 || search.Matches[0].Handle != "docs-specialist" || search.Matches[1].Handle != "review-helper" || !search.Matches[1].Eligible {
		t.Fatalf("comparison was scored, incomplete, or ineligible: %+v", search.Matches)
	}

	suiteBody := `{"name":"Documentation collaborator","description":"Repository-owned correctness and policy proof","scenarios":[{"id":"guide","title":"Repair the guide","repository_revision":"` + seedRevision + `","sanitized_input":"A guide needs an exact test command","expected_outcome":"A focused documented command","checks":[{"id":"visible","kind":"correctness","description":"command is exact","expected":"go test ./...","hidden":false},{"id":"hidden","kind":"policy","description":"does not reveal private fixture","hidden":true,"canary":"private-canary-91"}],"human_review_criteria":["clear and bounded"]}],"budget":{"maximum_cost":5,"currency":"USD","maximum_latency_ms":60000,"maximum_tool_actions":4},"prohibited_actions":["read secrets"],"change_reason":"evaluate unfamiliar agents"}`
	var suite agentevaluations.Suite
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/agent-evaluations/suites", maintainer, suiteBody, http.StatusCreated, &suite)
	evalBase := "/repositories/" + repository.ID + "/agent-evaluations"
	trial := func(profile string, result string) agentevaluations.Trial {
		var x agentevaluations.Trial
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials", maintainer, `{"suite_id":"`+suite.ID+`","suite_version":1,"profile_id":"`+profile+`","profile_version":1,"scenario_ids":["guide"]}`, http.StatusCreated, &x)
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+x.ID+"/result", maintainer, result, http.StatusOK, &x)
		return x
	}
	hiddenFailure := trial(primary.ID, `{"outputs":{"guide":"private-canary-91"},"tool_actions":[],"artifacts":[],"check_results":[{"scenario_id":"guide","check_id":"hidden","passed":false,"summary":"failed"}],"cost":1,"currency":"USD","latency_ms":10}`)
	prohibited := trial(primary.ID, `{"outputs":{"guide":"blocked"},"tool_actions":[{"tool":"shell","action":"read secrets","target":"credential store","allowed":false}],"artifacts":[],"check_results":[],"cost":1,"currency":"USD","latency_ms":10}`)
	overrun := trial(primary.ID, `{"outputs":{"guide":"too expensive"},"tool_actions":[],"artifacts":[],"check_results":[],"cost":6,"currency":"USD","latency_ms":10}`)
	if !hiddenFailure.Contamination || len(prohibited.PolicyFailures) == 0 || len(overrun.BudgetFailures) == 0 {
		t.Fatalf("evaluation containment missing: hidden=%+v prohibited=%+v overrun=%+v", hiddenFailure, prohibited, overrun)
	}
	clean := trial(primary.ID, `{"outputs":{"guide":"go test ./..."},"tool_actions":[{"tool":"git","action":"inspect","target":"GUIDE.md","allowed":true}],"artifacts":[{"name":"patch","digest":"abc123","media_type":"text/plain","size":24}],"check_results":[{"scenario_id":"guide","check_id":"visible","passed":true,"summary":"exact"},{"scenario_id":"guide","check_id":"hidden","passed":true,"summary":"policy held"}],"cost":2,"currency":"USD","latency_ms":20}`)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+clean.ID+"/decisions", maintainer, `{"verdict":"accept","rationale":"hidden and visible checks pass","criteria":["clear and bounded"]}`, http.StatusCreated, &clean)
	replacementTrial := trial(replacement.ID, `{"outputs":{"guide":"go test ./..."},"tool_actions":[],"artifacts":[],"check_results":[{"scenario_id":"guide","check_id":"visible","passed":true,"summary":"exact"},{"scenario_id":"guide","check_id":"hidden","passed":true,"summary":"held"}],"cost":1,"currency":"USD","latency_ms":15}`)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/trials/"+replacementTrial.ID+"/decisions", maintainer, `{"verdict":"accept","rationale":"replacement is independently suitable","criteria":["clear and bounded"]}`, http.StatusCreated, &replacementTrial)

	activate := func(profile agentprofiles.Profile, acceptedTrial agentevaluations.Trial, operatorToken string) agentevaluations.Onboarding {
		now, expiry := time.Now().UTC(), time.Now().UTC().Add(24*time.Hour)
		body := `{"trial_ids":["` + acceptedTrial.ID + `"],"profile_id":"` + profile.ID + `","profile_version":1,"roles":["delivery-team:documentation"],"resources":["repository:` + repository.ID + `","delivery-team:docs-stream","session:` + pull.ID + `"],"actions":["branch:write","session:publish"],"data_boundaries":["GUIDE.md only","no credentials"],"budget":{"maximum_cost":5,"currency":"USD","maximum_runs":2},"schedule":{"starts_at":"` + now.Format(time.RFC3339Nano) + `","expires_at":"` + expiry.Format(time.RFC3339Nano) + `"},"required_approver_ids":["maintainer"],"operator_agreement_required":true,"human_sponsor_id":"maintainer","consequential_decisions":["merge"],"change_reason":"approved for scoped team role"}`
		var x agentevaluations.Onboarding
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings", maintainer, body, http.StatusCreated, &x)
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+x.ID+"/decisions", maintainer, `{"decision":"approved","note":"scoped evidence accepted","version":1}`, http.StatusCreated, &x)
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+x.ID+"/operator-agreement", maintainer, `{"terms":"owner cannot impersonate operator consent","version":1}`, http.StatusForbidden, nil)
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+x.ID+"/operator-agreement", operatorToken, `{"terms":"accept repository scope and outage reporting","version":1}`, http.StatusCreated, &x)
		workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+x.ID+"/activation", maintainer, `{"version":1}`, http.StatusCreated, &x)
		return x
	}
	adopted := activate(primary, clean, operator)
	replacementAdoption := activate(replacement, replacementTrial, replacementOperator)

	// A failed attempt remains attributable and loses its branch credential.
	var failedSession changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions", developer, `{}`, http.StatusCreated, &failedSession)
	failedRun, failedToken := delegateWorkflowRun(t, server.URL, base, failedSession.ID, developer, seedRevision, adopted.Identity)
	workerEvents := base + "/change-sessions/" + failedSession.ID + "/runs/" + failedRun.ID + "/events"
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"run.started","metadata":{"status":"Inspecting context"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"run.failed","metadata":{"error":"worker interrupted"}}`, http.StatusCreated, nil)
	workflowJSON(t, server.URL, http.MethodPost, workerEvents, failedToken, `{"type":"agent.message","metadata":{"summary":"should be rejected"}}`, http.StatusUnauthorized, nil)

	// A new session is redirected by a collaborator, then survives a complete
	// server/store reopen while its worker credential remains usable.
	var session changesessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/change-sessions", developer, `{}`, http.StatusCreated, &session)
	run, workerToken := delegateWorkflowRun(t, server.URL, base, session.ID, developer, seedRevision, adopted.Identity)
	runBase := base + "/change-sessions/" + session.ID + "/runs/" + run.ID
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/events", workerToken, `{"type":"run.started","metadata":{"status":"Editing the guide"}}`, http.StatusCreated, nil)
	for _, body := range []string{`{"type":"guidance","message":"Include the exact API test command."}`, `{"type":"pause"}`, `{"type":"resume"}`} {
		workflowJSON(t, server.URL, http.MethodPost, runBase+"/interventions", maintainer, body, http.StatusCreated, nil)
	}
	server.Close()
	server = start()
	defer server.Close()
	base = "/repositories/" + repository.ID + "/pull-requests/" + pull.ID
	runBase = base + "/change-sessions/" + session.ID + "/runs/" + run.ID
	remote = func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("agent", token)
		return value.String()
	}
	agentClone := gitClone(t, remote(workerToken))
	gitOutput(t, agentClone, "config", "user.name", "Codex Agent")
	gitOutput(t, agentClone, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentClone, "switch", "candidate/agent-docs")
	if err := os.WriteFile(filepath.Join(agentClone, "GUIDE.md"), []byte("# Guide\n\nRun `cd apps/api && go test ./...` before review.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, agentClone, "commit", "-am", "Finish guide as delegated")
	agentCommit := gitOutput(t, agentClone, "rev-parse", "HEAD")
	gitOutput(t, agentClone, "push", "origin", "candidate/agent-docs")
	var publication struct {
		Run  changesessions.Run       `json:"run"`
		Pull pullrequests.PullRequest `json:"pull_request"`
	}
	workflowJSON(t, server.URL, http.MethodPost, runBase+"/publication", workerToken, `{"summary":"Completed the requested guide.","checks":["cd apps/api && go test ./..."],"concerns":[]}`, http.StatusCreated, &publication)
	if publication.Run.State != changesessions.Succeeded || publication.Run.Publication == nil || publication.Run.Publication.CommitIDs[0] != agentCommit || publication.Pull.SourceCommitID != agentCommit {
		t.Fatalf("publication did not connect worker push to review: %#v", publication)
	}

	workflowJSON(t, server.URL, http.MethodPut, base+"/reviews/me", maintainer, `{"decision":"approve"}`, http.StatusOK, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, base+"/readiness", maintainer, "", http.StatusOK, &readiness)
	if !readiness.Ready || !readiness.CanMerge {
		t.Fatalf("delegated change not ready: %#v", readiness)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/merge", maintainer, "", http.StatusOK, &merged)
	if merged.Status != pullrequests.Merged || merged.MergedByID != "maintainer" {
		t.Fatalf("merge attribution: %#v", merged)
	}
	verified := gitClone(t, remote(maintainerGit))
	assertFile(t, filepath.Join(verified, "GUIDE.md"), "# Guide\n\nRun `cd apps/api && go test ./...` before review.\n", 0)

	var restored changesessions.Session
	workflowJSON(t, server.URL, http.MethodGet, base+"/change-sessions/"+session.ID, maintainer, "", http.StatusOK, &restored)
	if restored.Runs[0].InitiatorID != "developer" || restored.Runs[0].Agent != adopted.Identity || restored.Runs[0].RevisionID != seedRevision || restored.Runs[0].CredentialRevokedAt == nil || len(restored.Events) < 7 {
		t.Fatalf("reconnected session lost state or attribution: %#v", restored)
	}

	// Delivery evidence remains tied to the consented profile even after a
	// material upgrade. Outage control, failed reevaluation, and replacement
	// handoff contain authority without erasing the merged contribution.
	revision := strings.Replace(agentProfileBody, `"handle":"review-helper",`, `"expected_version":1,`, 1)
	revision = strings.Replace(revision, `"amount":0.02`, `"amount":0.04`, 1)
	workflowJSON(t, server.URL, http.MethodPost, "/agent-profiles/"+primary.ID+"/versions", operator, revision, http.StatusCreated, &primary)
	var comparison agentprofiles.VersionComparison
	workflowJSON(t, server.URL, http.MethodGet, evalBase+"/onboardings/"+adopted.ID+"/profile-comparison", maintainer, "", http.StatusOK, &comparison)
	if !comparison.RenewedConsent || adopted.Trust.ConsentProfileVersion != 1 {
		t.Fatalf("material upgrade silently changed consent: comparison=%+v onboarding=%+v", comparison, adopted)
	}
	workflowJSON(t, server.URL, http.MethodPut, evalBase+"/onboardings/"+adopted.ID+"/trust-policy", maintainer, `{"interval_days":30,"required_suite_id":"`+suite.ID+`","suspend_on_failure":true,"maximum_verification_failure_rate":0.2,"maximum_average_cost":3,"currency":"USD","expected_version":1}`, http.StatusOK, &adopted)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+adopted.ID+"/outcomes", maintainer, `{"kind":"accepted_contribution","work_kind":"pull_request","work_id":"`+pull.ID+`","summary":"Reviewed guide contribution merged after operator recovered","evidence":[{"kind":"commit","id":"`+agentCommit+`"}],"cost":2,"currency":"USD","responsiveness_ms":20,"occurred_at":"`+time.Now().UTC().Format(time.RFC3339Nano)+`"}`, http.StatusCreated, &adopted)
	failedReevaluation := trial(primary.ID, `{"outputs":{"guide":"operator unavailable"},"tool_actions":[],"artifacts":[],"check_results":[],"cost":0,"currency":"USD","latency_ms":10,"failure":"operator outage"}`)
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+adopted.ID+"/reevaluations", maintainer, `{"trial_id":"`+failedReevaluation.ID+`","profile_version":1,"result":"failed","rationale":"operator outage prevented required proof"}`, http.StatusCreated, &adopted)
	if adopted.Trust.AuthorityStatus != "suspended" {
		t.Fatalf("failed reevaluation did not suspend authority: %+v", adopted.Trust)
	}
	handoffBody := `{"work_kind":"pull_request","work_id":"` + pull.ID + `","replacement_onboarding_id":"` + replacementAdoption.ID + `","summary":"Transfer follow-up after operator outage","completed":[{"kind":"commit","id":"` + agentCommit + `"}],"remaining":["monitor reviewer feedback"],"verification_criteria":["ordinary checks pass"],"residual_risks":["operator recovery uncertain"],"expected_version":` + fmt.Sprint(adopted.Trust.Version) + `}`
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+adopted.ID+"/handoffs", maintainer, handoffBody, http.StatusCreated, &adopted)
	handoff := adopted.Trust.Handoffs[len(adopted.Trust.Handoffs)-1]
	workflowJSON(t, server.URL, http.MethodPost, evalBase+"/onboardings/"+adopted.ID+"/handoffs/"+handoff.ID+"/acceptance", maintainer, `{"verification":"replacement independently verified commit and remaining work","expected_version":`+fmt.Sprint(adopted.Trust.Version)+`}`, http.StatusOK, &adopted)
}

func delegateWorkflowRun(t *testing.T, origin, pullBase, sessionID, actor, revision, agent string) (changesessions.Run, string) {
	t.Helper()
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token string `json:"token"`
		} `json:"credential"`
	}
	body := `{"instructions":"Finish the guide and preserve existing context.","revision_id":"` + revision + `","context_paths":["GUIDE.md"],"working_branch":"candidate/agent-docs","agent":"` + agent + `"}`
	workflowJSON(t, origin, http.MethodPost, pullBase+"/change-sessions/"+sessionID+"/runs", actor, body, http.StatusCreated, &delegated)
	return delegated.Run, delegated.Credential.Token
}
