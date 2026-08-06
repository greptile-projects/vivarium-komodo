package storage_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestWriteAndReadGitObjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	var objects storage.ObjectStore = repository

	blobContent := []byte("hello from storage\n")
	blobID := writeAndAssertObject(t, objects, repository.GitDir(), storage.BlobObject, blobContent)

	treeContent := append([]byte("100644 greeting.txt\x00"), mustDecodeObjectID(t, blobID)...)
	treeID := writeAndAssertObject(t, objects, repository.GitDir(), storage.TreeObject, treeContent)

	commitContent := []byte("tree " + string(treeID) + "\n" +
		"author Ada Lovelace <ada@example.com> 0 +0000\n" +
		"committer Ada Lovelace <ada@example.com> 0 +0000\n\n" +
		"Create greeting\n")
	commitID := writeAndAssertObject(t, objects, repository.GitDir(), storage.CommitObject, commitContent)

	tagContent := []byte("object " + string(commitID) + "\n" +
		"type commit\n" +
		"tag v1.0.0\n" +
		"tagger Ada Lovelace <ada@example.com> 0 +0000\n\n" +
		"First release\n")
	writeAndAssertObject(t, objects, repository.GitDir(), storage.TagObject, tagContent)
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "fsck", "--full")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fsck failed: %v\n%s", err, output)
	}

	info, err := repository.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if info.Empty {
		t.Fatal("repository with objects reported as empty")
	}
}

func TestObjectErrorsAndIdempotentWrites(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	first, err := repository.WriteObject(storage.BlobObject, []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.WriteObject(storage.BlobObject, []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identical writes returned %s and %s", first, second)
	}
	if _, err := repository.WriteObject("note", nil); !errors.Is(err, storage.ErrInvalidObjectType) {
		t.Fatalf("WriteObject returned %v, want ErrInvalidObjectType", err)
	}
	if _, err := repository.ReadObject("not-an-object-id"); !errors.Is(err, storage.ErrInvalidObjectID) {
		t.Fatalf("ReadObject returned %v, want ErrInvalidObjectID", err)
	}
	if _, err := repository.ReadObject("0000000000000000000000000000000000000000"); !errors.Is(err, storage.ErrObjectNotFound) {
		t.Fatalf("ReadObject returned %v, want ErrObjectNotFound", err)
	}
}

func writeAndAssertObject(t *testing.T, objects storage.ObjectStore, gitDir string, objectType storage.ObjectType, content []byte) storage.ObjectID {
	t.Helper()
	id, err := objects.WriteObject(objectType, content)
	if err != nil {
		t.Fatal(err)
	}
	object, err := objects.ReadObject(id)
	if err != nil {
		t.Fatal(err)
	}
	if object.ID != id || object.Type != objectType || !bytes.Equal(object.Content, content) {
		t.Fatalf("object changed during round trip: %+v", object)
	}

	assertGitOutput(t, gitDir, string(objectType), "cat-file", "-t", string(id))
	assertGitOutputBytes(t, gitDir, content, "cat-file", string(objectType), string(id))
	return id
}

func mustDecodeObjectID(t *testing.T, id storage.ObjectID) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(string(id))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestRepositoryLifecycle(t *testing.T) {
	root := t.TempDir()
	concreteStore, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var store storage.RepositoryStore = concreteStore

	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if repository.ID() == "" {
		t.Fatal("created repository has no ID")
	}

	info, err := repository.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != repository.ID() || !info.Bare || !info.Empty {
		t.Fatalf("unexpected repository info: %+v", info)
	}

	reopenedStore, err := storage.New(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := reopenedStore.Open(repository.ID())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ID() != repository.ID() || reopened.GitDir() != repository.GitDir() {
		t.Fatalf("reopened a different repository: %#v", reopened)
	}
}

func TestCreatedRepositoryIsRecognizedByGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	assertGitOutput(t, repository.GitDir(), "true", "rev-parse", "--is-bare-repository")
	assertGitOutput(t, repository.GitDir(), "refs/heads/main", "symbolic-ref", "HEAD")
	command := exec.Command("git", "--git-dir="+repository.GitDir(), "fsck", "--full")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git fsck failed: %v\n%s", err, output)
	}
}

func TestOpenRejectsUnknownInvalidAndMalformedRepositories(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Open("../outside"); !errors.Is(err, storage.ErrInvalidID) {
		t.Fatalf("Open returned %v, want ErrInvalidID", err)
	}
	if _, err := store.Open("00000000-0000-4000-8000-000000000000"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Open returned %v, want ErrNotFound", err)
	}

	id := storage.ID("11111111-1111-4111-8111-111111111111")
	malformedRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(malformedRoot, string(id)), 0o750); err != nil {
		t.Fatal(err)
	}
	malformedStore, err := storage.New(malformedRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := malformedStore.Open(id); !errors.Is(err, storage.ErrInvalidRepository) {
		t.Fatalf("Open returned %v, want ErrInvalidRepository", err)
	}
}

func assertGitOutput(t *testing.T, gitDir, expected string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"--git-dir=" + gitDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if actual := strings.TrimSpace(string(output)); actual != expected {
		t.Fatalf("git %s returned %q, want %q", strings.Join(args, " "), actual, expected)
	}
}

func assertGitOutputBytes(t *testing.T, gitDir string, expected []byte, args ...string) {
	t.Helper()
	commandArgs := append([]string{"--git-dir=" + gitDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if !bytes.Equal(output, expected) {
		t.Fatalf("git %s returned %q, want %q", strings.Join(args, " "), output, expected)
	}
}
