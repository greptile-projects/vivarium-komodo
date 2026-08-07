package main

import (
	"encoding/json"
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
