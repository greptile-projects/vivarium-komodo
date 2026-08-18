// Package runtimereplays retains privacy-safe scenarios derived from production
// observations and their isolated attempts. It never stores protected captures,
// credentials, or authority to operate on an environment.
package runtimereplays

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

var ErrNotFound = errors.New("runtime replay not found")
var ErrInvalid = errors.New("invalid runtime replay")
var ErrForbidden = errors.New("runtime replay forbidden")

type Input struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	Value            string `json:"value"`
	SourceEvidenceID string `json:"source_evidence_id"`
	Transformation   string `json:"transformation"`
}
type Invariant struct {
	Name        string `json:"name"`
	Expectation string `json:"expectation"`
}
type Refinement struct {
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Attempt struct {
	ID                    string            `json:"id"`
	ActorID               string            `json:"actor_id"`
	TargetKind            string            `json:"target_kind"`
	TargetID              string            `json:"target_id"`
	Revision              string            `json:"revision"`
	Environment           map[string]string `json:"environment"`
	Commands              []string          `json:"commands"`
	Traces                []string          `json:"traces"`
	Outputs               []string          `json:"outputs"`
	InvariantResults      map[string]bool   `json:"invariant_results"`
	Cost                  float64           `json:"cost"`
	ProductionDifferences []string          `json:"production_differences"`
	Blockers              []string          `json:"blockers"`
	Status                string            `json:"status"`
	Reproduced            bool              `json:"reproduced"`
	CreatedAt             time.Time         `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
type Scenario struct {
	ID                      string       `json:"id"`
	RepositoryID            string       `json:"repository_id"`
	WorkspaceID             string       `json:"workspace_id"`
	InvestigationID         string       `json:"investigation_id,omitempty"`
	Revision                string       `json:"revision"`
	Name                    string       `json:"name"`
	Behavior                string       `json:"behavior"`
	Audience                string       `json:"audience"`
	ParticipantIDs          []string     `json:"participant_ids"`
	EvidenceIDs             []string     `json:"evidence_ids"`
	StateKind               string       `json:"state_kind"`
	Inputs                  []Input      `json:"inputs"`
	Setup                   []string     `json:"setup"`
	Commands                []string     `json:"commands"`
	Invariants              []Invariant  `json:"invariants"`
	UnsafeSideEffects       []string     `json:"unsafe_side_effects,omitempty"`
	Refinements             []Refinement `json:"refinements"`
	Attempts                []Attempt    `json:"attempts"`
	Status                  string       `json:"status"`
	Reproduced              bool         `json:"reproduced"`
	RepeatedPassingAttempts int          `json:"repeated_passing_attempts"`
	Blockers                []string     `json:"blockers,omitempty"`
	Authority               []string     `json:"authority"`
	Events                  []Event      `json:"events"`
	CreatedAt               time.Time    `json:"created_at"`
	UpdatedAt               time.Time    `json:"updated_at"`
}
type CreateInput struct {
	InvestigationID   string      `json:"investigation_id"`
	Revision          string      `json:"revision"`
	Name              string      `json:"name"`
	Behavior          string      `json:"behavior"`
	Audience          string      `json:"audience"`
	ParticipantIDs    []string    `json:"participant_ids"`
	EvidenceIDs       []string    `json:"evidence_ids"`
	StateKind         string      `json:"state_kind"`
	Inputs            []Input     `json:"inputs"`
	Setup             []string    `json:"setup"`
	Commands          []string    `json:"commands"`
	Invariants        []Invariant `json:"invariants"`
	UnsafeSideEffects []string    `json:"unsafe_side_effects"`
}
type AttemptInput struct {
	TargetKind            string            `json:"target_kind"`
	TargetID              string            `json:"target_id"`
	Revision              string            `json:"revision"`
	Environment           map[string]string `json:"environment"`
	Commands              []string          `json:"commands"`
	Traces                []string          `json:"traces"`
	Outputs               []string          `json:"outputs"`
	InvariantResults      map[string]bool   `json:"invariant_results"`
	Cost                  float64           `json:"cost"`
	ProductionDifferences []string          `json:"production_differences"`
	Blockers              []string          `json:"blockers"`
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
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func clean(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func safeText(v string) bool {
	l := strings.ToLower(v)
	for _, x := range []string{"authorization: bearer", "private key", "password=", "secret=", "token=", "ghp_", "github_pat_", "sk-"} {
		if strings.Contains(l, x) {
			return false
		}
	}
	return len(v) <= 10000
}
func validateStrings(xs []string, max int) bool {
	if len(xs) > max {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || !safeText(x) {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, workspace, actor string, in CreateInput) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || workspace == "" || actor == "" || in.Revision == "" || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Behavior) == "" || !map[string]bool{"repository": true, "participants": true}[in.Audience] || !map[string]bool{"synthetic": true, "privacy_preserving": true}[in.StateKind] || len(in.EvidenceIDs) == 0 || len(in.Commands) == 0 || len(in.Invariants) == 0 || !validateStrings(in.Setup, 20) || !validateStrings(in.Commands, 20) || !validateStrings(in.UnsafeSideEffects, 20) {
		return Scenario{}, ErrInvalid
	}
	for _, v := range append([]string{in.Name, in.Behavior}, in.EvidenceIDs...) {
		if !safeText(v) {
			return Scenario{}, ErrInvalid
		}
	}
	for _, v := range in.Inputs {
		if v.Name == "" || v.Value == "" || v.SourceEvidenceID == "" || !has(in.EvidenceIDs, v.SourceEvidenceID) || !map[string]bool{"synthetic": true, "aggregate": true, "redacted": true}[v.Kind] || v.Transformation == "" || !safeText(v.Value) {
			return Scenario{}, ErrInvalid
		}
	}
	for _, v := range in.Invariants {
		if v.Name == "" || v.Expectation == "" || !safeText(v.Expectation) {
			return Scenario{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	v := Scenario{ID: id(), RepositoryID: repo, WorkspaceID: workspace, InvestigationID: in.InvestigationID, Revision: in.Revision, Name: strings.TrimSpace(in.Name), Behavior: strings.TrimSpace(in.Behavior), Audience: in.Audience, ParticipantIDs: clean(append(in.ParticipantIDs, actor)), EvidenceIDs: clean(in.EvidenceIDs), StateKind: in.StateKind, Inputs: in.Inputs, Setup: in.Setup, Commands: in.Commands, Invariants: in.Invariants, UnsafeSideEffects: clean(in.UnsafeSideEffects), Status: "draft", Authority: []string{}, CreatedAt: now, UpdatedAt: now}
	s.event(&v, "scenario.created", actor, "Sanitized scenario derived without copying protected evidence or granting runtime authority.")
	return v, s.write(v)
}
func (s *Store) List(repo, workspace string) ([]Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Scenario{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		v, x := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if x == nil && v.RepositoryID == repo && v.WorkspaceID == workspace {
			s.derive(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Get(repo, x string) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Scenario{}, ErrNotFound
	}
	s.derive(&v)
	return v, nil
}
func (s *Store) Refine(repo, x, actor, summary string) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Scenario{}, ErrNotFound
	}
	if !has(v.ParticipantIDs, actor) {
		return Scenario{}, ErrForbidden
	}
	if strings.TrimSpace(summary) == "" || !safeText(summary) {
		return Scenario{}, ErrInvalid
	}
	now := s.now().UTC()
	v.Refinements = append(v.Refinements, Refinement{ID: id(), Summary: summary, ActorID: actor, CreatedAt: now})
	s.event(&v, "scenario.refined", actor, summary)
	return v, s.write(v)
}
func (s *Store) Attempt(repo, x, actor string, in AttemptInput) (Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Scenario{}, ErrNotFound
	}
	if !has(v.ParticipantIDs, actor) {
		return Scenario{}, ErrForbidden
	}
	if !map[string]bool{"workspace": true, "preview": true}[in.TargetKind] || in.TargetID == "" || in.Revision == "" || len(in.Environment) == 0 || len(in.Commands) == 0 || in.Cost < 0 || !validateStrings(in.Commands, 30) || !validateStrings(in.Traces, 100) || !validateStrings(in.Outputs, 100) || !validateStrings(in.ProductionDifferences, 50) || !validateStrings(in.Blockers, 30) {
		return Scenario{}, ErrInvalid
	}
	blockers := clean(in.Blockers)
	if in.Revision != v.Revision {
		blockers = clean(append(blockers, "changed_revision"))
	}
	if len(v.UnsafeSideEffects) > 0 {
		blockers = clean(append(blockers, "unsafe_side_effects"))
	}
	all := true
	for _, q := range v.Invariants {
		passed, ok := in.InvariantResults[q.Name]
		if !ok || !passed {
			all = false
		}
	}
	reproduced := all && len(blockers) == 0
	status := "not_reproduced"
	if len(blockers) > 0 {
		status = "blocked"
	} else if reproduced {
		status = "observed"
	}
	now := s.now().UTC()
	a := Attempt{ID: id(), ActorID: actor, TargetKind: in.TargetKind, TargetID: in.TargetID, Revision: in.Revision, Environment: in.Environment, Commands: in.Commands, Traces: in.Traces, Outputs: in.Outputs, InvariantResults: in.InvariantResults, Cost: in.Cost, ProductionDifferences: clean(in.ProductionDifferences), Blockers: blockers, Status: status, Reproduced: reproduced, CreatedAt: now}
	v.Attempts = append(v.Attempts, a)
	s.event(&v, "attempt."+status, actor, a.ID)
	s.derive(&v)
	return v, s.write(v)
}
func (s *Store) derive(v *Scenario) {
	v.RepeatedPassingAttempts = 0
	v.Blockers = nil
	for _, a := range v.Attempts {
		if a.Reproduced {
			v.RepeatedPassingAttempts++
		}
		v.Blockers = append(v.Blockers, a.Blockers...)
	}
	v.Blockers = clean(v.Blockers)
	v.Reproduced = v.RepeatedPassingAttempts >= 2 && len(v.Blockers) == 0
	if v.Reproduced {
		v.Status = "reproduced"
	} else if len(v.Blockers) > 0 {
		v.Status = "blocked"
	} else if len(v.Attempts) > 0 {
		v.Status = "not_reproduced"
	} else {
		v.Status = "draft"
	}
}
func (s *Store) event(v *Scenario, kind, actor, detail string) {
	now := s.now().UTC()
	v.Events = append(v.Events, Event{Sequence: int64(len(v.Events) + 1), Kind: kind, ActorID: actor, Detail: detail, CreatedAt: now})
	v.UpdatedAt = now
}
func (s *Store) read(x string) (Scenario, error) {
	b, e := os.ReadFile(filepath.Join(s.root, x+".json"))
	if e != nil {
		return Scenario{}, e
	}
	var v Scenario
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Scenario) error {
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
