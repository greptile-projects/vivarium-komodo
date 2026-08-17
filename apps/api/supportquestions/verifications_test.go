package supportquestions

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAnswerVerificationRunsExactInstructionsAndRetainsRerunnableEvidence(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repo, _ := git.Create()
	blob, _ := repo.WriteObject(storage.BlobObject, []byte("supported\n"))
	tree, _ := repo.WriteObject(storage.TreeObject, verificationTreeEntry("100644", "version.txt", blob))
	commit, _ := repo.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Test <test@example.test> 0 +0000\ncommitter Test <test@example.test> 0 +0000\n\nfixture\n", tree)))
	store, _ := New(t.TempDir())
	q, _ := store.Create(string(repo.ID()), "asker", Input{Title: "Install", Question: "How?", Subject: Subject{Kind: "repository"}, SoftwareVersion: "v2", Environment: "ubuntu-24.04", Goal: "work", AttemptedSteps: []string{"tried"}, Urgency: "normal", Audience: "repository", Contact: Contact{Preference: "thread"}})
	answer := answerInput()
	answer.ApplicableVersions = []string{"v2"}
	answer.Instructions = []string{"test \"$(cat version.txt)\" = supported", "printf \"$(cat .komodo-inputs/case.txt)\" > result.txt"}
	q, _ = store.ReviseAnswer(string(repo.ID()), q.ID, "maintainer", answer)
	revision := q.Answers[0].Revisions[0]
	in := VerificationInputRequest{AnswerID: q.Answers[0].ID, AnswerRevisionID: revision.ID, SourceRevision: string(commit), SoftwareVersion: "v2", Environment: VerificationEnvironment{Name: "ubuntu-24.04", ImageDigest: "sha256:ubuntu", Tools: []string{"sh"}, Resources: VerificationResources{CPUSeconds: 10, MemoryMB: 256, DiskMB: 128}}, Dependencies: map[string]string{"shell": "dash-0.5"}, Inputs: []VerificationInput{{Name: "case.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("sanitized"))}}, ArtifactPaths: []string{"result.txt"}, CostUnits: 0.01}
	attempt, err := store.CreateVerification(q, "asker", "", in)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewVerificationRunner(store, git)
	runner.Start(attempt)
	completed := waitVerification(t, store, string(repo.ID()), q.ID, attempt.ID)
	if completed.State != "passed" || completed.InstructionsDigest == "" || len(completed.Events) < 4 || len(completed.Artifacts) != 1 || completed.CostUnits != 0.01 {
		t.Fatalf("attempt = %#v", completed)
	}
	artifact, _ := base64.StdEncoding.DecodeString(completed.Artifacts[0].Content)
	if string(artifact) != "sanitized" || completed.Inputs[0].SHA256 == "" {
		t.Fatalf("evidence = %#v", completed)
	}
	in.Inputs = completed.Inputs
	rerun, err := store.CreateVerification(q, "collaborator", completed.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	runner.Start(rerun)
	repeated := waitVerification(t, store, string(repo.ID()), q.ID, rerun.ID)
	if repeated.State != "passed" || repeated.RerunOf != completed.ID || repeated.CreatedByID != "collaborator" || repeated.InstructionsDigest != completed.InstructionsDigest {
		t.Fatalf("rerun = %#v", repeated)
	}
}

func TestAnswerVerificationRejectsCredentialsAndUnstatedVersions(t *testing.T) {
	store, _ := New(t.TempDir())
	q, _ := store.Create("repo", "asker", Input{Title: "Question", Question: "How?", Subject: Subject{Kind: "repository"}, SoftwareVersion: "v2", Environment: "linux", Goal: "work", AttemptedSteps: []string{"tried"}, Urgency: "normal", Audience: "repository", Contact: Contact{Preference: "none"}})
	q, _ = store.ReviseAnswer("repo", q.ID, "owner", answerInput())
	r := q.Answers[0].Revisions[0]
	base := VerificationInputRequest{AnswerID: q.Answers[0].ID, AnswerRevisionID: r.ID, SourceRevision: "commit", SoftwareVersion: "v2.1.0", Environment: VerificationEnvironment{Name: "linux", ImageDigest: "sha256:linux", Resources: VerificationResources{CPUSeconds: 1, MemoryMB: 128, DiskMB: 128}}}
	base.Inputs = []VerificationInput{{Name: "api-token.txt", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("redacted"))}}
	if _, err := store.CreateVerification(q, "asker", "", base); err != ErrInvalid {
		t.Fatalf("credential input accepted: %v", err)
	}
	base.Inputs = nil
	base.SoftwareVersion = "v3"
	if _, err := store.CreateVerification(q, "asker", "", base); err != ErrInvalid {
		t.Fatalf("unstated version accepted: %v", err)
	}
}

func waitVerification(t *testing.T, s *Store, repo, q, id string) VerificationAttempt {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		a, e := s.GetVerification(repo, q, id)
		if e != nil {
			t.Fatal(e)
		}
		if a.State == "passed" || a.State == "failed" {
			return a
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("verification did not finish")
	return VerificationAttempt{}
}
func verificationTreeEntry(mode, name string, id storage.ObjectID) []byte {
	decoded, _ := hex.DecodeString(string(id))
	return append([]byte(mode+" "+name+"\x00"), decoded...)
}
