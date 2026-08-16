package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilitypolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestReliabilityStewardshipWorkflow is the black-box boundary for the complete
// released-journey-to-sustained-reliability loop. It keeps telemetry, human and
// agent judgment, ordinary delivery authority, cost, and recovery connected.
func TestReliabilityStewardshipWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	objectives, _ := serviceobjectives.New(t.TempDir())
	policies, _ := reliabilitypolicies.New(t.TempDir())
	investigations, _ := reliabilityinvestigations.New(t.TempDir())
	improvements, _ := reliabilityimprovements.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerProposalsHTTP(mux, plans, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, plans, catalog, credentials, nil, runner, checks, nil)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerServiceObjectivesHTTP(mux, objectives, catalog, credentials)
	registerReliabilityPoliciesHTTP(mux, policies, objectives, catalog, credentials)
	registerReliabilityInvestigationsHTTP(mux, investigations, objectives, catalog, credentials)
	registerReliabilityImprovementsHTTP(mux, improvements, investigations, objectives, plans, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "journey-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	dependencyOwner := issueAccess(t, credentials, "database-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "codex", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "journey-owner", auth.Git, auth.GitRead, auth.GitWrite)
	agentGit := issueAccess(t, credentials, "codex", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"dependable-reviews","visibility":"private"}`, 201, &repository)
	for _, actor := range []string{"database-owner", "codex"} {
		if _, err := catalog.AddCollaborator("journey-owner", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	remote := func(token string) string {
		u, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	work := gitClone(t, remote(ownerGit))
	gitOutput(t, work, "config", "user.name", "Journey Owner")
	gitOutput(t, work, "config", "user.email", "owner@example.test")
	writeWorkflowFile(t, work, "service/review.sh", "#!/bin/sh\nprintf 'review available with unbounded retries\\n'\n")
	writeWorkflowFile(t, work, ".komodo/checks.json", `{"version":1,"checks":[{"name":"reliability/review-journey","command":"grep -q 'retry_budget=3' service/review.sh"}]}`)
	writeWorkflowFile(t, work, ".komodo/releases.json", `{"version":1,"builds":[{"name":"review-service","command":"mkdir -p dist; cp service/review.sh dist/review.sh","artifacts":["dist/review.sh"]}]}`)
	gitOutput(t, work, "add", ".")
	gitOutput(t, work, "commit", "-m", "Release review journey")
	degradedRevision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "main")
	base := "/repositories/" + string(repository.ID)
	var released releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+degradedRevision+`","notes":"Released contributor review journey"}`, 201, &released)

	objectiveBody := `{"title":"Review journey availability","description":"Contributors can open and review a released change","scopes":[{"kind":"release","resource_id":"` + released.ID + `","name":"v1 review journey"},{"kind":"environment","resource_id":"production","name":"production"}],"indicators":[{"id":"availability","name":"Successful review requests","description":"Review pages complete","signal":"review.success_ratio","signal_status":"available","calculation":"ratio","unit":"percent","good_event":"status below 500","total_event":"all review requests"}],"measurement_windows":[{"id":"rolling-28d","kind":"rolling","duration":"28d"}],"targets":[{"indicator_id":"availability","window_id":"rolling-28d","comparator":"gte","value":99.9,"error_budget_percent":0.1}],"journeys":[{"id":"review","name":"Review a change","behavior":"A contributor opens and reviews a released change","owner_ids":["journey-owner"]}],"dependencies":[{"id":"database","name":"Review database","kind":"service","required":true,"owner_ids":["database-owner"]}],"severity_thresholds":[{"level":"critical","budget_consumed_percent":100,"response":"contain affected rollout","owner_ids":["journey-owner","database-owner"]}],"owner_ids":["journey-owner"],"commitment_links":[{"kind":"release","resource_id":"` + released.ID + `","label":"Released journey","status":"linked"}],"exception_policy":"Both affected owners must approve a bounded expiry and follow-up","change_reason":"Make the released journey dependable"}`
	var objective serviceobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base+"/service-objectives", owner, objectiveBody, 201, &objective)
	// An unowned exception is rejected rather than becoming silent permission.
	badException := strings.TrimSuffix(objectiveBody, "}") + `,"exceptions":[{"id":"ship-anyway","reason":"ignore burn","approved_by":"","owner_id":"","expires_at":"` + time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano) + `"}],"expected_version":1}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/service-objectives/"+objective.ID+"/versions", owner, badException, 422, nil)
	var mapped serviceobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, base+"/service-objectives/"+objective.ID+"/signal-mappings", owner, `{"objective_version":1,"indicator_id":"availability","window_id":"rolling-28d","instrumentation_revision":"`+degradedRevision+`","change_reason":"Connect sanitized telemetry to the release","sources":[{"id":"success-ratio","kind":"metric","name":"sanitized success ratio","sanitized_fields":["status_class"]},{"id":"release","kind":"release","name":"review release","sanitized_fields":["release_id","commit"]},{"id":"database","kind":"dependent_service","name":"database health","sanitized_fields":["health"]}]}`, 201, &mapped)
	mapping := mapped.Mappings[0]
	start := time.Now().UTC().Add(-28 * 24 * time.Hour)
	workflowJSON(t, server.URL, http.MethodPost, base+"/service-objectives/"+objective.ID+"/attainment", owner, `{"mapping_id":"`+mapping.ID+`","mapping_version":1,"window_start":"`+start.Format(time.RFC3339Nano)+`","window_end":"`+time.Now().UTC().Format(time.RFC3339Nano)+`","value":99.62,"error_budget_consumed_percent":138,"uncertainty":"one noisy low-traffic shard may overstate burn","comparable_to_previous":true,"sanitized":true,"audience":"repository","evidence":[{"kind":"metric","resource_id":"review-success","revision":"sample-burn","label":"bounded production aggregate"},{"kind":"deployment","resource_id":"deploy-v1","revision":"`+degradedRevision+`","label":"degraded rollout"}]}`, 201, &mapped)
	if mapped.Attainment[0].Status != "missed" || mapped.Attainment[0].ErrorBudgetConsumedPercent != 138 {
		t.Fatalf("burn was not retained: %#v", mapped.Attainment)
	}

	var policy reliabilitypolicies.Policy
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-delivery-policies", owner, `{"name":"Review production budget","objective_id":"`+objective.ID+`","objective_version":1,"branches":["main"],"services":["review"],"environments":["production"],"journeys":["review"],"required_owner_ids":["journey-owner","database-owner"],"rules":[{"condition":"budget_exhausted","action":"pause"},{"condition":"missing_evidence","action":"pause"},{"condition":"dependency_failure","action":"rollback"}],"change_reason":"Contain harm before more rollout"}`, 201, &policy)
	context := `{"kind":"deployment","resource_id":"deploy-v1","revision":"` + degradedRevision + `","branch":"main","service":"review","environment":"production","journeys":["review"]}`
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-delivery-policies/"+policy.ID+"/impacts", owner, `{"context":`+context+`,"phase":"observed","observed_budget_consumed_percent":138,"regression":true,"evidence_status":"current","dependency_status":"unknown","summary":"Review failures burned the budget after deployment; dependency evidence is missing","evidence":["attainment:`+mapped.Attainment[0].ID+`","deployment:deploy-v1"]}`, 201, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-delivery-policies/"+policy.ID+"/acknowledgements", owner, `{"context":`+context+`,"decision":"acknowledged","rationale":"Pause rollout while collaborators investigate"}`, 201, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-delivery-policies/"+policy.ID+"/acknowledgements", dependencyOwner, `{"context":`+context+`,"decision":"rejected","rationale":"Missing database evidence cannot justify an exception"}`, 201, nil)
	var assessment reliabilitypolicies.Assessment
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-delivery-policies/assessment", owner, `{"context":`+context+`,"active_exceptions":[]}`, 200, &assessment)
	if assessment.Ready || !stringSliceContains(assessment.AvailableNextActions, "pause") {
		t.Fatalf("degraded rollout was not contained: %#v", assessment)
	}

	var investigation reliabilityinvestigations.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations", agent, `{"objective_id":"`+objective.ID+`","objective_version":1,"revision":"`+degradedRevision+`","trigger":{"kind":"deployment","resource_id":"deploy-v1","revision":"`+degradedRevision+`"},"title":"Review availability burn","question":"Did retry fanout or the database cause the released journey failure?","journey_ids":["review"],"evidence":[{"kind":"metric","resource_id":"review-success","revision":"baseline","window":"prior 28d","summary":"99.96 percent","audience":"repository","baseline":true},{"kind":"metric","resource_id":"review-success","revision":"sample-burn","window":"current 28d","summary":"99.62 percent; one shard is noisy","audience":"repository","baseline":false,"uncertainty":"low traffic shard"},{"kind":"code","resource_id":"service/review.sh","revision":"`+degradedRevision+`","summary":"unbounded retries in released code","audience":"repository","baseline":false}]}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations/"+investigation.ID+"/participants", owner, `{"user_id":"database-owner"}`, 200, &investigation)
	noisyEvidence, codeEvidence := investigation.Evidence[1].ID, investigation.Evidence[2].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations/"+investigation.ID+"/entries", agent, `{"kind":"hypothesis","body":"Retry fanout amplifies database timeouts, but the noisy shard cannot establish causality.","uncertainty":"Dependency trace is not yet available.","citations":[{"evidence_id":"`+noisyEvidence+`"},{"evidence_id":"`+codeEvidence+`"}]}`, 201, &investigation)
	hypothesis := investigation.Entries[0].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations/"+investigation.ID+"/input-requests", agent, `{"owner_id":"database-owner","owner_kind":"dependency","question":"Was database health degraded during the exact deployment window?","evidence_needed":["sanitized timeout ratio"]}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations/"+investigation.ID+"/entries", dependencyOwner, `{"kind":"challenge","body":"Database saturation stayed healthy; distinguish primary failures from retry amplification.","challenges":"`+hypothesis+`","citations":[{"kind":"dependency","resource_id":"database-health","revision":"deploy-v1"}]}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-investigations/"+investigation.ID+"/entries", agent, `{"kind":"conclusion","body":"Revision-exact retry fanout amplified primary failures while database health remained within its budget.","verdict":"supported","citations":[{"evidence_id":"`+codeEvidence+`"},{"kind":"dependency","resource_id":"database-health","revision":"deploy-v1"}]}`, 201, &investigation)
	conclusion := investigation.Entries[len(investigation.Entries)-1]
	if conclusion.ActorID != "codex" || !stringSliceContains(investigation.Blockers, "uncertain_evidence") || !stringSliceContains(investigation.Blockers, "dependency_input_pending") {
		t.Fatalf("diagnostic correction trail incomplete: %#v", investigation)
	}

	var made struct {
		Improvement reliabilityimprovements.Improvement `json:"improvement"`
		Proposal    proposals.Proposal                  `json:"proposal"`
		Tasks       []string                            `json:"tasks"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-improvements", owner, `{"objective_id":"`+objective.ID+`","objective_version":1,"source":{"kind":"finding","resource_id":"`+investigation.ID+`","entry_id":"`+conclusion.ID+`"},"base_revision":"`+degradedRevision+`","title":"Bound review retry fanout","affected_revisions":["`+degradedRevision+`","deploy-v1"],"journey_ids":["review"],"evidence_ids":["`+codeEvidence+`","`+noisyEvidence+`"],"dependency_context":["database-owner evidence: healthy during deploy-v1"],"acceptance_criteria":["review check passes","availability is at least 99.9 percent","error budget is restored"],"baseline":{"indicator":"availability","window":"prior 28d","value":99.96,"unit":"percent","evidence_id":"baseline"},"tasks":[{"title":"Implement bounded retry budget","owner_kind":"agent","owner_id":"codex","risk":"high","acceptance_criteria":["ordinary reliability check passes"]},{"title":"Review user and dependency impact","owner_kind":"human","owner_id":"journey-owner","depends_on":[1],"acceptance_criteria":["exact revision is reviewed"]}]}`, 201, &made)
	plan, _ := plans.GetPlan(string(repository.ID), made.Proposal.ID)
	if plan.Tasks[0].OwnerKind != "agent" || plan.Tasks[1].DependsOn[0] != plan.Tasks[0].ID {
		t.Fatalf("human-agent authority boundary lost: %#v", plan.Tasks)
	}
	var assigned proposals.Task
	workflowJSON(t, server.URL, http.MethodPut, base+"/proposals/"+made.Proposal.ID+"/plan/tasks/"+made.Tasks[0]+"/assignment", owner, `{"kind":"agent","assignee_id":"codex","mandate":"Change only the review retry budget and return through ordinary checks and owner review.","repository_id":"`+string(repository.ID)+`","base_revision":"`+degradedRevision+`"}`, 200, &assigned)
	if assigned.Assignment == nil || assigned.Assignment.CredentialIssued || len(assigned.Assignment.Permissions) != 2 {
		t.Fatalf("bounded assignment authority was not retained: %#v", assigned.Assignment)
	}

	agentWork := gitClone(t, remote(agentGit))
	gitOutput(t, agentWork, "config", "user.name", "Reliability Agent")
	gitOutput(t, agentWork, "config", "user.email", "agent@example.test")
	gitOutput(t, agentWork, "switch", "-c", "repair/review-retries")
	writeWorkflowFile(t, agentWork, "service/review.sh", "#!/bin/sh\nprintf 'retry_budget=unbounded cost_usd=0.18\\n'\n")
	gitOutput(t, agentWork, "add", "service/review.sh")
	gitOutput(t, agentWork, "commit", "-m", "Attempt to bound review retries")
	firstRepair := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push", "-u", "origin", "repair/review-retries")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", agent, `{"title":"Bound review retry fanout","body":"Agent task `+made.Tasks[0]+`; investigation `+investigation.ID+`; compute cost USD 0.18. No merge or deployment authority granted.","source_branch":"repair/review-retries","target_branch":"main","proposal_id":"`+made.Proposal.ID+`","task_id":"`+made.Tasks[0]+`"}`, 201, &pull)
	pullBase := base + "/pull-requests/" + pull.ID
	waitForWorkflowCheck(t, server.URL, pullBase, agent, firstRepair, checkruns.Failed)
	writeWorkflowFile(t, agentWork, "service/review.sh", "#!/bin/sh\nprintf 'retry_budget=3 cost_usd=0.26 handoff=journey-owner\\n'\n")
	gitOutput(t, agentWork, "add", "service/review.sh")
	gitOutput(t, agentWork, "commit", "-m", "Enforce bounded review retry budget")
	repairRevision := gitOutput(t, agentWork, "rev-parse", "HEAD")
	gitOutput(t, agentWork, "push")
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/synchronize", agent, `{}`, 200, &pull)
	waitForWorkflowCheck(t, server.URL, pullBase, agent, repairRevision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, pullBase+"/reviews/me", owner, `{"decision":"approve"}`, 200, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/reliability-improvements/"+made.Improvement.ID+"/delivery-links", owner, `{"kind":"pull_request","resource_id":"`+pull.ID+`","revision":"`+repairRevision+`","task_id":"`+made.Tasks[0]+`","summary":"Agent repair cost USD 0.26; failed first check retained; owner reviewed current revision"}`, 201, nil)
	workflowJSON(t, server.URL, http.MethodPost, pullBase+"/merge", owner, `{}`, 200, &pull)
	var repairedRelease releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.0.1","commit_id":"`+pull.MergeCommitID+`","notes":"Reviewed reliability repair"}`, 201, &repairedRelease)
	improvementBase := base + "/reliability-improvements/" + made.Improvement.ID
	workflowJSON(t, server.URL, http.MethodPost, improvementBase+"/delivery-links", owner, `{"kind":"release","resource_id":"`+repairedRelease.ID+`","revision":"`+pull.MergeCommitID+`","summary":"Ordinary reviewed repair release"}`, 201, nil)
	var current reliabilityimprovements.Improvement
	workflowJSON(t, server.URL, http.MethodPost, improvementBase+"/rollouts", owner, `{"deployment_id":"deploy-v1.0.1-canary","release_id":"`+repairedRelease.ID+`","revision":"`+pull.MergeCommitID+`","environment":"production","stage":"canary","rationale":"Contain the first repair because the staged measure failed.","measurements":[{"indicator":"availability","window":"canary 30m","value":99.72,"unit":"percent","evidence_id":"metric-canary-failed","passed":false}]}`, 201, &current)
	if current.State != "contained" || current.Rollouts[0].RequiredAction != "contain" {
		t.Fatalf("failed first repair escaped containment: %#v", current)
	}
	workflowJSON(t, server.URL, http.MethodPost, improvementBase+"/rollouts", owner, `{"deployment_id":"deploy-v1.0.1-progressive","release_id":"`+repairedRelease.ID+`","revision":"`+pull.MergeCommitID+`","environment":"production","stage":"complete","rationale":"Comparable sanitized evidence verifies recovery.","measurements":[{"indicator":"availability","window":"current 28d","value":99.97,"unit":"percent","evidence_id":"metric-recovered","passed":true}]}`, 201, &current)
	if current.State != "verified" || current.BudgetState != "restored" || !current.PriorImpactRetained || len(current.Rollouts) != 2 {
		t.Fatalf("sustained recovery trail incomplete: %#v", current)
	}
	var visible struct {
		Items []reliabilityimprovements.Improvement `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base+"/reliability-improvements", dependencyOwner, "", 200, &visible)
	if len(visible.Items) != 1 || visible.Items[0].BudgetState != "restored" {
		t.Fatalf("collaborators cannot inspect the retained recovery: %#v", visible.Items)
	}
}
