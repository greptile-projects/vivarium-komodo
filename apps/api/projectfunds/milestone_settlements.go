package projectfunds

import (
	"strings"
	"time"
)

// MilestoneSettlement is the append-only financial recognition of reviewed
// project evidence. It observes ordinary delivery resources but grants no
// authority over any of them.
type MilestoneSettlement struct {
	MilestoneID string                     `json:"milestone_id"`
	RecipientID string                     `json:"recipient_id"`
	Allocation  int64                      `json:"original_allocation"`
	State       string                     `json:"state"`
	Awarded     int64                      `json:"awarded"`
	Released    int64                      `json:"released"`
	Payment     string                     `json:"payment_state"`
	Events      []MilestoneSettlementEvent `json:"events"`
}

type MilestoneSettlementEvent struct {
	ID        string               `json:"id"`
	Kind      string               `json:"kind"`
	ActorID   string               `json:"actor_id"`
	Rationale string               `json:"rationale"`
	Dissent   []string             `json:"dissent"`
	Award     int64                `json:"award,omitempty"`
	Evidence  []ExecutionReference `json:"evidence"`
	CreatedAt time.Time            `json:"created_at"`
}

type MilestoneReviewInput struct {
	ExpectedVersion int64                `json:"expected_version"`
	Decision        string               `json:"decision"`
	Award           int64                `json:"award,omitempty"`
	Rationale       string               `json:"rationale"`
	Dissent         []string             `json:"dissent"`
	Evidence        []ExecutionReference `json:"evidence"`
}

type MilestoneRecoveryInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Action          string `json:"action"`
	Rationale       string `json:"rationale"`
}

func proposalMilestone(p DeliveryProposal, mid string) (ProposalMilestone, bool) {
	for _, m := range p.Terms.Milestones {
		if m.ID == mid {
			return m, true
		}
	}
	return ProposalMilestone{}, false
}

func outcomeMilestone(o FundedOutcome, mid string) (Milestone, bool) {
	for _, m := range o.Versions[len(o.Versions)-1].Terms.Milestones {
		if m.ID == mid {
			return m, true
		}
	}
	return Milestone{}, false
}

func settlement(x *DeliveryExecution, mid, recipient string, allocation int64) *MilestoneSettlement {
	for i := range x.Settlements {
		if x.Settlements[i].MilestoneID == mid {
			return &x.Settlements[i]
		}
	}
	x.Settlements = append(x.Settlements, MilestoneSettlement{MilestoneID: mid, RecipientID: recipient, Allocation: allocation, State: "pending_review", Payment: "unreleased", Events: []MilestoneSettlementEvent{}})
	return &x.Settlements[len(x.Settlements)-1]
}

func approvedForMilestone(x *DeliveryExecution, mid string) int64 {
	var n int64
	for _, e := range x.Expenses {
		if e.MilestoneID == mid && e.State == "approved" {
			n += e.Amount
		}
	}
	return n
}

func reviewerIDs(m Milestone, f Fund) []string {
	if len(m.ReviewerIDs) > 0 {
		return m.ReviewerIDs
	}
	return f.Terms.Approval.ApproverIDs
}

func adjustReservation(f *Fund, reservationID string, spentDelta, amountDelta int64) error {
	for i := range f.Reservations {
		r := &f.Reservations[i]
		if r.ID != reservationID {
			continue
		}
		if r.Spent+spentDelta < 0 || r.Amount+amountDelta < r.Spent+spentDelta {
			return ErrConflict
		}
		derive(f)
		if amountDelta > 0 && amountDelta > f.Balances.Available {
			return ErrConflict
		}
		r.Spent += spentDelta
		r.Amount += amountDelta
		return nil
	}
	return ErrNotFound
}

