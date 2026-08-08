package checkruns

import (
	"errors"
	"testing"
)

func TestStoreRetainsOrderedEvidenceForEveryAttempt(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Name: "unit", Command: "go test ./...", TimeoutSeconds: 60}
	first, _ := store.Create("repository", "pull", "commit", definition)
	_, _ = store.Start(first.ID)
	_ = store.AppendLog(first.ID, "stderr", "failed assertion\n")
	_, _ = store.Complete(first.ID, 1, false, "command failed")
	second, _ := store.Create("repository", "pull", "commit", definition)
	_, _ = store.Start(second.ID)
	_ = store.AppendLog(second.ID, "stdout", "ok\n")
	_, _ = store.Complete(second.ID, 0, false, "")

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := reopened.List("repository", "pull")
	if err != nil || len(runs) != 2 {
		t.Fatalf("runs = %#v, %v", runs, err)
	}
	states := map[State]bool{}
	for _, run := range runs {
		states[run.State] = true
		if run.CommitID != "commit" || len(run.Events) != 5 {
			t.Fatalf("run = %#v", run)
		}
		for index, event := range run.Events {
			if event.Sequence != int64(index+1) {
				t.Fatalf("events = %#v", run.Events)
			}
		}
	}
	if !states[Succeeded] || !states[Failed] {
		t.Fatalf("states = %#v", states)
	}
}

func TestStoreRetainsRerunAndCancellationAttribution(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Name: "unit", Command: "go test ./...", TimeoutSeconds: 60}
	original, err := store.Create("repository", "pull", "commit", definition)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := store.CreateAttempt("repository", "pull", "commit", definition, "collaborator", original.ID)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := store.Cancel(attempt.ID, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if canceled.State != Canceled || canceled.TriggeredByID != "collaborator" || canceled.RetryOfID != original.ID || canceled.CanceledByID != "reviewer" {
		t.Fatalf("canceled attempt = %#v", canceled)
	}
	last := canceled.Events[len(canceled.Events)-1]
	if last.Status != Canceled || last.ActorID != "reviewer" || last.Sequence != 2 {
		t.Fatalf("cancellation event = %#v", last)
	}
	if _, err := store.Start(attempt.ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("start canceled attempt error = %v", err)
	}
}
