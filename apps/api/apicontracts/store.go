// Package apicontracts owns immutable, repository-scoped service interface publications.
package apicontracts

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

var ErrNotFound = errors.New("api contract not found")
var ErrInvalid = errors.New("invalid api contract publication")
var ErrConflict = errors.New("api contract version conflict")

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}
type Schema struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Fields []Field `json:"fields"`
}
type Operation struct {
	ID             string   `json:"id"`
	Method         string   `json:"method"`
	Path           string   `json:"path"`
	Summary        string   `json:"summary"`
	Authentication []string `json:"authentication"`
	RequestSchema  string   `json:"request_schema,omitempty"`
	ResponseSchema string   `json:"response_schema"`
	ErrorCodes     []string `json:"error_codes"`
}
type APIError struct {
	Code       string `json:"code"`
	HTTPStatus int    `json:"http_status"`
	Meaning    string `json:"meaning"`
	Retryable  bool   `json:"retryable"`
}
type Authentication struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
}
type Environment struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	Availability string `json:"availability"`
	Notes        string `json:"notes,omitempty"`
}
type Limit struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Unit  string `json:"unit"`
	Scope string `json:"scope"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision,omitempty"`
	Label      string `json:"label"`
	Status     string `json:"status"`
}
type Compatibility struct {
	Promise         string   `json:"promise"`
	PreviousVersion string   `json:"previous_version,omitempty"`
	BreakingChanges []string `json:"breaking_changes"`
	Migration       string   `json:"migration,omitempty"`
}
type Input struct {
	Name              string           `json:"name"`
	Version           string           `json:"version"`
	Description       string           `json:"description"`
	SourceRevision    string           `json:"source_revision"`
	DefinitionPath    string           `json:"definition_path"`
	DefinitionFormat  string           `json:"definition_format"`
	DefinitionValid   bool             `json:"definition_valid"`
	ValidationSummary string           `json:"validation_summary"`
	Operations        []Operation      `json:"operations"`
	Schemas           []Schema         `json:"schemas"`
	Errors            []APIError       `json:"errors"`
	Authentication    []Authentication `json:"authentication_modes"`
	Environments      []Environment    `json:"environments"`
	Limits            []Limit          `json:"limits"`
	OwnerIDs          []string         `json:"owner_ids"`
	Stability         string           `json:"stability"`
	SupportPolicy     string           `json:"support_policy"`
	Compatibility     Compatibility    `json:"compatibility"`
	Links             []Link           `json:"links"`
	KnownGaps         []string         `json:"known_gaps"`
	ChangeReason      string           `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Gap struct {
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
	Version int64  `json:"version"`
}
type Contract struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
}
type Comparison struct {
	ContractID        string        `json:"contract_id"`
	From              Version       `json:"from"`
	To                Version       `json:"to"`
	AddedOperations   []string      `json:"added_operations"`
	RemovedOperations []string      `json:"removed_operations"`
	ChangedSchemas    []string      `json:"changed_schemas"`
	Breaking          bool          `json:"breaking"`
	Compatibility     Compatibility `json:"compatibility"`
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Version) == "" || in.SourceRevision == "" || in.DefinitionPath == "" || !map[string]bool{"openapi": true, "json_schema": true, "protobuf": true, "graphql": true}[in.DefinitionFormat] || len(in.Operations) == 0 || len(in.Schemas) == 0 || len(in.OwnerIDs) == 0 || in.SupportPolicy == "" || in.ChangeReason == "" || !map[string]bool{"experimental": true, "beta": true, "stable": true, "deprecated": true}[in.Stability] {
		return false
	}
	schemas := map[string]bool{}
	for _, x := range in.Schemas {
		if x.Name == "" || schemas[x.Name] || !map[string]bool{"object": true, "array": true, "scalar": true, "enum": true}[x.Kind] {
			return false
		}
		schemas[x.Name] = true
	}
	errs := map[string]bool{}
	for _, x := range in.Errors {
		if x.Code == "" || errs[x.Code] || x.HTTPStatus < 400 || x.HTTPStatus > 599 || x.Meaning == "" {
			return false
		}
		errs[x.Code] = true
	}
	auth := map[string]bool{}
	for _, x := range in.Authentication {
		if x.ID == "" || auth[x.ID] || x.Kind == "" || x.Description == "" {
			return false
		}
		auth[x.ID] = true
	}
	ops := map[string]bool{}
	for _, x := range in.Operations {
		if x.ID == "" || ops[x.ID] || !map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}[x.Method] || !strings.HasPrefix(x.Path, "/") || !schemas[x.ResponseSchema] {
			return false
		}
		for _, a := range x.Authentication {
			if !auth[a] {
				return false
			}
		}
		for _, e := range x.ErrorCodes {
			if !errs[e] {
				return false
			}
		}
		ops[x.ID] = true
	}
	for _, x := range in.Environments {
		if x.Name == "" || x.BaseURL == "" || !map[string]bool{"available": true, "degraded": true, "unavailable": true, "planned": true}[x.Availability] {
			return false
		}
	}
	for _, x := range in.Links {
		if !map[string]bool{"source": true, "release": true, "documentation": true, "data_use": true}[x.Kind] || x.ResourceID == "" || x.Label == "" || !map[string]bool{"current": true, "missing": true, "stale": true, "unreleased": true}[x.Status] {
			return false
		}
	}
	return true
}
func derive(v Version) []Gap {
	out := []Gap{}
	add := func(k, d string) { out = append(out, Gap{Kind: k, Detail: d, Version: v.Number}) }
	if !v.DefinitionValid {
		add("invalid_definition", v.ValidationSummary)
	}
	kinds := map[string]bool{}
	for _, l := range v.Links {
		kinds[l.Kind] = true
		if l.Status != "current" {
			k := l.Status
			if l.Kind == "documentation" && l.Status == "stale" {
				k = "stale_documentation"
			}
			if l.Kind == "release" && l.Status == "unreleased" {
				k = "unreleased_implementation"
			}
			add(k, l.Label)
		}
	}
	for _, k := range []string{"source", "release", "documentation", "data_use"} {
		if !kinds[k] {
			add("missing_"+k, "no "+k+" provenance is linked")
		}
	}
	for _, e := range v.Environments {
		if e.Availability != "available" {
			add("environment_"+e.Availability, e.Name)
		}
	}
	for _, g := range v.KnownGaps {
		add("declared_gap", g)
	}
	return out
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Contract) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Contract, error) {
	if actor == "" || !valid(in) {
		return Contract{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.list(repo)
	if e != nil {
		return Contract{}, e
	}
	for _, x := range items {
		for _, v := range x.Versions {
			if strings.EqualFold(v.Version, in.Version) && strings.EqualFold(v.Name, in.Name) {
				return Contract{}, ErrConflict
			}
		}
	}
	v := Version{Number: 1, Input: in, AuthorID: actor, PublishedAt: s.now().UTC()}
	x := Contract{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{v}, Gaps: derive(v)}
	return x, s.save(x)
}
func (s *Store) Revise(repo, cid, actor string, expected int64, in Input) (Contract, error) {
	if actor == "" || !valid(in) {
		return Contract{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, cid)
	if e != nil {
		return Contract{}, e
	}
	if x.CurrentVersion != expected {
		return Contract{}, ErrConflict
	}
	for _, v := range x.Versions {
		if strings.EqualFold(v.Version, in.Version) {
			return Contract{}, ErrConflict
		}
	}
	v := Version{Number: expected + 1, Input: in, AuthorID: actor, PublishedAt: s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v)
	return x, s.save(x)
}
func (s *Store) Get(repo, cid string) (Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, cid)
}
func (s *Store) List(repo string) ([]Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repo)
}
func (s *Store) read(repo, cid string) (Contract, error) {
	b, e := os.ReadFile(s.path(repo, cid))
	if errors.Is(e, fs.ErrNotExist) {
		return Contract{}, ErrNotFound
	}
	var x Contract
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != cid {
		return Contract{}, ErrNotFound
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Contract, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Contract{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Contract{}
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
func (s *Store) Compare(repo, cid string, from, to int64) (Comparison, error) {
	x, e := s.Get(repo, cid)
	if e != nil {
		return Comparison{}, e
	}
	var a, b *Version
	for i := range x.Versions {
		if x.Versions[i].Number == from {
			a = &x.Versions[i]
		}
		if x.Versions[i].Number == to {
			b = &x.Versions[i]
		}
	}
	if a == nil || b == nil || from == to {
		return Comparison{}, ErrInvalid
	}
	c := Comparison{ContractID: cid, From: *a, To: *b, Compatibility: b.Compatibility}
	ao := map[string]bool{}
	bo := map[string]bool{}
	for _, o := range a.Operations {
		ao[o.ID] = true
	}
	for _, o := range b.Operations {
		bo[o.ID] = true
		if !ao[o.ID] {
			c.AddedOperations = append(c.AddedOperations, o.ID)
		}
	}
	for id := range ao {
		if !bo[id] {
			c.RemovedOperations = append(c.RemovedOperations, id)
		}
	}
	as := map[string]string{}
	for _, z := range a.Schemas {
		q, _ := json.Marshal(z)
		as[z.Name] = string(q)
	}
	for _, z := range b.Schemas {
		q, _ := json.Marshal(z)
		if old, ok := as[z.Name]; ok && old != string(q) {
			c.ChangedSchemas = append(c.ChangedSchemas, z.Name)
		}
	}
	sort.Strings(c.AddedOperations)
	sort.Strings(c.RemovedOperations)
	sort.Strings(c.ChangedSchemas)
	c.Breaking = len(c.RemovedOperations) > 0 || len(c.ChangedSchemas) > 0 || len(b.Compatibility.BreakingChanges) > 0
	return c, nil
}
