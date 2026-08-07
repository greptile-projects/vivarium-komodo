package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestPullRequestExposesSnapshottedCommitsFilesAndDiscussion(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	if _, err := catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID)); err != nil {
		t.Fatal(err)
	}
	opened, _ := catalog.Open(repository.ID)
	oldReadme, _ := opened.WriteObject(storage.BlobObject, []byte("old setup\n"))
	oldTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"README.md": oldReadme}))
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(oldTree)+"\nauthor Base Author <base@example.com> 1 +0000\ncommitter Base Author <base@example.com> 1 +0000\n\nbase\n"))
	newReadme, _ := opened.WriteObject(storage.BlobObject, []byte("new setup\nwith details\n"))
	script, _ := opened.WriteObject(storage.BlobObject, []byte("#!/bin/sh\necho ready\n"))
	newTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"README.md": newReadme, "setup.sh": script}))
	change, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(newTree)+"\nparent "+string(base)+"\nauthor Contributor <c@example.com> 2 +0000\ncommitter Contributor <c@example.com> 2 +0000\n\nexplain setup\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: change})
	created, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: string(contributor.ID), Title: "Improve setup", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(change), TargetCommitID: string(base)})
	token := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	baseURL := server.URL + "/repositories/" + string(repository.ID) + "/pull-requests/" + created.ID

	response, _ := http.Get(baseURL + "/commits")
	var commits struct {
		Items []struct {
			ID, Message, Author string
			ParentIDs           []string `json:"parent_ids"`
		}
		Total int `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&commits)
	response.Body.Close()
	if response.StatusCode != 200 || commits.Total != 1 || commits.Items[0].ID != string(change) || commits.Items[0].Message != "explain setup" || commits.Items[0].Author == "" {
		t.Fatalf("commits = %#v, status %d", commits, response.StatusCode)
	}
	response, _ = http.Get(baseURL + "/files")
	var files struct {
		Items []struct {
			Path, Status, Patch  string
			Additions, Deletions int
			Binary               bool
		}
		Total int `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&files)
	response.Body.Close()
	if response.StatusCode != 200 || files.Total != 2 || files.Items[0].Path != "README.md" || files.Items[0].Status != "modified" || files.Items[0].Additions != 2 || files.Items[0].Deletions != 1 || !strings.Contains(files.Items[0].Patch, "+with details") || files.Items[1].Status != "added" {
		t.Fatalf("files = %#v, status %d", files, response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/comments", strings.NewReader(`{"body":"The extra detail is important."}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var comment pullrequests.Comment
	json.NewDecoder(response.Body).Decode(&comment)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || comment.AuthorID != string(contributor.ID) || comment.PullRequestID != created.ID {
		t.Fatalf("comment = %#v, status %d", comment, response.StatusCode)
	}
	response, _ = http.Get(baseURL + "/comments")
	var comments struct {
		Items []pullrequests.Comment `json:"items"`
		Total int                    `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&comments)
	response.Body.Close()
	if response.StatusCode != 200 || comments.Total != 1 || comments.Items[0] != comment {
		t.Fatalf("comments = %#v, status %d", comments, response.StatusCode)
	}
}

func testTree(t *testing.T, entries map[string]storage.ObjectID) []byte {
	t.Helper()
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	var content []byte
	for _, name := range names {
		objectID, err := hex.DecodeString(string(entries[name]))
		if err != nil {
			t.Fatal(err)
		}
		mode := "100644"
		if strings.HasSuffix(name, ".sh") {
			mode = "100755"
		}
		content = append(content, []byte(mode+" "+name)...)
		content = append(content, 0)
		content = append(content, objectID...)
	}
	return content
}

func TestContributorOpensPullRequestAtExactBranchState(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	if _, err := catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID)); err != nil {
		t.Fatal(err)
	}
	opened, _ := catalog.Open(repository.ID)
	treeID, _ := opened.WriteObject(storage.TreeObject, nil)
	targetID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nbase\n"))
	sourceID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nparent "+string(targetID)+"\nauthor A <a@example.com> 2 +0000\ncommitter A <a@example.com> 2 +0000\n\nchange\n"))
	if err := opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: targetID}); err != nil {
		t.Fatal(err)
	}
	if err := opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: sourceID}); err != nil {
		t.Fatal(err)
	}
	proposal, _ := proposalStore.Create(string(repository.ID), string(contributor.ID), "Improve setup", "Make setup reproducible.")
	token := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/pull-requests"
	body := `{"proposal_id":"` + proposal.ID + `","title":"Automate setup","body":"Adds one reproducible command.","source_branch":"candidate","target_branch":"main"}`
	request, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var created pullrequests.PullRequest
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || created.AuthorID != string(contributor.ID) || created.ProposalID != proposal.ID || created.SourceCommitID != string(sourceID) || created.TargetCommitID != string(targetID) || created.Status != pullrequests.Open {
		t.Fatalf("created pull request = %#v, status %d", created, response.StatusCode)
	}
	if response.Header.Get("Location") != "/repositories/"+string(repository.ID)+"/pull-requests/"+created.ID {
		t.Fatalf("location = %q", response.Header.Get("Location"))
	}

	// Later branch movement does not rewrite the repository state represented by the request.
	newSourceID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nparent "+string(sourceID)+"\nauthor A <a@example.com> 3 +0000\ncommitter A <a@example.com> 3 +0000\n\nmore\n"))
	if err := opened.UpdateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: newSourceID}); err != nil {
		t.Fatal(err)
	}
	request, _ = http.NewRequest(http.MethodGet, base+"/"+created.ID, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var inspected pullrequests.PullRequest
	json.NewDecoder(response.Body).Decode(&inspected)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || inspected.SourceCommitID != string(sourceID) {
		t.Fatalf("inspected pull request = %#v, status %d", inspected, response.StatusCode)
	}
}

func TestPullRequestRejectsMissingOrNonCommitBranches(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	token := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", strings.NewReader(`{"title":"Change","source_branch":"missing","target_branch":"main"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, request)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "invalid_branches") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}
