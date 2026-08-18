package durableschemas

import (
	"sort"
	"time"
)

type SafetyPoint struct {
	Phase         string    `json:"phase"`
	Progress      float64   `json:"progress"`
	ObservationID string    `json:"observation_id"`
	Reasons       []string  `json:"reasons"`
	CreatedAt     time.Time `json:"created_at"`
}

type RecoveryPoint struct {
	ID             string           `json:"id"`
	Phase          string           `json:"phase"`
	Progress       float64          `json:"progress"`
	SchemaVersion  int64            `json:"schema_version"`
	ArtifactDigest string           `json:"artifact_digest"`
	Attestation    string           `json:"attestation"`
	Counts         map[string]int64 `json:"counts"`
	ActorID        string           `json:"actor_id"`
	CreatedAt      time.Time        `json:"created_at"`
}

type RecoveryPointInput struct {
	ExpectedRevision int64            `json:"expected_revision"`
	SchemaVersion    int64            `json:"schema_version"`
	ArtifactDigest   string           `json:"artifact_digest"`
	Attestation      string           `json:"attestation"`
	Counts           map[string]int64 `json:"counts"`
}

type RecoveryAction struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	ActorKind       string    `json:"actor_kind"`
	ActorID         string    `json:"actor_id"`
	RecoveryPointID string    `json:"recovery_point_id,omitempty"`
	Evidence        []string  `json:"evidence"`
	Reason          string    `json:"reason"`
	RepairKind      string    `json:"repair_kind,omitempty"`
	RepairID        string    `json:"repair_id,omitempty"`
	OwnerID         string    `json:"owner_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type RecoveryActionInput struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Kind             string   `json:"kind"`
	RecoveryPointID  string   `json:"recovery_point_id"`
	Evidence         []string `json:"evidence"`
	Reason           string   `json:"reason"`
	RepairKind       string   `json:"repair_kind"`
	RepairID         string   `json:"repair_id"`
	OwnerID          string   `json:"owner_id"`
}

type RetirementApproval struct {
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

type EnvironmentSchema struct {
	EnvironmentID    string `json:"environment_id"`
	EnvironmentName  string `json:"environment_name"`
	SchemaVersion    int64  `json:"schema_version"`
	DefinitionDigest string `json:"definition_digest"`
}

type RetirementCompletion struct {
	RetainedData          []string            `json:"retained_data"`
	DeletedData           []string            `json:"deleted_data"`
	DeletionEvidence      []string            `json:"deletion_evidence"`
	IrreversibleDecisions []string            `json:"irreversible_decisions"`
	Exceptions            []string            `json:"exceptions"`
	Environments          []EnvironmentSchema `json:"environments"`
	Cost                  float64             `json:"cost"`
	Currency              string              `json:"currency"`
	CompletedBy           string              `json:"completed_by"`
	CompletedAt           time.Time           `json:"completed_at"`
}

type Retirement struct {
	Revision             int64                 `json:"revision"`
	ExecutionID          string                `json:"execution_id"`
	CompatibilityCode    []string              `json:"compatibility_code"`
	ObsoleteFields       []string              `json:"obsolete_fields"`
	SuccessEvidence      []string              `json:"success_evidence"`
	ObservationStartedAt time.Time             `json:"observation_started_at"`
	ObservationEndsAt    time.Time             `json:"observation_ends_at"`
	RequiredOwnerIDs     []string              `json:"required_owner_ids"`
	Approvals            []RetirementApproval  `json:"approvals"`
	Blockers             []string              `json:"blockers"`
	Completion           *RetirementCompletion `json:"completion,omitempty"`
	OpenedBy             string                `json:"opened_by"`
	OpenedAt             time.Time             `json:"opened_at"`
}

type RetirementInput struct {
	ExecutionID          string    `json:"execution_id"`
	CompatibilityCode    []string  `json:"compatibility_code"`
	ObsoleteFields       []string  `json:"obsolete_fields"`
	SuccessEvidence      []string  `json:"success_evidence"`
	ObservationStartedAt time.Time `json:"observation_started_at"`
	ObservationEndsAt    time.Time `json:"observation_ends_at"`
	RequiredOwnerIDs     []string  `json:"required_owner_ids"`
}

type RetirementCompletionInput struct {
	ExpectedRevision      int64               `json:"expected_revision"`
	RetainedData          []string            `json:"retained_data"`
	DeletedData           []string            `json:"deleted_data"`
	DeletionEvidence      []string            `json:"deletion_evidence"`
	IrreversibleDecisions []string            `json:"irreversible_decisions"`
	Exceptions            []string            `json:"exceptions"`
	Environments          []EnvironmentSchema `json:"environments"`
	Cost                  float64             `json:"cost"`
	Currency              string              `json:"currency"`
}

func slicesContainFailed(xs []LiveInvariant) bool {
	for _, x := range xs {
		if !x.Passed {
			return true
		}
	}
	return false
}
func servicesRegressed(xs []ServiceHealth) bool {
	for _, x := range xs {
		if x.Status != "healthy" {
			return true
		}
	}
	return false
}

func deriveRetirement(x Retirement, now time.Time) Retirement {
	x.Blockers = nil
	if now.Before(x.ObservationEndsAt) {
		x.Blockers = append(x.Blockers, "observation_period_incomplete")
	}
	decisions := map[string]string{}
	for _, a := range x.Approvals {
		decisions[a.OwnerID] = a.Decision
	}
	for _, owner := range x.RequiredOwnerIDs {
		if decisions[owner] != "approved" {
			x.Blockers = append(x.Blockers, "owner_approval_required:"+owner)
		}
	}
	sort.Strings(x.Blockers)
	return x
}

func (s *Store) RecordRecoveryPoint(repo, migration, execution, actor string, in RecoveryPointInput) (Migration, error) {
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
	if q.State != "running" || in.SchemaVersion < 1 || in.ArtifactDigest == "" || in.Attestation == "" || len(in.Counts) == 0 {
		return Migration{}, ErrInvalid
	}
	for name, count := range in.Counts {
		if name == "" || count < 0 {
			return Migration{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	q.Revision++
	q.RecoveryPoints = append(q.RecoveryPoints, RecoveryPoint{ID: id(), Phase: q.Phase, Progress: q.Progress, SchemaVersion: in.SchemaVersion, ArtifactDigest: in.ArtifactDigest, Attestation: in.Attestation, Counts: in.Counts, ActorID: actor, CreatedAt: now})
	q.UpdatedAt = now
	x.Executions[i] = deriveExecution(q)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "recovery_point_attested", ActorID: actor, Detail: q.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) RecoverExecution(repo, migration, execution, actor string, in RecoveryActionInput) (Migration, error) {
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
	if q.Revision != in.ExpectedRevision {
		return Migration{}, ErrConflict
	}
	if q.State != "paused" || in.Reason == "" || !nonempty(in.Evidence, true) || !map[string]bool{"retry": true, "restore": true, "traffic_rollback": true, "open_repair": true}[in.Kind] {
		return Migration{}, ErrInvalid
	}
	if agentActor(actor) && in.Kind != "open_repair" {
		return Migration{}, ErrInvalid
	}
	if in.Kind != "open_repair" && len(deriveMigration(x).Blockers) != 0 {
		return Migration{}, ErrInvalid
	}
	point := -1
	for j, p := range q.RecoveryPoints {
		if p.ID == in.RecoveryPointID {
			point = j
		}
	}
	if in.Kind == "restore" && point < 0 || in.Kind == "traffic_rollback" && (q.PhaseIndex >= 3 || !s.now().UTC().Before(q.CompatibilityEndsAt)) {
		return Migration{}, ErrInvalid
	}
	if in.Kind == "open_repair" && (!map[string]bool{"task": true, "session": true, "workspace": true}[in.RepairKind] || in.RepairID == "" || in.OwnerID == "") {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	action := RecoveryAction{ID: id(), Kind: in.Kind, ActorKind: "human", ActorID: actor, RecoveryPointID: in.RecoveryPointID, Evidence: in.Evidence, Reason: in.Reason, RepairKind: in.RepairKind, RepairID: in.RepairID, OwnerID: in.OwnerID, CreatedAt: now}
	if agentActor(actor) {
		action.ActorKind = "agent"
	}
	q.RecoveryActions = append(q.RecoveryActions, action)
	if in.Kind == "retry" {
		q.State = "running"
		q.SafetyPoint = nil
	}
	if in.Kind == "restore" {
		p := q.RecoveryPoints[point]
		q.Phase = p.Phase
		for j, v := range livePhases {
			if v == p.Phase {
				q.PhaseIndex = j
			}
		}
		q.Progress = p.Progress
		q.State = "running"
		q.SafetyPoint = nil
	}
	if in.Kind == "traffic_rollback" {
		q.State = "aborted"
	}
	q.Revision++
	q.UpdatedAt = now
	x.Executions[i] = deriveExecution(q)
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "recovery_" + in.Kind, ActorID: actor, Detail: action.ID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) OpenRetirement(repo, migration, actor string, in RetirementInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if x.Retirement != nil || !nonempty(in.CompatibilityCode, true) || !nonempty(in.ObsoleteFields, true) || !nonempty(in.SuccessEvidence, true) || !nonempty(in.RequiredOwnerIDs, true) || in.ObservationStartedAt.IsZero() || !in.ObservationEndsAt.After(in.ObservationStartedAt) {
		return Migration{}, ErrInvalid
	}
	i := executionIndex(x, in.ExecutionID)
	if i < 0 || x.Executions[i].State != "completed" {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	r := Retirement{Revision: 1, ExecutionID: in.ExecutionID, CompatibilityCode: in.CompatibilityCode, ObsoleteFields: in.ObsoleteFields, SuccessEvidence: in.SuccessEvidence, ObservationStartedAt: in.ObservationStartedAt, ObservationEndsAt: in.ObservationEndsAt, RequiredOwnerIDs: in.RequiredOwnerIDs, OpenedBy: actor, OpenedAt: now}
	r = deriveRetirement(r, now)
	x.Retirement = &r
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "retirement_opened", ActorID: actor, Detail: in.ExecutionID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) ApproveRetirement(repo, migration, actor, owner, decision, rationale string, expected int64) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if x.Retirement == nil {
		return Migration{}, ErrNotFound
	}
	r := *x.Retirement
	if r.Revision != expected {
		return Migration{}, ErrConflict
	}
	allowed := false
	for _, v := range r.RequiredOwnerIDs {
		allowed = allowed || v == owner
	}
	if !allowed || actor != owner || rationale == "" || !map[string]bool{"approved": true, "rejected": true}[decision] {
		return Migration{}, ErrInvalid
	}
	now := s.now().UTC()
	r.Revision++
	r.Approvals = append(r.Approvals, RetirementApproval{OwnerID: owner, Decision: decision, Rationale: rationale, ActorID: actor, CreatedAt: now})
	r = deriveRetirement(r, now)
	x.Retirement = &r
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "retirement_" + decision, ActorID: actor, Detail: owner, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}

func (s *Store) CompleteRetirement(repo, migration, actor string, in RetirementCompletionInput) (Migration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Migration
	if e := load(s.migrationPath(repo, migration), &x); e != nil {
		return x, e
	}
	if x.Retirement == nil {
		return Migration{}, ErrNotFound
	}
	r := deriveRetirement(*x.Retirement, s.now().UTC())
	if r.Revision != in.ExpectedRevision {
		return Migration{}, ErrConflict
	}
	if r.Completion != nil || len(r.Blockers) > 0 || !nonempty(in.RetainedData, true) || !nonempty(in.DeletedData, true) || !nonempty(in.DeletionEvidence, true) || !nonempty(in.IrreversibleDecisions, true) || len(in.Environments) == 0 || in.Cost < 0 || in.Currency == "" {
		return Migration{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, e := range in.Environments {
		if e.EnvironmentID == "" || e.EnvironmentName == "" || e.SchemaVersion != x.ToVersion || e.DefinitionDigest == "" || seen[e.EnvironmentID] {
			return Migration{}, ErrInvalid
		}
		completed := false
		for _, execution := range x.Executions {
			completed = completed || execution.EnvironmentID == e.EnvironmentID && execution.EnvironmentName == e.EnvironmentName && execution.State == "completed"
		}
		if !completed {
			return Migration{}, ErrInvalid
		}
		seen[e.EnvironmentID] = true
	}
	now := s.now().UTC()
	r.Revision++
	r.Completion = &RetirementCompletion{RetainedData: in.RetainedData, DeletedData: in.DeletedData, DeletionEvidence: in.DeletionEvidence, IrreversibleDecisions: in.IrreversibleDecisions, Exceptions: in.Exceptions, Environments: in.Environments, Cost: in.Cost, Currency: in.Currency, CompletedBy: actor, CompletedAt: now}
	x.Retirement = &r
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Kind: "retirement_completed", ActorID: actor, Detail: r.ExecutionID, CreatedAt: now})
	return deriveMigration(x), save(s.migrationPath(repo, migration), x)
}
