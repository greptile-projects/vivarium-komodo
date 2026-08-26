// Package capacityrehearsals retains bounded, revision-exact scaling experiments.
package capacityrehearsals

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

var (
	ErrNotFound = errors.New("capacity rehearsal not found")
	ErrInvalid  = errors.New("invalid capacity rehearsal")
	ErrConflict = errors.New("capacity rehearsal changed")
)

type Candidate struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	Approach                  string `json:"approach"`
	ReleaseID                 string `json:"release_id"`
	ReleaseRevision           string `json:"release_revision"`
	InfrastructurePlanID      string `json:"infrastructure_plan_id"`
	InfrastructureRevision    string `json:"infrastructure_revision"`
	SchemaID                  string `json:"schema_id"`
	SchemaRevision            string `json:"schema_revision"`
	DependencyConfigurationID string `json:"dependency_configuration_id"`
	DependencyRevision        string `json:"dependency_revision"`
}
type Limit struct {
	MaxDurationSeconds   int64   `json:"max_duration_seconds"`
	MaxVirtualUsers      int64   `json:"max_virtual_users"`
	MaxRequestsPerSecond float64 `json:"max_requests_per_second"`
	MaxCost              float64 `json:"max_cost"`
	Currency             string  `json:"currency"`
}
type Scenario struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Kind                string   `json:"kind"`
	WorkloadSource      string   `json:"workload_source"`
	Demand              float64  `json:"demand"`
	DemandUnit          string   `json:"demand_unit"`
	DurationSeconds     int64    `json:"duration_seconds"`
	Failure             string   `json:"failure,omitempty"`
	CorrectnessCriteria []string `json:"correctness_criteria"`
}
type Input struct {
	ObjectiveID         string      `json:"objective_id"`
	ObjectiveVersion    int64       `json:"objective_version"`
	ModelID             string      `json:"model_id,omitempty"`
	ModelRevision       int64       `json:"model_revision,omitempty"`
	Title               string      `json:"title"`
	DefinitionPath      string      `json:"definition_path"`
	DefinitionRevision  string      `json:"definition_revision"`
	EnvironmentID       string      `json:"environment_id"`
	EnvironmentRevision string      `json:"environment_revision"`
	EnvironmentClass    string      `json:"environment_class"`
	PolicyApprovalID    string      `json:"policy_approval_id,omitempty"`
	CoordinatedLoadKey  string      `json:"coordinated_load_key"`
	Limits              Limit       `json:"limits"`
	Candidates          []Candidate `json:"candidates"`
	Scenarios           []Scenario  `json:"scenarios"`
	OwnerIDs            []string    `json:"owner_ids"`
}
type Metrics struct {
	Throughput        float64            `json:"throughput"`
	ThroughputUnit    string             `json:"throughput_unit"`
	LatencyP50MS      float64            `json:"latency_p50_ms"`
	LatencyP95MS      float64            `json:"latency_p95_ms"`
	LatencyP99MS      float64            `json:"latency_p99_ms"`
	ErrorRate         float64            `json:"error_rate"`
	Saturation        float64            `json:"saturation"`
	SaturationUnit    string             `json:"saturation_unit"`
	RecoverySeconds   float64            `json:"recovery_seconds"`
	CorrectnessPassed bool               `json:"correctness_passed"`
	Resources         map[string]float64 `json:"resources"`
	CarbonGrams       *float64           `json:"carbon_grams,omitempty"`
	Cost              float64            `json:"cost"`
	Currency          string             `json:"currency"`
}
type AttemptInput struct {
	ExpectedRevision    int64     `json:"expected_revision"`
	CandidateID         string    `json:"candidate_id"`
	ScenarioID          string    `json:"scenario_id"`
	ActorKind           string    `json:"actor_kind"`
	StartedAt           time.Time `json:"started_at"`
	EndedAt             time.Time `json:"ended_at"`
	EnvironmentRevision string    `json:"environment_revision"`
	WorkloadDigest      string    `json:"workload_digest"`
	Repetitions         int       `json:"repetitions"`
	NoisePercent        float64   `json:"noise_percent"`
	Status              string    `json:"status"`
	Metrics             Metrics   `json:"metrics"`
	Logs                []string  `json:"logs"`
	ArtifactDigests     []string  `json:"artifact_digests"`
	Notes               string    `json:"notes,omitempty"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
	Classification string    `json:"classification"`
	Proof          bool      `json:"proof"`
	Gaps           []Gap     `json:"gaps"`
}
type Gap struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	Reference string `json:"reference,omitempty"`
}
type Rehearsal struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	AuthorID     string    `json:"author_id"`
	CreatedAt    time.Time `json:"created_at"`
	Attempts     []Attempt `json:"attempts"`
	Gaps         []Gap     `json:"gaps"`
	NonAuthority []string  `json:"non_authority"`
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
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func valid(in Input) bool {
	if in.ObjectiveID == "" || in.ObjectiveVersion < 1 || strings.TrimSpace(in.Title) == "" || in.DefinitionPath == "" || in.DefinitionRevision == "" || in.EnvironmentID == "" || in.EnvironmentRevision == "" || !map[string]bool{"isolated": true, "policy_approved": true}[in.EnvironmentClass] || in.EnvironmentClass == "policy_approved" && in.PolicyApprovalID == "" || in.CoordinatedLoadKey == "" || in.Limits.MaxDurationSeconds < 1 || in.Limits.MaxVirtualUsers < 1 || in.Limits.MaxRequestsPerSecond <= 0 || in.Limits.MaxCost <= 0 || in.Limits.Currency == "" || len(in.Candidates) < 2 || len(in.Scenarios) == 0 || len(in.OwnerIDs) == 0 {
		return false
	}
	cids := map[string]bool{}
	for _, c := range in.Candidates {
		if c.ID == "" || cids[c.ID] || !map[string]bool{"vertical": true, "horizontal": true, "architectural": true, "caching": true, "queueing": true, "demand_shaping": true}[c.Approach] || c.ReleaseID == "" || c.ReleaseRevision == "" || c.InfrastructurePlanID == "" || c.InfrastructureRevision == "" || c.SchemaID == "" || c.SchemaRevision == "" || c.DependencyConfigurationID == "" || c.DependencyRevision == "" {
			return false
		}
		cids[c.ID] = true
	}
	sids := map[string]bool{}
	for _, s := range in.Scenarios {
		if s.ID == "" || sids[s.ID] || !map[string]bool{"load": true, "failure": true, "load_and_failure": true}[s.Kind] || !map[string]bool{"synthetic": true, "privacy_preserving": true}[s.WorkloadSource] || s.Demand <= 0 || s.DemandUnit == "" || s.DurationSeconds < 1 || s.DurationSeconds > in.Limits.MaxDurationSeconds || len(s.CorrectnessCriteria) == 0 || strings.Contains(s.DemandUnit, "request") && s.Demand > in.Limits.MaxRequestsPerSecond || strings.Contains(s.DemandUnit, "user") && s.Demand > float64(in.Limits.MaxVirtualUsers) {
			return false
		}
		sids[s.ID] = true
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
func (s *Store) AppendAttempt(repo, rid, actor string, in AttemptInput) (Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if in.ExpectedRevision != r.Revision {
		return r, ErrConflict
	}
	if !validAttempt(r, in) {
		return r, ErrInvalid
	}
	a := Attempt{ID: id(), AttemptInput: in, ActorID: actor, CreatedAt: s.now().UTC()}
	a = classify(r, a)
	r.Revision++
	r.Attempts = append(r.Attempts, a)
	return r, s.write(r)
}
func validAttempt(r Rehearsal, a AttemptInput) bool {
	if a.CandidateID == "" || a.ScenarioID == "" || !map[string]bool{"human": true, "agent": true}[a.ActorKind] || a.StartedAt.IsZero() || !a.EndedAt.After(a.StartedAt) || a.EnvironmentRevision == "" || a.WorkloadDigest == "" || a.Repetitions < 1 || a.NoisePercent < 0 || !map[string]bool{"completed": true, "failed": true, "aborted": true, "untestable": true}[a.Status] || len(a.Logs) == 0 || a.Metrics.Throughput < 0 || a.Metrics.ErrorRate < 0 || a.Metrics.Cost < 0 || a.Metrics.Currency != r.Limits.Currency {
		return false
	}
	ci, si := false, false
	for _, c := range r.Candidates {
		ci = ci || c.ID == a.CandidateID
	}
	for _, s := range r.Scenarios {
		si = si || s.ID == a.ScenarioID
	}
	return ci && si
}
func classify(r Rehearsal, a Attempt) Attempt {
	a.Classification = "valid"
	dur := a.EndedAt.Sub(a.StartedAt)
	switch {
	case a.EnvironmentRevision != r.EnvironmentRevision:
		a.Classification = "incomparable"
		a.Gaps = append(a.Gaps, Gap{Kind: "environment_drift", Detail: "Attempt environment does not match the frozen rehearsal."})
	case dur > time.Duration(r.Limits.MaxDurationSeconds)*time.Second || a.Metrics.Cost > r.Limits.MaxCost:
		a.Classification = "unsafe"
		a.Gaps = append(a.Gaps, Gap{Kind: "limit_exceeded", Detail: "Observed duration or cost exceeded the declared bound."})
	case a.Status == "untestable" || a.Status == "aborted":
		a.Classification = "untestable"
		a.Gaps = append(a.Gaps, Gap{Kind: "untestable", Detail: "The candidate produced no comparable completed evidence."})
	case a.Status != "completed":
		a.Classification = "failed"
	case a.Repetitions < 2 || a.NoisePercent > 10:
		a.Classification = "noisy"
		a.Gaps = append(a.Gaps, Gap{Kind: "noisy_result", Detail: "At least two repetitions and noise at or below ten percent are required for proof."})
	case !a.Metrics.CorrectnessPassed:
		a.Classification = "incorrect"
		a.Gaps = append(a.Gaps, Gap{Kind: "correctness_failure", Detail: "Performance evidence cannot prove capacity while correctness criteria fail."})
	}
	a.Proof = a.Classification == "valid"
	return a
}
func Resolve(r Rehearsal) Rehearsal {
	r.Gaps = nil
	r.NonAuthority = []string{"Capacity rehearsals grant no spending, provider, repository, release, infrastructure, schema, dependency, environment, credential, deployment, or operational authority."}
	digests := map[string]map[string]bool{}
	for _, a := range r.Attempts {
		if a.Proof {
			if digests[a.ScenarioID] == nil {
				digests[a.ScenarioID] = map[string]bool{}
			}
			digests[a.ScenarioID][a.WorkloadDigest] = true
		}
	}
	for i := range r.Attempts {
		a := &r.Attempts[i]
		if a.Proof && len(digests[a.ScenarioID]) > 1 {
			a.Proof = false
			a.Classification = "incomparable"
			a.Gaps = append(a.Gaps, Gap{Kind: "workload_mismatch", Detail: "Valid attempts for this scenario use different workload digests."})
		}
	}
	proofs := map[string]map[string]bool{}
	for _, a := range r.Attempts {
		if a.Proof {
			if proofs[a.CandidateID] == nil {
				proofs[a.CandidateID] = map[string]bool{}
			}
			proofs[a.CandidateID][a.ScenarioID] = true
		}
	}
	for _, c := range r.Candidates {
		for _, sc := range r.Scenarios {
			if !proofs[c.ID][sc.ID] {
				r.Gaps = append(r.Gaps, Gap{Kind: "missing_comparable_proof", Detail: "Candidate " + c.ID + " lacks valid proof for scenario " + sc.ID + ".", Reference: c.ID})
			}
		}
	}
	return r
}
func (s *Store) Get(repo, rid string) (Rehearsal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, rid)
}
func (s *Store) List(repo string) ([]Rehearsal, error) {
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
		out = append(out, r)
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
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "rehearsal-*.tmp")
	if e != nil {
		return e
	}
	n := f.Name()
	defer os.Remove(n)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if x := f.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, r.ID+".json"))
	}
	return e
}
