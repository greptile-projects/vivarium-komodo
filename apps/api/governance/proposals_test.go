package governance

import (
	"errors"
	"testing"
	"time"
)

func TestGovernedProposalEligibilitySecretBallotsAndDeterministicTally(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	v, _ := s.Publish("repository", "repo", "owner", 0, validInput(), Preview{})
	v, _ = s.Approve("repository", "repo", "owner", "", 1)
	v, _ = s.Activate("repository", "repo", "owner", 1, Preview{})
	for _, person := range []string{"alice", "bob", "carol"} {
		v, _ = s.Invite("repository", "repo", "owner", 1, StandingInput{PrincipalID: person, Role: "maintainer", Evidence: []Evidence{{Kind: "review", Reference: "pull:1", Summary: "reviewed"}}})
		v, _ = s.Transition("repository", "repo", v.Standings[len(v.Standings)-1].ID, person, "accept", "")
	}
	p, e := s.OpenProposal("repository", "repo", "alice", ProposalInput{Kind: "technical_decision", Title: "Select cache", Summary: "Choose the durable cache", Scope: "runtime", DecisionClass: "policy", Alternatives: []ProposalAlternative{{ID: "a", Title: "A", Description: "Fast"}, {ID: "b", Title: "B", Description: "Safe"}}, Evidence: []ProposalEvidence{{Kind: "benchmark", Reference: "commit:abc", Summary: "Measured latency"}}, AffectedResources: []string{"branches:main"}, DisclosureRequirements: []string{"vendor interests"}, ImplementationEffects: []string{"migrate storage"}, SecretBallot: true, DiscussionHours: 24})
	if e != nil || p.CharterVersion != 1 || p.Quorum != 1 {
		t.Fatalf("open %#v %v", p, e)
	}
	p, e = s.Discuss("repository", "repo", p.ID, "agent:reviewer", "agent", "Prefer B", nil)
	if !errors.Is(e, ErrInvalid) {
		t.Fatalf("uncited agent analysis = %v", e)
	}
	p, e = s.Discuss("repository", "repo", p.ID, "agent:reviewer", "agent", "Prefer B", []ProposalEvidence{{Kind: "code", Reference: "commit:abc", Summary: "verified retry path"}})
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Cast("repository", "repo", p.ID, "alice", "b", "safer", false)
	if e != nil || p.Ballots[0].Choice != "" || p.Ballots[0].Reason != "" {
		t.Fatalf("secret ballot leaked %#v %v", p.Ballots, e)
	}
	_, e = s.Cast("repository", "repo", p.ID, "alice", "b", "again", false)
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("duplicate = %v", e)
	}
	_, e = s.Cast("repository", "repo", p.ID, "agent:reviewer", "b", "", false)
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("agent voted = %v", e)
	}
	_, _ = s.Cast("repository", "repo", p.ID, "bob", "", "conflict", true)
	for i := range v.Standings {
		if v.Standings[i].PrincipalID == "alice" {
			_, _ = s.Transition("repository", "repo", v.Standings[i].ID, "alice", "recuse", "")
		}
	}
	p, e = s.Finalize("repository", "repo", p.ID, true)
	if e != nil || p.Tally.Outcome != "rejected" || p.Tally.Abstentions != 1 || len(p.Tally.ExcludedBallots) != 1 || p.Tally.Digest == "" {
		t.Fatalf("tally %#v %v", p.Tally, e)
	}
	p, e = s.Contest("repository", "repo", p.ID, "carol", "Eligibility changed after the vote", []ProposalEvidence{{Kind: "standing", Reference: "alice", Summary: "Recusal timing is disputed"}})
	if e != nil || p.State != "contested" {
		t.Fatalf("contest %#v %v", p, e)
	}
}
