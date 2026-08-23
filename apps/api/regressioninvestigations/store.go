// Package regressioninvestigations owns durable, shared boundaries for locating regressions.
package regressioninvestigations

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
)

var ErrNotFound = errors.New("regression investigation not found")
var ErrConflict = errors.New("regression investigation conflict")

type Source struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision,omitempty"`
}
type Boundary struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	CommitID  string `json:"commit_id"`
	ReleaseID string `json:"release_id,omitempty"`
}
type Evidence struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision,omitempty"`
	Summary    string    `json:"summary"`
	Audience   string    `json:"audience"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Entry struct {
	ID        string    `json:"id"`
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Scope struct {
	ExpectedBehavior   string   `json:"expected_behavior"`
	RegressedBehavior  string   `json:"regressed_behavior"`
	KnownGood          Boundary `json:"known_good"`
	KnownBad           Boundary `json:"known_bad"`
	Environments       []string `json:"environments"`
	Comparability      string   `json:"comparability"`
	Severity           string   `json:"severity"`
	OwnerIDs           []string `json:"owner_ids"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type ScopeChange struct {
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason"`
	Scope     Scope     `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}
type Investigation struct {
	ID           string        `json:"id"`
	RepositoryID string        `json:"repository_id"`
	Title        string        `json:"title"`
	Source       Source        `json:"source"`
	CreatorID    string        `json:"creator_id"`
	Version      int64         `json:"version"`
	Scope        Scope         `json:"scope"`
	Status       string        `json:"status"`
	Blockers     []string      `json:"blockers"`
	StaleInputs  []string      `json:"stale_inputs"`
	Evidence     []Evidence    `json:"evidence"`
	Entries      []Entry       `json:"entries"`
	ScopeChanges []ScopeChange `json:"scope_changes"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}
type Input struct {
	Title    string     `json:"title"`
	Source   Source     `json:"source"`
	Scope    Scope      `json:"scope"`
	Evidence []Evidence `json:"evidence"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in Input) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Investigation{ID: id(), RepositoryID: repo, Title: strings.TrimSpace(in.Title), Source: in.Source, CreatorID: actor, Version: 1, Scope: cleanScope(in.Scope), Status: "open", CreatedAt: now, UpdatedAt: now}
	v.Evidence = stampEvidence(in.Evidence, actor, now)
	v.Blockers = blockers(v.Scope)
	v.ScopeChanges = []ScopeChange{{Version: 1, ActorID: actor, Reason: "investigation opened", Scope: v.Scope, CreatedAt: now}}
	return v, s.write(v)
}
func (s *Store) Get(repo, key string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Investigation{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if er == nil && v.RepositoryID == repo {
			v.Entries = nil
			v.Evidence = nil
			v.ScopeChanges = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) ChangeScope(repo, key, actor, reason string, expected int64, scope Scope) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if v.Version != expected || strings.TrimSpace(reason) == "" {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Version++
	v.Scope = cleanScope(scope)
	v.Blockers = blockers(v.Scope)
	v.ScopeChanges = append(v.ScopeChanges, ScopeChange{Version: v.Version, ActorID: actor, Reason: strings.TrimSpace(reason), Scope: v.Scope, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddEvidence(repo, key, actor string, e Evidence) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	now := s.now().UTC()
	e = stampEvidence([]Evidence{e}, actor, now)[0]
	v.Evidence = append(v.Evidence, e)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddEntry(repo, key, actor string, e Entry) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	e.Body = strings.TrimSpace(e.Body)
	if e.Body == "" || len(e.Body) > 10000 || !validEntry(e.Kind) {
		return Investigation{}, ErrConflict
	}
	e.ID = id()
	e.Sequence = int64(len(v.Entries) + 1)
	e.ActorID = actor
	e.CreatedAt = s.now().UTC()
	v.Entries = append(v.Entries, e)
	v.UpdatedAt = e.CreatedAt
	return v, s.write(v)
}
func (s *Store) SetStatus(repo, key, actor, status, reason string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !validStatus(status) || strings.TrimSpace(reason) == "" || (status == "ready" && len(v.Blockers) > 0) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Status = status
	v.Entries = append(v.Entries, Entry{ID: id(), Sequence: int64(len(v.Entries) + 1), Kind: "status_change", Body: strings.TrimSpace(reason), ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func cleanScope(v Scope) Scope {
	v.ExpectedBehavior = strings.TrimSpace(v.ExpectedBehavior)
	v.RegressedBehavior = strings.TrimSpace(v.RegressedBehavior)
	v.Comparability = strings.TrimSpace(v.Comparability)
	v.Severity = strings.TrimSpace(v.Severity)
	v.Environments = clean(v.Environments)
	v.OwnerIDs = clean(v.OwnerIDs)
	v.AcceptanceCriteria = clean(v.AcceptanceCriteria)
	return v
}
func clean(xs []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func blockers(v Scope) []string {
	out := []string{}
	if v.ExpectedBehavior == "" {
		out = append(out, "expected_behavior_missing")
	}
	if v.RegressedBehavior == "" {
		out = append(out, "regressed_behavior_missing")
	}
	if v.KnownGood.CommitID == "" {
		out = append(out, "known_good_missing")
	}
	if v.KnownBad.CommitID == "" {
		out = append(out, "known_bad_missing")
	}
	if len(v.Environments) == 0 {
		out = append(out, "affected_environment_missing")
	}
	if v.Comparability == "" {
		out = append(out, "comparability_missing")
	}
	if v.Severity == "" {
		out = append(out, "severity_missing")
	}
	if len(v.OwnerIDs) == 0 {
		out = append(out, "owner_missing")
	}
	if len(v.AcceptanceCriteria) == 0 {
		out = append(out, "acceptance_criteria_missing")
	}
	return out
}
func stampEvidence(es []Evidence, actor string, now time.Time) []Evidence {
	out := make([]Evidence, len(es))
	for i, e := range es {
		e.ID = id()
		e.Kind = strings.TrimSpace(e.Kind)
		e.ResourceID = strings.TrimSpace(e.ResourceID)
		e.Summary = strings.TrimSpace(e.Summary)
		e.ActorID = actor
		e.CreatedAt = now
		if e.Audience == "" {
			e.Audience = "repository"
		}
		out[i] = e
	}
	return out
}
func validEntry(v string) bool {
	return v == "discussion" || v == "hypothesis" || v == "scope_note" || v == "status_change"
}
func validStatus(v string) bool { return v == "open" || v == "ready" || v == "paused" || v == "closed" }
func (s *Store) read(key string) (Investigation, error) {
	b, e := os.ReadFile(filepath.Join(s.root, key+".json"))
	if os.IsNotExist(e) {
		return Investigation{}, ErrNotFound
	}
	var v Investigation
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) write(v Investigation) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
