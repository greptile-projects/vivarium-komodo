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
type ScenarioInput struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Value          string `json:"value,omitempty"`
	SourceRevision string `json:"source_revision,omitempty"`
}
type Fixture struct {
	Name           string `json:"name"`
	Reference      string `json:"reference"`
	Classification string `json:"classification"`
	Transformation string `json:"transformation,omitempty"`
}
type ScenarioDefinition struct {
	Title                   string          `json:"title"`
	ExpectedBehavior        string          `json:"expected_behavior"`
	RegressedBehavior       string          `json:"regressed_behavior"`
	Inputs                  []ScenarioInput `json:"inputs"`
	Commands                []string        `json:"commands"`
	Fixtures                []Fixture       `json:"fixtures"`
	EnvironmentRequirements []string        `json:"environment_requirements"`
	TimeoutSeconds          int64           `json:"timeout_seconds"`
	CostLimit               float64         `json:"cost_limit"`
}
type Scenario struct {
	ID                   string             `json:"id"`
	Version              int64              `json:"version"`
	InvestigationVersion int64              `json:"investigation_version"`
	Derived              bool               `json:"derived"`
	Definition           ScenarioDefinition `json:"definition"`
	CreatedByID          string             `json:"created_by_id"`
	CreatedAt            time.Time          `json:"created_at"`
}
type Target struct {
	Kind              string            `json:"kind"`
	Reference         string            `json:"reference,omitempty"`
	CommitID          string            `json:"commit_id,omitempty"`
	ReleaseID         string            `json:"release_id,omitempty"`
	AttestationDigest string            `json:"attestation_digest,omitempty"`
	Dependencies      map[string]string `json:"dependencies,omitempty"`
}
type Environment struct {
	Image                string            `json:"image"`
	DefinitionDigest     string            `json:"definition_digest"`
	OS                   string            `json:"os"`
	Architecture         string            `json:"architecture"`
	Isolation            string            `json:"isolation"`
	Network              string            `json:"network"`
	Toolchain            map[string]string `json:"toolchain"`
	DependencyLockDigest string            `json:"dependency_lock_digest,omitempty"`
	SetupCommands        []string          `json:"setup_commands"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}
type Provenance struct {
	RunnerID        string `json:"runner_id"`
	RunnerVersion   string `json:"runner_version"`
	ActorKind       string `json:"actor_kind"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at"`
	RepetitionCount int64  `json:"repetition_count"`
}
type AttemptInput struct {
	Target         Target          `json:"target"`
	Environment    Environment     `json:"environment"`
	Inputs         []ScenarioInput `json:"inputs"`
	Commands       []string        `json:"commands"`
	Outputs        []string        `json:"outputs"`
	Logs           []string        `json:"logs"`
	Artifacts      []Artifact      `json:"artifacts"`
	Classification string          `json:"classification"`
	Rationale      string          `json:"rationale"`
	Cost           float64         `json:"cost"`
	Currency       string          `json:"currency"`
	Provenance     Provenance      `json:"provenance"`
}
type Attempt struct {
	ID              string `json:"id"`
	ScenarioID      string `json:"scenario_id"`
	ScenarioVersion int64  `json:"scenario_version"`
	AttemptInput
	ActorID   string    `json:"actor_id"`
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
	Scenarios    []Scenario    `json:"scenarios"`
	Attempts     []Attempt     `json:"attempts"`
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
			v.Scenarios = nil
			v.Attempts = nil
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
func (s *Store) CreateScenario(repo, key, actor string, derived bool, d ScenarioDefinition) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	d.Title, d.ExpectedBehavior, d.RegressedBehavior = strings.TrimSpace(d.Title), strings.TrimSpace(d.ExpectedBehavior), strings.TrimSpace(d.RegressedBehavior)
	d.Commands, d.EnvironmentRequirements = clean(d.Commands), clean(d.EnvironmentRequirements)
	if derived {
		if d.ExpectedBehavior == "" {
			d.ExpectedBehavior = v.Scope.ExpectedBehavior
		}
		if d.RegressedBehavior == "" {
			d.RegressedBehavior = v.Scope.RegressedBehavior
		}
	}
	if d.Title == "" || d.ExpectedBehavior == "" || d.RegressedBehavior == "" || len(d.Commands) == 0 || len(d.EnvironmentRequirements) == 0 || d.TimeoutSeconds < 1 || d.TimeoutSeconds > 3600 || d.CostLimit < 0 || !validFixtures(d.Fixtures) || !validInputs(d.Inputs) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Scenarios = append(v.Scenarios, Scenario{ID: id(), Version: 1, InvestigationVersion: v.Version, Derived: derived, Definition: d, CreatedByID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddAttempt(repo, key, scenario, actor string, in AttemptInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var sc *Scenario
	for i := range v.Scenarios {
		if v.Scenarios[i].ID == scenario {
			sc = &v.Scenarios[i]
			break
		}
	}
	if sc == nil {
		return Investigation{}, ErrNotFound
	}
	in.Commands, in.Outputs, in.Logs, in.Environment.SetupCommands = clean(in.Commands), clean(in.Outputs), clean(in.Logs), clean(in.Environment.SetupCommands)
	in.Rationale, in.Currency = strings.TrimSpace(in.Rationale), strings.TrimSpace(in.Currency)
	if !validAttempt(in, *sc) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Attempts = append(v.Attempts, Attempt{ID: id(), ScenarioID: sc.ID, ScenarioVersion: sc.Version, AttemptInput: in, ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func validFixtures(v []Fixture) bool {
	if len(v) == 0 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x.Name) == "" || strings.TrimSpace(x.Reference) == "" || !map[string]bool{"synthetic": true, "explicitly_permitted": true, "unsafe": true}[x.Classification] {
			return false
		}
	}
	return true
}
func validInputs(v []ScenarioInput) bool {
	for _, x := range v {
		if strings.TrimSpace(x.Name) == "" || !map[string]bool{"string": true, "number": true, "boolean": true, "artifact_reference": true}[x.Kind] {
			return false
		}
	}
	return true
}
func validAttempt(v AttemptInput, s Scenario) bool {
	classes := map[string]bool{"expected_behavior": true, "regressed_behavior": true, "incompatible_setup": true, "missing_dependencies": true, "flaky": true, "unsafe_fixture": true, "untestable_revision": true}
	targets := map[string]bool{"revision": true, "release": true, "dependency_combination": true}
	if !classes[v.Classification] || !targets[v.Target.Kind] || v.Rationale == "" || v.Cost < 0 || v.Currency == "" || v.Environment.Image == "" || v.Environment.DefinitionDigest == "" || v.Environment.OS == "" || v.Environment.Architecture == "" || v.Environment.Isolation != "isolated" || v.Environment.Network == "unrestricted" || len(v.Commands) == 0 || !validInputs(v.Inputs) || v.Provenance.RunnerID == "" || v.Provenance.RunnerVersion == "" || !map[string]bool{"human": true, "agent": true, "system": true}[v.Provenance.ActorKind] || v.Provenance.StartedAt == "" || v.Provenance.CompletedAt == "" || v.Provenance.RepetitionCount < 1 {
		return false
	}
	if v.Cost > s.Definition.CostLimit || v.Target.CommitID == "" || (v.Target.Kind == "dependency_combination" && len(v.Target.Dependencies) == 0) || (v.Target.Kind == "release" && (v.Target.ReleaseID == "" || v.Target.AttestationDigest == "")) {
		return false
	}
	if v.Classification == "flaky" && v.Provenance.RepetitionCount < 2 {
		return false
	}
	for _, f := range s.Definition.Fixtures {
		if f.Classification == "unsafe" && v.Classification != "unsafe_fixture" {
			return false
		}
	}
	retained := append(append(append([]string{}, v.Commands...), v.Environment.SetupCommands...), v.Outputs...)
	retained = append(retained, v.Logs...)
	for _, x := range v.Inputs {
		retained = append(retained, x.Name, x.Value)
	}
	for _, x := range retained {
		if len(x) > 10000 || credentialShaped(x) {
			return false
		}
	}
	for _, a := range v.Artifacts {
		if a.Name == "" || a.Digest == "" || a.MediaType == "" || a.Size < 0 {
			return false
		}
	}
	return true
}

func credentialShaped(v string) bool {
	x := strings.ToLower(v)
	for _, marker := range []string{"-----begin private key-----", "authorization: bearer ", "password=", "secret=", "api_key=", "access_token="} {
		if strings.Contains(x, marker) {
			return true
		}
	}
	return false
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
