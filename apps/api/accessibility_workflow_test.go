package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestAccessibilityAssuranceWorkflow is the black-box boundary for the complete
// released-barrier-to-retained-regression loop. Lived evidence stays consent
// projected while ordinary Git, review, checks, preview, merge, and release
// contracts remain authoritative.
func TestAccessibilityAssuranceWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	previewStore, _ := previews.New(t.TempDir())
	barriers, _ := accessibilitybarriers.New(t.TempDir())
	commitments, _ := accessibilitycommitments.New(t.TempDir())
	assessments, _ := accessibilityassessments.New(t.TempDir())
	policies, _ := accessibilitypolicies.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, catalog)
	previewRunner := previews.NewRunner(previewStore, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, checkRunner, checks, previewStore, policies, assessments)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, catalog, credentials, nil, nil)
	registerPreviewsHTTP(mux, previewStore, previewRunner, pulls, catalog, credentials, previewSources{})
	registerReleasesHTTP(mux, releaseStore, checks, checkRunner, pulls, catalog, credentials)
	registerAccessibilityCommitmentsHTTP(mux, commitments, catalog, credentials)
	registerAccessibilityBarriersHTTP(mux, barriers, catalog, credentials, accessibilityBarrierSources{releases: releaseStore, previews: previewStore, repositories: catalog})
	registerAccessibilityAssessmentsHTTP(mux, assessments, catalog, credentials, accessibilityAssessmentSources{pulls: pulls, runs: checks, previews: previewStore, barriers: barriers, repositories: catalog, commitments: commitments, plans: plans})
	registerAccessibilityPoliciesHTTP(mux, policies, catalog, credentials, commitments, previewStore, assessments, checks, pulls)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	reporter := issueAccess(t, credentials, "reporter", auth.API, auth.RepositoryRead)
	specialist := issueAccess(t, credentials, "specialist", auth.API, auth.RepositoryRead)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	viewer := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"inclusive-review","visibility":"public"}`, 201, &repository)
	if _, err := catalog.AddCollaborator("maintainer", repository.ID, "codex"); err != nil {
		t.Fatal(err)
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}

	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Maintainer")
	gitOutput(t, work, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, work, "site/review.html", "<main><button id=comment>Comment</button><div role=dialog hidden></div></main>\n")
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"review-site","command":"mkdir -p dist; cp site/review.html dist/review.html","artifacts":["dist/review.html"]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Publish keyboard review journey")
	baseline := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	var affected releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+baseline+`","notes":"Released review journey."}`, 201, &affected)

	expires := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339Nano)
	commitmentBody := `{"title":"Keyboard-accessible code review","scopes":[{"kind":"journey","resource_id":"review-comment","name":"Comment on a changed line"}],"standards":[{"id":"wcag","name":"WCAG","version":"2.2","level":"AA"}],"assistive_technologies":[{"id":"keyboard-sr","name":"Keyboard and screen reader","platform":"web"}],"target_audiences":["keyboard and screen reader users"],"required_scenarios":[{"id":"review-comment","name":"Open and close the comment dialog","scope_ids":["journey:review-comment"],"standard_ids":["wcag"],"assistive_technology_ids":["keyboard-sr"]}],"severity_policy":[{"severity":"high","definition":"Focus is lost","review_effect":"block_merge","resolution_target_days":7}],"owner_ids":["maintainer"],"exceptions":[{"id":"temporary-focus-exception","scenario_ids":["review-comment"],"reason":"Repair is being validated","approved_by":"maintainer","expires_at":"` + expires + `"}],"links":[{"kind":"release_policy","resource_id":"stable"}],"change_reason":"Retain the lived journey as a release contract"}`
	var commitment accessibilitycommitments.Commitment
	commitmentBase := "/repositories/" + string(repository.ID) + "/accessibility-commitments"
	workflowJSON(t, server.URL, http.MethodPost, commitmentBase, owner, commitmentBody, 201, &commitment)
	workflowJSON(t, server.URL, http.MethodGet, commitmentBase+"/"+commitment.ID, owner, "", 200, &commitment)
	if len(commitment.Blockers) == 0 || !strings.Contains(commitment.Blockers[len(commitment.Blockers)-1].Kind, "exception") {
		t.Fatalf("expiring exception is not explicit: %#v", commitment.Blockers)
	}

	// The reporter shares only needs and a restricted, redacted attachment. A
	// repository reader can follow the impact but cannot retrieve its body.
	barrierBase := "/repositories/" + string(repository.ID) + "/accessibility-barriers"
	barrierBody := `{"context":{"kind":"release","resource_id":"` + affected.ID + `","revision":"` + baseline + `"},"access_needs":"Keep keyboard focus announced after opening a comment","expected_outcome":"Close the dialog and return to the changed line","interaction_steps":["Focus Comment","Press Enter","Press Escape"],"environment":{"browser":"Firefox","device_class":"desktop","assistive_technology":"NVDA","sensitive_device_data":"private speech profile"},"identity_visibility":"maintainers","device_data_visibility":"maintainers","evidence":[{"kind":"speech_output","name":"nvda.txt","media_type":"text/plain","content":"redacted: focus moved to document","visibility":"maintainers","redacted":true}]}`
	var barrier accessibilitybarriers.Barrier
	workflowJSON(t, server.URL, http.MethodPost, barrierBase, reporter, barrierBody, 201, &barrier)
	var projected accessibilitybarriers.Barrier
	workflowJSON(t, server.URL, http.MethodGet, barrierBase+"/"+barrier.ID, viewer, "", 200, &projected)
	if projected.ReporterID != "" || projected.Evidence[0].Content != "" || projected.Environment.SensitiveDeviceData != "" {
		t.Fatalf("consent boundary leaked: %#v", projected)
	}

	// A bounded preview lets the maintainer reproduce without requesting the
	// reporter's machine, credentials, diagnosis, or inaccessible attachment.
	reproduction, _ := previewStore.Create(previews.Preview{RepositoryID: string(repository.ID), PullRequestID: "reproduction", Revision: baseline, State: "ready", Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	attemptBody := `{"execution_kind":"preview","execution_id":"` + reproduction.ID + `","revision":"` + baseline + `","environment":{"browser":"Firefox","device_class":"clean desktop","assistive_technology":"NVDA"},"result":"reproducible","notes":"Accessibility tree shows focus falls back to the document; used a clean environment rather than the restricted attachment.","evidence":[{"kind":"accessibility_tree","name":"clean-tree.txt","media_type":"text/plain","content":"dialog closed; active=document","visibility":"audience","redacted":true}]}`
	workflowJSON(t, server.URL, http.MethodPost, barrierBase+"/"+barrier.ID+"/attempts", owner, attemptBody, 201, &barrier)

	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/proposals", owner, `{"title":"Restore comment-dialog focus","body":"Repair the confirmed released barrier and retain its scenario."}`, 201, &proposal)
	// A baseline pull provides the revision-exact assessment from which repair
	// work is governed; it is never merged.
	gitOutput(t, work, "switch", "-c", "assessment/baseline")
	gitOutput(t, work, "push", "-u", "origin", "assessment/baseline")
	var assessmentPull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", owner, `{"title":"Assess released focus barrier","body":"Revision-exact assessment only.","source_branch":"assessment/baseline","target_branch":"main","proposal_id":"`+proposal.ID+`"}`, 201, &assessmentPull)
	assessmentBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + assessmentPull.ID + "/accessibility-assessments"
	var assessment accessibilityassessments.Assessment
	assessmentInput := `{"revision":"` + baseline + `","commitment_id":"` + commitment.ID + `","commitment_version":1,"scenarios":[{"id":"review-comment","name":"Open and close the comment dialog","journey":"Comment on a changed line","affected_audiences":["keyboard and screen reader users"],"required_evaluations":["focus","keyboard"],"source_locations":[{"path":"site/review.html"}]}]}`
	workflowJSON(t, server.URL, http.MethodPost, assessmentBase, owner, assessmentInput, 201, &assessment)
	finding := func(summary, result string) {
		workflowJSON(t, server.URL, http.MethodPost, assessmentBase+"/"+assessment.ID+"/findings", specialist, `{"scenario_id":"review-comment","evaluation":"focus","result":"`+result+`","severity":"high","affected_audiences":["keyboard and screen reader users"],"source_locations":[{"path":"site/review.html"}],"summary":"`+summary+`","requires_human_evaluation":true,"citation":{"kind":"reproduction","resource_id":"`+barrier.ID+`","evidence_ids":["`+barrier.Attempts[0].Evidence[0].ID+`"]}}`, 201, &assessment)
	}
	finding("Focus is lost after Escape", "barrier")
	finding("The button has no accessible name", "barrier")
	workflowJSON(t, server.URL, http.MethodPost, assessmentBase+"/"+assessment.ID+"/findings/"+assessment.Findings[1].ID+"/decisions", owner, `{"outcome":"false_positive","rationale":"The retained source and clean accessibility tree both expose the Comment name."}`, 201, &assessment)
	workflowJSON(t, server.URL, http.MethodPost, assessmentBase+"/"+assessment.ID+"/findings/"+assessment.Findings[0].ID+"/decisions", owner, `{"outcome":"confirmed","rationale":"The bounded reproduction confirms focus loss without private device context."}`, 201, &assessment)

	var repair struct {
		Repair accessibilityassessments.Repair `json:"repair"`
		Task   proposals.Task                  `json:"task"`
	}
	repairRoot := assessmentBase + "/" + assessment.ID + "/findings/" + assessment.Findings[0].ID + "/repairs"
	workflowJSON(t, server.URL, http.MethodPost, repairRoot, owner, `{"kind":"task","proposal_id":"`+proposal.ID+`","title":"Return focus to the changed line","owner_kind":"agent","owner_id":"codex","commitment_id":"`+commitment.ID+`","commitment_version":1,"acceptance_criteria":["Escape returns focus to Comment","NVDA announces Comment"],"evidence_ids":["`+barrier.Attempts[0].Evidence[0].ID+`"],"component_guidance":["Use the shared dialog return-focus target"]}`, 201, &repair)
	if repair.Task.ReasoningContext == nil || repair.Task.ReasoningContext.Kind != "accessibility_repair" {
		t.Fatalf("agent lost evidence-bound reasoning: %#v", repair.Task)
	}

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Codex Agent")
	gitOutput(t, agentWork, "config", "user.email", "codex@agents.local")
	gitOutput(t, agentWork, "switch", "-c", "repair/focus")
	writeWorkflowFile(t, agentWork, "site/review.html", "<main><button id=comment>Comment</button><div role=dialog aria-modal=true hidden data-return-focus=comment></div></main>\n")
	gitOutput(t, agentWork, "add", ".")
	gitOutput(t, agentWork, "commit", "-m", "Return focus after closing comment dialog")
	candidate := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "repair/focus")
	var repairPull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", agent, `{"title":"Restore comment-dialog focus","body":"Agent repair from retained accessibility reasoning.","source_branch":"repair/focus","target_branch":"main","proposal_id":"`+proposal.ID+`","task_id":"`+repair.Task.ID+`"}`, 201, &repairPull)
	candidatePreview, _ := previewStore.Create(previews.Preview{RepositoryID: string(repository.ID), PullRequestID: repairPull.ID, Revision: candidate, State: "ready", Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	workflowJSON(t, server.URL, http.MethodPost, repairRoot+"/"+repair.Repair.ID+"/delivery", owner, `{"pull_request_id":"`+repairPull.ID+`","revision":"`+candidate+`","preview_id":"`+candidatePreview.ID+`","design_changes":["Restore the invoking target"],"code_changes":["Bind dialog return focus to Comment"],"interaction_tradeoffs":["Wait for close before restoring focus"],"content_tradeoffs":["Keep the existing accessible name"]}`, 201, nil)

	// Current automation and assistive-technology judgment cover the repaired
	// candidate. The repository policy makes both independently necessary.
	currentBase := "/repositories/" + string(repository.ID) + "/pull-requests/" + repairPull.ID + "/accessibility-assessments"
	workflowJSON(t, server.URL, http.MethodPost, currentBase, owner, strings.ReplaceAll(assessmentInput, baseline, candidate), 201, &assessment)
	run, _ := checks.Create(string(repository.ID), repairPull.ID, candidate, checkruns.Definition{Name: "accessibility/review-focus", Accessibility: &checkruns.AccessibilitySpec{ScenarioIDs: []string{"review-comment"}, Evaluations: []string{"keyboard"}, Inputs: []string{"site/review.html"}, AffectedAudiences: []string{"keyboard and screen reader users"}, RequiresHumanEvaluation: []string{"focus"}}})
	run, _ = checks.Start(run.ID)
	run, _ = checks.Complete(run.ID, 0, false, "")
	workflowJSON(t, server.URL, http.MethodPost, currentBase+"/"+assessment.ID+"/automation", agent, `{"run_id":"`+run.ID+`"}`, 201, &assessment)
	workflowJSON(t, server.URL, http.MethodPost, currentBase+"/"+assessment.ID+"/findings", specialist, `{"scenario_id":"review-comment","evaluation":"focus","result":"passed","severity":"none","affected_audiences":["keyboard and screen reader users"],"source_locations":[{"path":"site/review.html"}],"summary":"Escape returns focus and NVDA announces Comment","requires_human_evaluation":true,"citation":{"kind":"preview","resource_id":"`+candidatePreview.ID+`"}}`, 201, &assessment)
	var policy accessibilitypolicies.Policy
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/accessibility-delivery-policies", owner, `{"name":"Review journey gate","commitment_id":"`+commitment.ID+`","commitment_version":1,"target_branches":["main"],"paths":["site/**"],"required_checks":["accessibility/review-focus"],"scenarios":[{"scenario_id":"review-comment","required_evaluations":["keyboard","focus"],"required_roles":["feedback"]}]}`, 201, &policy)

	oldPreview, _ := previewStore.Create(previews.Preview{RepositoryID: string(repository.ID), PullRequestID: repairPull.ID, Revision: baseline, State: "ready", Definition: previews.Definition{Resources: previews.Resources{LifetimeMinutes: 60}}})
	invite := func(p previews.Preview) {
		workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/previews/"+p.ID+"/invitations", owner, `{"user_id":"reporter","role":"feedback","source_kind":"user","expires_at":"`+time.Now().UTC().Add(30*time.Minute).Format(time.RFC3339Nano)+`"}`, 201, nil)
	}
	ack := func(p previews.Preview, rationale string) {
		workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/accessibility-acknowledgements", reporter, `{"policy_id":"`+policy.ID+`","preview_id":"`+p.ID+`","scenario_id":"review-comment","role":"feedback","decision":"confirmed","rationale":"`+rationale+`"}`, 201, nil)
	}
	invite(oldPreview)
	ack(oldPreview, "The old candidate still loses focus")
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/readiness", owner, "", 200, &readiness)
	if readiness.Ready {
		t.Fatal("stale reporter acceptance satisfied the exact candidate")
	}
	invite(candidatePreview)
	ack(candidatePreview, "In the bounded candidate preview, focus returns and NVDA announces Comment")
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/readiness", owner, "", 200, &readiness)
	if !readiness.Ready || readiness.Accessibility == nil || !readiness.Accessibility.Ready {
		t.Fatalf("accessible candidate not ready: %#v", readiness)
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+repairPull.ID+"/merge", owner, `{}`, 200, &repairPull)
	var fixed releases.Release
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", owner, `{"version":"v1.1.0","commit_id":"`+repairPull.MergeCommitID+`","prior_release_id":"`+affected.ID+`","notes":"Reporter-confirmed accessible review journey."}`, 201, &fixed)
	workflowJSON(t, server.URL, http.MethodPost, commitmentBase+"/"+commitment.ID+"/coverage", owner, `{"version":1,"scenario_id":"review-comment","assistive_technology_id":"keyboard-sr","status":"passed","revision":"`+repairPull.MergeCommitID+`","evidence":"release `+fixed.ID+`; check `+run.ID+`; reporter preview `+candidatePreview.ID+`","notes":"Retained regression coverage from the repaired released barrier."}`, 201, &commitment)
	if len(commitment.Coverage) != 1 || commitment.Coverage[0].Revision != repairPull.MergeCommitID || fixed.PullRequests[0].AuthorID != "codex" {
		t.Fatalf("delivery trail or regression coverage lost: release=%#v commitment=%#v", fixed, commitment)
	}
}
