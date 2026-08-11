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

func TestRepairVerificationRetainsRevisionBoundDissentAndOverride(t *testing.T) {
	s, _ := New(t.TempDir())
	item, _ := s.Create(CreateInput{RepositoryID: "repo", ReporterID: "reporter", Title: "broken", ExpectedBehavior: "works", ObservedBehavior: "fails", Severity: "high", Environment: "linux", ReproductionSteps: []string{"run"}, Visibility: "repository"})
	_, repair, _ := s.CreateRepair("repo", item.ID, "owner", Repair{ReproductionID: "attempt", InvestigationID: "investigation", ConclusionEntryID: "conclusion", Revision: "base", AcceptanceCriteria: []string{"request succeeds"}, ProposalID: "proposal", TaskID: "task", OwnerKind: "agent", OwnerID: "codex"})
	_, verification, err := s.AddRepairVerification("repo", item.ID, repair.ID, "owner", RepairVerification{Revision: "candidate", PullRequestID: "pull", ReproductionAttemptID: "candidate-attempt", OriginalDefinitionDigest: "definition", CandidateDefinitionDigest: "definition", InputDigest: "inputs", RequiredChecks: []string{"test"}, CheckRunIDs: []string{"run"}, AcceptanceCriteria: []string{"request succeeds"}})
	if err != nil || verification.ID == "" {
		t.Fatalf("verification = %#v, %v", verification, err)
	}
	_, verification, err = s.DecideRepairVerification("repo", item.ID, repair.ID, verification.ID, "reporter", "rejected", "still fails in my workflow", "candidate", "evidence")
	if err != nil {
		t.Fatal(err)
	}
	_, verification, err = s.DecideRepairVerification("repo", item.ID, repair.ID, verification.ID, "owner", "override", "bounded rollout accepted after independent confirmation", "candidate", "evidence")
	if err != nil || len(verification.Decisions) != 2 || verification.Decisions[0].Kind != "rejected" || verification.Decisions[1].Kind != "override" {
		t.Fatalf("decisions = %#v, %v", verification.Decisions, err)
	}
	reopened, _ := New(s.root)
	durable, _ := reopened.Get("repo", item.ID)
	if len(durable.Repairs[0].Verifications[0].Decisions) != 2 {
		t.Fatalf("dissent was rewritten: %#v", durable.Repairs[0].Verifications)
	}
}
