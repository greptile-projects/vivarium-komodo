package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestGitCloneSupportsEmptyRepositoryAndPrimaryBranch(t *testing.T) {
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

	clone := gitClone(t, server.URL+"/repositories/"+string(repository.ID()))
	if branch := gitOutput(t, clone, "symbolic-ref", "--short", "HEAD"); branch != "trunk" {
		t.Fatalf("cloned empty repository branch = %q, want trunk", branch)
	}
	if status := gitOutput(t, clone, "status", "--short", "--branch"); status != "## No commits yet on trunk...origin/trunk [gone]" {
		t.Fatalf("unexpected empty clone status: %q", status)
	}
}

func TestGitCloneChecksOutPrimaryBranchWithReachableHistoryAndFiles(t *testing.T) {
	requireGit(t)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	readmeID := writeObject(t, repository, storage.BlobObject, []byte("# First version\n"))
	scriptID := writeObject(t, repository, storage.BlobObject, []byte("#!/bin/sh\necho hello\n"))
	toolsTreeID := writeObject(t, repository, storage.TreeObject, treeEntry("100755", "hello.sh", scriptID))
	firstTreeID := writeObject(t, repository, storage.TreeObject, append(
		treeEntry("100644", "README.md", readmeID),
		treeEntry("40000", "tools", toolsTreeID)...,
	))
	firstCommitID := writeCommit(t, repository, firstTreeID, nil, "Initial snapshot")

	readmeID = writeObject(t, repository, storage.BlobObject, []byte("# Current version\n\nReady to collaborate.\n"))
	secondTreeID := writeObject(t, repository, storage.TreeObject, append(
		treeEntry("100644", "README.md", readmeID),
		treeEntry("40000", "tools", toolsTreeID)...,
	))
	secondCommitID := writeCommit(t, repository, secondTreeID, []storage.ObjectID{firstCommitID}, "Prepare working copy")
	if err := repository.CreateReference(storage.Reference{Name: "refs/heads/trunk", ObjectID: secondCommitID}); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetDefaultBranch("refs/heads/trunk"); err != nil {
		t.Fatal(err)
	}
	server := gitServer(t, store)

	clone := gitClone(t, server.URL+"/repositories/"+string(repository.ID()))
	if branch := gitOutput(t, clone, "branch", "--show-current"); branch != "trunk" {
		t.Fatalf("checked-out branch = %q, want trunk", branch)
	}
	if head := gitOutput(t, clone, "rev-parse", "HEAD"); head != string(secondCommitID) {
		t.Fatalf("clone HEAD = %s, want %s", head, secondCommitID)
	}
	if history := gitOutput(t, clone, "log", "--format=%s"); history != "Prepare working copy\nInitial snapshot" {
		t.Fatalf("unexpected cloned history:\n%s", history)
	}
	assertFile(t, filepath.Join(clone, "README.md"), "# Current version\n\nReady to collaborate.\n", 0)
	assertFile(t, filepath.Join(clone, "tools", "hello.sh"), "#!/bin/sh\necho hello\n", 0o111)

	serverObjects := gitOutput(t, repository.GitDir(), "rev-list", "--objects", "--all")
	cloneObjects := gitOutput(t, clone, "rev-list", "--objects", "--all")
	if cloneObjects != serverObjects {
		t.Fatalf("clone reachable objects differ from server:\nclone:\n%s\nserver:\n%s", cloneObjects, serverObjects)
	}
}

