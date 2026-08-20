// Package capabilityretirements owns acknowledged, inventory-bound capability retirement contracts.
package capabilityretirements

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
)

var ErrNotFound = errors.New("capability retirement plan not found")
var ErrInvalid = errors.New("invalid capability retirement plan")
var ErrConflict = errors.New("capability retirement plan conflict")
var ErrForbidden = errors.New("capability retirement owner action forbidden")

type Inventories interface {
	Get(repository, inventory string) (capabilityinventories.Inventory, error)
}
type Replacement struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Reference      string   `json:"reference"`
	Revision       string   `json:"revision"`
	MigrationGuide string   `json:"migration_guide"`
	OwnerIDs       []string `json:"owner_ids"`
}
type Audience struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	ConsumerIDs   []string `json:"consumer_ids"`
	OwnerIDs      []string `json:"owner_ids"`
	StopsWorking  string   `json:"stops_working"`
	MigrationPath string   `json:"migration_path"`
	Embargoed     bool     `json:"embargoed"`
}
type Stage struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	StartsAt      time.Time `json:"starts_at"`
	EndsAt        time.Time `json:"ends_at"`
	Compatibility string    `json:"compatibility"`
	EntryCriteria []string  `json:"entry_criteria"`
	ExitCriteria  []string  `json:"exit_criteria"`
	RollbackTo    string    `json:"rollback_to,omitempty"`
}
type CommunicationPolicy struct {
	OwnerID          string     `json:"owner_id"`
	Channels         []string   `json:"channels"`
	Cadence          string     `json:"cadence"`
	NoticePeriodDays int        `json:"notice_period_days"`
	EmbargoUntil     *time.Time `json:"embargo_until,omitempty"`
	Escalation       string     `json:"escalation"`
}
type ApprovalRequirement struct {
	OwnerID  string    `json:"owner_id"`
	Scope    string    `json:"scope"`
	Deadline time.Time `json:"deadline"`
}
type Commitment struct {
	ID        string     `json:"id"`
	Reference string     `json:"reference"`
	Revision  string     `json:"revision"`
	OwnerID   string     `json:"owner_id"`
	Guarantee string     `json:"guarantee"`
	Until     *time.Time `json:"until,omitempty"`
	Conflicts bool       `json:"conflicts"`
}
type Exception struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"`
	Rationale string    `json:"rationale"`
	OwnerID   string    `json:"owner_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Decision  string    `json:"decision,omitempty"`
}
type Input struct {
	InventoryID       string                `json:"inventory_id"`
	InventoryVersion  int64                 `json:"inventory_version"`
	Rationale         string                `json:"rationale"`
	Replacements      []Replacement         `json:"replacements"`
	Audiences         []Audience            `json:"audiences"`
	Stages            []Stage               `json:"stages"`
	RemovalDeadline   time.Time             `json:"removal_deadline"`
	SuccessCriteria   []string              `json:"success_criteria"`
	RollbackCriteria  []string              `json:"rollback_criteria"`
	Communication     CommunicationPolicy   `json:"communication_policy"`
	RequiredApprovals []ApprovalRequirement `json:"required_approvals"`
	Assumptions       []string              `json:"assumptions"`
	Commitments       []Commitment          `json:"commitments"`
	Exceptions        []Exception           `json:"exceptions"`
}
type Assessment struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Body              string    `json:"body"`
	EvidenceReference string    `json:"evidence_reference"`
	AudienceIDs       []string  `json:"audience_ids"`
	AuthorID          string    `json:"author_id"`
	AuthorKind        string    `json:"author_kind"`
	CreatedAt         time.Time `json:"created_at"`
}
type Approval struct {
	OwnerID   string     `json:"owner_id"`
	Scope     string     `json:"scope"`
	Deadline  time.Time  `json:"deadline"`
	Decision  string     `json:"decision,omitempty"`
	Rationale string     `json:"rationale,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
}
type PolicyDecision struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Subject   string    `json:"subject"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	ExpiresAt time.Time `json:"expires_at"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind                 string `json:"kind"`
	Subject              string `json:"subject"`
	Detail               string `json:"detail"`
	AttributedTo         string `json:"attributed_to,omitempty"`
	ResolvedByDecisionID string `json:"resolved_by_decision_id,omitempty"`
}
type Plan struct {
	ID              string           `json:"id"`
	RepositoryID    string           `json:"repository_id"`
	Input           Input            `json:"input"`
	CreatedByID     string           `json:"created_by_id"`
	CreatedAt       time.Time        `json:"created_at"`
	Assessments     []Assessment     `json:"assessments"`
	Approvals       []Approval       `json:"approvals"`
	PolicyDecisions []PolicyDecision `json:"policy_decisions"`
	Blockers        []Blocker        `json:"blockers"`
	Ready           bool             `json:"ready"`
	NonAuthority    []string         `json:"non_authority"`
}
type Store struct {
	root        string
	inventories Inventories
	mu          sync.Mutex
	now         func() time.Time
}

