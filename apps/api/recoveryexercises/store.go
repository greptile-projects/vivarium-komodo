// Package recoveryexercises retains bounded, metadata-only continuity rehearsal evidence.
package recoveryexercises

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
)

var (
	ErrNotFound = errors.New("recovery exercise not found")
	ErrInvalid  = errors.New("invalid recovery exercise")
	ErrConflict = errors.New("recovery exercise conflict")
)

type SelectedResource struct {
	ResourceID         string            `json:"resource_id"`
	SourceVersion      string            `json:"source_version"`
	DependencyVersions map[string]string `json:"dependency_versions"`
}
type Step struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	ResourceID string   `json:"resource_id,omitempty"`
	DependsOn  []string `json:"depends_on"`
	Command    string   `json:"command"`
	Expected   string   `json:"expected"`
}
type Check struct {
	ID                   string   `json:"id"`
	Kind                 string   `json:"kind"`
	Command              string   `json:"command"`
	Expected             string   `json:"expected"`
	ObjectiveResourceIDs []string `json:"objective_resource_ids"`
}
type LaunchInput struct {
	IdempotencyKey             string             `json:"idempotency_key"`
	Scenario                   string             `json:"scenario"`
	FailureModes               []string           `json:"failure_modes"`
	PlanID                     string             `json:"plan_id"`
	PlanVersion                int64              `json:"plan_version"`
	CaptureID                  string             `json:"capture_id"`
	Resources                  []SelectedResource `json:"resources"`
	EnvironmentID              string             `json:"environment_id"`
	IsolationKind              string             `json:"isolation_kind"`
	AuthoritativeStateWritable bool               `json:"authoritative_state_writable"`
	ProductionSecretsAvailable bool               `json:"production_secrets_available"`
	MaximumDurationSeconds     int64              `json:"maximum_duration_seconds"`
	MaximumCost                float64            `json:"maximum_cost"`
	Steps                      []Step             `json:"steps"`
	Checks                     []Check            `json:"checks"`
}
type Artifact struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"size_bytes"`
}
type StepResult struct {
	StepID      string     `json:"step_id"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  time.Time  `json:"finished_at"`
	Status      string     `json:"status"`
	Command     string     `json:"command"`
	LogExcerpt  string     `json:"log_excerpt,omitempty"`
	LogDigest   string     `json:"log_digest"`
	Redacted    bool       `json:"redacted"`
	Artifacts   []Artifact `json:"artifacts"`
	ManualSteps []string   `json:"manual_steps"`
	Gaps        []string   `json:"gaps"`
}
type CheckResult struct {
	CheckID                      string    `json:"check_id"`
	Status                       string    `json:"status"`
	StartedAt                    time.Time `json:"started_at"`
	FinishedAt                   time.Time `json:"finished_at"`
	Command                      string    `json:"command"`
	LogExcerpt                   string    `json:"log_excerpt,omitempty"`
	LogDigest                    string    `json:"log_digest"`
	Redacted                     bool      `json:"redacted"`
	AchievedObjectiveResourceIDs []string  `json:"achieved_objective_resource_ids"`
	Gaps                         []string  `json:"gaps"`
}
type ResultInput struct {
	ExpectedRevision int64         `json:"expected_revision"`
	StartedAt        time.Time     `json:"started_at"`
	FinishedAt       time.Time     `json:"finished_at"`
	Cost             float64       `json:"cost"`
	StepResults      []StepResult  `json:"step_results"`
	CheckResults     []CheckResult `json:"check_results"`
	Summary          string        `json:"summary"`
}
type Result struct {
	ResultInput
	ActorID    string    `json:"actor_id"`
	RecordedAt time.Time `json:"recorded_at"`
}
type Exercise struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	LaunchInput
	LaunchedBy                   string    `json:"launched_by"`
	LaunchedAt                   time.Time `json:"launched_at"`
	Result                       *Result   `json:"result,omitempty"`
	Current                      bool      `json:"current"`
	Status                       string    `json:"status"`
	Blockers                     []string  `json:"blockers"`
	AchievedObjectiveResourceIDs []string  `json:"achieved_objective_resource_ids"`
	Gaps                         []string  `json:"gaps"`
	DurationSeconds              int64     `json:"duration_seconds,omitempty"`
}

