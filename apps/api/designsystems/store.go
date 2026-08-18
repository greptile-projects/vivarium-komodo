// Package designsystems owns immutable repository-scoped visual and interaction decisions.
package designsystems

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

var ErrNotFound = errors.New("design system not found")
var ErrInvalid = errors.New("invalid design system")
var ErrConflict = errors.New("design system version conflict")

type Token struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Value       string `json:"value"`
	Description string `json:"description"`
}
type Component struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose"`
	Usage    string   `json:"usage"`
	DoNotUse string   `json:"do_not_use,omitempty"`
	Props    []string `json:"props"`
}
type Pattern struct {
	Name     string `json:"name"`
	Trigger  string `json:"trigger"`
	Behavior string `json:"behavior"`
	Feedback string `json:"feedback"`
	Keyboard string `json:"keyboard"`
}
type ContentRule struct {
	Name     string `json:"name"`
	Guidance string `json:"guidance"`
	Example  string `json:"example"`
	Avoid    string `json:"avoid,omitempty"`
}
type ResponsiveRule struct {
	Name         string `json:"name"`
	MinimumWidth int    `json:"minimum_width"`
	MaximumWidth int    `json:"maximum_width,omitempty"`
	Behavior     string `json:"behavior"`
}
type Theme struct {
	Name           string            `json:"name"`
	Purpose        string            `json:"purpose"`
	TokenOverrides map[string]string `json:"token_overrides"`
}
type Example struct {
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Markup      string `json:"markup"`
	Theme       string `json:"theme"`
	Locale      string `json:"locale"`
	Viewport    string `json:"viewport"`
	Description string `json:"description"`
}
type Constraint struct {
	Subject     string `json:"subject"`
	Requirement string `json:"requirement"`
	Evidence    string `json:"evidence,omitempty"`
}
type Consumer struct {
	Name                   string `json:"name"`
	RepositoryID           string `json:"repository_id,omitempty"`
	ImplementationRevision string `json:"implementation_revision"`
	ReleaseRevision        string `json:"release_revision,omitempty"`
	AdoptedVersion         int64  `json:"adopted_version"`
	Status                 string `json:"status"`
	Notes                  string `json:"notes,omitempty"`
}
type AdoptionPolicy struct {
	Required      bool     `json:"required"`
	Consumers     []string `json:"consumers"`
	Exceptions    string   `json:"exceptions"`
	ReviewCadence string   `json:"review_cadence"`
}
type Provenance struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Rationale string `json:"rationale"`
}
type Input struct {
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	SourceRevision  string           `json:"source_revision"`
	DefinitionPath  string           `json:"definition_path"`
	ReleaseRevision string           `json:"release_revision"`
	Tokens          []Token          `json:"tokens"`
	Components      []Component      `json:"components"`
	Patterns        []Pattern        `json:"interaction_patterns"`
	ContentRules    []ContentRule    `json:"content_rules"`
	ResponsiveRules []ResponsiveRule `json:"responsive_rules"`
	Themes          []Theme          `json:"themes"`
	Examples        []Example        `json:"examples"`
	Accessibility   []Constraint     `json:"accessibility_constraints"`
	Localization    []Constraint     `json:"localization_constraints"`
	OwnerIDs        []string         `json:"owner_ids"`
	Adoption        AdoptionPolicy   `json:"adoption_policy"`
	Consumers       []Consumer       `json:"consumers"`
	Provenance      []Provenance     `json:"provenance"`
	ChangeReason    string           `json:"change_reason"`
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
	Version int64  `json:"version"`
}
type System struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
}
type Conflict struct {
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Systems []string `json:"systems"`
	Values  []string `json:"values"`
}
type Catalog struct {
	Items     []System   `json:"items"`
	Conflicts []Conflict `json:"conflicts"`
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
func cleanUnique(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		k := strings.ToLower(strings.TrimSpace(v))
		if k == "" || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || in.SourceRevision == "" || in.DefinitionPath == "" || in.ReleaseRevision == "" || in.ChangeReason == "" || len(in.Tokens) == 0 || len(in.Components) == 0 || len(in.Patterns) == 0 || len(in.ContentRules) == 0 || len(in.ResponsiveRules) == 0 || len(in.Themes) == 0 || len(in.Examples) == 0 || len(in.Accessibility) == 0 || len(in.Localization) == 0 {
		return false
	}
	names := []string{}
	themes := map[string]bool{}
	tokens := map[string]bool{}
	for _, x := range in.Tokens {
		names = append(names, x.Name)
		tokens[x.Name] = true
		if x.Category == "" || x.Value == "" || x.Description == "" {
			return false
		}
	}
	if !cleanUnique(names) {
		return false
	}
	names = nil
	for _, x := range in.Components {
		names = append(names, x.Name)
		if x.Purpose == "" || x.Usage == "" {
			return false
		}
	}
	if !cleanUnique(names) {
		return false
	}
	names = nil
	for _, x := range in.Patterns {
		names = append(names, x.Name)
		if x.Trigger == "" || x.Behavior == "" || x.Feedback == "" {
			return false
		}
	}
	if !cleanUnique(names) {
		return false
	}
	for _, x := range in.ContentRules {
		if x.Name == "" || x.Guidance == "" || x.Example == "" {
			return false
		}
	}
	for _, x := range in.ResponsiveRules {
		if x.Name == "" || x.MinimumWidth < 0 || (x.MaximumWidth > 0 && x.MaximumWidth < x.MinimumWidth) || x.Behavior == "" {
			return false
		}
	}
	names = nil
	for _, x := range in.Themes {
		names = append(names, x.Name)
		themes[x.Name] = true
		if x.Purpose == "" {
			return false
		}
		for k := range x.TokenOverrides {
			if !tokens[k] {
				return false
			}
		}
	}
	if !cleanUnique(names) {
		return false
	}
	for _, x := range in.Examples {
		if x.Name == "" || x.Subject == "" || x.Markup == "" || !themes[x.Theme] || x.Locale == "" || x.Viewport == "" {
			return false
		}
	}
	for _, x := range append(append([]Constraint{}, in.Accessibility...), in.Localization...) {
		if x.Subject == "" || x.Requirement == "" {
			return false
		}
	}
	for _, x := range in.Consumers {
		if x.Name == "" || x.ImplementationRevision == "" || x.AdoptedVersion < 1 || !map[string]bool{"current": true, "stale": true, "unsupported": true, "planned": true}[x.Status] {
			return false
		}
	}
	for _, x := range in.Provenance {
		if !map[string]bool{"decision": true, "pull_request": true, "release": true, "research": true, "documentation": true}[x.Kind] || x.Reference == "" || x.Rationale == "" {
			return false
		}
	}
	return true
}
func derive(v Version) []Gap {
	out := []Gap{}
	add := func(k, s, d string) { out = append(out, Gap{Kind: k, Subject: s, Detail: d, Version: v.Number}) }
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "no accountable owner is declared")
	}
	if len(v.Provenance) == 0 {
		add("missing_provenance", v.Name, "no reviewed decision or implementation provenance is linked")
	}
	for _, c := range v.Consumers {
		if c.Status == "unsupported" {
			add("unsupported_consumer", c.Name, c.Notes)
		}
		if c.Status == "stale" || c.AdoptedVersion != v.Number {
			add("stale_implementation", c.Name, "implementation "+c.ImplementationRevision+" adopts design version "+itoa(c.AdoptedVersion))
		}
		if c.Status == "planned" {
			add("unimplemented_consumer", c.Name, c.Notes)
		}
	}
	return out
}
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x System) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (System, error) {
	if actor == "" || !valid(in) {
		return System{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.list(repo)
	if e != nil {
		return System{}, e
	}
	for _, x := range items {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return System{}, ErrConflict
		}
	}
	v := Version{Number: 1, Input: in, AuthorID: actor, PublishedAt: s.now().UTC()}
	x := System{ID: identifier(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{v}, Gaps: derive(v)}
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (System, error) {
	if actor == "" || !valid(in) {
		return System{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return System{}, e
	}
	if x.CurrentVersion != expected {
		return System{}, ErrConflict
	}
	v := Version{Number: expected + 1, Input: in, AuthorID: actor, PublishedAt: s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v)
	return x, s.save(x)
}
func (s *Store) Get(repo, id string) (System, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.list(repo)
	if e != nil {
		return Catalog{}, e
	}
	return Catalog{Items: items, Conflicts: conflicts(items)}, nil
}
func (s *Store) read(repo, id string) (System, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return System{}, ErrNotFound
	}
	var x System
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return System{}, ErrNotFound
	}
	return x, nil
}
func (s *Store) list(repo string) ([]System, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []System{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []System{}
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
func conflicts(items []System) []Conflict {
	type claim struct{ system, value string }
	all := map[string][]claim{}
	for _, x := range items {
		v := x.Versions[len(x.Versions)-1]
		for _, t := range v.Tokens {
			all["token\x00"+strings.ToLower(t.Name)] = append(all["token\x00"+strings.ToLower(t.Name)], claim{x.ID, t.Value})
		}
		for _, c := range v.Components {
			all["component\x00"+strings.ToLower(c.Name)] = append(all["component\x00"+strings.ToLower(c.Name)], claim{x.ID, c.Usage})
		}
		for _, p := range v.Patterns {
			all["interaction_pattern\x00"+strings.ToLower(p.Name)] = append(all["interaction_pattern\x00"+strings.ToLower(p.Name)], claim{x.ID, p.Behavior})
		}
	}
	out := []Conflict{}
	for k, cs := range all {
		values := map[string]bool{}
		for _, c := range cs {
			values[c.value] = true
		}
		if len(values) < 2 {
			continue
		}
		parts := strings.Split(k, "\x00")
		c := Conflict{Kind: parts[0], Name: parts[1]}
		for _, x := range cs {
			c.Systems = append(c.Systems, x.system)
			c.Values = append(c.Values, x.value)
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
