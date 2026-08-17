// Package durableschemas owns reviewed durable-state contracts and migration plans.
package durableschemas

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

var (
	ErrNotFound = errors.New("durable schema not found")
	ErrInvalid  = errors.New("invalid durable schema")
	ErrConflict = errors.New("durable schema version conflict")
)

type Field struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Required       bool   `json:"required"`
	Classification string `json:"classification"`
	Description    string `json:"description"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label"`
}
type VersionInput struct {
	Name               string   `json:"name"`
	StoreKind          string   `json:"store_kind"`
	Description        string   `json:"description"`
	SourceRevision     string   `json:"source_revision"`
	DefinitionPath     string   `json:"definition_path"`
	Format             string   `json:"format"`
	Fields             []Field  `json:"fields"`
	OwnerIDs           []string `json:"owner_ids"`
	Compatibility      string   `json:"compatibility"`
	Retention          string   `json:"retention"`
	PrivacyCommitments []string `json:"privacy_commitments"`
	Links              []Link   `json:"links"`
	ChangeReason       string   `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Schema struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []string  `json:"gaps"`
}

type Operation struct {
	Kind        string `json:"kind"`
	Object      string `json:"object"`
	Description string `json:"description"`
	Destructive bool   `json:"destructive"`
	Reversible  bool   `json:"reversible"`
}
type Step struct {
	ID             string   `json:"id"`
	Description    string   `json:"description"`
	DependsOn      []string `json:"depends_on"`
	OperationKinds []string `json:"operation_kinds"`
	OwnerID        string   `json:"owner_id"`
}
type Approval struct {
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
type MigrationInput struct {
	Title               string      `json:"title"`
	SourceKind          string      `json:"source_kind"`
	SourceID            string      `json:"source_id"`
	SchemaID            string      `json:"schema_id"`
	FromVersion         int64       `json:"from_version"`
	ToVersion           int64       `json:"to_version"`
	Operations          []Operation `json:"operations"`
	AffectedConsumers   []string    `json:"affected_consumers"`
	RollbackLimits      []string    `json:"rollback_limits"`
	Steps               []Step      `json:"steps"`
	SuccessMeasures     []string    `json:"success_measures"`
	RequiredApproverIDs []string    `json:"required_approver_ids"`
	Summary             string      `json:"summary"`
}

// WorkItem is a migration-scoped coordination record. ResourceID points at an
// ordinary repository task, agent session, or workspace; this record neither
// creates credentials nor copies context from the schema definition.
type WorkItem struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	Phase              string    `json:"phase"`
	RepositoryID       string    `json:"repository_id"`
	ResourceID         string    `json:"resource_id"`
	OwnerKind          string    `json:"owner_kind"`
	OwnerID            string    `json:"owner_id"`
	Position           int       `json:"position"`
	DependsOn          []string  `json:"depends_on"`
	BaseRevision       string    `json:"base_revision"`
	AllowedPaths       []string  `json:"allowed_paths"`
	Context            []string  `json:"context"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}
