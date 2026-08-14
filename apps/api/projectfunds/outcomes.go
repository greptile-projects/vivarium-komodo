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

type OutcomeOrigin struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
}

type Milestone struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Budget               int64    `json:"budget"`
	AcceptanceCriteria   []string `json:"acceptance_criteria"`
	EvidenceRequirements []string `json:"evidence_requirements"`
}

type OutcomeTerms struct {
	Origin                 OutcomeOrigin `json:"origin"`
	Title                  string        `json:"title"`
	Scope                  string        `json:"scope"`
	AcceptanceCriteria     []string      `json:"acceptance_criteria"`
	EvidenceRequirements   []string      `json:"evidence_requirements"`
	Budget                 int64         `json:"budget"`
	Deadline               time.Time     `json:"deadline"`
	ContributorEligibility []string      `json:"contributor_eligibility"`
	AllocationMethod       string        `json:"allocation_method"`
	CancellationTerms      string        `json:"cancellation_terms"`
	Milestones             []Milestone   `json:"milestones"`
	Dependencies           []string      `json:"dependencies"`
	Risks                  []string      `json:"risks"`
	DeclaredConflicts      []string      `json:"declared_conflicts"`
	OverlapKeys            []string      `json:"overlap_keys"`
	Embargoed              bool          `json:"embargoed"`
}

type OutcomeVersion struct {
	Version      int64        `json:"version"`
	Terms        OutcomeTerms `json:"terms"`
	AuthorID     string       `json:"author_id"`
	ChangeReason string       `json:"change_reason"`
	CreatedAt    time.Time    `json:"created_at"`
}

type Pledge struct {
	ID        string    `json:"id"`
	BackerID  string    `json:"backer_id"`
	Target    string    `json:"target"`
	Amount    int64     `json:"amount"`
	State     string    `json:"state"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReplanEvent struct {
	Kind        string    `json:"kind"`
	ActorID     string    `json:"actor_id"`
	Reason      string    `json:"reason"`
	FromVersion int64     `json:"from_version"`
	ToVersion   int64     `json:"to_version"`
	CreatedAt   time.Time `json:"created_at"`
}

type OutcomeBlocker struct {
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	ResourceID string `json:"resource_id,omitempty"`
}

type FundedOutcome struct {
	ID                   string           `json:"id"`
	RepositoryID         string           `json:"repository_id"`
	FundID               string           `json:"fund_id"`
	Version              int64            `json:"version"`
	Versions             []OutcomeVersion `json:"versions"`
	Pledges              []Pledge         `json:"pledges"`
	Replanning           []ReplanEvent    `json:"replanning"`
	Pledged              int64            `json:"pledged"`
	Blockers             []OutcomeBlocker `json:"blockers"`
	Delivery             []DeliveryReport `json:"delivery"`
	CreatedByID          string           `json:"created_by_id"`
	CreatedAt            time.Time        `json:"created_at"`
	OperationalAuthority []string         `json:"operational_authority"`
}

type DeliveryReport struct {
	ProposalID  string            `json:"delivery_proposal_id"`
	RecipientID string            `json:"recipient_id"`
	Execution   DeliveryExecution `json:"execution"`
}

type CreateOutcomeInput struct {
	FundID string       `json:"fund_id"`
	Terms  OutcomeTerms `json:"terms"`
}
type PledgeInput struct {
	Target string `json:"target"`
	Amount int64  `json:"amount"`
}
type WithdrawInput struct {
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason"`
}
type ReplanInput struct {
	ExpectedVersion int64        `json:"expected_version"`
	Terms           OutcomeTerms `json:"terms"`
	Reason          string       `json:"reason"`
}

var originKinds = map[string]bool{"issue": true, "roadmap_outcome": true, "proposal": true, "stewardship_opportunity": true, "incident_follow_up": true, "security_repair": true}

func validOutcomeTerms(t OutcomeTerms, f Fund) bool {
	if !originKinds[t.Origin.Kind] || strings.TrimSpace(t.Origin.ResourceID) == "" || len(t.Origin.ResourceID) > 200 || strings.TrimSpace(t.Title) == "" || len(t.Title) > 160 || strings.TrimSpace(t.Scope) == "" || len(t.Scope) > 10000 || t.Budget <= 0 || t.Budget > f.Terms.Limits.Total || t.Deadline.IsZero() || !t.Deadline.After(time.Now().UTC()) || !clean(t.AcceptanceCriteria) || !clean(t.EvidenceRequirements) || !clean(t.ContributorEligibility) || !cleanOptional(t.Dependencies) || !cleanOptional(t.Risks) || !cleanOptional(t.DeclaredConflicts) || !cleanOptional(t.OverlapKeys) || strings.TrimSpace(t.AllocationMethod) == "" || len(t.AllocationMethod) > 2000 || strings.TrimSpace(t.CancellationTerms) == "" || len(t.CancellationTerms) > 5000 {
		return false
	}
	for _, e := range t.ContributorEligibility {
		if !contains(f.Terms.EligibleRecipients, e) {
			return false
		}
	}
	seen, total := map[string]bool{}, int64(0)
	for _, m := range t.Milestones {
		if strings.TrimSpace(m.ID) == "" || seen[m.ID] || strings.TrimSpace(m.Name) == "" || m.Budget <= 0 || !clean(m.AcceptanceCriteria) || !clean(m.EvidenceRequirements) {
			return false
		}
		seen[m.ID] = true
		total += m.Budget
	}
	return len(t.Milestones) == 0 || total == t.Budget
}