func TestGitFetchAndPullAdvanceExistingWorkingCopy(t *testing.T) {
	requireGit(t)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	readmeID := writeObject(t, repository, storage.BlobObject, []byte("# Project\n"))
	firstTreeID := writeObject(t, repository, storage.TreeObject, treeEntry("100644", "README.md", readmeID))
	firstCommitID := writeCommit(t, repository, firstTreeID, nil, "Start project")
	if err := repository.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: firstCommitID}); err != nil {
		t.Fatal(err)
	}
	server := gitServer(t, store)
	clone := gitClone(t, server.URL+"/repositories/"+string(repository.ID()))

	guideID := writeObject(t, repository, storage.BlobObject, []byte("Collaborate here.\n"))
	secondTreeID := writeObject(t, repository, storage.TreeObject, append(
		treeEntry("100644", "README.md", readmeID),
		treeEntry("100644", "CONTRIBUTING.md", guideID)...,
	))
	secondCommitID := writeCommit(t, repository, secondTreeID, []storage.ObjectID{firstCommitID}, "Add contributor guide")
	if err := repository.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: secondCommitID}); err != nil {
		t.Fatal(err)
	}

	gitFails(t, clone, "cat-file", "-e", string(secondCommitID)+"^{commit}")
	gitOutput(t, clone, "fetch", "origin")
	if head := gitOutput(t, clone, "rev-parse", "HEAD"); head != string(firstCommitID) {
		t.Fatalf("fetch changed checked-out HEAD to %s, want %s", head, firstCommitID)
	}
	if remoteHead := gitOutput(t, clone, "rev-parse", "origin/main"); remoteHead != string(secondCommitID) {
		t.Fatalf("fetched origin/main = %s, want %s", remoteHead, secondCommitID)
	}
	gitOutput(t, clone, "cat-file", "-e", string(secondCommitID)+"^{commit}")
	assertFile(t, filepath.Join(clone, "README.md"), "# Project\n", 0)
	if _, err := os.Stat(filepath.Join(clone, "CONTRIBUTING.md")); !os.IsNotExist(err) {
		t.Fatalf("fetch unexpectedly updated working tree: %v", err)
	}

	readmeID = writeObject(t, repository, storage.BlobObject, []byte("# Project\n\nReady for contributors.\n"))
	thirdTreeID := writeObject(t, repository, storage.TreeObject, append(
		treeEntry("100644", "README.md", readmeID),
		treeEntry("100644", "CONTRIBUTING.md", guideID)...,
	))
	thirdCommitID := writeCommit(t, repository, thirdTreeID, []storage.ObjectID{secondCommitID}, "Welcome contributors")
	if err := repository.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: thirdCommitID}); err != nil {
		t.Fatal(err)
	}

	gitOutput(t, clone, "pull", "--ff-only")
	if head := gitOutput(t, clone, "rev-parse", "HEAD"); head != string(thirdCommitID) {
		t.Fatalf("pulled HEAD = %s, want %s", head, thirdCommitID)
	}
	if history := gitOutput(t, clone, "log", "--format=%s"); history != "Welcome contributors\nAdd contributor guide\nStart project" {
		t.Fatalf("unexpected pulled history:\n%s", history)
	}
	assertFile(t, filepath.Join(clone, "README.md"), "# Project\n\nReady for contributors.\n", 0)
	assertFile(t, filepath.Join(clone, "CONTRIBUTING.md"), "Collaborate here.\n", 0)
}

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

func gitClone(t *testing.T, remote string) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "clone")
	command := exec.Command("git", "clone", remote, destination)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git clone %s failed: %v\n%s", remote, err, output)
	}
	return destination
}

func gitOutput(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func gitFails(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("git %s unexpectedly succeeded:\n%s", strings.Join(arguments, " "), output)
	}
}

func writeObject(t *testing.T, repository *storage.Repository, objectType storage.ObjectType, content []byte) storage.ObjectID {
	t.Helper()
	id, err := repository.WriteObject(objectType, content)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func treeEntry(mode, name string, id storage.ObjectID) []byte {
	entry := append([]byte(mode+" "+name+"\x00"), decodeObjectID(id)...)
	return entry
}

func decodeObjectID(id storage.ObjectID) []byte {
	decoded, err := hex.DecodeString(string(id))
	if err != nil {
		panic(err)
	}
	return decoded
}

func writeCommit(t *testing.T, repository *storage.Repository, tree storage.ObjectID, parents []storage.ObjectID, message string) storage.ObjectID {
	t.Helper()
	var content strings.Builder
	fmt.Fprintf(&content, "tree %s\n", tree)
	for _, parent := range parents {
		fmt.Fprintf(&content, "parent %s\n", parent)
	}
	content.WriteString("author Ada Lovelace <ada@example.com> 0 +0000\n")
	content.WriteString("committer Ada Lovelace <ada@example.com> 0 +0000\n\n")
	content.WriteString(message + "\n")
	return writeObject(t, repository, storage.CommitObject, []byte(content.String()))
}

func assertFile(t *testing.T, path, want string, executable os.FileMode) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contents, []byte(want)) {
		t.Fatalf("%s contents = %q, want %q", path, contents, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 != executable {
		t.Fatalf("%s executable mode = %03o, want %03o", path, info.Mode()&0o111, executable)
	}
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