type Store struct {
	root  string
	plans *protectionplans.Store
	mu    sync.Mutex
	now   func() time.Time
}

func New(root string, plans *protectionplans.Store) (*Store, error) {
	if strings.TrimSpace(root) == "" || plans == nil {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, plans: plans, now: time.Now}, e
}
func newid() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) load(repo, id string) (Exercise, error) {
	var x Exercise
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(x Exercise) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func validLaunch(in LaunchInput) bool {
	if in.IdempotencyKey == "" || in.Scenario == "" || len(in.FailureModes) == 0 || in.PlanID == "" || in.PlanVersion < 1 || in.CaptureID == "" || len(in.Resources) == 0 || in.EnvironmentID == "" || in.IsolationKind == "" || in.AuthoritativeStateWritable || in.ProductionSecretsAvailable || in.MaximumDurationSeconds < 1 || in.MaximumDurationSeconds > 604800 || in.MaximumCost < 0 || len(in.Steps) == 0 || len(in.Steps) > 100 || len(in.Checks) == 0 || len(in.Checks) > 100 {
		return false
	}
	ids := map[string]bool{}
	for _, r := range in.Resources {
		if r.ResourceID == "" || r.SourceVersion == "" || ids[r.ResourceID] {
			return false
		}
		ids[r.ResourceID] = true
	}
	done := map[string]bool{}
	for _, x := range in.Steps {
		if x.ID == "" || x.Command == "" || x.Expected == "" || done[x.ID] {
			return false
		}
		for _, d := range x.DependsOn {
			if !done[d] {
				return false
			}
		}
		done[x.ID] = true
	}
	seen := map[string]bool{}
	for _, x := range in.Checks {
		if x.ID == "" || x.Command == "" || x.Expected == "" || len(x.ObjectiveResourceIDs) == 0 || seen[x.ID] {
			return false
		}
		if x.Kind != "integrity" && x.Kind != "user_journey" {
			return false
		}
		seen[x.ID] = true
	}
	return true
}
func captureFor(p protectionplans.Plan, id string) (protectionplans.Capture, bool) {
	for _, c := range p.Captures {
		if c.ID == id {
			return c, true
		}
	}
	return protectionplans.Capture{}, false
}
func (s *Store) validateSelection(repo string, in LaunchInput) bool {
	p, e := s.plans.Get(repo, in.PlanID)
	if e != nil {
		return false
	}
	c, ok := captureFor(p, in.CaptureID)
	if !ok || c.PlanVersion != in.PlanVersion || !c.Recoverable {
		return false
	}
	got := map[string]protectionplans.ManifestResource{}
	for _, r := range c.Resources {
		got[r.ResourceID] = r
	}
	for _, r := range in.Resources {
		x, ok := got[r.ResourceID]
		if !ok || x.SourceVersion != r.SourceVersion || !mapsEqual(x.DependencyVersions, r.DependencyVersions) {
			return false
		}
	}
	return true
}
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
func (s *Store) derive(x Exercise) Exercise {
	x.Blockers = nil
	x.Gaps = nil
	x.AchievedObjectiveResourceIDs = nil
	x.Current = true
	p, e := s.plans.Get(x.RepositoryID, x.PlanID)
	if e != nil {
		x.Blockers = append(x.Blockers, "protection_plan_unavailable")
	} else {
		if p.CurrentVersion != x.PlanVersion {
			x.Blockers = append(x.Blockers, "protection_plan_changed")
		}
		c, ok := captureFor(p, x.CaptureID)
		if !ok || !c.Recoverable {
			x.Blockers = append(x.Blockers, "protected_capture_no_longer_recoverable")
		}
		if p.LatestRecoverableCaptureID != "" {
			latest, _ := captureFor(p, p.LatestRecoverableCaptureID)
			lm := map[string]protectionplans.ManifestResource{}
			for _, r := range latest.Resources {
				lm[r.ResourceID] = r
			}
			for _, r := range x.Resources {
				if z, ok := lm[r.ResourceID]; !ok || !mapsEqual(z.DependencyVersions, r.DependencyVersions) {
					x.Blockers = append(x.Blockers, "dependency_versions_changed")
					break
				}
			}
		}
	}
	if len(x.Blockers) > 0 {
		x.Current = false
	}
	x.Status = "planned"
	if x.Result != nil {
		x.Status = "passed"
		x.DurationSeconds = int64(x.Result.FinishedAt.Sub(x.Result.StartedAt).Seconds())
		for _, r := range x.Result.StepResults {
			x.Gaps = append(x.Gaps, r.Gaps...)
			if r.Status != "passed" {
				x.Status = "failed"
			}
		}
		for _, r := range x.Result.CheckResults {
			x.Gaps = append(x.Gaps, r.Gaps...)
			if r.Status != "passed" {
				x.Status = "failed"
			}
			x.AchievedObjectiveResourceIDs = append(x.AchievedObjectiveResourceIDs, r.AchievedObjectiveResourceIDs...)
		}
		if x.DurationSeconds > x.MaximumDurationSeconds {
			x.Gaps = append(x.Gaps, "restoration_time_exceeded")
			x.Status = "failed"
		}
		if x.Result.Cost > x.MaximumCost {
			x.Gaps = append(x.Gaps, "exercise_cost_exceeded")
			x.Status = "failed"
		}
	}
	x.Blockers = unique(x.Blockers)
	x.Gaps = unique(x.Gaps)
	x.AchievedObjectiveResourceIDs = unique(x.AchievedObjectiveResourceIDs)
	return x
}
func unique(v []string) []string {
	m := map[string]bool{}
	o := []string{}
	for _, x := range v {
		if !m[x] {
			m[x] = true
			o = append(o, x)
		}
	}
	sort.Strings(o)
	return o
}
func (s *Store) Launch(repo, actor string, in LaunchInput) (Exercise, error) {
	if repo == "" || actor == "" || !validLaunch(in) || !s.validateSelection(repo, in) {
		return Exercise{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo)
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		var x Exercise
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if json.Unmarshal(b, &x) == nil && x.IdempotencyKey == in.IdempotencyKey {
			a, _ := json.Marshal(x.LaunchInput)
			q, _ := json.Marshal(in)
			if string(a) != string(q) {
				return Exercise{}, ErrConflict
			}
			return s.derive(x), nil
		}
	}
	x := Exercise{ID: newid(), RepositoryID: repo, Revision: 1, LaunchInput: in, LaunchedBy: actor, LaunchedAt: s.now().UTC()}
	if e := s.save(x); e != nil {
		return x, e
	}
	return s.derive(x), nil
}
func validResult(in ResultInput, x Exercise) bool {
	if in.StartedAt.IsZero() || in.FinishedAt.Before(in.StartedAt) || in.Cost < 0 || in.Summary == "" || len(in.StepResults) != len(x.Steps) || len(in.CheckResults) != len(x.Checks) {
		return false
	}
	steps := map[string]Step{}
	for _, s := range x.Steps {
		steps[s.ID] = s
	}
	for _, r := range in.StepResults {
		s, ok := steps[r.StepID]
		if !ok || r.Command != s.Command || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || !r.Redacted || r.LogDigest == "" || (r.Status != "passed" && r.Status != "failed") {
			return false
		}
		for _, a := range r.Artifacts {
			if a.Name == "" || a.Digest == "" || a.SizeBytes < 0 {
				return false
			}
		}
	}
	checks := map[string]Check{}
	for _, c := range x.Checks {
		checks[c.ID] = c
	}
	for _, r := range in.CheckResults {
		c, ok := checks[r.CheckID]
		if !ok || r.Command != c.Command || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || !r.Redacted || r.LogDigest == "" || (r.Status != "passed" && r.Status != "failed") {
			return false
		}
	}
	return true
}
func (s *Store) Record(repo, id, actor string, in ResultInput) (Exercise, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	if x.Revision != in.ExpectedRevision || x.Result != nil {
		return Exercise{}, ErrConflict
	}
	if actor == "" || !validResult(in, x) {
		return Exercise{}, ErrInvalid
	}
	x.Revision++
	x.Result = &Result{ResultInput: in, ActorID: actor, RecordedAt: s.now().UTC()}
	if e = s.save(x); e != nil {
		return x, e
	}
	return s.derive(x), nil
}
func (s *Store) Get(repo, id string) (Exercise, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	return s.derive(x), e
}
func (s *Store) List(repo string) ([]Exercise, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Exercise{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Exercise{}
	for _, f := range es {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		x, e := s.load(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, s.derive(x))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LaunchedAt.After(out[j].LaunchedAt) })
	return out, nil
}
