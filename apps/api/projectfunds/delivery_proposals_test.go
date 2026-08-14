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
	p, err = s.RecordProgress("repo", o.ID, p.ID, "operator", ProgressInput{ExpectedVersion: 5, MilestoneID: "profile", Status: "active", Percent: 50, Summary: "Profile captured in the ordinary workspace", AgentCompute: 12, AccessState: "active", HandoffState: "ready", Evidence: []ExecutionReference{{Kind: "workspace", ID: "work-1", Revision: "abc", Summary: "bounded profile"}}, ForecastCompletionAt: ptrTime(time.Now().Add(30 * time.Minute))})
	if err != nil || p.Execution.Forecast.Percent != 25 || p.Execution.AgentCompute != 12 {
		t.Fatalf("progress = %+v, %v", p.Execution, err)
	}
	p, err = s.SubmitExpense("repo", o.ID, p.ID, "operator", ExpenseInput{ExpectedVersion: 6, MilestoneID: "profile", Amount: 1000, Description: "approved profiling tranche", Evidence: []ExecutionReference{{Kind: "check", ID: "check-1", Summary: "profile check passed"}}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.DecideExpense("repo", o.ID, p.ID, p.Execution.Expenses[0].ID, "steward", ExpenseDecisionInput{ExpectedVersion: 7, Approve: true})
	if err != nil || p.Execution.ApprovedExpenses != 1000 {
		t.Fatalf("expense = %+v, %v", p.Execution, err)
	}
	f, _ = s.Get("repo", f.ID)
	if f.Balances.Spent != 1000 || f.Balances.Reserved != 5000 {
		t.Fatalf("settled reservation = %+v", f.Balances)
	}
	p, err = s.RecordProgress("repo", o.ID, p.ID, "operator", ProgressInput{ExpectedVersion: 8, MilestoneID: "repair", Status: "inactive", Percent: 20, Summary: "credential was revoked before handoff", AccessState: "revoked", HandoffState: "failed", Evidence: []ExecutionReference{{Kind: "pull_request", ID: "17", Revision: "def", Summary: "legitimate partial contribution remains reviewable"}}})
	if err != nil || len(p.Execution.Blockers) != 3 {
		t.Fatalf("containment = %+v, %v", p.Execution, err)
	}
	if _, err = s.SubmitExpense("repo", o.ID, p.ID, "operator", ExpenseInput{ExpectedVersion: 9, MilestoneID: "repair", Amount: 100, Description: "must stop", Evidence: []ExecutionReference{}}); err != ErrConflict {
		t.Fatalf("blocked spending = %v", err)
	}
	p, err = s.ControlExecution("repo", o.ID, p.ID, "steward", ExecutionControlInput{ExpectedVersion: 9, Action: "replace", Reason: "preserve partial work under an already-authorized contributor", RecipientID: "maintainer"})
	if err != nil || p.Execution.ActiveRecipientID != "maintainer" || len(p.OperationalAuthority) != 0 || len(p.Execution.Progress) != 2 {
		t.Fatalf("replacement = %+v, %v", p, err)
	}
	outcome, _ := s.GetOutcome("repo", o.ID)
	if len(outcome.Delivery) != 1 || outcome.Delivery[0].Execution.ApprovedExpenses != 1000 || !hasBlocker(outcome.Blockers, "revoked_access") {
		t.Fatalf("outcome delivery report = %+v", outcome.Delivery)
	}
}

func ptrTime(v time.Time) *time.Time { return &v }

func TestPendingExpenseOverrunStopsFurtherSpending(t *testing.T) {
	x := DeliveryExecution{State: "active", Budget: 100, Expenses: []Expense{{Amount: 101, State: "pending"}}}
	deriveExecution(&x)
	if !hasBlocker(x.Blockers, "overrun") || !spendingBlocked(&x) || x.ApprovedExpenses != 0 {
		t.Fatalf("pending overrun was not contained: %+v", x)
	}
}

func hasBlocker(items []OutcomeBlocker, kind string) bool {
	for _, b := range items {
		if b.Kind == kind {
			return true
		}
	}
	return false
}
