// Package qualitygates retains revision-exact release confidence and its attributable risk decisions.
package qualitygates

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

var ErrNotFound = errors.New("quality gate not found")
var ErrInvalid = errors.New("invalid quality gate")
var ErrConflict = errors.New("quality gate changed")

type Selector struct {
	Branches     []string `json:"branches,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	Journeys     []string `json:"journeys,omitempty"`
	RiskClasses  []string `json:"risk_classes,omitempty"`
	Locales      []string `json:"locales,omitempty"`
	Platforms    []string `json:"platforms,omitempty"`
	Releases     []string `json:"releases,omitempty"`
}

// UnmarshalJSON preserves the public names of every independently selectable dimension.
func (s *Selector) UnmarshalJSON(b []byte) error {
	type wire struct {
		Branches    []string `json:"branches"`
		Paths       []string `json:"paths"`
		Journeys    []string `json:"journeys"`
		RiskClasses []string `json:"risk_classes"`
		Locales     []string `json:"locales"`
		Platforms   []string `json:"platforms"`
		Releases    []string `json:"releases"`
	}
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	s.Branches, s.Paths, s.Journeys, s.RiskClasses, s.Locales, s.Platforms, s.Releases = w.Branches, w.Paths, w.Journeys, w.RiskClasses, w.Locales, w.Platforms, w.Releases
	return nil
}
func (s Selector) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Branches    []string `json:"branches,omitempty"`
		Paths       []string `json:"paths,omitempty"`
		Journeys    []string `json:"journeys,omitempty"`
		RiskClasses []string `json:"risk_classes,omitempty"`
		Locales     []string `json:"locales,omitempty"`
		Platforms   []string `json:"platforms,omitempty"`
		Releases    []string `json:"releases,omitempty"`
	}{s.Branches, s.Paths, s.Journeys, s.RiskClasses, s.Locales, s.Platforms, s.Releases})
}

type Requirement struct {
	ID, BehaviorID, ScenarioID, Kind, Environment, Journey, RiskClass, Locale, Platform, OwnerID string
	Required                                                                                     bool `json:"required"`
}

func (r Requirement) MarshalJSON() ([]byte, error) {
	type out struct {
		ID          string `json:"id"`
		BehaviorID  string `json:"behavior_id"`
		ScenarioID  string `json:"scenario_id,omitempty"`
		Kind        string `json:"kind"`
		Environment string `json:"environment"`
		Journey     string `json:"journey,omitempty"`
		RiskClass   string `json:"risk_class,omitempty"`
		Locale      string `json:"locale,omitempty"`
		Platform    string `json:"platform,omitempty"`
		OwnerID     string `json:"owner_id"`
		Required    bool   `json:"required"`
	}
	return json.Marshal(out{r.ID, r.BehaviorID, r.ScenarioID, r.Kind, r.Environment, r.Journey, r.RiskClass, r.Locale, r.Platform, r.OwnerID, r.Required})
}
func (r *Requirement) UnmarshalJSON(b []byte) error {
	type out struct {
		ID          string `json:"id"`
		BehaviorID  string `json:"behavior_id"`
		ScenarioID  string `json:"scenario_id"`
		Kind        string `json:"kind"`
		Environment string `json:"environment"`
		Journey     string `json:"journey"`
		RiskClass   string `json:"risk_class"`
		Locale      string `json:"locale"`
		Platform    string `json:"platform"`
		OwnerID     string `json:"owner_id"`
		Required    bool   `json:"required"`
	}
	var x out
	if e := json.Unmarshal(b, &x); e != nil {
		return e
	}
	*r = Requirement{x.ID, x.BehaviorID, x.ScenarioID, x.Kind, x.Environment, x.Journey, x.RiskClass, x.Locale, x.Platform, x.OwnerID, x.Required}
	return nil
}

type PolicyInput struct {
	Name         string        `json:"name"`
	PlanID       string        `json:"plan_id"`
	PlanVersion  int64         `json:"plan_version"`
	Selector     Selector      `json:"selector"`
	Requirements []Requirement `json:"requirements"`
	ChangeReason string        `json:"change_reason"`
}
type PolicyVersion struct {
	Number int64 `json:"number"`
	PolicyInput
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Policy struct {
	ID, RepositoryID string
	CurrentVersion   int64           `json:"current_version"`
	Versions         []PolicyVersion `json:"versions"`
}

type Target struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
	Branch    string `json:"branch,omitempty"`
	Release   string `json:"release,omitempty"`
}
type OpenInput struct {
	PolicyID      string `json:"policy_id"`
	PolicyVersion int64  `json:"policy_version"`
	Target        Target `json:"target"`
}
type AttemptInput struct {
	RequirementID       string   `json:"requirement_id"`
	Kind                string   `json:"kind"`
	Status              string   `json:"status"`
	ScenarioVersion     int64    `json:"scenario_version,omitempty"`
	Environment         string   `json:"environment"`
	Locale              string   `json:"locale,omitempty"`
	Platform            string   `json:"platform,omitempty"`
	InputPaths          []string `json:"input_paths"`
	DependencyRevisions []string `json:"dependency_revisions"`
	Evidence            []string `json:"evidence"`
	FlakeReason         string   `json:"flake_reason,omitempty"`
	QuarantineReason    string   `json:"quarantine_reason,omitempty"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	Revision     string    `json:"revision"`
	ActorID      string    `json:"actor_id"`
	CreatedAt    time.Time `json:"created_at"`
	Stale        bool      `json:"stale"`
	StaleReasons []string  `json:"stale_reasons"`
}
type Acknowledgement struct {
	ID            string    `json:"id"`
	RequirementID string    `json:"requirement_id"`
	Decision      string    `json:"decision"`
	Rationale     string    `json:"rationale,omitempty"`
	Revision      string    `json:"revision"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type OverrideInput struct {
	RequirementIDs            []string `json:"requirement_ids"`
	Rationale, FollowUpWorkID string
	ExpiresAt                 time.Time `json:"expires_at"`
}
type Override struct {
	ID string `json:"id"`
	OverrideInput
	Revision  string    `json:"revision"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
	Active    bool      `json:"active"`
}
type RevisionInput struct {
	ExpectedRevision    string   `json:"expected_revision"`
	Revision            string   `json:"revision"`
	ChangedPaths        []string `json:"changed_paths"`
	ChangedDependencies []string `json:"changed_dependencies"`
}
type SignalInput struct {
	RequirementID  string `json:"requirement_id"`
	ReleaseID      string `json:"release_id"`
	Status         string `json:"status"`
	Evidence       string `json:"evidence"`
	Rationale      string `json:"rationale,omitempty"`
	FollowUpWorkID string `json:"follow_up_work_id,omitempty"`
}
type Signal struct {
	ID string `json:"id"`
	SignalInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type MatrixRow struct {
	Requirement      Requirement       `json:"requirement"`
	Attempts         []Attempt         `json:"attempts"`
	Status           string            `json:"status"`
	Gap              string            `json:"gap,omitempty"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Overrides        []Override        `json:"overrides"`
	Signals          []Signal          `json:"post_release_signals"`
}
type Gate struct {
	ID, RepositoryID string
	PolicyID         string            `json:"policy_id"`
	PolicyVersion    int64             `json:"policy_version"`
	Target           Target            `json:"target"`
	OpenedByID       string            `json:"opened_by_id"`
	OpenedAt         time.Time         `json:"opened_at"`
	Attempts         []Attempt         `json:"attempts"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Overrides        []Override        `json:"overrides"`
	Signals          []Signal          `json:"post_release_signals"`
	Matrix           []MatrixRow       `json:"matrix"`
	Ready            bool              `json:"ready"`
	Blockers         []string          `json:"blockers"`
}
type Catalog struct {
	Policies []Policy `json:"policies"`
	Gates    []Gate   `json:"gates"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func one(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func policyValid(x PolicyInput) bool {
	if x.Name == "" || x.PlanID == "" || x.PlanVersion < 1 || x.ChangeReason == "" || len(x.Requirements) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, r := range x.Requirements {
		if r.ID == "" || seen[r.ID] || r.BehaviorID == "" || r.Environment == "" || r.OwnerID == "" || !one(r.Kind, "scenario", "exploratory", "test") || (r.Kind == "scenario" && r.ScenarioID == "") {
			return false
		}
		seen[r.ID] = true
	}
	return true
}
func (s *Store) CreatePolicy(repo, actor string, in PolicyInput) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || !policyValid(in) {
		return Policy{}, ErrInvalid
	}
	now := s.now().UTC()
	p := Policy{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []PolicyVersion{{Number: 1, PolicyInput: in, AuthorID: actor, PublishedAt: now}}}
	return p, s.write("policies", repo, p.ID, p)
}
func (s *Store) RevisePolicy(repo, pid, actor string, expected int64, in PolicyInput) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readPolicy(repo, pid)
	if e != nil {
		return p, e
	}
	if p.CurrentVersion != expected {
		return p, ErrConflict
	}
	if actor == "" || !policyValid(in) {
		return p, ErrInvalid
	}
	p.CurrentVersion++
	p.Versions = append(p.Versions, PolicyVersion{Number: p.CurrentVersion, PolicyInput: in, AuthorID: actor, PublishedAt: s.now().UTC()})
	return p, s.write("policies", repo, p.ID, p)
}
func (s *Store) Open(repo, actor string, in OpenInput) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.readPolicy(repo, in.PolicyID)
	if e != nil {
		return Gate{}, e
	}
	if actor == "" || in.PolicyVersion < 1 || in.PolicyVersion > p.CurrentVersion || !one(in.Target.Kind, "pull_request", "merge_queue", "release") || in.Target.Reference == "" || in.Target.Revision == "" {
		return Gate{}, ErrInvalid
	}
	g := Gate{ID: id(), RepositoryID: repo, PolicyID: p.ID, PolicyVersion: in.PolicyVersion, Target: in.Target, OpenedByID: actor, OpenedAt: s.now().UTC(), Attempts: []Attempt{}, Acknowledgements: []Acknowledgement{}, Overrides: []Override{}, Signals: []Signal{}}
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func (s *Store) AddAttempt(repo, gid, actor string, in AttemptInput) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, gid)
	if e != nil {
		return g, e
	}
	r, ok := req(p, g.PolicyVersion, in.RequirementID)
	if actor == "" || !ok || in.Kind != r.Kind || !one(in.Status, "passed", "failed", "flaky", "quarantined") || in.Environment != r.Environment || in.Locale != r.Locale || in.Platform != r.Platform {
		return g, ErrInvalid
	}
	if len(in.Evidence) == 0 || (in.Status == "flaky" && in.FlakeReason == "") || (in.Status == "quarantined" && in.QuarantineReason == "") {
		return g, ErrInvalid
	}
	g.Attempts = append(g.Attempts, Attempt{ID: id(), AttemptInput: in, Revision: g.Target.Revision, ActorID: actor, CreatedAt: s.now().UTC(), StaleReasons: []string{}})
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func (s *Store) Acknowledge(repo, gid, actor, requirement, decision, rationale string) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, gid)
	if e != nil {
		return g, e
	}
	r, ok := req(p, g.PolicyVersion, requirement)
	if actor == "" || !ok || actor != r.OwnerID || !one(decision, "accepted", "rejected") || (decision == "rejected" && rationale == "") {
		return g, ErrInvalid
	}
	g.Acknowledgements = append(g.Acknowledgements, Acknowledgement{ID: id(), RequirementID: requirement, Decision: decision, Rationale: rationale, Revision: g.Target.Revision, ActorID: actor, CreatedAt: s.now().UTC()})
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func (s *Store) Override(repo, gid, actor string, in OverrideInput) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, gid)
	if e != nil {
		return g, e
	}
	if actor == "" || in.Rationale == "" || in.FollowUpWorkID == "" || !in.ExpiresAt.After(s.now()) || len(in.RequirementIDs) == 0 {
		return g, ErrInvalid
	}
	for _, x := range in.RequirementIDs {
		if _, ok := req(p, g.PolicyVersion, x); !ok {
			return g, ErrInvalid
		}
	}
	g.Overrides = append(g.Overrides, Override{ID: id(), OverrideInput: in, Revision: g.Target.Revision, ActorID: actor, CreatedAt: s.now().UTC()})
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func (s *Store) Revise(repo, gid, actor string, in RevisionInput) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, gid)
	if e != nil {
		return g, e
	}
	if actor == "" || in.ExpectedRevision != g.Target.Revision || in.Revision == "" || in.Revision == g.Target.Revision {
		return g, ErrConflict
	}
	g.Target.Revision = in.Revision
	for i := range g.Attempts {
		a := &g.Attempts[i]
		reasons := []string{}
		if intersects(a.InputPaths, in.ChangedPaths) {
			reasons = append(reasons, "affected_code_changed")
		}
		if intersects(a.DependencyRevisions, in.ChangedDependencies) {
			reasons = append(reasons, "affected_dependency_changed")
		}
		if len(reasons) > 0 {
			a.Stale = true
			a.StaleReasons = append(a.StaleReasons, reasons...)
		}
	}
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func (s *Store) Signal(repo, gid, actor string, in SignalInput) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, gid)
	if e != nil {
		return g, e
	}
	if _, ok := req(p, g.PolicyVersion, in.RequirementID); actor == "" || !ok || in.ReleaseID == "" || in.Evidence == "" || !one(in.Status, "verified", "reopened") || (in.Status == "reopened" && (in.Rationale == "" || in.FollowUpWorkID == "")) {
		return g, ErrInvalid
	}
	g.Signals = append(g.Signals, Signal{ID: id(), SignalInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	s.derive(&g, p)
	return g, s.write("gates", repo, g.ID, g)
}
func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y || strings.HasPrefix(x, strings.TrimSuffix(y, "*")) || strings.HasPrefix(y, strings.TrimSuffix(x, "*")) {
				return true
			}
		}
	}
	return false
}
func req(p Policy, n int64, id string) (Requirement, bool) {
	if n < 1 || int(n) > len(p.Versions) {
		return Requirement{}, false
	}
	for _, r := range p.Versions[n-1].Requirements {
		if r.ID == id {
			return r, true
		}
	}
	return Requirement{}, false
}
func (s *Store) derive(g *Gate, p Policy) {
	now := s.now().UTC()
	g.Matrix = []MatrixRow{}
	g.Blockers = []string{}
	g.Ready = true
	v := p.Versions[g.PolicyVersion-1]
	for _, r := range v.Requirements {
		row := MatrixRow{Requirement: r, Attempts: []Attempt{}, Acknowledgements: []Acknowledgement{}, Overrides: []Override{}, Signals: []Signal{}, Status: "missing", Gap: "no_current_attempt"}
		for _, a := range g.Attempts {
			if a.RequirementID == r.ID {
				row.Attempts = append(row.Attempts, a)
				if !a.Stale && a.Revision == g.Target.Revision {
					row.Status = a.Status
					row.Gap = ""
				}
			}
		}
		for _, a := range g.Acknowledgements {
			if a.RequirementID == r.ID && a.Revision == g.Target.Revision {
				row.Acknowledgements = append(row.Acknowledgements, a)
			}
		}
		for _, o := range g.Overrides {
			active := o.Revision == g.Target.Revision && o.ExpiresAt.After(now)
			for _, x := range o.RequirementIDs {
				if x == r.ID {
					o.Active = active
					row.Overrides = append(row.Overrides, o)
					if active && (row.Status != "passed") {
						row.Status = "overridden"
						row.Gap = "accepted_risk"
					}
				}
			}
		}
		for _, x := range g.Signals {
			if x.RequirementID == r.ID {
				row.Signals = append(row.Signals, x)
				if x.Status == "reopened" {
					row.Status = "reopened"
					row.Gap = "post_release_risk_reopened"
				}
			}
		}
		accepted := false
		for _, a := range row.Acknowledgements {
			if a.Decision == "accepted" {
				accepted = true
			}
			if a.Decision == "rejected" {
				accepted = false
			}
		}
		if r.Required && (row.Status != "passed" && row.Status != "overridden" || !accepted) {
			g.Ready = false
			g.Blockers = append(g.Blockers, r.ID+":"+row.Gap)
			if !accepted {
				g.Blockers = append(g.Blockers, r.ID+":owner_acknowledgement_missing")
			}
		}
		g.Matrix = append(g.Matrix, row)
	}
	sort.Strings(g.Blockers)
}
func (s *Store) Get(repo, id string) (Gate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, p, e := s.load(repo, id)
	if e == nil {
		s.derive(&g, p)
	}
	return g, e
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var c Catalog
	c.Policies = []Policy{}
	c.Gates = []Gate{}
	for _, kind := range []string{"policies", "gates"} {
		es, e := os.ReadDir(filepath.Join(s.root, kind, repo))
		if errors.Is(e, fs.ErrNotExist) {
			continue
		}
		if e != nil {
			return c, e
		}
		for _, x := range es {
			if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
				continue
			}
			id := strings.TrimSuffix(x.Name(), ".json")
			if kind == "policies" {
				p, e := s.readPolicy(repo, id)
				if e != nil {
					return c, e
				}
				c.Policies = append(c.Policies, p)
			} else {
				g, p, e := s.load(repo, id)
				if e != nil {
					return c, e
				}
				s.derive(&g, p)
				c.Gates = append(c.Gates, g)
			}
		}
	}
	return c, nil
}
func (s *Store) load(repo, id string) (Gate, Policy, error) {
	var g Gate
	if e := s.read("gates", repo, id, &g); e != nil {
		return g, Policy{}, e
	}
	p, e := s.readPolicy(repo, g.PolicyID)
	return g, p, e
}
func (s *Store) readPolicy(repo, id string) (Policy, error) {
	var p Policy
	e := s.read("policies", repo, id, &p)
	return p, e
}
func (s *Store) read(kind, repo, id string, out any) error {
	b, e := os.ReadFile(filepath.Join(s.root, kind, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return ErrNotFound
	}
	if e != nil {
		return e
	}
	if json.Unmarshal(b, out) != nil {
		return ErrNotFound
	}
	return nil
}
func (s *Store) write(kind, repo, id string, v any) error {
	dir := filepath.Join(s.root, kind, repo)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".quality-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(dir, id+".json"))
}
