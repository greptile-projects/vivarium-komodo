// Package reliabilitypolicies applies service-objective terms to delivery decisions.
package reliabilitypolicies

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

var ErrInvalid = errors.New("invalid reliability delivery policy")
var ErrNotFound = errors.New("reliability delivery policy not found")

type Rule struct {
	Condition        string  `json:"condition"`
	Action           string  `json:"action"`
	ThresholdPercent float64 `json:"threshold_percent,omitempty"`
}
type PolicyInput struct {
	Name             string   `json:"name"`
	ObjectiveID      string   `json:"objective_id"`
	ObjectiveVersion int64    `json:"objective_version"`
	Branches         []string `json:"branches,omitempty"`
	Services         []string `json:"services,omitempty"`
	Environments     []string `json:"environments,omitempty"`
	Journeys         []string `json:"journeys,omitempty"`
	RiskClasses      []string `json:"risk_classes,omitempty"`
	RequiredOwnerIDs []string `json:"required_owner_ids"`
	Rules            []Rule   `json:"rules"`
	ChangeReason     string   `json:"change_reason"`
}
type Policy struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	PolicyInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Context struct {
	Kind        string   `json:"kind"`
	ResourceID  string   `json:"resource_id"`
	Revision    string   `json:"revision"`
	Branch      string   `json:"branch,omitempty"`
	Service     string   `json:"service,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Journeys    []string `json:"journeys,omitempty"`
	RiskClass   string   `json:"risk_class,omitempty"`
}
type ImpactInput struct {
	Context                        Context  `json:"context"`
	Phase                          string   `json:"phase"`
	PredictedBudgetConsumedPercent float64  `json:"predicted_budget_consumed_percent,omitempty"`
	ObservedBudgetConsumedPercent  float64  `json:"observed_budget_consumed_percent,omitempty"`
	Regression                     bool     `json:"regression"`
	EvidenceStatus                 string   `json:"evidence_status"`
	DependencyStatus               string   `json:"dependency_status"`
	Summary                        string   `json:"summary"`
	Evidence                       []string `json:"evidence"`
}
type Impact struct {
	ID       string `json:"id"`
	PolicyID string `json:"policy_id"`
	ImpactInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Acknowledgement struct {
	ID        string    `json:"id"`
	PolicyID  string    `json:"policy_id"`
	Context   Context   `json:"context"`
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type Requirement struct {
	PolicyID string `json:"policy_id"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
	Action   string `json:"action"`
	Blocking bool   `json:"blocking"`
}
type Assessment struct {
	Context              Context           `json:"context"`
	AppliedPolicyIDs     []string          `json:"applied_policy_ids"`
	Impacts              []Impact          `json:"impacts"`
	Acknowledgements     []Acknowledgement `json:"acknowledgements"`
	ActiveExceptions     []string          `json:"active_exceptions"`
	Requirements         []Requirement     `json:"requirements"`
	AvailableNextActions []string          `json:"available_next_actions"`
	Ready                bool              `json:"ready"`
}
type ledger struct {
	Impacts          []Impact          `json:"impacts"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func listOK(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validPolicy(x PolicyInput) bool {
	if x.Name == "" || x.ObjectiveID == "" || x.ObjectiveVersion < 1 || x.ChangeReason == "" || !listOK(x.RequiredOwnerIDs, true) || len(x.Rules) == 0 {
		return false
	}
	for _, v := range [][]string{x.Branches, x.Services, x.Environments, x.Journeys, x.RiskClasses} {
		if !listOK(v, false) {
			return false
		}
	}
	for _, r := range x.Rules {
		if !map[string]bool{"budget_exhausted": true, "budget_threshold": true, "regression": true, "missing_evidence": true, "dependency_failure": true}[r.Condition] || !map[string]bool{"block": true, "slow": true, "pause": true, "rollback": true}[r.Action] || r.ThresholdPercent < 0 {
			return false
		}
	}
	return true
}
func write(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		e = os.WriteFile(path, b, 0640)
	}
	return e
}
func read[T any](path string) (T, error) {
	var v T
	b, e := os.ReadFile(path)
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) policyPath(repo, p string) string {
	return filepath.Join(s.root, repo, "policies", p+".json")
}
func (s *Store) ledgerPath(repo, p string) string {
	return filepath.Join(s.root, repo, "ledgers", p+".json")
}
func (s *Store) Create(repo, actor string, in PolicyInput) (Policy, error) {
	if repo == "" || actor == "" || !validPolicy(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Policy{ID: id(), RepositoryID: repo, PolicyInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	return p, write(s.policyPath(repo, p.ID), p)
}
func (s *Store) Get(repo, p string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return read[Policy](s.policyPath(repo, p))
}
func (s *Store) List(repo string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo, "policies"))
	if errors.Is(e, fs.ErrNotExist) {
		return []Policy{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Policy{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			p, x := read[Policy](filepath.Join(s.root, repo, "policies", f.Name()))
			if x != nil {
				return nil, x
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func validContext(c Context) bool {
	return map[string]bool{"pull_request": true, "integration_queue": true, "release": true, "deployment": true}[c.Kind] && c.ResourceID != "" && c.Revision != ""
}
func (s *Store) RecordImpact(repo, p, actor string, in ImpactInput) (Impact, error) {
	if actor == "" || !validContext(in.Context) || !map[string]bool{"predicted": true, "observed": true}[in.Phase] || !map[string]bool{"current": true, "missing": true, "stale": true}[in.EvidenceStatus] || !map[string]bool{"healthy": true, "failed": true, "unknown": true}[in.DependencyStatus] || in.Summary == "" || len(in.Evidence) == 0 {
		return Impact{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, e := read[Policy](s.policyPath(repo, p)); e != nil {
		return Impact{}, e
	}
	l, _ := read[ledger](s.ledgerPath(repo, p))
	x := Impact{ID: id(), PolicyID: p, ImpactInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	l.Impacts = append(l.Impacts, x)
	return x, write(s.ledgerPath(repo, p), l)
}
func same(a, b Context) bool {
	return a.Kind == b.Kind && a.ResourceID == b.ResourceID && a.Revision == b.Revision
}
func (s *Store) Acknowledge(repo, p, actor, decision, rationale string, c Context) (Acknowledgement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, e := read[Policy](s.policyPath(repo, p))
	if e != nil {
		return Acknowledgement{}, e
	}
	owner := false
	for _, x := range q.RequiredOwnerIDs {
		owner = owner || x == actor
	}
	if !owner || !validContext(c) || !map[string]bool{"acknowledged": true, "rejected": true}[decision] || rationale == "" {
		return Acknowledgement{}, ErrInvalid
	}
	l, _ := read[ledger](s.ledgerPath(repo, p))
	a := Acknowledgement{ID: id(), PolicyID: p, Context: c, OwnerID: actor, Decision: decision, Rationale: rationale, CreatedAt: s.now().UTC()}
	l.Acknowledgements = append(l.Acknowledgements, a)
	return a, write(s.ledgerPath(repo, p), l)
}
func intersects(xs []string, v string, vs []string) bool {
	if len(xs) == 0 {
		return true
	}
	for _, x := range xs {
		if x == v {
			return true
		}
		for _, y := range vs {
			if x == y {
				return true
			}
		}
	}
	return false
}
func applies(p Policy, c Context) bool {
	return intersects(p.Branches, c.Branch, nil) && intersects(p.Services, c.Service, nil) && intersects(p.Environments, c.Environment, nil) && intersects(p.Journeys, "", c.Journeys) && intersects(p.RiskClasses, c.RiskClass, nil)
}
func (s *Store) Assess(repo string, c Context, activeExceptions []string) (Assessment, error) {
	if !validContext(c) {
		return Assessment{}, ErrInvalid
	}
	ps, e := s.List(repo)
	if e != nil {
		return Assessment{}, e
	}
	out := Assessment{Context: c, AppliedPolicyIDs: []string{}, Impacts: []Impact{}, Acknowledgements: []Acknowledgement{}, ActiveExceptions: activeExceptions, Requirements: []Requirement{}, AvailableNextActions: []string{"supply_current_evidence", "request_owner_acknowledgement", "record_exception"}, Ready: true}
	for _, p := range ps {
		if !applies(p, c) {
			continue
		}
		out.AppliedPolicyIDs = append(out.AppliedPolicyIDs, p.ID)
		l, _ := read[ledger](s.ledgerPath(repo, p.ID))
		var latest *Impact
		for i := range l.Impacts {
			if same(l.Impacts[i].Context, c) {
				out.Impacts = append(out.Impacts, l.Impacts[i])
				latest = &l.Impacts[i]
			}
		}
		acked := map[string]bool{}
		for _, a := range l.Acknowledgements {
			if same(a.Context, c) {
				out.Acknowledgements = append(out.Acknowledgements, a)
				if a.Decision == "acknowledged" {
					acked[a.OwnerID] = true
				}
			}
		}
		if latest == nil {
			out.Requirements = append(out.Requirements, Requirement{p.ID, "missing_impact", "predicted or observed reliability impact is required", "block", true})
			out.Ready = false
			continue
		}
		for _, r := range p.Rules {
			hit := r.Condition == "budget_exhausted" && max(latest.PredictedBudgetConsumedPercent, latest.ObservedBudgetConsumedPercent) >= 100 || r.Condition == "budget_threshold" && max(latest.PredictedBudgetConsumedPercent, latest.ObservedBudgetConsumedPercent) >= r.ThresholdPercent || r.Condition == "regression" && latest.Regression || r.Condition == "missing_evidence" && latest.EvidenceStatus != "current" || r.Condition == "dependency_failure" && latest.DependencyStatus != "healthy"
			if hit {
				blocking := r.Action != "slow"
				out.Requirements = append(out.Requirements, Requirement{p.ID, r.Condition, "policy action required by current reliability impact", r.Action, blocking})
				if blocking {
					out.Ready = false
				}
				out.AvailableNextActions = append(out.AvailableNextActions, r.Action)
			}
		}
		for _, owner := range p.RequiredOwnerIDs {
			if !acked[owner] {
				out.Requirements = append(out.Requirements, Requirement{p.ID, "owner_acknowledgement", "required owner " + owner + " has not acknowledged this exact revision", "block", true})
				out.Ready = false
			}
		}
	}
	return out, nil
}
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
