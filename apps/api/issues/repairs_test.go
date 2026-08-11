package issues

import "testing"

func TestRepairRetainsEvidenceAndLinkedReview(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.Create(CreateInput{RepositoryID: "repo", ReporterID: "reporter", Title: "Crash", ExpectedBehavior: "request succeeds", ObservedBehavior: "request exits", Severity: "high", Environment: "linux", ReproductionSteps: []string{"run it"}, Visibility: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	updated, repair, err := s.CreateRepair("repo", item.ID, "maintainer", Repair{ReproductionID: "attempt", InvestigationID: "investigation", ConclusionEntryID: "conclusion", Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AcceptanceCriteria: []string{"reported request succeeds"}, ProposalID: "proposal", TaskID: "task", OwnerKind: "agent", OwnerID: "codex"})
	if err != nil || len(updated.Repairs) != 1 || repair.Revision == "" {
		t.Fatalf("repair=%#v issue=%#v err=%v", repair, updated, err)
	}
	updated, linked, err := s.LinkRepairPullRequest("repo", item.ID, repair.ID, "pull", "contributor")
	if err != nil || linked.PullRequestID != "pull" || len(updated.Relationships) != 1 || updated.History[len(updated.History)-1].Type != "repair.pull_request_linked" {
		t.Fatalf("repair=%#v issue=%#v err=%v", linked, updated, err)
	}
	if _, _, err := s.LinkRepairPullRequest("repo", item.ID, repair.ID, "different", "contributor"); err != ErrConflict {
		t.Fatalf("replacement err=%v", err)
	}
}
