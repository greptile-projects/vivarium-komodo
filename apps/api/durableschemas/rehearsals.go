package durableschemas

import (
	"sort"
	"strings"
	"time"
)

// Rehearsal is evidence about an isolated migration simulation. It deliberately
// stores only bounded metadata and sanitized output, never fixture contents.
type RehearsalInput struct {
	Title                  string            `json:"title"`
	ApplicationRevisions   map[string]string `json:"application_revisions"`
	MigrationRevision      string            `json:"migration_revision"`
	DefinitionPath         string            `json:"definition_path"`
	DefinitionDigest       string            `json:"definition_digest"`
	Dataset                Dataset           `json:"dataset"`
	Dependencies           map[string]string `json:"dependencies"`
	Checks                 []RehearsalCheck  `json:"checks"`
	MaximumDurationSeconds int64             `json:"maximum_duration_seconds"`
	MaximumCost            float64           `json:"maximum_cost"`
	Currency               string            `json:"currency"`
}

type Dataset struct {
	Kind          string `json:"kind"`
	Generator     string `json:"generator"`
	ShapeDigest   string `json:"shape_digest"`
	PrivacyMethod string `json:"privacy_method"`
	RowCount      int64  `json:"row_count"`
	ObjectCount   int64  `json:"object_count"`
	ByteCount     int64  `json:"byte_count"`
}

type RehearsalCheck struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	Command          string   `json:"command"`
	InputKeys        []string `json:"input_keys"`
	Expected         []string `json:"expected"`
	RollbackPossible bool     `json:"rollback_possible"`
}

type Count struct {
	Name   string `json:"name"`
	Before int64  `json:"before"`
	After  int64  `json:"after"`
}
type Invariant struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}
type Performance struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	Limit float64 `json:"limit"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}

type CheckResult struct {
	CheckID      string        `json:"check_id"`
	Status       string        `json:"status"`
	SanitizedLog string        `json:"sanitized_log"`
	Redacted     bool          `json:"redacted"`
	Counts       []Count       `json:"counts"`
	Invariants   []Invariant   `json:"invariants"`
	Performance  []Performance `json:"performance"`
	Artifacts    []Artifact    `json:"artifacts"`
	DurationMS   int64         `json:"duration_ms"`
	Cost         float64       `json:"cost"`
	Stale        bool          `json:"stale"`
	StaleInputs  []string      `json:"stale_inputs"`
}

type AttemptInput struct {
	ExpectedVersion int64             `json:"expected_version"`
	InputDigests    map[string]string `json:"input_digests"`
	Results         []CheckResult     `json:"results"`
	Attestation     string            `json:"attestation"`
}
type Attempt struct {
	ID           string            `json:"id"`
	Number       int               `json:"number"`
	InputDigests map[string]string `json:"input_digests"`
	Results      []CheckResult     `json:"results"`
	Attestation  string            `json:"attestation"`
	ActorID      string            `json:"actor_id"`
	CreatedAt    time.Time         `json:"created_at"`
}
type RehearsalAttestation struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	AttemptID string    `json:"attempt_id"`
	Stale     bool      `json:"stale"`
	CreatedAt time.Time `json:"created_at"`
}
type InvestigationNote struct {
	ID          string    `json:"id"`
	ActorKind   string    `json:"actor_kind"`
	ActorID     string    `json:"actor_id"`
	AttemptID   string    `json:"attempt_id"`
	CheckID     string    `json:"check_id"`
	Body        string    `json:"body"`
	Evidence    []string  `json:"evidence"`
	Uncertainty string    `json:"uncertainty"`
	CreatedAt   time.Time `json:"created_at"`
}
type Rehearsal struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	MigrationID  string `json:"migration_id"`
	SchemaID     string `json:"schema_id"`
	FromVersion  int64  `json:"from_version"`
	ToVersion    int64  `json:"to_version"`
	Version      int64  `json:"version"`
	RehearsalInput
	Attempts      []Attempt              `json:"attempts"`
	Attestations  []RehearsalAttestation `json:"attestations"`
	Investigation []InvestigationNote    `json:"investigation"`
	Blockers      []string               `json:"blockers"`
	Authority     []string               `json:"authority"`
	CreatedBy     string                 `json:"created_by"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

