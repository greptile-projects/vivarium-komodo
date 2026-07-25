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
	store, err := storage.New(filepath.Join(t.TempDir(), "repositories"))
	if err != nil {
		t.Fatal(err)
	}

	created, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if len(created.ID()) != 32 {
		t.Fatalf("ID length = %d, want 32", len(created.ID()))
	}

	info, err := created.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != created.ID() || !info.Bare || info.DefaultBranch != "main" {
		t.Fatalf("unexpected repository info: %+v", info)
	}

	reopened, err := store.Open(created.ID())
	if err != nil {
		t.Fatal(err)
	}
	reopenedInfo, err := reopened.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if reopenedInfo != info {
		t.Fatalf("reopened info = %+v, want %+v", reopenedInfo, info)
	}

	assertGitOutput(t, info.Path, "rev-parse", "--is-bare-repository", "true")
	assertGitOutput(t, info.Path, "symbolic-ref", "HEAD", "refs/heads/main")
	assertGitSuccess(t, info.Path, "fsck", "--full")
}

func TestOpenRejectsMissingAndMalformedRepositories(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"../outside", strings.Repeat("0", 32)} {
		if _, err := store.Open(id); !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("Open(%q) error = %v, want ErrNotFound", id, err)
		}
	}

	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	info, err := repository.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info.Path, "HEAD"), []byte("invalid\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(repository.ID()); !errors.Is(err, storage.ErrInvalidRepository) {
		t.Fatalf("Open malformed repository error = %v, want ErrInvalidRepository", err)
	}
}

func assertGitSuccess(t *testing.T, repositoryPath string, arguments ...string) {
	t.Helper()
	commandArguments := append([]string{"--git-dir", repositoryPath}, arguments...)
	if output, err := exec.Command("git", commandArguments...).CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", arguments[0], err, output)
	}
}

func assertGitOutput(t *testing.T, repositoryPath string, argument string, arguments ...string) {
	t.Helper()
	want := arguments[len(arguments)-1]
	arguments = arguments[:len(arguments)-1]

	commandArguments := append([]string{"--git-dir", repositoryPath, argument}, arguments...)
	output, err := exec.Command("git", commandArguments...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", argument, err, output)
	}
	if got := strings.TrimSpace(string(output)); got != want {
		t.Fatalf("git %s output = %q, want %q", argument, got, want)
	}
}
