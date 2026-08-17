// Package agentevaluations owns bounded, revision-exact project agent trials.
package agentevaluations

import (
	"crypto/rand"
	"crypto/sha256"
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

var ErrNotFound = errors.New("agent evaluation not found")
var ErrInvalid = errors.New("invalid agent evaluation")
var ErrConflict = errors.New("agent evaluation conflict")

type Check struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Expected    string `json:"expected,omitempty"`
	Hidden      bool   `json:"hidden"`
	Canary      string `json:"canary,omitempty"`
}
type Scenario struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	RepositoryRevision  string   `json:"repository_revision"`
	SanitizedInput      string   `json:"sanitized_input"`
	ExpectedOutcome     string   `json:"expected_outcome"`
	Checks              []Check  `json:"checks"`
	HumanReviewCriteria []string `json:"human_review_criteria"`
}
type Budget struct {
	MaximumCost        float64 `json:"maximum_cost"`
	Currency           string  `json:"currency"`
	MaximumLatencyMS   int64   `json:"maximum_latency_ms"`
	MaximumToolActions int     `json:"maximum_tool_actions"`
}
type SuiteInput struct {
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Scenarios         []Scenario `json:"scenarios"`
	Budget            Budget     `json:"budget"`
	ProhibitedActions []string   `json:"prohibited_actions"`
	ChangeReason      string     `json:"change_reason"`
	ExpectedVersion   int64      `json:"expected_version,omitempty"`
}
type SuiteVersion struct {
	Number int64 `json:"number"`
	SuiteInput
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
}
type Suite struct {
	ID             string         `json:"id"`
	RepositoryID   string         `json:"repository_id"`
	CurrentVersion int64          `json:"current_version"`
	Versions       []SuiteVersion `json:"versions"`
}

