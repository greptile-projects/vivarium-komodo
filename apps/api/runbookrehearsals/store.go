// Package runbookrehearsals retains bounded proof that an exact runbook revision is usable.
package runbookrehearsals

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

var ErrNotFound = errors.New("runbook rehearsal not found")
var ErrInvalid = errors.New("invalid runbook rehearsal")
var ErrConflict = errors.New("runbook rehearsal changed")

type Limit struct {
	MaxDurationSeconds int64   `json:"max_duration_seconds"`
	MaxCost            float64 `json:"max_cost"`
	Currency           string  `json:"currency"`
}
type BoundReference struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
}
type Scenario struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Failure          string           `json:"failure"`
	EvidenceSource   string           `json:"evidence_source"`
	InputDigest      string           `json:"input_digest"`
	ExpectedOutcomes []string         `json:"expected_outcomes"`
	References       []BoundReference `json:"references"`
}
type Input struct {
	RunbookID           string     `json:"runbook_id"`
	RunbookVersion      int64      `json:"runbook_version"`
	Title               string     `json:"title"`
	EnvironmentID       string     `json:"environment_id"`
	EnvironmentRevision string     `json:"environment_revision"`
	EnvironmentClass    string     `json:"environment_class"`
	PolicyApprovalID    string     `json:"policy_approval_id,omitempty"`
	Limits              Limit      `json:"limits"`
	Scenarios           []Scenario `json:"scenarios"`
	OwnerIDs            []string   `json:"owner_ids"`
}
type Permission struct {
	Capability         string `json:"capability"`
	ResourceID         string `json:"resource_id"`
	Granted            bool   `json:"granted"`
	AuthorityReference string `json:"authority_reference,omitempty"`
}
type Branch struct {
	StepID    string `json:"step_id"`
	Question  string `json:"question"`
	Decision  string `json:"decision"`
	ActorID   string `json:"actor_id"`
	Rationale string `json:"rationale"`
}
type StepResult struct {
	StepID              string    `json:"step_id"`
	Status              string    `json:"status"`
	Command             string    `json:"command,omitempty"`
	Output              string    `json:"output,omitempty"`
	StartedAt           time.Time `json:"started_at"`
	EndedAt             time.Time `json:"ended_at"`
	ArtifactDigests     []string  `json:"artifact_digests"`
	Destructive         bool      `json:"destructive"`
	DestructiveHandling string    `json:"destructive_handling,omitempty"`
}
type AttemptInput struct {
	ExpectedRevision    int64        `json:"expected_revision"`
	ScenarioID          string       `json:"scenario_id"`
	ActorKind           string       `json:"actor_kind"`
	EnvironmentRevision string       `json:"environment_revision"`
	StartedAt           time.Time    `json:"started_at"`
	EndedAt             time.Time    `json:"ended_at"`
	InputDigest         string       `json:"input_digest"`
	Permissions         []Permission `json:"permissions"`
	Branches            []Branch     `json:"branches"`
	Steps               []StepResult `json:"steps"`
	AchievedOutcomes    []string     `json:"achieved_outcomes"`
	ManualGaps          []string     `json:"manual_gaps"`
	Cost                float64      `json:"cost"`
	Currency            string       `json:"currency"`
	Notes               string       `json:"notes,omitempty"`
}
type Gap struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	Reference string `json:"reference,omitempty"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
	Classification string    `json:"classification"`
	Proof          bool      `json:"proof"`
	Stale          bool      `json:"stale"`
	Gaps           []Gap     `json:"gaps"`
}
type ObservationInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Kind             string `json:"kind"`
	ResourceID       string `json:"resource_id"`
	PreviousRevision string `json:"previous_revision"`
	CurrentRevision  string `json:"current_revision"`
	Detail           string `json:"detail"`
}
type Observation struct {
	ID string `json:"id"`
	ObservationInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Rehearsal struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	AuthorID     string        `json:"author_id"`
	CreatedAt    time.Time     `json:"created_at"`
	Attempts     []Attempt     `json:"attempts"`
	Observations []Observation `json:"observations"`
	Ready        bool          `json:"ready"`
	Gaps         []Gap         `json:"gaps"`
	NonAuthority []string      `json:"non_authority"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func unique(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	m := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func valid(in Input) bool {
	if in.RunbookID == "" || in.RunbookVersion < 1 || strings.TrimSpace(in.Title) == "" || in.EnvironmentID == "" || in.EnvironmentRevision == "" || !map[string]bool{"isolated": true, "policy_approved": true}[in.EnvironmentClass] || in.EnvironmentClass == "policy_approved" && in.PolicyApprovalID == "" || in.Limits.MaxDurationSeconds < 1 || in.Limits.MaxCost <= 0 || in.Limits.Currency == "" || !unique(in.OwnerIDs, true) || len(in.Scenarios) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range in.Scenarios {
		if s.ID == "" || seen[s.ID] || s.Name == "" || s.Failure == "" || !map[string]bool{"synthetic": true, "permitted": true}[s.EvidenceSource] || s.InputDigest == "" || !unique(s.ExpectedOutcomes, true) {
			return false
		}
		seen[s.ID] = true
		for _, r := range s.References {
			if !map[string]bool{"service": true, "dependency": true, "credential": true, "policy": true, "runbook_step": true}[r.Kind] || r.ResourceID == "" || r.Revision == "" {
				return false
			}
		}
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Rehearsal, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Rehearsal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Rehearsal{ID: id(), RepositoryID: repo, Revision: 1, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	return r, s.write(r)
}
func validAttempt(r Rehearsal, a AttemptInput) bool {
	if a.ScenarioID == "" || !map[string]bool{"human": true, "agent": true}[a.ActorKind] || a.EnvironmentRevision == "" || a.StartedAt.IsZero() || !a.EndedAt.After(a.StartedAt) || a.InputDigest == "" || a.Cost < 0 || a.Currency != r.Limits.Currency || len(a.Steps) == 0 {
		return false
	}
	ok := false
	for _, x := range r.Scenarios {
		ok = ok || x.ID == a.ScenarioID
	}
	for _, x := range a.Steps {
		if x.StepID == "" || !map[string]bool{"completed": true, "failed": true, "skipped": true}[x.Status] || x.StartedAt.IsZero() || x.EndedAt.Before(x.StartedAt) || x.Destructive && !map[string]bool{"simulated": true, "excluded": true}[x.DestructiveHandling] {
			return false
		}
	}
	return ok
}
func scenario(r Rehearsal, id string) Scenario {
	for _, x := range r.Scenarios {
		if x.ID == id {
			return x
		}
	}
	return Scenario{}
}
func classify(r Rehearsal, a Attempt) Attempt {
	sc := scenario(r, a.ScenarioID)
	add := func(k, d, ref string) { a.Gaps = append(a.Gaps, Gap{k, d, ref}) }
	if a.EnvironmentRevision != r.EnvironmentRevision {
		add("environment_drift", "Attempt environment differs from the frozen environment.", a.EnvironmentRevision)
	}
	if a.InputDigest != sc.InputDigest {
		add("input_drift", "Attempt inputs differ from the selected scenario.", a.InputDigest)
	}
	if a.EndedAt.Sub(a.StartedAt) > time.Duration(r.Limits.MaxDurationSeconds)*time.Second || a.Cost > r.Limits.MaxCost {
		add("limit_exceeded", "Attempt exceeded its duration or cost bound.", a.ScenarioID)
	}
	for _, p := range a.Permissions {
		if !p.Granted {
			add("missing_permission", "Required permission was unavailable.", p.Capability+":"+p.ResourceID)
		}
		if p.Granted && p.AuthorityReference == "" {
			add("unattributed_permission", "Granted permission lacks an authority reference.", p.Capability)
		}
	}
	for _, x := range a.Steps {
		explicitlyExcluded := x.Destructive && x.DestructiveHandling == "excluded" && x.Status == "skipped"
		if x.Status != "completed" && !explicitlyExcluded {
			add("step_incomplete", "A runbook step did not complete.", x.StepID)
		}
		if x.Command == "" || x.Output == "" {
			add("missing_step_evidence", "Step command and output must be retained.", x.StepID)
		}
		if x.Destructive && x.DestructiveHandling == "" {
			add("unsafe_destructive_step", "Destructive work was neither simulated nor excluded.", x.StepID)
		}
	}
	if len(a.ManualGaps) > 0 {
		add("manual_gap", "The rehearsal required undocumented or manual work.", a.ScenarioID)
	}
	for _, want := range sc.ExpectedOutcomes {
		found := false
		for _, got := range a.AchievedOutcomes {
			found = found || got == want
		}
		if !found {
			add("outcome_not_achieved", "A promised scenario outcome was not demonstrated.", want)
		}
	}
	a.Proof = len(a.Gaps) == 0
	a.Classification = "valid"
	if !a.Proof {
		a.Classification = "incomplete"
	}
	return a
}
func (s *Store) AppendAttempt(repo, rid, actor string, in AttemptInput) (Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != in.ExpectedRevision {
		return r, ErrConflict
	}
	if actor == "" || !validAttempt(r, in) {
		return r, ErrInvalid
	}
	a := classify(r, Attempt{ID: id(), AttemptInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	r.Attempts = append(r.Attempts, a)
	r.Revision++
	return r, s.write(r)
}
func (s *Store) Observe(repo, rid, actor string, in ObservationInput) (Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != in.ExpectedRevision {
		return r, ErrConflict
	}
	if actor == "" || !map[string]bool{"service": true, "dependency": true, "credential": true, "policy": true, "runbook_step": true}[in.Kind] || in.ResourceID == "" || in.PreviousRevision == "" || in.CurrentRevision == "" || in.PreviousRevision == in.CurrentRevision || in.Detail == "" {
		return r, ErrInvalid
	}
	r.Observations = append(r.Observations, Observation{ID: id(), ObservationInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	r.Revision++
	return r, s.write(r)
}
func Resolve(r Rehearsal) Rehearsal {
	r.Gaps = nil
	r.NonAuthority = []string{"Runbook rehearsals grant no repository, secret, workflow, agent, communication, incident, deployment, environment, credential, or operational authority."}
	for i := range r.Attempts {
		a := &r.Attempts[i]
		a.Stale = false
		for _, o := range r.Observations {
			if o.Kind == "runbook_step" && o.ResourceID == "*" && o.PreviousRevision == stringInt(r.RunbookVersion) {
				a.Stale = true
			}
			for _, ref := range scenario(r, a.ScenarioID).References {
				if ref.Kind == o.Kind && ref.ResourceID == o.ResourceID && ref.Revision == o.PreviousRevision {
					a.Stale = true
				}
			}
		}
		if a.Stale {
			a.Proof = false
			a.Classification = "stale"
			a.Gaps = appendGap(a.Gaps, Gap{"stale_evidence", "A bound operational input changed after this attempt.", a.ScenarioID})
		}
	}
	proof := map[string]bool{}
	for _, a := range r.Attempts {
		if a.Proof {
			proof[a.ScenarioID] = true
		}
	}
	for _, sc := range r.Scenarios {
		if !proof[sc.ID] {
			r.Gaps = append(r.Gaps, Gap{"missing_current_proof", "Scenario lacks current complete proof.", sc.ID})
		}
	}
	r.Ready = len(r.Gaps) == 0
	return r
}
func stringInt(n int64) string { b, _ := json.Marshal(n); return string(b) }
func appendGap(xs []Gap, g Gap) []Gap {
	for _, x := range xs {
		if x.Kind == g.Kind && x.Reference == g.Reference {
			return xs
		}
	}
	return append(xs, g)
}
func (s *Store) Get(repo, rid string) (Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, rid)
}
func (s *Store) List(repo, runbook string) ([]Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Rehearsal{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var r Rehearsal
		if x == nil {
			x = json.Unmarshal(b, &r)
		}
		if x != nil {
			return nil, x
		}
		if r.RunbookID == runbook {
			out = append(out, r)
		}
	}
	return out, nil
}
func (s *Store) read(repo, rid string) (Rehearsal, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, rid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Rehearsal{}, ErrNotFound
	}
	var r Rehearsal
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	return r, e
}
func (s *Store) write(r Rehearsal) error {
	d := filepath.Join(s.root, r.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e == nil {
		e = os.WriteFile(filepath.Join(d, r.ID+".json"), append(b, '\n'), 0640)
	}
	return e
}
