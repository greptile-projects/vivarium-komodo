package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestProjectFundingWorkflow is the black-box boundary for the complete
// backing-to-delivered-outcome loop. Financial records observe ordinary Git,
// review, check, preview, merge, and release evidence but grant no authority.
func TestProjectFundingWorkflow(t *testing.T) {
	requireGit(t)
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	checks, _ := checkruns.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	funds, _ := projectfunds.New(t.TempDir())
	runner := checkruns.NewRunner(checks, catalog)
	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials, nil, runner, checks)
	registerCheckRunsHTTP(mux, checks, runner, pulls, catalog, credentials, nil, nil)
	registerReleasesHTTP(mux, releaseStore, checks, runner, pulls, catalog, credentials)
	registerProjectFundsHTTP(mux, funds, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	owner := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	developer := issueAccess(t, credentials, "developer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	operator := issueAccess(t, credentials, "agent-operator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	backup := issueAccess(t, credentials, "backup-operator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	backer := issueAccess(t, credentials, "community-backer", auth.API, auth.RepositoryRead)
	ownerGit := issueAccess(t, credentials, "maintainer", auth.Git, auth.GitRead, auth.GitWrite)
	developerGit := issueAccess(t, credentials, "developer", auth.Git, auth.GitRead, auth.GitWrite)
	var repository repositories.Repository
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", owner, `{"name":"community-funded-roadmap","visibility":"private"}`, http.StatusCreated, &repository)
	for _, actor := range []string{"developer", "agent-operator", "backup-operator", "community-backer"} {
		if _, err := catalog.AddCollaborator("maintainer", repository.ID, actor); err != nil {
			t.Fatal(err)
		}
	}
	base := "/repositories/" + string(repository.ID)

	var fund projectfunds.Fund
	workflowJSON(t, server.URL, http.MethodPost, base+"/funds", owner, `{"name":"Community roadmap fund","steward_ids":["maintainer"],"accepted_funding_sources":["community-provider"],"unit":"USD","unit_kind":"currency","spending_limits":{"per_allocation":12000,"per_recipient":12000,"total":20000},"approval_rule":{"minimum_approvals":1,"approver_ids":["maintainer"]},"eligible_recipients":["human","approved_agent_operator"],"refund_policy":"refund rejected or disputed value","ledger_visibility":"repository"}`, http.StatusCreated, &fund)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funds/"+fund.ID+"/commitments", backer, `{"reference":"roadmap-backing-1","source":"community-provider","amount":12000,"settled":12000,"state":"settled"}`, http.StatusCreated, &fund)

	terms := `{"origin":{"kind":"roadmap_outcome","resource_id":"roadmap-outcome-faster-review"},"title":"Make review status actionable","scope":"Explain the next review action","acceptance_criteria":["ordinary checks pass","review flow is measurable"],"evidence_requirements":["reviewed pull request","preview","release","outcome measure"],"budget":12000,"deadline":"2027-08-14T00:00:00Z","contributor_eligibility":["human","approved_agent_operator"],"allocation_method":"complementary milestone awards","cancellation_terms":"reject or refund unsupported value","milestones":[{"id":"implementation","name":"Reviewed implementation","budget":5000,"acceptance_criteria":["merged change"],"evidence_requirements":["commit","check","preview"],"reviewer_ids":["maintainer"]},{"id":"verification","name":"Agent verification","budget":4000,"acceptance_criteria":["measured behavior"],"evidence_requirements":["check","outcome measure"],"reviewer_ids":["maintainer"]},{"id":"adoption","name":"Measured adoption","budget":3000,"acceptance_criteria":["target reached"],"evidence_requirements":["release","outcome measure"],"reviewer_ids":["maintainer"]}],"dependencies":["ordinary repository review"],"risks":["agent compute overrun"],"declared_conflicts":[],"overlap_keys":["review-next-action"],"embargoed":false}`
	var outcome projectfunds.FundedOutcome
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes", owner, `{"fund_id":"`+fund.ID+`","terms":`+terms+`}`, http.StatusCreated, &outcome)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/pledges", backer, `{"target":"outcome","amount":12000}`, http.StatusCreated, &outcome)
	replanned := replaceOnce(terms, "Explain the next review action", "Explain and instrument the next review action")
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/replan", owner, `{"expected_version":2,"reason":"community review added measurable instrumentation","terms":`+replanned+`}`, http.StatusOK, &outcome)

	human := fundingProposal(t, server.URL, base, outcome.ID, developer, "human", "developer", `[{"id":"implementation","approach":"implement and preview","cost":5000,"deliverables":["pull request","preview"]},{"id":"adoption","approach":"measure released adoption","cost":3000,"deliverables":["release measure"]}]`, 8000)
	agent := fundingProposal(t, server.URL, base, outcome.ID, operator, "approved_agent_operator", "agent-operator", `[{"id":"verification","approach":"verify checks and measures","cost":4000,"deliverables":["check evidence"]}]`, 4000)
	selectFundingProposal(t, server.URL, base, outcome.ID, owner, developer, &human)
	selectFundingProposal(t, server.URL, base, outcome.ID, owner, operator, &agent)

	remote := func(token string) string {
		u, _ := url.Parse(server.URL + base)
		u.User = url.UserPassword("git", token)
		return u.String()
	}
	initial := gitClone(t, remote(ownerGit))
	gitOutput(t, initial, "config", "user.name", "Maintainer")
	gitOutput(t, initial, "config", "user.email", "maintainer@example.com")
	writeWorkflowFile(t, initial, ".komodo/checks.json", `{"version":1,"checks":[{"name":"funded-outcome","command":"grep -q actionable review.txt"}]}`)
	writeWorkflowFile(t, initial, ".komodo/releases.json", `{"version":1,"builds":[{"name":"review-ui","command":"mkdir -p dist; cp review.txt dist/review.txt","artifacts":["dist/review.txt"]}]}`)
	writeWorkflowFile(t, initial, "review.txt", "baseline\n")
	gitOutput(t, initial, "add", ".")
	gitOutput(t, initial, "commit", "-m", "Initialize funded roadmap outcome")
	gitOutput(t, initial, "push", "-u", "origin", "main")
	workflowJSON(t, server.URL, http.MethodPut, base+"/required-checks", owner, `{"branch":"main","checks":["funded-outcome"]}`, http.StatusOK, nil)
	work := gitClone(t, remote(developerGit))
	gitOutput(t, work, "config", "user.name", "Funded Developer")
	gitOutput(t, work, "config", "user.email", "developer@example.com")
	gitOutput(t, work, "switch", "-c", "funded/actionable-review")
	writeWorkflowFile(t, work, "review.txt", "actionable review status with measured next step\n")
	gitOutput(t, work, "add", "review.txt")
	gitOutput(t, work, "commit", "-m", "Make review status actionable")
	revision := gitOutput(t, work, "rev-parse", "HEAD")
	gitOutput(t, work, "push", "-u", "origin", "funded/actionable-review")
	var pull pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests", developer, `{"title":"Deliver community-funded review outcome","body":"Human implementation with approved-agent verification.","source_branch":"funded/actionable-review","target_branch":"main"}`, http.StatusCreated, &pull)
	check := waitForWorkflowCheck(t, server.URL, base+"/pull-requests/"+pull.ID, developer, revision, checkruns.Succeeded)
	workflowJSON(t, server.URL, http.MethodPut, base+"/pull-requests/"+pull.ID+"/reviews/me", owner, `{"decision":"approve"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/pull-requests/"+pull.ID+"/merge", owner, `{}`, http.StatusOK, &pull)
	var release releases.Release
	workflowJSON(t, server.URL, http.MethodPost, base+"/releases", owner, `{"version":"v1.0.0","commit_id":"`+pull.MergeCommitID+`","notes":"Community-funded actionable review outcome."}`, http.StatusCreated, &release)
	waitForReleaseArtifact(t, server.URL, string(repository.ID), release.ID, owner)

	humanEvidence := `[{"kind":"commit","id":"` + revision + `","revision":"` + revision + `","summary":"developer implementation"},{"kind":"pull_request","id":"` + pull.ID + `","revision":"` + revision + `","summary":"approved ordinary review"},{"kind":"check","id":"` + check.ID + `","revision":"` + revision + `","summary":"required check passed"},{"kind":"preview","id":"preview-` + pull.ID + `","revision":"` + revision + `","summary":"exact-revision review preview"},{"kind":"release","id":"` + release.ID + `","revision":"` + pull.MergeCommitID + `","summary":"released artifact"}]`
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+human.ID+"/progress", developer, `{"expected_version":4,"milestone_id":"implementation","status":"completed","percent":100,"summary":"implemented, reviewed, checked, previewed, merged and released","evidence":`+humanEvidence+`,"agent_compute":0,"access_state":"active","handoff_state":"accepted"}`, http.StatusCreated, &human)

	// The operator reports a cost above its reservation. The report is retained,
	// approval is contained, and a separately authorized replacement completes it.
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/expenses", operator, `{"expected_version":4,"milestone_id":"verification","amount":4500,"description":"agent compute exceeded the governed cap","evidence":[{"kind":"check","id":"`+check.ID+`","summary":"verification compute evidence"}]}`, http.StatusCreated, &agent)
	if !fundingBlocker(agent.Execution.Blockers, "overrun") {
		t.Fatalf("agent overrun was hidden: %#v", agent.Execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/expenses/"+agent.Execution.Expenses[0].ID+"/decision", owner, `{"expected_version":5,"approve":true,"reason":"must stay inside reservation"}`, http.StatusConflict, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/expenses/"+agent.Execution.Expenses[0].ID+"/decision", owner, `{"expected_version":5,"approve":false,"reason":"reject cost above the governed reservation"}`, http.StatusOK, &agent)
	if fundingBlocker(agent.Execution.Blockers, "overrun") {
		t.Fatalf("rejected excess cost did not recover spending: %#v", agent.Execution)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/controls", owner, `{"expected_version":6,"action":"replace","reason":"approved backup completes bounded verification","recipient_id":"backup-operator"}`, http.StatusOK, &agent)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/progress", backup, `{"expected_version":7,"milestone_id":"verification","status":"completed","percent":100,"summary":"verified the released result after bounded handoff","evidence":[{"kind":"check","id":"`+check.ID+`","revision":"`+revision+`","summary":"required verification passed"},{"kind":"outcome_measure","id":"review-action-rate","summary":"target measure passed"}],"agent_compute":800,"access_state":"active","handoff_state":"accepted"}`, http.StatusCreated, &agent)

	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+human.ID+"/milestones/implementation/reviews", owner, `{"expected_version":5,"decision":"accepted","rationale":"ordinary review, check, preview, merge and release prove delivery","evidence":`+humanEvidence+`}`, http.StatusCreated, &human)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+human.ID+"/milestones/adoption/reviews", owner, `{"expected_version":6,"decision":"rejected","rationale":"the first adoption sample did not reach the milestone","dissent":["developer expects delayed adoption"],"evidence":[{"kind":"outcome_measure","id":"adoption-sample-1","summary":"target missed"}]}`, http.StatusCreated, &human)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/milestones/verification/reviews", owner, `{"expected_version":8,"decision":"disputed","rationale":"compute attribution needs independent confirmation","evidence":[{"kind":"check","id":"`+check.ID+`","summary":"technical result passed"}]}`, http.StatusCreated, &agent)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/milestones/verification/reviews", owner, `{"expected_version":9,"decision":"accepted","rationale":"backup operator confirmed bounded evidence","evidence":[{"kind":"check","id":"`+check.ID+`","summary":"independently confirmed"},{"kind":"outcome_measure","id":"review-action-rate","summary":"measure passed"}]}`, http.StatusCreated, &agent)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/delivery-proposals/"+agent.ID+"/milestones/verification/recoveries", owner, `{"expected_version":10,"action":"refund","rationale":"community refund approved after the retained dispute"}`, http.StatusCreated, &agent)

	workflowJSON(t, server.URL, http.MethodGet, base+"/funds/"+fund.ID, owner, "", http.StatusOK, &fund)
	workflowJSON(t, server.URL, http.MethodGet, base+"/funded-outcomes/"+outcome.ID, backer, "", http.StatusOK, &outcome)
	if fund.Balances.Spent != 5000 || fund.Balances.Available != 7000 || len(outcome.Versions) != 2 || len(outcome.OperationalAuthority) != 0 || len(human.OperationalAuthority) != 0 || len(agent.OperationalAuthority) != 0 {
		t.Fatalf("retained funding trail is incomplete: fund=%+v outcome=%+v", fund.Balances, outcome)
	}
	if human.Execution.Settlements[0].Events[0].ActorID != "maintainer" || agent.Execution.Settlements[0].Payment != "refunded" || agent.Execution.Settlements[0].RecipientID != "agent-operator" || len(agent.Execution.Changes) != 1 {
		t.Fatalf("receipt, refund, or replacement attribution was lost: human=%+v agent=%+v", human.Execution, agent.Execution)
	}
}

func fundingProposal(t *testing.T, origin, base, outcome, token, kind, recipient, milestones string, cost int) projectfunds.DeliveryProposal {
	t.Helper()
	var p projectfunds.DeliveryProposal
	body := `{"terms":{"recipient_kind":"` + kind + `","recipient_id":"` + recipient + `","approach":"complementary funded delivery","milestones":` + milestones + `,"cost":` + strconv.Itoa(cost) + `,"dependencies":["ordinary repository policy"],"availability":"this iteration","required_access":["separately governed repository collaboration"],"relevant_attributed_work":[{"kind":"pull_request","id":"prior-work","description":"reviewed prior delivery"}]}}`
	workflowJSON(t, origin, http.MethodPost, base+"/funded-outcomes/"+outcome+"/delivery-proposals", token, body, http.StatusCreated, &p)
	workflowJSON(t, origin, http.MethodPost, base+"/funded-outcomes/"+outcome+"/delivery-proposals/"+p.ID+"/accept", token, `{"expected_version":1}`, http.StatusOK, &p)
	return p
}

func selectFundingProposal(t *testing.T, origin, base, outcome, owner, recipient string, p *projectfunds.DeliveryProposal) {
	t.Helper()
	workflowJSON(t, origin, http.MethodPost, base+"/funded-outcomes/"+outcome+"/delivery-proposals/"+p.ID+"/approve", owner, `{"expected_version":2}`, http.StatusOK, p)
	workflowJSON(t, origin, http.MethodPost, base+"/funded-outcomes/"+outcome+"/delivery-proposals/"+p.ID+"/select", owner, `{"expected_version":3,"reason":"approved complementary recipient with separate authority","connections":[{"kind":"proposal_task","id":"task-`+recipient+`"}]}`, http.StatusOK, p)
}

func fundingBlocker(items []projectfunds.OutcomeBlocker, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}
func replaceOnce(value, old, next string) string { return strings.Replace(value, old, next, 1) }
