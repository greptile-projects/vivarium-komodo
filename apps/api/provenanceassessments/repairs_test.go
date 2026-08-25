package provenanceassessments

import "testing"

func TestRepairPreservesContextAndEnforcesCleanRoomBoundary(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Create(Assessment{RepositoryID: "repo", CandidateKind: "release_candidate", CandidateID: "v1", Revision: "rev-1", GraphID: "graph", PolicyID: "policy", PolicyVersion: 2, DistributionTargets: []string{"public"}, InputKeys: []InputKey{{Kind: "candidate", Reference: "repo", Revision: "rev-1"}}, Findings: []Finding{{ID: "finding", Kind: "unpermitted_origin", Subject: "file:x", Blocking: true}}, CreatedByID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Annotate(a.ID, "reviewer", "human", a.RevisionNumber, Annotation{Kind: "origin_evidence", FindingID: "finding", Body: "restricted source excerpt", Citation: "evidence:restricted", Audience: "restricted"})
	if err != nil {
		t.Fatal(err)
	}
	restricted := a.Annotations[0].ID
	projected := Project(Derive(a, a.InputKeys, s.now()), false)
	if projected.Annotations[0].Body != "" || projected.Annotations[0].Citation != "" {
		t.Fatalf("restricted evidence leaked through reader projection: %#v", projected.Annotations[0])
	}
	input := Repair{FindingID: "finding", Strategy: "reimplement", OwnerKind: "agent", OwnerID: "builder", AcceptanceCriteria: []string{"replacement passes provenance check"}, PermittedEvidenceIDs: []string{restricted}, CleanRoom: true, EvidenceReviewerIDs: []string{"reviewer"}, Links: []WorkLink{{Kind: "workspace", ResourceID: "workspace-1"}}}
	if _, _, err = s.CreateRepair(a.ID, "owner", a.RevisionNumber, input); err != ErrInvalid {
		t.Fatalf("restricted evidence crossed clean-room boundary: %v", err)
	}
	input.PermittedEvidenceIDs = nil
	a, repair, err := s.CreateRepair(a.ID, "owner", a.RevisionNumber, input)
	if err != nil {
		t.Fatal(err)
	}
	if repair.AffectedRevision != "rev-1" || repair.PolicyID != "policy" || repair.PolicyVersion != 2 || repair.OwnerID != "builder" {
		t.Fatalf("repair context not frozen: %#v", repair)
	}
	a, err = s.ProgressRepair(a.ID, repair.ID, "builder", "completed", "implemented without restricted source access", a.RevisionNumber)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DeliverRepair(a.ID, repair.ID, "builder", a.RevisionNumber, RepairDelivery{Revision: "rev-2", PullRequestID: "pull", CheckRunIDs: []string{"provenance"}, AuthorshipPreserved: false, Summary: "replacement"}); err != ErrInvalid {
		t.Fatalf("delivery rewrote authorship: %v", err)
	}
	completed, err := s.DeliverRepair(a.ID, repair.ID, "builder", a.RevisionNumber, RepairDelivery{Revision: "rev-2", PullRequestID: "pull", CheckRunIDs: []string{"provenance"}, AuthorshipPreserved: true, Summary: "clean implementation ready for ordinary review"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Repairs[0].Delivery.ActorID != "builder" {
		t.Fatalf("delivery attribution missing: %#v", completed.Repairs[0].Delivery)
	}
}