func validRehearsal(in RehearsalInput) bool {
	if in.Title == "" || len(in.ApplicationRevisions) == 0 || in.MigrationRevision == "" || in.DefinitionPath == "" || in.DefinitionDigest == "" || len(in.Dependencies) == 0 || in.MaximumDurationSeconds < 1 || in.MaximumCost < 0 || in.Currency == "" || !map[string]bool{"synthetic": true, "privacy_preserving_representative": true}[in.Dataset.Kind] || in.Dataset.Generator == "" || in.Dataset.ShapeDigest == "" || in.Dataset.PrivacyMethod == "" || in.Dataset.RowCount < 0 || in.Dataset.ObjectCount < 0 || in.Dataset.ByteCount < 0 {
		return false
	}
	if len(in.Checks) == 0 || len(in.Checks) > 50 {
		return false
	}
	seen, inputs := map[string]bool{}, canonicalInputs(in)
	for _, c := range in.Checks {
		if c.ID == "" || seen[c.ID] || c.Command == "" || !nonempty(c.InputKeys, true) || !nonempty(c.Expected, true) || !map[string]bool{"upgrade": true, "dual_read": true, "dual_write": true, "backfill": true, "validation": true, "rollback": true, "failure_injection": true}[c.Kind] {
			return false
		}
		for _, key := range c.InputKeys {
			if inputs[key] == "" {
				return false
			}
		}
		seen[c.ID] = true
	}
	return true
}

func canonicalInputs(in RehearsalInput) map[string]string {
	x := map[string]string{"migration": in.MigrationRevision, "definition": in.DefinitionDigest, "data_shape": in.Dataset.ShapeDigest}
	for k, v := range in.ApplicationRevisions {
		x["application:"+k] = v
	}
	for k, v := range in.Dependencies {
		x["dependency:"+k] = v
	}
	return x
}

func redactLog(v string) (string, bool) {
	lines, redacted := strings.Split(v, "\n"), false
	markers := []string{"authorization", "password", "secret", "token", "cookie", "private_key", "ghp_", "github_pat_", "-----begin private key"}
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, m := range markers {
			if strings.Contains(lower, m) {
				lines[i] = "[redacted sensitive output]"
				redacted = true
				break
			}
		}
	}
	return strings.Join(lines, "\n"), redacted
}

func deriveRehearsal(x Rehearsal) Rehearsal {
	x.Blockers = nil
	current := canonicalInputs(x.RehearsalInput)
	passed := map[string]bool{}
	if len(x.Attempts) == 0 {
		x.Blockers = append(x.Blockers, "rehearsal_not_run")
	}
	for ai := range x.Attempts {
		for ri := range x.Attempts[ai].Results {
			r := &x.Attempts[ai].Results[ri]
			r.Stale = false
			r.StaleInputs = nil
			var check *RehearsalCheck
			for i := range x.Checks {
				if x.Checks[i].ID == r.CheckID {
					check = &x.Checks[i]
					break
				}
			}
			if check == nil {
				r.Stale = true
				r.StaleInputs = []string{"check_definition"}
			} else {
				for _, k := range check.InputKeys {
					if x.Attempts[ai].InputDigests[k] != current[k] {
						r.Stale = true
						r.StaleInputs = append(r.StaleInputs, k)
					}
				}
			}
			if !r.Stale {
				passed[r.CheckID] = r.Status == "passed"
			}
		}
	}
	for i := range x.Attestations {
		x.Attestations[i].Stale = true
		for _, attempt := range x.Attempts {
			if attempt.ID != x.Attestations[i].AttemptID {
				continue
			}
			x.Attestations[i].Stale = false
			for _, result := range attempt.Results {
				x.Attestations[i].Stale = x.Attestations[i].Stale || result.Stale
			}
		}
	}
	for _, c := range x.Checks {
		if !passed[c.ID] {
			x.Blockers = append(x.Blockers, "current_check_required:"+c.ID)
		}
	}
	sort.Strings(x.Blockers)
	x.Authority = []string{}
	return x
}