func New(root string, inventories Inventories) (*Store, error) {
	if strings.TrimSpace(root) == "" || inventories == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, inventories: inventories, now: time.Now}, e
}
func identifier() string                     { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func unique(v string, m map[string]bool) bool {
	v = strings.TrimSpace(v)
	if v == "" || m[v] {
		return false
	}
	m[v] = true
	return true
}
func valid(in Input) bool {
	if in.InventoryID == "" || in.InventoryVersion < 1 || in.Rationale == "" || len(in.Replacements) == 0 || len(in.Audiences) == 0 || len(in.Stages) == 0 || in.RemovalDeadline.IsZero() || len(in.SuccessCriteria) == 0 || len(in.RollbackCriteria) == 0 || in.Communication.OwnerID == "" || len(in.Communication.Channels) == 0 || in.Communication.NoticePeriodDays < 0 || len(in.RequiredApprovals) == 0 {
		return false
	}
	ids := map[string]bool{}
	for _, x := range in.Replacements {
		if !unique(x.ID, ids) || !allowed(x.Kind, "interface", "package", "service", "workflow", "documentation", "none") || x.Reference == "" || x.Revision == "" || x.MigrationGuide == "" || len(x.OwnerIDs) == 0 {
			return false
		}
	}
	ids = map[string]bool{}
	for _, x := range in.Audiences {
		if !unique(x.ID, ids) || x.Name == "" || len(x.OwnerIDs) == 0 || x.StopsWorking == "" || x.MigrationPath == "" {
			return false
		}
	}
	ids = map[string]bool{}
	for i, x := range in.Stages {
		if !unique(x.ID, ids) || x.Name == "" || x.StartsAt.IsZero() || x.EndsAt.IsZero() || !x.EndsAt.After(x.StartsAt) || x.Compatibility == "" || len(x.ExitCriteria) == 0 || (i > 0 && x.StartsAt.Before(in.Stages[i-1].EndsAt)) {
			return false
		}
	}
	seen := map[string]bool{}
	for _, x := range in.RequiredApprovals {
		if x.OwnerID == "" || x.Scope == "" || x.Deadline.IsZero() || seen[x.OwnerID+"\x00"+x.Scope] {
			return false
		}
		seen[x.OwnerID+"\x00"+x.Scope] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Plan, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	inv, e := s.inventories.Get(repo, in.InventoryID)
	if e != nil || in.InventoryVersion > inv.CurrentVersion {
		return Plan{}, ErrInvalid
	}
	found := false
	for _, v := range inv.Versions {
		found = found || v.Number == in.InventoryVersion
	}
	if !found {
		return Plan{}, ErrInvalid
	}
	now := s.now().UTC()
	p := Plan{ID: identifier(), RepositoryID: repo, Input: in, CreatedByID: actor, CreatedAt: now, Assessments: []Assessment{}, PolicyDecisions: []PolicyDecision{}, NonAuthority: []string{"repository write", "consumer access", "approval impersonation", "merge", "release", "deployment", "credential", "environment", "operational authority"}}
	for _, r := range in.RequiredApprovals {
		p.Approvals = append(p.Approvals, Approval{OwnerID: r.OwnerID, Scope: r.Scope, Deadline: r.Deadline})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p = s.derive(p, inv, now)
	return p, s.save(p)
}
func (s *Store) derive(p Plan, inv capabilityinventories.Inventory, now time.Time) Plan {
	b := []Blocker{}
	add := func(k, s, d, a string) { b = append(b, Blocker{Kind: k, Subject: s, Detail: d, AttributedTo: a}) }
	if inv.CurrentVersion != p.Input.InventoryVersion {
		add("changed_usage", "inventory", "capability inventory has changed since this plan was opened", inv.Versions[len(inv.Versions)-1].AuthorID)
	}
	for _, g := range inv.Gaps {
		add("inventory_"+g.Kind, g.Subject, g.Detail, g.AttributedTo)
	}
	for _, a := range p.Input.Audiences {
		if a.Embargoed {
			add("embargoed_dependency", a.ID, "affected dependency is embargoed", a.OwnerIDs[0])
		}
	}
	for _, c := range p.Input.Commitments {
		if c.Conflicts || (c.Until != nil && c.Until.After(p.Input.RemovalDeadline)) {
			add("conflicting_commitment", c.ID, c.Guarantee, c.OwnerID)
		}
	}
	for _, x := range p.Input.Exceptions {
		if x.Decision == "" || !x.ExpiresAt.After(now) {
			add("exception_pending", x.ID, x.Rationale, x.OwnerID)
		}
	}
	for _, x := range p.Assessments {
		if x.Kind == "challenge" {
			add("cited_challenge", x.ID, x.Body+" ("+x.EvidenceReference+")", x.AuthorID)
		}
	}
	for _, a := range p.Approvals {
		if a.Decision == "rejected" {
			add("owner_rejected", a.Scope, a.Rationale, a.OwnerID)
		} else if a.Decision == "changes_requested" {
			add("owner_changes_requested", a.Scope, a.Rationale, a.OwnerID)
		} else if a.Decision == "" {
			k := "owner_approval_pending"
			if !a.Deadline.After(now) {
				k = "owner_unresponsive"
			}
			add(k, a.Scope, "required owner has not acknowledged the retirement contract", a.OwnerID)
		}
	}
	for i := range b {
		for _, d := range p.PolicyDecisions {
			if d.ExpiresAt.After(now) && d.Kind == b[i].Kind && d.Subject == b[i].Subject {
				b[i].ResolvedByDecisionID = d.ID
				break
			}
		}
	}
	p.Blockers = b
	p.Ready = true
	for _, x := range b {
		if x.ResolvedByDecisionID == "" {
			p.Ready = false
		}
	}
	return p
}
func (s *Store) save(p Plan) error {
	if e := os.MkdirAll(filepath.Dir(s.path(p.RepositoryID, p.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(p.RepositoryID, p.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, id string) (Plan, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Plan{}, ErrNotFound
	}
	var p Plan
	if e != nil || json.Unmarshal(b, &p) != nil || p.RepositoryID != repo || p.ID != id {
		return Plan{}, ErrNotFound
	}
	inv, e := s.inventories.Get(repo, p.Input.InventoryID)
	if e != nil {
		return Plan{}, ErrInvalid
	}
	return s.derive(p, inv, s.now().UTC()), nil
}
func (s *Store) Get(repo, id string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Plan{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Plan{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			p, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Assess(repo, id, actor, authorKind, kind, body, evidence string, audiences []string) (Plan, error) {
	if actor == "" || !allowed(authorKind, "human", "read_only_agent") || !allowed(kind, "impact", "challenge", "assumption", "alternative") || body == "" || evidence == "" {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, id)
	if e != nil {
		return Plan{}, e
	}
	p.Assessments = append(p.Assessments, Assessment{identifier(), kind, body, evidence, audiences, actor, authorKind, s.now().UTC()})
	return p, s.save(p)
}
func (s *Store) DecideApproval(repo, id, actor, scope, decision, rationale string) (Plan, error) {
	if !allowed(decision, "approved", "changes_requested", "rejected") || rationale == "" {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, id)
	if e != nil {
		return Plan{}, e
	}
	found := false
	now := s.now().UTC()
	for i := range p.Approvals {
		if p.Approvals[i].OwnerID == actor && p.Approvals[i].Scope == scope {
			p.Approvals[i].Decision = decision
			p.Approvals[i].Rationale = rationale
			p.Approvals[i].DecidedAt = &now
			found = true
		}
	}
	if !found {
		return Plan{}, ErrForbidden
	}
	inv, _ := s.inventories.Get(repo, p.Input.InventoryID)
	p = s.derive(p, inv, now)
	return p, s.save(p)
}
func (s *Store) AddPolicyDecision(repo, id, actor, kind, subject, decision, rationale string, expires time.Time) (Plan, error) {
	if actor == "" || kind == "" || subject == "" || !allowed(decision, "proceed", "extend", "pause", "exclude") || rationale == "" || !expires.After(s.now()) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, id)
	if e != nil {
		return Plan{}, e
	}
	owner := false
	for _, a := range p.Approvals {
		owner = owner || a.OwnerID == actor
	}
	if !owner {
		return Plan{}, ErrForbidden
	}
	d := PolicyDecision{identifier(), kind, subject, decision, rationale, expires, actor, s.now().UTC()}
	p.PolicyDecisions = append(p.PolicyDecisions, d)
	inv, _ := s.inventories.Get(repo, p.Input.InventoryID)
	p = s.derive(p, inv, s.now().UTC())
	return p, s.save(p)
}
