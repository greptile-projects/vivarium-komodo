package durableschemas

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var livePhases = []string{"expand", "deploy", "backfill", "cutover", "contract"}

type LiveInvariant struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type ServiceHealth struct {
	Service string  `json:"service"`
	Status  string  `json:"status"`
	Value   float64 `json:"value,omitempty"`
	Unit    string  `json:"unit,omitempty"`
	Detail  string  `json:"detail"`
}

type LiveObservation struct {
	ID              string          `json:"id"`
	Phase           string          `json:"phase"`
	ActorID         string          `json:"actor_id"`
	Progress        float64         `json:"progress"`
	Lag             float64         `json:"lag"`
	LagUnit         string          `json:"lag_unit"`
	Invariants      []LiveInvariant `json:"invariants"`
	ServiceHealth   []ServiceHealth `json:"service_health"`
	PrivacyStatus   string          `json:"privacy_status"`
	PrivacyDetail   string          `json:"privacy_detail"`
	IncrementalCost float64         `json:"incremental_cost"`
	Summary         string          `json:"summary"`
	DeploymentID    string          `json:"deployment_id,omitempty"`
	FailureKinds    []string        `json:"failure_kinds,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

type LiveControl struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason"`
	Throttle  int       `json:"throttle,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Execution struct {
	ID                  string            `json:"id"`
	Revision            int64             `json:"revision"`
	EnvironmentID       string            `json:"environment_id"`
	EnvironmentName     string            `json:"environment_name"`
	ActiveRevision      string            `json:"active_revision"`
	ControllerKind      string            `json:"controller_kind"`
	ControllerID        string            `json:"controller_id"`
	DelegatedWorkItemID string            `json:"delegated_work_item_id,omitempty"`
	CompatibilityEndsAt time.Time         `json:"compatibility_ends_at"`
	MaximumCost         float64           `json:"maximum_cost"`
	Currency            string            `json:"currency"`
	Phase               string            `json:"phase"`
	PhaseIndex          int               `json:"phase_index"`
	State               string            `json:"state"`
	Throttle            int               `json:"throttle"`
	Progress            float64           `json:"progress"`
	Lag                 float64           `json:"lag"`
	LagUnit             string            `json:"lag_unit"`
	Invariants          []LiveInvariant   `json:"invariants"`
	ServiceHealth       []ServiceHealth   `json:"service_health"`
	PrivacyConstraints  []string          `json:"privacy_constraints"`
	PrivacyStatus       string            `json:"privacy_status"`
	Cost                float64           `json:"cost"`
	Blockers            []string          `json:"blockers"`
	NextActions         []string          `json:"next_actions"`
	Observations        []LiveObservation `json:"observations"`
	Controls            []LiveControl     `json:"controls"`
	SafetyPoint         *SafetyPoint      `json:"safety_point,omitempty"`
	RecoveryPoints      []RecoveryPoint   `json:"recovery_points"`
	RecoveryActions     []RecoveryAction  `json:"recovery_actions"`
	StartedBy           string            `json:"started_by"`
	StartedAt           time.Time         `json:"started_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

type StartExecutionInput struct {
	EnvironmentID       string    `json:"environment_id"`
	EnvironmentName     string    `json:"environment_name"`
	ActiveRevision      string    `json:"active_revision"`
	ControllerKind      string    `json:"controller_kind"`
	ControllerID        string    `json:"controller_id"`
	DelegatedWorkItemID string    `json:"delegated_work_item_id"`
	CompatibilityEndsAt time.Time `json:"compatibility_ends_at"`
	MaximumCost         float64   `json:"maximum_cost"`
	Currency            string    `json:"currency"`
	PrivacyConstraints  []string  `json:"privacy_constraints"`
}

type ObservationInput struct {
	ExpectedRevision int64           `json:"expected_revision"`
	Progress         float64         `json:"progress"`
	Lag              float64         `json:"lag"`
	LagUnit          string          `json:"lag_unit"`
	Invariants       []LiveInvariant `json:"invariants"`
	ServiceHealth    []ServiceHealth `json:"service_health"`
	PrivacyStatus    string          `json:"privacy_status"`
	PrivacyDetail    string          `json:"privacy_detail"`
	IncrementalCost  float64         `json:"incremental_cost"`
	Summary          string          `json:"summary"`
	DeploymentID     string          `json:"deployment_id"`
	FailureKinds     []string        `json:"failure_kinds"`
}

type ControlInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Kind             string `json:"kind"`
	Reason           string `json:"reason"`
	Throttle         int    `json:"throttle"`
}

