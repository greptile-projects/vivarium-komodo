package integrationqueue

import "testing"

func TestQueuePreservesAdmissionOrderAndSnapshots(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Enqueue("repo", "pull-1", "main", "source-1", "target-1", "candidate-1", "tree-1", "owner", []string{"unit"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Enqueue("repo", "pull-2", "main", "source-2", "target-1", "candidate-2", "tree-2", "owner", []string{"unit"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("positions = %d, %d", first.Position, second.Position)
	}
	if _, err := store.Enqueue("repo", "pull-1", "main", "source-1", "target-1", "candidate-1", "tree-1", "owner", []string{"unit"}); err != ErrConflict {
		t.Fatalf("duplicate error = %v", err)
	}
	items, err := store.List("repo", "main")
	if err != nil || len(items) != 2 || items[0].PullRequestID != "pull-1" || items[0].CandidateCommitID != "candidate-1" || items[0].CandidateTreeID != "tree-1" || len(items[0].RequiredChecks) != 1 || items[1].SourceCommitID != "source-2" || items[1].TargetCommitID != "target-1" {
		t.Fatalf("items = %#v, %v", items, err)
	}
	reopened, err := New(store.root)
	if err != nil {
		t.Fatal(err)
	}
	items, err = reopened.List("repo", "main")
	if err != nil || len(items) != 2 {
		t.Fatalf("reopened items = %#v, %v", items, err)
	}
}

func TestCandidateReplacementAndTerminalTransitionAreDurable(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	entry, err := store.Enqueue("repo", "pull", "main", "source", "base-1", "candidate-1", "tree-1", "owner", []string{"unit"})
	if err != nil {
		t.Fatal(err)
	}
	replaced, err := store.ReplaceCandidate(entry.ID, "base-2", "candidate-2", "tree-2", []string{"unit"})
	if err != nil || replaced.Generation != 2 || replaced.CandidateCommitID != "candidate-2" || replaced.TargetCommitID != "base-2" {
		t.Fatalf("replacement = %#v, %v", replaced, err)
	}
	terminal, err := store.Transition(entry.ID, "removed", "source_updated", true)
	if err != nil || terminal.CompletedAt == nil || terminal.Reason != "source_updated" {
		t.Fatalf("terminal = %#v, %v", terminal, err)
	}
	if active, err := store.List("repo", "main"); err != nil || len(active) != 0 {
		t.Fatalf("active = %#v, %v", active, err)
	}
	reopened, _ := New(store.root)
	retained, err := reopened.Get(entry.ID)
	if err != nil || retained.Generation != 2 || retained.State != "removed" {
		t.Fatalf("retained = %#v, %v", retained, err)
	}
}

func TestMaintainerOperationsPreserveOrderAttributionAndHistory(t *testing.T) {
	store, _ := New(t.TempDir())
	first, _ := store.Enqueue("repo", "pull-1", "main", "source-1", "base", "candidate-1", "tree-1", "owner", nil)
	second, _ := store.Enqueue("repo", "pull-2", "main", "source-2", "base", "candidate-2", "tree-2", "owner", nil)

	moved, err := store.Operate(second.ID, "reprioritize", "maintainer", 1)
	if err != nil || moved.Position != 1 || moved.Events[len(moved.Events)-1].ActorID != "maintainer" {
		t.Fatalf("reprioritized = %#v, %v", moved, err)
	}
	ordered, _ := store.List("repo", "main")
	if len(ordered) != 2 || ordered[0].ID != second.ID || ordered[1].ID != first.ID || ordered[1].Position != 2 {
		t.Fatalf("order = %#v", ordered)
	}
	paused, _ := store.Operate(second.ID, "pause", "maintainer", 0)
	if paused.State != "paused" || paused.Reason != "paused_by_maintainer" {
		t.Fatalf("paused = %#v", paused)
	}
	resumed, _ := store.Operate(second.ID, "resume", "maintainer", 0)
	if resumed.State != "verifying" || resumed.Reason != "" {
		t.Fatalf("resumed = %#v", resumed)
	}
	removed, _ := store.Operate(second.ID, "remove", "maintainer", 0)
	if removed.CompletedAt == nil || removed.State != "removed" {
		t.Fatalf("removed = %#v", removed)
	}
	history, _ := store.History("repo", "main")
	if len(history) != 2 || history[1].ID != second.ID || len(history[1].Candidates) != 1 {
		t.Fatalf("history = %#v", history)
	}
}
