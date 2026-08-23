// Package regressioninvestigations owns durable, shared boundaries for locating regressions.
package regressioninvestigations

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("regression investigation not found")
var ErrConflict = errors.New("regression investigation conflict")

type Source struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision,omitempty"`
}
type Boundary struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	CommitID  string `json:"commit_id"`
	ReleaseID string `json:"release_id,omitempty"`
}
type Evidence struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision,omitempty"`
	Summary    string    `json:"summary"`
	Audience   string    `json:"audience"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Entry struct {
	ID        string    `json:"id"`
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Scope struct {
	ExpectedBehavior   string   `json:"expected_behavior"`
	RegressedBehavior  string   `json:"regressed_behavior"`
	KnownGood          Boundary `json:"known_good"`
	KnownBad           Boundary `json:"known_bad"`
	Environments       []string `json:"environments"`
	Comparability      string   `json:"comparability"`
	Severity           string   `json:"severity"`
	OwnerIDs           []string `json:"owner_ids"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type ScopeChange struct {
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason"`
	Scope     Scope     `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}
type ScenarioInput struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Value          string `json:"value,omitempty"`
	SourceRevision string `json:"source_revision,omitempty"`
}
type Fixture struct {
	Name           string `json:"name"`
	Reference      string `json:"reference"`
	Classification string `json:"classification"`
	Transformation string `json:"transformation,omitempty"`
}
type ScenarioDefinition struct {
	Title                   string          `json:"title"`
	ExpectedBehavior        string          `json:"expected_behavior"`
	RegressedBehavior       string          `json:"regressed_behavior"`
	Inputs                  []ScenarioInput `json:"inputs"`
	Commands                []string        `json:"commands"`
	Fixtures                []Fixture       `json:"fixtures"`
	EnvironmentRequirements []string        `json:"environment_requirements"`
	TimeoutSeconds          int64           `json:"timeout_seconds"`
	CostLimit               float64         `json:"cost_limit"`
}
type Scenario struct {
	ID                   string             `json:"id"`
	Version              int64              `json:"version"`
	InvestigationVersion int64              `json:"investigation_version"`
	Derived              bool               `json:"derived"`
	Definition           ScenarioDefinition `json:"definition"`
	CreatedByID          string             `json:"created_by_id"`
	CreatedAt            time.Time          `json:"created_at"`
}
type Target struct {
	Kind              string            `json:"kind"`
	Reference         string            `json:"reference,omitempty"`
	CommitID          string            `json:"commit_id,omitempty"`
	ReleaseID         string            `json:"release_id,omitempty"`
	AttestationDigest string            `json:"attestation_digest,omitempty"`
	Dependencies      map[string]string `json:"dependencies,omitempty"`
}
type Environment struct {
	Image                string            `json:"image"`
	DefinitionDigest     string            `json:"definition_digest"`
	OS                   string            `json:"os"`
	Architecture         string            `json:"architecture"`
	Isolation            string            `json:"isolation"`
	Network              string            `json:"network"`
	Toolchain            map[string]string `json:"toolchain"`
	DependencyLockDigest string            `json:"dependency_lock_digest,omitempty"`
	SetupCommands        []string          `json:"setup_commands"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
}
type Provenance struct {
	RunnerID        string `json:"runner_id"`
	RunnerVersion   string `json:"runner_version"`
	ActorKind       string `json:"actor_kind"`
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at"`
	RepetitionCount int64  `json:"repetition_count"`
}
type AttemptInput struct {
	Target         Target          `json:"target"`
	Environment    Environment     `json:"environment"`
	Inputs         []ScenarioInput `json:"inputs"`
	Commands       []string        `json:"commands"`
	Outputs        []string        `json:"outputs"`
	Logs           []string        `json:"logs"`
	Artifacts      []Artifact      `json:"artifacts"`
	Classification string          `json:"classification"`
	Rationale      string          `json:"rationale"`
	Cost           float64         `json:"cost"`
	Currency       string          `json:"currency"`
	Provenance     Provenance      `json:"provenance"`
}
type Attempt struct {
	ID              string `json:"id"`
	ScenarioID      string `json:"scenario_id"`
	ScenarioVersion int64  `json:"scenario_version"`
	AttemptInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type SearchRevision struct {
	Key          string   `json:"key"`
	Kind         string   `json:"kind"`
	RepositoryID string   `json:"repository_id,omitempty"`
	Package      string   `json:"package,omitempty"`
	Revision     string   `json:"revision"`
	Parents      []string `json:"parents"`
	Summary      string   `json:"summary,omitempty"`
	DiffPaths    []string `json:"diff_paths,omitempty"`
	OwnerIDs     []string `json:"owner_ids,omitempty"`
	PullIDs      []string `json:"pull_request_ids,omitempty"`
	DecisionIDs  []string `json:"decision_ids,omitempty"`
}
type SearchInput struct {
	ScenarioID       string           `json:"scenario_id"`
	GoodKey          string           `json:"good_key"`
	BadKey           string           `json:"bad_key"`
	Revisions        []SearchRevision `json:"revisions"`
	ConfidenceTarget float64          `json:"confidence_target"`
}
type CandidateClassification struct {
	ID             string    `json:"id"`
	RevisionKey    string    `json:"revision_key"`
	AttemptIDs     []string  `json:"attempt_ids"`
	Classification string    `json:"classification"`
	Rationale      string    `json:"rationale"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type CulpritRange struct {
	GoodKey    string  `json:"good_key"`
	BadKey     string  `json:"bad_key"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
}
type CausalHypothesis struct {
	ID           string    `json:"id"`
	RevisionKeys []string  `json:"revision_keys"`
	Body         string    `json:"body"`
	EvidenceIDs  []string  `json:"evidence_ids"`
	DiffPaths    []string  `json:"diff_paths"`
	Confidence   float64   `json:"confidence"`
	ActorID      string    `json:"actor_id"`
	ActorKind    string    `json:"actor_kind"`
	State        string    `json:"state"`
	CreatedAt    time.Time `json:"created_at"`
}
type ResponseOption struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	Title            string   `json:"title"`
	Summary          string   `json:"summary"`
	Tradeoffs        []string `json:"tradeoffs"`
	AffectedReleases []string `json:"affected_releases"`
	AffectedWork     []string `json:"affected_current_work"`
	BackportTargets  []string `json:"backport_targets,omitempty"`
	EvidenceIDs      []string `json:"evidence_ids"`
}
type ResponsePlanInput struct {
	SearchID           string           `json:"search_id"`
	CulpritGoodKey     string           `json:"culprit_good_key"`
	CulpritBadKey      string           `json:"culprit_bad_key"`
	ReproductionIDs    []string         `json:"reproduction_ids"`
	Constraints        []string         `json:"constraints"`
	AcceptanceCriteria []string         `json:"acceptance_criteria"`
	OriginalIntent     string           `json:"original_change_intent"`
	OriginalAuthorIDs  []string         `json:"original_author_ids"`
	Options            []ResponseOption `json:"options"`
	SelectedOptionID   string           `json:"selected_option_id,omitempty"`
	Rationale          string           `json:"rationale,omitempty"`
}
type ResponseWork struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	ResourceID         string    `json:"resource_id"`
	OwnerID            string    `json:"owner_id"`
	OwnerKind          string    `json:"owner_kind"`
	OptionID           string    `json:"option_id"`
	PullRequestID      string    `json:"pull_request_id,omitempty"`
	Published          bool      `json:"published"`
	Intent             string    `json:"original_change_intent"`
	AuthorIDs          []string  `json:"original_author_ids"`
	Tradeoffs          []string  `json:"tradeoffs"`
	BackportTargets    []string  `json:"backport_targets,omitempty"`
	CulpritRange       []string  `json:"culprit_range"`
	ReproductionIDs    []string  `json:"reproduction_ids"`
	Constraints        []string  `json:"constraints"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	CreatedByID        string    `json:"created_by_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type ResponsePlan struct {
	ID string `json:"id"`
	ResponsePlanInput
	Work        []ResponseWork `json:"work"`
	CreatedByID string         `json:"created_by_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
type ProofCheck struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	EvidenceID string `json:"evidence_id,omitempty"`
}
type CorrectionCandidateInput struct {
	ResponseID        string   `json:"response_id"`
	WorkID            string   `json:"work_id"`
	Kind              string   `json:"kind"`
	Target            Target   `json:"target"`
	ScenarioID        string   `json:"scenario_id"`
	AffectedChecks    []string `json:"affected_checks"`
	RequirementIDs    []string `json:"requirement_ids"`
	ChangeCriteria    []string `json:"original_change_acceptance_criteria"`
	QualityPlanID     string   `json:"quality_plan_id,omitempty"`
	RequiredCheckName string   `json:"required_check_name,omitempty"`
}
type CorrectionProof struct {
	ID                 string       `json:"id"`
	ScenarioAttemptID  string       `json:"scenario_attempt_id"`
	BaselineAttemptIDs []string     `json:"baseline_attempt_ids"`
	Checks             []ProofCheck `json:"checks"`
	Revision           string       `json:"revision"`
	ActorID            string       `json:"actor_id"`
	CreatedAt          time.Time    `json:"created_at"`
}
type DeliveryEvent struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type CorrectionCandidate struct {
	ID string `json:"id"`
	CorrectionCandidateInput
	InvestigationVersion int64             `json:"investigation_version"`
	ScenarioVersion      int64             `json:"scenario_version"`
	OriginalIntent       string            `json:"original_change_intent"`
	AcceptanceCriteria   []string          `json:"regression_acceptance_criteria"`
	Proofs               []CorrectionProof `json:"proofs"`
	Delivery             []DeliveryEvent   `json:"delivery"`
	State                string            `json:"state"`
	Blockers             []string          `json:"blockers"`
	ReopenedReason       string            `json:"reopened_reason,omitempty"`
	CreatedByID          string            `json:"created_by_id"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}
