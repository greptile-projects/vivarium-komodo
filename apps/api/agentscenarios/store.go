// Package agentscenarios owns domain-authored, reusable agent behavior cases.
package agentscenarios

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

var ErrNotFound = errors.New("agent scenario not found")
var ErrInvalid = errors.New("invalid agent scenario")
var ErrConflict = errors.New("agent scenario version conflict")
var ErrUnsafeContext = errors.New("unsafe agent scenario context")
var ErrForbidden = errors.New("agent scenario owner permission required")

type Source struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision"`
	Audience   string `json:"audience"`
	Provenance string `json:"provenance"`
	License    string `json:"license"`
	Sanitized  bool   `json:"sanitized"`
	Accessible bool   `json:"accessible"`
}
type Context struct {
	Name                 string   `json:"name"`
	Content              string   `json:"content"`
	Audience             string   `json:"audience"`
	Provenance           string   `json:"provenance"`
	License              string   `json:"license"`
	Sanitized            bool     `json:"sanitized"`
	ContainsPersonalData bool     `json:"contains_personal_data"`
	Embargoed            bool     `json:"embargoed"`
	Hidden               bool     `json:"hidden"`
	PermittedUses        []string `json:"permitted_uses"`
}
type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Weight      string `json:"weight"`
	Hidden      bool   `json:"hidden"`
}
type Budget struct {
	Kind  string  `json:"kind"`
	Limit float64 `json:"limit"`
	Unit  string  `json:"unit"`
}
type Contribution struct {
	Kind         string   `json:"kind"`
	Reference    string   `json:"reference"`
	Revision     string   `json:"revision"`
	Branch       string   `json:"branch,omitempty"`
	Workspace    string   `json:"workspace,omitempty"`
	ActorKind    string   `json:"actor_kind"`
	ActorID      string   `json:"actor_id"`
	ChangedPaths []string `json:"changed_paths"`
	Scope        []string `json:"scope,omitempty"`
}
type Input struct {
	Name                  string       `json:"name"`
	Purpose               string       `json:"purpose"`
	AgentProjectID        string       `json:"agent_project_id"`
	AgentProjectVersion   int64        `json:"agent_project_version"`
	RepositoryRevision    string       `json:"repository_revision"`
	DefinitionPath        string       `json:"definition_path"`
	Audience              string       `json:"audience"`
	Sources               []Source     `json:"sources"`
	Inputs                []string     `json:"inputs"`
	PermittedContext      []Context    `json:"permitted_context"`
	ExpectedOutcomes      []string     `json:"expected_outcomes"`
	Rubric                []Criterion  `json:"rubric"`
	ProhibitedBehavior    []string     `json:"prohibited_behavior"`
	Budgets               []Budget     `json:"budgets"`
	Uncertainty           []string     `json:"uncertainty"`
	RequiredHumanJudgment []string     `json:"required_human_judgment"`
	OwnerIDs              []string     `json:"owner_ids"`
	AllowedUses           []string     `json:"allowed_uses"`
	Contribution          Contribution `json:"contribution"`
	ChangeReason          string       `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Review struct {
	ID              string    `json:"id"`
	ScenarioVersion int64     `json:"scenario_version"`
	ReviewerID      string    `json:"reviewer_id"`
	ReviewerKind    string    `json:"reviewer_kind"`
	Decision        string    `json:"decision"`
	Rationale       string    `json:"rationale"`
	CreatedAt       time.Time `json:"created_at"`
}
type Scenario struct {
	ID                       string    `json:"id"`
	RepositoryID             string    `json:"repository_id"`
	CurrentVersion           int64     `json:"current_version"`
	Versions                 []Version `json:"versions"`
	Reviews                  []Review  `json:"reviews"`
	Approved                 bool      `json:"approved"`
	TrainingAllowed          bool      `json:"training_allowed"`
	BroaderEvaluationAllowed bool      `json:"broader_evaluation_allowed"`
	GrantsAuthority          bool      `json:"grants_authority"`
}
type Catalog struct {
	Items []Scenario `json:"items"`
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
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func has(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func validate(in Input) error {
	if in.Name == "" || in.Purpose == "" || in.AgentProjectID == "" || in.AgentProjectVersion < 1 || in.RepositoryRevision == "" || in.DefinitionPath == "" || in.ChangeReason == "" || !allowed(in.Audience, "public", "protected") || len(in.Sources) == 0 || len(in.Inputs) == 0 || len(in.ExpectedOutcomes) == 0 || len(in.Rubric) == 0 || len(in.ProhibitedBehavior) == 0 || len(in.Uncertainty) == 0 || len(in.RequiredHumanJudgment) == 0 || len(in.OwnerIDs) == 0 || !has(in.AllowedUses, "scenario_evaluation") {
		return ErrInvalid
	}
	for _, s := range in.Sources {
		if !allowed(s.Kind, "issue", "support_thread", "task", "incident", "decision", "prior_session") || s.Reference == "" || s.Revision == "" || s.Provenance == "" || s.License == "" || !s.Accessible || (s.Kind == "prior_session" && !s.Sanitized) {
			return ErrUnsafeContext
		}
		if in.Audience == "public" && s.Audience != "public" {
			return ErrUnsafeContext
		}
	}
	for _, c := range in.PermittedContext {
		if c.Name == "" || c.Provenance == "" || c.License == "" || !c.Sanitized || c.ContainsPersonalData || !has(c.PermittedUses, "scenario_evaluation") || (in.Audience == "public" && (c.Audience != "public" || c.Embargoed || c.Hidden)) {
			return ErrUnsafeContext
		}
	}
	if in.Audience == "public" {
		for _, c := range in.Rubric {
			if c.Hidden {
				return ErrUnsafeContext
			}
		}
	}
	if in.Contribution.Kind != "branch" && in.Contribution.Kind != "workspace" {
		return ErrInvalid
	}
	if in.Contribution.Reference == "" || in.Contribution.Revision == "" || in.Contribution.ActorID == "" || !allowed(in.Contribution.ActorKind, "human", "agent") || len(in.Contribution.ChangedPaths) == 0 {
		return ErrInvalid
	}
	if in.Contribution.ActorKind == "agent" && (len(in.Contribution.Scope) == 0 || !has(in.Contribution.Scope, in.DefinitionPath)) {
		return ErrForbidden
	}
	return nil
}
func identifier() string                     { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func derive(x *Scenario) {
	v := x.Versions[len(x.Versions)-1]
	x.TrainingAllowed = has(v.AllowedUses, "training")
	x.BroaderEvaluationAllowed = has(v.AllowedUses, "broader_evaluation")
	x.Approved = false
	for _, r := range x.Reviews {
		if r.ScenarioVersion == x.CurrentVersion && r.Decision == "approve" {
			x.Approved = true
		}
		if r.ScenarioVersion == x.CurrentVersion && r.Decision == "request_changes" {
			x.Approved = false
		}
	}
	x.GrantsAuthority = false
}
func (s *Store) save(x Scenario) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Scenario, error) {
	if repo == "" || actor == "" {
		return Scenario{}, ErrInvalid
	}
	if e := validate(in); e != nil {
		return Scenario{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Version{1, in, actor, s.now().UTC()}
	x := Scenario{ID: identifier(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{v}}
	derive(&x)
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (Scenario, error) {
	if actor == "" {
		return Scenario{}, ErrInvalid
	}
	if e := validate(in); e != nil {
		return Scenario{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return Scenario{}, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{x.CurrentVersion, in, actor, s.now().UTC()})
	derive(&x)
	return x, s.save(x)
}
func (s *Store) Review(repo, id, actor, kind string, version int64, decision, rationale string) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	v := x.Versions[len(x.Versions)-1]
	if version != x.CurrentVersion {
		return Scenario{}, ErrConflict
	}
	owner := has(v.OwnerIDs, actor)
	scoped := kind == "agent" && v.Contribution.ActorKind == "agent" && v.Contribution.ActorID == actor && has(v.Contribution.Scope, v.DefinitionPath)
	if !owner && !scoped {
		return Scenario{}, ErrForbidden
	}
	if !allowed(decision, "approve", "request_changes", "comment") || rationale == "" {
		return Scenario{}, ErrInvalid
	}
	x.Reviews = append(x.Reviews, Review{identifier(), version, actor, kind, decision, rationale, s.now().UTC()})
	derive(&x)
	return x, s.save(x)
}
func (s *Store) read(repo, id string) (Scenario, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Scenario{}, ErrNotFound
	}
	var x Scenario
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Scenario{}, ErrNotFound
	}
	derive(&x)
	return x, nil
}
func (s *Store) Get(repo, id string) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return Catalog{Items: []Scenario{}}, nil
	}
	if e != nil {
		return Catalog{}, e
	}
	out := []Scenario{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return Catalog{}, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].PublishedAt.After(out[j].Versions[len(out[j].Versions)-1].PublishedAt)
	})
	return Catalog{out}, nil
}
func Project(x Scenario, protected bool) Scenario {
	if protected {
		return x
	}
	protectedVersions := map[int64]bool{}
	for vi := range x.Versions {
		v := &x.Versions[vi]
		if v.Audience == "protected" {
			protectedVersions[v.Number] = true
			for i := range v.Inputs {
				v.Inputs[i] = "[protected input]"
			}
			for i := range v.ExpectedOutcomes {
				v.ExpectedOutcomes[i] = "[protected expectation]"
			}
			for i := range v.Sources {
				v.Sources[i].Reference = "[protected source]"
				v.Sources[i].Provenance = "[protected provenance]"
			}
			for i := range v.Rubric {
				if v.Rubric[i].Hidden {
					v.Rubric[i].Description = "[protected criterion]"
				}
			}
			for i := range v.PermittedContext {
				v.PermittedContext[i].Content = "[protected context]"
			}
		}
	}
	for i := range x.Reviews {
		if protectedVersions[x.Reviews[i].ScenarioVersion] {
			x.Reviews[i].Rationale = "[protected review]"
		}
	}
	return x
}