type TrialInput struct {
	SuiteID          string   `json:"suite_id"`
	SuiteVersion     int64    `json:"suite_version"`
	ProfileID        string   `json:"profile_id"`
	ProfileVersion   int64    `json:"profile_version"`
	ScenarioIDs      []string `json:"scenario_ids"`
	OperatorSupplied bool     `json:"operator_supplied"`
	ReproductionOf   string   `json:"reproduction_of,omitempty"`
}
type Authority struct {
	Isolated    bool `json:"isolated"`
	Publish     bool `json:"publish"`
	Secrets     bool `json:"secrets"`
	Merge       bool `json:"merge"`
	Environment bool `json:"environment"`
}
type ToolAction struct {
	Tool       string    `json:"tool"`
	Action     string    `json:"action"`
	Target     string    `json:"target,omitempty"`
	Allowed    bool      `json:"allowed"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}
type CheckResult struct {
	ScenarioID string `json:"scenario_id"`
	CheckID    string `json:"check_id"`
	Passed     bool   `json:"passed"`
	Summary    string `json:"summary"`
}
type ResultInput struct {
	Outputs           map[string]string `json:"outputs"`
	ToolActions       []ToolAction      `json:"tool_actions"`
	Artifacts         []Artifact        `json:"artifacts"`
	CheckResults      []CheckResult     `json:"check_results"`
	Cost              float64           `json:"cost"`
	Currency          string            `json:"currency"`
	LatencyMS         int64             `json:"latency_ms"`
	Failure           string            `json:"failure,omitempty"`
	ReproductionNotes string            `json:"reproduction_notes,omitempty"`
}
type DecisionInput struct {
	Verdict   string   `json:"verdict"`
	Rationale string   `json:"rationale"`
	Criteria  []string `json:"criteria"`
}
type Decision struct {
	DecisionInput
	Evaluator string    `json:"evaluator"`
	CreatedAt time.Time `json:"created_at"`
}
type Trial struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	TrialInput
	SourceRevisions      map[string]string `json:"source_revisions"`
	InputDigest          string            `json:"input_digest"`
	ProofLabel           string            `json:"proof_label"`
	Authority            Authority         `json:"authority"`
	Status               string            `json:"status"`
	CreatedBy            string            `json:"created_by"`
	CreatedAt            time.Time         `json:"created_at"`
	StartedAt            time.Time         `json:"started_at"`
	CompletedAt          *time.Time        `json:"completed_at,omitempty"`
	Result               *ResultInput      `json:"result,omitempty"`
	Contamination        bool              `json:"contamination"`
	ContaminationReasons []string          `json:"contamination_reasons"`
	BudgetFailures       []string          `json:"budget_failures"`
	PolicyFailures       []string          `json:"policy_failures"`
	Decisions            []Decision        `json:"decisions"`
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
		e = os.MkdirAll(filepath.Join(a, "suites"), 0750)
	}
	if e == nil {
		e = os.MkdirAll(filepath.Join(a, "trials"), 0750)
	}
	if e == nil {
		e = os.MkdirAll(filepath.Join(a, "onboardings"), 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func validList(xs []string) bool {
	if len(xs) > 100 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}
func validate(in SuiteInput) bool {
	if in.Name == "" || in.Description == "" || in.ChangeReason == "" || len(in.Scenarios) == 0 || len(in.Scenarios) > 50 || in.Budget.MaximumCost < 0 || in.Budget.MaximumLatencyMS < 1 || in.Budget.MaximumToolActions < 1 || !validList(in.ProhibitedActions) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range in.Scenarios {
		if s.ID == "" || seen[s.ID] || s.Title == "" || s.RepositoryRevision == "" || s.SanitizedInput == "" || s.ExpectedOutcome == "" || len(s.Checks) == 0 || !validList(s.HumanReviewCriteria) {
			return false
		}
		seen[s.ID] = true
		cs := map[string]bool{}
		for _, c := range s.Checks {
			if c.ID == "" || cs[c.ID] || !map[string]bool{"correctness": true, "policy": true}[c.Kind] || c.Description == "" || (!c.Hidden && c.Expected == "") {
				return false
			}
			cs[c.ID] = true
		}
	}
	return true
}
func (s *Store) Create(repo, actor string, in SuiteInput) (Suite, error) {
	if !validate(in) || in.ExpectedVersion != 0 {
		return Suite{}, ErrInvalid
	}
	in.ExpectedVersion = 0
	x := Suite{ID: id("aes_"), RepositoryID: repo, CurrentVersion: 1}
	x.Versions = []SuiteVersion{{Number: 1, SuiteInput: in, PublishedBy: actor, PublishedAt: s.now().UTC()}}
	s.mu.Lock()
	defer s.mu.Unlock()
	return x, s.write("suites", x.ID, x)
}
func (s *Store) Revise(repo, suite, actor string, in SuiteInput) (Suite, error) {
	if !validate(in) {
		return Suite{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Suite
	if s.read("suites", suite, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if in.ExpectedVersion != x.CurrentVersion {
		return x, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, SuiteVersion{Number: x.CurrentVersion, SuiteInput: in, PublishedBy: actor, PublishedAt: s.now().UTC()})
	return x, s.write("suites", x.ID, x)
}
func (s *Store) GetSuite(repo, id string, hidden bool) (Suite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Suite
	if s.read("suites", id, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if !hidden {
		redact(&x)
	}
	return x, nil
}
func (s *Store) ListSuites(repo string) ([]Suite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "suites"))
	if e != nil {
		return nil, e
	}
	out := []Suite{}
	for _, f := range es {
		var x Suite
		if s.read("suites", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo {
			redact(&x)
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func redact(x *Suite) {
	for vi := range x.Versions {
		for si := range x.Versions[vi].Scenarios {
			for ci := range x.Versions[vi].Scenarios[si].Checks {
				c := &x.Versions[vi].Scenarios[si].Checks[ci]
				if c.Hidden {
					c.Expected = ""
					c.Canary = ""
					c.Description = "hidden " + c.Kind + " check"
				}
			}
		}
	}
}
func version(x Suite, n int64) (SuiteVersion, bool) {
	for _, v := range x.Versions {
		if v.Number == n {
			return v, true
		}
	}
	return SuiteVersion{}, false
}
func digest(v SuiteVersion, in TrialInput) string {
	b, _ := json.Marshal(struct {
		V SuiteVersion
		I TrialInput
	}{v, in})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) Start(repo, actor string, in TrialInput) (Trial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var suite Suite
	if s.read("suites", in.SuiteID, &suite) != nil || suite.RepositoryID != repo {
		return Trial{}, ErrNotFound
	}
	v, ok := version(suite, in.SuiteVersion)
	if !ok || in.ProfileID == "" || in.ProfileVersion < 1 || len(in.ScenarioIDs) == 0 {
		return Trial{}, ErrInvalid
	}
	selected := map[string]bool{}
	for _, q := range in.ScenarioIDs {
		selected[q] = true
	}
	revs := map[string]string{}
	for _, q := range v.Scenarios {
		if selected[q.ID] {
			revs[q.ID] = q.RepositoryRevision
			delete(selected, q.ID)
		}
	}
	if len(selected) > 0 {
		return Trial{}, ErrInvalid
	}
	d := digest(v, in)
	label := "first_party_trial"
	es, _ := os.ReadDir(filepath.Join(s.root, "trials"))
	for _, f := range es {
		var prior Trial
		if s.read("trials", strings.TrimSuffix(f.Name(), ".json"), &prior) == nil && prior.RepositoryID == repo && prior.InputDigest == d {
			label = "repeated_trial"
			break
		}
	}
	if in.OperatorSupplied {
		label = "operator_supplied_trial"
	}
	if in.ReproductionOf != "" {
		var prior Trial
		if s.read("trials", in.ReproductionOf, &prior) != nil || prior.RepositoryID != repo {
			return Trial{}, ErrInvalid
		}
		label = "reproduction_trial"
	}
	now := s.now().UTC()
	x := Trial{ID: id("aet_"), RepositoryID: repo, TrialInput: in, SourceRevisions: revs, InputDigest: d, ProofLabel: label, Authority: Authority{Isolated: true}, Status: "running", CreatedBy: actor, CreatedAt: now, StartedAt: now}
	return x, s.write("trials", x.ID, x)
}
func (s *Store) Complete(repo, trial string, in ResultInput) (Trial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Trial
	if s.read("trials", trial, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if x.Status != "running" || in.Cost < 0 || in.LatencyMS < 0 || len(in.ToolActions) > 1000 {
		return x, ErrInvalid
	}
	var suite Suite
	if s.read("suites", x.SuiteID, &suite) != nil {
		return x, ErrNotFound
	}
	v, _ := version(suite, x.SuiteVersion)
	if in.Cost > v.Budget.MaximumCost {
		x.BudgetFailures = append(x.BudgetFailures, "cost budget exceeded")
	}
	if in.LatencyMS > v.Budget.MaximumLatencyMS {
		x.BudgetFailures = append(x.BudgetFailures, "latency budget exceeded")
	}
	if len(in.ToolActions) > v.Budget.MaximumToolActions {
		x.BudgetFailures = append(x.BudgetFailures, "tool-action budget exceeded")
	}
	for _, a := range in.ToolActions {
		for _, p := range v.ProhibitedActions {
			if strings.Contains(strings.ToLower(a.Action+" "+a.Target), strings.ToLower(p)) {
				x.PolicyFailures = append(x.PolicyFailures, "prohibited action attempted: "+p)
			}
		}
	}
	blob, _ := json.Marshal(in)
	low := strings.ToLower(string(blob))
	for _, q := range v.Scenarios {
		for _, c := range q.Checks {
			if c.Hidden && c.Canary != "" && strings.Contains(low, strings.ToLower(c.Canary)) {
				x.Contamination = true
				x.ContaminationReasons = append(x.ContaminationReasons, "hidden-check canary appeared in trial output or actions")
			}
		}
	}
	now := s.now().UTC()
	x.Result = &in
	x.CompletedAt = &now
	x.Status = "completed"
	if in.Failure != "" {
		x.Status = "failed"
	}
	return x, s.write("trials", x.ID, x)
}
func (s *Store) Decide(repo, trial, actor string, in DecisionInput) (Trial, error) {
	if !map[string]bool{"accept": true, "reject": true, "needs_review": true}[in.Verdict] || in.Rationale == "" || !validList(in.Criteria) {
		return Trial{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Trial
	if s.read("trials", trial, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if x.Status == "running" {
		return x, ErrInvalid
	}
	x.Decisions = append(x.Decisions, Decision{DecisionInput: in, Evaluator: actor, CreatedAt: s.now().UTC()})
	return x, s.write("trials", x.ID, x)
}
func (s *Store) GetTrial(repo, id string) (Trial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Trial
	if s.read("trials", id, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	return x, nil
}
func (s *Store) ListTrials(repo string) ([]Trial, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "trials"))
	if e != nil {
		return nil, e
	}
	out := []Trial{}
	for _, f := range es {
		var x Trial
		if s.read("trials", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) write(k, id string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(s.root, k, id+".json"), b, 0640)
}
func (s *Store) read(k, id string, v any) error {
	b, e := os.ReadFile(filepath.Join(s.root, k, id+".json"))
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
