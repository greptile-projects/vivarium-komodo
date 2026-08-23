package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/regressioninvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRegressionInvestigationDefinesSharedComparableBoundary(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "regression", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repository.ID, "collaborator")
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	good, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\ngood\n", tree)))
	bad, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@example.test> 2 +0000\ncommitter A <a@example.test> 2 +0000\n\nbad\n", tree, good)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/good", ObjectID: good})
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: bad})
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	peer := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, store, repos, credentials, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/regression-investigations"
	body := `{"title":"Review navigation stopped","source":{"kind":"failed_check","resource_id":"check-42","revision":"` + string(bad) + `"},"scope":{"expected_behavior":"Review opens from the keyboard","regressed_behavior":"The shortcut does nothing","known_good":{"kind":"revision","reference":"good"},"known_bad":{"kind":"revision","reference":"main"},"environments":["linux/chromium"],"comparability":"Same synthetic fixture and browser major version","severity":"high","owner_ids":["owner"],"acceptance_criteria":["shortcut opens review"]},"evidence":[{"kind":"reproduction","resource_id":"repro-7","summary":"Credential-free keyboard reproduction","audience":"repository"}]}`
	var investigation ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, peer, body, 201, &investigation)
	if investigation.Scope.KnownGood.CommitID != string(good) || investigation.Scope.KnownBad.CommitID != string(bad) || len(investigation.Blockers) != 0 || investigation.Evidence[0].ActorID != "collaborator" || investigation.ScopeChanges[0].ActorID != "collaborator" {
		t.Fatalf("boundary was not retained: %#v", investigation)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", owner, `{"kind":"hypothesis","body":"The navigation refactor changed shortcut routing."}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/status", owner, `{"status":"ready","reason":"The team agrees on the bounded history and success condition."}`, 200, &investigation)
	if investigation.Status != "ready" || len(investigation.Entries) != 2 || investigation.Entries[0].ActorID != "owner" {
		t.Fatalf("collaboration trail missing: %#v", investigation)
	}
	newBad, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor A <a@example.test> 3 +0000\ncommitter A <a@example.test> 3 +0000\n\nnew bad\n", tree, bad)))
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: newBad})
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+investigation.ID, peer, "", 200, &investigation)
	if len(investigation.StaleInputs) != 1 || investigation.StaleInputs[0] != "known_bad_revision_changed" {
		t.Fatalf("branch movement was not exposed: %#v", investigation.StaleInputs)
	}
	workflowJSON(t, server.URL, http.MethodPost, base, peer, `{"title":"Reversed","source":{"kind":"issue","resource_id":"1"},"scope":{"known_good":{"kind":"revision","reference":"main"},"known_bad":{"kind":"revision","reference":"good"}}}`, 422, nil)
}

func TestRegressionInvestigationExposesMissingInputsAndScopeHistory(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	store, _ := ri.New(t.TempDir())
	repository, _ := repos.Create("owner", repositories.Metadata{Name: "gaps", Visibility: repositories.Public})
	opened, _ := repos.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerRegressionInvestigationsHTTP(mux, store, repos, credentials, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repository.ID) + "/regression-investigations"
	var v ri.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, token, `{"title":"Unbounded report","source":{"kind":"issue","resource_id":"issue-9"},"scope":{}}`, 201, &v)
	if len(v.Blockers) != 9 {
		t.Fatalf("missing inputs hidden: %#v", v.Blockers)
	}
	scope := `{"expected_version":1,"reason":"Support confirmed both endpoints and the affected runtime.","scope":{"expected_behavior":"works","regressed_behavior":"fails","known_good":{"kind":"revision","reference":"main"},"known_bad":{"kind":"revision","reference":"main"},"environments":["test"],"comparability":"same fixture","severity":"medium","owner_ids":["owner"],"acceptance_criteria":["works"]}}`
	workflowJSON(t, server.URL, http.MethodPut, base+"/"+v.ID+"/scope", token, scope, 200, &v)
	if v.Version != 2 || len(v.Blockers) != 0 || len(v.ScopeChanges) != 2 || v.ScopeChanges[1].Reason == "" {
		t.Fatalf("scope history incomplete: %#v", v)
	}
}
