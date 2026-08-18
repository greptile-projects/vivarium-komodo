// Package designgovernance retains design acceptance policy and the accountable
// work created when a shared interaction system evolves or a shipped surface
// regresses. It deliberately carries no repository or deployment authority.
package designgovernance

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
)

var ErrInvalid = errors.New("invalid design governance record")
var ErrNotFound = errors.New("design governance record not found")

type Selector struct {
	Components  []string `json:"components,omitempty"`
	Journeys    []string `json:"journeys,omitempty"`
	Paths       []string `json:"paths,omitempty"`
	RiskClasses []string `json:"risk_classes,omitempty"`
}
type PolicyInput struct {
	Name           string   `json:"name"`
	TargetBranches []string `json:"target_branches"`
	Selector
	RequiredRoles []string `json:"required_roles"`
}
type Policy struct {
	ID             string `json:"id"`
	ScopeKind      string `json:"scope_kind"`
	ScopeID        string `json:"scope_id"`
	OrganizationID string `json:"organization_id,omitempty"`
	PolicyInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Acceptance struct {
	ID            string    `json:"id"`
	RepositoryID  string    `json:"repository_id"`
	PullRequestID string    `json:"pull_request_id"`
	PolicyID      string    `json:"policy_id"`
	Revision      string    `json:"revision"`
	PreviewID     string    `json:"preview_id"`
	Role          string    `json:"role"`
	Decision      string    `json:"decision"`
	Rationale     string    `json:"rationale"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type Exception struct {
	ID            string    `json:"id"`
	RepositoryID  string    `json:"repository_id"`
	PullRequestID string    `json:"pull_request_id"`
	PolicyID      string    `json:"policy_id"`
	Revision      string    `json:"revision"`
	Reason        string    `json:"reason"`
	OwnerID       string    `json:"owner_id"`
	FollowUpKind  string    `json:"follow_up_kind"`
	FollowUpID    string    `json:"follow_up_id"`
	CreatedByID   string    `json:"created_by_id"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}
type Usage struct {
	ComponentID string `json:"component_id"`
	Version     int64  `json:"version"`
	Path        string `json:"path"`
	Obsolete    bool   `json:"obsolete"`
}
type Requirement struct {
	PolicyID string `json:"policy_id,omitempty"`
	Role     string `json:"role,omitempty"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
}
type Assessment struct {
	Revision          string        `json:"revision"`
	AppliedPolicyIDs  []string      `json:"applied_policy_ids"`
	Requirements      []Requirement `json:"requirements"`
	ObsoleteUses      []Usage       `json:"obsolete_component_uses"`
	ExpiringException []Exception   `json:"expiring_exceptions"`
	Ready             bool          `json:"ready"`
}
type WorkInput struct {
	Kind               string   `json:"kind"`
	SystemID           string   `json:"system_id,omitempty"`
	SystemVersion      int64    `json:"system_version,omitempty"`
	ReleaseID          string   `json:"release_id,omitempty"`
	SourceKind         string   `json:"source_kind"`
	SourceID           string   `json:"source_id"`
	AffectedRepository string   `json:"affected_repository"`
	DocumentationIDs   []string `json:"documentation_ids,omitempty"`
	OwnerID            string   `json:"owner_id"`
	Summary            string   `json:"summary"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type Work struct {
	ID string `json:"id"`
	WorkInput
	CreatedByID     string    `json:"created_by_id"`
	CreatedAt       time.Time `json:"created_at"`
	GrantsAuthority bool      `json:"grants_authority"`
}
type ledger struct {
	Acceptances []Acceptance `json:"acceptances"`
	Exceptions  []Exception  `json:"exceptions"`
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
func ident() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validList(v []string, required bool) bool {
	if required && len(v) == 0 || len(v) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 500 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validPolicy(v PolicyInput) bool {
	return strings.TrimSpace(v.Name) != "" && validList(v.TargetBranches, true) && validList(v.RequiredRoles, true) && validList(v.Components, false) && validList(v.Journeys, false) && validList(v.Paths, false) && validList(v.RiskClasses, false)
}
func (s *Store) policyPath(scope, id string) string {
	return filepath.Join(s.root, "policies", scope, id+".json")
}
func (s *Store) ledgerPath(repo, pull string) string {
	return filepath.Join(s.root, "ledgers", repo, pull+".json")
}
func write(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, path)
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
func (s *Store) CreatePolicy(kind, scope, organization, actor string, in PolicyInput) (Policy, error) {
	if !map[string]bool{"repository": true, "organization": true}[kind] || scope == "" || actor == "" || !validPolicy(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Policy{ID: ident(), ScopeKind: kind, ScopeID: scope, OrganizationID: organization, PolicyInput: in, CreatedByID: actor, CreatedAt: s.now().UTC()}
	return p, write(s.policyPath(kind+"-"+scope, p.ID), p)
}
func (s *Store) ListPolicies(kind, scope string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, "policies", kind+"-"+scope)
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []Policy{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Policy{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := read[Policy](filepath.Join(dir, x.Name()))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) loadLedger(repo, pull string) ledger {
	v, _ := read[ledger](s.ledgerPath(repo, pull))
	if v.Acceptances == nil {
		v.Acceptances = []Acceptance{}
	}
	if v.Exceptions == nil {
		v.Exceptions = []Exception{}
	}
	return v
}
func (s *Store) findPolicy(id string) (Policy, error) {
	paths, _ := filepath.Glob(filepath.Join(s.root, "policies", "*", id+".json"))
	if len(paths) != 1 {
		return Policy{}, ErrNotFound
	}
	return read[Policy](paths[0])
}
func (s *Store) Accept(repo, pull, policy, revision, preview, role, decision, rationale, actor string) (Acceptance, error) {
	if repo == "" || pull == "" || policy == "" || revision == "" || preview == "" || actor == "" || !map[string]bool{"accepted": true, "rejected": true}[decision] || strings.TrimSpace(rationale) == "" {
		return Acceptance{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.findPolicy(policy)
	if e != nil {
		return Acceptance{}, e
	}
	required := false
	for _, r := range p.RequiredRoles {
		required = required || r == role
	}
	if !required {
		return Acceptance{}, ErrInvalid
	}
	a := Acceptance{ID: ident(), RepositoryID: repo, PullRequestID: pull, PolicyID: policy, Revision: revision, PreviewID: preview, Role: role, Decision: decision, Rationale: rationale, ActorID: actor, CreatedAt: s.now().UTC()}
	v := s.loadLedger(repo, pull)
	v.Acceptances = append(v.Acceptances, a)
	return a, write(s.ledgerPath(repo, pull), v)
}
func (s *Store) Except(repo, pull, policy, revision, reason, owner, followKind, followID, actor string, expires time.Time) (Exception, error) {
	if repo == "" || pull == "" || policy == "" || revision == "" || reason == "" || owner == "" || followID == "" || actor == "" || !map[string]bool{"issue": true, "proposal": true, "task": true}[followKind] || !expires.After(s.now()) {
		return Exception{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, e := s.findPolicy(policy); e != nil {
		return Exception{}, e
	}
	x := Exception{ID: ident(), RepositoryID: repo, PullRequestID: pull, PolicyID: policy, Revision: revision, Reason: reason, OwnerID: owner, FollowUpKind: followKind, FollowUpID: followID, CreatedByID: actor, ExpiresAt: expires.UTC(), CreatedAt: s.now().UTC()}
	v := s.loadLedger(repo, pull)
	v.Exceptions = append(v.Exceptions, x)
	return x, write(s.ledgerPath(repo, pull), v)
}
func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y || strings.HasSuffix(x, "/**") && strings.HasPrefix(y, strings.TrimSuffix(x, "**")) {
				return true
			}
		}
	}
	return false
}
func applies(p Policy, branch string, c Selector) bool {
	if !intersects(p.TargetBranches, []string{branch}) {
		return false
	}
	return len(p.Components)+len(p.Journeys)+len(p.Paths)+len(p.RiskClasses) == 0 || intersects(p.Components, c.Components) || intersects(p.Journeys, c.Journeys) || intersects(p.Paths, c.Paths) || intersects(p.RiskClasses, c.RiskClasses)
}
func (s *Store) Assess(repo, pull, revision, branch string, context Selector, policies []Policy, runs []interfacechecks.Run, uses []Usage) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.loadLedger(repo, pull)
	a := Assessment{Revision: revision, Requirements: []Requirement{}, ObsoleteUses: []Usage{}, ExpiringException: []Exception{}, Ready: true}
	applicable := []Policy{}
	for _, p := range policies {
		if applies(p, branch, context) {
			applicable = append(applicable, p)
		}
	}
	except := map[string]bool{}
	now := s.now()
	for _, x := range v.Exceptions {
		if x.Revision == revision && x.ExpiresAt.After(now) {
			except[x.PolicyID] = true
			if x.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
				a.ExpiringException = append(a.ExpiringException, x)
			}
		}
	}
	for _, u := range uses {
		if u.Obsolete && (len(context.Paths) == 0 || intersects([]string{u.Path}, context.Paths)) {
			a.ObsoleteUses = append(a.ObsoleteUses, u)
			a.Requirements = append(a.Requirements, Requirement{Kind: "obsolete_component", Detail: "obsolete component " + u.ComponentID + " is used at " + u.Path, Blocking: true})
		}
	}
	previewCurrent := false
	unresolved := false
	for _, r := range runs {
		if r.Current && r.Revision == revision {
			previewCurrent = true
			for _, c := range r.Cases {
				for _, d := range c.Differences {
					if d.Current && (d.Classification == "" || d.Classification == "regression") {
						unresolved = true
					}
				}
			}
		}
	}
	if len(applicable) > 0 && !previewCurrent {
		a.Requirements = append(a.Requirements, Requirement{Kind: "stale_preview", Detail: "a current interface preview is required", Blocking: true})
	}
	if len(applicable) > 0 && unresolved {
		a.Requirements = append(a.Requirements, Requirement{Kind: "unresolved_deviation", Detail: "current interface differences remain unresolved", Blocking: true})
	}
	for _, p := range applicable {
		a.AppliedPolicyIDs = append(a.AppliedPolicyIDs, p.ID)
		for _, role := range p.RequiredRoles {
			ok := false
			for i := len(v.Acceptances) - 1; i >= 0; i-- {
				x := v.Acceptances[i]
				if x.PolicyID == p.ID && x.Role == role && x.Revision == revision {
					ok = x.Decision == "accepted"
					break
				}
			}
			if !ok && !except[p.ID] {
				a.Requirements = append(a.Requirements, Requirement{PolicyID: p.ID, Role: role, Kind: "acceptance_required", Detail: "current " + role + " acceptance is required", Blocking: true})
			}
		}
	}
	for _, r := range a.Requirements {
		if r.Blocking {
			a.Ready = false
		}
	}
	return a, nil
}
func (s *Store) CreateWork(actor string, in WorkInput) (Work, error) {
	if actor == "" || in.SourceID == "" || in.AffectedRepository == "" || in.OwnerID == "" || in.Summary == "" || !validList(in.AcceptanceCriteria, true) || !map[string]bool{"migration": true, "repair": true}[in.Kind] || !map[string]bool{"design_system_change": true, "feedback": true, "regression": true}[in.SourceKind] || in.Kind == "migration" && (in.SystemID == "" || in.SystemVersion < 1) {
		return Work{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w := Work{ID: ident(), WorkInput: in, CreatedByID: actor, CreatedAt: s.now().UTC(), GrantsAuthority: false}
	return w, write(filepath.Join(s.root, "work", in.AffectedRepository, w.ID+".json"), w)
}
