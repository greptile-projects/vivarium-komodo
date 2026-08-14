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

func TestPrivacyEvidenceSanitizer(t *testing.T) {
	got := sanitizePrivacyOutput("email=person@example.test token=secret Authorization: Bearer Cookie: session=abc\n")
	if got != "email=[REDACTED] token=[REDACTED] Authorization:[REDACTED] Cookie:[REDACTED]\n" {
		t.Fatalf("sanitized output = %q", got)
	}
}

func TestPrivacyCheckRunsWithSyntheticExactInputAndSanitizedEvidence(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	input, _ := repository.WriteObject(storage.BlobObject, []byte("synthetic-user\n"))
	checks, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1,"checks":[{"name":"unit","command":"true"}]}`))
	privacy, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1,"checks":[{"name":"privacy/account","command":"printf 'email=person@example.test token=secret\\n'; printf 'Cookie: session=abc\\n' > privacy.trace","artifacts":["privacy.trace"],"privacy":{"journey_ids":["account-lifecycle"],"dimensions":["consent","deletion","recipient"],"inputs":["journey.txt"],"commitment_ids":["data-use-v2"],"synthetic_data":true,"requires_preview":true}}]}`))
	config, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"checks.json": {checks, 0o100644}, "privacy-checks.json": {privacy, 0o100644}}))
	root, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {config, 0o40000}, "journey.txt": {input, 0o100644}}))
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(root)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nprivacy\n"))
	store, _ := New(t.TempDir())
	runner := NewRunner(store, testRepositories{repository})
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-privacy", string(commit)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, _ := store.List(string(repository.ID()), "pull-privacy")
		if len(runs) == 2 {
			for _, run := range runs {
				if run.Definition.Privacy == nil {
					continue
				}
				if run.State == Failed {
					t.Fatalf("privacy run failed: %#v", run)
				}
				if run.State != Succeeded {
					break
				}
				var log string
				var artifact *Artifact
				for _, event := range run.Events {
					log += event.Message
					if event.Artifact != nil {
						artifact = event.Artifact
					}
				}
				if log != "email=[REDACTED] token=[REDACTED]\n" || artifact == nil {
					t.Fatalf("unsanitized evidence: %q %#v", log, artifact)
				}
				_, file, e := store.OpenArtifact(string(repository.ID()), "pull-privacy", run.ID, artifact.ID)
				if e != nil {
					t.Fatal(e)
				}
				body, _ := io.ReadAll(file)
				_ = file.Close()
				if string(body) != "Cookie:[REDACTED]\n" {
					t.Fatalf("artifact=%q", body)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("privacy check did not complete")
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

func TestDocumentationChecksRetainMatrixEvidenceAndReuseOnlyUnaffectedInputs(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	makeCommit := func(guide, code, unrelated string) storage.ObjectID {
		guideBlob, _ := repository.WriteObject(storage.BlobObject, []byte(guide))
		codeBlob, _ := repository.WriteObject(storage.BlobObject, []byte(code))
		otherBlob, _ := repository.WriteObject(storage.BlobObject, []byte(unrelated))
		checksBlob, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1,"checks":[{"name":"unit","command":"true","timeout_seconds":5}]}`))
		docManifest := `{"version":1,"checks":[{"name":"docs/tutorial","command":"printf 'expected\\n' > expected.txt; printf 'actual\\n' > actual.txt; diff -u expected.txt actual.txt > output.diff || true; printf '<html>built</html>' > built.html","timeout_seconds":5,"artifacts":["built.html","output.diff"],"documentation":{"kind":"tutorial","collection_id":"guide","inputs":["docs/guide.md","src/api.txt"],"pages":["docs/guide.md"],"symbols":["Client.Open"],"links":["/reference/client"],"versions":[{"label":"v1","source_commit":"source-v1"},{"label":"sdk-2","package":"@example/sdk@2"}],"expected_output":"expected","coverage":{"links":1,"symbols":1,"samples":1}}}]}`
		docsChecksBlob, _ := repository.WriteObject(storage.BlobObject, []byte(docManifest))
		configTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"checks.json": {checksBlob, 0o100644}, "documentation-checks.json": {docsChecksBlob, 0o100644}}))
		docsTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"guide.md": {guideBlob, 0o100644}}))
		srcTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"api.txt": {codeBlob, 0o100644}}))
		rootTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}, "docs": {docsTree, 0o40000}, "src": {srcTree, 0o40000}, "notes.txt": {otherBlob, 0o100644}}))
		commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\ndocs\n"))
		return commit
	}
	store, _ := New(t.TempDir())
	runner := NewRunner(store, testRepositories{repository})
	wait := func(want int) []Run {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			runs, _ := store.List(string(repository.ID()), "pull-docs")
			terminal := len(runs) == want
			for _, run := range runs {
				terminal = terminal && (run.State == Succeeded || run.State == Failed)
			}
			if terminal {
				return runs
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("documentation checks did not complete")
		return nil
	}
	firstCommit := makeCommit("guide", "api", "one")
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-docs", string(firstCommit)); err != nil {
		t.Fatal(err)
	}
	first := wait(2)
	var original Run
	for _, run := range first {
		if run.Definition.Kind == "documentation" {
			original = run
		}
	}
	if original.State != Succeeded || original.Definition.Documentation == nil || len(original.Definition.Documentation.Versions) != 2 || original.Definition.Documentation.Coverage["samples"] != 1 {
		t.Fatalf("documentation evidence = %#v", original)
	}
	artifacts := 0
	for _, event := range original.Events {
		if event.Artifact != nil {
			artifacts++
		}
	}
	if artifacts != 2 {
		t.Fatalf("artifacts = %#v", original.Events)
	}
	secondCommit := makeCommit("guide", "api", "unrelated changed")
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-docs", string(secondCommit)); err != nil {
		t.Fatal(err)
	}
	second := wait(4)
	var reused Run
	for _, run := range second {
		if run.CommitID == string(secondCommit) && run.Definition.Kind == "documentation" {
			reused = run
		}
	}
	if reused.State != Succeeded || reused.Definition.Documentation.ReusedFromRunID != original.ID || reused.Definition.Documentation.InputDigest != original.Definition.Documentation.InputDigest {
		t.Fatalf("reused evidence = %#v", reused)
	}
	thirdCommit := makeCommit("guide changed", "api", "unrelated changed")
	if err := runner.Start(string(repository.ID()), string(repository.ID()), "pull-docs", string(thirdCommit)); err != nil {
		t.Fatal(err)
	}
	third := wait(6)
	for _, run := range third {
		if run.CommitID == string(thirdCommit) && run.Definition.Kind == "documentation" && run.Definition.Documentation.ReusedFromRunID != "" {
			t.Fatalf("changed documentation evidence was reused: %#v", run)
		}
	}
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

func TestReleaseManifestRetainsOrderedDependencies(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	manifest, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1,"builds":[{"name":"compile","command":"true"},{"name":"package","command":"true","dependencies":["compile"],"artifacts":["dist/app"]}]}`))
	configTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"releases.json": {manifest, 0o100644}}))
	rootTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}}))
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nrelease\n"))
	definitions, err := readReleaseManifest(repository, commit)
	if err != nil || len(definitions) != 2 || !slices.Equal(definitions[1].Dependencies, []string{"compile"}) {
		t.Fatalf("definitions = %#v, %v", definitions, err)
	}
}

