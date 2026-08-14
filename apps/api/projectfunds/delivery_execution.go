package projectfunds

import (
	"strings"
	"time"
)

// DeliveryExecution is an inspectable account of selected work. References are
// observations of ordinary platform resources and never confer their authority.
type DeliveryExecution struct {
	State             string                `json:"state"`
	ActiveRecipientID string                `json:"active_recipient_id"`
	Budget            int64                 `json:"budget"`
	MilestoneCount    int                   `json:"milestone_count"`
	ApprovedExpenses  int64                 `json:"approved_expenses"`
	AgentCompute      int64                 `json:"agent_compute"`
	Progress          []ProgressObservation `json:"progress"`
	Expenses          []Expense             `json:"expenses"`
	Changes           []ExecutionChange     `json:"changes"`
	Blockers          []OutcomeBlocker      `json:"blockers"`
	Forecast          Forecast              `json:"forecast"`
}
type ExecutionReference struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty"`
	Revision string `json:"revision,omitempty"`
	Summary  string `json:"summary"`
}
type ProgressObservation struct {
	ID           string               `json:"id"`
	ActorID      string               `json:"actor_id"`
	MilestoneID  string               `json:"milestone_id"`
	Status       string               `json:"status"`
	Percent      int                  `json:"percent"`
	Summary      string               `json:"summary"`
	Evidence     []ExecutionReference `json:"evidence"`
	AgentCompute int64                `json:"agent_compute"`
	AccessState  string               `json:"access_state"`
	HandoffState string               `json:"handoff_state"`
	ObservedAt   time.Time            `json:"observed_at"`
}
type Expense struct {
	ID           string               `json:"id"`
	ActorID      string               `json:"actor_id"`
	MilestoneID  string               `json:"milestone_id"`
	Amount       int64                `json:"amount"`
	Description  string               `json:"description"`
	Evidence     []ExecutionReference `json:"evidence"`
	State        string               `json:"state"`
	ApprovedByID string               `json:"approved_by_id,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
	DecidedAt    *time.Time           `json:"decided_at,omitempty"`
}
type ExecutionChange struct {
	ID              string    `json:"id"`
	ActorID         string    `json:"actor_id"`
	Kind            string    `json:"kind"`
	Reason          string    `json:"reason"`
	FromRecipientID string    `json:"from_recipient_id,omitempty"`
	ToRecipientID   string    `json:"to_recipient_id,omitempty"`
	PreviousBudget  int64     `json:"previous_budget,omitempty"`
	Budget          int64     `json:"budget,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
type Forecast struct {
	Percent         int        `json:"percent"`
	Status          string     `json:"status"`
	CompletionAt    *time.Time `json:"completion_at,omitempty"`
	RemainingBudget int64      `json:"remaining_budget"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}
type ProgressInput struct {
	ExpectedVersion      int64                `json:"expected_version"`
	MilestoneID          string               `json:"milestone_id"`
	Status               string               `json:"status"`
	Percent              int                  `json:"percent"`
	Summary              string               `json:"summary"`
	Evidence             []ExecutionReference `json:"evidence"`
	AgentCompute         int64                `json:"agent_compute"`
	AccessState          string               `json:"access_state"`
	HandoffState         string               `json:"handoff_state"`
	ForecastCompletionAt *time.Time           `json:"forecast_completion_at"`
}
type ExpenseInput struct {
	ExpectedVersion int64                `json:"expected_version"`
	MilestoneID     string               `json:"milestone_id"`
	Amount          int64                `json:"amount"`
	Description     string               `json:"description"`
	Evidence        []ExecutionReference `json:"evidence"`
}
type ExpenseDecisionInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Approve         bool   `json:"approve"`
	Reason          string `json:"reason"`
}
type ExecutionControlInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	RecipientID     string `json:"recipient_id"`
	Budget          int64  `json:"budget"`
}

var workKinds = map[string]bool{"proposal_task": true, "session": true, "workspace": true, "fork": true, "pull_request": true, "check": true, "preview": true, "delivery_team": true}
var progressStates = map[string]bool{"not_started": true, "active": true, "blocked": true, "inactive": true, "completed": true}
var accessStates = map[string]bool{"active": true, "limited": true, "revoked": true}
var handoffStates = map[string]bool{"not_required": true, "pending": true, "ready": true, "accepted": true, "failed": true}

func validEvidence(xs []ExecutionReference) bool {
	for _, x := range xs {
		if !workKinds[x.Kind] || strings.TrimSpace(x.ID) == "" || strings.TrimSpace(x.Summary) == "" {
			return false
		}
	}
	return len(xs) <= 100
}
func milestoneExists(p DeliveryProposal, id string) bool {
	for _, m := range p.Terms.Milestones {
		if m.ID == id {
			return true
		}
	}
	return false
}
func spendingBlocked(x *DeliveryExecution) bool {
	if x.State != "active" {
		return true
	}
	for _, b := range x.Blockers {
		if b.Kind == "overrun" || b.Kind == "revoked_access" || b.Kind == "failed_handoff" || b.Kind == "inactive" {
			return true
		}
	}
	return false
}
func deriveExecution(x *DeliveryExecution) {
	x.ApprovedExpenses, x.AgentCompute, x.Blockers = 0, 0, []OutcomeBlocker{}
	latest := map[string]ProgressObservation{}
	for _, p := range x.Progress {
		latest[p.MilestoneID] = p
		x.AgentCompute += p.AgentCompute
	}
	total := 0
	var updated *time.Time
	for _, p := range latest {
		total += p.Percent
		t := p.ObservedAt
		if updated == nil || t.After(*updated) {
			updated = &t
		}
		if p.AccessState == "revoked" {
			x.Blockers = append(x.Blockers, OutcomeBlocker{Kind: "revoked_access", Detail: "the recipient reported revoked access", ResourceID: p.MilestoneID})
		}
		if p.HandoffState == "failed" {
			x.Blockers = append(x.Blockers, OutcomeBlocker{Kind: "failed_handoff", Detail: "a required handoff failed", ResourceID: p.MilestoneID})
		}
		if p.Status == "inactive" {
			x.Blockers = append(x.Blockers, OutcomeBlocker{Kind: "inactive", Detail: "a milestone is inactive", ResourceID: p.MilestoneID})
		}
	}
	for _, e := range x.Expenses {
		if e.State == "approved" {
			x.ApprovedExpenses += e.Amount
		}
	}
	if x.ApprovedExpenses > x.Budget {
		x.Blockers = append(x.Blockers, OutcomeBlocker{Kind: "overrun", Detail: "approved expenses exceed the execution budget"})
	}
	count := x.MilestoneCount
	if count == 0 {
		count = len(latest)
	}
	if count > 0 {
		x.Forecast.Percent = total / count
	}
	x.Forecast.RemainingBudget = x.Budget - x.ApprovedExpenses
	x.Forecast.UpdatedAt = updated
	if x.State == "cancelled" {
		x.Forecast.Status = "cancelled"
	} else if len(x.Blockers) > 0 || x.State == "paused" {
		x.Forecast.Status = "blocked"
	} else if x.Forecast.Percent == 100 {
		x.Forecast.Status = "complete"
	} else {
		x.Forecast.Status = "on_track"
	}
}
func (s *Store) RecordProgress(repo, oid, pid, actor string, in ProgressInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	if p.State != "selected" || p.Execution == nil || actor != p.Execution.ActiveRecipientID {
		return p, ErrForbidden
	}
	if !milestoneExists(p, in.MilestoneID) || !progressStates[in.Status] || !accessStates[in.AccessState] || !handoffStates[in.HandoffState] || in.Percent < 0 || in.Percent > 100 || strings.TrimSpace(in.Summary) == "" || in.AgentCompute < 0 || !validEvidence(in.Evidence) {
		return p, ErrInvalid
	}
	if p.Execution.State == "cancelled" {
		return p, ErrConflict
	}
	now := s.now().UTC()
	p.Execution.Progress = append(p.Execution.Progress, ProgressObservation{ID: id(), ActorID: actor, MilestoneID: in.MilestoneID, Status: in.Status, Percent: in.Percent, Summary: strings.TrimSpace(in.Summary), Evidence: in.Evidence, AgentCompute: in.AgentCompute, AccessState: in.AccessState, HandoffState: in.HandoffState, ObservedAt: now})
	p.Execution.Forecast.CompletionAt = in.ForecastCompletionAt
	deriveExecution(p.Execution)
	p.Version++
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) SubmitExpense(repo, oid, pid, actor string, in ExpenseInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	if p.Execution == nil || actor != p.Execution.ActiveRecipientID {
		return p, ErrForbidden
	}
	deriveExecution(p.Execution)
	if spendingBlocked(p.Execution) {
		return p, ErrConflict
	}
	if !milestoneExists(p, in.MilestoneID) || in.Amount <= 0 || strings.TrimSpace(in.Description) == "" || !validEvidence(in.Evidence) {
		return p, ErrInvalid
	}
	now := s.now().UTC()
	p.Execution.Expenses = append(p.Execution.Expenses, Expense{ID: id(), ActorID: actor, MilestoneID: in.MilestoneID, Amount: in.Amount, Description: strings.TrimSpace(in.Description), Evidence: in.Evidence, State: "pending", CreatedAt: now})
	p.Version++
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) DecideExpense(repo, oid, pid, eid, actor string, in ExpenseDecisionInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return p, e
	}
	f, e := s.read(repo, o.FundID)
	if e != nil {
		return p, e
	}
	if !contains(f.Terms.StewardIDs, actor) {
		return p, ErrForbidden
	}
	if p.Execution == nil {
		return p, ErrConflict
	}
	deriveExecution(p.Execution)
	if in.Approve && spendingBlocked(p.Execution) {
		return p, ErrConflict
	}
	found := false
	now := s.now().UTC()
	var amount int64
	for i := range p.Execution.Expenses {
		v := &p.Execution.Expenses[i]
		if v.ID == eid {
			found = true
			if v.State != "pending" {
				return p, ErrConflict
			}
			if in.Approve {
				if p.Execution.ApprovedExpenses+v.Amount > p.Execution.Budget {
					return p, ErrConflict
				}
				v.State = "approved"
				amount = v.Amount
			} else {
				v.State = "rejected"
			}
			v.ApprovedByID = actor
			v.DecidedAt = &now
		}
	}
	if !found {
		return p, ErrNotFound
	}
	if amount > 0 {
		for i := range f.Reservations {
			if f.Reservations[i].ID == p.Selection.ReservationID {
				f.Reservations[i].Spent += amount
			}
		}
		f.Version++
		derive(&f)
		if e = s.write(f); e != nil {
			return p, e
		}
	}
	deriveExecution(p.Execution)
	p.Version++
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) ControlExecution(repo, oid, pid, actor string, in ExecutionControlInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return p, e
	}
	f, e := s.read(repo, o.FundID)
	if e != nil {
		return p, e
	}
	if !contains(f.Terms.StewardIDs, actor) {
		return p, ErrForbidden
	}
	if p.Execution == nil || strings.TrimSpace(in.Reason) == "" {
		return p, ErrInvalid
	}
	now := s.now().UTC()
	c := ExecutionChange{ID: id(), ActorID: actor, Kind: in.Action, Reason: strings.TrimSpace(in.Reason), CreatedAt: now}
	switch in.Action {
	case "pause":
		p.Execution.State = "paused"
	case "resume":
		deriveExecution(p.Execution)
		if len(p.Execution.Blockers) > 0 {
			return p, ErrConflict
		}
		p.Execution.State = "active"
	case "replace":
		if strings.TrimSpace(in.RecipientID) == "" || in.RecipientID == p.Execution.ActiveRecipientID {
			return p, ErrInvalid
		}
		c.FromRecipientID = p.Execution.ActiveRecipientID
		c.ToRecipientID = in.RecipientID
		p.Execution.ActiveRecipientID = in.RecipientID
		p.Execution.State = "active"
	case "budget":
		terms := o.Versions[len(o.Versions)-1].Terms
		var otherRecipientReserved int64
		for _, reservation := range f.Reservations {
			if reservation.ID != p.Selection.ReservationID && reservation.State == "active" && reservation.Recipient == p.Execution.ActiveRecipientID {
				otherRecipientReserved += reservation.Amount
			}
		}
		increase := in.Budget - p.Execution.Budget
		derive(&f)
		if in.Budget < p.Execution.ApprovedExpenses || in.Budget <= 0 || in.Budget > terms.Budget || in.Budget > f.Terms.Limits.PerAllocation || otherRecipientReserved+in.Budget > f.Terms.Limits.PerRecipient || increase > f.Balances.Available {
			return p, ErrInvalid
		}
		c.PreviousBudget = p.Execution.Budget
		c.Budget = in.Budget
		p.Execution.Budget = in.Budget
		for i := range f.Reservations {
			if f.Reservations[i].ID == p.Selection.ReservationID {
				f.Reservations[i].Amount = in.Budget
			}
		}
		f.Version++
		derive(&f)
		if e = s.write(f); e != nil {
			return p, e
		}
	case "cancel":
		p.Execution.State = "cancelled"
		for i := range f.Reservations {
			if f.Reservations[i].ID == p.Selection.ReservationID {
				f.Reservations[i].State = "cancelled"
			}
		}
		f.Version++
		derive(&f)
		if e = s.write(f); e != nil {
			return p, e
		}
	default:
		return p, ErrInvalid
	}
	p.Execution.Changes = append(p.Execution.Changes, c)
	deriveExecution(p.Execution)
	p.Version++
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
