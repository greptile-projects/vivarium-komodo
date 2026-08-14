package projectfunds

import (
	"testing"
	"time"
)

func TestMilestoneAcceptanceSettlesEvidenceAndRecoversWithoutAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	f, err := s.Create("repo", "owner", Terms{Name: "results", StewardIDs: []string{"steward"}, FundingSources: []string{"provider"}, Unit: "USD", UnitKind: "currency", Limits: SpendingLimits{PerAllocation: 1000, PerRecipient: 1000, Total: 1000}, Approval: ApprovalRule{MinimumApprovals: 1, ApproverIDs: []string{"reviewer"}}, EligibleRecipients: []string{"human"}, RefundPolicy: "refund failed or rejected awards", LedgerVisibility: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	f, _ = s.Commit("repo", f.ID, "backer", TransferInput{Reference: "paid", Source: "provider", Amount: 1000, Settled: 1000, State: "settled"})
	o, err := s.CreateOutcome("repo", "steward", CreateOutcomeInput{FundID: f.ID, Terms: OutcomeTerms{Origin: OutcomeOrigin{Kind: "issue", ResourceID: "1"}, Title: "Ship accepted result", Scope: "ordinary reviewed delivery", AcceptanceCriteria: []string{"measure passes"}, EvidenceRequirements: []string{"commit and release"}, Budget: 1000, Deadline: time.Now().Add(time.Hour), ContributorEligibility: []string{"human"}, AllocationMethod: "original milestone allocation", CancellationTerms: "release rejected work", Milestones: []Milestone{{ID: "ship", Name: "Ship", Budget: 1000, AcceptanceCriteria: []string{"measure passes"}, EvidenceRequirements: []string{"commit", "authorship", "handoff", "check", "preview", "release", "deployment", "outcome measure"}, ReviewerIDs: []string{"reviewer"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.SubmitDeliveryProposal("repo", o.ID, "recipient", SubmitDeliveryProposalInput{Terms: DeliveryProposalTerms{RecipientKind: "human", RecipientID: "recipient", Approach: "deliver through ordinary review", Milestones: []ProposalMilestone{{ID: "ship", Approach: "implement and measure", Cost: 1000, Deliverables: []string{"release"}}}, Cost: 1000, Availability: "now"}})
	if err != nil {
		t.Fatal(err)
	}
	p, _ = s.AcceptDeliveryProposal("repo", o.ID, p.ID, "recipient", AcceptDeliveryProposalInput{ExpectedVersion: p.Version})
	p, _ = s.ApproveDeliveryProposal("repo", o.ID, p.ID, "reviewer", ApproveDeliveryProposalInput{ExpectedVersion: p.Version})
	p, err = s.SelectDeliveryProposal("repo", o.ID, p.ID, "steward", SelectDeliveryProposalInput{ExpectedVersion: p.Version, Reason: "evidence-bound allocation"})
	if err != nil {
		t.Fatal(err)
	}
	evidence := []ExecutionReference{{Kind: "commit", ID: "abc", Revision: "abc", Summary: "authored result"}, {Kind: "authorship", ID: "abc:recipient", Summary: "recipient authored commit"}, {Kind: "handoff", ID: "handoff-1", Summary: "accepted handoff"}, {Kind: "check", ID: "check-1", Summary: "checks pass"}, {Kind: "preview", ID: "preview-1", Summary: "reviewed preview"}, {Kind: "release", ID: "release-1", Revision: "abc", Summary: "ordinary release"}, {Kind: "deployment", ID: "deployment-1", Revision: "abc", Summary: "ordinary deployment"}, {Kind: "outcome_measure", ID: "measure-1", Summary: "declared measure passes"}}
	p, err = s.ReviewMilestone("repo", o.ID, p.ID, "ship", "reviewer", MilestoneReviewInput{ExpectedVersion: p.Version, Decision: "correction_requested", Rationale: "deployment evidence needs attribution", Dissent: []string{"recipient believes the release is sufficient"}, Evidence: evidence})
	if err != nil || p.Execution.Settlements[0].State != "correction_requested" {
		t.Fatalf("correction = %+v, %v", p.Execution.Settlements, err)
	}
	p, err = s.ReviewMilestone("repo", o.ID, p.ID, "ship", "reviewer", MilestoneReviewInput{ExpectedVersion: p.Version, Decision: "partial_award", Award: 400, Rationale: "verified shipped portion", Evidence: evidence})
	if err != nil || p.Execution.Settlements[0].Awarded != 400 {
		t.Fatalf("partial = %+v, %v", p.Execution.Settlements, err)
	}
	f, _ = s.Get("repo", f.ID)
	if f.Balances.Spent != 400 || f.Balances.Reserved != 600 {
		t.Fatalf("partial balances = %+v", f.Balances)
	}
	p, err = s.ReviewMilestone("repo", o.ID, p.ID, "ship", "reviewer", MilestoneReviewInput{ExpectedVersion: p.Version, Decision: "disputed", Rationale: "measure interpretation is disputed", Dissent: []string{"recipient contests the baseline"}, Evidence: evidence})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.RecoverMilestone("repo", o.ID, p.ID, "ship", "recipient", MilestoneRecoveryInput{ExpectedVersion: p.Version, Action: "appeal", Rationale: "the retained baseline resolves the dispute"})
	if err != nil || p.Execution.Settlements[0].State != "appealed" {
		t.Fatalf("appeal = %+v, %v", p.Execution.Settlements, err)
	}
	p, err = s.ReviewMilestone("repo", o.ID, p.ID, "ship", "reviewer", MilestoneReviewInput{ExpectedVersion: p.Version, Decision: "accepted", Rationale: "all original criteria and measures are demonstrated", Evidence: evidence})
	if err != nil || p.Execution.Settlements[0].Awarded != 1000 || len(p.OperationalAuthority) != 0 {
		t.Fatalf("acceptance = %+v, %v", p, err)
	}
	f, _ = s.Get("repo", f.ID)
	if f.Balances.Spent != 1000 || f.Balances.Reserved != 0 {
		t.Fatalf("accepted balances = %+v", f.Balances)
	}
	p, err = s.RecoverMilestone("repo", o.ID, p.ID, "ship", "steward", MilestoneRecoveryInput{ExpectedVersion: p.Version, Action: "payment_failed", Rationale: "provider rejected the transfer"})
	if err != nil {
		t.Fatal(err)
	}
	f, _ = s.Get("repo", f.ID)
	if f.Balances.Spent != 0 || f.Balances.Reserved != 1000 {
		t.Fatalf("failed payment did not restore reservation: %+v", f.Balances)
	}
	p, err = s.RecoverMilestone("repo", o.ID, p.ID, "ship", "steward", MilestoneRecoveryInput{ExpectedVersion: p.Version, Action: "retry_payment", Rationale: "provider confirmed retry"})
	if err != nil || p.Execution.Settlements[0].Payment != "settled" {
		t.Fatalf("retry = %+v, %v", p.Execution.Settlements, err)
	}
}
