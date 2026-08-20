// Package agentprojects owns reviewable, revision-exact agent behavior contracts.
package agentprojects

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

var ErrNotFound = errors.New("agent project not found")
var ErrInvalid = errors.New("invalid agent project")
var ErrConflict = errors.New("agent project version conflict")

type ReviewedText struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Content  string   `json:"content"`
	Revision string   `json:"revision"`
	OwnerIDs []string `json:"owner_ids"`
}
type Dependency struct {
	Kind               string   `json:"kind"`
	Reference          string   `json:"reference"`
	Revision           string   `json:"revision"`
	Accessible         bool     `json:"accessible"`
	InaccessibleReason string   `json:"inaccessible_reason,omitempty"`
	OwnerIDs           []string `json:"owner_ids"`
}
type Tool struct {
	Name         string   `json:"name"`
	Revision     string   `json:"revision"`
	Capabilities []string `json:"capabilities"`
	Boundary     string   `json:"boundary"`
	OwnerIDs     []string `json:"owner_ids"`
}
type Model struct {
	Provider   string      `json:"provider"`
	Name       string      `json:"name"`
	Revision   string      `json:"revision"`
	Guarantees []Guarantee `json:"guarantees"`
}
type Guarantee struct {
	Claim             string `json:"claim"`
	Supported         bool   `json:"supported"`
	Evidence          string `json:"evidence,omitempty"`
	UnsupportedReason string `json:"unsupported_reason,omitempty"`
}
type KnowledgeSource struct {
	Reference          string `json:"reference"`
	Revision           string `json:"revision"`
	Audience           string `json:"audience"`
	DataUse            string `json:"data_use"`
	Accessible         bool   `json:"accessible"`
	InaccessibleReason string `json:"inaccessible_reason,omitempty"`
}
type MemoryPolicy struct {
	Scope        string `json:"scope"`
	Retention    string `json:"retention"`
	TrainingUse  bool   `json:"training_use"`
	DeletionRule string `json:"deletion_rule"`
}
type Budget struct {
	Kind   string  `json:"kind"`
	Limit  float64 `json:"limit"`
	Unit   string  `json:"unit"`
	Period string  `json:"period"`
}
type Escalation struct {
	Trigger    string   `json:"trigger"`
	OwnerIDs   []string `json:"owner_ids"`
	Action     string   `json:"action"`
	BlocksWork bool     `json:"blocks_work"`
}
type Input struct {
	Name                 string            `json:"name"`
	Purpose              string            `json:"purpose"`
	RepositoryRevision   string            `json:"repository_revision"`
	DefinitionPath       string            `json:"definition_path"`
	Prompts              []ReviewedText    `json:"prompts"`
	Instructions         []ReviewedText    `json:"instructions"`
	Tools                []Tool            `json:"tools"`
	Models               []Model           `json:"models"`
	KnowledgeSources     []KnowledgeSource `json:"knowledge_sources"`
	Dependencies         []Dependency      `json:"dependencies"`
	MemoryPolicy         MemoryPolicy      `json:"memory_policy"`
	SupportedTasks       []string          `json:"supported_tasks"`
	ExpectedOutputs      []string          `json:"expected_outputs"`
	ProhibitedActions    []string          `json:"prohibited_actions"`
	DataUseTerms         []string          `json:"data_use_terms"`
	Budgets              []Budget          `json:"budgets"`
	OwnerIDs             []string          `json:"owner_ids"`
	HumanEscalations     []Escalation      `json:"human_escalations"`
	DeploymentBoundaries []string          `json:"deployment_boundaries"`
	ChangeReason         string            `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Gap struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to"`
}
type Project struct {
	ID                    string    `json:"id"`
	RepositoryID          string    `json:"repository_id"`
	CurrentVersion        int64     `json:"current_version"`
	Versions              []Version `json:"versions"`
	EffectiveCapabilities []string  `json:"effective_capabilities"`
	Gaps                  []Gap     `json:"gaps"`
	GrantsAuthority       bool      `json:"grants_authority"`
}
type Catalog struct {
	Items []Project `json:"items"`
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
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Purpose) == "" || in.RepositoryRevision == "" || in.DefinitionPath == "" || in.ChangeReason == "" || len(in.Prompts) == 0 || len(in.Instructions) == 0 || len(in.Models) == 0 || len(in.SupportedTasks) == 0 || len(in.ExpectedOutputs) == 0 || len(in.ProhibitedActions) == 0 || len(in.DataUseTerms) == 0 || len(in.DeploymentBoundaries) == 0 {
		return false
	}
	for _, x := range append(append([]ReviewedText{}, in.Prompts...), in.Instructions...) {
		if x.ID == "" || (x.Kind != "prompt" && x.Kind != "instruction") || x.Content == "" || x.Revision == "" {
			return false
		}
	}
	for _, x := range in.Tools {
		if x.Name == "" || x.Revision == "" || x.Boundary == "" {
			return false
		}
	}
	for _, x := range in.Models {
		if x.Provider == "" || x.Name == "" || x.Revision == "" {
			return false
		}
	}
	for _, x := range in.Dependencies {
		if x.Kind == "" || x.Reference == "" || x.Revision == "" || (!x.Accessible && x.InaccessibleReason == "") {
			return false
		}
	}
	for _, x := range in.KnowledgeSources {
		if x.Reference == "" || x.Revision == "" || x.DataUse == "" || (!x.Accessible && x.InaccessibleReason == "") {
			return false
		}
	}
	return in.MemoryPolicy.Scope != "" && in.MemoryPolicy.Retention != "" && in.MemoryPolicy.DeletionRule != ""
}
func derive(v Version) ([]string, []Gap) {
	caps := []string{}
	gaps := []Gap{}
	add := func(k, s, d string) { gaps = append(gaps, Gap{k, s, d, v.AuthorID}) }
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "agent project has no accountable owner")
	}
	for _, x := range v.Tools {
		caps = append(caps, x.Capabilities...)
		if len(x.OwnerIDs) == 0 {
			add("missing_owner", x.Name, "tool boundary has no accountable owner")
		}
	}
	for _, x := range v.Dependencies {
		if !x.Accessible {
			add("inaccessible_dependency", x.Reference, x.InaccessibleReason)
		}
		if len(x.OwnerIDs) == 0 {
			add("missing_owner", x.Reference, "dependency has no accountable owner")
		}
	}
	for _, x := range v.KnowledgeSources {
		if !x.Accessible {
			add("inaccessible_dependency", x.Reference, x.InaccessibleReason)
		}
	}
	for _, m := range v.Models {
		for _, g := range m.Guarantees {
			if !g.Supported {
				add("unsupported_guarantee", m.Provider+"/"+m.Name+": "+g.Claim, g.UnsupportedReason)
			}
		}
	}
	seen := map[string]string{}
	for _, x := range append(append([]ReviewedText{}, v.Prompts...), v.Instructions...) {
		key := strings.ToLower(strings.TrimSpace(x.Content))
		if p, ok := seen[key]; ok && p != x.Kind {
			add("conflicting_instruction", x.ID, "duplicates "+p+" content with a different precedence class")
		}
		seen[key] = x.Kind
		if len(x.OwnerIDs) == 0 {
			add("missing_owner", x.ID, "reviewed behavior text has no owner")
		}
	}
	sort.Strings(caps)
	caps = compact(caps)
	return caps, gaps
}
func compact(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		if len(out) == 0 || out[len(out)-1] != x {
			out = append(out, x)
		}
	}
	return out
}
func id() string                             { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Project) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Project, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Project{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Project{}, e
	}
	for _, x := range xs {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Project{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	c, g := derive(v)
	x := Project{id(), repo, 1, []Version{v}, c, g, false}
	return x, s.save(x)
}
func (s *Store) Revise(repo, idv, actor string, expected int64, in Input) (Project, error) {
	if actor == "" || !valid(in) {
		return Project{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, idv)
	if e != nil {
		return Project{}, e
	}
	if x.CurrentVersion != expected {
		return Project{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.EffectiveCapabilities, x.Gaps = derive(v)
	return x, s.save(x)
}
func (s *Store) Get(repo, idv string) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, idv)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
}
func (s *Store) read(repo, idv string) (Project, error) {
	b, e := os.ReadFile(s.path(repo, idv))
	if errors.Is(e, fs.ErrNotExist) {
		return Project{}, ErrNotFound
	}
	var x Project
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != idv {
		return Project{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.EffectiveCapabilities, x.Gaps = derive(x.Versions[len(x.Versions)-1])
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Project, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Project{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Project{}
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