func cleanOptional(xs []string) bool {
	if len(xs) == 0 {
		return true
	}
	return clean(xs)
}

func (s *Store) CreateOutcome(repo, actor string, in CreateOutcomeInput) (FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, e := s.read(repo, in.FundID)
	if e != nil {
		return FundedOutcome{}, e
	}
	if actor == "" || !contains(f.Terms.StewardIDs, actor) || !validOutcomeTerms(in.Terms, f) {
		if actor != "" && !contains(f.Terms.StewardIDs, actor) {
			return FundedOutcome{}, ErrForbidden
		}
		return FundedOutcome{}, ErrInvalid
	}
	now := s.now().UTC()
	o := FundedOutcome{ID: id(), RepositoryID: repo, FundID: f.ID, Version: 1, Pledges: []Pledge{}, Replanning: []ReplanEvent{}, CreatedByID: actor, CreatedAt: now, OperationalAuthority: []string{}}
	o.Versions = []OutcomeVersion{{Version: 1, Terms: in.Terms, AuthorID: actor, ChangeReason: "initial terms", CreatedAt: now}}
	s.deriveOutcome(&o)
	return o, s.writeOutcome(o)
}

func (s *Store) PledgeOutcome(repo, oid, actor string, in PledgeInput) (FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return o, e
	}
	f, e := s.read(repo, o.FundID)
	if e != nil {
		return o, e
	}
	if actor == "" || in.Amount <= 0 || in.Amount > f.Terms.Limits.Total || !validTarget(in.Target, o.Versions[len(o.Versions)-1].Terms) {
		return o, ErrInvalid
	}
	now := s.now().UTC()
	o.Pledges = append(o.Pledges, Pledge{ID: id(), BackerID: actor, Target: in.Target, Amount: in.Amount, State: "active", CreatedAt: now, UpdatedAt: now})
	o.Version++
	s.deriveOutcome(&o)
	return o, s.writeOutcome(o)
}

func validTarget(target string, t OutcomeTerms) bool {
	if target == "outcome" {
		return true
	}
	for _, m := range t.Milestones {
		if target == "milestone:"+m.ID {
			return true
		}
	}
	return false
}

func (s *Store) WithdrawPledge(repo, oid, pid, actor string, in WithdrawInput) (FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return o, e
	}
	if in.ExpectedVersion != o.Version {
		return o, ErrConflict
	}
	if strings.TrimSpace(in.Reason) == "" {
		return o, ErrInvalid
	}
	found := false
	for i := range o.Pledges {
		p := &o.Pledges[i]
		if p.ID == pid {
			found = true
			if p.BackerID != actor {
				return o, ErrForbidden
			}
			if p.State != "active" {
				return o, ErrConflict
			}
			p.State = "withdrawn"
			p.Reason = in.Reason
			p.UpdatedAt = s.now().UTC()
		}
	}
	if !found {
		return o, ErrNotFound
	}
	o.Version++
	o.Replanning = append(o.Replanning, ReplanEvent{Kind: "backing_withdrawn", ActorID: actor, Reason: in.Reason, FromVersion: o.Version - 1, ToVersion: o.Version, CreatedAt: s.now().UTC()})
	s.deriveOutcome(&o)
	return o, s.writeOutcome(o)
}

func (s *Store) ReplanOutcome(repo, oid, actor string, in ReplanInput) (FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.readOutcome(repo, oid)
	if e != nil {
		return o, e
	}
	f, e := s.read(repo, o.FundID)
	if e != nil {
		return o, e
	}
	if in.ExpectedVersion != o.Version {
		return o, ErrConflict
	}
	if !contains(f.Terms.StewardIDs, actor) {
		return o, ErrForbidden
	}
	if strings.TrimSpace(in.Reason) == "" || !validOutcomeTerms(in.Terms, f) {
		return o, ErrInvalid
	}
	from := o.Versions[len(o.Versions)-1]
	o.Version++
	o.Versions = append(o.Versions, OutcomeVersion{Version: o.Version, Terms: in.Terms, AuthorID: actor, ChangeReason: in.Reason, CreatedAt: s.now().UTC()})
	kind := "terms_changed"
	if from.Terms.Scope != in.Terms.Scope {
		kind = "scope_changed"
	}
	o.Replanning = append(o.Replanning, ReplanEvent{Kind: kind, ActorID: actor, Reason: in.Reason, FromVersion: from.Version, ToVersion: o.Version, CreatedAt: s.now().UTC()})
	s.deriveOutcome(&o)
	return o, s.writeOutcome(o)
}

