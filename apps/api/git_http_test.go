package main

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestGitLsRemoteSupportsEmptyRepository(t *testing.T) {
	requireGit(t)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDefaultBranch("refs/heads/trunk"); err != nil {
		t.Fatal(err)
	}
	server := gitServer(t, store)

	remote := server.URL + "/repositories/" + string(repository.ID())
	output := gitLsRemote(t, remote, "--symref", remote, "HEAD")
	if output != "" {
		t.Fatalf("unexpected empty repository advertisement: %q", output)
	}
}

func TestGitLsRemoteAdvertisesPopulatedRepositoryAndDefaultBranch(t *testing.T) {
	requireGit(t)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	commitID := createAdvertisedCommit(t, repository)
	if err := repository.CreateReference(storage.Reference{Name: "refs/heads/trunk", ObjectID: commitID}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateReference(storage.Reference{Name: "refs/tags/v1", ObjectID: commitID}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDefaultBranch("refs/heads/trunk"); err != nil {
		t.Fatal(err)
	}
	server := gitServer(t, store)

	remote := server.URL + "/repositories/" + string(repository.ID())
	output := gitLsRemote(t, remote, "--symref", remote)
	want := "ref: refs/heads/trunk\tHEAD\n" + string(commitID) + "\tHEAD\n" +
		string(commitID) + "\trefs/heads/trunk\n" + string(commitID) + "\trefs/tags/v1"
	if output != want {
		t.Fatalf("unexpected populated repository advertisement:\n%s\nwant:\n%s", output, want)
	}
}

func gitServer(t *testing.T, store storage.RepositoryStore) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	registerGitHTTP(mux, store)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func gitLsRemote(t *testing.T, remote string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "protocol.version=2", "ls-remote"}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-remote %s failed: %v\n%s", remote, err, output)
	}
	return strings.TrimSpace(string(output))
}

func createAdvertisedCommit(t *testing.T, repository *storage.Repository) storage.ObjectID {
	t.Helper()
	treeID, err := repository.WriteObject(storage.TreeObject, nil)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("tree " + string(treeID) + "\n" +
		"author Ada Lovelace <ada@example.com> 0 +0000\n" +
		"committer Ada Lovelace <ada@example.com> 0 +0000\n\nInitial commit\n")
	commitID, err := repository.WriteObject(storage.CommitObject, content)
	if err != nil {
		t.Fatal(err)
	}
	return commitID
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}