func TestFailedReleaseBuildRerunRetainsEarlierEvidence(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	repository, _ := gitStore.Create()
	manifest, _ := repository.WriteObject(storage.BlobObject, []byte(`{"version":1,"builds":[{"name":"package","command":"echo diagnostic >&2; false","timeout_seconds":5}]}`))
	configTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{"releases.json": {manifest, 0o100644}}))
	rootTree, _ := repository.WriteObject(storage.TreeObject, tree(t, map[string]treeItem{".komodo": {configTree, 0o40000}}))
	commit, _ := repository.WriteObject(storage.CommitObject, []byte("tree "+string(rootTree)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nrelease\n"))
	store, _ := New(t.TempDir())
	runner := NewRunner(store, testRepositories{repository})
	runs, err := runner.StartRelease(string(repository.ID()), "release-1", string(commit), "maintainer")
	if err != nil || len(runs) != 1 {
		t.Fatalf("start = %#v, %v", runs, err)
	}
	waitTerminal := func(want int) []Run {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			items, _ := store.List(string(repository.ID()), "release:release-1")
			if len(items) == want && items[0].State == Failed {
				return items
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("release attempt did not fail")
		return nil
	}
	first := waitTerminal(1)[0]
	if _, err := runner.Rerun(string(repository.ID()), "release:release-1", first.ID, "maintainer"); err != nil {
		t.Fatal(err)
	}
	attempts := waitTerminal(2)
	if attempts[0].RetryOfID != first.ID || attempts[1].ID != first.ID || len(attempts[1].Events) == 0 {
		t.Fatalf("retained attempts = %#v", attempts)
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
