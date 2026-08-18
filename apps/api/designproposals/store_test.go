package designproposals

import "testing"

func proposalInput() Input {
	return Input{Title: "Safer publish flow", Origin: Origin{Kind: "accessibility_finding", ID: "finding-1", Revision: "r1"}, UserGoal: "Publish confidently without losing context", Journeys: []Journey{{Name: "publish", Steps: []string{"review", "confirm"}, Outcome: "change is published"}}, States: []State{{Name: "review", Trigger: "publish", Behavior: "summarize impact", Content: "Review changes"}}, Content: []string{"Review changes", "Publish"}, Constraints: []Constraint{{Kind: "accessibility", Requirement: "keyboard complete"}}, Alternatives: []Alternative{{Name: "immediate publish", Tradeoff: "fast but error prone", Reason: "rejected after finding"}}, SuccessMeasures: []Measure{{Name: "completion", Target: "95%"}}, AffectedComponents: []string{"PublishDialog"}, Evidence: []Evidence{{ID: "public-1", Kind: "issue", Reference: "issue-1", Summary: "reported confusion", Audience: "repository"}, {ID: "private-1", Kind: "research", Reference: "study-1", Summary: "restricted interview", Audience: "private_research"}}, Uncertainty: []string{"copy may be too long"}, ChangeReason: "initial review"}
}

func TestReviewWorkflowPreservesRevisionAndEvidenceBoundaries(t *testing.T) {
	s, _ := New(t.TempDir())
	p, e := s.Create("repo", "designer", proposalInput())
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Revisions[0].Evidence) != 1 || p.PrivateEvidenceCount != 1 {
		t.Fatalf("private evidence propagated: %#v", p)
	}
	p, e = s.Get("repo", p.ID)
	if e != nil || p.PrivateEvidenceCount != 1 {
		t.Fatalf("restricted evidence was not durably retained: %#v, %v", p, e)
	}
	if _, e = s.Invite("repo", p.ID, "designer", "agent-1", "agent", "reviewer", []string{"private-1"}, 1); e != ErrInvalid {
		t.Fatalf("private grounding accepted: %v", e)
	}
	p, e = s.Invite("repo", p.ID, "designer", "agent-1", "agent", "reviewer", []string{"public-1"}, 1)
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.AddArtifact("repo", p.ID, "developer", ArtifactInput{Kind: "prototype", Title: "Confirmation", ProposalRevision: 1, Frames: []Frame{{Name: "review", Format: "html", Body: "<button>Publish</button>"}}, Interactions: []Interaction{{Trigger: "activate", Action: "publish", Result: "success"}}, EvidenceIDs: []string{"public-1"}, Uncertainty: []string{"mobile density"}, ChangeReason: "make behavior testable"})
	if e != nil {
		t.Fatal(e)
	}
	a := p.Artifacts[0]
	p, e = s.Comment("repo", p.ID, "user-1", "artifact", a.ID, "The purpose is unclear", "dissent", "I only tested keyboard use", 1, []string{"public-1"})
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.RequestAcknowledgement("repo", p.ID, "designer", "component-owner", 1)
	if e != nil {
		t.Fatal(e)
	}
	ack := p.Acknowledgements[0]
	in := proposalInput()
	in.Content = []string{"Review affected people and changes", "Publish"}
	in.ChangeReason = "address user dissent"
	p, e = s.Revise("repo", p.ID, "designer", 1, in)
	if e != nil {
		t.Fatal(e)
	}
	if p.Acknowledgements[0].Current {
		t.Fatal("old acknowledgement remained current")
	}
	if _, e = s.Respond("repo", p.ID, ack.ID, "component-owner", "acknowledged", "looks good"); e != ErrConflict {
		t.Fatalf("stale acknowledgement accepted: %v", e)
	}
	if len(p.Comments) != 1 || p.Comments[0].Stance != "dissent" || p.Artifacts[0].Revisions[0].ProposalRevision != 1 {
		t.Fatalf("history lost: %#v", p)
	}
}
