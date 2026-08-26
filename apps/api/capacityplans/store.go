// Package capacityplans coordinates delivery of an evidence-supported scaling choice.
package capacityplans

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("capacity plan not found")
	ErrInvalid   = errors.New("invalid capacity plan")
	ErrConflict  = errors.New("capacity plan changed")
	ErrForbidden = errors.New("capacity plan action forbidden")
)

type Reservation struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	ProviderID string    `json:"provider_id,omitempty"`
	Quantity   float64   `json:"quantity"`
	Unit       string    `json:"unit"`
	NeededBy   time.Time `json:"needed_by"`
	OwnerID    string    `json:"owner_id"`
	ApprovalID string    `json:"approval_id,omitempty"`
}
type Dependency struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	ResourceID  string    `json:"resource_id"`
	Requirement string    `json:"requirement"`
	OwnerID     string    `json:"owner_id"`
	NeededBy    time.Time `json:"needed_by"`
	ApprovalID  string    `json:"approval_id,omitempty"`
}
type Gate struct {
	Kind     string `json:"kind"`
	Required bool   `json:"required"`
}
type Phase struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Order           int      `json:"order"`
	Scope           []string `json:"scope"`
	OwnerIDs        []string `json:"owner_ids"`
	Budget          float64  `json:"budget"`
	Currency        string   `json:"currency"`
	DependsOn       []string `json:"depends_on,omitempty"`
	ReservationIDs  []string `json:"reservation_ids,omitempty"`
	DependencyIDs   []string `json:"dependency_ids,omitempty"`
	Gates           []Gate   `json:"gates"`
	SuccessCriteria []string `json:"success_criteria"`
	ExitCriteria    []string `json:"exit_criteria"`
}
type DecisionPoint struct {
	ID               string    `json:"id"`
	AfterPhaseID     string    `json:"after_phase_id"`
	Question         string    `json:"question"`
	OwnerID          string    `json:"owner_id"`
	DueAt            time.Time `json:"due_at"`
	Options          []string  `json:"options"`
	EvidenceRequired []string  `json:"evidence_required"`
}
type Input struct {
	ObjectiveID       string          `json:"objective_id"`
	ObjectiveVersion  int64           `json:"objective_version"`
	ModelID           string          `json:"model_id"`
	ModelRevision     int64           `json:"model_revision"`
	RehearsalID       string          `json:"rehearsal_id"`
	RehearsalRevision int64           `json:"rehearsal_revision"`
	CandidateID       string          `json:"candidate_id"`
	Title             string          `json:"title"`
	Rationale         string          `json:"rationale"`
	OwnerIDs          []string        `json:"owner_ids"`
	Budget            float64         `json:"budget"`
	Currency          string          `json:"currency"`
	Reservations      []Reservation   `json:"reservations"`
	Dependencies      []Dependency    `json:"dependencies"`
	Phases            []Phase         `json:"phases"`
	DecisionPoints    []DecisionPoint `json:"decision_points"`
	ExitStrategy      []string        `json:"exit_strategy"`
}
type ApprovalInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Decision         string `json:"decision"`
	Rationale        string `json:"rationale"`
}
type Approval struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type WorkInput struct {
	ExpectedRevision int64             `json:"expected_revision"`
	PhaseID          string            `json:"phase_id"`
	Kind             string            `json:"kind"`
	ResourceID       string            `json:"resource_id"`
	Revision         string            `json:"revision,omitempty"`
	OwnerKind        string            `json:"owner_kind"`
	OwnerID          string            `json:"owner_id"`
	Status           string            `json:"status"`
	GateEvidence     map[string]string `json:"gate_evidence,omitempty"`
	Notes            string            `json:"notes,omitempty"`
}
type Work struct {
	ID string `json:"id"`
	WorkInput
	CreatorID string    `json:"creator_id"`
	CreatedAt time.Time `json:"created_at"`
}
type DecisionInput struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Outcome          string   `json:"outcome"`
	EvidenceIDs      []string `json:"evidence_ids"`
	Rationale        string   `json:"rationale"`
}
type Decision struct {
	ID              string    `json:"id"`
	DecisionPointID string    `json:"decision_point_id"`
	Outcome         string    `json:"outcome"`
	EvidenceIDs     []string  `json:"evidence_ids"`
	Rationale       string    `json:"rationale"`
	OwnerID         string    `json:"owner_id"`
	CreatedAt       time.Time `json:"created_at"`
}
type Gap struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	Reference string `json:"reference,omitempty"`
}
type Plan struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	AuthorID     string     `json:"author_id"`
	CreatedAt    time.Time  `json:"created_at"`
	Approvals    []Approval `json:"approvals"`
	Work         []Work     `json:"work"`
	Decisions    []Decision `json:"decisions"`
	Status       string     `json:"status"`
	Gaps         []Gap      `json:"gaps"`
	NonAuthority []string   `json:"non_authority"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func uid() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func valid(in Input) bool {
	if in.ObjectiveID == "" || in.ObjectiveVersion < 1 || in.ModelID == "" || in.ModelRevision < 1 || in.RehearsalID == "" || in.RehearsalRevision < 1 || in.CandidateID == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Rationale) == "" || len(in.OwnerIDs) == 0 || in.Budget <= 0 || in.Currency == "" || len(in.Phases) == 0 || len(in.ExitStrategy) == 0 {
		return false
	}
	owners := map[string]bool{}
	for _, x := range in.OwnerIDs {
		if x == "" || owners[x] {
			return false
		}
		owners[x] = true
	}
	rids := map[string]bool{}
	for _, x := range in.Reservations {
		if x.ID == "" || rids[x.ID] || !map[string]bool{"capacity": true, "quota": true, "hardware": true, "contract": true}[x.Kind] || x.ResourceID == "" || x.Quantity <= 0 || x.Unit == "" || x.NeededBy.IsZero() || x.OwnerID == "" {
			return false
		}
		rids[x.ID] = true
	}
	dids := map[string]bool{}
	for _, x := range in.Dependencies {
		if x.ID == "" || dids[x.ID] || !map[string]bool{"procurement": true, "quota": true, "provider": true, "dependency": true}[x.Kind] || x.ResourceID == "" || x.Requirement == "" || x.OwnerID == "" || x.NeededBy.IsZero() {
			return false
		}
		dids[x.ID] = true
	}
	pids := map[string]bool{}
	orders := map[int]bool{}
	for _, p := range in.Phases {
		if p.ID == "" || pids[p.ID] || p.Order < 1 || orders[p.Order] || len(p.Scope) == 0 || len(p.OwnerIDs) == 0 || p.Budget < 0 || p.Currency != in.Currency || len(p.Gates) == 0 || len(p.SuccessCriteria) == 0 || len(p.ExitCriteria) == 0 {
			return false
		}
		pids[p.ID] = true
		orders[p.Order] = true
		for _, r := range p.ReservationIDs {
			if !rids[r] {
				return false
			}
		}
		for _, d := range p.DependencyIDs {
			if !dids[d] {
				return false
			}
		}
	}
	for _, p := range in.Phases {
		for _, d := range p.DependsOn {
			if !pids[d] || d == p.ID {
				return false
			}
		}
	}
	points := map[string]bool{}
	for _, d := range in.DecisionPoints {
		if d.ID == "" || points[d.ID] || !pids[d.AfterPhaseID] || d.Question == "" || d.OwnerID == "" || d.DueAt.IsZero() || len(d.Options) < 2 || len(d.EvidenceRequired) == 0 {
			return false
		}
		points[d.ID] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Plan, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Plan{ID: uid(), RepositoryID: repo, Revision: 1, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	return p, s.write(p)
}
func (s *Store) Approve(repo, pid, actor string, in ApprovalInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return p, e
	}
	if p.Revision != in.ExpectedRevision {
		return p, ErrConflict
	}
	if !contains(p.OwnerIDs, actor) {
		return p, ErrForbidden
	}
	if !map[string]bool{"approved": true, "rejected": true, "revoked": true}[in.Decision] || strings.TrimSpace(in.Rationale) == "" {
		return p, ErrInvalid
	}
	p.Revision++
	p.Approvals = append(p.Approvals, Approval{ID: uid(), OwnerID: actor, Decision: in.Decision, Rationale: in.Rationale, CreatedAt: s.now().UTC()})
	return p, s.write(p)
}
func (s *Store) AddWork(repo, pid, actor string, in WorkInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return p, e
	}
	if p.Revision != in.ExpectedRevision {
		return p, ErrConflict
	}
	if !contains(p.OwnerIDs, actor) {
		return p, ErrForbidden
	}
	if !phase(p, in.PhaseID) || !map[string]bool{"task": true, "session": true, "workspace": true, "pull_request": true, "infrastructure_plan": true, "schema_change": true, "dependency_negotiation": true, "observability": true, "operational_documentation": true, "release": true, "environment_change": true}[in.Kind] || in.ResourceID == "" || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || in.OwnerID == "" || !map[string]bool{"planned": true, "active": true, "blocked": true, "completed": true, "cancelled": true}[in.Status] {
		return p, ErrInvalid
	}
	p.Revision++
	p.Work = append(p.Work, Work{ID: uid(), WorkInput: in, CreatorID: actor, CreatedAt: s.now().UTC()})
	return p, s.write(p)
}
func (s *Store) Decide(repo, pid, did, actor string, in DecisionInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pid)
	if e != nil {
		return p, e
	}
	if p.Revision != in.ExpectedRevision {
		return p, ErrConflict
	}
	var point *DecisionPoint
	for i := range p.DecisionPoints {
		if p.DecisionPoints[i].ID == did {
			point = &p.DecisionPoints[i]
		}
	}
	if point == nil {
		return p, ErrNotFound
	}
	if point.OwnerID != actor {
		return p, ErrForbidden
	}
	if !contains(point.Options, in.Outcome) || len(in.EvidenceIDs) == 0 || in.Rationale == "" {
		return p, ErrInvalid
	}
	p.Revision++
	p.Decisions = append(p.Decisions, Decision{ID: uid(), DecisionPointID: did, Outcome: in.Outcome, EvidenceIDs: in.EvidenceIDs, Rationale: in.Rationale, OwnerID: actor, CreatedAt: s.now().UTC()})
	return p, s.write(p)
}
func Resolve(p Plan) Plan {
	p.Gaps = nil
	p.NonAuthority = []string{"Plan approval grants no spending, procurement, provider, quota, repository, secret, review, merge, release, environment, deployment, or operational authority. Every linked resource remains subject to its ordinary owner approvals, checks, queues, and controls."}
	latest := map[string]string{}
	for _, a := range p.Approvals {
		latest[a.OwnerID] = a.Decision
	}
	for _, o := range p.OwnerIDs {
		if latest[o] != "approved" {
			p.Gaps = append(p.Gaps, Gap{Kind: "missing_plan_approval", Detail: "Current approval is required from plan owner " + o + ".", Reference: o})
		}
	}
	for _, r := range p.Reservations {
		if r.ApprovalID == "" {
			p.Gaps = append(p.Gaps, Gap{Kind: "unapproved_reservation", Detail: "Reservation retains separate owner authority.", Reference: r.ID})
		}
	}
	for _, d := range p.Dependencies {
		if d.ApprovalID == "" {
			p.Gaps = append(p.Gaps, Gap{Kind: "unresolved_dependency", Detail: "Procurement, quota, provider, or dependency approval remains external.", Reference: d.ID})
		}
	}
	for _, d := range p.DecisionPoints {
		found := false
		for _, x := range p.Decisions {
			found = found || x.DecisionPointID == d.ID
		}
		if !found {
			p.Gaps = append(p.Gaps, Gap{Kind: "open_decision", Detail: "Decision point has no accountable outcome.", Reference: d.ID})
		}
	}
	p.Status = "draft"
	if len(p.Gaps) == 0 {
		p.Status = "approved"
	}
	return p
}
func (s *Store) Get(repo, id string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Plan{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var p Plan
		if x == nil {
			x = json.Unmarshal(b, &p)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, p)
	}
	return out, nil
}
func (s *Store) read(repo, id string) (Plan, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Plan{}, ErrNotFound
	}
	var p Plan
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) write(p Plan) error {
	d := filepath.Join(s.root, p.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "plan-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e == nil {
		e = f.Chmod(0640)
	}
	if x := f.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(d, p.ID+".json"))
	}
	return e
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func phase(p Plan, id string) bool {
	for _, x := range p.Phases {
		if x.ID == id {
			return true
		}
	}
	return false
}
