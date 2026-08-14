package projectfunds

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type WorkReference struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description"`
}
type ProposalMilestone struct {
	ID           string   `json:"id"`
	Approach     string   `json:"approach"`
	Cost         int64    `json:"cost"`
	Deliverables []string `json:"deliverables"`
}
type DeliveryProposalTerms struct {
	RecipientKind  string              `json:"recipient_kind"`
	RecipientID    string              `json:"recipient_id"`
	Approach       string              `json:"approach"`
	Milestones     []ProposalMilestone `json:"milestones"`
	Cost           int64               `json:"cost"`
	Dependencies   []string            `json:"dependencies"`
	Availability   string              `json:"availability"`
	RequiredAccess []string            `json:"required_access"`
	RelevantWork   []WorkReference     `json:"relevant_attributed_work"`
}
type ProposalAcceptance struct {
	ActorID    string    `json:"actor_id"`
	AcceptedAt time.Time `json:"accepted_at"`
}
type ProposalConflict struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
type ProposalApproval struct {
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type DeliveryConnection struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	URL  string `json:"url,omitempty"`
}
type ProposalSelection struct {
	StewardID     string               `json:"steward_id"`
	Reason        string               `json:"reason"`
	ReservationID string               `json:"reservation_id"`
	Connections   []DeliveryConnection `json:"connections"`
	SelectedAt    time.Time            `json:"selected_at"`
}
type DeliveryProposal struct {
	ID                   string                `json:"id"`
	OutcomeID            string                `json:"outcome_id"`
	RepositoryID         string                `json:"repository_id"`
	Version              int64                 `json:"version"`
	Terms                DeliveryProposalTerms `json:"terms"`
	SubmittedByID        string                `json:"submitted_by_id"`
	State                string                `json:"state"`
	Acceptance           *ProposalAcceptance   `json:"acceptance,omitempty"`
	Conflicts            []ProposalConflict    `json:"conflicts"`
	Approvals            []ProposalApproval    `json:"approvals"`
	Selection            *ProposalSelection    `json:"selection,omitempty"`
	Execution            *DeliveryExecution    `json:"execution,omitempty"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	OperationalAuthority []string              `json:"operational_authority"`
}
type SubmitDeliveryProposalInput struct {
	Terms DeliveryProposalTerms `json:"terms"`
}
type AcceptDeliveryProposalInput struct {
	ExpectedVersion int64 `json:"expected_version"`
}
type DiscloseConflictInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Detail          string `json:"detail"`
}
type ApproveDeliveryProposalInput struct {
	ExpectedVersion int64 `json:"expected_version"`
}
type SelectDeliveryProposalInput struct {
	ExpectedVersion int64                `json:"expected_version"`
	Reason          string               `json:"reason"`
	Connections     []DeliveryConnection `json:"connections"`
}

func validDeliveryTerms(t DeliveryProposalTerms, o FundedOutcome, f Fund) bool {
	if !contains(f.Terms.EligibleRecipients, t.RecipientKind) || strings.TrimSpace(t.RecipientID) == "" || strings.TrimSpace(t.Approach) == "" || len(t.Approach) > 10000 || t.Cost <= 0 || t.Cost > f.Terms.Limits.PerAllocation || strings.TrimSpace(t.Availability) == "" || !cleanOptional(t.Dependencies) || !cleanOptional(t.RequiredAccess) {
		return false
	}
	if len(t.Milestones) == 0 {
		return false
	}
	seen := map[string]bool{}
	var total int64
	for _, m := range t.Milestones {
		if strings.TrimSpace(m.ID) == "" || seen[m.ID] || strings.TrimSpace(m.Approach) == "" || m.Cost <= 0 || !clean(m.Deliverables) {
			return false
		}
		seen[m.ID] = true
		total += m.Cost
	}
	if total != t.Cost {
		return false
	}
	for _, w := range t.RelevantWork {
		if strings.TrimSpace(w.Kind) == "" || strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.Description) == "" {
			return false
		}
	}
	return t.Cost <= o.Versions[len(o.Versions)-1].Terms.Budget
}
func (s *Store) SubmitDeliveryProposal(repo, oid, actor string, in SubmitDeliveryProposalInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return DeliveryProposal{}, e
	}
	f, e := s.read(repo, o.FundID)
	if e != nil {
		return DeliveryProposal{}, e
	}
	if actor == "" || !validDeliveryTerms(in.Terms, o, f) {
		return DeliveryProposal{}, ErrInvalid
	}
	now := s.now().UTC()
	p := DeliveryProposal{ID: id(), OutcomeID: oid, RepositoryID: repo, Version: 1, Terms: in.Terms, SubmittedByID: actor, State: "proposed", Conflicts: []ProposalConflict{}, Approvals: []ProposalApproval{}, CreatedAt: now, UpdatedAt: now, OperationalAuthority: []string{}}
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) AcceptDeliveryProposal(repo, oid, pid, actor string, in AcceptDeliveryProposalInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	if actor != p.Terms.RecipientID {
		return p, ErrForbidden
	}
	if p.State != "proposed" {
		return p, ErrConflict
	}
	now := s.now().UTC()
	p.Version++
	p.State = "accepted"
	p.Acceptance = &ProposalAcceptance{ActorID: actor, AcceptedAt: now}
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) DiscloseProposalConflict(repo, oid, pid, actor string, in DiscloseConflictInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	if strings.TrimSpace(in.Detail) == "" {
		return p, ErrInvalid
	}
	now := s.now().UTC()
	p.Version++
	p.Conflicts = append(p.Conflicts, ProposalConflict{ID: id(), ActorID: actor, Detail: strings.TrimSpace(in.Detail), CreatedAt: now})
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) ApproveDeliveryProposal(repo, oid, pid, actor string, in ApproveDeliveryProposalInput) (DeliveryProposal, error) {
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
	if p.State != "accepted" {
		return p, ErrConflict
	}
	if !contains(f.Terms.Approval.ApproverIDs, actor) {
		return p, ErrForbidden
	}
	for _, approval := range p.Approvals {
		if approval.ActorID == actor {
			return p, ErrConflict
		}
	}
	now := s.now().UTC()
	p.Version++
	p.Approvals = append(p.Approvals, ProposalApproval{ActorID: actor, CreatedAt: now})
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) SelectDeliveryProposal(repo, oid, pid, actor string, in SelectDeliveryProposalInput) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readDeliveryProposal(repo, oid, pid)
	if e != nil {
		return p, e
	}
	if in.ExpectedVersion != p.Version {
		return p, ErrConflict
	}
	if p.State != "accepted" {
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
	if strings.TrimSpace(in.Reason) == "" {
		return p, ErrInvalid
	}
	if len(p.Approvals) < f.Terms.Approval.MinimumApprovals {
		return p, ErrConflict
	}
	var recipientReserved int64
	for _, r := range f.Reservations {
		if r.State == "active" {
			if r.ProposalID == pid {
				return p, ErrConflict
			}
			if r.Recipient == p.Terms.RecipientID {
				recipientReserved += r.Amount
			}
		}
	}
	derive(&f)
	if p.Terms.Cost > f.Balances.Available || recipientReserved+p.Terms.Cost > f.Terms.Limits.PerRecipient {
		return p, ErrConflict
	}
	for _, c := range in.Connections {
		if (c.Kind != "proposal_task" && c.Kind != "delivery_team") || strings.TrimSpace(c.ID) == "" {
			return p, ErrInvalid
		}
	}
	now := s.now().UTC()
	reservation := Reservation{ID: id(), OutcomeID: oid, ProposalID: pid, Recipient: p.Terms.RecipientID, Amount: p.Terms.Cost, State: "active", CreatedBy: actor, CreatedAt: now}
	f.Reservations = append(f.Reservations, reservation)
	f.Version++
	derive(&f)
	if e = s.write(f); e != nil {
		return p, e
	}
	p.Version++
	p.State = "selected"
	p.Selection = &ProposalSelection{StewardID: actor, Reason: strings.TrimSpace(in.Reason), ReservationID: reservation.ID, Connections: in.Connections, SelectedAt: now}
	p.Execution = &DeliveryExecution{State: "active", ActiveRecipientID: p.Terms.RecipientID, Budget: p.Terms.Cost, MilestoneCount: len(p.Terms.Milestones), Progress: []ProgressObservation{}, Expenses: []Expense{}, Changes: []ExecutionChange{}}
	deriveExecution(p.Execution)
	p.UpdatedAt = now
	return p, s.writeDeliveryProposal(p)
}
func (s *Store) GetDeliveryProposal(repo, oid, pid string) (DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readDeliveryProposal(repo, oid, pid)
}
func (s *Store) ListDeliveryProposals(repo, oid string) ([]DeliveryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, e := s.readOutcome(repo, oid); e != nil {
		return nil, e
	}
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "outcomes", oid, "delivery-proposals", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []DeliveryProposal{}
	for _, path := range paths {
		b, x := os.ReadFile(path)
		var p DeliveryProposal
		if x == nil {
			x = json.Unmarshal(b, &p)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) readDeliveryProposal(repo, oid, pid string) (DeliveryProposal, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, "outcomes", oid, "delivery-proposals", pid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return DeliveryProposal{}, ErrNotFound
	}
	var p DeliveryProposal
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) writeDeliveryProposal(p DeliveryProposal) error {
	d := filepath.Join(s.root, p.RepositoryID, "outcomes", p.OutcomeID, "delivery-proposals")
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "proposal-*.tmp")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if x := tmp.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, p.ID+".json"))
	}
	return e
}