type Search struct {
	ID               string                    `json:"id"`
	ScenarioID       string                    `json:"scenario_id"`
	GoodKey          string                    `json:"good_key"`
	BadKey           string                    `json:"bad_key"`
	ConfidenceTarget float64                   `json:"confidence_target"`
	Revisions        []SearchRevision          `json:"revisions"`
	Classifications  []CandidateClassification `json:"classifications"`
	Hypotheses       []CausalHypothesis        `json:"causal_hypotheses"`
	RemainingKeys    []string                  `json:"remaining_search_space"`
	ScheduledKeys    []string                  `json:"scheduled_candidates"`
	Ranges           []CulpritRange            `json:"culprit_ranges"`
	Verdict          string                    `json:"verdict"`
	Blockers         []string                  `json:"blockers"`
	GraphDigest      string                    `json:"graph_digest"`
	CreatedByID      string                    `json:"created_by_id"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}
type Investigation struct {
	ID           string                `json:"id"`
	RepositoryID string                `json:"repository_id"`
	Title        string                `json:"title"`
	Source       Source                `json:"source"`
	CreatorID    string                `json:"creator_id"`
	Version      int64                 `json:"version"`
	Scope        Scope                 `json:"scope"`
	Status       string                `json:"status"`
	Blockers     []string              `json:"blockers"`
	StaleInputs  []string              `json:"stale_inputs"`
	Evidence     []Evidence            `json:"evidence"`
	Entries      []Entry               `json:"entries"`
	ScopeChanges []ScopeChange         `json:"scope_changes"`
	Scenarios    []Scenario            `json:"scenarios"`
	Attempts     []Attempt             `json:"attempts"`
	Searches     []Search              `json:"searches"`
	Responses    []ResponsePlan        `json:"responses"`
	Corrections  []CorrectionCandidate `json:"correction_candidates"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

func (s *Store) CreateCorrection(repo, key, actor string, in CorrectionCandidateInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	in.ResponseID, in.WorkID, in.Kind, in.ScenarioID = strings.TrimSpace(in.ResponseID), strings.TrimSpace(in.WorkID), strings.TrimSpace(in.Kind), strings.TrimSpace(in.ScenarioID)
	in.AffectedChecks, in.RequirementIDs, in.ChangeCriteria = clean(in.AffectedChecks), clean(in.RequirementIDs), clean(in.ChangeCriteria)
	var response *ResponsePlan
	var work *ResponseWork
	for i := range v.Responses {
		if v.Responses[i].ID == in.ResponseID {
			response = &v.Responses[i]
		}
	}
	if response != nil {
		for i := range response.Work {
			if response.Work[i].ID == in.WorkID {
				work = &response.Work[i]
			}
		}
	}
	var scenario *Scenario
	for i := range v.Scenarios {
		if v.Scenarios[i].ID == in.ScenarioID {
			scenario = &v.Scenarios[i]
		}
	}
	if response == nil || work == nil || scenario == nil || !map[string]bool{"repair": true, "backport": true}[in.Kind] || in.Target.CommitID == "" || len(in.AffectedChecks) == 0 || len(in.RequirementIDs) == 0 || len(in.ChangeCriteria) == 0 || (in.RequiredCheckName != "" && in.QualityPlanID == "") {
		return Investigation{}, ErrConflict
	}
	if in.Kind == "backport" && len(work.BackportTargets) == 0 {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	c := CorrectionCandidate{ID: id(), CorrectionCandidateInput: in, InvestigationVersion: v.Version, ScenarioVersion: scenario.Version, OriginalIntent: response.OriginalIntent, AcceptanceCriteria: append([]string{}, response.AcceptanceCriteria...), State: "awaiting_proof", CreatedByID: actor, CreatedAt: now, UpdatedAt: now}
	deriveCorrection(&c, v)
	v.Corrections = append(v.Corrections, c)
	v.UpdatedAt = now
	return v, s.write(v)
}

func (s *Store) AddCorrectionProof(repo, key, candidate, actor string, in CorrectionProof) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var c *CorrectionCandidate
	for i := range v.Corrections {
		if v.Corrections[i].ID == candidate {
			c = &v.Corrections[i]
		}
	}
	if c == nil {
		return Investigation{}, ErrNotFound
	}
	in.BaselineAttemptIDs = clean(in.BaselineAttemptIDs)
	in.Revision = strings.TrimSpace(in.Revision)
	known := map[string]Attempt{}
	for _, a := range v.Attempts {
		known[a.ID] = a
	}
	scenarioAttempt, ok := known[in.ScenarioAttemptID]
	if !ok || scenarioAttempt.ScenarioID != c.ScenarioID || scenarioAttempt.ScenarioVersion != c.ScenarioVersion || scenarioAttempt.Target.CommitID != c.Target.CommitID || in.Revision != c.Target.CommitID {
		return Investigation{}, ErrConflict
	}
	if len(in.BaselineAttemptIDs) < 2 {
		return Investigation{}, ErrConflict
	}
	hasGood, hasBad := false, false
	for _, x := range in.BaselineAttemptIDs {
		a, exists := known[x]
		if !exists || a.ScenarioID != c.ScenarioID {
			return Investigation{}, ErrConflict
		}
		hasGood = hasGood || a.Classification == "expected_behavior"
		hasBad = hasBad || a.Classification == "regressed_behavior"
	}
	if !hasGood || !hasBad {
		return Investigation{}, ErrConflict
	}
	required := map[string]bool{}
	for _, x := range c.AffectedChecks {
		required["check:"+x] = true
	}
	for _, x := range c.RequirementIDs {
		required["requirement:"+x] = true
	}
	for _, x := range c.AcceptanceCriteria {
		required["regression_criterion:"+x] = true
	}
	for _, x := range c.ChangeCriteria {
		required["change_criterion:"+x] = true
	}
	seen := map[string]bool{}
	for i := range in.Checks {
		x := &in.Checks[i]
		x.Name = strings.TrimSpace(x.Name)
		if !map[string]bool{"check": true, "requirement": true, "regression_criterion": true, "change_criterion": true}[x.Kind] || !map[string]bool{"passed": true, "failed": true}[x.Status] || !required[x.Kind+":"+x.Name] || seen[x.Kind+":"+x.Name] {
			return Investigation{}, ErrConflict
		}
		seen[x.Kind+":"+x.Name] = true
	}
	if len(seen) != len(required) {
		return Investigation{}, ErrConflict
	}
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	c.Proofs = append(c.Proofs, in)
	c.UpdatedAt = in.CreatedAt
	deriveCorrection(c, v)
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}

func (s *Store) AddCorrectionDelivery(repo, key, candidate, actor string, in DeliveryEvent) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var c *CorrectionCandidate
	for i := range v.Corrections {
		if v.Corrections[i].ID == candidate {
			c = &v.Corrections[i]
		}
	}
	if c == nil {
		return Investigation{}, ErrNotFound
	}
	in.Kind, in.ResourceID, in.Revision, in.Status, in.Summary = strings.TrimSpace(in.Kind), strings.TrimSpace(in.ResourceID), strings.TrimSpace(in.Revision), strings.TrimSpace(in.Status), strings.TrimSpace(in.Summary)
	if !map[string]bool{"review": true, "merge": true, "release": true, "deployment": true, "outcome": true}[in.Kind] || !map[string]bool{"passed": true, "failed": true, "reverted": true, "disagreed": true}[in.Status] || in.ResourceID == "" || in.Revision == "" || in.Summary == "" {
		return Investigation{}, ErrConflict
	}
	if in.Kind != "outcome" && in.Revision != c.Target.CommitID {
		return Investigation{}, ErrConflict
	}
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	c.Delivery = append(c.Delivery, in)
	c.UpdatedAt = in.CreatedAt
	deriveCorrection(c, v)
	if c.State == "reopened" {
		v.Status = "open"
		v.Entries = append(v.Entries, Entry{ID: id(), Sequence: int64(len(v.Entries) + 1), Kind: "status_change", Body: "Correction evidence reopened: " + c.ReopenedReason, ActorID: actor, CreatedAt: in.CreatedAt})
	}
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}

