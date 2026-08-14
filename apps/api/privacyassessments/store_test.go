package privacyassessments

import "testing"

func sample() Input {
	return Input{Revision: "candidate", TargetRevision: "target", Summary: "analytics changes", Comparisons: []FlowComparison{{Kind: "collection", Summary: "collect locale", Categories: []string{"locale"}, After: "on sign in", Evidence: []Location{{Path: "privacy.go", BlobID: "one"}}}}, Commitments: []CommitmentRef{{ID: "analytics", BaselineVersion: 1, CandidateVersion: 2, DataUseIDs: []string{"usage"}}}, Requirements: []Requirement{{ID: "privacy-owner", Kind: "owner_acknowledgement", OwnerIDs: []string{"owner"}, Rationale: "new collection"}, {ID: "notice", Kind: "notice", OwnerIDs: []string{"owner"}, Rationale: "tell users"}}, ResidualRisk: "locale may identify small groups"}
}
func TestAssessmentCollaborationAndStaleness(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Create("repo", "pull", "author", sample())
	if e != nil {
		t.Fatal(e)
	}
	a, e = s.AddEntry("repo", "pull", a.ID, "agent", EntryInput{Kind: "challenge", Body: "recipient is unclear", RequirementIDs: []string{"notice"}, Evidence: []Location{{Path: "privacy.go", BlobID: "one"}}})
	if e != nil {
		t.Fatal(e)
	}
	a, e = s.Acknowledge("repo", "pull", a.ID, "owner", "privacy-owner", "accept", "collection is minimized", "candidate")
	if e != nil {
		t.Fatal(e)
	}
	Derive(&a, "candidate", map[string]string{"privacy.go": "one"})
	if a.Stale || len(a.Blockers) != 1 || a.Blockers[0].RequirementID != "notice" {
		t.Fatalf("unexpected current projection: %+v", a)
	}
	Derive(&a, "new-candidate", map[string]string{"privacy.go": "two"})
	if !a.Stale || !a.Acknowledgements[0].Stale {
		t.Fatalf("source update must stale evidence and acceptance: %+v", a)
	}
}
func TestOnlyNamedOwnerAcknowledges(t *testing.T) {
	s, _ := New(t.TempDir())
	a, _ := s.Create("repo", "pull", "author", sample())
	if _, e := s.Acknowledge("repo", "pull", a.ID, "reader", "privacy-owner", "accept", "looks fine", "candidate"); e != ErrInvalid {
		t.Fatalf("got %v", e)
	}
}
