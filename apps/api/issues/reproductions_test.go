package issues

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestReproductionAttemptRetainsBoundedEvidenceAndExactRerun(t *testing.T) {
	git, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := git.Create()
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":1,"environment":"ubuntu/bubblewrap with no network","tools":["sh"],"setup":[],"resources":{"cpu_seconds":10,"memory_mb":256,"disk_mb":128},"reproductions":[{"name":"reported-crash","command":"test \"$(cat .komodo-inputs/case.txt)\" = crash; printf retained > result.txt; exit 7","timeout_seconds":10,"expected_exit_code":7,"artifacts":["result.txt"]}]}`
	manifestBlob, _ := repository.WriteObject(storage.BlobObject, []byte(manifest))
	komodoTree, _ := repository.WriteObject(storage.TreeObject, testTreeEntry("100644", "reproductions.json", manifestBlob))
	rootTree, _ := repository.WriteObject(storage.TreeObject, testTreeEntry("40000", ".komodo", komodoTree))
	commitBody := fmt.Sprintf("tree %s\nauthor Test <test@example.com> 0 +0000\ncommitter Test <test@example.com> 0 +0000\n\nfixture\n", rootTree)
	commit, _ := repository.WriteObject(storage.CommitObject, []byte(commitBody))

	store, _ := New(t.TempDir())
	repositoryID := string(repository.ID())
	issue, err := store.Create(CreateInput{RepositoryID: repositoryID, ReporterID: "reporter", Title: "crash", ExpectedBehavior: "works", ObservedBehavior: "exit 7", Severity: "high", Environment: "sanitized", ReproductionSteps: []string{"run fixture"}, AffectedCommitID: string(commit), Visibility: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewReproductionRunner(store, git)
	definition, digest, err := runner.Definition(repositoryID, string(commit))
	if err != nil {
		t.Fatal(err)
	}
	input := ReproductionInput{Name: "case.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("crash"))}
	attempt, err := store.CreateReproduction(issue, string(commit), "", "", "reporter", "", definition, digest, definition.Reproductions[0], []ReproductionInput{input})
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(attempt)
	completed := waitReproduction(t, store, repositoryID, issue.ID, attempt.ID)
	if completed.State != "completed" || !completed.Reproduced || completed.ObservedResult != "command exited with expected code 7" {
		t.Fatalf("attempt = %#v", completed)
	}
	if len(completed.Events) < 2 || len(completed.Artifacts) != 1 || string(mustDecode(t, completed.Artifacts[0].Content)) != "retained" {
		t.Fatalf("evidence = %#v %#v", completed.Events, completed.Artifacts)
	}
	if completed.DefinitionDigest != digest || completed.Revision != string(commit) || completed.Inputs[0].SHA256 == "" {
		t.Fatalf("immutable context = %#v", completed)
	}

	rerun, err := store.CreateReproduction(issue, completed.Revision, completed.ReleaseID, completed.ReleaseVersion, "collaborator", completed.ID, completed.Definition, completed.DefinitionDigest, completed.Command, completed.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(rerun)
	repeated := waitReproduction(t, store, repositoryID, issue.ID, rerun.ID)
	if !repeated.Reproduced || repeated.RerunOf != completed.ID || repeated.CreatedByID != "collaborator" || repeated.DefinitionDigest != completed.DefinitionDigest {
		t.Fatalf("rerun = %#v", repeated)
	}
}

func TestReproductionRejectsCredentialLikeInputs(t *testing.T) {
	store, _ := New(t.TempDir())
	issue, _ := store.Create(CreateInput{RepositoryID: "repo", ReporterID: "reporter", Title: "crash", ExpectedBehavior: "works", ObservedBehavior: "fails", Severity: "high", Environment: "clean", ReproductionSteps: []string{"run"}, AffectedCommitID: "commit", Visibility: "repository"})
	definition := ReproductionDefinition{Version: 1, Environment: "bounded", Resources: ReproductionResources{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128}, Reproductions: []ReproductionCommand{{Name: "run", Command: "true", TimeoutSeconds: 1}}}
	_, err := store.CreateReproduction(issue, "commit", "", "", "reporter", "", definition, "digest", definition.Reproductions[0], []ReproductionInput{{Name: "api-token.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("redacted"))}})
	if err != ErrInvalid {
		t.Fatalf("credential-like input error = %v", err)
	}
}

func waitReproduction(t *testing.T, store *Store, repositoryID, issueID, attemptID string) ReproductionAttempt {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		attempt, err := store.GetReproduction(repositoryID, issueID, attemptID)
		if err != nil {
			t.Fatal(err)
		}
		if attempt.State == "completed" || attempt.State == "failed" {
			return attempt
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("reproduction did not finish")
	return ReproductionAttempt{}
}
func mustDecode(t *testing.T, value string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
func testTreeEntry(mode, name string, id storage.ObjectID) []byte {
	decoded, _ := hex.DecodeString(string(id))
	return append([]byte(mode+" "+name+"\x00"), decoded...)
}
