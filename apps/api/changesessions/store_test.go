package changesessions

import (
	"errors"
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

func TestCollaboratorInterventionsHaveOrderedControlSemantics(t *testing.T) {
	store, _ := New(t.TempDir())
	session, _ := store.Create("repo", "pull", "initiator", "revision")
	run, _ := store.Delegate("repo", "pull", session.ID, DelegateParams{InitiatorID: "initiator", Agent: "codex", Instructions: "work", RevisionID: "revision", WorkingBranch: "agent/work", CredentialGrantID: "grant", CredentialExpiresAt: time.Now().Add(time.Hour)})
	if _, _, err := store.Intervene("repo", "pull", session.ID, run.ID, "peer", "guidance", "Keep the API compatible."); err != nil {
		t.Fatal(err)
	}
	if _, paused, err := store.Intervene("repo", "pull", session.ID, run.ID, "peer", "pause", ""); err != nil || paused.State != Paused {
		t.Fatalf("pause: %#v %v", paused, err)
	}
	if _, err := store.AppendRunEvent("repo", "pull", session.ID, run.ID, "agent.message", map[string]string{"message": "still working"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("paused worker event: %v", err)
	}
	if _, resumed, err := store.Intervene("repo", "pull", session.ID, run.ID, "initiator", "resume", ""); err != nil || resumed.State != Running {
		t.Fatalf("resume: %#v %v", resumed, err)
	}
	if _, canceled, err := store.Intervene("repo", "pull", session.ID, run.ID, "peer", "cancel", "Stop before publishing."); err != nil || canceled.State != Canceled {
		t.Fatalf("cancel: %#v %v", canceled, err)
	}
	if _, _, err := store.Intervene("repo", "pull", session.ID, run.ID, "peer", "resume", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("resumed canceled run: %v", err)
	}
	got, _ := store.Get("repo", "pull", session.ID)
	if len(got.Events) != 6 || got.Events[3].Metadata["action"] != "pause" || got.Events[5].Metadata["action"] != "cancel" {
		t.Fatalf("timeline: %#v", got.Events)
	}
}