func acceptedCurrentRehearsal(x Migration) bool {
	for _, r := range x.Rehearsals {
		r = deriveRehearsal(r)
		if len(r.Blockers) != 0 || len(r.Attempts) == 0 {
			continue
		}
		latest := r.Attempts[len(r.Attempts)-1].ID
		for _, a := range r.Attestations {
			if a.AttemptID == latest && a.Decision == "accepted" && !a.Stale {
				return true
			}
		}
	}
	return false
}

func deriveExecution(x Execution) Execution {
	x.Blockers = nil
	if time.Now().UTC().After(x.CompatibilityEndsAt) && x.State != "completed" && x.State != "aborted" {
		x.Blockers = append(x.Blockers, "compatibility_window_expired")
	}
	if x.Cost > x.MaximumCost {
		x.Blockers = append(x.Blockers, "cost_limit_exceeded")
	}
	for _, q := range x.Invariants {
		if !q.Passed {
			x.Blockers = append(x.Blockers, "invariant_failed:"+q.Name)
		}
	}
	for _, h := range x.ServiceHealth {
		if h.Status != "healthy" {
			x.Blockers = append(x.Blockers, "service_unhealthy:"+h.Service)
		}
	}
	if x.PrivacyStatus != "compliant" && len(x.Observations) > 0 {
		x.Blockers = append(x.Blockers, "privacy_constraint_failed")
	}
	sort.Strings(x.Blockers)
	x.NextActions = nil
	switch x.State {
	case "running":
		x.NextActions = append(x.NextActions, "pause", "throttle")
		if len(x.Blockers) == 0 && x.Progress == 100 {
			if x.Phase == "contract" {
				x.NextActions = append(x.NextActions, "complete")
			} else {
				x.NextActions = append(x.NextActions, "advance_to:"+livePhases[x.PhaseIndex+1])
			}
		}
		if x.PhaseIndex < 3 {
			x.NextActions = append(x.NextActions, "abort")
		}
	case "paused":
		x.NextActions = []string{"retry_idempotently", "restore_attested_recovery_point", "open_repair", "throttle"}
		if x.PhaseIndex < 3 {
			x.NextActions = append(x.NextActions, "roll_back_application_traffic", "abort")
		}
	}
	return x
}

func executionIndex(x Migration, execution string) int {
	for i := range x.Executions {
		if x.Executions[i].ID == execution {
			return i
		}
	}
	return -1
}

func agentActor(actor string) bool { return strings.HasPrefix(actor, "agent:") }

