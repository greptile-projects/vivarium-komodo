package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestWorkerPublishesAttributedRunTimeline(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "owner", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "exact-revision", TargetCommitID: "base"})
	session, _ := sessions.Create(string(repository.ID), pull.ID, "owner", "exact-revision")
	issued, _ := credentials.IssueRepositoryGit("owner", "worker", string(repository.ID), "refs/heads/agent/work", 24*time.Hour)
	run, _ := sessions.Delegate(string(repository.ID), pull.ID, session.ID, changesessions.DelegateParams{InitiatorID: "owner", Agent: "codex", Instructions: "Work", RevisionID: "exact-revision", WorkingBranch: "agent/work", CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt})
	mux := http.NewServeMux()
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil)
	base := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID + "/change-sessions/" + session.ID + "/runs/" + run.ID + "/events"
	for _, body := range []string{
		`{"type":"run.started","metadata":{"status":"Inspecting repository"}}`,
		`{"type":"tool.completed","metadata":{"tool":"go test","summary":"All API tests passed"}}`,
		`{"type":"artifact.produced","metadata":{"kind":"patch","path":"apps/api/change_sessions_http.go"}}`,
		`{"type":"branch.updated","metadata":{"branch":"agent/work","commit_id":"abc123"}}`,
		`{"type":"run.completed","metadata":{"summary":"Published the requested change"}}`,
	} {
		req := httptest.NewRequest(http.MethodPost, base, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+issued.Token)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("publish %s: %d %s", body, res.Code, res.Body.String())
		}
		var event changesessions.Event
		_ = json.NewDecoder(res.Body).Decode(&event)
		if event.RunID != run.ID || event.InitiatorID != "owner" || event.ActorID != "owner" || event.Agent != "codex" || event.RevisionID != "exact-revision" {
			t.Fatalf("lost durable attribution: %#v", event)
		}
	}
	restored, _ := sessions.Get(string(repository.ID), pull.ID, session.ID)
	if restored.Runs[0].State != changesessions.Succeeded || len(restored.Events) != 7 {
		t.Fatalf("restored timeline %#v", restored)
	}
}

func TestPeerCollaboratorGuidesPausesResumesAndCancelsRun(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repository.ID, "peer")
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "owner", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "revision", TargetCommitID: "base"})
	session, _ := sessions.Create(string(repository.ID), pull.ID, "owner", "revision")
	issued, _ := credentials.IssueRepositoryGit("owner", "worker", string(repository.ID), "refs/heads/agent/work", 24*time.Hour)
	run, _ := sessions.Delegate(string(repository.ID), pull.ID, session.ID, changesessions.DelegateParams{InitiatorID: "owner", Agent: "codex", Instructions: "Work", RevisionID: "revision", WorkingBranch: "agent/work", CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt})
	peerToken := issueAccess(t, credentials, "peer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerChangeSessionsHTTP(mux, sessions, pulls, catalog, credentials, nil)
	path := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID + "/change-sessions/" + session.ID + "/runs/" + run.ID + "/interventions"
	for _, body := range []string{`{"type":"guidance","message":"Do not change the public response."}`, `{"type":"answer","message":"Yes, preserve legacy records."}`, `{"type":"pause"}`, `{"type":"resume"}`, `{"type":"cancel","message":"Stop; requirements changed."}`} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+peerToken)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != http.StatusCreated {
			t.Fatalf("%s: %d %s", body, res.Code, res.Body.String())
		}
		if strings.Contains(body, `"guidance"`) {
			control := httptest.NewRequest(http.MethodGet, strings.TrimSuffix(path, "/interventions")+"/control", nil)
			control.Header.Set("Authorization", "Bearer "+issued.Token)
			controlResponse := httptest.NewRecorder()
			mux.ServeHTTP(controlResponse, control)
			var snapshot struct {
				State         changesessions.RunState `json:"state"`
				Interventions []changesessions.Event  `json:"interventions"`
			}
			_ = json.NewDecoder(controlResponse.Body).Decode(&snapshot)
			if controlResponse.Code != http.StatusOK || snapshot.State != changesessions.Queued || len(snapshot.Interventions) != 1 || snapshot.Interventions[0].Metadata["message"] != "Do not change the public response." {
				t.Fatalf("control snapshot: %d %#v", controlResponse.Code, snapshot)
			}
		}
	}
	got, _ := sessions.Get(string(repository.ID), pull.ID, session.ID)
	if got.Runs[0].State != changesessions.Canceled || got.Runs[0].CredentialRevokedAt == nil || len(got.Events) != 7 {
		t.Fatalf("controlled run: %#v", got)
	}
	if _, err := credentials.Authenticate(issued.Token, auth.GitWrite); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("canceled credential usable: %v", err)
	}
}
