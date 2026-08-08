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
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAgentAssignedTaskStartsBeforePullRequest(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	plans, _ := proposals.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "planned-work", Description: "Shared repository context", Visibility: repositories.Private})
	opened, _ := catalog.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, []byte{})
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor Owner <owner@example.test> 1 +0000\ncommitter Owner <owner@example.test> 1 +0000\n\nbase\n"))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	proposal, _ := plans.Create(string(repository.ID), "owner", "Improve onboarding", "New contributors need a clean setup path.")
	dependency, _ := plans.CreateTask(string(repository.ID), proposal.ID, "owner", proposals.TaskInput{Title: "Choose flow", Outcome: "The setup flow is agreed."})
	dependency, _ = plans.UpdateTask(string(repository.ID), proposal.ID, dependency.ID, "owner", proposals.TaskInput{Title: dependency.Title, Outcome: dependency.Outcome, Status: proposals.TaskCompleted, Position: 1})
	task, _ := plans.CreateTask(string(repository.ID), proposal.ID, "owner", proposals.TaskInput{Title: "Implement flow", Outcome: "A candidate implementation exists.", DependsOn: []string{dependency.ID}})
	task, _ = plans.AssignTask(string(repository.ID), proposal.ID, task.ID, "owner", "", proposals.AssignmentInput{Kind: proposals.AgentAssignee, AssigneeID: "codex", Mandate: "Implement only the agreed onboarding flow.", RepositoryID: string(repository.ID), BaseRevision: string(base)})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerProposalTaskSessionsHTTP(mux, plans, sessions, catalog, credentials, nil)
	server := httptest.NewServer(mux)
	defer server.Close()
	baseURL := server.URL + "/repositories/" + string(repository.ID) + "/proposals/" + proposal.ID + "/plan/tasks/" + task.ID + "/change-sessions"

	request, _ := http.NewRequest(http.MethodPost, baseURL, strings.NewReader(`{"expected_assignment_id":"`+task.Assignment.ID+`"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var started struct {
		Task       proposals.Task                 `json:"task"`
		Session    changesessions.Session         `json:"session"`
		Run        changesessions.Run             `json:"run"`
		Credential struct{ Token, Branch string } `json:"credential"`
	}
	_ = json.NewDecoder(response.Body).Decode(&started)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || started.Task.Status != proposals.TaskInProgress || started.Session.PullRequestID == "" || started.Session.TaskContext == nil || started.Session.TaskContext.ProposalDescription != proposal.Body || len(started.Session.TaskContext.Dependencies) != 1 || started.Run.State != changesessions.Queued {
		t.Fatalf("started = %#v status %d", started, response.StatusCode)
	}
	branch, err := opened.ReadReference(storage.ReferenceName(started.Credential.Branch))
	if err != nil || branch.ObjectID != base {
		t.Fatalf("working branch = %#v, %v", branch, err)
	}
	grant, err := credentials.Authenticate(started.Credential.Token, auth.GitWrite)
	if err != nil || grant.RepositoryID != string(repository.ID) || grant.Branch != started.Credential.Branch {
		t.Fatalf("worker grant = %#v, %v", grant, err)
	}

	request, _ = http.NewRequest(http.MethodPost, baseURL+"/"+started.Session.ID+"/runs/"+started.Run.ID+"/interventions", strings.NewReader(`{"type":"pause"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("pause = %d", response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodPost, baseURL+"/"+started.Session.ID+"/runs/"+started.Run.ID+"/interventions", strings.NewReader(`{"type":"resume"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("resume = %d", response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodPost, baseURL+"/"+started.Session.ID+"/runs/"+started.Run.ID+"/events", strings.NewReader(`{"type":"agent.message","metadata":{"status":"Using captured plan context"}}`))
	request.Header.Set("Authorization", "Bearer "+started.Credential.Token)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("worker progress = %d", response.StatusCode)
	}

	response, _ = http.Get(baseURL + "/" + started.Session.ID)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("private reconnect without auth = %d", response.StatusCode)
	}
	response.Body.Close()
	request, _ = http.NewRequest(http.MethodGet, baseURL+"/"+started.Session.ID, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var reconnected changesessions.Session
	_ = json.NewDecoder(response.Body).Decode(&reconnected)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || reconnected.Runs[0].State != changesessions.Running || len(reconnected.Events) < 5 {
		t.Fatalf("reconnected = %#v status %d", reconnected, response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodPost, baseURL, strings.NewReader(`{"expected_assignment_id":"`+task.Assignment.ID+`"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate start = %d", response.StatusCode)
	}
}
