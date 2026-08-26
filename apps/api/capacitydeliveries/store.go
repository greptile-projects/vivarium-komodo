// Package capacitydeliveries retains progressive production scaling and proof of usable capacity.
package capacitydeliveries

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

var ErrNotFound = errors.New("capacity delivery not found")
var ErrInvalid = errors.New("invalid capacity delivery")
var ErrConflict = errors.New("capacity delivery changed")
var ErrForbidden = errors.New("capacity delivery action forbidden")

type Phase struct {
	ID                  string   `json:"id"`
	PlanPhaseID         string   `json:"plan_phase_id"`
	Name                string   `json:"name"`
	EnvironmentID       string   `json:"environment_id"`
	EnvironmentRevision string   `json:"environment_revision"`
	ControllerID        string   `json:"controller_id"`
	OperatorIDs         []string `json:"operator_ids"`
	DelegatedAgentIDs   []string `json:"delegated_agent_ids,omitempty"`
	TargetCapacity      float64  `json:"target_capacity"`
	CapacityUnit        string   `json:"capacity_unit"`
	MaxLoad             float64  `json:"max_load"`
	MinHeadroomPercent  float64  `json:"min_headroom_percent"`
	MaxCost             float64  `json:"max_cost"`
	Currency            string   `json:"currency"`
}
type Input struct {
	PlanID            string  `json:"plan_id"`
	PlanRevision      int64   `json:"plan_revision"`
	ObjectiveID       string  `json:"objective_id"`
	ObjectiveVersion  int64   `json:"objective_version"`
	ModelID           string  `json:"model_id"`
	ModelRevision     int64   `json:"model_revision"`
	DecisionRevisitID string  `json:"decision_revisit_id"`
	Phases            []Phase `json:"phases"`
}
type ServiceLevel struct {
	Name   string  `json:"name"`
	Target float64 `json:"target"`
	Actual float64 `json:"actual"`
	Unit   string  `json:"unit"`
	Met    bool    `json:"met"`
}
type DependencyHealth struct {
	DependencyID string `json:"dependency_id"`
	Region       string `json:"region,omitempty"`
	Status       string `json:"status"`
	EvidenceID   string `json:"evidence_id"`
}
type ObservationInput struct {
	ExpectedRevision                 int64              `json:"expected_revision"`
	PhaseID                          string             `json:"phase_id"`
	ReleaseRevision                  string             `json:"release_revision"`
	InfrastructureRevision           string             `json:"infrastructure_revision"`
	SchemaRevision                   string             `json:"schema_revision,omitempty"`
	DependencyConfigurationRevision  string             `json:"dependency_configuration_revision,omitempty"`
	EvidenceWindowStart              time.Time          `json:"evidence_window_start"`
	EvidenceWindowEnd                time.Time          `json:"evidence_window_end"`
	ProductionEvidenceIDs            []string           `json:"production_evidence_ids"`
	AllocatedCapacity                float64            `json:"allocated_capacity"`
	UsableCapacity                   float64            `json:"usable_capacity"`
	Load                             float64            `json:"load"`
	ForecastLoad                     float64            `json:"forecast_load"`
	HeadroomPercent                  float64            `json:"headroom_percent"`
	ScalingLagSeconds                float64            `json:"scaling_lag_seconds"`
	MaxScalingLagSeconds             float64            `json:"max_scaling_lag_seconds"`
	RegionalImbalancePercent         float64            `json:"regional_imbalance_percent"`
	MaxRegionalImbalancePercent      float64            `json:"max_regional_imbalance_percent"`
	ServiceLevels                    []ServiceLevel     `json:"service_levels"`
	Dependencies                     []DependencyHealth `json:"dependencies"`
	Correctness                      string             `json:"correctness"`
	Reliability                      string             `json:"reliability"`
	Quota                            string             `json:"quota"`
	Cost                             float64            `json:"cost"`
	ReservationUtilizationPercent    float64            `json:"reservation_utilization_percent"`
	MinReservationUtilizationPercent float64            `json:"min_reservation_utilization_percent"`
}
type Observation struct {
	ID string `json:"id"`
	ObservationInput
	ActorKind string    `json:"actor_kind"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ControlInput struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Action           string   `json:"action"`
	PhaseID          string   `json:"phase_id"`
	ThrottlePercent  float64  `json:"throttle_percent,omitempty"`
	Rationale        string   `json:"rationale"`
	EvidenceIDs      []string `json:"evidence_ids"`
}
type Control struct {
	ID string `json:"id"`
	ControlInput
	ActorKind string    `json:"actor_kind"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind        string `json:"kind"`
	Detail      string `json:"detail"`
	Reference   string `json:"reference,omitempty"`
	Containment string `json:"containment"`
}
type Delivery struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	CreatorID           string        `json:"creator_id"`
	CreatedAt           time.Time     `json:"created_at"`
	ActivePhaseID       string        `json:"active_phase_id"`
	State               string        `json:"state"`
	Observations        []Observation `json:"observations"`
	Controls            []Control     `json:"controls"`
	Blockers            []Blocker     `json:"blockers"`
	PredictedNextAction string        `json:"predicted_next_action"`
	ObjectiveValidated  bool          `json:"objective_validated"`
	ForecastValidated   bool          `json:"forecast_validated"`
	NonAuthority        []string      `json:"non_authority"`
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
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func allowed(v string, xs ...string) bool { return contains(xs, v) }
func valid(in Input) bool {
	if in.PlanID == "" || in.PlanRevision < 1 || in.ObjectiveID == "" || in.ObjectiveVersion < 1 || in.ModelID == "" || in.ModelRevision < 1 || in.DecisionRevisitID == "" || len(in.Phases) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, p := range in.Phases {
		if p.ID == "" || p.PlanPhaseID == "" || p.Name == "" || p.EnvironmentID == "" || p.EnvironmentRevision == "" || p.ControllerID == "" || len(p.OperatorIDs) == 0 || p.TargetCapacity <= 0 || p.CapacityUnit == "" || p.MaxLoad <= 0 || p.MinHeadroomPercent < 0 || p.MaxCost <= 0 || p.Currency == "" || seen[p.ID] {
			return false
		}
		seen[p.ID] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Delivery, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Delivery{}, ErrInvalid
	}
	owner := false
	for _, p := range in.Phases {
		owner = owner || contains(p.OperatorIDs, actor)
	}
	if !owner {
		return Delivery{}, ErrForbidden
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := Delivery{ID: id(), RepositoryID: repo, Revision: 1, Input: in, CreatorID: actor, CreatedAt: s.now().UTC(), ActivePhaseID: in.Phases[0].ID, State: "staged", Observations: []Observation{}, Controls: []Control{}}
	d = derive(d)
	return d, s.write(d)
}
func phase(d Delivery, pid string) (Phase, bool) {
	for _, p := range d.Phases {
		if p.ID == pid {
			return p, true
		}
	}
	return Phase{}, false
}
func latest(d Delivery, pid string) *Observation {
	for i := len(d.Observations) - 1; i >= 0; i-- {
		if d.Observations[i].PhaseID == pid {
			return &d.Observations[i]
		}
	}
	return nil
}
func derive(d Delivery) Delivery {
	d.Blockers = nil
	d.ObjectiveValidated = false
	d.ForecastValidated = false
	p, _ := phase(d, d.ActivePhaseID)
	o := latest(d, d.ActivePhaseID)
	add := func(k, detail, contain string) {
		d.Blockers = append(d.Blockers, Blocker{Kind: k, Detail: detail, Reference: d.DecisionRevisitID, Containment: contain})
	}
	if o == nil {
		add("missing_production_evidence", "current phase has no production-derived observation", "pause")
	} else {
		if len(o.ProductionEvidenceIDs) == 0 {
			add("missing_production_evidence", "observation has no current production evidence", "pause")
		}
		if o.Quota != "granted" {
			add("quota_denial", "required provider quota is not granted", "pause")
		}
		if o.RegionalImbalancePercent > o.MaxRegionalImbalancePercent {
			add("regional_imbalance", "regional imbalance exceeds the declared bound", "throttle")
		}
		if o.ScalingLagSeconds > o.MaxScalingLagSeconds {
			add("scaling_lag", "capacity arrived too slowly", "throttle")
		}
		if o.Correctness != "passed" {
			add("correctness_regression", "production correctness did not pass", "rollback")
		}
		if o.Reliability != "healthy" {
			add("reliability_regression", "production reliability is not healthy", "rollback")
		}
		for _, sl := range o.ServiceLevels {
			if !sl.Met {
				add("service_level_regression", sl.Name+" missed its target", "rollback")
			}
		}
		for _, dep := range o.Dependencies {
			if dep.Status != "healthy" {
				add("dependency_unhealthy", dep.DependencyID+" is "+dep.Status, "pause")
			}
		}
		if o.Cost > p.MaxCost {
			add("budget_breach", "observed phase cost exceeds its bound", "pause")
		}
		if o.ReservationUtilizationPercent < o.MinReservationUtilizationPercent {
			add("unused_reservation", "reserved capacity is materially unused", "replan")
		}
		if o.Load > p.MaxLoad || o.ForecastLoad > 0 && (o.Load > o.ForecastLoad*1.2 || o.Load < o.ForecastLoad*0.8) {
			add("demand_shift", "production demand differs materially from the bound forecast", "replan")
		}
		if o.UsableCapacity > o.AllocatedCapacity || o.Load > o.UsableCapacity || o.HeadroomPercent < p.MinHeadroomPercent {
			add("insufficient_usable_capacity", "allocated capacity has not produced required usable headroom", "pause")
		}
		d.ObjectiveValidated = len(d.Blockers) == 0 && o.Load <= o.UsableCapacity
		d.ForecastValidated = len(d.Blockers) == 0 && o.ForecastLoad > 0
	}
	if len(d.Blockers) > 0 {
		d.State = containmentState(d.Blockers[0].Containment)
		d.PredictedNextAction = d.Blockers[0].Containment + " phase " + d.ActivePhaseID + " and revisit decision " + d.DecisionRevisitID
	} else if d.State == "active" {
		d.PredictedNextAction = "stage the next plan phase after operator review of current proof"
	} else {
		d.PredictedNextAction = "resume the staged phase when its operator accepts current evidence"
	}
	if len(d.Controls) > 0 {
		c := d.Controls[len(d.Controls)-1]
		if allowed(c.Action, "pause", "throttle", "rollback", "replan") {
			d.State = containmentState(c.Action)
			d.PredictedNextAction = "operator containment is active; revisit decision " + d.DecisionRevisitID
		} else if c.Action == "resume" && len(d.Blockers) == 0 {
			d.State = "active"
			d.PredictedNextAction = "observe production load, usable capacity, service levels, dependencies, and cost"
		} else if c.Action == "stage" && len(d.Blockers) == 0 {
			d.State = "staged"
		}
	}
	d.NonAuthority = []string{"Delivery records and controls grant no spending, quota, provider, repository, agent, credential, release, environment, deployment, or operational authority; protected-environment and delegated-agent decisions remain authoritative."}
	return d
}

func containmentState(action string) string {
	switch action {
	case "pause":
		return "paused"
	case "throttle":
		return "throttled"
	case "rollback":
		return "rolled_back"
	case "replan":
		return "replanning"
	default:
		return action
	}
}
func (s *Store) mutate(repo, did string, expected int64, fn func(*Delivery) error) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo, did)
	if e != nil {
		return d, e
	}
	if d.Revision != expected {
		return Delivery{}, ErrConflict
	}
	if e = fn(&d); e != nil {
		return Delivery{}, e
	}
	d.Revision++
	d = derive(d)
	return d, s.write(d)
}
func (s *Store) Observe(repo, did, kind, actor string, in ObservationInput) (Delivery, error) {
	if !allowed(kind, "human", "agent") || actor == "" || in.PhaseID == "" || in.ReleaseRevision == "" || in.InfrastructureRevision == "" || in.EvidenceWindowStart.IsZero() || !in.EvidenceWindowEnd.After(in.EvidenceWindowStart) || in.AllocatedCapacity < 0 || in.UsableCapacity < 0 || in.Load < 0 || in.ForecastLoad < 0 || in.HeadroomPercent < 0 || in.Cost < 0 || !allowed(in.Correctness, "passed", "failed") || !allowed(in.Reliability, "healthy", "degraded", "regressed") || !allowed(in.Quota, "granted", "denied", "pending") {
		return Delivery{}, ErrInvalid
	}
	return s.mutate(repo, did, in.ExpectedRevision, func(d *Delivery) error {
		p, ok := phase(*d, in.PhaseID)
		if !ok {
			return ErrInvalid
		}
		if kind == "agent" && !contains(p.DelegatedAgentIDs, actor) {
			return ErrForbidden
		}
		d.Observations = append(d.Observations, Observation{ID: id(), ObservationInput: in, ActorKind: kind, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Control(repo, did, kind, actor string, in ControlInput) (Delivery, error) {
	if !allowed(kind, "human", "agent") || actor == "" || !allowed(in.Action, "stage", "pause", "resume", "throttle", "rollback", "replan") || in.PhaseID == "" || strings.TrimSpace(in.Rationale) == "" || len(in.EvidenceIDs) == 0 || in.Action == "throttle" && (in.ThrottlePercent <= 0 || in.ThrottlePercent > 100) {
		return Delivery{}, ErrInvalid
	}
	return s.mutate(repo, did, in.ExpectedRevision, func(d *Delivery) error {
		p, ok := phase(*d, in.PhaseID)
		if !ok {
			return ErrInvalid
		}
		if kind == "agent" {
			if !contains(p.DelegatedAgentIDs, actor) || !allowed(in.Action, "stage", "pause", "throttle") {
				return ErrForbidden
			}
		} else if !contains(p.OperatorIDs, actor) {
			return ErrForbidden
		}
		if in.Action == "resume" && len(derive(*d).Blockers) > 0 {
			return ErrConflict
		}
		d.ActivePhaseID = in.PhaseID
		d.Controls = append(d.Controls, Control{ID: id(), ControlInput: in, ActorKind: kind, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Get(repo, did string) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo, did)
	return derive(d), e
}
func (s *Store) List(repo, plan string) ([]Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fsx, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(fsx)
	out := []Delivery{}
	for _, f := range fsx {
		b, x := os.ReadFile(f)
		var d Delivery
		if x == nil {
			x = json.Unmarshal(b, &d)
		}
		if x != nil {
			return nil, x
		}
		if d.PlanID == plan {
			out = append(out, derive(d))
		}
	}
	return out, nil
}
func (s *Store) read(repo, id string) (Delivery, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Delivery{}, ErrNotFound
	}
	var d Delivery
	if e == nil {
		e = json.Unmarshal(b, &d)
	}
	return d, e
}
func (s *Store) write(d Delivery) error {
	dir := filepath.Join(s.root, d.RepositoryID)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, "delivery-*.tmp")
	if e != nil {
		return e
	}
	n := f.Name()
	defer os.Remove(n)
	if _, e = f.Write(b); e == nil {
		e = f.Chmod(0640)
	}
	if x := f.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(dir, d.ID+".json"))
	}
	return e
}
