package workspaces

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCheckpointCapturesSafeDiffAndRestoresWithoutRuntimeData(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	readme, _ := repository.WriteObject(storage.BlobObject, []byte("base\n"))
	manifest, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1}`))
	komodo, _ := repository.WriteObject(storage.TreeObject, checkpointTreeEntry("100644", "workspaces.json", manifest))
	root, _ := repository.WriteObject(storage.TreeObject, append(checkpointTreeEntry("040000", ".komodo", komodo), checkpointTreeEntry("100644", "README.md", readme)...))
	commitText := fmt.Sprintf("tree %s\nauthor Test <test@example.com> 0 +0000\ncommitter Test <test@example.com> 0 +0000\n\nbase\n", root)
	commit, _ := repository.WriteObject(storage.CommitObject, []byte(commitText))
	store, _ := New(t.TempDir())
	definition := Definition{Version: 1, Dependencies: []string{"go modules"}, Setup: []string{"true"}, Resources: ResourceLimits{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128, SetupTimeoutSeconds: 1}}
	workspace, _ := store.Create(string(repository.ID()), string(commit), "owner", SourceContext{Type: "repository"}, Access{}, definition, "definition")
	_ = os.Mkdir(store.Environment(workspace.ID), 0750)
	_ = os.WriteFile(store.Environment(workspace.ID)+"/README.md", []byte("unfinished\n"), 0640)
	_ = os.WriteFile(store.Environment(workspace.ID)+"/.env", []byte("TOKEN=secret"), 0640)
	workspace, _ = store.Finish(workspace.ID, true, "")
	runner := NewRunner(store, gitStore)
	updated, err := runner.Checkpoint(workspace, "peer", CheckpointRequest{Summary: "preserve parser work", Paths: []string{"README.md"}, Reproducibility: Reproducibility{Dependencies: []string{"go modules"}, Commands: []string{"go test ./..."}}})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := updated.Checkpoints[0]
	if checkpoint.CreatorID != "peer" || checkpoint.Changes[0].Operation != "modify" || !strings.Contains(checkpoint.Changes[0].Patch, "+unfinished") || !checkpoint.Status.Reproducible {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
	encoded, _ := json.Marshal(checkpoint)
	if strings.Contains(string(encoded), "blob_digest") {
		t.Fatal("private blob address leaked through public checkpoint shape")
	}
	if _, err = runner.Checkpoint(updated, "peer", CheckpointRequest{Summary: "unsafe", Paths: []string{".env"}}); err != ErrUnsafeCheckpoint {
		t.Fatalf("secret error = %v", err)
	}
	_ = os.WriteFile(store.Environment(workspace.ID)+"/README.md", []byte("conflicting\n"), 0640)
	current, _ := store.Get(string(repository.ID()), workspace.ID)
	_, status, err := runner.Restore(current, "owner", checkpoint.ID)
	if err != ErrConflict || !status.Diverged || len(status.Conflicts) != 1 {
		t.Fatalf("conflict = %#v, %v", status, err)
	}
	_ = os.WriteFile(store.Environment(workspace.ID)+"/README.md", []byte("base\n"), 0640)
	current, _ = store.Get(string(repository.ID()), workspace.ID)
	if _, _, err = runner.Restore(current, "owner", checkpoint.ID); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(store.Environment(workspace.ID) + "/README.md")
	if string(data) != "unfinished\n" {
		t.Fatalf("restored = %q", data)
	}
	if secret, _ := os.ReadFile(store.Environment(workspace.ID) + "/.env"); string(secret) != "TOKEN=secret" {
		t.Fatal("restore touched unrelated runtime data")
	}
}

func checkpointTreeEntry(mode, name string, id storage.ObjectID) []byte {
	raw, _ := hex.DecodeString(string(id))
	return append([]byte(mode+" "+name+"\x00"), raw...)
}
