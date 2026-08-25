package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributionopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningoutcomes"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

// TestDeveloperLearningWorkflow is the black-box boundary for the complete
// project-learning-to-trusted-contributor loop. Learning actions cross public
// HTTP surfaces and the contribution crosses stock Git, review, checks, and
// merge without ever making the learner an upstream collaborator.
func TestDeveloperLearningWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pathways, _ := learningpathways.New(t.TempDir())
	exercises, _ := learningexercises.New(t.TempDir())
	assessments, _ := learningassessments.New(t.TempDir())
	outcomes, _ := learningoutcomes.New(t.TempDir())
	opportunities, _ := contributionopportunities.New(t.TempDir())
	contributorGuidance, _ := contributorpathways.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	orgStore, _ := organizations.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	workspaceRunner := workspaces.NewRunner(workspaceStore, repos)
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	checkRunner := checkruns.NewRunner(checks, repos)

	repo, _ := repos.Create("maintainer", repositories.Metadata{Name: "learnable-parser", Visibility: repositories.Public})
	for _, actor := range []string{"mentor", "reviewer"} {
		_, _ = repos.AddCollaborator("maintainer", repo.ID, actor)
	}
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	learner := issueAccess(t, credentials, "learner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mentor := issueAccess(t, credentials, "mentor", auth.API, auth.RepositoryRead)
	reviewer := issueAccess(t, credentials, "reviewer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	maintainerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	learnerGit := issueAccess(t, credentials, "learner", auth.Git, auth.GitRead, auth.GitWrite)

	mux := http.NewServeMux()
	registerGitHTTP(mux, repos, credentials)
	registerRepositoriesHTTP(mux, repos, credentials)
	registerLearningPathwaysHTTP(mux, pathways, repos, credentials, learningPathwaySources{})
	registerLearningExercisesHTTP(mux, exercises, pathways, repos, credentials, nil)
	registerLearningAssessmentsHTTP(mux, assessments, pathways, repos, credentials)
	registerLearningOutcomesHTTP(mux, outcomes, repos, credentials)
	registerContributionOpportunitiesHTTP(mux, opportunities, repos, credentials, issueStore, proposalStore, orgStore, contributorGuidance, workspaceStore, workspaceRunner, pathways, exercises, assessments)
	registerPullRequestsHTTP(mux, pulls, nil, repos, credentials, nil, checkRunner, checks)
	registerCheckRunsHTTP(mux, checks, checkRunner, pulls, repos, credentials, nil, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID)
	remote := func(id, token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + id)
		value.User = url.UserPassword("git", token)
		return value.String()
	}

	ownerClone := gitClone(t, remote(string(repo.ID), maintainerGit))
	configureWorkflowGit(t, ownerClone, "Maintainer", "maintainer@example.test")
	writeWorkflowFile(t, ownerClone, "parser.go", "package parser\n\nfunc Parse(v string) bool { return v != \"\" }\n")
	writeWorkflowFile(t, ownerClone, "parser_test.go", "package parser\n\nimport \"testing\"\nfunc TestParse(t *testing.T) { if !Parse(\"ok\") { t.Fatal(\"parse\") } }\n")
	writeWorkflowFile(t, ownerClone, ".komodo/checks.json", `{"version":1,"checks":[{"name":"parser","command":"grep -q strings parser.go && grep -q 'Parse(\"   \"' parser_test.go"}]}`)
	writeWorkflowFile(t, ownerClone, "go.mod", "module example.test/parser\n\ngo 1.25\n")
	gitOutput(t, ownerClone, "add", ".")
	gitOutput(t, ownerClone, "commit", "-m", "Create learnable parser")
	gitOutput(t, ownerClone, "push", "-u", "origin", "main")
	revision := strings.TrimSpace(gitOutput(t, ownerClone, "rev-parse", "HEAD"))
	workflowJSON(t, server.URL, http.MethodPut, root+"/required-checks", maintainer, `{"branch":"main","checks":["parser"]}`, http.StatusOK, nil)

	// The first publication honestly retains an unsupported environment,
	// inaccessible reading, missing prerequisite, and departed mentor.
	pathwayInput := map[string]any{
		"expected_version": 0, "role": "Parser contributor", "outcome": "Diagnose and repair parser behavior",
		"prerequisites": []string{"Go fundamentals", "missing: repository test conventions"}, "objectives": []string{"trace parsing", "repair an edge case"},
		"supported_revisions": []string{revision}, "mentor_ids": []string{"mentor", "departed-mentor"}, "expected_effort_minutes": 60,
		"learner_environments": []map[string]any{{"name": "browser", "requirement": "Go 1.25", "supported": true}, {"name": "legacy-shell", "requirement": "unsupported toolchain", "supported": false}},
		"completion_evidence":  []string{"reproducible exercise", "practical assessment"}, "change_reason": "publish role pathway",
		"modules": []map[string]any{{"id": "parser", "title": "Repair the parser", "why_it_matters": "Every request enters here", "objectives": []string{"locate and test edge behavior"}, "expected_effort_minutes": 60,
			"exercises": []map[string]any{{"title": "Debug empty input", "kinds": []string{"debugging", "tests", "small_change"}, "instructions": "Reproduce safely, recover the environment, and test a fix.", "acceptance_criteria": []string{"focused test passes"}, "tools": []map[string]string{{"name": "go", "version": "1.25"}}, "data": []map[string]string{{"name": "empty-input", "kind": "synthetic", "digest": "sha256:empty"}}, "setup_commands": []string{"go test ./..."}, "maximum_cost": 2}},
			"resources": []map[string]string{{"kind": "symbol", "label": "Parser", "path": "parser.go", "symbol": "Parse", "revision": revision}, {"kind": "documentation", "label": "Private maintainer note", "path": "private/maintainer.md", "revision": revision}}}},
	}
	var pathway learningpathways.Pathway
	workflowValue(t, server.URL, http.MethodPost, root+"/learning-pathways/parser/versions", maintainer, pathwayInput, http.StatusCreated, &pathway)
	if len(pathway.Versions[0].Findings) < 3 {
		t.Fatalf("learning gaps were hidden: %#v", pathway.Versions[0].Findings)
	}

	attemptBase := root + "/learning-pathways/parser/attempts"
	var exercise learningexercises.Attempt
	workflowValue(t, server.URL, http.MethodPost, attemptBase, learner, map[string]any{"pathway_version": 1, "module_id": "parser", "exercise_index": 0}, http.StatusCreated, &exercise)
	// A broken setup, misleading hint, and correction remain in the trail.
	for _, event := range []map[string]any{
		{"kind": "setup", "summary": "legacy shell cannot locate Go"},
		{"kind": "command", "summary": "initial setup failed", "command": "go test ./...", "output": "go: command not found"},
		{"kind": "hint", "summary": "agent suggested changing production configuration"},
		{"kind": "recovery", "summary": "learner rejected the misleading hint and selected the supported toolchain"},
		{"kind": "command", "summary": "focused reproduction", "command": "go test ./...", "output": "ok"},
		{"kind": "checkpoint", "summary": "learner-authored repair", "digest": "sha256:learner-checkpoint"},
		{"kind": "check", "summary": "focused test passes", "output": "ok", "digest": "sha256:stable-check", "cost": 1.0},
		{"kind": "complete", "summary": "exercise complete"},
	} {
		workflowValue(t, server.URL, http.MethodPost, attemptBase+"/"+exercise.ID+"/events", learner, event, http.StatusCreated, &exercise)
	}
	if !exercise.Reproducible || exercise.Status != "completed" || exercise.HintsUsed != 1 {
		t.Fatalf("exercise recovery not retained: %#v", exercise)
	}
	workflowValue(t, server.URL, http.MethodPost, attemptBase+"/"+exercise.ID+"/help", learner, map[string]any{"kind": "question", "recipient_kind": "mentor", "recipient_id": "mentor", "body": "Why is empty input a boundary?", "shared_event_numbers": []int{2, 4}, "workspace_access": "observe", "learner_authorized": true}, http.StatusCreated, &exercise)
	workflowValue(t, server.URL, http.MethodPost, attemptBase+"/"+exercise.ID+"/help", mentor, map[string]any{"kind": "guidance", "guidance_kind": "hint", "body": "Compare the public contract with the empty-input branch; retain authorship.", "citations": []map[string]string{{"kind": "symbol", "label": "Parser", "path": "parser.go", "revision": revision}}}, http.StatusCreated, &exercise)
	// The unavailable agent path fails closed instead of inventing approval.
	workflowValue(t, server.URL, http.MethodPost, attemptBase+"/"+exercise.ID+"/help", learner, map[string]any{"kind": "question", "recipient_kind": "agent", "recipient_id": "opaque-agent", "agent_approval_id": "missing", "body": "Solve it"}, http.StatusUnprocessableEntity, nil)

	assessmentBase := root + "/learning-pathways/parser/assessments"
	var assessment learningassessments.Assessment
	definition := map[string]any{"id": "parser-readiness", "title": "Parser readiness", "summary": "Repair an unseen edge", "pathway_version": 1, "revision": revision,
		"criteria":        []map[string]any{{"id": "diagnosis", "title": "Diagnosis", "description": "Explain and repair independently", "required": true, "human_judgment_required": true}},
		"protected_cases": []map[string]string{{"id": "hidden", "title": "Unseen whitespace case", "digest": "sha256:hidden", "material": "SECRET EXPECTED SOLUTION"}}, "checks": []map[string]any{{"name": "parser", "required": true}}, "owner_ids": []string{"maintainer"}, "reviewer_ids": []string{"reviewer"}, "maximum_attempts": 2, "appeal_owner_ids": []string{"maintainer"}}
	workflowValue(t, server.URL, http.MethodPost, assessmentBase, maintainer, definition, http.StatusCreated, &assessment)
	encoded, _ := json.Marshal(assessment)
	if strings.Contains(string(encoded), "SECRET EXPECTED SOLUTION") {
		t.Fatalf("protected assessment case leaked: %s", encoded)
	}
	newAttempt := func() learningassessments.Assessment {
		var got learningassessments.Assessment
		workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/parser-readiness/attempts", learner, map[string]any{"revision": revision, "workspace_digest": "sha256:assessment", "reproduction_commands": []string{"go test ./..."}, "assistance": []string{"mentor conceptual hint; learner authored code"}}, http.StatusCreated, &got)
		return got
	}
	assessment = newAttempt()
	failedID := assessment.Attempts[len(assessment.Attempts)-1].ID
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/parser-readiness/attempts/"+failedID+"/evidence", learner, map[string]any{"kind": "repository_check", "summary": "first attempt fails", "reference": "check:failed", "digest": "sha256:failed", "check_name": "parser", "check_status": "fail"}, http.StatusCreated, &assessment)
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/parser-readiness/attempts/"+failedID+"/judgments", reviewer, map[string]any{"outcome": "fail", "feedback": "Diagnosis missed whitespace", "rubric": []map[string]any{{"criterion_id": "diagnosis", "decision": "fail", "rationale": "failed practical evidence", "evidence_numbers": []int{1}}}, "integrity": map[string]any{"copied_solution": false, "agent_overreach": false}}, http.StatusCreated, &assessment)
	assessment = newAttempt()
	passedID := assessment.Attempts[len(assessment.Attempts)-1].ID
	for _, evidence := range []map[string]any{{"kind": "repository_check", "summary": "stable parser check", "reference": "check:pass", "digest": "sha256:pass", "check_name": "parser", "check_status": "pass"}, {"kind": "explanation", "summary": "learner explains the boundary", "reference": "artifact:explanation", "digest": "sha256:explanation"}} {
		workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/parser-readiness/attempts/"+passedID+"/evidence", learner, evidence, http.StatusCreated, &assessment)
	}
	workflowValue(t, server.URL, http.MethodPost, assessmentBase+"/parser-readiness/attempts/"+passedID+"/judgments", reviewer, map[string]any{"outcome": "pass", "feedback": "Independent diagnosis and repair demonstrated", "rubric": []map[string]any{{"criterion_id": "diagnosis", "decision": "pass", "rationale": "stable check and explanation", "evidence_numbers": []int{1, 2}}}, "integrity": map[string]any{"copied_solution": false, "agent_overreach": false}}, http.StatusCreated, &assessment)
	if !assessment.Attempts[len(assessment.Attempts)-1].CompletionSupported {
		t.Fatalf("current competence was not supported: %#v", assessment.Attempts)
	}

	opportunity, err := opportunities.Publish(string(repo.ID), "maintainer", "Handle whitespace input", revision, "open", true, contributionopportunities.Input{Source: contributionopportunities.Source{Kind: "issue", ResourceID: "learning-followup"}, RequiredSkills: []string{"Go", "parser debugging"}, Interests: []string{"reliability"}, ExpectedOutcome: "Whitespace is rejected", Scope: []string{"parser.go", "parser_test.go"}, Risk: "low", MentorIDs: []string{"mentor"}, Assistance: "human_or_agent", AcceptanceCriteria: []string{"parser check passes"}})
	if err != nil {
		t.Fatal(err)
	}
	var matches struct {
		Items       []json.RawMessage `json:"items"`
		GrantsWrite bool              `json:"grants_write_access"`
	}
	workflowValue(t, server.URL, http.MethodGet, root+"/learning-pathways/parser/contribution-matches", learner, nil, http.StatusOK, &matches)
	if len(matches.Items) != 1 || matches.GrantsWrite {
		t.Fatalf("learning match boundary failed: %#v", matches)
	}
	var claimResponse struct {
		Claim contributionopportunities.Claim `json:"claim"`
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/contribution-opportunities/"+opportunity.ID+"/claims", learner, map[string]any{"note": "first try", "hours": 2}, http.StatusCreated, &claimResponse)
	workflowValue(t, server.URL, http.MethodPost, root+"/contribution-opportunity-claims/"+claimResponse.Claim.ID+"/release", learner, nil, http.StatusOK, nil)
	workflowValue(t, server.URL, http.MethodPost, root+"/contribution-opportunities/"+opportunity.ID+"/claims", learner, map[string]any{"note": "recovered after abandoning the first claim", "hours": 8}, http.StatusCreated, &claimResponse)

	var fork struct {
		ID         string `json:"id"`
		UpstreamID string `json:"upstream_id"`
	}
	workflowValue(t, server.URL, http.MethodPost, root+"/forks", learner, map[string]any{"name": "learner-parser-fork", "visibility": "private"}, http.StatusCreated, &fork)
	forkClone := gitClone(t, remote(fork.ID, learnerGit))
	configureWorkflowGit(t, forkClone, "Learner", "learner@example.test")
	gitOutput(t, forkClone, "switch", "-c", "whitespace")
	writeWorkflowFile(t, forkClone, "parser.go", "package parser\n\nimport \"strings\"\nfunc Parse(v string) bool { return strings.TrimSpace(v) != \"\" }\n")
	writeWorkflowFile(t, forkClone, "parser_test.go", "package parser\n\nimport \"testing\"\nfunc TestParse(t *testing.T) { if !Parse(\"ok\") || Parse(\"   \") { t.Fatal(\"parse boundary\") } }\n")
	gitOutput(t, forkClone, "add", ".")
	gitOutput(t, forkClone, "commit", "-m", "Learner repairs whitespace parsing")
	gitOutput(t, forkClone, "push", "-u", "origin", "whitespace")
	var pull pullrequests.PullRequest
	workflowValue(t, server.URL, http.MethodPost, root+"/pull-requests", learner, map[string]any{"title": "Reject whitespace input", "body": "Matched from pathway parser v1; exercise " + exercise.ID + "; assessment attempt " + passedID + "; mentor hint retained; learner authored the repair.", "source_repository_id": fork.ID, "source_branch": "whitespace", "target_branch": "main"}, http.StatusCreated, &pull)
	pullBase := root + "/pull-requests/" + pull.ID
	passed := waitForWorkflowCheck(t, server.URL, pullBase, maintainer, pull.SourceCommitID, checkruns.Succeeded)
	workflowValue(t, server.URL, http.MethodPut, pullBase+"/reviews/me", maintainer, map[string]any{"decision": "approve"}, http.StatusOK, &pull)
	var ready readinessResponse
	workflowValue(t, server.URL, http.MethodGet, pullBase+"/readiness", maintainer, nil, http.StatusOK, &ready)
	if !ready.Ready || ready.Checks.Requirements[0].RunID != passed.ID {
		t.Fatalf("ordinary contribution gate failed: %#v", ready)
	}
	workflowValue(t, server.URL, http.MethodPost, pullBase+"/merge", maintainer, nil, http.StatusOK, &pull)
	if pull.AuthorID != "learner" || pull.MergedByID != "maintainer" {
		t.Fatalf("authorship/authority lost: %#v", pull)
	}
	if member, _ := repos.IsCollaborator(repo.ID, "learner"); member {
		t.Fatal("learning or merge silently granted upstream authority")
	}

	outcomeBase := root + "/learning-pathways/parser/outcomes"
	workflowValue(t, server.URL, http.MethodPost, outcomeBase+"/observations", maintainer, map[string]any{"id": "review-correction", "kind": "reviewer_correction", "module_id": "parser", "pathway_version": 1, "project_revision": pull.MergeCommitID, "audience": "repository", "consent": "granted", "count": 1, "summary": "Reviewer clarified whitespace semantics after the learner contribution", "evidence_references": []string{"pull:" + pull.ID, "assessment-attempt:" + passedID}}, http.StatusCreated, nil)
	workflowValue(t, server.URL, http.MethodPost, outcomeBase+"/findings", maintainer, map[string]any{"id": "clarify-whitespace", "kind": "curriculum_gap", "module_id": "parser", "summary": "Exercise should name whitespace behavior", "observation_ids": []string{"review-correction"}, "confidence": "supported", "actor_kind": "human"}, http.StatusCreated, nil)
	var outcome learningoutcomes.Record
	workflowValue(t, server.URL, http.MethodPost, outcomeBase+"/improvements", maintainer, map[string]any{"id": "pathway-pr", "finding_ids": []string{"clarify-whitespace"}, "kind": "pathway", "summary": "Teach whitespace semantics to the next learner", "base_pathway_version": 1, "target_pathway_version": 2, "project_revision": pull.MergeCommitID, "delivery_kind": "pull_request", "delivery_id": pull.ID, "delivery_revision": pull.MergeCommitID, "review_status": "approved", "reviewer_id": "reviewer", "material": false}, http.StatusCreated, &outcome)
	if len(outcome.Improvements) != 1 || outcome.Improvements[0].ReviewStatus != "approved" {
		t.Fatalf("reviewed learning improvement missing: %#v", outcome)
	}

	// Moving main makes v1 module material stale, while its achievement and
	// contribution evidence remain append-only and inspectable.
	var inspected learningpathways.Pathway
	workflowValue(t, server.URL, http.MethodGet, root+"/learning-pathways/parser", "", nil, http.StatusOK, &inspected)
	if inspected.Versions[0].Modules[0].Resources[0].Status != "stale" || pull.Status != pullrequests.Merged || failedID == passedID {
		t.Fatalf("staleness or recovery history was erased: pathway=%#v pull=%#v", inspected, pull)
	}
}
