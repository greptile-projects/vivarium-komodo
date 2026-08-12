package docscollections

import "testing"

func TestGroundedTaskRequiresCitedSuggestions(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	revision := "1111111111111111111111111111111111111111"
	c, err := s.Create("repo", "owner", Input{ExpectedVersion: 0, Name: "Guide", RootPath: "docs", EntryPaths: []string{"guide.md"}, Versions: []VersionMapping{{Label: "current", SourceRevision: revision}}, OwnerIDs: []string{"owner"}, Audiences: []string{"developers"}, Policy: Policy{Navigation: "path", Renderer: "markdown", Publication: "maintainer_reviewed"}, ChangeReason: "start"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask("repo", c.ID, "owner", "Explain behavior", "guide.md", revision, TaskOrigin{Kind: "proposal", ResourceID: "proposal-1"}, []string{"proposal:proposal-1@" + revision}, "branch", "docs/explain")
	if err != nil {
		t.Fatal(err)
	}
	if task.Path != "docs/guide.md" || task.CollectionVersion != 1 {
		t.Fatalf("unexpected task: %#v", task)
	}
	if _, err = s.AddTaskEvent("repo", task.ID, "codex", TaskEvent{Type: "suggestion", Body: "State the invariant."}); err != ErrInvalid {
		t.Fatalf("uncited suggestion error = %v", err)
	}
	got, err := s.AddTaskEvent("repo", task.ID, "codex", TaskEvent{Type: "suggestion", Body: "State the invariant.", Citations: []string{"src/api.go:10@" + revision}, Uncertainty: "Callers outside this repository were not inspected."})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Events) != 2 || got.Events[1].ActorID != "codex" || got.Events[1].Sequence != 2 {
		t.Fatalf("unexpected events: %#v", got.Events)
	}
}