type WorkItemInput struct {
	Kind               string   `json:"kind"`
	Phase              string   `json:"phase"`
	RepositoryID       string   `json:"repository_id"`
	ResourceID         string   `json:"resource_id"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	DependsOn          []string `json:"depends_on"`
	BaseRevision       string   `json:"base_revision"`
	AllowedPaths       []string `json:"allowed_paths"`
	Context            []string `json:"context"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// PullContract freezes the coexistence assumptions reviewed with an ordinary
// pull request. Repository permissions remain authoritative for the pull.
type PullContract struct {
	ID                  string    `json:"id"`
	RepositoryID        string    `json:"repository_id"`
	PullRequestID       string    `json:"pull_request_id"`
	Revision            string    `json:"revision"`
	WorkItemIDs         []string  `json:"work_item_ids"`
	OldReaders          []string  `json:"old_readers"`
	NewReaders          []string  `json:"new_readers"`
	OldWriters          []string  `json:"old_writers"`
	NewWriters          []string  `json:"new_writers"`
	RolloutFlags        []string  `json:"rollout_flags"`
	Idempotency         string    `json:"idempotency"`
	DataTransformations []string  `json:"data_transformations"`
	OwnerIDs            []string  `json:"owner_ids"`
	RollbackAssumptions []string  `json:"rollback_assumptions"`
	CreatedBy           string    `json:"created_by"`
	CreatedAt           time.Time `json:"created_at"`
}
type PullContractInput struct {
	RepositoryID        string   `json:"repository_id"`
	PullRequestID       string   `json:"pull_request_id"`
	Revision            string   `json:"revision"`
	WorkItemIDs         []string `json:"work_item_ids"`
	OldReaders          []string `json:"old_readers"`
	NewReaders          []string `json:"new_readers"`
	OldWriters          []string `json:"old_writers"`
	NewWriters          []string `json:"new_writers"`
	RolloutFlags        []string `json:"rollout_flags"`
	Idempotency         string   `json:"idempotency"`
	DataTransformations []string `json:"data_transformations"`
	OwnerIDs            []string `json:"owner_ids"`
	RollbackAssumptions []string `json:"rollback_assumptions"`
}
type Migration struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	MigrationInput
	CreatedBy     string         `json:"created_by"`
	CreatedAt     time.Time      `json:"created_at"`
	Approvals     []Approval     `json:"approvals"`
	Events        []Event        `json:"events"`
	WorkItems     []WorkItem     `json:"work_items"`
	PullContracts []PullContract `json:"pull_contracts"`
	Blockers      []string       `json:"blockers"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) schemaPath(repo, schema string) string {
	return filepath.Join(s.root, repo, "schemas", schema+".json")
}
func (s *Store) migrationPath(repo, migration string) string {
	return filepath.Join(s.root, repo, "migrations", migration+".json")
}
func save(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func load(path string, v any) error {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, v)
	}
	return e
}
func nonempty(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return len(xs) <= 100
}
func validVersion(v VersionInput) bool {
	if v.Name == "" || !map[string]bool{"database": true, "queue": true, "index": true, "object_store": true, "cache": true, "event_log": true, "other": true}[v.StoreKind] || v.Description == "" || v.SourceRevision == "" || v.DefinitionPath == "" || v.Format == "" || v.Compatibility == "" || v.Retention == "" || v.ChangeReason == "" || !nonempty(v.OwnerIDs, true) || !nonempty(v.PrivacyCommitments, true) {
		return false
	}
	seen := map[string]bool{}
	for _, f := range v.Fields {
		if f.Name == "" || f.Type == "" || f.Description == "" || seen[f.Name] || !map[string]bool{"public": true, "internal": true, "personal": true, "sensitive": true, "restricted": true}[f.Classification] {
			return false
		}
		seen[f.Name] = true
	}
	if len(v.Fields) == 0 {
		return false
	}
	for _, l := range v.Links {
		if !map[string]bool{"service": true, "environment": true, "privacy": true, "documentation": true}[l.Kind] || l.ResourceID == "" || l.Label == "" {
			return false
		}
	}
	return true
}
func deriveSchema(x Schema) Schema {
	x.Gaps = nil
	if len(x.Versions) == 0 {
		return x
	}
	v := x.Versions[len(x.Versions)-1]
	service, env := false, false
	for _, l := range v.Links {
		service = service || l.Kind == "service"
		env = env || l.Kind == "environment"
	}
	if !service {
		x.Gaps = append(x.Gaps, "missing_service_link")
	}
	if !env {
		x.Gaps = append(x.Gaps, "missing_environment_link")
	}
	return x
}
func (s *Store) CreateSchema(repo, actor string, in VersionInput) (Schema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validVersion(in) {
		return Schema{}, ErrInvalid
	}
	x := Schema{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}}}
	return deriveSchema(x), save(s.schemaPath(repo, x.ID), x)
}
func (s *Store) GetSchema(repo, schema string) (Schema, error) {
	var x Schema
	e := load(s.schemaPath(repo, schema), &x)
	return deriveSchema(x), e
}
func (s *Store) ListSchemas(repo string) ([]Schema, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "schemas", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Schema{}
	for _, p := range paths {
		var x Schema
		if e = load(p, &x); e != nil {
			return nil, e
		}
		out = append(out, deriveSchema(x))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].Name < out[j].Versions[len(out[j].Versions)-1].Name
	})
	return out, nil
}
func (s *Store) ReviseSchema(repo, schema, actor string, expected int64, in VersionInput) (Schema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validVersion(in) {
		return Schema{}, ErrInvalid
	}
	var x Schema
	if e := load(s.schemaPath(repo, schema), &x); e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return Schema{}, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return deriveSchema(x), save(s.schemaPath(repo, schema), x)
}
func validMigration(in MigrationInput, schema Schema) bool {
	if in.Title == "" || !map[string]bool{"pull_request": true, "decision": true}[in.SourceKind] || in.SourceID == "" || in.SchemaID != schema.ID || in.FromVersion < 1 || in.ToVersion < 1 || in.FromVersion == in.ToVersion || in.FromVersion > schema.CurrentVersion || in.ToVersion > schema.CurrentVersion || !nonempty(in.AffectedConsumers, true) || !nonempty(in.RollbackLimits, true) || !nonempty(in.SuccessMeasures, true) || !nonempty(in.RequiredApproverIDs, true) || in.Summary == "" {
		return false
	}
	kinds := map[string]bool{"read": true, "write": true, "backfill": true, "destructive": true}
	seenKinds := map[string]bool{}
	for _, o := range in.Operations {
		if !kinds[o.Kind] || o.Object == "" || o.Description == "" || (o.Kind == "destructive" && !o.Destructive) {
			return false
		}
		seenKinds[o.Kind] = true
	}
	if len(in.Operations) == 0 {
		return false
	}
	steps := map[string]bool{}
	for _, x := range in.Steps {
		if x.ID == "" || x.Description == "" || x.OwnerID == "" || steps[x.ID] || !nonempty(x.OperationKinds, true) {
			return false
		}
		for _, k := range x.OperationKinds {
			if !seenKinds[k] {
				return false
			}
		}
		steps[x.ID] = true
	}
	if len(in.Steps) == 0 {
		return false
	}
	for _, x := range in.Steps {
		for _, d := range x.DependsOn {
			if !steps[d] || d == x.ID {
				return false
			}
		}
	}
	return true
}
func deriveMigration(x Migration) Migration {
	x.Blockers = nil
	approved := map[string]bool{}
	for _, a := range x.Approvals {
		if a.Decision == "approved" {
			approved[a.OwnerID] = true
		} else if a.Decision == "rejected" {
			x.Blockers = append(x.Blockers, "approval_rejected:"+a.OwnerID)
		}
	}
	for _, o := range x.RequiredApproverIDs {
		if !approved[o] {
			x.Blockers = append(x.Blockers, "approval_required:"+o)
		}
	}
	for _, op := range x.Operations {
		if op.Destructive && !op.Reversible && len(x.RollbackLimits) == 0 {
			x.Blockers = append(x.Blockers, "irreversible_operation_without_rollback_limit")
		}
	}
	for _, w := range x.WorkItems {
		for _, dependency := range w.DependsOn {
			found := false
			for _, candidate := range x.WorkItems {
				found = found || candidate.ID == dependency
			}
			if !found {
				x.Blockers = append(x.Blockers, "work_dependency_missing:"+w.ID+":"+dependency)
			}
		}
	}
	sort.Strings(x.Blockers)
	return x
}

func validWorkItem(in WorkItemInput, x Migration) bool {
	if !map[string]bool{"task": true, "session": true, "workspace": true}[in.Kind] || !map[string]bool{"schema": true, "compatibility": true, "backfill": true, "verification": true, "cleanup": true}[in.Phase] || in.RepositoryID == "" || in.ResourceID == "" || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || in.OwnerID == "" || in.BaseRevision == "" || !nonempty(in.AllowedPaths, true) || !nonempty(in.AcceptanceCriteria, true) || !nonempty(in.Context, false) {
		return false
	}
	seen := map[string]bool{}
	for _, w := range x.WorkItems {
		seen[w.ID] = true
		if w.RepositoryID == in.RepositoryID && w.Kind == in.Kind && w.ResourceID == in.ResourceID {
			return false
		}
	}
	for _, dependency := range in.DependsOn {
		if !seen[dependency] {
			return false
		}
	}
	return true
}

func (s *Store) AddWorkItem(repo, migration, actor string, in WorkItemInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if !validWorkItem(in, x) {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	w := WorkItem{ID: id(), Kind: in.Kind, Phase: in.Phase, RepositoryID: in.RepositoryID, ResourceID: in.ResourceID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, Position: len(x.WorkItems) + 1, DependsOn: append([]string{}, in.DependsOn...), BaseRevision: in.BaseRevision, AllowedPaths: append([]string{}, in.AllowedPaths...), Context: append([]string{}, in.Context...), AcceptanceCriteria: append([]string{}, in.AcceptanceCriteria...), CreatedBy: actor, CreatedAt: now}
	x.WorkItems = append(x.WorkItems, w)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "work_item_created", ActorID: actor, Detail: w.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func validPullContract(in PullContractInput, x Migration) bool {
	if in.RepositoryID == "" || in.PullRequestID == "" || in.Revision == "" || in.Idempotency == "" || !nonempty(in.WorkItemIDs, true) || !nonempty(in.OldReaders, true) || !nonempty(in.NewReaders, true) || !nonempty(in.OldWriters, true) || !nonempty(in.NewWriters, true) || !nonempty(in.RolloutFlags, true) || !nonempty(in.DataTransformations, true) || !nonempty(in.OwnerIDs, true) || !nonempty(in.RollbackAssumptions, true) {
		return false
	}
	work := map[string]bool{}
	for _, w := range x.WorkItems {
		work[w.ID] = true
	}
	for _, wid := range in.WorkItemIDs {
		if !work[wid] {
			return false
		}
	}
	for _, p := range x.PullContracts {
		if p.RepositoryID == in.RepositoryID && p.PullRequestID == in.PullRequestID && p.Revision == in.Revision {
			return false
		}
	}
	return true
}

func (s *Store) AddPullContract(repo, migration, actor string, in PullContractInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if !validPullContract(in, x) {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	p := PullContract{ID: id(), RepositoryID: in.RepositoryID, PullRequestID: in.PullRequestID, Revision: in.Revision, WorkItemIDs: append([]string{}, in.WorkItemIDs...), OldReaders: append([]string{}, in.OldReaders...), NewReaders: append([]string{}, in.NewReaders...), OldWriters: append([]string{}, in.OldWriters...), NewWriters: append([]string{}, in.NewWriters...), RolloutFlags: append([]string{}, in.RolloutFlags...), Idempotency: in.Idempotency, DataTransformations: append([]string{}, in.DataTransformations...), OwnerIDs: append([]string{}, in.OwnerIDs...), RollbackAssumptions: append([]string{}, in.RollbackAssumptions...), CreatedBy: actor, CreatedAt: now}
	x.PullContracts = append(x.PullContracts, p)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "pull_contract_linked", ActorID: actor, Detail: p.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}
func (s *Store) CreateMigration(repo, actor string, in MigrationInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var schema Schema
	if e := load(s.schemaPath(repo, in.SchemaID), &schema); e != nil {
		return Migration{}, e
	}
	if !validMigration(in, schema) {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	x := Migration{ID: id(), RepositoryID: repo, MigrationInput: in, CreatedBy: actor, CreatedAt: now, Events: []Event{{Sequence: 1, Kind: "created", ActorID: actor, Detail: in.Summary, CreatedAt: now}}}
	return deriveMigration(x), save(s.migrationPath(repo, x.ID), x)
}
func (s *Store) GetMigration(repo, migration string) (Migration, error) {
	var x Migration
	e := load(s.migrationPath(repo, migration), &x)
	return deriveMigration(x), e
}
func (s *Store) ListMigrations(repo string) ([]Migration, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "migrations", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Migration{}
	for _, p := range paths {
		var x Migration
		if e = load(p, &x); e != nil {
			return nil, e
		}
		out = append(out, deriveMigration(x))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Approve(repo, migration, actor, owner, decision, rationale string) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner != actor || !map[string]bool{"approved": true, "rejected": true}[decision] || rationale == "" {
		return Migration{}, ErrInvalid
	}
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	required := false
	for _, o := range x.RequiredApproverIDs {
		required = required || o == owner
	}
	if !required {
		return Migration{}, ErrInvalid
	}
	for _, a := range x.Approvals {
		if a.OwnerID == owner {
			return Migration{}, ErrConflict
		}
	}
	now := s.now().UTC()
	x.Approvals = append(x.Approvals, Approval{OwnerID: owner, Decision: decision, Rationale: rationale, ActorID: actor, CreatedAt: now})
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "approval_" + decision, ActorID: actor, Detail: rationale, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}
