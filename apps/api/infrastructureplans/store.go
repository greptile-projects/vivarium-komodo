// Package infrastructureplans retains immutable, non-executing infrastructure change plans.
package infrastructureplans

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

var (
	ErrNotFound = errors.New("infrastructure plan not found")
	ErrInvalid  = errors.New("invalid infrastructure plan")
)

type Pull interface {
	CurrentRevision(repository, pull string) (string, error)
}
type PullOutcome interface {
	MergedRevision(repository, pull string) (sourceRevision, mergeRevision string, merged bool, err error)
}
type EnvironmentAuthority interface {
	ExecutionEnvironment(repository, environment string) (requiredApprovals int, exists bool)
}
type Definitions interface {
	Get(repository, definition string) (infrastructurestate.Definition, error)
}
type DefinitionRef struct {
	ID             string   `json:"id"`
	Version        int64    `json:"version"`
	ObservationIDs []string `json:"observation_ids"`
}
type Risk struct {
	Kind       string `json:"kind"`
	Level      string `json:"level"`
	Detail     string `json:"detail"`
	Mitigation string `json:"mitigation,omitempty"`
}
type Change struct {
	ResourceID     string   `json:"resource_id"`
	Action         string   `json:"action"`
	EnvironmentIDs []string `json:"environment_ids"`
	DependsOn      []string `json:"depends_on"`
	OwnerIDs       []string `json:"owner_ids"`
	Summary        string   `json:"summary"`
	Risks          []Risk   `json:"risks"`
	RollbackLimit  string   `json:"rollback_limit"`
}
type PolicyEffect struct {
	PolicyID string `json:"policy_id"`
	Revision string `json:"revision"`
	Effect   string `json:"effect"`
	Detail   string `json:"detail"`
}
type Input struct {
	Revision       string          `json:"revision"`
	Definitions    []DefinitionRef `json:"definitions"`
	Changes        []Change        `json:"changes"`
	PolicyEffects  []PolicyEffect  `json:"policy_effects"`
	Assumptions    []string        `json:"assumptions"`
	RollbackLimits []string        `json:"rollback_limits"`
}
type Annotation struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Body              string    `json:"body"`
	ResourceIDs       []string  `json:"resource_ids,omitempty"`
	EvidenceReference string    `json:"evidence_reference,omitempty"`
	AuthorID          string    `json:"author_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Acknowledgement struct {
	ID            string     `json:"id"`
	OwnerID       string     `json:"owner_id"`
	ResourceIDs   []string   `json:"resource_ids"`
	RequestedByID string     `json:"requested_by_id"`
	RequestedAt   time.Time  `json:"requested_at"`
	Decision      string     `json:"decision,omitempty"`
	Rationale     string     `json:"rationale,omitempty"`
	DecidedByID   string     `json:"decided_by_id,omitempty"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	Current       bool       `json:"current"`
}
type Invalidation struct {
	Kind      string    `json:"kind"`
	Reference string    `json:"reference"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Plan struct {
	ID               string            `json:"id"`
	RepositoryID     string            `json:"repository_id"`
	PullRequestID    string            `json:"pull_request_id"`
	Input            Input             `json:"input"`
	DependencyOrder  []string          `json:"dependency_order"`
	CreatedByID      string            `json:"created_by_id"`
	CreatedAt        time.Time         `json:"created_at"`
	Annotations      []Annotation      `json:"annotations"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Invalidations    []Invalidation    `json:"invalidations"`
	Rehearsals       []Rehearsal       `json:"rehearsals"`
	Executions       []Execution       `json:"executions"`
	Stale            bool              `json:"stale"`
	StaleReasons     []string          `json:"stale_reasons"`
	NonAuthority     []string          `json:"non_authority"`
}
type Store struct {
	root         string
	pulls        Pull
	definitions  Definitions
	mu           sync.Mutex
	now          func() time.Time
	environments EnvironmentAuthority
}

// ConfigureExecutionAuthority binds execution to the repository's established
// release environments. Plans remain usable without it, but cannot execute.
func (s *Store) ConfigureExecutionAuthority(environments EnvironmentAuthority) {
	s.environments = environments
}

