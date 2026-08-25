package learningexercises

import (
	"errors"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
)

func TestLearnerControlsGroundedAgentHelp(t *testing.T) {
	s, _ := New(t.TempDir())
	grounding := []learningpathways.Resource{{Kind: "documentation", Label: "Guide", Path: "guide.md", Revision: "commit"}}
	a, err := s.Create("repo", "path", 1, "module", 0, "learner", "commit", learningpathways.Exercise{Title: "Practice"}, grounding)
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Help("repo", "path", a.ID, "learner", nil, HelpInput{Kind: "question", RecipientKind: "agent", RecipientID: "agent:repository:approved", AgentApprovalID: "approved", Body: "Explain the boundary", LearnerAuthorized: true})
	if err != nil || a.AgentStates["agent:repository:approved"] != "active" {
		t.Fatalf("agent invitation: %#v %v", a, err)
	}
	cite := []Citation{{Kind: "documentation", Label: "Guide", Path: "guide.md", Revision: "commit"}}
	_, err = s.Help("repo", "path", a.ID, "agent:repository:approved", nil, HelpInput{Kind: "guidance", GuidanceKind: "direct_action", Body: "I will do it", Citations: cite})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("agent direct action = %v", err)
	}
	_, err = s.Help("repo", "path", a.ID, "agent:repository:approved", nil, HelpInput{Kind: "guidance", GuidanceKind: "hint", Body: "Read the hidden assessment answer key", Citations: cite})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("protected answer = %v", err)
	}
	a, err = s.Help("repo", "path", a.ID, "learner", nil, HelpInput{Kind: "guide_agent", RecipientID: "agent:repository:approved", Body: "Explain concepts; do not edit."})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Help("repo", "path", a.ID, "learner", nil, HelpInput{Kind: "pause_agent", RecipientID: "agent:repository:approved"})
	if err != nil || a.AgentStates["agent:repository:approved"] != "paused" {
		t.Fatalf("pause: %#v %v", a.AgentStates, err)
	}
	_, err = s.Help("repo", "path", a.ID, "agent:repository:approved", nil, HelpInput{Kind: "guidance", GuidanceKind: "hint", Body: "Read the guide", Citations: cite})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("paused guidance = %v", err)
	}
	a, err = s.Help("repo", "path", a.ID, "learner", nil, HelpInput{Kind: "revoke_agent", RecipientID: "agent:repository:approved"})
	if err != nil || a.AgentStates["agent:repository:approved"] != "revoked" {
		t.Fatalf("revoke: %#v %v", a.AgentStates, err)
	}
}
