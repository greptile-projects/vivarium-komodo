package incidents

import "testing"

func TestIncidentRetainsCoordinationTimelineAndAcknowledgements(t *testing.T) {
	store, _ := New(t.TempDir())
	item, err := store.Create(CreateInput{RepositoryID: "repo", ActorID: "alice", Title: "API unavailable", Summary: "Elevated errors", Severity: "critical", Roles: map[string]string{"commander": "alice", "operations": "bob"}, Affected: []AffectedEnvironment{{RepositoryID: "repo", EnvironmentID: "production"}}})
	if err != nil || item.Status != "declared" || len(item.Timeline) != 1 || len(item.Followers) != 1 {
		t.Fatalf("declared incident = %#v, %v", item, err)
	}
	item, _ = store.Update("repo", item.ID, UpdateInput{ActorID: "bob", Status: "investigating"})
	item, _ = store.AddUpdate("repo", item.ID, "bob", "public", "We are investigating elevated errors.")
	update := item.Timeline[len(item.Timeline)-1]
	item, _ = store.Follow("repo", item.ID, "bob", true)
	item, _ = store.Acknowledge("repo", item.ID, "alice", update.Sequence)
	if item.Status != "investigating" || len(item.Followers) != 2 || len(item.Acknowledgements) != 1 || item.Acknowledgements[0].UpdateSequence != update.Sequence || len(item.Timeline) != 5 {
		t.Fatalf("coordinated incident = %#v", item)
	}
	restored, _ := store.Get("repo", item.ID)
	if restored.Timeline[2].Audience != "public" || restored.Timeline[2].ActorID != "bob" {
		t.Fatalf("restored timeline = %#v", restored.Timeline)
	}
}

func TestResolvedIncidentRejectsFurtherUpdates(t *testing.T) {
	store, _ := New(t.TempDir())
	item, _ := store.Create(CreateInput{RepositoryID: "repo", ActorID: "alice", Title: "Degraded", Summary: "Impact", Severity: "high", Roles: map[string]string{"commander": "alice"}, Affected: []AffectedEnvironment{{RepositoryID: "repo"}}})
	item, _ = store.Update("repo", item.ID, UpdateInput{ActorID: "alice", Status: "resolved"})
	if _, err := store.AddUpdate("repo", item.ID, "alice", "participants", "late"); err != ErrTransition {
		t.Fatalf("resolved update error = %v", err)
	}
}