func deriveCorrection(c *CorrectionCandidate, v Investigation) {
	c.Blockers = nil
	c.ReopenedReason = ""
	if c.InvestigationVersion != v.Version {
		c.Blockers = append(c.Blockers, "stale_investigation_baseline")
	}
	var scenario *Scenario
	for i := range v.Scenarios {
		if v.Scenarios[i].ID == c.ScenarioID {
			scenario = &v.Scenarios[i]
		}
	}
	if scenario == nil || scenario.Version != c.ScenarioVersion {
		c.Blockers = append(c.Blockers, "stale_scenario_baseline")
	}
	passed := false
	proofs := c.Proofs
	if len(proofs) > 0 {
		proofs = proofs[len(proofs)-1:]
	}
	for _, p := range proofs {
		ok := true
		for _, x := range p.Checks {
			if x.Status != "passed" {
				ok = false
				c.Blockers = append(c.Blockers, "partial_correction")
			}
		}
		if a := attemptByID(v.Attempts, p.ScenarioAttemptID); a == nil || a.Classification != "expected_behavior" {
			ok = false
			c.Blockers = append(c.Blockers, "historical_regression_still_present")
		}
		passed = passed || ok
	}
	if len(c.Proofs) == 0 {
		c.Blockers = append(c.Blockers, "correction_proof_missing")
	}
	c.State = "awaiting_proof"
	if passed && len(c.Blockers) == 0 {
		c.State = "verified"
	}
	latestDelivery := map[string]DeliveryEvent{}
	for _, x := range c.Delivery {
		latestDelivery[x.Kind] = x
	}
	for _, x := range latestDelivery {
		if x.Status == "failed" || x.Status == "reverted" || x.Status == "disagreed" {
			c.State = "reopened"
			c.ReopenedReason = x.Kind + "_" + x.Status
			c.Blockers = append(c.Blockers, c.ReopenedReason)
		}
	}
	if c.State == "verified" {
		complete := true
		for _, kind := range []string{"review", "merge", "release", "deployment", "outcome"} {
			if latestDelivery[kind].Status != "passed" {
				complete = false
			}
		}
		if complete {
			c.State = "observed"
		}
	}
}
func attemptByID(xs []Attempt, id string) *Attempt {
	for i := range xs {
		if xs[i].ID == id {
			return &xs[i]
		}
	}
	return nil
}

