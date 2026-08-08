package integrationqueue

import "testing"

func TestQueuePreservesAdmissionOrderAndSnapshots(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Enqueue("repo", "pull-1", "main", "source-1", "target-1", "owner")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Enqueue("repo", "pull-2", "main", "source-2", "target-1", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if first.Position != 1 || second.Position != 2 {
		t.Fatalf("positions = %d, %d", first.Position, second.Position)
	}
	if _, err := store.Enqueue("repo", "pull-1", "main", "source-1", "target-1", "owner"); err != ErrConflict {
		t.Fatalf("duplicate error = %v", err)
	}
	items, err := store.List("repo", "main")
	if err != nil || len(items) != 2 || items[0].PullRequestID != "pull-1" || items[1].SourceCommitID != "source-2" || items[1].TargetCommitID != "target-1" {
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
