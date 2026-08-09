package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	owned "github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
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

func TestEmbargoedBranchesRequireExactScopedVisibility(t *testing.T) {
	requireGit(t)
	store, _ := storage.New(t.TempDir())
	repository, _ := store.Create()
	tree := writeObject(t, repository, storage.TreeObject, nil)
	commit := writeCommit(t, repository, tree, nil, "private repair")
	public := "refs/heads/trunk"
	private := "refs/heads/embargo/4f5c9a"
	if err := repository.CreateReference(storage.Reference{Name: storage.ReferenceName(public), ObjectID: commit}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateReference(storage.Reference{Name: storage.ReferenceName(private), ObjectID: commit}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	ordinary, err := runGitServiceWithVisibility(request, repository, uploadPackService, "", "--advertise-refs")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ordinary, []byte(private)) {
		t.Fatalf("ordinary advertisement leaked embargo ref: %q", ordinary)
	}
	if !bytes.Contains(ordinary, []byte(public)) {
		t.Fatalf("ordinary advertisement omitted public branch: %q", ordinary)
	}
	scoped, err := runGitServiceWithVisibility(request, repository, uploadPackService, private, "--advertise-refs")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(scoped, []byte(private)) {
		t.Fatalf("scoped advertisement omitted exact embargo ref: %q", scoped)
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

func TestGitPushCompletesBranchLifecycle(t *testing.T) {
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

	workingCopy := filepath.Join(t.TempDir(), "publisher")
	gitCommand(t, "", "init", "--initial-branch=trunk", workingCopy)
	gitOutput(t, workingCopy, "config", "user.name", "Ada Lovelace")
	gitOutput(t, workingCopy, "config", "user.email", "ada@example.com")
	gitOutput(t, workingCopy, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(workingCopy, "README.md"), []byte("# Project\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "add", "README.md")
	gitOutput(t, workingCopy, "commit", "-m", "Start project")
	firstCommit := gitOutput(t, workingCopy, "rev-parse", "HEAD")
	gitOutput(t, workingCopy, "push", "--set-upstream", "origin", "trunk")
	if remoteHead := gitOutput(t, repository.GitDir(), "rev-parse", "refs/heads/trunk"); remoteHead != firstCommit {
		t.Fatalf("initial pushed head = %s, want %s", remoteHead, firstCommit)
	}

	if err := os.WriteFile(filepath.Join(workingCopy, "README.md"), []byte("# Project\n\nReady to collaborate.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "commit", "-am", "Invite collaborators")
	secondCommit := gitOutput(t, workingCopy, "rev-parse", "HEAD")
	gitOutput(t, workingCopy, "push", "origin", "trunk")
	if remoteHead := gitOutput(t, repository.GitDir(), "rev-parse", "refs/heads/trunk"); remoteHead != secondCommit {
		t.Fatalf("fast-forward pushed head = %s, want %s", remoteHead, secondCommit)
	}

	gitOutput(t, workingCopy, "reset", "--hard", firstCommit)
	if err := os.WriteFile(filepath.Join(workingCopy, "README.md"), []byte("# Rewritten project\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "commit", "-am", "Rewrite published history")
	replacementCommit := gitOutput(t, workingCopy, "rev-parse", "HEAD")
	gitFails(t, workingCopy, "push", "origin", "trunk")
	if remoteHead := gitOutput(t, repository.GitDir(), "rev-parse", "refs/heads/trunk"); remoteHead != secondCommit {
		t.Fatalf("ordinary non-fast-forward push changed head to %s, want %s", remoteHead, secondCommit)
	}
	gitOutput(t, workingCopy, "push", "--force", "origin", "trunk")
	if remoteHead := gitOutput(t, repository.GitDir(), "rev-parse", "refs/heads/trunk"); remoteHead != replacementCommit {
		t.Fatalf("force-pushed head = %s, want %s", remoteHead, replacementCommit)
	}
	if advertised := gitLsRemote(t, remote, remote, "refs/heads/trunk"); advertised != replacementCommit+"\trefs/heads/trunk" {
		t.Fatalf("force-updated advertisement = %q", advertised)
	}
	replacementClone := gitClone(t, remote)
	if head := gitOutput(t, replacementClone, "rev-parse", "HEAD"); head != replacementCommit {
		t.Fatalf("clone after force push has head %s, want %s", head, replacementCommit)
	}

	if err := os.WriteFile(filepath.Join(workingCopy, "CONTRIBUTING.md"), []byte("Send changes.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "add", "CONTRIBUTING.md")
	gitOutput(t, workingCopy, "commit", "-m", "Document contributions")
	advancedCommit := gitOutput(t, workingCopy, "rev-parse", "HEAD")
	gitOutput(t, workingCopy, "push", "origin", "trunk")
	gitOutput(t, replacementClone, "pull", "--ff-only")
	if head := gitOutput(t, replacementClone, "rev-parse", "HEAD"); head != advancedCommit {
		t.Fatalf("pull after force push has head %s, want %s", head, advancedCommit)
	}

	gitOutput(t, workingCopy, "switch", "-c", "topic")
	if err := os.WriteFile(filepath.Join(workingCopy, "TOPIC.md"), []byte("candidate work\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, workingCopy, "add", "TOPIC.md")
	gitOutput(t, workingCopy, "commit", "-m", "Publish topic")
	topicCommit := gitOutput(t, workingCopy, "rev-parse", "HEAD")
	gitOutput(t, workingCopy, "push", "origin", "topic")
	if remoteTopic := gitOutput(t, repository.GitDir(), "rev-parse", "refs/heads/topic"); remoteTopic != topicCommit {
		t.Fatalf("pushed topic head = %s, want %s", remoteTopic, topicCommit)
	}
	gitOutput(t, workingCopy, "push", "origin", ":topic")
	gitFails(t, repository.GitDir(), "rev-parse", "--verify", "refs/heads/topic")
	gitOutput(t, workingCopy, "tag", "-a", "candidate-release", "-m", "Candidate release")
	tagObject := gitOutput(t, workingCopy, "rev-parse", "candidate-release^{tag}")
	gitFails(t, workingCopy, "push", "origin", "refs/tags/candidate-release")
	gitFails(t, repository.GitDir(), "cat-file", "-e", tagObject)

	gitOutput(t, workingCopy, "switch", "trunk")
	gitOutput(t, workingCopy, "push", "origin", ":trunk")
	if advertised := gitLsRemote(t, remote, "--symref", remote); advertised != "" {
		t.Fatalf("deleted repository advertisement = %q, want empty", advertised)
	}
	gitFails(t, repository.GitDir(), "rev-parse", "--verify", "refs/heads/trunk")
	emptyClone := gitClone(t, remote)
	if branch := gitOutput(t, emptyClone, "symbolic-ref", "--short", "HEAD"); branch != "trunk" {
		t.Fatalf("clone after deletion selected branch %q, want trunk", branch)
	}

	gitOutput(t, workingCopy, "push", "origin", "trunk")
	gitOutput(t, emptyClone, "pull", "origin", "trunk")
	if head := gitOutput(t, emptyClone, "rev-parse", "HEAD"); head != advancedCommit {
		t.Fatalf("pull after branch recovery has head %s, want %s", head, advancedCommit)
	}
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

func TestGitHTTPRequiresTheServiceSpecificScopeAndHonorsRevocation(t *testing.T) {
	repositoryStorage, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := repositoryStorage.Create()
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := credentials.Issue("actor", "read-only clone", auth.Git, []auth.Scope{auth.GitRead}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerGitHTTP(mux, testGitStore{RepositoryStore: repositoryStorage, owner: "actor", visibility: owned.Private}, credentials)
	path := "/repositories/" + string(repository.ID()) + "/info/refs"
	request := func(service string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path+"?service="+service, nil)
		if authenticated {
			r.SetBasicAuth("git", issued.Token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if response := request(uploadPackService, false); response.Code != http.StatusUnauthorized || response.Header().Get("WWW-Authenticate") == "" {
		t.Fatalf("anonymous response = %d, %q", response.Code, response.Header().Get("WWW-Authenticate"))
	}
	if response := request(uploadPackService, true); response.Code != http.StatusOK {
		t.Fatalf("read response = %d: %s", response.Code, response.Body.String())
	}
	if response := request(receivePackService, true); response.Code != http.StatusUnauthorized {
		t.Fatalf("write response with read token = %d", response.Code)
	}
	if _, err := credentials.Revoke("actor", issued.ID); err != nil {
		t.Fatal(err)
	}
	if response := request(uploadPackService, true); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked response = %d", response.Code)
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
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := credentials.Issue("test-user", "Git test", auth.Git, []auth.Scope{auth.GitRead, auth.GitWrite}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerGitHTTP(mux, testGitStore{RepositoryStore: store, owner: "test-user", visibility: owned.Private}, credentials)
	server := httptest.NewServer(mux)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword("git", issued.Token)
	server.URL = parsed.String()
	t.Cleanup(server.Close)
	return server
}

type testGitStore struct {
	storage.RepositoryStore
	owner      string
	visibility owned.Visibility
}

func (s testGitStore) Inspect(id storage.ID) (owned.Repository, error) {
	repository, err := s.RepositoryStore.Open(id)
	if err != nil {
		return owned.Repository{}, err
	}
	info, err := repository.Inspect()
	if err != nil {
		return owned.Repository{}, err
	}
	return owned.Repository{ID: id, OwnerID: s.owner, Visibility: s.visibility, Empty: info.Empty}, nil
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

func gitCommand(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
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

func TestReceiveHookLimitsWorkerCredentialToWorkingBranch(t *testing.T) {
	directory, err := receiveHooks(true, "refs/heads/main", "refs/heads/agent/task")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	hook := filepath.Join(directory, "pre-receive")
	allowed := exec.Command(hook)
	allowed.Stdin = strings.NewReader(strings.Repeat("0", 40) + " " + strings.Repeat("1", 40) + " refs/heads/agent/task\n")
	if output, err := allowed.CombinedOutput(); err != nil {
		t.Fatalf("allowed branch rejected: %v: %s", err, output)
	}
	denied := exec.Command(hook)
	denied.Stdin = strings.NewReader(strings.Repeat("0", 40) + " " + strings.Repeat("1", 40) + " refs/heads/main\n")
	if output, err := denied.CombinedOutput(); err == nil || !strings.Contains(string(output), "credential is limited") {
		t.Fatalf("other branch accepted: %v: %s", err, output)
	}
}
