// Package runbooks owns immutable, reviewable operational procedures.
package runbooks

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

var ErrNotFound = errors.New("runbook not found")
var ErrInvalid = errors.New("invalid runbook")
var ErrConflict = errors.New("runbook version conflict")

type Scope struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	OwnerID    string `json:"owner_id"`
}
type Precondition struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
	OwnerID     string `json:"owner_id"`
	Assumption  string `json:"assumption,omitempty"`
	Safe        bool   `json:"safe"`
}
type Reference struct {
	Kind          string `json:"kind"`
	ResourceID    string `json:"resource_id"`
	Revision      string `json:"revision"`
	Detail        string `json:"detail"`
	Accessible    bool   `json:"accessible"`
	Reviewed      bool   `json:"reviewed"`
	SecretBearing bool   `json:"secret_bearing"`
	OwnerID       string `json:"owner_id"`
}
type Decision struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	HumanRequired bool     `json:"human_required"`
	OwnerID       string   `json:"owner_id"`
}
type Step struct {
	ID                string      `json:"id"`
	Kind              string      `json:"kind"`
	Title             string      `json:"title"`
	Purpose           string      `json:"purpose"`
	Preconditions     []string    `json:"precondition_ids"`
	References        []Reference `json:"references"`
	ExpectedEvidence  []string    `json:"expected_evidence"`
	Decision          *Decision   `json:"decision,omitempty"`
	RequiredAuthority []string    `json:"required_authority"`
	OwnerIDs          []string    `json:"owner_ids"`
	RequiredSkills    []string    `json:"required_skills"`
	DependsOn         []string    `json:"depends_on"`
	RollbackCriteria  []string    `json:"rollback_criteria"`
}
type Escalation struct {
	Condition      string   `json:"condition"`
	OwnerID        string   `json:"owner_id"`
	TeamID         string   `json:"team_id,omitempty"`
	RequiredSkills []string `json:"required_skills"`
	AudienceIDs    []string `json:"audience_ids"`
	Action         string   `json:"action"`
}
type PolicyReference struct {
	Kind        string `json:"kind"`
	ResourceID  string `json:"resource_id"`
	Revision    string `json:"revision"`
	Accessible  bool   `json:"accessible"`
	Conflicting bool   `json:"conflicting"`
	OwnerID     string `json:"owner_id"`
}
type Input struct {
	Name             string            `json:"name"`
	Purpose          string            `json:"purpose"`
	Scope            Scope             `json:"scope"`
	Preconditions    []Precondition    `json:"preconditions"`
	Steps            []Step            `json:"steps"`
	RollbackCriteria []string          `json:"rollback_criteria"`
	OwnerIDs         []string          `json:"owner_ids"`
	RequiredSkills   []string          `json:"required_skills"`
	EscalationPaths  []Escalation      `json:"escalation_paths"`
	PolicyReferences []PolicyReference `json:"policy_references"`
	ChangeReason     string            `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Finding struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to,omitempty"`
}
type AuthorityPreview struct {
	StepID                string   `json:"step_id"`
	Inspects              []string `json:"inspects"`
	Changes               []string `json:"changes"`
	RequiresHumanJudgment bool     `json:"requires_human_judgment"`
	RequiredAuthority     []string `json:"required_authority"`
	Granted               bool     `json:"granted"`
}
type Runbook struct {
	ID               string             `json:"id"`
	RepositoryID     string             `json:"repository_id"`
	CurrentVersion   int64              `json:"current_version"`
	Versions         []Version          `json:"versions"`
	Findings         []Finding          `json:"findings"`
	AuthorityPreview []AuthorityPreview `json:"authority_preview"`
	NonAuthority     []string           `json:"non_authority"`
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
func clean(s string) bool { return strings.TrimSpace(s) != "" }
func list(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func one(s string, xs ...string) bool {
	for _, x := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func valid(in Input) bool {
	if !clean(in.Name) || !clean(in.Purpose) || !one(in.Scope.Kind, "service", "environment", "dependency", "signal") || !clean(in.Scope.ResourceID) || !clean(in.Scope.Revision) || !clean(in.Scope.OwnerID) || !list(in.OwnerIDs, true) || !list(in.RequiredSkills, true) || !list(in.RollbackCriteria, true) || !clean(in.ChangeReason) || len(in.Preconditions) == 0 || len(in.Steps) == 0 || len(in.EscalationPaths) == 0 {
		return false
	}
	pre := map[string]bool{}
	for _, p := range in.Preconditions {
		if !clean(p.ID) || pre[p.ID] || !clean(p.Description) || !clean(p.Evidence) || !clean(p.OwnerID) {
			return false
		}
		pre[p.ID] = true
	}
	steps := map[string]bool{}
	for _, s := range in.Steps {
		if !clean(s.ID) || steps[s.ID] || !one(s.Kind, "diagnostic", "action", "decision") || !clean(s.Title) || !clean(s.Purpose) || !list(s.Preconditions, true) || !list(s.ExpectedEvidence, true) || !list(s.RequiredAuthority, false) || !list(s.OwnerIDs, false) || !list(s.RequiredSkills, true) || !list(s.RollbackCriteria, false) {
			return false
		}
		for _, p := range s.Preconditions {
			if !pre[p] {
				return false
			}
		}
		for _, r := range s.References {
			if !one(r.Kind, "command", "workflow_component", "documentation", "agent") || !clean(r.ResourceID) || !clean(r.Revision) || !clean(r.Detail) || !clean(r.OwnerID) {
				return false
			}
		}
		if s.Kind == "decision" && (s.Decision == nil || !clean(s.Decision.Question) || !list(s.Decision.Options, true)) {
			return false
		}
		steps[s.ID] = true
	}
	for _, s := range in.Steps {
		for _, d := range s.DependsOn {
			if !steps[d] || d == s.ID {
				return false
			}
		}
	}
	for _, e := range in.EscalationPaths {
		if !clean(e.Condition) || !clean(e.OwnerID) || !list(e.RequiredSkills, true) || !list(e.AudienceIDs, true) || !clean(e.Action) {
			return false
		}
	}
	for _, p := range in.PolicyReferences {
		if !clean(p.Kind) || !clean(p.ResourceID) || !clean(p.Revision) || !clean(p.OwnerID) {
			return false
		}
	}
	return true
}
func derive(v Version) ([]Finding, []AuthorityPreview) {
	f := []Finding{}
	add := func(k, s, d, a string) { f = append(f, Finding{k, s, d, a}) }
	previews := []AuthorityPreview{}
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "runbook has no accountable owner", v.AuthorID)
	}
	for _, p := range v.Preconditions {
		if !p.Safe {
			add("unsafe_assumption", p.ID, p.Assumption, p.OwnerID)
		}
	}
	for _, s := range v.Steps {
		if len(s.OwnerIDs) == 0 {
			add("missing_owner", s.ID, "step has no accountable owner", v.Scope.OwnerID)
		}
		inspect, change := []string{}, []string{}
		human := s.Kind == "decision" && s.Decision != nil && s.Decision.HumanRequired
		for _, r := range s.References {
			label := r.Kind + ":" + r.ResourceID + "@" + r.Revision
			if s.Kind == "diagnostic" {
				inspect = append(inspect, label)
			} else {
				change = append(change, label)
			}
			if !r.Accessible {
				add("inaccessible_resource", s.ID+":"+r.ResourceID, "referenced resource cannot be inspected", r.OwnerID)
			}
			if !r.Reviewed {
				add("unreviewed_reference", s.ID+":"+r.ResourceID, "referenced operational input is not reviewed", r.OwnerID)
			}
			if r.SecretBearing {
				add("secret_bearing_input", s.ID+":"+r.ResourceID, "step input is declared secret-bearing and must not be embedded", r.OwnerID)
			}
		}
		previews = append(previews, AuthorityPreview{s.ID, inspect, change, human, append([]string(nil), s.RequiredAuthority...), false})
	}
	for _, p := range v.PolicyReferences {
		if !p.Accessible {
			add("inaccessible_policy", p.Kind+":"+p.ResourceID, "governing policy cannot be inspected", p.OwnerID)
		}
		if p.Conflicting {
			add("conflicting_policy", p.Kind+":"+p.ResourceID, "governing policy conflicts with this procedure", p.OwnerID)
		}
	}
	return f, previews
}
func id() string                             { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Runbook) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(p, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Runbook, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Runbook{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Version{1, in, actor, s.now().UTC()}
	f, a := derive(v)
	x := Runbook{id(), repo, 1, []Version{v}, f, a, []string{"Runbooks and authority previews grant no repository, secret, workflow, agent, communication, incident, deployment, environment, or operational authority."}}
	return x, s.save(x)
}
func (s *Store) Revise(repo, rid, actor string, expected int64, in Input) (Runbook, error) {
	if actor == "" || !valid(in) {
		return Runbook{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, rid)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return x, ErrConflict
	}
	x.CurrentVersion++
	v := Version{x.CurrentVersion, in, actor, s.now().UTC()}
	x.Versions = append(x.Versions, v)
	x.Findings, x.AuthorityPreview = derive(v)
	return x, s.save(x)
}
func (s *Store) read(repo, rid string) (Runbook, error) {
	b, e := os.ReadFile(s.path(repo, rid))
	if errors.Is(e, fs.ErrNotExist) {
		return Runbook{}, ErrNotFound
	}
	var x Runbook
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Get(repo, rid string) (Runbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, rid)
}
func (s *Store) List(repo string) ([]Runbook, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Runbook{}
	for _, p := range files {
		b, x := os.ReadFile(p)
		var r Runbook
		if x == nil {
			x = json.Unmarshal(b, &r)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, r)
	}
	return out, nil
}
