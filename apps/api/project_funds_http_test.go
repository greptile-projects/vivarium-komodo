package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectFundKeepsUnverifiedValueUnavailable(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	creds, _ := auth.New(t.TempDir())
	owner, backer, steward := "owner", "backer", "steward"
	repo, _ := repos.Create(owner, repositories.Metadata{Name: "funded", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator(owner, repo.ID, backer)
	_, _ = repos.AddCollaborator(owner, repo.ID, steward)
	ownerToken := issueAccess(t, creds, owner, auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	backerToken := issueAccess(t, creds, backer, auth.API, auth.RepositoryRead)
	stewardToken := issueAccess(t, creds, steward, auth.API, auth.RepositoryRead)
	store, _ := projectfunds.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectFundsHTTP(mux, store, repos, creds)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/funds"
	body := `{"name":"Community delivery fund","description":"Back reviewed work","steward_ids":["steward"],"accepted_funding_sources":["stripe"],"unit":"USD","unit_kind":"currency","spending_limits":{"per_allocation":50000,"per_recipient":100000,"total":200000},"approval_rule":{"minimum_approvals":1,"approver_ids":["owner"],"threshold":10000},"eligible_recipients":["human","approved_agent_operator"],"refund_policy":"Refund unreserved value on request","ledger_visibility":"repository"}`
	var f projectfunds.Fund
	workflowJSON(t, server.URL, http.MethodPost, base, ownerToken, body, 201, &f)
	if len(f.OperationalAuthority) != 0 {
		t.Fatal("fund granted authority")
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+f.ID+"/commitments", backerToken, `{"reference":"charge-1","source":"stripe","amount":10000,"state":"pending"}`, 201, &f)
	if f.Balances.Available != 0 {
		t.Fatal("pending transfer became spendable")
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+f.ID+"/commitments", backerToken, `{"reference":"charge-1","source":"stripe","amount":10000,"state":"settled","settled":10000}`, 409, nil)
	tid := f.Transfers[0].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+f.ID+"/transfers/"+tid+"/reconcile", backerToken, `{"expected_version":2,"state":"settled","settled":10000}`, 403, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+f.ID+"/transfers/"+tid+"/reconcile", stewardToken, `{"expected_version":2,"state":"partial","settled":6000,"note":"processor settled first tranche"}`, 200, &f)
	if f.Balances.Available != 6000 || f.Transfers[0].State != "partial" {
		t.Fatalf("bad partial balance: %+v", f)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+f.ID+"/commitments", backerToken, `{"reference":"charge-failed","source":"stripe","amount":9000,"state":"failed"}`, 201, &f)
	if f.Balances.Available != 6000 {
		t.Fatal("failed transfer changed value")
	}
}

func TestFundedOutcomeMakesEvidenceBackingAndReplanningExplicit(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	creds, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("steward", repositories.Metadata{Name: "measurable", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("steward", repo.ID, "backer")
	stewardToken := issueAccess(t, creds, "steward", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	backerToken := issueAccess(t, creds, "backer", auth.API, auth.RepositoryRead)
	store, _ := projectfunds.New(t.TempDir())
	mux := http.NewServeMux()
	registerProjectFundsHTTP(mux, store, repos, creds)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID)
	var fund projectfunds.Fund
	workflowJSON(t, server.URL, http.MethodPost, base+"/funds", stewardToken, `{"name":"Compute fund","steward_ids":["steward"],"accepted_funding_sources":["provider"],"unit":"credits","unit_kind":"credit","spending_limits":{"per_allocation":10000,"per_recipient":20000,"total":30000},"approval_rule":{"minimum_approvals":1,"approver_ids":["steward"]},"eligible_recipients":["human","approved_agent_operator"],"refund_policy":"return unreserved credits","ledger_visibility":"repository"}`, 201, &fund)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funds/"+fund.ID+"/commitments", backerToken, `{"reference":"credits-1","source":"provider","amount":10000,"settled":10000,"state":"settled"}`, 201, &fund)

	terms := `{"origin":{"kind":"issue","resource_id":"issue-42"},"title":"Remove parser bottleneck","scope":"Optimize parser without changing output","acceptance_criteria":["p95 below 50ms","all correctness checks pass"],"evidence_requirements":["comparable benchmark trial","passing check run"],"budget":8000,"deadline":"2027-08-14T00:00:00Z","contributor_eligibility":["human","approved_agent_operator"],"allocation_method":"milestone review","cancellation_terms":"unmet guardrail cancels remaining work","milestones":[{"id":"measure","name":"Validated improvement","budget":8000,"acceptance_criteria":["p95 below 50ms"],"evidence_requirements":["comparable benchmark trial"]}],"dependencies":["benchmark fixture"],"risks":["noisy host"],"declared_conflicts":["maintainer is also a backer"],"overlap_keys":["parser-p95"],"embargoed":true}`
	var outcome projectfunds.FundedOutcome
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes", stewardToken, `{"fund_id":"`+fund.ID+`","terms":`+terms+`}`, 201, &outcome)
	if len(outcome.OperationalAuthority) != 0 || len(outcome.Versions) != 1 || outcome.Blockers[0].Kind != "underfunded" {
		t.Fatalf("outcome contract was not explicit: %+v", outcome)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/pledges", backerToken, `{"target":"milestone:measure","amount":8000}`, 201, &outcome)
	if outcome.Pledged != 8000 || hasOutcomeBlocker(outcome.Blockers, "underfunded") || !hasOutcomeBlocker(outcome.Blockers, "embargoed_work") || !hasOutcomeBlocker(outcome.Blockers, "declared_conflict") {
		t.Fatalf("bad backing projection: %+v", outcome)
	}
	pledgeID := outcome.Pledges[0].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/pledges/"+pledgeID+"/withdraw", backerToken, `{"expected_version":2,"reason":"compute grant withdrawn"}`, 200, &outcome)
	if !hasOutcomeBlocker(outcome.Blockers, "underfunded") || outcome.Replanning[0].Kind != "backing_withdrawn" || outcome.Replanning[0].ActorID != "backer" {
		t.Fatalf("withdrawal did not produce attributable replanning: %+v", outcome)
	}

	var overlapping projectfunds.FundedOutcome
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes", stewardToken, `{"fund_id":"`+fund.ID+`","terms":`+terms+`}`, 201, &overlapping)
	workflowJSON(t, server.URL, http.MethodGet, base+"/funded-outcomes/"+outcome.ID, backerToken, "", 200, &outcome)
	if !hasOutcomeBlocker(outcome.Blockers, "overlapping_award") {
		t.Fatalf("overlap was hidden: %+v", outcome.Blockers)
	}

	replanned := strings.Replace(terms, "Optimize parser without changing output", "Optimize lexer and parser without changing output", 1)
	workflowJSON(t, server.URL, http.MethodPost, base+"/funded-outcomes/"+outcome.ID+"/replan", stewardToken, `{"expected_version":3,"reason":"profiling expanded the required scope","terms":`+replanned+`}`, 200, &outcome)
	if len(outcome.Versions) != 2 || outcome.Replanning[len(outcome.Replanning)-1].Kind != "scope_changed" || outcome.Versions[0].Terms.Scope == outcome.Versions[1].Terms.Scope {
		t.Fatalf("scope history was rewritten or unattributed: %+v", outcome)
	}
}

func hasOutcomeBlocker(items []projectfunds.OutcomeBlocker, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}
