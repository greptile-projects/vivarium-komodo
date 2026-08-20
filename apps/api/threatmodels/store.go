// Package threatmodels retains revision-bound, collaborative design-time attack analysis.
package threatmodels

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

var ErrNotFound = errors.New("threat model not found")
var ErrInvalid = errors.New("invalid threat model")

type Origin struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
}
type InputBinding struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
}
type EntryPoint struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Privileges  []string `json:"privileges"`
	OwnerIDs    []string `json:"owner_ids"`
}
type DataFlow struct {
	ID            string   `json:"id"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Data          []string `json:"data"`
	Boundary      string   `json:"trust_boundary"`
	DependencyIDs []string `json:"dependency_ids"`
}
type Dependency struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Revision string   `json:"revision"`
	Trust    string   `json:"trust"`
	OwnerIDs []string `json:"owner_ids"`
}
type AttackerGoal struct {
	ID         string `json:"id"`
	Actor      string `json:"actor"`
	Goal       string `json:"goal"`
	Capability string `json:"capability"`
	Impact     string `json:"impact"`
}
type AbusePath struct {
	ID            string   `json:"id"`
	GoalID        string   `json:"goal_id"`
	EntryPointIDs []string `json:"entry_point_ids"`
	DataFlowIDs   []string `json:"data_flow_ids"`
	DependencyIDs []string `json:"dependency_ids"`
	Steps         []string `json:"steps"`
	MitigationIDs []string `json:"mitigation_ids"`
	ResidualRisk  string   `json:"residual_risk"`
	Severity      string   `json:"severity"`
	OwnerIDs      []string `json:"owner_ids"`
}
type Mitigation struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Evidence    []Citation `json:"evidence"`
	OwnerIDs    []string   `json:"owner_ids"`
}
type Alternative struct {
	ID             string     `json:"id"`
	Description    string     `json:"description"`
	SecurityEffect string     `json:"security_effect"`
	AbusePathIDs   []string   `json:"abuse_path_ids"`
	Evidence       []Citation `json:"evidence"`
}
type Citation struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision"`
	Path       string `json:"path,omitempty"`
	Detail     string `json:"detail"`
	Visibility string `json:"visibility"`
}
type Input struct {
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	Origin        Origin         `json:"origin"`
	Inputs        []InputBinding `json:"inputs"`
	EntryPoints   []EntryPoint   `json:"entry_points"`
	DataFlows     []DataFlow     `json:"data_flows"`
	Dependencies  []Dependency   `json:"dependencies"`
	AttackerGoals []AttackerGoal `json:"attacker_goals"`
	AbusePaths    []AbusePath    `json:"abuse_paths"`
	Mitigations   []Mitigation   `json:"mitigations"`
	Alternatives  []Alternative  `json:"alternatives"`
	OwnerIDs      []string       `json:"owner_ids"`
	ResidualRisk  string         `json:"residual_risk"`
}
type FindingInput struct {
	Kind           string     `json:"kind"`
	Body           string     `json:"body"`
	AbusePathIDs   []string   `json:"abuse_path_ids"`
	AlternativeIDs []string   `json:"alternative_ids"`
	Citations      []Citation `json:"citations"`
}
type Finding struct {
	ID string `json:"id"`
	FindingInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Acknowledgement struct {
	OwnerID        string    `json:"owner_id"`
	Decision       string    `json:"decision"`
	Rationale      string    `json:"rationale"`
	OriginRevision string    `json:"origin_revision"`
	CreatedAt      time.Time `json:"created_at"`
	Stale          bool      `json:"stale"`
}
type StaleInput struct {
	Kind             string `json:"kind"`
	Reference        string `json:"reference"`
	ExpectedRevision string `json:"expected_revision"`
	CurrentRevision  string `json:"current_revision"`
}
type Model struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	CreatedByID      string            `json:"created_by_id"`
	CreatedAt        time.Time         `json:"created_at"`
	Findings         []Finding         `json:"findings"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Stale            bool              `json:"stale"`
	StaleInputs      []StaleInput      `json:"stale_inputs"`
	NonAuthority     []string          `json:"non_authority"`
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
func id() string                { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
func unique(v string, seen map[string]bool) bool {
	if !text(v, 200) || seen[v] {
		return false
	}
	seen[v] = true
	return true
}
func refs(xs []string, known map[string]bool) bool {
	for _, x := range xs {
		if !known[x] {
			return false
		}
	}
	return true
}
func citation(c Citation) bool {
	return text(c.Kind, 50) && text(c.Reference, 500) && text(c.Revision, 200) && text(c.Detail, 4000) && (c.Visibility == "public" || c.Visibility == "repository")
}
func valid(in Input) bool {
	if !text(in.Title, 500) || !text(in.Summary, 65536) || !map[string]bool{"design_proposal": true, "pull_request": true, "api_evolution": true, "schema_evolution": true, "infrastructure_plan": true, "product_experiment": true}[in.Origin.Kind] || !text(in.Origin.Reference, 500) || !text(in.Origin.Revision, 200) || len(in.Inputs) == 0 || len(in.EntryPoints) == 0 || len(in.AttackerGoals) == 0 || len(in.AbusePaths) == 0 || len(in.OwnerIDs) == 0 || !text(in.ResidualRisk, 65536) {
		return false
	}
	for _, x := range in.Inputs {
		if !map[string]bool{"code": true, "architecture": true, "dependency": true, "trust_boundary": true}[x.Kind] || !text(x.Reference, 500) || !text(x.Revision, 200) {
			return false
		}
	}
	eps, flows, deps, goals, paths, mits, alts := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.EntryPoints {
		if !unique(x.ID, eps) || !text(x.Description, 4000) || len(x.Privileges) == 0 {
			return false
		}
	}
	for _, x := range in.Dependencies {
		if !unique(x.ID, deps) || !text(x.Name, 500) || !text(x.Revision, 200) || !text(x.Trust, 4000) {
			return false
		}
	}
	for _, x := range in.DataFlows {
		if !unique(x.ID, flows) || !text(x.From, 500) || !text(x.To, 500) || len(x.Data) == 0 || !text(x.Boundary, 1000) || !refs(x.DependencyIDs, deps) {
			return false
		}
	}
	for _, x := range in.AttackerGoals {
		if !unique(x.ID, goals) || !text(x.Actor, 1000) || !text(x.Goal, 4000) || !text(x.Capability, 4000) || !text(x.Impact, 4000) {
			return false
		}
	}
	for _, x := range in.Mitigations {
		if !unique(x.ID, mits) || !text(x.Description, 4000) || !map[string]bool{"proposed": true, "implemented": true, "verified": true, "rejected": true}[x.Status] {
			return false
		}
		for _, c := range x.Evidence {
			if !citation(c) {
				return false
			}
		}
	}
	for _, x := range in.AbusePaths {
		if !unique(x.ID, paths) || !goals[x.GoalID] || !refs(x.EntryPointIDs, eps) || !refs(x.DataFlowIDs, flows) || !refs(x.DependencyIDs, deps) || !refs(x.MitigationIDs, mits) || len(x.Steps) == 0 || !text(x.ResidualRisk, 4000) || !map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[x.Severity] || len(x.OwnerIDs) == 0 {
			return false
		}
	}
	for _, x := range in.Alternatives {
		if !unique(x.ID, alts) || !text(x.Description, 4000) || !text(x.SecurityEffect, 4000) || !refs(x.AbusePathIDs, paths) {
			return false
		}
		for _, c := range x.Evidence {
			if !citation(c) {
				return false
			}
		}
	}
	return true
}
func (s *Store) path(repo, mid string) string { return filepath.Join(s.root, repo, mid+".json") }
func (s *Store) write(m Model) error {
	if e := os.MkdirAll(filepath.Dir(s.path(m.RepositoryID, m.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(m, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(m.RepositoryID, m.ID), b, 0640)
	}
	return e
}
func (s *Store) read(repo, mid string) (Model, error) {
	var m Model
	b, e := os.ReadFile(s.path(repo, mid))
	if errors.Is(e, fs.ErrNotExist) {
		return m, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &m)
	}
	if e != nil || m.ID != mid || m.RepositoryID != repo {
		return Model{}, ErrNotFound
	}
	return m, nil
}
func (s *Store) Create(repo, actor string, in Input) (Model, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Model{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := Model{ID: id(), RepositoryID: repo, Input: in, CreatedByID: actor, CreatedAt: s.now().UTC(), Findings: []Finding{}, Acknowledgements: []Acknowledgement{}, NonAuthority: []string{"repository write", "secret or credential access", "security approval", "review or merge", "release, deployment, environment, or provider authority"}}
	return m, s.write(m)
}
func (s *Store) mutate(repo, mid string, fn func(*Model) error) (Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.read(repo, mid)
	if e == nil {
		e = fn(&m)
	}
	if e == nil {
		e = s.write(m)
	}
	return m, e
}
func (s *Store) AddFinding(repo, mid, actor string, in FindingInput) (Model, error) {
	return s.mutate(repo, mid, func(m *Model) error {
		if actor == "" || !map[string]bool{"finding": true, "challenge": true, "assumption": true, "alternative_comparison": true}[in.Kind] || !text(in.Body, 65536) || len(in.Citations) == 0 {
			return ErrInvalid
		}
		p, a := map[string]bool{}, map[string]bool{}
		for _, x := range m.AbusePaths {
			p[x.ID] = true
		}
		for _, x := range m.Alternatives {
			a[x.ID] = true
		}
		if !refs(in.AbusePathIDs, p) || !refs(in.AlternativeIDs, a) {
			return ErrInvalid
		}
		for _, c := range in.Citations {
			if !citation(c) {
				return ErrInvalid
			}
		}
		m.Findings = append(m.Findings, Finding{ID: id(), FindingInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Acknowledge(repo, mid, actor, decision, rationale, revision string) (Model, error) {
	return s.mutate(repo, mid, func(m *Model) error {
		ok := false
		for _, o := range m.OwnerIDs {
			ok = ok || o == actor
		}
		if !ok || !map[string]bool{"acknowledge": true, "request_changes": true}[decision] || !text(rationale, 65536) || revision != m.Origin.Revision {
			return ErrInvalid
		}
		m.Acknowledgements = append(m.Acknowledgements, Acknowledgement{OwnerID: actor, Decision: decision, Rationale: rationale, OriginRevision: revision, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Get(repo, mid string) (Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, mid)
}
func (s *Store) List(repo string) ([]Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Model{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Model{}
	for _, x := range es {
		if x.IsDir() {
			continue
		}
		m, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func Derive(m *Model, current map[string]string) {
	m.StaleInputs = []StaleInput{}
	for _, x := range m.Inputs {
		key := x.Kind + ":" + x.Reference
		if v, ok := current[key]; ok && v != x.Revision {
			m.StaleInputs = append(m.StaleInputs, StaleInput{x.Kind, x.Reference, x.Revision, v})
		}
	}
	if v, ok := current["origin:"+m.Origin.Reference]; ok && v != m.Origin.Revision {
		m.StaleInputs = append(m.StaleInputs, StaleInput{"origin", m.Origin.Reference, m.Origin.Revision, v})
	}
	m.Stale = len(m.StaleInputs) > 0
	for i := range m.Acknowledgements {
		m.Acknowledgements[i].Stale = m.Stale || m.Acknowledgements[i].OriginRevision != m.Origin.Revision
	}
}
