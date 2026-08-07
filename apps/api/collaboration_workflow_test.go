package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/inbox"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

// TestNewcomerAndMaintainerCompleteCollaborationWorkflow provisions only the
// running application. Every actor action and observation uses JSON HTTP or a
// stock Git client against the same public server.
func TestNewcomerAndMaintainerCompleteCollaborationWorkflow(t *testing.T) {
	requireGit(t)
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	activityStore, _ := activities.New(t.TempDir(), userStore)
	inboxStore, _ := inbox.New(t.TempDir())
	mux := http.NewServeMux()
	registerUsersHTTP(mux, userStore, credentials)
	registerAuthHTTP(mux, credentials, userStore)
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerCollaboratorsHTTP(mux, catalog, userStore, credentials, activityStore)
	registerProposalsHTTP(mux, proposalStore, catalog, credentials, activityStore)
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials, activityStore)
	registerActivitiesHTTP(mux, activityStore, catalog, credentials)
	registerInboxHTTP(mux, activityStore, inboxStore, catalog, proposalStore, pullRequestStore, userStore, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	type actor struct {
		ID, API, Git string
	}
	newActor := func(handle, displayName string) actor {
		var user users.User
		workflowJSON(t, server.URL, http.MethodPost, "/users", "", `{"handle":"`+handle+`","display_name":"`+displayName+`","password":"correct horse battery staple"}`, http.StatusCreated, &user)
		response := workflowRequest(t, server.URL, http.MethodPost, "/sessions", "", `{"handle":"`+handle+`","password":"correct horse battery staple"}`, nil)
		if response.StatusCode != http.StatusCreated || len(response.Cookies()) != 1 {
			t.Fatalf("create %s session = %d", handle, response.StatusCode)
		}
		cookie := response.Cookies()[0]
		response.Body.Close()
		grant := func(kind, scopes string) string {
			response := workflowRequest(t, server.URL, http.MethodPost, "/access-grants", "", `{"name":"workflow","kind":"`+kind+`","scopes":`+scopes+`,"expires_in_hours":1}`, cookie)
			defer response.Body.Close()
			var issued struct {
				Token string `json:"token"`
			}
			if response.StatusCode != http.StatusCreated || json.NewDecoder(response.Body).Decode(&issued) != nil || issued.Token == "" {
				t.Fatalf("issue %s grant for %s = %d", kind, handle, response.StatusCode)
			}
			return issued.Token
		}
		return actor{ID: string(user.ID), API: grant("api", `["repository:read","repository:write"]`), Git: grant("git", `["git:read","git:write"]`)}
	}
	maintainer := newActor("maintainer", "Project Maintainer")
	newcomer := newActor("newcomer", "New Contributor")

	var repository struct {
		ID string `json:"id"`
	}
	workflowJSON(t, server.URL, http.MethodPost, "/repositories", maintainer.API, `{"name":"welcome","visibility":"public"}`, http.StatusCreated, &repository)
	workflowJSON(t, server.URL, http.MethodPut, "/repositories/"+repository.ID+"/collaborators/"+newcomer.ID, maintainer.API, "", http.StatusOK, nil)
	remote := func(token string) string {
		value, _ := url.Parse(server.URL + "/repositories/" + repository.ID)
		value.User = url.UserPassword("git", token)
		return value.String()
	}
	maintainerClone := gitClone(t, remote(maintainer.Git))
	gitOutput(t, maintainerClone, "config", "user.name", "Project Maintainer")
	gitOutput(t, maintainerClone, "config", "user.email", "maintainer@example.com")
	if err := os.WriteFile(filepath.Join(maintainerClone, "README.md"), []byte("# Welcome\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, maintainerClone, "add", "README.md")
	gitOutput(t, maintainerClone, "commit", "-m", "Start the project")
	gitOutput(t, maintainerClone, "push", "--set-upstream", "origin", "main")

	var proposal proposals.Proposal
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/proposals", newcomer.API, `{"title":"Add contributor guide","body":"Help first-time contributors get started."}`, http.StatusCreated, &proposal)
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/proposals/"+proposal.ID+"/comments", maintainer.API, `{"body":"Please include the test command."}`, http.StatusCreated, nil)

	newcomerClone := gitClone(t, remote(newcomer.Git))
	gitOutput(t, newcomerClone, "config", "user.name", "New Contributor")
	gitOutput(t, newcomerClone, "config", "user.email", "newcomer@example.com")
	gitOutput(t, newcomerClone, "switch", "-c", "candidate/contributor-guide")
	if err := os.WriteFile(filepath.Join(newcomerClone, "CONTRIBUTING.md"), []byte("# Contributing\n\nOpen a proposal first.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, newcomerClone, "add", "CONTRIBUTING.md")
	gitOutput(t, newcomerClone, "commit", "-m", "Add contributor guide")
	gitOutput(t, newcomerClone, "push", "--set-upstream", "origin", "candidate/contributor-guide")

	var pullRequest pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, "/repositories/"+repository.ID+"/pull-requests", newcomer.API, `{"proposal_id":"`+proposal.ID+`","title":"Add contributor guide","body":"Documents the newcomer workflow.","source_branch":"candidate/contributor-guide","target_branch":"main"}`, http.StatusCreated, &pullRequest)
	reviewPath := "/repositories/" + repository.ID + "/pull-requests/" + pullRequest.ID
	workflowJSON(t, server.URL, http.MethodPut, reviewPath+"/reviews/me", maintainer.API, `{"decision":"request_changes"}`, http.StatusOK, nil)
	workflowJSON(t, server.URL, http.MethodPost, reviewPath+"/comments", maintainer.API, `{"body":"@newcomer please add the exact API test command."}`, http.StatusCreated, nil)
	var responseInbox struct {
		Items []inboxItem `json:"items"`
		Total int         `json:"total_count"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/inbox?classification=response&per_page=100", newcomer.API, "", http.StatusOK, &responseInbox)
	if responseInbox.Total == 0 || !strings.Contains(responseInbox.Items[0].Href, "pull="+pullRequest.ID) {
		t.Fatalf("response inbox = %#v", responseInbox)
	}

	if err := os.WriteFile(filepath.Join(newcomerClone, "CONTRIBUTING.md"), []byte("# Contributing\n\nOpen a proposal first, then run `go test ./...`.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, newcomerClone, "commit", "-am", "Address review feedback")
	followUpCommit := gitOutput(t, newcomerClone, "rev-parse", "HEAD")
	gitOutput(t, newcomerClone, "push")
	var synchronized pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, reviewPath+"/synchronize", newcomer.API, "", http.StatusOK, &synchronized)
	if synchronized.SourceCommitID != followUpCommit {
		t.Fatalf("synchronized source = %s, want %s", synchronized.SourceCommitID, followUpCommit)
	}
	workflowJSON(t, server.URL, http.MethodPost, reviewPath+"/comments", newcomer.API, `{"body":"Added the requested test command."}`, http.StatusCreated, nil)

	workflowJSON(t, server.URL, http.MethodPut, reviewPath+"/reviews/me", maintainer.API, `{"decision":"approve"}`, http.StatusOK, nil)
	var readiness readinessResponse
	workflowJSON(t, server.URL, http.MethodGet, reviewPath+"/readiness", maintainer.API, "", http.StatusOK, &readiness)
	if !readiness.Ready || !readiness.CanMerge || readiness.Reviews.StaleReviews != 0 {
		t.Fatalf("readiness = %#v", readiness)
	}
	var merged pullrequests.PullRequest
	workflowJSON(t, server.URL, http.MethodPost, reviewPath+"/merge", maintainer.API, "", http.StatusOK, &merged)
	if merged.Status != pullrequests.Merged || merged.MergedByID != maintainer.ID {
		t.Fatalf("merged pull request = %#v", merged)
	}

	verified := gitClone(t, server.URL+"/repositories/"+repository.ID)
	assertFile(t, filepath.Join(verified, "CONTRIBUTING.md"), "# Contributing\n\nOpen a proposal first, then run `go test ./...`.\n", 0)
	var closed proposals.Proposal
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/proposals/"+proposal.ID, "", "", http.StatusOK, &closed)
	if closed.State != proposals.Closed || closed.ClosedByID != maintainer.ID {
		t.Fatalf("closed proposal = %#v", closed)
	}
	var activity struct {
		Items []activities.Event `json:"items"`
		Total int                `json:"total_count"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/repositories/"+repository.ID+"/activity?per_page=100", "", "", http.StatusOK, &activity)
	wantTypes := map[string]bool{"access.granted": false, "proposal.created": false, "pull_request.created": false, "pull_request.synchronized": false, "review.submitted": false, "review.replaced": false, "pull_request.merged": false, "mention.created": false}
	for _, event := range activity.Items {
		if _, wanted := wantTypes[event.Type]; wanted {
			wantTypes[event.Type] = true
		}
		if event.ActorID == "" || event.Resource.ID == "" || event.RepositoryID != repository.ID {
			t.Fatalf("unattributed activity = %#v", event)
		}
		if event.Type == "mention.created" && event.TargetUserID != newcomer.ID {
			t.Fatalf("mention = %#v", event)
		}
	}
	for eventType, found := range wantTypes {
		if !found {
			t.Errorf("missing %s in %d activity events", eventType, activity.Total)
		}
	}
	var newcomerInbox struct {
		Items []inboxItem `json:"items"`
		Total int         `json:"total_count"`
	}
	workflowJSON(t, server.URL, http.MethodGet, "/inbox?per_page=100", newcomer.API, "", http.StatusOK, &newcomerInbox)
	classifications := map[string]bool{}
	for _, item := range newcomerInbox.Items {
		classifications[item.Classification] = true
		if item.Href == "" || item.RepositoryID != repository.ID {
			t.Fatalf("non-actionable inbox item = %#v", item)
		}
	}
	if !classifications["awareness"] {
		t.Fatalf("newcomer inbox classifications = %#v", classifications)
	}
	if len(newcomerInbox.Items) == 0 {
		t.Fatal("newcomer inbox is empty")
	}
	cleared := newcomerInbox.Items[0].ID
	workflowJSON(t, server.URL, http.MethodDelete, "/inbox/"+cleared, newcomer.API, "", http.StatusNoContent, nil)
	workflowJSON(t, server.URL, http.MethodGet, "/inbox?per_page=100", newcomer.API, "", http.StatusOK, &newcomerInbox)
	for _, item := range newcomerInbox.Items {
		if item.ID == cleared {
			t.Fatalf("cleared inbox item %s returned", cleared)
		}
	}
}

func workflowRequest(t *testing.T, origin, method, path, bearer, body string, cookie *http.Cookie) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, origin+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func workflowJSON(t *testing.T, origin, method, path, bearer, body string, want int, output any) {
	t.Helper()
	response := workflowRequest(t, origin, method, path, bearer, body, nil)
	defer response.Body.Close()
	if response.StatusCode != want {
		contents, _ := io.ReadAll(response.Body)
		t.Fatalf("%s %s = %d, want %d: %s", method, path, response.StatusCode, want, contents)
	}
	if output != nil && json.NewDecoder(response.Body).Decode(output) != nil {
		t.Fatalf("decode %s %s", method, path)
	}
}
