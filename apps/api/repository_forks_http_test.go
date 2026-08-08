package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestReadableRepositoryCanBeForkedOwnedAndSynchronized(t *testing.T) {
	requireGit(t)
	storageRoot := t.TempDir()
	gitStorage, err := storage.New(storageRoot)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := repositories.New(t.TempDir(), gitStorage)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := catalog.Create("maintainer", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	if err != nil {
		t.Fatal(err)
	}
	upstreamStorage, _ := catalog.Open(upstream.ID)
	first := forkTestCommit(t, upstreamStorage, "first", "")
	if err := upstreamStorage.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: first}); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerRepositoryBrowserHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	apiGrant, _ := credentials.Issue("newcomer", "fork workflow", auth.API, []auth.Scope{auth.RepositoryRead, auth.RepositoryWrite}, time.Hour)

	response := forkRequest(t, server.URL+"/repositories/"+string(upstream.ID)+"/forks", apiGrant.Token, map[string]any{"name": "my-project"})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("fork status = %d", response.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	forkID := storage.ID(created["id"].(string))
	if created["owner_id"] != "newcomer" || created["upstream_repository_id"] != string(upstream.ID) || created["visibility"] != "private" {
		t.Fatalf("fork resource = %#v", created)
	}
	upstreamObject := filepath.Join(storageRoot, string(upstream.ID), "objects", string(first)[:2], string(first)[2:])
	forkObject := filepath.Join(storageRoot, string(forkID), "objects", string(first)[:2], string(first)[2:])
	upstreamInfo, _ := os.Stat(upstreamObject)
	forkInfo, _ := os.Stat(forkObject)
	if upstreamInfo == nil || forkInfo == nil || !os.SameFile(upstreamInfo, forkInfo) {
		t.Fatal("forked commit was not retained as one shared immutable object")
	}

	gitGrant, _ := credentials.Issue("newcomer", "fork Git", auth.Git, []auth.Scope{auth.GitRead, auth.GitWrite}, time.Hour)
	remote, _ := url.Parse(server.URL + "/repositories/" + string(forkID))
	remote.User = url.UserPassword("git", gitGrant.Token)
	workingCopy := filepath.Join(t.TempDir(), "fork")
	gitOutput(t, "", "clone", remote.String(), workingCopy)
	gitOutput(t, workingCopy, "checkout", "-b", "experiment")
	if err := os.WriteFile(filepath.Join(workingCopy, "idea.txt"), []byte("independent work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "add", "idea.txt")
	gitOutput(t, workingCopy, "-c", "user.name=Newcomer", "-c", "user.email=new@example.test", "commit", "-m", "experiment")
	gitOutput(t, workingCopy, "push", "origin", "experiment")

	second := forkTestCommit(t, upstreamStorage, "second", first)
	if err := upstreamStorage.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: second}); err != nil {
		t.Fatal(err)
	}
	response = forkRequest(t, server.URL+"/repositories/"+string(forkID)+"/sync", apiGrant.Token, map[string]any{"branch": "main"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("sync status = %d", response.StatusCode)
	}
	var synchronized map[string]any
	if err := json.NewDecoder(response.Body).Decode(&synchronized); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if synchronized["updated"] != true || synchronized["after_commit_id"] != string(second) {
		t.Fatalf("sync result = %#v", synchronized)
	}
	forkStorage, _ := catalog.Open(forkID)
	mainRef, _ := forkStorage.ReadReference("refs/heads/main")
	experimentRef, experimentErr := forkStorage.ReadReference("refs/heads/experiment")
	if mainRef.ObjectID != second || experimentErr != nil || experimentRef.ObjectID == "" {
		t.Fatalf("fork refs after sync: main=%#v experiment=%#v err=%v", mainRef, experimentRef, experimentErr)
	}
	if err := catalog.Delete("maintainer", upstream.ID); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, "", "--git-dir", filepath.Join(storageRoot, string(forkID)), "fsck", "--full")
}

func forkRequest(t *testing.T, endpoint, token string, body map[string]any) *http.Response {
	t.Helper()
	encoded, _ := json.Marshal(body)
	request, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(encoded))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func forkTestCommit(t *testing.T, repository storage.RepositoryStorage, message string, parent storage.ObjectID) storage.ObjectID {
	t.Helper()
	tree, err := repository.WriteObject(storage.TreeObject, nil)
	if err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("tree %s\n", tree)
	if parent != "" {
		content += fmt.Sprintf("parent %s\n", parent)
	}
	content += "author Test <test@example.test> 1700000000 +0000\ncommitter Test <test@example.test> 1700000000 +0000\n\n" + strings.TrimSpace(message) + "\n"
	commit, err := repository.WriteObject(storage.CommitObject, []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}