func (s *Store) StartExecution(repo, migration, actor string, in StartExecutionInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	x = deriveMigration(x)
	if len(x.Blockers) != 0 || !acceptedCurrentRehearsal(x) || in.EnvironmentID == "" || in.EnvironmentName == "" || in.ActiveRevision == "" || in.ControllerID == "" || in.ControllerID != actor || !map[string]bool{"human": true, "agent": true}[in.ControllerKind] || agentActor(actor) != (in.ControllerKind == "agent") || !in.CompatibilityEndsAt.After(s.now().UTC()) || in.MaximumCost <= 0 || in.Currency == "" || !nonempty(in.PrivacyConstraints, true) {
		return Migration{}, ErrInvalid
	}
	if len(x.Executions) > 0 {
		q := deriveExecution(x.Executions[len(x.Executions)-1])
		if q.State != "completed" && q.State != "aborted" {
			return Migration{}, ErrConflict
		}
	}
	if in.ControllerKind == "agent" {
		found := false
		for _, w := range x.WorkItems {
			found = found || (w.ID == in.DelegatedWorkItemID && w.OwnerKind == "agent" && w.OwnerID == actor && w.Phase == "schema")
		}
		if !found {
			return Migration{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	q := Execution{ID: id(), Revision: 1, EnvironmentID: in.EnvironmentID, EnvironmentName: in.EnvironmentName, ActiveRevision: in.ActiveRevision, ControllerKind: in.ControllerKind, ControllerID: actor, DelegatedWorkItemID: in.DelegatedWorkItemID, CompatibilityEndsAt: in.CompatibilityEndsAt, MaximumCost: in.MaximumCost, Currency: in.Currency, Phase: "expand", State: "running", Throttle: 100, PrivacyConstraints: in.PrivacyConstraints, PrivacyStatus: "unknown", StartedBy: actor, StartedAt: now, UpdatedAt: now}
	q = deriveExecution(q)
	x.Executions = append(x.Executions, q)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "execution_started", ActorID: actor, Detail: q.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) ObserveExecution(repo, migration, execution, actor string, in ObservationInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := executionIndex(x, execution)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	q := x.Executions[i]
	if q.Revision != in.ExpectedRevision {
		return Migration{}, ErrConflict
	}
	if q.State != "running" || in.Progress < q.Progress || in.Progress > 100 || in.Lag < 0 || in.LagUnit == "" || in.Summary == "" || in.IncrementalCost < 0 || !map[string]bool{"compliant": true, "violated": true, "unknown": true}[in.PrivacyStatus] || len(in.Invariants) == 0 || len(in.ServiceHealth) == 0 {
		return Migration{}, ErrInvalid
	}
	if agentActor(actor) {
		delegated := false
		for _, w := range x.WorkItems {
			delegated = delegated || (w.ID == q.DelegatedWorkItemID && w.OwnerKind == "agent" && w.OwnerID == actor && q.Phase == "expand" && w.Phase == "schema")
		}
		if !delegated {
			return Migration{}, ErrInvalid
		}
	}
	for _, v := range in.Invariants {
		if v.Name == "" || v.Detail == "" {
			return Migration{}, ErrInvalid
		}
	}
	for _, v := range in.ServiceHealth {
		if v.Service == "" || v.Detail == "" || !map[string]bool{"healthy": true, "degraded": true, "unhealthy": true}[v.Status] {
			return Migration{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	validFailures := map[string]bool{"invariant_failure": true, "service_regression": true, "conflicting_writes": true, "capacity_exhaustion": true, "interrupted_backfill": true}
	for _, failure := range in.FailureKinds {
		if !validFailures[failure] {
			return Migration{}, ErrInvalid
		}
	}
	o := LiveObservation{ID: id(), Phase: q.Phase, ActorID: actor, Progress: in.Progress, Lag: in.Lag, LagUnit: in.LagUnit, Invariants: in.Invariants, ServiceHealth: in.ServiceHealth, PrivacyStatus: in.PrivacyStatus, PrivacyDetail: in.PrivacyDetail, IncrementalCost: in.IncrementalCost, Summary: in.Summary, DeploymentID: in.DeploymentID, FailureKinds: in.FailureKinds, CreatedAt: now}
	q.Revision++
	q.Progress = in.Progress
	q.Lag = in.Lag
	q.LagUnit = in.LagUnit
	q.Invariants = in.Invariants
	q.ServiceHealth = in.ServiceHealth
	q.PrivacyStatus = in.PrivacyStatus
	q.Cost += in.IncrementalCost
	q.Observations = append(q.Observations, o)
	if len(in.FailureKinds) > 0 || slicesContainFailed(in.Invariants) || servicesRegressed(in.ServiceHealth) || q.Cost > q.MaximumCost {
		q.State = "paused"
		reasons := append([]string(nil), in.FailureKinds...)
		if slicesContainFailed(in.Invariants) {
			reasons = append(reasons, "invariant_failure")
		}
		if servicesRegressed(in.ServiceHealth) {
			reasons = append(reasons, "service_regression")
		}
		if q.Cost > q.MaximumCost {
			reasons = append(reasons, "capacity_exhaustion")
		}
		q.SafetyPoint = &SafetyPoint{Phase: q.Phase, Progress: q.Progress, ObservationID: o.ID, Reasons: reasons, CreatedAt: now}
	}
	q.UpdatedAt = now
	q = deriveExecution(q)
	x.Executions[i] = q
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "execution_observed", ActorID: actor, Detail: q.ID + ":" + q.Phase, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) ControlExecution(repo, migration, execution, actor string, in ControlInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := executionIndex(x, execution)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	q := deriveExecution(x.Executions[i])
	if agentActor(actor) {
		return Migration{}, ErrInvalid
	}
	if q.Revision != in.ExpectedRevision {
		return Migration{}, ErrConflict
	}
	if in.Reason == "" || !map[string]bool{"pause": true, "resume": true, "throttle": true, "abort": true}[in.Kind] {
		return Migration{}, ErrInvalid
	}
	if in.Kind == "pause" && q.State != "running" && q.State != "paused" || in.Kind == "resume" && q.State != "paused" || in.Kind == "throttle" && (q.State != "running" && q.State != "paused") || in.Kind == "abort" && (q.State == "completed" || q.State == "aborted" || q.PhaseIndex >= 3) {
		return Migration{}, ErrInvalid
	}
	if in.Kind == "resume" && len(deriveMigration(x).Blockers) != 0 {
		return Migration{}, ErrInvalid
	}
	if in.Kind == "throttle" && (in.Throttle < 1 || in.Throttle > 100) {
		return Migration{}, ErrInvalid
	}
	switch in.Kind {
	case "pause":
		q.State = "paused"
	case "resume":
		q.State = "running"
	case "throttle":
		q.Throttle = in.Throttle
	case "abort":
		q.State = "aborted"
	}
	now := s.now().UTC()
	q.Revision++
	q.UpdatedAt = now
	q.Controls = append(q.Controls, LiveControl{Sequence: int64(len(q.Controls) + 1), Kind: in.Kind, ActorID: actor, Reason: in.Reason, Throttle: in.Throttle, CreatedAt: now})
	q = deriveExecution(q)
	x.Executions[i] = q
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "execution_" + in.Kind, ActorID: actor, Detail: q.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) AdvanceExecution(repo, migration, execution, actor string, expected int64) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	i := executionIndex(x, execution)
	if i < 0 {
		return Migration{}, ErrNotFound
	}
	q := deriveExecution(x.Executions[i])
	if agentActor(actor) {
		return Migration{}, ErrInvalid
	}
	if q.Revision != expected {
		return Migration{}, ErrConflict
	}
	if q.State != "running" || q.Progress != 100 || len(q.Blockers) != 0 {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	q.Revision++
	if q.Phase == "contract" {
		q.State = "completed"
	} else {
		q.PhaseIndex++
		q.Phase = livePhases[q.PhaseIndex]
		q.Progress = 0
		q.Lag = 0
		q.LagUnit = ""
		q.Invariants = nil
		q.ServiceHealth = nil
		q.PrivacyStatus = "unknown"
	}
	q.UpdatedAt = now
	q = deriveExecution(q)
	x.Executions[i] = q
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "execution_advanced", ActorID: actor, Detail: fmt.Sprintf("%s:%s", q.ID, q.Phase), CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}
