package projectfunds

import (
	"testing"
	"time"
)

func TestDeliverySelectionSeparatesAcceptanceReservationAndAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	f, err := s.Create("repo", "owner", Terms{Name: "delivery", StewardIDs: []string{"steward"}, FundingSources: []string{"provider"}, Unit: "USD", UnitKind: "currency", Limits: SpendingLimits{PerAllocation: 7000, PerRecipient: 9000, Total: 10000}, Approval: ApprovalRule{MinimumApprovals: 1, ApproverIDs: []string{"steward"}}, EligibleRecipients: []string{"human", "approved_agent_operator", "team"}, RefundPolicy: "return unreserved", LedgerVisibility: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	f, _ = s.Commit("repo", f.ID, "backer", TransferInput{Reference: "paid", Source: "provider", Amount: 10000, Settled: 10000, State: "settled"})
	o, err := s.CreateOutcome("repo", "steward", CreateOutcomeInput{FundID: f.ID, Terms: OutcomeTerms{Origin: OutcomeOrigin{Kind: "issue", ResourceID: "42"}, Title: "Ship repair", Scope: "repair parser", AcceptanceCriteria: []string{"checks pass"}, EvidenceRequirements: []string{"pull request"}, Budget: 7000, Deadline: time.Now().Add(time.Hour), ContributorEligibility: []string{"human", "approved_agent_operator"}, AllocationMethod: "complementary selection", CancellationTerms: "cancel unstarted work"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.SubmitDeliveryProposal("repo", o.ID, "operator", SubmitDeliveryProposalInput{Terms: DeliveryProposalTerms{RecipientKind: "approved_agent_operator", RecipientID: "operator", Approach: "profile then repair", Milestones: []ProposalMilestone{{ID: "profile", Approach: "measure", Cost: 2000, Deliverables: []string{"profile"}}, {ID: "repair", Approach: "implement", Cost: 4000, Deliverables: []string{"pull request"}}}, Cost: 6000, Availability: "this week", RequiredAccess: []string{"repository:write"}, RelevantWork: []WorkReference{{Kind: "pull_request", ID: "17", Description: "prior parser repair"}}}})
	if err != nil || len(p.OperationalAuthority) != 0 {
		t.Fatalf("proposal = %+v, %v", p, err)
	}
	if _, err = s.SelectDeliveryProposal("repo", o.ID, p.ID, "steward", SelectDeliveryProposalInput{ExpectedVersion: 1, Reason: "best evidence"}); err != ErrConflict {
		t.Fatalf("unaccepted selection = %v", err)
	}
	p, err = s.AcceptDeliveryProposal("repo", o.ID, p.ID, "operator", AcceptDeliveryProposalInput{ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.DiscloseProposalConflict("repo", o.ID, p.ID, "steward", DiscloseConflictInput{ExpectedVersion: 2, Detail: "reviewed prior work"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.ApproveDeliveryProposal("repo", o.ID, p.ID, "steward", ApproveDeliveryProposalInput{ExpectedVersion: 3})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.SelectDeliveryProposal("repo", o.ID, p.ID, "steward", SelectDeliveryProposalInput{ExpectedVersion: 4, Reason: "complements human review", Connections: []DeliveryConnection{{Kind: "proposal_task", ID: "task-1"}, {Kind: "delivery_team", ID: "team-1"}}})
	if err != nil || p.State != "selected" || len(p.OperationalAuthority) != 0 {
		t.Fatalf("selection = %+v, %v", p, err)
	}
	f, _ = s.Get("repo", f.ID)
	if f.Balances.Reserved != 6000 || f.Balances.Available != 4000 || len(f.Reservations) != 1 {
		t.Fatalf("fund = %+v", f)
	}
}