func New(root string, pulls Pull, definitions Definitions) (*Store, error) {
	if strings.TrimSpace(root) == "" || pulls == nil || definitions == nil {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, pulls: pulls, definitions: definitions, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, pull, plan string) string {
	return filepath.Join(s.root, repo, pull, plan+".json")
}
func (s *Store) save(p Plan) error {
	f := s.path(p.RepositoryID, p.PullRequestID, p.ID)
	if e := os.MkdirAll(filepath.Dir(f), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e == nil {
		e = os.WriteFile(f+".tmp", b, 0640)
	}
	if e == nil {
		e = os.Rename(f+".tmp", f)
	}
	return e
}
func (s *Store) load(repo, pull, plan string) (Plan, error) {
	var p Plan
	b, e := os.ReadFile(s.path(repo, pull, plan))
	if os.IsNotExist(e) {
		return p, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func valid(in Input) bool {
	if in.Revision == "" || len(in.Definitions) == 0 || len(in.Changes) == 0 || len(in.PolicyEffects) == 0 || len(in.Assumptions) == 0 || len(in.RollbackLimits) == 0 {
		return false
	}
	seen := map[string]bool{}
	riskKinds := map[string]bool{}
	for _, c := range in.Changes {
		if c.ResourceID == "" || seen[c.ResourceID] || !map[string]bool{"create": true, "change": true, "replace": true, "destroy": true}[c.Action] || len(c.EnvironmentIDs) == 0 || c.Summary == "" || c.RollbackLimit == "" {
			return false
		}
		if secretShaped(c.Summary) || secretShaped(c.RollbackLimit) {
			return false
		}
		seen[c.ResourceID] = true
		for _, r := range c.Risks {
			if !map[string]bool{"availability": true, "security": true, "privacy": true, "continuity": true, "cost": true, "data": true}[r.Kind] || !map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[r.Level] || r.Detail == "" || secretShaped(r.Detail) || secretShaped(r.Mitigation) {
				return false
			}
			riskKinds[r.Kind] = true
		}
	}
	for _, kind := range []string{"availability", "security", "privacy", "continuity", "cost", "data"} {
		if !riskKinds[kind] {
			return false
		}
	}
	for _, c := range in.Changes {
		for _, d := range c.DependsOn {
			if !seen[d] {
				return false
			}
		}
	}
	for _, p := range in.PolicyEffects {
		if p.PolicyID == "" || p.Revision == "" || !map[string]bool{"satisfy": true, "violate": true, "exception_required": true, "unknown": true}[p.Effect] || p.Detail == "" || secretShaped(p.Detail) {
			return false
		}
	}
	for _, values := range [][]string{in.Assumptions, in.RollbackLimits} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" || secretShaped(value) {
				return false
			}
		}
	}
	return true
}

func secretShaped(value string) bool {
	v := strings.ToLower(value)
	for _, m := range []string{"-----begin private key", "password=", "password:", "token=", "token:", "secret=", "secret:", "vka_"} {
		if strings.Contains(v, m) {
			return true
		}
	}
	return false
}
func order(cs []Change) ([]string, bool) {
	by := map[string]Change{}
	for _, c := range cs {
		by[c.ResourceID] = c
	}
	out := []string{}
	state := map[string]int{}
	var visit func(string) bool
	visit = func(x string) bool {
		if state[x] == 1 {
			return false
		}
		if state[x] == 2 {
			return true
		}
		state[x] = 1
		ds := append([]string{}, by[x].DependsOn...)
		sort.Strings(ds)
		for _, d := range ds {
			if !visit(d) {
				return false
			}
		}
		state[x] = 2
		out = append(out, x)
		return true
	}
	ids := []string{}
	for x := range by {
		ids = append(ids, x)
	}
	sort.Strings(ids)
	for _, x := range ids {
		if !visit(x) {
			return nil, false
		}
	}
	return out, true
}
func (s *Store) validateRefs(repo string, in Input) bool {
	for _, r := range in.Definitions {
		d, e := s.definitions.Get(repo, r.ID)
		if e != nil || r.Version < 1 || r.Version > int64(len(d.Versions)) {
			return false
		}
		known := map[string]bool{}
		for _, o := range d.Observations {
			known[o.ID] = true
		}
		for _, o := range r.ObservationIDs {
			if !known[o] {
				return false
			}
		}
	}
	return true
}
func (s *Store) Create(repo, pull, actor string, in Input) (Plan, error) {
	rev, e := s.pulls.CurrentRevision(repo, pull)
	ord, ok := order(in.Changes)
	if e != nil || actor == "" || !valid(in) || rev != in.Revision || !ok || !s.validateRefs(repo, in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Plan{ID: id(), RepositoryID: repo, PullRequestID: pull, Input: in, DependencyOrder: ord, CreatedByID: actor, CreatedAt: s.now().UTC()}
	if e = s.save(p); e != nil {
		return Plan{}, e
	}
	return s.derive(p), nil
}
func (s *Store) derive(p Plan) Plan {
	p.StaleReasons = nil
	rev, e := s.pulls.CurrentRevision(p.RepositoryID, p.PullRequestID)
	if e != nil || rev != p.Input.Revision {
		p.StaleReasons = append(p.StaleReasons, "source_revision_changed")
	}
	for _, r := range p.Input.Definitions {
		d, e := s.definitions.Get(p.RepositoryID, r.ID)
		if e != nil {
			p.StaleReasons = append(p.StaleReasons, "definition_unavailable:"+r.ID)
			continue
		}
		if d.CurrentVersion != r.Version {
			p.StaleReasons = append(p.StaleReasons, "definition_changed:"+r.ID)
		}
		latest := map[string]string{}
		for _, o := range d.Observations {
			latest[o.EnvironmentID] = o.ID
		}
		used := map[string]bool{}
		for _, o := range r.ObservationIDs {
			used[o] = true
		}
		for _, o := range latest {
			if !used[o] {
				p.StaleReasons = append(p.StaleReasons, "observed_state_changed:"+r.ID)
				break
			}
		}
	}
	for _, i := range p.Invalidations {
		p.StaleReasons = append(p.StaleReasons, i.Kind+"_changed:"+i.Reference)
	}
	sort.Strings(p.StaleReasons)
	p.Stale = len(p.StaleReasons) > 0
	p.NonAuthority = []string{"plan and collaboration grant no provider, credential, deployment, environment, policy, approval, or execution authority"}
	for x := range p.Acknowledgements {
		p.Acknowledgements[x].Current = !p.Stale
	}
	for x := range p.Rehearsals {
		deriveRehearsal(&p.Rehearsals[x], p.Stale)
	}
	for x := range p.Executions {
		deriveExecution(&p.Executions[x], s.now().UTC())
	}
	return p
}
func (s *Store) mutate(repo, pull, plan string, fn func(*Plan) error) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.load(repo, pull, plan)
	if e == nil {
		e = fn(&p)
	}
	if e == nil {
		e = s.save(p)
	}
	return s.derive(p), e
}
func (s *Store) Get(repo, pull, plan string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.load(repo, pull, plan)
	return s.derive(p), e
}
func (s *Store) List(repo, pull string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, pull)
	es, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return []Plan{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Plan{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			p, er := s.load(repo, pull, strings.TrimSuffix(f.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, s.derive(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Annotate(repo, pull, plan, actor, kind, body, evidence string, resources []string) (Plan, error) {
	if actor == "" || body == "" || secretShaped(body) || secretShaped(evidence) || !map[string]bool{"assumption": true, "impact": true, "investigation": true, "concern": true}[kind] {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		known := map[string]bool{}
		for _, c := range p.Input.Changes {
			known[c.ResourceID] = true
		}
		for _, r := range resources {
			if !known[r] {
				return ErrInvalid
			}
		}
		p.Annotations = append(p.Annotations, Annotation{ID: id(), Kind: kind, Body: body, ResourceIDs: resources, EvidenceReference: evidence, AuthorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Request(repo, pull, plan, actor, owner string, resources []string) (Plan, error) {
	if actor == "" || owner == "" || len(resources) == 0 {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		matched := map[string]bool{}
		for _, r := range resources {
			for _, c := range p.Input.Changes {
				if c.ResourceID == r {
					for _, o := range c.OwnerIDs {
						if o == owner {
							matched[r] = true
						}
					}
				}
			}
		}
		if len(matched) != len(resources) {
			return ErrInvalid
		}
		p.Acknowledgements = append(p.Acknowledgements, Acknowledgement{ID: id(), OwnerID: owner, ResourceIDs: resources, RequestedByID: actor, RequestedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Decide(repo, pull, plan, ack, actor, decision, rationale string) (Plan, error) {
	if actor == "" || rationale == "" || !map[string]bool{"acknowledged": true, "concern": true}[decision] {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		for x := range p.Acknowledgements {
			a := &p.Acknowledgements[x]
			if a.ID == ack && a.OwnerID == actor && a.Decision == "" {
				n := s.now().UTC()
				a.Decision = decision
				a.Rationale = rationale
				a.DecidedByID = actor
				a.DecidedAt = &n
				return nil
			}
		}
		return ErrInvalid
	})
}
func (s *Store) Invalidate(repo, pull, plan, actor, kind, ref string) (Plan, error) {
	if actor == "" || ref == "" || !map[string]bool{"provider": true, "policy": true, "observed_state": true, "source": true}[kind] {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		p.Invalidations = append(p.Invalidations, Invalidation{Kind: kind, Reference: ref, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
