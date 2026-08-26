// Package signalrollouts retains protected, progressive instrumentation releases and production quality evidence.
package signalrollouts

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

var ErrNotFound = errors.New("signal rollout not found")
var ErrInvalid = errors.New("invalid signal rollout")
var ErrConflict = errors.New("signal rollout changed")
var ErrForbidden = errors.New("signal rollout action forbidden")

type Stage struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	EnvironmentID       string   `json:"environment_id"`
	EnvironmentRevision string   `json:"environment_revision"`
	ServiceIDs          []string `json:"service_ids"`
	Audiences           []string `json:"audiences"`
	Regions             []string `json:"regions"`
	TrafficPercent      float64  `json:"traffic_percent"`
}
type Input struct {
	ContractID          string   `json:"contract_id"`
	ContractVersion     int64    `json:"contract_version"`
	PullRequestID       string   `json:"pull_request_id"`
	ImplementationRunID string   `json:"implementation_run_id"`
	DeployedRevision    string   `json:"deployed_revision"`
	CollectorRevision   string   `json:"collector_revision"`
	ControllerID        string   `json:"controller_id"`
	OperatorIDs         []string `json:"operator_ids"`
	PrivacyControls     []string `json:"privacy_controls"`
	Stages              []Stage  `json:"stages"`
	MaxCardinality      int64    `json:"max_cardinality"`
	MaxStorageBytes     int64    `json:"max_storage_bytes"`
	MaxQueryCost        float64  `json:"max_query_cost"`
	Currency            string   `json:"currency"`
}
type ObservationInput struct {
	ExpectedRevision        int64     `json:"expected_revision"`
	StageID                 string    `json:"stage_id"`
	WindowStart             time.Time `json:"window_start"`
	WindowEnd               time.Time `json:"window_end"`
	EvidenceIDs             []string  `json:"evidence_ids"`
	SignalHealth            string    `json:"signal_health"`
	CoveragePercent         float64   `json:"coverage_percent"`
	LatencyMS               float64   `json:"latency_ms"`
	MissingPercent          float64   `json:"missing_percent"`
	SamplingBiasPercent     float64   `json:"sampling_bias_percent"`
	Cardinality             int64     `json:"cardinality"`
	StorageBytes            int64     `json:"storage_bytes"`
	QueryCost               float64   `json:"query_cost"`
	PipelineLossPercent     float64   `json:"pipeline_loss_percent"`
	MalformedPayloads       int64     `json:"malformed_payloads"`
	UnexpectedSensitiveData bool      `json:"unexpected_sensitive_data"`
	CollectorStatus         string    `json:"collector_status"`
	ServiceStatus           string    `json:"service_status"`
}
type Observation struct {
	ID string `json:"id"`
	ObservationInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ControlInput struct {
	ExpectedRevision     int64    `json:"expected_revision"`
	Action               string   `json:"action"`
	StageID              string   `json:"stage_id"`
	NarrowTrafficPercent float64  `json:"narrow_traffic_percent,omitempty"`
	Rationale            string   `json:"rationale"`
	EvidenceIDs          []string `json:"evidence_ids"`
}
type Control struct {
	ID string `json:"id"`
	ControlInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Finding struct {
	Kind        string   `json:"kind"`
	Detail      string   `json:"detail"`
	Containment string   `json:"containment"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Rollout struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	CreatedByID             string        `json:"created_by_id"`
	CreatedAt               time.Time     `json:"created_at"`
	ActiveStageID           string        `json:"active_stage_id"`
	EffectiveTrafficPercent float64       `json:"effective_traffic_percent"`
	State                   string        `json:"state"`
	Observations            []Observation `json:"observations"`
	Controls                []Control     `json:"controls"`
	Findings                []Finding     `json:"findings"`
	PredictedNextAction     string        `json:"predicted_next_action"`
	NonAuthority            []string      `json:"non_authority"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func has(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func valid(in Input) bool {
	if in.ContractID == "" || in.ContractVersion < 1 || in.PullRequestID == "" || in.ImplementationRunID == "" || in.DeployedRevision == "" || in.CollectorRevision == "" || in.ControllerID == "" || len(in.OperatorIDs) == 0 || len(in.PrivacyControls) == 0 || len(in.Stages) == 0 || in.MaxCardinality <= 0 || in.MaxStorageBytes <= 0 || in.MaxQueryCost <= 0 || in.Currency == "" {
		return false
	}
	seen := map[string]bool{}
	for _, s := range in.Stages {
		if s.ID == "" || s.Name == "" || s.EnvironmentID == "" || s.EnvironmentRevision == "" || len(s.ServiceIDs) == 0 || len(s.Audiences) == 0 || len(s.Regions) == 0 || s.TrafficPercent <= 0 || s.TrafficPercent > 100 || seen[s.ID] {
			return false
		}
		seen[s.ID] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Rollout, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Rollout{}, ErrInvalid
	}
	if !has(in.OperatorIDs, actor) {
		return Rollout{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := Rollout{ID: id(), RepositoryID: repo, Revision: 1, Input: in, CreatedByID: actor, CreatedAt: s.now().UTC(), ActiveStageID: in.Stages[0].ID, EffectiveTrafficPercent: in.Stages[0].TrafficPercent, State: "staged", Observations: []Observation{}, Controls: []Control{}}
	r = derive(r)
	return r, s.write(r)
}
func stage(r Rollout, id string) (Stage, bool) {
	for _, x := range r.Stages {
		if x.ID == id {
			return x, true
		}
	}
	return Stage{}, false
}
func latest(r Rollout) *Observation {
	for i := len(r.Observations) - 1; i >= 0; i-- {
		if r.Observations[i].StageID == r.ActiveStageID {
			return &r.Observations[i]
		}
	}
	return nil
}
func derive(r Rollout) Rollout {
	r.Findings = nil
	add := func(k, d, c string, e []string) { r.Findings = append(r.Findings, Finding{k, d, c, e}) }
	o := latest(r)
	if o == nil {
		add("missing_production_evidence", "active stage has no production quality observation", "pause", nil)
	} else {
		e := o.EvidenceIDs
		if len(e) == 0 {
			add("missing_evidence", "observation has no revision-bound evidence", "pause", e)
		}
		if o.UnexpectedSensitiveData {
			add("unexpected_sensitive_data", "payload contained data outside reviewed privacy controls", "rollback", e)
		}
		if o.MalformedPayloads > 0 {
			add("malformed_payloads", "collector received malformed signal payloads", "pause", e)
		}
		if o.CollectorStatus != "healthy" {
			add("collector_outage", "collector is not healthy", "pause", e)
		}
		if o.ServiceStatus != "healthy" || o.SignalHealth != "healthy" {
			add("service_regression", "service or signal health regressed", "rollback", e)
		}
		if o.QueryCost > r.MaxQueryCost || o.StorageBytes > r.MaxStorageBytes {
			add("budget_breach", "storage or query cost exceeded the rollout bound", "pause", e)
		}
		if o.Cardinality > r.MaxCardinality {
			add("cardinality_breach", "observed cardinality exceeded the rollout bound", "narrow", e)
		}
		if o.SamplingBiasPercent > 10 {
			add("sampling_skew", "sampling bias exceeded ten percent", "narrow", e)
		}
		if o.PipelineLossPercent > 5 {
			add("pipeline_loss", "pipeline loss exceeded five percent", "pause", e)
		}
	}
	if len(r.Findings) > 0 {
		c := r.Findings[0].Containment
		for _, f := range r.Findings {
			if f.Containment == "rollback" {
				c = "rollback"
				break
			}
		}
		switch c {
		case "rollback":
			r.State = "rolled_back"
			r.EffectiveTrafficPercent = 0
		case "narrow":
			r.State = "narrowed"
			st, _ := stage(r, r.ActiveStageID)
			r.EffectiveTrafficPercent = st.TrafficPercent / 2
		default:
			r.State = "paused"
			r.EffectiveTrafficPercent = 0
		}
		r.PredictedNextAction = "containment remains active until an operator submits current passing evidence"
	} else {
		r.PredictedNextAction = "operator may resume or advance after reviewing current production proof"
	}
	if len(r.Controls) > 0 {
		c := r.Controls[len(r.Controls)-1]
		if c.Action == "pause" {
			r.State = "paused"
			r.EffectiveTrafficPercent = 0
		}
		if c.Action == "rollback" {
			r.State = "rolled_back"
			r.EffectiveTrafficPercent = 0
		}
		if c.Action == "narrow" {
			r.State = "narrowed"
			r.EffectiveTrafficPercent = c.NarrowTrafficPercent
		}
		if c.Action == "resume" && len(r.Findings) == 0 {
			r.State = "active"
			st, _ := stage(r, c.StageID)
			r.EffectiveTrafficPercent = st.TrafficPercent
		}
	}
	r.NonAuthority = []string{"Rollout records and controls grant no repository, telemetry, collector, data, agent, credential, release, deployment, environment, spending, or operational authority."}
	return r
}
func (s *Store) mutate(repo, rid string, expected int64, fn func(*Rollout) error) (Rollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	if e != nil {
		return r, e
	}
	if r.Revision != expected {
		return Rollout{}, ErrConflict
	}
	if e = fn(&r); e != nil {
		return Rollout{}, e
	}
	r.Revision++
	r = derive(r)
	return r, s.write(r)
}
func (s *Store) Observe(repo, rid, kind, actor string, in ObservationInput) (Rollout, error) {
	if kind != "human" || actor == "" || in.StageID == "" || in.WindowStart.IsZero() || !in.WindowEnd.After(in.WindowStart) || in.CoveragePercent < 0 || in.CoveragePercent > 100 || in.MissingPercent < 0 || in.SamplingBiasPercent < 0 || in.Cardinality < 0 || in.StorageBytes < 0 || in.QueryCost < 0 || in.PipelineLossPercent < 0 || in.MalformedPayloads < 0 || !has([]string{"healthy", "degraded", "failed"}, in.SignalHealth) || !has([]string{"healthy", "outage", "degraded"}, in.CollectorStatus) || !has([]string{"healthy", "regressed"}, in.ServiceStatus) {
		return Rollout{}, ErrInvalid
	}
	return s.mutate(repo, rid, in.ExpectedRevision, func(r *Rollout) error {
		if !has(r.OperatorIDs, actor) {
			return ErrForbidden
		}
		if _, ok := stage(*r, in.StageID); !ok {
			return ErrInvalid
		}
		r.ActiveStageID = in.StageID
		r.Observations = append(r.Observations, Observation{ID: id(), ObservationInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Control(repo, rid, kind, actor string, in ControlInput) (Rollout, error) {
	if kind != "human" || actor == "" || !has([]string{"pause", "narrow", "resume", "rollback"}, in.Action) || in.StageID == "" || strings.TrimSpace(in.Rationale) == "" || len(in.EvidenceIDs) == 0 || in.Action == "narrow" && (in.NarrowTrafficPercent <= 0 || in.NarrowTrafficPercent > 100) {
		return Rollout{}, ErrInvalid
	}
	return s.mutate(repo, rid, in.ExpectedRevision, func(r *Rollout) error {
		if !has(r.OperatorIDs, actor) {
			return ErrForbidden
		}
		if _, ok := stage(*r, in.StageID); !ok {
			return ErrInvalid
		}
		if in.Action == "resume" && len(derive(*r).Findings) > 0 {
			return ErrConflict
		}
		r.ActiveStageID = in.StageID
		r.Controls = append(r.Controls, Control{ID: id(), ControlInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Get(repo, rid string) (Rollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, rid)
	return derive(r), e
}
func (s *Store) List(repo, contract string) ([]Rollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fsx, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(fsx)
	out := []Rollout{}
	for _, f := range fsx {
		b, x := os.ReadFile(f)
		var r Rollout
		if x == nil {
			x = json.Unmarshal(b, &r)
		}
		if x != nil {
			return nil, x
		}
		if r.ContractID == contract {
			out = append(out, derive(r))
		}
	}
	return out, nil
}
func (s *Store) read(repo, rid string) (Rollout, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, rid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Rollout{}, ErrNotFound
	}
	var r Rollout
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	return r, e
}
func (s *Store) write(r Rollout) error {
	d := filepath.Join(s.root, r.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(d, r.ID+".json"), b, 0640)
}
