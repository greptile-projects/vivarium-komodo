package reviewrouting

import (
	"testing"
	"time"
)

func TestSuggestionsAndBoundedAssignmentLifecycle(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 8, 25, 18, 0, 0, 0, time.UTC) }
	areas := []Area{{ID: "security", Expertise: []string{"security"}, Paths: []string{"auth.go"}, Questions: []string{"Are sessions revoked?"}}}
	candidates := []Candidate{
		{ParticipantID: "reviewer", Kind: "human", Expertise: []string{"security"}, CodeOwnership: true, Available: true, Capacity: 2, Evidence: []Evidence{{Kind: "ownership", Reference: "CODEOWNERS@abc", Summary: "owns auth.go", Accessible: true}}},
		{ParticipantID: "busy", Kind: "human", ProjectKnowledge: true, Available: true, CurrentLoad: 2, Capacity: 2},
		{ParticipantID: "agent", Kind: "agent", Expertise: []string{"security"}, Available: true, Evidence: []Evidence{{Kind: "private-session", Reference: "hidden", Accessible: false}}},
	}
	x, err := s.Suggest("repo", "pull", "candidate", 1, areas, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Suggestions) != 3 || !x.Suggestions[0].Eligible || x.Suggestions[1].Eligible || x.Suggestions[2].Eligible {
		t.Fatalf("suggestions = %#v", x.Suggestions)
	}
	deadline := time.Now().UTC().Add(24 * time.Hour)
	x, err = s.Invite("repo", "pull", "maintainer", 1, "candidate", areas[0], candidates[0], &deadline, "team-lead", "Own the security review", "")
	if err != nil {
		t.Fatal(err)
	}
	a := x.Assignments[0]
	if a.State != "invited" || a.AuthorityNotice == "" || len(a.Scope) != 1 {
		t.Fatalf("assignment = %#v", a)
	}
	x, err = s.Transition("repo", "pull", a.ID, "reviewer", "recused", "I authored the affected session code", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.ReassignmentAreas) != 1 || x.ReassignmentAreas[0] != "security" {
		t.Fatalf("reassignment = %#v", x)
	}
	if _, err = s.Transition("repo", "pull", a.ID, "stranger", "accepted", "take it", false); err == nil {
		t.Fatal("uninvited participant accepted assignment")
	}
}
