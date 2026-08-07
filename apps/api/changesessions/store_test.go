package changesessions

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionSurvivesReopenWithTimeline(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	store, _ := New(root)
	now := time.Date(2026, 8, 7, 16, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	created, err := store.Create("repo", "pull", "user", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if created.State != AwaitingInstructions || len(created.Events) != 1 || created.Events[0].Type != "session.started" {
		t.Fatalf("unexpected session: %#v", created)
	}
	reopened, _ := New(root)
	got, err := reopened.Get("repo", "pull", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.InitiatorID != "user" || got.SourceCommitID != "abc123" || len(got.Events) != 1 {
		t.Fatalf("unexpected reopened session: %#v", got)
	}
}
