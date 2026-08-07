package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCollaboratorStartsAndReconnectsToChangeSession(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	proposals, _ := proposals.New(t.TempDir())
	sessionsRoot := t.TempDir()
	sessions, _ := changesessions.New(sessionsRoot)
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	catalog.AddCollaborator("owner", repository.ID, "collaborator")
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "collaborator", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "captured-source", TargetCommitID: "base"})
	token := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pulls, proposals, catalog, credentials)
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil)
	path := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID + "/change-sessions"
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var created changesessions.Session
	json.NewDecoder(response.Body).Decode(&created)
	if response.Code != http.StatusCreated || created.InitiatorID != "collaborator" || created.SourceCommitID != "captured-source" || created.State != changesessions.AwaitingInstructions {
		t.Fatalf("created %#v status %d", created, response.Code)
	}

	reopened, _ := changesessions.New(sessionsRoot)
	mux = http.NewServeMux()
	registerChangeSessionsHTTP(mux, reopened, pulls, catalog, credentials, nil)
	request = httptest.NewRequest(http.MethodGet, path+"/"+created.ID+"/events", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var timeline struct {
		Items []changesessions.Event `json:"items"`
		Total int                    `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&timeline)
	if response.Code != http.StatusOK || timeline.Total != 1 || timeline.Items[0].Type != "session.started" || timeline.Items[0].ActorID != "collaborator" {
		t.Fatalf("timeline %#v status %d", timeline, response.Code)
	}
}

func TestCollaboratorDelegatesBoundedRunAndRevokesCredential(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repository.ID, "collaborator")
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "collaborator", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "captured-source", TargetCommitID: "base"})
	token := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil)
	base := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID + "/change-sessions"
	req := httptest.NewRequest(http.MethodPost, base, strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var session changesessions.Session
	_ = json.NewDecoder(res.Body).Decode(&session)
	body := `{"instructions":"Add retry coverage and keep the public API stable.","revision_id":"captured-source","context_paths":["apps/api","docs/README.md"],"working_branch":"agent/retry-coverage"}`
	req = httptest.NewRequest(http.MethodPost, base+"/"+session.ID+"/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	var delegated struct {
		Run        changesessions.Run `json:"run"`
		Credential struct {
			Token        string `json:"token"`
			RepositoryID string `json:"repository_id"`
			Branch       string `json:"branch"`
		} `json:"credential"`
	}
	_ = json.NewDecoder(res.Body).Decode(&delegated)
	if res.Code != 201 || delegated.Run.InitiatorID != "collaborator" || delegated.Run.RevisionID != "captured-source" || delegated.Run.WorkingBranch != "agent/retry-coverage" || delegated.Credential.RepositoryID != string(repository.ID) || delegated.Credential.Branch != "refs/heads/agent/retry-coverage" {
		t.Fatalf("delegation %#v status %d", delegated, res.Code)
	}
	grant, err := credentials.Authenticate(delegated.Credential.Token, auth.GitWrite)
	if err != nil || grant.RepositoryID != string(repository.ID) || grant.Branch != "refs/heads/agent/retry-coverage" {
		t.Fatalf("scoped grant %#v %v", grant, err)
	}
	req = httptest.NewRequest(http.MethodDelete, base+"/"+session.ID+"/runs/"+delegated.Run.ID+"/credential", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res = httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != 204 {
		t.Fatalf("revoke status %d %s", res.Code, res.Body.String())
	}
	if _, err = credentials.Authenticate(delegated.Credential.Token, auth.GitRead); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("credential remained usable: %v", err)
	}
}
