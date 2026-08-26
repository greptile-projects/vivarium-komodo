package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRunbookRehearsalPublicContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "operations", Visibility: repositories.Private})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	books, _ := runbooks.New(t.TempDir())
	book, _ := books.Create(string(repo.ID), "owner", runbooks.Input{Name: "Restore API", Purpose: "recover safely", Scope: runbooks.Scope{Kind: "service", ResourceID: "api", Revision: "v7", OwnerID: "owner"}, Preconditions: []runbooks.Precondition{{ID: "impact", Description: "confirm impact", Evidence: "synthetic dashboard", OwnerID: "owner", Safe: true}}, Steps: []runbooks.Step{{ID: "inspect", Kind: "diagnostic", Title: "Inspect", Purpose: "diagnose", Preconditions: []string{"impact"}, ExpectedEvidence: []string{"errors"}, RequiredSkills: []string{"diagnosis"}}}, RollbackCriteria: []string{"health worsens"}, OwnerIDs: []string{"owner"}, RequiredSkills: []string{"diagnosis"}, EscalationPaths: []runbooks.Escalation{{Condition: "blocked", OwnerID: "owner", RequiredSkills: []string{"command"}, AudienceIDs: []string{"team"}, Action: "escalate"}}, ChangeReason: "initial"})
	store, _ := runbookrehearsals.New(t.TempDir())
	mux := http.NewServeMux()
	registerRunbookRehearsalsHTTP(mux, store, books, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID) + "/runbooks/" + book.ID + "/rehearsals"
	body := `{"runbook_version":1,"title":"API failure","environment_id":"sandbox","environment_revision":"env-v2","environment_class":"isolated","limits":{"max_duration_seconds":300,"max_cost":5,"currency":"USD"},"scenarios":[{"id":"errors","name":"Elevated errors","failure":"synthetic 500s","evidence_source":"synthetic","input_digest":"sha256:in","expected_outcomes":["healthy"],"references":[{"kind":"service","resource_id":"api","revision":"v7"}]}],"owner_ids":["owner"]}`
	var x runbookrehearsals.Rehearsal
	workflowJSON(t, server.URL, http.MethodPost, root, token, body, 201, &x)
	if x.RunbookID != book.ID || x.Ready || len(x.Gaps) != 1 {
		t.Fatalf("missing scenario proof should be explicit: %#v", x)
	}
	workflowJSON(t, server.URL, http.MethodPost, root, token, `{"runbook_version":9}`, 422, nil)
}