func (s *Store) CreateResponse(repo, key, actor string, in ResponsePlanInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var q *Search
	for i := range v.Searches {
		if v.Searches[i].ID == in.SearchID {
			q = &v.Searches[i]
		}
	}
	if q == nil {
		return Investigation{}, ErrNotFound
	}
	in.ReproductionIDs, in.Constraints, in.AcceptanceCriteria, in.OriginalAuthorIDs = clean(in.ReproductionIDs), clean(in.Constraints), clean(in.AcceptanceCriteria), clean(in.OriginalAuthorIDs)
	in.OriginalIntent, in.Rationale, in.SelectedOptionID = strings.TrimSpace(in.OriginalIntent), strings.TrimSpace(in.Rationale), strings.TrimSpace(in.SelectedOptionID)
	if len(in.Options) != 4 || len(in.ReproductionIDs) == 0 || len(in.Constraints) == 0 || len(in.AcceptanceCriteria) == 0 || in.OriginalIntent == "" {
		return Investigation{}, ErrConflict
	}
	rangeOK, evidence := false, map[string]bool{}
	for _, r := range q.Ranges {
		if r.GoodKey == in.CulpritGoodKey && r.BadKey == in.CulpritBadKey && r.Status == "supported" {
			rangeOK = true
		}
	}
	for _, a := range v.Attempts {
		evidence[a.ID] = true
	}
	for _, x := range v.Evidence {
		evidence[x.ID] = true
	}
	for _, x := range in.ReproductionIDs {
		if !evidence[x] {
			return Investigation{}, ErrConflict
		}
	}
	kinds, ids := map[string]bool{}, map[string]bool{}
	for i := range in.Options {
		o := &in.Options[i]
		o.ID, o.Kind, o.Title, o.Summary = strings.TrimSpace(o.ID), strings.TrimSpace(o.Kind), strings.TrimSpace(o.Title), strings.TrimSpace(o.Summary)
		o.Tradeoffs, o.AffectedReleases, o.AffectedWork, o.BackportTargets, o.EvidenceIDs = clean(o.Tradeoffs), clean(o.AffectedReleases), clean(o.AffectedWork), clean(o.BackportTargets), clean(o.EvidenceIDs)
		if o.ID == "" || ids[o.ID] || o.Title == "" || o.Summary == "" || len(o.Tradeoffs) == 0 || len(o.EvidenceIDs) == 0 || !map[string]bool{"revert": true, "containment": true, "dependency_adjustment": true, "forward_repair": true}[o.Kind] {
			return Investigation{}, ErrConflict
		}
		for _, x := range o.EvidenceIDs {
			if !evidence[x] {
				return Investigation{}, ErrConflict
			}
		}
		ids[o.ID] = true
		kinds[o.Kind] = true
	}
	if !rangeOK || len(kinds) != 4 || (in.SelectedOptionID != "" && (!ids[in.SelectedOptionID] || in.Rationale == "")) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Responses = append(v.Responses, ResponsePlan{ID: id(), ResponsePlanInput: in, CreatedByID: actor, CreatedAt: now, UpdatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}

func (s *Store) AddResponseWork(repo, key, responseID, actor string, in ResponseWork) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var p *ResponsePlan
	for i := range v.Responses {
		if v.Responses[i].ID == responseID {
			p = &v.Responses[i]
		}
	}
	if p == nil {
		return Investigation{}, ErrNotFound
	}
	in.Kind, in.ResourceID, in.OwnerID, in.OwnerKind, in.OptionID, in.PullRequestID = strings.TrimSpace(in.Kind), strings.TrimSpace(in.ResourceID), strings.TrimSpace(in.OwnerID), strings.TrimSpace(in.OwnerKind), strings.TrimSpace(in.OptionID), strings.TrimSpace(in.PullRequestID)
	validOption := false
	var option ResponseOption
	for _, o := range p.Options {
		if o.ID == in.OptionID {
			validOption = true
			option = o
		}
	}
	if !validOption || in.ResourceID == "" || in.OwnerID == "" || !map[string]bool{"task": true, "session": true, "workspace": true}[in.Kind] || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] || (in.Published && in.PullRequestID == "") {
		return Investigation{}, ErrConflict
	}
	in.ID = id()
	in.Intent = p.OriginalIntent
	in.AuthorIDs = append([]string{}, p.OriginalAuthorIDs...)
	in.Tradeoffs = append([]string{}, option.Tradeoffs...)
	in.BackportTargets = append([]string{}, option.BackportTargets...)
	in.CulpritRange = []string{p.CulpritGoodKey, p.CulpritBadKey}
	in.ReproductionIDs = append([]string{}, p.ReproductionIDs...)
	in.Constraints = append([]string{}, p.Constraints...)
	in.AcceptanceCriteria = append([]string{}, p.AcceptanceCriteria...)
	in.CreatedByID = actor
	in.CreatedAt = s.now().UTC()
	p.Work = append(p.Work, in)
	p.UpdatedAt = in.CreatedAt
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}

