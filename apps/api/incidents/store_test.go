package incidents

import (
	"testing"
	"time"
)

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

func TestInvestigationConnectsFindingsToTimeBoundedEvidence(t *testing.T) {
	store, _ := New(t.TempDir())
	item, _ := store.Create(CreateInput{RepositoryID: "repo", ActorID: "alice", Title: "Degraded", Summary: "Impact", Severity: "high", Roles: map[string]string{"commander": "alice"}, Affected: []AffectedEnvironment{{RepositoryID: "repo"}}})
	start, end := time.Now().Add(-time.Hour), time.Now()
	item, err := store.AddEvidence("repo", item.ID, "alice", Evidence{Kind: "logs", RepositoryID: "repo", ResourceID: "deploy", StartAt: &start, EndAt: &end, Title: "Error-rate increase", Audience: "participants"})
	if err != nil || len(item.Evidence) != 1 || item.Evidence[0].AttachedByID != "alice" {
		t.Fatalf("evidence = %#v, %v", item.Evidence, err)
	}
	item, err = store.AddFinding("repo", item.ID, "bob", Finding{Kind: "hypothesis", Body: "The rollout exhausted connections.", Query: "rate(errors[5m])", EvidenceIDs: []string{item.Evidence[0].ID}, Audience: "public"})
	if err != nil || len(item.Findings) != 1 || item.Findings[0].AuthorID != "bob" || len(item.Timeline) != 3 {
		t.Fatalf("finding = %#v, %v", item, err)
	}
	if _, err = store.AddFinding("repo", item.ID, "bob", Finding{Kind: "conclusion", Body: "Unsupported", EvidenceIDs: []string{"missing"}, Audience: "public"}); err != ErrInvalid {
		t.Fatalf("missing source = %v", err)
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

func TestDelegatedInvestigationIsBoundedControllableAndDurable(t *testing.T) {
	store, _ := New(t.TempDir())
	item, _ := store.Create(CreateInput{RepositoryID: "repo", ActorID: "alice", Title: "Degraded", Summary: "Impact", Severity: "high", Roles: map[string]string{"commander": "alice"}, Affected: []AffectedEnvironment{{RepositoryID: "repo", EnvironmentID: "production"}}})
	item, _ = store.AddEvidence("repo", item.ID, "alice", Evidence{Kind: "deployment", RepositoryID: "repo", ResourceID: "deploy", Title: "Failed rollout", Audience: "participants"})
	item, credential, err := store.StartInvestigation("repo", item.ID, InvestigationInput{ActorID: "alice", Agent: "codex", Mandate: "Identify the most likely cause; do not mitigate.", EvidenceIDs: []string{item.Evidence[0].ID}, Revisions: []Revision{{RepositoryID: "repo", CommitID: "abc123"}}, OperationalAccess: []OperationalAccess{{RepositoryID: "repo", Kind: "deployment_logs", ResourceID: "deploy"}}})
	if err != nil || credential == "" || len(item.Investigations) != 1 || item.Investigations[0].CredentialDigest == credential {
		t.Fatalf("delegation = %#v, credential=%q, err=%v", item.Investigations, credential, err)
	}
	_, investigation, err := store.AddInvestigationRecord(credential, "finding", "Failures begin after pool exhaustion.", "medium confidence", []string{item.Evidence[0].ID})
	if err != nil || investigation.Records[0].ActorID != "agent:codex" || len(investigation.Authority) != 4 {
		t.Fatalf("agent record = %#v, %v", investigation, err)
	}
	item, _ = store.ControlInvestigation("repo", item.ID, investigation.ID, "alice", "pause", "Validate against the prior release.")
	if _, _, err = store.AddInvestigationRecord(credential, "tool", "query logs", "", nil); err != ErrTransition {
		t.Fatalf("paused publish = %v", err)
	}
	item, _ = store.ControlInvestigation("repo", item.ID, investigation.ID, "alice", "guide", "Compare connection limits.")
	item, _ = store.ControlInvestigation("repo", item.ID, investigation.ID, "alice", "resume", "Continue.")
	item, _ = store.ControlInvestigation("repo", item.ID, investigation.ID, "alice", "cancel", "Responder ended delegation.")
	if _, _, err = store.InvestigationContext(credential); err != ErrNotFound {
		t.Fatalf("cancelled credential = %v", err)
	}
	restored, _ := store.Get("repo", item.ID)
	if restored.Investigations[0].State != "cancelled" || len(restored.Investigations[0].Records) != 5 || restored.Timeline[len(restored.Timeline)-1].Type != "investigation.cancel" {
		t.Fatalf("restored investigation = %#v", restored.Investigations[0])
	}
}
