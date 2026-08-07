package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestOwnerAdmitsContributorWhoPublishesCandidateBranch(t *testing.T) {
	requireGit(t)
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	outsider, _ := userStore.Create(users.Profile{Handle: "outsider", DisplayName: "Outsider"})
	repository, err := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	if err != nil {
		t.Fatal(err)
	}
	ownerAPI := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	contributorAPI := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, string(owner.ID), auth.Git, auth.GitRead, auth.GitWrite)
	contributorGit := issueAccess(t, credentials, string(contributor.ID), auth.Git, auth.GitRead, auth.GitWrite)
	outsiderGit := issueAccess(t, credentials, string(outsider.ID), auth.Git, auth.GitRead, auth.GitWrite)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerCollaboratorsHTTP(mux, catalog, userStore, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	request, _ := http.NewRequest(http.MethodPut, server.URL+"/repositories/"+string(repository.ID)+"/collaborators/"+string(contributor.ID), nil)
	request.Header.Set("Authorization", "Bearer "+ownerAPI)
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("admit contributor: status %v, error %v", response.StatusCode, err)
	}
	response.Body.Close()

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/repositories?affiliation=all", nil)
	request.Header.Set("Authorization", "Bearer "+contributorAPI)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var workspace struct {
		Items []map[string]any `json:"items"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&workspace) != nil {
		t.Fatalf("contributor workspace status = %d", response.StatusCode)
	}
	response.Body.Close()
	if len(workspace.Items) != 1 || workspace.Items[0]["id"] != string(repository.ID) {
		t.Fatalf("contributor workspace = %#v", workspace.Items)
	}

	get := func(token string) int {
		r, _ := http.NewRequest(http.MethodGet, server.URL+"/repositories/"+string(repository.ID), nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if got := get(contributorAPI); got != http.StatusOK {
		t.Fatalf("contributor API read = %d", got)
	}
	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/repositories/"+string(repository.ID), strings.NewReader(`{"visibility":"public"}`))
	request.Header.Set("Authorization", "Bearer "+contributorAPI)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("contributor administration = %d", response.StatusCode)
	}

	remoteWith := func(token string) string {
		remote, _ := url.Parse(server.URL + "/repositories/" + string(repository.ID))
		remote.User = url.UserPassword("git", token)
		return remote.String()
	}
	ownerClone := gitClone(t, remoteWith(ownerGit))
	gitOutput(t, ownerClone, "config", "user.name", "Owner")
	gitOutput(t, ownerClone, "config", "user.email", "owner@example.com")
	if err := os.WriteFile(filepath.Join(ownerClone, "README.md"), []byte("maintained\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, ownerClone, "add", "README.md")
	gitOutput(t, ownerClone, "commit", "-m", "Establish main")
	mainCommit := gitOutput(t, ownerClone, "rev-parse", "HEAD")
	gitOutput(t, ownerClone, "push", "origin", "main")

	contributorClone := gitClone(t, remoteWith(contributorGit))
	gitOutput(t, contributorClone, "config", "user.name", "Contributor")
	gitOutput(t, contributorClone, "config", "user.email", "contributor@example.com")
	gitOutput(t, contributorClone, "switch", "-c", "candidate/contributor")
	if err := os.WriteFile(filepath.Join(contributorClone, "candidate.txt"), []byte("proposal\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, contributorClone, "add", "candidate.txt")
	gitOutput(t, contributorClone, "commit", "-m", "Propose candidate")
	candidateCommit := gitOutput(t, contributorClone, "rev-parse", "HEAD")
	gitOutput(t, contributorClone, "push", "origin", "candidate/contributor")
	if advertised := gitLsRemote(t, remoteWith(contributorGit), remoteWith(contributorGit), "refs/heads/candidate/contributor"); advertised != candidateCommit+"\trefs/heads/candidate/contributor" {
		t.Fatalf("candidate advertisement = %q", advertised)
	}
	gitFails(t, contributorClone, "push", "origin", "HEAD:main", "--force")
	assertRemoteBranch(t, remoteWith(ownerGit), mainCommit)
	gitFails(t, t.TempDir(), "ls-remote", remoteWith(outsiderGit))

	request, _ = http.NewRequest(http.MethodDelete, server.URL+"/repositories/"+string(repository.ID)+"/collaborators/"+string(contributor.ID), nil)
	request.Header.Set("Authorization", "Bearer "+ownerAPI)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("remove contributor = %d", response.StatusCode)
	}
	gitFails(t, contributorClone, "fetch", "origin")
}
