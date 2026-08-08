package checkruns

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type testRepositories struct{ repository *storage.Repository }

func (r testRepositories) Open(storage.ID) (*storage.Repository, error) { return r.repository, nil }

type mappedRepositories map[storage.ID]*storage.Repository

func (r mappedRepositories) Open(id storage.ID) (*storage.Repository, error) { return r[id], nil }

func TestRunnerExecutesVersionedManifestAgainstExactSnapshot(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	revision, _ := repository.WriteObject(storage.BlobObject, []byte("candidate\n"))
	manifest, _ := json.Marshal(map[string]any{"version": 1, "checks": []map[string]any{{"name": "unit", "command": `test "$(cat revision.txt)" = candidate && test ! -e .git && test "$CHECK_MODE" = exact && echo live-output && echo warning >&2 && printf evidence > report.txt`, "timeout_seconds": 5, "environment": map[string]string{"CHECK_MODE": "exact"}, "artifacts": []string{"report.txt"}}}})
	manifestBlob, _ := repository.WriteObject(storage.BlobObject, manifest)
	configTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"checks.json": {manifestBlob, 0o100644}}))
	rootTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}, "revision.txt": {revision, 0o100644}}))
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\ncandidate\n"))

	root := t.TempDir()
	store, _ := New(root)
	runner := NewRunner(store, testRepositories{repository})
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-1", string(commit)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var run Run
	for time.Now().Before(deadline) {
		items, err := store.List(string(repository.ID()), "pull-1")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 1 {
			run = items[0]
			if run.State == Succeeded || run.State == Failed {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.State != Succeeded || run.CommitID != string(commit) || run.Definition.Name != "unit" || run.StartedAt == nil || run.CompletedAt == nil || run.ExitCode == nil || *run.ExitCode != 0 {
		t.Fatalf("run = %#v", run)
	}
	var stdout, stderr string
	var artifact *Artifact
	for index, event := range run.Events {
		if event.Sequence != int64(index+1) {
			t.Fatalf("event sequence = %#v", run.Events)
		}
		if event.Stream == "stdout" {
			stdout += event.Message
		}
		if event.Stream == "stderr" {
			stderr += event.Message
		}
		if event.Artifact != nil {
			artifact = event.Artifact
		}
	}
	if stdout != "live-output\n" || stderr != "warning\n" || artifact == nil || artifact.Path != "report.txt" {
		t.Fatalf("evidence = stdout %q, stderr %q, artifact %#v", stdout, stderr, artifact)
	}
	_, file, err := store.OpenArtifact(string(repository.ID()), "pull-1", run.ID, artifact.ID)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(file)
	_ = file.Close()
	if string(content) != "evidence" {
		t.Fatalf("artifact content = %q", content)
	}
	reopened, _ := New(root)
	durable, err := reopened.List(string(repository.ID()), "pull-1")
	if err != nil || len(durable) != 1 || durable[0].State != Succeeded || len(durable[0].Events) != len(run.Events) {
		t.Fatalf("reopened runs = %#v, %v", durable, err)
	}
}

func TestRunnerExecutesUpstreamCheckAgainstForkSnapshot(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	upstream, _ := gitStore.Create()
	fork, _ := gitStore.Create()
	manifest, _ := fork.WriteObject(storage.BlobObject, []byte(`{"version":1,"checks":[{"name":"fork-quality","command":"true","timeout_seconds":5}]}`))
	configTree, _ := fork.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"checks.json": {manifest, 0o100644}}))
	rootTree, _ := fork.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}}))
	commit, _ := fork.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor Outside <outside@example.com> 1 +0000\ncommitter Outside <outside@example.com> 1 +0000\n\nfork change\n"))
	store, _ := New(t.TempDir())
	runner := NewRunner(store, mappedRepositories{upstream.ID(): upstream, fork.ID(): fork})
	if err := runner.Start(string(upstream.ID()), string(fork.ID()), "pull-1", string(commit)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := store.List(string(upstream.ID()), "pull-1")
		if len(runs) == 1 && (runs[0].State == Succeeded || runs[0].State == Failed) {
			if runs[0].State != Succeeded || runs[0].RepositoryID != string(upstream.ID()) || runs[0].SourceRepositoryID != string(fork.ID()) || runs[0].CommitID != string(commit) {
				t.Fatalf("cross-repository run = %#v", runs[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cross-repository check did not complete")
}

func TestRunnerRejectsUnsupportedManifestBeforeQueuing(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	manifest, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":2,"checks":[{"name":"unit","command":"true"}]}`))
	configTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"checks.json": {manifest, 0o100644}}))
	rootTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}}))
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nbad config\n"))
	store, _ := New(t.TempDir())
	runner := NewRunner(store, testRepositories{repository})
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-1", string(commit)); err == nil {
		t.Fatal("expected invalid manifest")
	}
	items, _ := store.List(string(repository.ID()), "pull-1")
	if len(items) != 0 {
		t.Fatalf("runs = %#v", items)
	}
}

type treeItem struct {
	id   storage.ObjectID
	mode uint32
}

func tree(t *testing.T, entries map[string]treeItem) []byte {
	t.Helper()
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	var data []byte
	for _, name := range names {
		item := entries[name]
		raw, err := hex.DecodeString(string(item.id))
		if err != nil {
			t.Fatal(err)
		}
		mode := "100644"
		if item.mode == 0o40000 {
			mode = "40000"
		}
		data = append(data, []byte(mode+" "+name)...)
		data = append(data, 0)
		data = append(data, raw...)
	}
	return data
}
