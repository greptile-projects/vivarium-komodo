package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestStockGitCompletesSingleBranchWorkflow is deliberately black-box after
// repository provisioning: every repository state transition and observation
// uses an unmodified Git client over the public smart-HTTP remote.
func TestStockGitCompletesSingleBranchWorkflow(t *testing.T) {
	requireGit(t)
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	server := gitServer(t, store)
	remote := server.URL + "/repositories/" + string(repository.ID())

	publisher := gitClone(t, remote)
	gitOutput(t, publisher, "config", "user.name", "Ada Lovelace")
	gitOutput(t, publisher, "config", "user.email", "ada@example.com")
	if err := os.WriteFile(filepath.Join(publisher, "README.md"), []byte("# Project\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, publisher, "add", "README.md")
	gitOutput(t, publisher, "commit", "-m", "Start project")
	initialCommit := gitOutput(t, publisher, "rev-parse", "HEAD")
	gitOutput(t, publisher, "push", "--set-upstream", "origin", "main")
	assertRemoteBranch(t, remote, initialCommit)

	collaborator := gitClone(t, remote)
	assertFile(t, filepath.Join(collaborator, "README.md"), "# Project\n", 0)

	if err := os.WriteFile(filepath.Join(publisher, "README.md"), []byte("# Project\n\nReady to collaborate.\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, publisher, "commit", "-am", "Invite collaborators")
	ordinaryCommit := gitOutput(t, publisher, "rev-parse", "HEAD")
	gitOutput(t, publisher, "push")
	gitOutput(t, collaborator, "pull", "--ff-only")
	if head := gitOutput(t, collaborator, "rev-parse", "HEAD"); head != ordinaryCommit {
		t.Fatalf("pulled head = %s, want %s", head, ordinaryCommit)
	}
	assertFile(t, filepath.Join(collaborator, "README.md"), "# Project\n\nReady to collaborate.\n", 0)

	gitOutput(t, publisher, "reset", "--hard", initialCommit)
	if err := os.WriteFile(filepath.Join(publisher, "README.md"), []byte("# Reimagined project\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, publisher, "commit", "-am", "Reimagine project")
	forcedCommit := gitOutput(t, publisher, "rev-parse", "HEAD")
	gitOutput(t, publisher, "push", "--force-with-lease")
	assertRemoteBranch(t, remote, forcedCommit)
	forcedClone := gitClone(t, remote)
	assertFile(t, filepath.Join(forcedClone, "README.md"), "# Reimagined project\n", 0)

	gitOutput(t, publisher, "push", "origin", ":main")
	if advertised := gitLsRemote(t, remote, remote, "refs/heads/main"); advertised != "" {
		t.Fatalf("deleted branch advertisement = %q, want empty", advertised)
	}
	emptyClone := gitClone(t, remote)
	if branch := gitOutput(t, emptyClone, "symbolic-ref", "--short", "HEAD"); branch != "main" {
		t.Fatalf("clone after deletion selected branch %q, want main", branch)
	}

	gitOutput(t, publisher, "push", "--set-upstream", "origin", "main")
	assertRemoteBranch(t, remote, forcedCommit)
	gitOutput(t, emptyClone, "pull", "origin", "main")
	if head := gitOutput(t, emptyClone, "rev-parse", "HEAD"); head != forcedCommit {
		t.Fatalf("recovered head = %s, want %s", head, forcedCommit)
	}
	assertFile(t, filepath.Join(emptyClone, "README.md"), "# Reimagined project\n", 0)
}

func assertRemoteBranch(t *testing.T, remote, want string) {
	t.Helper()
	if advertised := gitLsRemote(t, remote, remote, "refs/heads/main"); advertised != want+"\trefs/heads/main" {
		t.Fatalf("remote branch advertisement = %q, want %s", advertised, want)
	}
}
