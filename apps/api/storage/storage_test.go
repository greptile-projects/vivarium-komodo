package storage_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

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