func (s *Store) GetOutcome(repo, oid string) (FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.readOutcome(repo, oid)
	if e == nil {
		s.deriveOutcome(&o)
		s.addOverlapBlockers(repo, &o)
	}
	return o, e
}
func (s *Store) ListOutcomes(repo string) ([]FundedOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "outcomes", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []FundedOutcome{}
	for _, p := range paths {
		b, x := os.ReadFile(p)
		var o FundedOutcome
		if x == nil {
			x = json.Unmarshal(b, &o)
		}
		if x != nil {
			return nil, x
		}
		s.deriveOutcome(&o)
		out = append(out, o)
	}
	for i := range out {
		for j := range out {
			if i != j {
				addOverlap(&out[i], out[j])
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) deriveOutcome(o *FundedOutcome) {
	o.Pledged = 0
	o.Blockers = []OutcomeBlocker{}
	o.Delivery = []DeliveryReport{}
	targetBacking := map[string]int64{}
	for _, p := range o.Pledges {
		if p.State == "active" {
			o.Pledged += p.Amount
			targetBacking[p.Target] += p.Amount
		}
	}
	t := o.Versions[len(o.Versions)-1].Terms
	f, e := s.read(o.RepositoryID, o.FundID)
	aggregate := o.Pledged
	paths, _ := filepath.Glob(filepath.Join(s.root, o.RepositoryID, "outcomes", "*.json"))
	for _, path := range paths {
		if strings.TrimSuffix(filepath.Base(path), ".json") == o.ID {
			continue
		}
		b, x := os.ReadFile(path)
		var other FundedOutcome
		if x == nil && json.Unmarshal(b, &other) == nil && other.FundID == o.FundID {
			for _, pledge := range other.Pledges {
				if pledge.State == "active" {
					aggregate += pledge.Amount
				}
			}
		}
	}
	if e == nil && aggregate > f.Balances.Available+f.Balances.Reserved {
		o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "insufficient_funds", Detail: "active backing across funded outcomes exceeds the fund's settled available balance", ResourceID: f.ID})
	}
	if o.Pledged < t.Budget {
		o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "underfunded", Detail: "active backing does not cover the outcome budget"})
	}
	if o.Pledged > t.Budget {
		o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "overfunded", Detail: "active backing exceeds the current outcome budget and requires replanning"})
	}
	for _, milestone := range t.Milestones {
		if targetBacking["milestone:"+milestone.ID] > milestone.Budget {
			o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "milestone_overfunded", Detail: "milestone backing exceeds its declared budget", ResourceID: milestone.ID})
		}
	}
	if t.Embargoed {
		o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "embargoed_work", Detail: "work is embargoed and requires its separate visibility boundary"})
	}
	for _, c := range t.DeclaredConflicts {
		o.Blockers = append(o.Blockers, OutcomeBlocker{Kind: "declared_conflict", Detail: c})
	}
	proposals, _ := filepath.Glob(filepath.Join(s.root, o.RepositoryID, "outcomes", o.ID, "delivery-proposals", "*.json"))
	for _, path := range proposals {
		b, err := os.ReadFile(path)
		var p DeliveryProposal
		if err == nil && json.Unmarshal(b, &p) == nil && p.Execution != nil {
			deriveExecution(p.Execution)
			o.Delivery = append(o.Delivery, DeliveryReport{ProposalID: p.ID, RecipientID: p.Execution.ActiveRecipientID, Execution: *p.Execution})
			for _, blocker := range p.Execution.Blockers {
				o.Blockers = append(o.Blockers, blocker)
			}
		}
	}
}
func addOverlap(a *FundedOutcome, b FundedOutcome) {
	at, bt := a.Versions[len(a.Versions)-1].Terms, b.Versions[len(b.Versions)-1].Terms
	if a.FundID != b.FundID {
		return
	}
	for _, x := range at.OverlapKeys {
		for _, y := range bt.OverlapKeys {
			if x != "" && x == y {
				a.Blockers = append(a.Blockers, OutcomeBlocker{Kind: "overlapping_award", Detail: "another funded outcome claims the same award scope", ResourceID: b.ID})
				return
			}
		}
	}
}
func (s *Store) addOverlapBlockers(repo string, o *FundedOutcome) {
	paths, _ := filepath.Glob(filepath.Join(s.root, repo, "outcomes", "*.json"))
	for _, p := range paths {
		if strings.TrimSuffix(filepath.Base(p), ".json") == o.ID {
			continue
		}
		b, e := os.ReadFile(p)
		var other FundedOutcome
		if e == nil && json.Unmarshal(b, &other) == nil {
			addOverlap(o, other)
		}
	}
}
func (s *Store) readOutcome(repo, oid string) (FundedOutcome, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, "outcomes", oid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return FundedOutcome{}, ErrNotFound
	}
	var o FundedOutcome
	if e == nil {
		e = json.Unmarshal(b, &o)
	}
	return o, e
}
func (s *Store) writeOutcome(o FundedOutcome) error {
	d := filepath.Join(s.root, o.RepositoryID, "outcomes")
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "outcome-*.tmp")
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
		e = os.Rename(n, filepath.Join(d, o.ID+".json"))
	}
	return e
}
