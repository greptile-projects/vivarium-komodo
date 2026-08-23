// Package workflowdefinitions owns repository-reviewed collaboration workflow definitions.
package workflowdefinitions

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

var ErrNotFound = errors.New("workflow definition not found")
var ErrInvalid = errors.New("invalid workflow definition")
var ErrConflict = errors.New("workflow definition conflict")
var ErrBlocked = errors.New("workflow activation blocked")

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}
type Trigger struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`
	Event         string            `json:"event"`
	Conditions    []string          `json:"conditions"`
	InputMappings map[string]string `json:"input_mappings"`
}
type Retry struct {
	MaximumAttempts int `json:"maximum_attempts"`
	BackoffSeconds  int `json:"backoff_seconds"`
}
type Invocation struct {
	Kind         string   `json:"kind"`
	Reference    string   `json:"reference"`
	Revision     string   `json:"revision"`
	Accessible   bool     `json:"accessible"`
	OwnerIDs     []string `json:"owner_ids"`
	Capabilities []string `json:"capabilities"`
	Emits        []string `json:"emits"`
	ActionClass  string   `json:"action_class,omitempty"`
}
type ActionRequirement struct {
	ActionClass        string   `json:"action_class"`
	OwnerIDs           []string `json:"owner_ids"`
	MinimumApprovals   int      `json:"minimum_approvals"`
	SeparateFromAuthor bool     `json:"separate_from_author"`
	ApprovalTTLSeconds int      `json:"approval_ttl_seconds"`
}
type SimulationCase struct {
	ID                  string   `json:"id"`
	Event               string   `json:"event"`
	ExpectedEffects     []string `json:"expected_effects"`
	ExpectedPermissions []string `json:"expected_permissions"`
	MaximumCost         float64  `json:"maximum_cost"`
}
type Governance struct {
	RequiredReviewerIDs []string            `json:"required_reviewer_ids"`
	RequiredOwnerIDs    []string            `json:"required_owner_ids"`
	SimulationCases     []SimulationCase    `json:"simulation_cases"`
	ActionRequirements  []ActionRequirement `json:"action_requirements"`
}
type Step struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Needs              []string          `json:"needs"`
	Conditions         []string          `json:"conditions"`
	Inputs             map[string]string `json:"inputs"`
	Outputs            []Field           `json:"outputs"`
	Invocation         Invocation        `json:"invocation"`
	Retry              Retry             `json:"retry"`
	TimeoutSeconds     int               `json:"timeout_seconds"`
	MaximumCost        float64           `json:"maximum_cost"`
	Optional           bool              `json:"optional"`
	CompletionCriteria []string          `json:"completion_criteria"`
}
type Policy struct {
	ID         string   `json:"id"`
	Effect     string   `json:"effect"`
	Capability string   `json:"capability"`
	OwnerIDs   []string `json:"owner_ids"`
}
type Input struct {
	Name               string     `json:"name"`
	Outcome            string     `json:"outcome"`
	RepositoryRevision string     `json:"repository_revision"`
	DefinitionPath     string     `json:"definition_path"`
	Triggers           []Trigger  `json:"triggers"`
	Inputs             []Field    `json:"inputs"`
	Steps              []Step     `json:"steps"`
	Outputs            []Field    `json:"outputs"`
	MaximumCost        float64    `json:"maximum_cost"`
	Currency           string     `json:"currency"`
	MaximumConcurrency int        `json:"maximum_concurrency,omitempty"`
	RateLimit          RateLimit  `json:"rate_limit,omitempty"`
	OwnerIDs           []string   `json:"owner_ids"`
	CompletionCriteria []string   `json:"completion_criteria"`
	Policies           []Policy   `json:"policies"`
	Governance         Governance `json:"governance,omitempty"`
	ChangeReason       string     `json:"change_reason"`
}
type RateLimit struct {
	MaximumInvocations int `json:"maximum_invocations"`
	WindowSeconds      int `json:"window_seconds"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Diagnostic struct {
	Kind         string   `json:"kind"`
	Subject      string   `json:"subject"`
	Detail       string   `json:"detail"`
	AttributedTo []string `json:"attributed_to"`
	Blocking     bool     `json:"blocking"`
}
type Authority struct {
	Capabilities    []string `json:"capabilities"`
	Resources       []string `json:"resources"`
	OwnerIDs        []string `json:"owner_ids"`
	GrantsAuthority bool     `json:"grants_authority"`
}
type Activation struct {
	Version     int64     `json:"version"`
	ActivatedBy string    `json:"activated_by"`
	ActivatedAt time.Time `json:"activated_at"`
}
type CandidateDecision struct {
	Version   int64     `json:"version"`
	Kind      string    `json:"kind"`
	Subject   string    `json:"subject"`
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type SimulationResult struct {
	Version         int64     `json:"version"`
	CaseID          string    `json:"case_id"`
	ActorID         string    `json:"actor_id"`
	Passed          bool      `json:"passed"`
	Effects         []string  `json:"effects"`
	Permissions     []string  `json:"permissions"`
	Cost            float64   `json:"cost"`
	PolicyConflicts []string  `json:"policy_conflicts"`
	CreatedAt       time.Time `json:"created_at"`
}
type Exception struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	Scope     string    `json:"scope"`
	OwnerID   string    `json:"owner_id"`
	Rationale string    `json:"rationale"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
type Workflow struct {
	ID                 string              `json:"id"`
	RepositoryID       string              `json:"repository_id"`
	CurrentVersion     int64               `json:"current_version"`
	Versions           []Version           `json:"versions"`
	EventSubscriptions []string            `json:"event_subscriptions"`
	EffectiveAuthority Authority           `json:"effective_authority"`
	Diagnostics        []Diagnostic        `json:"diagnostics"`
	Activation         *Activation         `json:"activation,omitempty"`
	CandidateDecisions []CandidateDecision `json:"candidate_decisions"`
	SimulationResults  []SimulationResult  `json:"simulation_results"`
	Exceptions         []Exception         `json:"exceptions"`
	DisabledAt         *time.Time          `json:"disabled_at,omitempty"`
	DisabledBy         string              `json:"disabled_by,omitempty"`
	DisableReason      string              `json:"disable_reason,omitempty"`
	State              string              `json:"state"`
}
type Catalog struct {
	Items []Workflow `json:"items"`
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
func identifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Outcome) == "" || in.RepositoryRevision == "" || in.DefinitionPath == "" || in.ChangeReason == "" || len(in.Triggers) == 0 || len(in.Steps) == 0 || len(in.OwnerIDs) == 0 || len(in.CompletionCriteria) == 0 || in.MaximumCost < 0 || in.Currency == "" {
		return false
	}
	if in.MaximumConcurrency < 0 || in.RateLimit.MaximumInvocations < 0 || in.RateLimit.WindowSeconds < 0 || (in.RateLimit.MaximumInvocations == 0) != (in.RateLimit.WindowSeconds == 0) {
		return false
	}
	for _, f := range append(append([]Field{}, in.Inputs...), in.Outputs...) {
		if !identifier(f.Name) || (f.Type != "string" && f.Type != "number" && f.Type != "boolean" && f.Type != "object" && f.Type != "array") {
			return false
		}
	}
	for _, t := range in.Triggers {
		if !identifier(t.ID) || t.Event == "" || (t.Type != "repository_event" && t.Type != "schedule" && t.Type != "manual" && t.Type != "webhook") {
			return false
		}
	}
	for _, s := range in.Steps {
		if !identifier(s.ID) || s.Name == "" || (s.Invocation.Kind != "platform_action" && s.Invocation.Kind != "component" && s.Invocation.Kind != "approved_agent" && s.Invocation.Kind != "manual") || s.Invocation.Reference == "" || s.Invocation.Revision == "" || s.TimeoutSeconds < 1 || s.Retry.MaximumAttempts < 1 || s.MaximumCost < 0 || len(s.CompletionCriteria) == 0 {
			return false
		}
	}
	for _, c := range in.Governance.SimulationCases {
		if !identifier(c.ID) || c.Event == "" || len(c.ExpectedEffects) == 0 || c.MaximumCost < 0 {
			return false
		}
	}
	for _, q := range in.Governance.ActionRequirements {
		if !identifier(q.ActionClass) || len(q.OwnerIDs) == 0 || q.MinimumApprovals < 1 || q.MinimumApprovals > len(q.OwnerIDs) || q.ApprovalTTLSeconds < 1 {
			return false
		}
	}
	return true
}
func derive(v Version) ([]string, Authority, []Diagnostic) {
	events, caps, resources := []string{}, []string{}, []string{}
	ds := []Diagnostic{}
	add := func(k, s, d string, owners []string) { ds = append(ds, Diagnostic{k, s, d, owners, true}) }
	for _, t := range v.Triggers {
		events = append(events, t.Event)
	}
	steps := map[string]Step{}
	for _, s := range v.Steps {
		if _, ok := steps[s.ID]; ok {
			add("invalid_graph", s.ID, "step id is duplicated", v.OwnerIDs)
		}
		steps[s.ID] = s
		caps = append(caps, s.Invocation.Capabilities...)
		resources = append(resources, s.Invocation.Kind+":"+s.Invocation.Reference+"@"+s.Invocation.Revision)
		if !s.Invocation.Accessible {
			add("inaccessible_resource", s.ID, "invoked resource is not accessible", s.Invocation.OwnerIDs)
		}
		if len(s.Invocation.OwnerIDs) == 0 {
			add("missing_owner", s.ID, "invoked resource has no accountable owner", v.OwnerIDs)
		}
		for _, e := range s.Invocation.Emits {
			for _, t := range v.Triggers {
				if e == t.Event {
					add("trigger_loop", s.ID, "step emits subscribed event "+e, s.Invocation.OwnerIDs)
				}
			}
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if visiting[id] {
			add("invalid_graph", id, "dependency cycle detected", v.OwnerIDs)
			return
		}
		if done[id] {
			return
		}
		visiting[id] = true
		s, ok := steps[id]
		if ok {
			for _, n := range s.Needs {
				if _, exists := steps[n]; !exists {
					add("invalid_graph", id, "missing dependency "+n, v.OwnerIDs)
				} else {
					walk(n)
				}
			}
		}
		visiting[id] = false
		done[id] = true
	}
	for id := range steps {
		walk(id)
	}
	for _, p := range v.Policies {
		if p.Effect != "allow" && p.Effect != "deny" {
			add("conflicting_policy", p.ID, "policy effect must be allow or deny", p.OwnerIDs)
			continue
		}
		if p.Effect == "deny" {
			for _, c := range caps {
				if c == p.Capability {
					add("conflicting_policy", p.ID, "policy denies requested capability "+c, p.OwnerIDs)
				}
			}
		}
	}
	sort.Strings(events)
	events = unique(events)
	sort.Strings(caps)
	caps = unique(caps)
	sort.Strings(resources)
	resources = unique(resources)
	owners := append([]string{}, v.OwnerIDs...)
	sort.Strings(owners)
	return events, Authority{caps, resources, owners, false}, ds
}
func unique(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		if len(out) == 0 || out[len(out)-1] != x {
			out = append(out, x)
		}
	}
	return out
}
func newID() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Workflow) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func refresh(x *Workflow) {
	v := x.Versions[len(x.Versions)-1]
	x.EventSubscriptions, x.EffectiveAuthority, x.Diagnostics = derive(v)
	if x.Activation != nil && x.Activation.Version != x.CurrentVersion {
		x.State = "draft"
		x.Activation = nil
	} else if x.Activation != nil && x.DisabledAt == nil {
		x.State = "active"
	} else {
		x.State = "draft"
	}
}
func (s *Store) Create(repo, actor string, in Input) (Workflow, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Workflow{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Version{1, in, actor, s.now().UTC()}
	x := Workflow{ID: newID(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{v}}
	refresh(&x)
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (Workflow, error) {
	if actor == "" || !valid(in) {
		return Workflow{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return Workflow{}, e
	}
	if x.CurrentVersion != expected {
		return Workflow{}, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{x.CurrentVersion, in, actor, s.now().UTC()})
	s.containExecutions(repo, id, actor, "workflow_authority_changed")
	refresh(&x)
	return x, s.save(x)
}
func (s *Store) Activate(repo, id, actor string, version int64) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return Workflow{}, e
	}
	if version != x.CurrentVersion {
		return Workflow{}, ErrConflict
	}
	if !contains(x.Versions[len(x.Versions)-1].OwnerIDs, actor) {
		return Workflow{}, ErrConflict
	}
	if len(x.Diagnostics) > 0 {
		return x, ErrBlocked
	}
	v := x.Versions[len(x.Versions)-1]
	for _, c := range v.Governance.SimulationCases {
		ok := false
		for _, r := range x.SimulationResults {
			if r.Version == version && r.CaseID == c.ID && r.Passed && len(r.PolicyConflicts) == 0 {
				ok = true
			}
		}
		if !ok {
			return x, ErrBlocked
		}
	}
	for _, id := range v.Governance.RequiredReviewerIDs {
		ok := false
		for _, d := range x.CandidateDecisions {
			if d.Version == version && d.Kind == "review" && d.ActorID == id && d.Decision == "approved" && (!contains(v.OwnerIDs, d.ActorID) || !contains(v.Governance.RequiredOwnerIDs, d.ActorID)) {
				ok = true
			}
		}
		if !ok {
			return x, ErrBlocked
		}
	}
	for _, id := range v.Governance.RequiredOwnerIDs {
		ok := false
		for _, d := range x.CandidateDecisions {
			if d.Version == version && d.Kind == "owner_acknowledgement" && d.ActorID == id && d.Decision == "acknowledged" {
				ok = true
			}
		}
		if !ok {
			return x, ErrBlocked
		}
	}
	x.Activation = &Activation{version, actor, s.now().UTC()}
	x.DisabledAt = nil
	x.DisabledBy = ""
	x.DisableReason = ""
	refresh(&x)
	return x, s.save(x)
}

func (s *Store) RecordCandidateDecision(repo, id, actor string, version int64, kind, decision, rationale string) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if version != x.CurrentVersion || rationale == "" || !contains([]string{"review", "owner_acknowledgement"}, kind) {
		return x, ErrInvalid
	}
	v := x.Versions[len(x.Versions)-1]
	allowed := kind == "review" && contains(v.Governance.RequiredReviewerIDs, actor) || kind == "owner_acknowledgement" && contains(v.Governance.RequiredOwnerIDs, actor)
	if !allowed || kind == "review" && decision != "approved" && decision != "rejected" || kind == "owner_acknowledgement" && decision != "acknowledged" && decision != "rejected" {
		return x, ErrConflict
	}
	x.CandidateDecisions = append(x.CandidateDecisions, CandidateDecision{version, kind, "workflow_definition", actor, decision, rationale, s.now().UTC()})
	return x, s.save(x)
}
func (s *Store) RecordSimulation(repo, id, actor string, in SimulationResult) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if in.Version != x.CurrentVersion || in.CaseID == "" || in.Cost < 0 {
		return x, ErrInvalid
	}
	v := x.Versions[len(x.Versions)-1]
	found := false
	for _, c := range v.Governance.SimulationCases {
		if c.ID == in.CaseID && c.MaximumCost >= in.Cost {
			found = true
		}
	}
	if !found {
		return x, ErrInvalid
	}
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	x.SimulationResults = append(x.SimulationResults, in)
	return x, s.save(x)
}
func (s *Store) AddException(repo, id, actor string, in Exception) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if in.Version != x.CurrentVersion || in.Scope == "" || in.Rationale == "" || !in.ExpiresAt.After(s.now().UTC()) || !contains(x.Versions[len(x.Versions)-1].OwnerIDs, actor) {
		return x, ErrInvalid
	}
	in.ID = newID()
	in.OwnerID = actor
	in.CreatedAt = s.now().UTC()
	x.Exceptions = append(x.Exceptions, in)
	return x, s.save(x)
}
func (s *Store) Disable(repo, id, actor, reason string) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if reason == "" || !contains(x.Versions[len(x.Versions)-1].OwnerIDs, actor) {
		return x, ErrConflict
	}
	now := s.now().UTC()
	x.DisabledAt = &now
	x.DisabledBy = actor
	x.DisableReason = reason
	s.containExecutions(repo, id, actor, "workflow_emergency_disabled")
	refresh(&x)
	return x, s.save(x)
}

func (s *Store) containExecutions(repo, workflow, actor, reason string) {
	xs, _ := s.listExecutions(repo, workflow)
	for _, x := range xs {
		if executionTerminal(x.State) || x.State == "paused" {
			continue
		}
		now := s.now().UTC()
		x.State = "paused"
		x.Revision++
		x.UpdatedAt = now
		x.Blockers = append(x.Blockers, reason)
		for i := range x.Steps {
			if x.Steps[i].Credential != nil && x.Steps[i].Credential.RevokedAt == nil {
				x.Steps[i].Credential.RevokedAt = &now
			}
			if x.Steps[i].State == "running" {
				x.Steps[i].State = "paused"
			}
		}
		x.event("effects_contained", actor, "", reason, now)
		_ = s.saveExecution(x)
	}
}
func (s *Store) Rollback(repo, id, actor string, version int64, reason string) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if version < 1 || version >= x.CurrentVersion || reason == "" || !contains(x.Versions[len(x.Versions)-1].OwnerIDs, actor) {
		return x, ErrConflict
	}
	prior := x.Versions[version-1]
	x.CurrentVersion++
	prior.Number = x.CurrentVersion
	prior.AuthorID = actor
	prior.PublishedAt = s.now().UTC()
	prior.ChangeReason = reason
	x.Versions = append(x.Versions, prior)
	x.Activation = nil
	x.DisabledAt = nil
	refresh(&x)
	return x, s.save(x)
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) Get(repo, id string) (Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	return Catalog{xs}, e
}
func (s *Store) read(repo, id string) (Workflow, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Workflow{}, ErrNotFound
	}
	var x Workflow
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Workflow{}, ErrNotFound
	}
	refresh(&x)
	return x, nil
}
func (s *Store) list(repo string) ([]Workflow, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Workflow{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Workflow{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].PublishedAt.After(out[j].Versions[len(out[j].Versions)-1].PublishedAt)
	})
	return out, nil
}
