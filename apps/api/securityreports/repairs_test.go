package securityreports

import (
	"errors"
	"testing"
	"time"
)

func TestEmbargoedRepairsRetainCrossRepositoryWorkAndPrivateReview(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report, err := store.Create(CreateInput{
		ActorID: "reporter", Title: "private issue", Summary: "details",
		Contact: Contact{Channel: "email", Value: "safe@example.test"},
		Affected: []AffectedRepository{
			{RepositoryID: "repo-a", Versions: []string{"1.x"}},
			{RepositoryID: "repo-b", Versions: []string{"2.x"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	allow := func(string) bool { return true }
	firstReport, first, err := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo-a", Version: "1.x", Outcome: "remove unsafe parser path", BaseRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Branch: "refs/heads/embargo/opaque-a"}, allow)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo-b", Version: "2.x", Outcome: "update dependent binding", BaseRevision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Branch: "refs/heads/embargo/opaque-b", DependencyIDs: []string{first.ID}}, allow)
	if err != nil || second.DependencyIDs[0] != first.ID {
		t.Fatalf("dependency=%#v err=%v", second, err)
	}
	updated, session, err := store.StartRepairSession(report.ID, first.ID, "owner", "agent", "codex", "repair only the captured line", "credential-name", time.Now().Add(time.Hour), allow)
	if err != nil || session.State != "active" || len(updated.Repairs) != 2 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	updated, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "agent:codex", "branch_update", "bounded fix published", "cccccccccccccccccccccccccccccccccccccccc", "", allow)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "reviewer", "review", "validated exact repair", "cccccccccccccccccccccccccccccccccccccccc", "approve", allow)
	if err != nil {
		t.Fatal(err)
	}
	if records := updated.Repairs[0].Sessions[0].Records; len(records) != 2 || records[1].Decision != "approve" || records[0].ActorID != "agent:codex" {
		t.Fatalf("records=%#v", records)
	}
	_, revoked, err := store.RevokeRepairSession(report.ID, first.ID, session.ID, "owner", allow)
	if err != nil || revoked.State != "revoked" {
		t.Fatalf("revoked=%#v err=%v", revoked, err)
	}
	if _, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "agent:codex", "message", "late message", "", "", allow); !errors.Is(err, ErrTransition) {
		t.Fatalf("late record err=%v", err)
	}
	if len(firstReport.Audit) == 0 || first.Branch == "" {
		t.Fatal("repair did not retain private audit and isolated branch")
	}
}