func (s *Store) ReviewMilestone(repo, oid, pid, mid, actor string, in MilestoneReviewInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readDeliveryProposal(repo, oid, pid)
	if err != nil {
		return p, err
	}
	if p.Version != in.ExpectedVersion {
		return p, ErrConflict
	}
	if p.State != "selected" || p.Execution == nil || p.Selection == nil {
		return p, ErrConflict
	}
	o, err := s.readOutcome(repo, oid)
	if err != nil {
		return p, err
	}
	f, err := s.read(repo, o.FundID)
	if err != nil {
		return p, err
	}
	pm, ok := proposalMilestone(p, mid)
	if !ok {
		return p, ErrNotFound
	}
	om, ok := outcomeMilestone(o, mid)
	if !ok {
		return p, ErrConflict
	}
	if !contains(reviewerIDs(om, f), actor) {
		return p, ErrForbidden
	}
	if strings.TrimSpace(in.Rationale) == "" || !cleanOptional(in.Dissent) || !validEvidence(in.Evidence) {
		return p, ErrInvalid
	}
	if in.Decision != "accepted" && in.Decision != "partial_award" && in.Decision != "correction_requested" && in.Decision != "rejected" && in.Decision != "disputed" {
		return p, ErrInvalid
	}
	x := settlement(p.Execution, mid, p.Terms.RecipientID, pm.Cost)
	if x.State == "accepted" || x.State == "refunded" || x.State == "recipient_withdrawn" || x.State == "timed_out" {
		return p, ErrConflict
	}
	remaining := pm.Cost - approvedForMilestone(p.Execution, mid) - x.Awarded
	award := int64(0)
	if in.Decision == "accepted" {
		award = remaining
	}
	if in.Decision == "partial_award" {
		award = in.Award
	}
	if award < 0 || award > remaining || (in.Decision == "partial_award" && award <= 0) {
		return p, ErrInvalid
	}
	if in.Decision != "accepted" && in.Decision != "partial_award" && in.Award != 0 {
		return p, ErrInvalid
	}
	amountDelta := int64(0)
	if x.Released > 0 && award > 0 {
		amountDelta = award
		x.Released -= award
	}
	if in.Decision == "rejected" {
		x.Released += remaining
		amountDelta = -remaining
	}
	if award > 0 {
		if err = adjustReservation(&f, p.Selection.ReservationID, award, amountDelta); err != nil {
			return p, err
		}
		x.Awarded += award
		x.Payment = "settled"
	}
	x.State = in.Decision
	if in.Decision == "partial_award" && x.Awarded == pm.Cost-approvedForMilestone(p.Execution, mid) {
		x.State = "accepted"
	}
	now := s.now().UTC()
	x.Events = append(x.Events, MilestoneSettlementEvent{ID: id(), Kind: in.Decision, ActorID: actor, Rationale: strings.TrimSpace(in.Rationale), Dissent: in.Dissent, Award: award, Evidence: in.Evidence, CreatedAt: now})
	f.Version++
	derive(&f)
	if err = s.write(f); err != nil {
		return p, err
	}
	p.Version++
	p.UpdatedAt = now
	deriveExecution(p.Execution)
	return p, s.writeDeliveryProposal(p)
}

func (s *Store) RecoverMilestone(repo, oid, pid, mid, actor string, in MilestoneRecoveryInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readDeliveryProposal(repo, oid, pid)
	if err != nil {
		return p, err
	}
	if p.Version != in.ExpectedVersion {
		return p, ErrConflict
	}
	if p.Execution == nil || p.Selection == nil || strings.TrimSpace(in.Rationale) == "" {
		return p, ErrInvalid
	}
	o, err := s.readOutcome(repo, oid)
	if err != nil {
		return p, err
	}
	f, err := s.read(repo, o.FundID)
	if err != nil {
		return p, err
	}
	pm, ok := proposalMilestone(p, mid)
	if !ok {
		return p, ErrNotFound
	}
	om, ok := outcomeMilestone(o, mid)
	if !ok {
		return p, ErrConflict
	}
	x := settlement(p.Execution, mid, p.Terms.RecipientID, pm.Cost)
	reviewer := contains(reviewerIDs(om, f), actor)
	steward := contains(f.Terms.StewardIDs, actor)
	remaining := pm.Cost - approvedForMilestone(p.Execution, mid) - x.Awarded - x.Released
	spentDelta, amountDelta := int64(0), int64(0)
	switch in.Action {
	case "recipient_withdrawal":
		if actor != p.Terms.RecipientID || x.Awarded > 0 {
			return p, ErrForbidden
		}
		x.State = "recipient_withdrawn"
		x.Released += remaining
		amountDelta = -remaining
	case "timeout":
		if !reviewer || s.now().UTC().Before(o.Versions[len(o.Versions)-1].Terms.Deadline) || x.Awarded > 0 {
			return p, ErrForbidden
		}
		x.State = "timed_out"
		x.Released += remaining
		amountDelta = -remaining
	case "appeal":
		if actor != p.Terms.RecipientID || (x.State != "rejected" && x.State != "disputed" && x.State != "timed_out") {
			return p, ErrForbidden
		}
		x.State = "appealed"
	case "payment_failed":
		if !steward || x.Awarded == 0 || x.Payment != "settled" {
			return p, ErrForbidden
		}
		spentDelta = -x.Awarded
		x.Payment = "failed"
		x.State = "payment_failed"
	case "retry_payment":
		if !steward || x.Payment != "failed" {
			return p, ErrForbidden
		}
		spentDelta = x.Awarded
		x.Payment = "settled"
		x.State = "accepted"
	case "refund":
		if !steward || x.Awarded == 0 || x.Payment != "settled" {
			return p, ErrForbidden
		}
		spentDelta = -x.Awarded
		amountDelta = -x.Awarded
		x.Released += x.Awarded
		x.Payment = "refunded"
		x.State = "refunded"
	default:
		return p, ErrInvalid
	}
	if err = adjustReservation(&f, p.Selection.ReservationID, spentDelta, amountDelta); err != nil {
		return p, err
	}
	now := s.now().UTC()
	x.Events = append(x.Events, MilestoneSettlementEvent{ID: id(), Kind: in.Action, ActorID: actor, Rationale: strings.TrimSpace(in.Rationale), Evidence: []ExecutionReference{}, Dissent: []string{}, CreatedAt: now})
	f.Version++
	derive(&f)
	if err = s.write(f); err != nil {
		return p, err
	}
	p.Version++
	p.UpdatedAt = now
	deriveExecution(p.Execution)
	return p, s.writeDeliveryProposal(p)
}