func (s *Store) CreateRehearsal(repo, migration, actor string, in RehearsalInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if !validRehearsal(in) {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	r := Rehearsal{ID: id(), RepositoryID: repo, MigrationID: migration, SchemaID: x.SchemaID, FromVersion: x.FromVersion, ToVersion: x.ToVersion, Version: 1, RehearsalInput: in, CreatedBy: actor, CreatedAt: now, UpdatedAt: now}
	x.Rehearsals = append(x.Rehearsals, r)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "rehearsal_created", ActorID: actor, Detail: r.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func rehearsalIndex(x Migration, id string) int {
	for i := range x.Rehearsals {
		if x.Rehearsals[i].ID == id {
			return i
		}
	}
	return -1
}
func (s *Store) UpdateRehearsalInputs(repo, migration, rehearsal, actor string, expected int64, apps, deps map[string]string, migrationRev, definitionDigest, dataShape string) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := rehearsalIndex(x, rehearsal)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	r := &x.Rehearsals[i]
	if r.Version != expected {
		return Migration{}, ErrConflict
	}
	if len(apps) > 0 {
		r.ApplicationRevisions = apps
	}
	if len(deps) > 0 {
		r.Dependencies = deps
	}
	if migrationRev != "" {
		r.MigrationRevision = migrationRev
	}
	if definitionDigest != "" {
		r.DefinitionDigest = definitionDigest
	}
	if dataShape != "" {
		r.Dataset.ShapeDigest = dataShape
	}
	if !validRehearsal(r.RehearsalInput) {
		return Migration{}, ErrInvalid
	}
	r.Version++
	r.UpdatedAt = s.now().UTC()
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "rehearsal_inputs_changed", ActorID: actor, Detail: r.ID, CreatedAt: r.UpdatedAt})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) RecordAttempt(repo, migration, rehearsal, actor string, in AttemptInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := rehearsalIndex(x, rehearsal)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	r := &x.Rehearsals[i]
	if r.Version != in.ExpectedVersion || in.Attestation == "" || len(in.Results) == 0 {
		return Migration{}, ErrConflict
	}
	checks := map[string]bool{}
	for _, c := range r.Checks {
		checks[c.ID] = true
	}
	seen := map[string]bool{}
	totalCost := 0.0
	totalMS := int64(0)
	for j := range in.Results {
		q := &in.Results[j]
		if !checks[q.CheckID] || seen[q.CheckID] || !map[string]bool{"passed": true, "failed": true, "error": true, "skipped": true}[q.Status] || q.DurationMS < 0 || q.Cost < 0 {
			return Migration{}, ErrInvalid
		}
		seen[q.CheckID] = true
		q.SanitizedLog, q.Redacted = redactLog(q.SanitizedLog)
		for _, a := range q.Artifacts {
			if a.Name == "" || a.Digest == "" || a.MediaType == "" || a.Size < 0 {
				return Migration{}, ErrInvalid
			}
		}
		totalCost += q.Cost
		totalMS += q.DurationMS
	}
	if totalCost > r.MaximumCost || totalMS > r.MaximumDurationSeconds*1000 {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	a := Attempt{ID: id(), Number: len(r.Attempts) + 1, InputDigests: in.InputDigests, Results: in.Results, Attestation: in.Attestation, ActorID: actor, CreatedAt: now}
	r.Attempts = append(r.Attempts, a)
	r.Version++
	r.UpdatedAt = now
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "rehearsal_attempt_recorded", ActorID: actor, Detail: a.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) AttestRehearsal(repo, migration, rehearsal, actor, attempt, decision, rationale string) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := rehearsalIndex(x, rehearsal)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	if !map[string]bool{"accepted": true, "rejected": true}[decision] || rationale == "" {
		return Migration{}, ErrInvalid
	}
	found := false
	for _, a := range x.Rehearsals[i].Attempts {
		found = found || a.ID == attempt
	}
	if !found {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	x.Rehearsals[i].Attestations = append(x.Rehearsals[i].Attestations, RehearsalAttestation{ActorID: actor, Decision: decision, Rationale: rationale, AttemptID: attempt, CreatedAt: now})
	x.Rehearsals[i].Version++
	x.Rehearsals[i].UpdatedAt = now
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) AddInvestigation(repo, migration, rehearsal, actor string, in InvestigationNote) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := rehearsalIndex(x, rehearsal)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	if !map[string]bool{"human": true, "agent": true}[in.ActorKind] || in.Body == "" || in.Uncertainty == "" || !nonempty(in.Evidence, true) {
		return Migration{}, ErrInvalid
	}
	if (in.ActorKind == "agent") != strings.HasPrefix(actor, "agent:") {
		return Migration{}, ErrInvalid
	}
	found := false
	for _, a := range x.Rehearsals[i].Attempts {
		if a.ID == in.AttemptID {
			for _, q := range a.Results {
				found = found || q.CheckID == in.CheckID
			}
		}
	}
	if !found {
		return Migration{}, ErrInvalid
	}
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	x.Rehearsals[i].Investigation = append(x.Rehearsals[i].Investigation, in)
	x.Rehearsals[i].Version++
	x.Rehearsals[i].UpdatedAt = in.CreatedAt
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}
