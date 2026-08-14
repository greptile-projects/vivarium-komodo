package main

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
	"net/http/httptest"
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
