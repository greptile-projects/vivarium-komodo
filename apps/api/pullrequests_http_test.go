package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

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
