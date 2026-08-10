package workspaces

import (
	"os"
	"testing"
)

func TestResumeRejectsChangedFoundation(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Version: 1, Setup: []string{"true"}, Resources: ResourceLimits{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128, SetupTimeoutSeconds: 1}}
	item, err := store.Create("repository", "revision", "actor", SourceContext{Type: "repository"}, Access{RepositoryID: "repository", ActorID: "actor", Permission: "repository:write"}, definition, "digest-a")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Append(item.ID, Event{Type: "log", Message: "setup"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Finish(item.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err = ensureEnvironment(store.Environment(item.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Suspend("repository", item.ID, "actor"); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Resume("repository", item.ID, "actor", "digest-b"); err != ErrInvalidTransition {
		t.Fatalf("changed foundation error = %v", err)
	}
	resumed, err := store.Resume("repository", item.ID, "actor", "digest-a")
	if err != nil || resumed.State != Ready {
		t.Fatalf("resume = %#v, %v", resumed, err)
	}
}

func ensureEnvironment(path string) error { return os.Mkdir(path, 0o750) }
