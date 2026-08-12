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

func TestApprovedDecisionReceiptRoutesWithoutGrantingAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	v, _ := s.Publish("repository", "repo", "owner", 0, validInput(), Preview{})
	v, _ = s.Approve("repository", "repo", "owner", "", 1)
	v, _ = s.Activate("repository", "repo", "owner", 1, Preview{})
	v, _ = s.Invite("repository", "repo", "owner", 1, StandingInput{PrincipalID: "alice", Role: "maintainer", Evidence: []Evidence{{Kind: "review", Reference: "pull:1", Summary: "reviewed"}}})
	_, _ = s.Transition("repository", "repo", v.Standings[0].ID, "alice", "accept", "")
	p, _ := s.OpenProposal("repository", "repo", "alice", ProposalInput{Kind: "technical_decision", Title: "Adopt cache", Summary: "Use a bounded cache", Scope: "runtime", DecisionClass: "policy", Alternatives: []ProposalAlternative{{ID: "yes", Title: "Adopt", Description: "Proceed", Effects: []string{"update docs"}}, {ID: "no", Title: "Decline", Description: "Keep current"}}, Evidence: []ProposalEvidence{{Kind: "benchmark", Reference: "commit:abc", Summary: "Measured"}}, AffectedResources: []string{"branches:main"}, ImplementationEffects: []string{"change cache"}, DiscussionHours: 1})
	_, _ = s.Cast("repository", "repo", p.ID, "alice", "yes", "", false)
	p, err := s.Finalize("repository", "repo", p.ID, true)
	if err != nil || p.DecisionReceipt == nil || p.DecisionReceipt.Digest == "" || p.DecisionReceipt.AuthorityGranted || p.Implementation.State != "awaiting_owner_approval" || len(p.Implementation.OperationalAuthority) != 0 {
		t.Fatalf("receipt %#v implementation %#v err %v", p.DecisionReceipt, p.Implementation, err)
	}
	in := ImplementationInput{ExpectedReceiptDigest: p.DecisionReceipt.Digest, ArtifactKind: "task_plan", ResourceRef: "proposal:delivery-plan", Detail: "Owner created the ordinary task plan", Scope: p.DecisionReceipt.Scope, AffectedResources: p.DecisionReceipt.AffectedResources, ImplementationEffects: p.DecisionReceipt.ImplementationEffects}
	p, err = s.RecordImplementation("repository", "repo", p.ID, "owner", in)
	if err != nil || p.Implementation.State != "routed" || !p.Implementation.Steps[0].OwnerApproval || p.Implementation.Steps[0].ResourceRef != "proposal:delivery-plan" {
		t.Fatalf("routed %#v err %v", p.Implementation, err)
	}
	in.MaterialChange = true
	in.Detail = "Cost and protected effects changed"
	p, err = s.RecordImplementation("repository", "repo", p.ID, "owner", in)
	if err != nil || !p.Implementation.AmendmentRequired || p.Implementation.Steps[1].Status != "blocked_amendment_required" {
		t.Fatalf("amendment %#v err %v", p.Implementation, err)
	}
}
