package runtimerepairs

import "testing"

func TestRepairVerificationAndProductionContainment(t *testing.T) {
	s, _ := New(t.TempDir())
	in := CreateInput{WorkspaceID: "w", ReplayID: "r", InvestigationID: "i", CauseClaimID: "c", Title: "repair", OwnerKind: "agent", OwnerID: "a", AffectedRevision: "old", AcceptanceCriteria: []string{"behavior changes"}, RegressionCriteria: []string{"scenario stays fixed"}}
	v, e := s.Create("repo", "owner", "p", "t", in)
	if e != nil || len(v.Authority) != 0 {
		t.Fatalf("create: %+v %v", v, e)
	}
	v, e = s.Verify("repo", v.ID, "owner", VerificationInput{PullRequestID: "pr", Revision: "new", ReplayAttemptID: "attempt", RequiredCheckRunIDs: []string{"check"}}, true)
	if e != nil || v.State != "verified_for_review" {
		t.Fatalf("verify: %+v %v", v, e)
	}
	v, e = s.Validate("repo", v.ID, "owner", ValidationInput{DeploymentID: "d", ReleaseID: "rel", Revision: "new", Stage: "canary", FailureAction: "restore_known_good", Signals: []Signal{{Name: "original error", EvidenceID: "signal", OriginalBehavior: "failed", ObservedValue: "still failed", Passed: false}}})
	if e != nil || v.State != "restore_known_good" || v.Validations[0].RequiredAction != "restore_known_good" {
		t.Fatalf("contain: %+v %v", v, e)
	}
}