type Input struct {
	Title    string     `json:"title"`
	Source   Source     `json:"source"`
	Scope    Scope      `json:"scope"`
	Evidence []Evidence `json:"evidence"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in Input) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Investigation{ID: id(), RepositoryID: repo, Title: strings.TrimSpace(in.Title), Source: in.Source, CreatorID: actor, Version: 1, Scope: cleanScope(in.Scope), Status: "open", CreatedAt: now, UpdatedAt: now}
	v.Evidence = stampEvidence(in.Evidence, actor, now)
	v.Blockers = blockers(v.Scope)
	v.ScopeChanges = []ScopeChange{{Version: 1, ActorID: actor, Reason: "investigation opened", Scope: v.Scope, CreatedAt: now}}
	return v, s.write(v)
}
func (s *Store) Get(repo, key string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Investigation{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if er == nil && v.RepositoryID == repo {
			v.Entries = nil
			v.Evidence = nil
			v.ScopeChanges = nil
			v.Scenarios = nil
			v.Attempts = nil
			v.Searches = nil
			v.Responses = nil
			v.Corrections = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) ChangeScope(repo, key, actor, reason string, expected int64, scope Scope) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if v.Version != expected || strings.TrimSpace(reason) == "" {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Version++
	v.Scope = cleanScope(scope)
	v.Blockers = blockers(v.Scope)
	for i := range v.Corrections {
		deriveCorrection(&v.Corrections[i], v)
	}
	v.ScopeChanges = append(v.ScopeChanges, ScopeChange{Version: v.Version, ActorID: actor, Reason: strings.TrimSpace(reason), Scope: v.Scope, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddEvidence(repo, key, actor string, e Evidence) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	now := s.now().UTC()
	e = stampEvidence([]Evidence{e}, actor, now)[0]
	v.Evidence = append(v.Evidence, e)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddEntry(repo, key, actor string, e Entry) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	e.Body = strings.TrimSpace(e.Body)
	if e.Body == "" || len(e.Body) > 10000 || !validEntry(e.Kind) {
		return Investigation{}, ErrConflict
	}
	e.ID = id()
	e.Sequence = int64(len(v.Entries) + 1)
	e.ActorID = actor
	e.CreatedAt = s.now().UTC()
	v.Entries = append(v.Entries, e)
	v.UpdatedAt = e.CreatedAt
	return v, s.write(v)
}
func (s *Store) SetStatus(repo, key, actor, status, reason string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(key)
	if x != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	openCorrection := false
	for _, c := range v.Corrections {
		if c.State != "observed" {
			openCorrection = true
		}
	}
	if !validStatus(status) || strings.TrimSpace(reason) == "" || (status == "ready" && len(v.Blockers) > 0) || (status == "closed" && openCorrection) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Status = status
	v.Entries = append(v.Entries, Entry{ID: id(), Sequence: int64(len(v.Entries) + 1), Kind: "status_change", Body: strings.TrimSpace(reason), ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) CreateScenario(repo, key, actor string, derived bool, d ScenarioDefinition) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	d.Title, d.ExpectedBehavior, d.RegressedBehavior = strings.TrimSpace(d.Title), strings.TrimSpace(d.ExpectedBehavior), strings.TrimSpace(d.RegressedBehavior)
	d.Commands, d.EnvironmentRequirements = clean(d.Commands), clean(d.EnvironmentRequirements)
	if derived {
		if d.ExpectedBehavior == "" {
			d.ExpectedBehavior = v.Scope.ExpectedBehavior
		}
		if d.RegressedBehavior == "" {
			d.RegressedBehavior = v.Scope.RegressedBehavior
		}
	}
	if d.Title == "" || d.ExpectedBehavior == "" || d.RegressedBehavior == "" || len(d.Commands) == 0 || len(d.EnvironmentRequirements) == 0 || d.TimeoutSeconds < 1 || d.TimeoutSeconds > 3600 || d.CostLimit < 0 || !validFixtures(d.Fixtures) || !validInputs(d.Inputs) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Scenarios = append(v.Scenarios, Scenario{ID: id(), Version: 1, InvestigationVersion: v.Version, Derived: derived, Definition: d, CreatedByID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddAttempt(repo, key, scenario, actor string, in AttemptInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var sc *Scenario
	for i := range v.Scenarios {
		if v.Scenarios[i].ID == scenario {
			sc = &v.Scenarios[i]
			break
		}
	}
	if sc == nil {
		return Investigation{}, ErrNotFound
	}
	in.Commands, in.Outputs, in.Logs, in.Environment.SetupCommands = clean(in.Commands), clean(in.Outputs), clean(in.Logs), clean(in.Environment.SetupCommands)
	in.Rationale, in.Currency = strings.TrimSpace(in.Rationale), strings.TrimSpace(in.Currency)
	if !validAttempt(in, *sc) {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	v.Attempts = append(v.Attempts, Attempt{ID: id(), ScenarioID: sc.ID, ScenarioVersion: sc.Version, AttemptInput: in, ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) CreateSearch(repo, key, actor string, in SearchInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	foundScenario := false
	for _, x := range v.Scenarios {
		if x.ID == in.ScenarioID {
			foundScenario = true
		}
	}
	if !foundScenario || in.ConfidenceTarget <= 0 || in.ConfidenceTarget > 1 || len(in.Revisions) < 2 || len(in.Revisions) > 2000 {
		return Investigation{}, ErrConflict
	}
	keys := map[string]bool{}
	for i := range in.Revisions {
		x := &in.Revisions[i]
		x.Key, x.Kind, x.Revision = strings.TrimSpace(x.Key), strings.TrimSpace(x.Kind), strings.TrimSpace(x.Revision)
		x.Parents, x.DiffPaths, x.OwnerIDs, x.PullIDs, x.DecisionIDs = clean(x.Parents), clean(x.DiffPaths), clean(x.OwnerIDs), clean(x.PullIDs), clean(x.DecisionIDs)
		if x.Key == "" || x.Revision == "" || keys[x.Key] || !map[string]bool{"commit": true, "repository_revision": true, "package_revision": true}[x.Kind] {
			return Investigation{}, ErrConflict
		}
		keys[x.Key] = true
		if x.Kind == "repository_revision" && x.RepositoryID == "" || x.Kind == "package_revision" && x.Package == "" {
			return Investigation{}, ErrConflict
		}
	}
	if !keys[in.GoodKey] || !keys[in.BadKey] || hasSearchCycle(in.Revisions) {
		return Investigation{}, ErrConflict
	}
	for _, x := range in.Revisions {
		for _, p := range x.Parents {
			if !keys[p] {
				return Investigation{}, ErrConflict
			}
		}
	}
	b, _ := json.Marshal(in.Revisions)
	sum := sha256.Sum256(b)
	now := s.now().UTC()
	search := Search{ID: id(), ScenarioID: in.ScenarioID, GoodKey: in.GoodKey, BadKey: in.BadKey, ConfidenceTarget: in.ConfidenceTarget, Revisions: in.Revisions, GraphDigest: "sha256:" + hex.EncodeToString(sum[:]), CreatedByID: actor, CreatedAt: now, UpdatedAt: now}
	deriveSearch(&search)
	v.Searches = append(v.Searches, search)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) ClassifyCandidate(repo, key, searchID, actor string, in CandidateClassification) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var q *Search
	for i := range v.Searches {
		if v.Searches[i].ID == searchID {
			q = &v.Searches[i]
		}
	}
	if q == nil {
		return Investigation{}, ErrNotFound
	}
	in.Rationale = strings.TrimSpace(in.Rationale)
	in.AttemptIDs = clean(in.AttemptIDs)
	validKey := false
	for _, x := range q.Revisions {
		if x.Key == in.RevisionKey {
			validKey = true
		}
	}
	validAttempts := map[string]bool{}
	for _, x := range v.Attempts {
		if x.ScenarioID == q.ScenarioID {
			validAttempts[x.ID] = true
		}
	}
	if !validKey || in.Rationale == "" || !map[string]bool{"working": true, "regressed": true, "invalid": true, "flaky": true, "inconclusive": true}[in.Classification] {
		return Investigation{}, ErrConflict
	}
	if (in.Classification == "working" || in.Classification == "regressed") && len(in.AttemptIDs) == 0 {
		return Investigation{}, ErrConflict
	}
	for _, x := range in.AttemptIDs {
		if !validAttempts[x] {
			return Investigation{}, ErrConflict
		}
	}
	now := s.now().UTC()
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = now
	q.Classifications = append(q.Classifications, in)
	q.UpdatedAt = now
	deriveSearch(q)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) AddHypothesis(repo, key, searchID, actor string, in CausalHypothesis) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(key)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	var q *Search
	for i := range v.Searches {
		if v.Searches[i].ID == searchID {
			q = &v.Searches[i]
		}
	}
	if q == nil {
		return Investigation{}, ErrNotFound
	}
	in.Body = strings.TrimSpace(in.Body)
	in.RevisionKeys, in.EvidenceIDs, in.DiffPaths = clean(in.RevisionKeys), clean(in.EvidenceIDs), clean(in.DiffPaths)
	keys := map[string]bool{}
	knownEvidence := map[string]bool{}
	for _, x := range q.Revisions {
		keys[x.Key] = true
	}
	for _, x := range v.Attempts {
		knownEvidence[x.ID] = true
	}
	for _, x := range v.Evidence {
		knownEvidence[x.ID] = true
	}
	for _, x := range in.RevisionKeys {
		if !keys[x] {
			return Investigation{}, ErrConflict
		}
	}
	for _, x := range in.EvidenceIDs {
		if !knownEvidence[x] {
			return Investigation{}, ErrConflict
		}
	}
	if in.Body == "" || len(in.RevisionKeys) == 0 || len(in.EvidenceIDs) == 0 || in.Confidence < 0 || in.Confidence > 1 || !map[string]bool{"human": true, "agent": true}[in.ActorKind] || !map[string]bool{"proposed": true, "supported": true, "disputed": true, "rejected": true}[in.State] {
		return Investigation{}, ErrConflict
	}
	now := s.now().UTC()
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = now
	q.Hypotheses = append(q.Hypotheses, in)
	q.UpdatedAt = now
	v.UpdatedAt = now
	return v, s.write(v)
}
func hasSearchCycle(rs []SearchRevision) bool {
	m := map[string][]string{}
	for _, x := range rs {
		m[x.Key] = x.Parents
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(k string) bool {
		if visiting[k] {
			return true
		}
		if done[k] {
			return false
		}
		visiting[k] = true
		for _, p := range m[k] {
			if visit(p) {
				return true
			}
		}
		visiting[k] = false
		done[k] = true
		return false
	}
	for k := range m {
		if visit(k) {
			return true
		}
	}
	return false
}
func deriveSearch(q *Search) {
	latest := map[string]string{}
	stable := map[string]bool{}
	invalid := map[string]bool{}
	for _, x := range q.Classifications {
		latest[x.RevisionKey] = x.Classification
		if x.Classification == "working" || x.Classification == "regressed" {
			stable[x.RevisionKey] = true
		}
		if x.Classification == "invalid" || x.Classification == "flaky" {
			invalid[x.RevisionKey] = true
		}
	}
	q.RemainingKeys = nil
	q.ScheduledKeys = nil
	q.Ranges = nil
	q.Blockers = nil
	q.Verdict = ""
	for _, x := range q.Revisions {
		if !stable[x.Key] && !invalid[x.Key] {
			q.RemainingKeys = append(q.RemainingKeys, x.Key)
		}
	}
	for _, x := range q.Revisions {
		if len(q.ScheduledKeys) >= 4 {
			break
		}
		if latest[x.Key] == "" && x.Key != q.GoodKey && x.Key != q.BadKey {
			q.ScheduledKeys = append(q.ScheduledKeys, x.Key)
		}
	}
	for _, bad := range q.Revisions {
		if latest[bad.Key] != "regressed" {
			continue
		}
		for _, p := range bad.Parents {
			if latest[p] == "working" {
				confidence := 1.0
				if len(bad.Parents) > 1 {
					confidence = .75
					q.Blockers = append(q.Blockers, "merge_ancestry_requires_parent_disambiguation")
				}
				q.Ranges = append(q.Ranges, CulpritRange{GoodKey: p, BadKey: bad.Key, Confidence: confidence, Status: "supported"})
			}
		}
	}
	if invalid[q.GoodKey] || invalid[q.BadKey] {
		q.Blockers = append(q.Blockers, "boundary_trial_invalid")
	}
	if len(q.Ranges) > 1 {
		q.Blockers = append(q.Blockers, "competing_culprit_ranges")
	}
	if len(q.Ranges) == 1 && q.Ranges[0].Confidence >= q.ConfidenceTarget && len(q.Blockers) == 0 {
		q.Verdict = fmt.Sprintf("%s..%s", q.Ranges[0].GoodKey, q.Ranges[0].BadKey)
	} else if len(q.Ranges) > 0 {
		q.Verdict = "multiple_or_ambiguous"
	} else {
		q.Verdict = "unresolved"
	}
}
func validFixtures(v []Fixture) bool {
	if len(v) == 0 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x.Name) == "" || strings.TrimSpace(x.Reference) == "" || !map[string]bool{"synthetic": true, "explicitly_permitted": true, "unsafe": true}[x.Classification] {
			return false
		}
	}
	return true
}
func validInputs(v []ScenarioInput) bool {
	for _, x := range v {
		if strings.TrimSpace(x.Name) == "" || !map[string]bool{"string": true, "number": true, "boolean": true, "artifact_reference": true}[x.Kind] {
			return false
		}
	}
	return true
}
func validAttempt(v AttemptInput, s Scenario) bool {
	classes := map[string]bool{"expected_behavior": true, "regressed_behavior": true, "incompatible_setup": true, "missing_dependencies": true, "flaky": true, "unsafe_fixture": true, "untestable_revision": true}
	targets := map[string]bool{"revision": true, "release": true, "dependency_combination": true}
	if !classes[v.Classification] || !targets[v.Target.Kind] || v.Rationale == "" || v.Cost < 0 || v.Currency == "" || v.Environment.Image == "" || v.Environment.DefinitionDigest == "" || v.Environment.OS == "" || v.Environment.Architecture == "" || v.Environment.Isolation != "isolated" || v.Environment.Network == "unrestricted" || len(v.Commands) == 0 || !validInputs(v.Inputs) || v.Provenance.RunnerID == "" || v.Provenance.RunnerVersion == "" || !map[string]bool{"human": true, "agent": true, "system": true}[v.Provenance.ActorKind] || v.Provenance.StartedAt == "" || v.Provenance.CompletedAt == "" || v.Provenance.RepetitionCount < 1 {
		return false
	}
	if v.Cost > s.Definition.CostLimit || v.Target.CommitID == "" || (v.Target.Kind == "dependency_combination" && len(v.Target.Dependencies) == 0) || (v.Target.Kind == "release" && (v.Target.ReleaseID == "" || v.Target.AttestationDigest == "")) {
		return false
	}
	if v.Classification == "flaky" && v.Provenance.RepetitionCount < 2 {
		return false
	}
	for _, f := range s.Definition.Fixtures {
		if f.Classification == "unsafe" && v.Classification != "unsafe_fixture" {
			return false
		}
	}
	retained := append(append(append([]string{}, v.Commands...), v.Environment.SetupCommands...), v.Outputs...)
	retained = append(retained, v.Logs...)
	for _, x := range v.Inputs {
		retained = append(retained, x.Name, x.Value)
	}
	for _, x := range retained {
		if len(x) > 10000 || credentialShaped(x) {
			return false
		}
	}
	for _, a := range v.Artifacts {
		if a.Name == "" || a.Digest == "" || a.MediaType == "" || a.Size < 0 {
			return false
		}
	}
	return true
}

func credentialShaped(v string) bool {
	x := strings.ToLower(v)
	for _, marker := range []string{"-----begin private key-----", "authorization: bearer ", "password=", "secret=", "api_key=", "access_token="} {
		if strings.Contains(x, marker) {
			return true
		}
	}
	return false
}
func cleanScope(v Scope) Scope {
	v.ExpectedBehavior = strings.TrimSpace(v.ExpectedBehavior)
	v.RegressedBehavior = strings.TrimSpace(v.RegressedBehavior)
	v.Comparability = strings.TrimSpace(v.Comparability)
	v.Severity = strings.TrimSpace(v.Severity)
	v.Environments = clean(v.Environments)
	v.OwnerIDs = clean(v.OwnerIDs)
	v.AcceptanceCriteria = clean(v.AcceptanceCriteria)
	return v
}
func clean(xs []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
func blockers(v Scope) []string {
	out := []string{}
	if v.ExpectedBehavior == "" {
		out = append(out, "expected_behavior_missing")
	}
	if v.RegressedBehavior == "" {
		out = append(out, "regressed_behavior_missing")
	}
	if v.KnownGood.CommitID == "" {
		out = append(out, "known_good_missing")
	}
	if v.KnownBad.CommitID == "" {
		out = append(out, "known_bad_missing")
	}
	if len(v.Environments) == 0 {
		out = append(out, "affected_environment_missing")
	}
	if v.Comparability == "" {
		out = append(out, "comparability_missing")
	}
	if v.Severity == "" {
		out = append(out, "severity_missing")
	}
	if len(v.OwnerIDs) == 0 {
		out = append(out, "owner_missing")
	}
	if len(v.AcceptanceCriteria) == 0 {
		out = append(out, "acceptance_criteria_missing")
	}
	return out
}
func stampEvidence(es []Evidence, actor string, now time.Time) []Evidence {
	out := make([]Evidence, len(es))
	for i, e := range es {
		e.ID = id()
		e.Kind = strings.TrimSpace(e.Kind)
		e.ResourceID = strings.TrimSpace(e.ResourceID)
		e.Summary = strings.TrimSpace(e.Summary)
		e.ActorID = actor
		e.CreatedAt = now
		if e.Audience == "" {
			e.Audience = "repository"
		}
		out[i] = e
	}
	return out
}
func validEntry(v string) bool {
	return v == "discussion" || v == "hypothesis" || v == "scope_note" || v == "status_change"
}
func validStatus(v string) bool { return v == "open" || v == "ready" || v == "paused" || v == "closed" }
func (s *Store) read(key string) (Investigation, error) {
	b, e := os.ReadFile(filepath.Join(s.root, key+".json"))
	if os.IsNotExist(e) {
		return Investigation{}, ErrNotFound
	}
	var v Investigation
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) write(v Investigation) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
