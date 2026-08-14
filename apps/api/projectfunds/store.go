// Package projectfunds owns governed, repository-scoped resource funds.
package projectfunds

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("project fund not found")
	ErrInvalid   = errors.New("invalid project fund")
	ErrConflict  = errors.New("project fund conflict")
	ErrForbidden = errors.New("project fund action forbidden")
)

type ApprovalRule struct {
	MinimumApprovals int      `json:"minimum_approvals"`
	ApproverIDs      []string `json:"approver_ids"`
	Threshold        int64    `json:"threshold,omitempty"`
}
type SpendingLimits struct {
	PerAllocation int64 `json:"per_allocation"`
	PerRecipient  int64 `json:"per_recipient"`
	Total         int64 `json:"total"`
}
type Terms struct {
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	StewardIDs         []string       `json:"steward_ids"`
	FundingSources     []string       `json:"accepted_funding_sources"`
	Unit               string         `json:"unit"`
	UnitKind           string         `json:"unit_kind"`
	Limits             SpendingLimits `json:"spending_limits"`
	Approval           ApprovalRule   `json:"approval_rule"`
	EligibleRecipients []string       `json:"eligible_recipients"`
	RefundPolicy       string         `json:"refund_policy"`
	LedgerVisibility   string         `json:"ledger_visibility"`
}
type Balances struct {
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Spent     int64 `json:"spent"`
	Refunded  int64 `json:"refunded"`
	Disputed  int64 `json:"disputed"`
}
type Reservation struct {
	ID         string    `json:"id"`
	OutcomeID  string    `json:"outcome_id"`
	ProposalID string    `json:"delivery_proposal_id"`
	Recipient  string    `json:"recipient_id"`
	Amount     int64     `json:"amount"`
	State      string    `json:"state"`
	CreatedBy  string    `json:"created_by_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Transfer struct {
	ID            string    `json:"id"`
	Reference     string    `json:"reference"`
	Source        string    `json:"source"`
	ContributorID string    `json:"contributor_id"`
	Requested     int64     `json:"requested"`
	Settled       int64     `json:"settled"`
	State         string    `json:"state"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
type Fund struct {
	ID                   string        `json:"id"`
	RepositoryID         string        `json:"repository_id"`
	Version              int64         `json:"version"`
	Terms                Terms         `json:"terms"`
	Balances             Balances      `json:"balances"`
	Transfers            []Transfer    `json:"transfers"`
	Reservations         []Reservation `json:"reservations"`
	CreatedByID          string        `json:"created_by_id"`
	CreatedAt            time.Time     `json:"created_at"`
	OperationalAuthority []string      `json:"operational_authority"`
}
type TransferInput struct {
	Reference string `json:"reference"`
	Source    string `json:"source"`
	Amount    int64  `json:"amount"`
	Settled   int64  `json:"settled"`
	State     string `json:"state"`
	Note      string `json:"note"`
}
type ReconcileInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Settled         int64  `json:"settled"`
	State           string `json:"state"`
	Note            string `json:"note"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func clean(xs []string) bool {
	if len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 200 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validTerms(t Terms) bool {
	return strings.TrimSpace(t.Name) != "" && len(t.Name) <= 120 && clean(t.StewardIDs) && clean(t.FundingSources) && strings.TrimSpace(t.Unit) != "" && (t.UnitKind == "currency" || t.UnitKind == "credit") && t.Limits.PerAllocation > 0 && t.Limits.PerRecipient > 0 && t.Limits.Total > 0 && t.Limits.PerAllocation <= t.Limits.PerRecipient && t.Limits.PerRecipient <= t.Limits.Total && t.Approval.MinimumApprovals > 0 && t.Approval.MinimumApprovals <= len(t.Approval.ApproverIDs) && clean(t.Approval.ApproverIDs) && clean(t.EligibleRecipients) && strings.TrimSpace(t.RefundPolicy) != "" && (t.LedgerVisibility == "public" || t.LedgerVisibility == "repository")
}
func (s *Store) Create(repo, actor string, t Terms) (Fund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || !validTerms(t) {
		return Fund{}, ErrInvalid
	}
	now := s.now().UTC()
	f := Fund{ID: id(), RepositoryID: repo, Version: 1, Terms: t, Transfers: []Transfer{}, Reservations: []Reservation{}, CreatedByID: actor, CreatedAt: now, OperationalAuthority: []string{}}
	return f, s.write(f)
}
func (s *Store) Commit(repo, fid, actor string, in TransferInput) (Fund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, e := s.read(repo, fid)
	if e != nil {
		return f, e
	}
	if actor == "" || strings.TrimSpace(in.Reference) == "" || in.Amount <= 0 || in.Amount > f.Terms.Limits.Total || !contains(f.Terms.FundingSources, in.Source) || !validState(in.State) || in.Settled < 0 || in.Settled > in.Amount {
		return f, ErrInvalid
	}
	for _, x := range f.Transfers {
		if x.Source == in.Source && x.Reference == in.Reference {
			return f, ErrConflict
		}
	}
	if in.State != "settled" && in.State != "partial" && in.Settled != 0 {
		return f, ErrInvalid
	}
	if in.State == "settled" && in.Settled != in.Amount {
		return f, ErrInvalid
	}
	if in.State == "partial" && (in.Settled == 0 || in.Settled == in.Amount) {
		return f, ErrInvalid
	}
	now := s.now().UTC()
	f.Transfers = append(f.Transfers, Transfer{ID: id(), Reference: strings.TrimSpace(in.Reference), Source: in.Source, ContributorID: actor, Requested: in.Amount, Settled: in.Settled, State: in.State, Note: in.Note, CreatedAt: now, UpdatedAt: now})
	f.Version++
	derive(&f)
	return f, s.write(f)
}
func (s *Store) Reconcile(repo, fid, tid, actor string, in ReconcileInput) (Fund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, e := s.read(repo, fid)
	if e != nil {
		return f, e
	}
	if in.ExpectedVersion != f.Version {
		return f, ErrConflict
	}
	if !contains(f.Terms.StewardIDs, actor) {
		return f, ErrForbidden
	}
	if !validState(in.State) {
		return f, ErrInvalid
	}
	found := false
	for i := range f.Transfers {
		x := &f.Transfers[i]
		if x.ID != tid {
			continue
		}
		found = true
		if terminal(x.State) {
			return f, ErrConflict
		}
		if in.Settled < 0 || in.Settled > x.Requested || (in.State == "settled" && in.Settled != x.Requested) || (in.State == "partial" && (in.Settled == 0 || in.Settled == x.Requested)) || ((in.State != "settled" && in.State != "partial") && in.Settled != 0) {
			return f, ErrInvalid
		}
		x.Settled = in.Settled
		x.State = in.State
		x.Note = in.Note
		x.UpdatedAt = s.now().UTC()
	}
	if !found {
		return f, ErrNotFound
	}
	f.Version++
	derive(&f)
	return f, s.write(f)
}
func validState(x string) bool {
	return map[string]bool{"pending": true, "settled": true, "partial": true, "failed": true, "revoked": true, "refunded": true, "disputed": true}[x]
}
func terminal(x string) bool { return x == "failed" || x == "revoked" || x == "refunded" }
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func derive(f *Fund) {
	f.Balances = Balances{}
	for _, x := range f.Transfers {
		switch x.State {
		case "settled", "partial":
			f.Balances.Available += x.Settled
		case "refunded":
			f.Balances.Refunded += x.Requested
		case "disputed":
			f.Balances.Disputed += x.Requested
		}
	}
	for _, r := range f.Reservations {
		if r.State == "active" {
			f.Balances.Reserved += r.Amount
			f.Balances.Available -= r.Amount
		}
	}
}
func (s *Store) Get(repo, id string) (Fund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Fund, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Fund{}
	for _, p := range paths {
		b, x := os.ReadFile(p)
		var f Fund
		if x == nil {
			x = json.Unmarshal(b, &f)
		}
		if x != nil {
			return nil, x
		}
		derive(&f)
		out = append(out, f)
	}
	return out, nil
}
func (s *Store) read(repo, id string) (Fund, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Fund{}, ErrNotFound
	}
	var f Fund
	if e == nil {
		e = json.Unmarshal(b, &f)
	}
	derive(&f)
	return f, e
}
func (s *Store) write(f Fund) error {
	d := filepath.Join(s.root, f.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(f, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "fund-*.tmp")
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
		e = os.Rename(n, filepath.Join(d, f.ID+".json"))
	}
	return e
}
