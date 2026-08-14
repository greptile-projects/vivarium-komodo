package privacydrift

import (
	"testing"
	"time"
)

func TestSanitizedDriftContainmentAndRepairLedger(t *testing.T) {
	s, _ := New(t.TempDir())
	m, err := s.CreateMonitor("repo", "owner", MonitorInput{Name: "production contract", CommitmentID: "commitment", CommitmentVersion: 2, DataUseIDs: []string{"analytics"}, SignalKinds: []string{"failed_deletion", "unexpected_recipient"}, ReleaseID: "release-7", ReleaseRevision: "abc123", EnvironmentID: "production-eu", ExtensionID: "metrics", OwnerIDs: []string{"privacy-owner"}, ParticipantIDs: []string{"processor-owner"}, RetentionDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	d, err := s.Report("repo", "collaborator", SignalInput{MonitorID: m.ID, Kind: "failed_deletion", DataUseID: "analytics", Observed: "aggregate deletion completion fell below the declared threshold", Expected: "all rows deleted within seven days", Evidence: Evidence{SignalReference: "permitted-monitor:deletion-lag", Metric: "overdue_deletion_count", AggregateCount: 12, WindowStart: now.Add(-time.Hour), WindowEnd: now, Digest: "sha256:aggregate", Summary: "Twelve pseudonymous rows exceeded the deletion window; no subject identifiers retained.", Sanitized: true}})
	if err != nil || d.ReleaseID != "release-7" || d.EnvironmentID != "production-eu" || d.AuthorityGranted {
		t.Fatalf("bad drift: %+v %v", d, err)
	}
	if _, err = s.Report("repo", "collaborator", SignalInput{MonitorID: m.ID, Kind: "failed_deletion", DataUseID: "analytics", Observed: "raw", Expected: "deleted", Evidence: Evidence{Sanitized: false}}); err == nil {
		t.Fatal("unsanitized evidence accepted")
	}
	d, event, err := s.AddEvent("repo", d.ID, "privacy-owner", "contain", "requested suspension of the affected deletion worker input", "", "", nil)
	if err != nil || d.State != "contained" || event.ActorID != "privacy-owner" {
		t.Fatalf("containment missing: %+v %v", d, err)
	}
	if _, _, err = s.AddEvent("repo", d.ID, "privacy-owner", "notify", "notify unrelated user", "", "", []string{"stranger"}); err == nil {
		t.Fatal("unexpected recipient notified")
	}
	d, err = s.LinkRepair("repo", d.ID, "owner", Repair{OwnerKind: "agent", OwnerID: "codex", ProposalID: "proposal", TaskID: "task", BaseRevision: "abc123", AcceptanceCriteria: []string{"deletion verification passes"}})
	if err != nil || d.State != "repairing" || d.Repair.TaskID != "task" {
		t.Fatalf("repair missing: %+v %v", d, err)
	}
}
