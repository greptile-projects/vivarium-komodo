// Package testscenarios owns reusable, revision-exact executable behavior specifications.
package testscenarios

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

var ErrNotFound = errors.New("test scenario not found")
var ErrInvalid = errors.New("invalid test scenario")
var ErrConflict = errors.New("test scenario version conflict")
var ErrUnsafeFixture = errors.New("unsafe reusable fixture")

type Source struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision"`
	Rationale  string `json:"rationale"`
	Accessible bool   `json:"accessible"`
}
type Parameter struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Values      []string `json:"values"`
	Required    bool     `json:"required"`
}
type Step struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
}
type Assertion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Matcher     string `json:"matcher"`
	Expected    string `json:"expected"`
}
type Fixture struct {
	ID                     string   `json:"id"`
	Kind                   string   `json:"kind"`
	Description            string   `json:"description"`
	Source                 string   `json:"source"`
	SourceRevision         string   `json:"source_revision"`
	PrivacyClassification  string   `json:"privacy_classification"`
	Synthetic              bool     `json:"synthetic"`
	Accessible             bool     `json:"accessible"`
	ContainsSecret         bool     `json:"contains_secret"`
	ContainsProductionData bool     `json:"contains_production_personal_data"`
	Transformations        []string `json:"transformations"`
	Generator              string   `json:"generator,omitempty"`
}
type Environment struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Requirements []string `json:"requirements"`
	Network      string   `json:"network"`
}
type Contribution struct {
	Kind         string   `json:"kind"`
	Reference    string   `json:"reference"`
	Revision     string   `json:"revision"`
	Branch       string   `json:"branch,omitempty"`
	ChangedPaths []string `json:"changed_paths"`
	Contributor  string   `json:"contributor"`
	ActorKind    string   `json:"actor_kind"`
	Scope        []string `json:"scope,omitempty"`
}
type Generation struct {
	Generated   bool     `json:"generated"`
	Generator   string   `json:"generator,omitempty"`
	Assumptions []string `json:"assumptions"`
	Provenance  []string `json:"provenance"`
}
type Input struct {
	Name               string        `json:"name"`
	Purpose            string        `json:"purpose"`
	SourceRevision     string        `json:"source_revision"`
	DefinitionPath     string        `json:"definition_path"`
	QualityPlanID      string        `json:"quality_plan_id,omitempty"`
	QualityPlanVersion int64         `json:"quality_plan_version,omitempty"`
	Sources            []Source      `json:"sources"`
	Parameters         []Parameter   `json:"parameters"`
	Preconditions      []Step        `json:"preconditions"`
	Actions            []Step        `json:"actions"`
	Assertions         []Assertion   `json:"assertions"`
	Fixtures           []Fixture     `json:"fixtures"`
	Environments       []Environment `json:"environment_requirements"`
	Contribution       Contribution  `json:"contribution"`
	Generation         Generation    `json:"generation"`
	Tags               []string      `json:"tags"`
	ChangeReason       string        `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Gap struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}
type Scenario struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
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
func identifier() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func unique(v string, seen map[string]bool) bool {
	v = strings.TrimSpace(v)
	if v == "" || seen[v] {
		return false
	}
	seen[v] = true
	return true
}
func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Purpose) == "" || strings.TrimSpace(in.SourceRevision) == "" || strings.TrimSpace(in.DefinitionPath) == "" || strings.TrimSpace(in.ChangeReason) == "" || len(in.Sources) == 0 || len(in.Actions) == 0 || len(in.Assertions) == 0 || len(in.Environments) == 0 {
		return ErrInvalid
	}
	if filepath.IsAbs(in.DefinitionPath) || strings.Contains(filepath.Clean(in.DefinitionPath), "..") {
		return ErrInvalid
	}
	ids := map[string]bool{}
	for _, x := range in.Sources {
		if !unique(x.ID, ids) || !allowed(x.Kind, "issue", "reproduction", "design", "api_contract", "documentation", "journey") || x.Reference == "" || x.Revision == "" || x.Rationale == "" || !x.Accessible {
			return ErrInvalid
		}
	}
	params := map[string]bool{}
	for _, x := range in.Parameters {
		if !unique(x.Name, params) || !allowed(x.Type, "string", "integer", "number", "boolean", "enum", "json") || x.Description == "" || (x.Required && len(x.Values) == 0) {
			return ErrInvalid
		}
	}
	steps := map[string]bool{}
	for _, group := range [][]Step{in.Preconditions, in.Actions} {
		for _, x := range group {
			if !unique(x.ID, steps) || !allowed(x.Kind, "state", "command", "request", "interaction", "event", "wait") || x.Description == "" || (x.Kind == "command" && x.Command == "") {
				return ErrInvalid
			}
		}
	}
	assertions := map[string]bool{}
	for _, x := range in.Assertions {
		if !unique(x.ID, assertions) || x.Description == "" || !allowed(x.Matcher, "equals", "contains", "matches", "status", "count", "schema", "invariant", "accessible") || x.Expected == "" {
			return ErrInvalid
		}
	}
	fixtures := map[string]bool{}
	for _, x := range in.Fixtures {
		if !unique(x.ID, fixtures) || !allowed(x.Kind, "inline", "file", "generator", "factory", "seed") || x.Description == "" || x.Source == "" || x.SourceRevision == "" || !allowed(x.PrivacyClassification, "public", "internal", "confidential", "restricted") {
			return ErrInvalid
		}
		if !x.Accessible || x.ContainsSecret || x.ContainsProductionData {
			return ErrUnsafeFixture
		}
		if !x.Synthetic && len(x.Transformations) == 0 {
			return ErrUnsafeFixture
		}
		if x.Kind == "generator" && x.Generator == "" {
			return ErrInvalid
		}
	}
	envs := map[string]bool{}
	for _, x := range in.Environments {
		if !unique(x.ID, envs) || x.Description == "" || len(x.Requirements) == 0 || !allowed(x.Network, "none", "loopback", "sandbox") {
			return ErrInvalid
		}
	}
	c := in.Contribution
	if !allowed(c.Kind, "branch", "workspace", "pull_request") || c.Reference == "" || c.Revision == "" || c.Contributor == "" || !allowed(c.ActorKind, "human", "agent") {
		return ErrInvalid
	}
	if c.ActorKind == "agent" && len(c.Scope) == 0 {
		return ErrInvalid
	}
	for _, p := range c.ChangedPaths {
		if filepath.IsAbs(p) || strings.Contains(filepath.Clean(p), "..") {
			return ErrInvalid
		}
	}
	if in.Generation.Generated && (in.Generation.Generator == "" || len(in.Generation.Assumptions) == 0 || len(in.Generation.Provenance) == 0) {
		return ErrInvalid
	}
	if !in.Generation.Generated && (in.Generation.Generator != "" || len(in.Generation.Assumptions) > 0) {
		return ErrInvalid
	}
	return nil
}
func derive(v Version) []Gap {
	out := []Gap{}
	if len(v.Parameters) == 0 {
		out = append(out, Gap{"unparameterized", v.Name, "scenario has no declared variation"})
	}
	if len(v.Fixtures) == 0 {
		out = append(out, Gap{"missing_fixture", v.Name, "scenario does not declare reusable test data"})
	}
	if len(v.Preconditions) == 0 {
		out = append(out, Gap{"missing_precondition", v.Name, "starting state is implicit"})
	}
	if len(v.Contribution.ChangedPaths) == 0 {
		out = append(out, Gap{"missing_changed_paths", v.Contribution.Reference, "ordinary contribution does not identify its scenario or fixture paths"})
	}
	return out
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
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
	xs, e := s.list(repo)
	if e != nil {
		return Scenario{}, e
	}
	for _, x := range xs {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Scenario{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	x := Scenario{identifier(), repo, 1, []Version{v}, derive(v)}
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
		return Scenario{}, e
	}
	if x.CurrentVersion != expected {
		return Scenario{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v)
	return x, s.save(x)
}
func (s *Store) Get(repo, id string) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
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
	if len(x.Versions) > 0 {
		x.Gaps = derive(x.Versions[len(x.Versions)-1])
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Scenario, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Scenario{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Scenario{}
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
