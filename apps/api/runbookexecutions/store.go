// Package runbookexecutions retains safe launches of exact operational procedures.
package runbookexecutions

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

var ErrNotFound = errors.New("runbook execution not found")
var ErrInvalid = errors.New("invalid runbook execution")
var ErrConflict = errors.New("duplicate runbook execution")
var ErrBlocked = errors.New("runbook execution blocked")
var ErrForbidden = errors.New("runbook execution action forbidden")

type Origin struct {
	Kind              string `json:"kind"`
	ResourceID        string `json:"resource_id"`
	Revision          string `json:"revision"`
	TimelineReference string `json:"timeline_reference"`
	Audience          string `json:"audience"`
}
type SignalWindow struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
type Context struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	Permitted  bool   `json:"permitted"`
	Audience   string `json:"audience"`
	Accessible bool   `json:"accessible"`
}
type Check struct {
	ID                string `json:"id"`
	Satisfied         bool   `json:"satisfied"`
	EvidenceReference string `json:"evidence_reference,omitempty"`
	Detail            string `json:"detail,omitempty"`
}
type Access struct {
	Capability         string `json:"capability"`
	ResourceID         string `json:"resource_id"`
	Granted            bool   `json:"granted"`
	AuthorityReference string `json:"authority_reference,omitempty"`
}
type LaunchInput struct {
	IdempotencyKey          string             `json:"idempotency_key"`
	RunbookID               string             `json:"runbook_id"`
	RunbookVersion          int64              `json:"runbook_version"`
	Origin                  Origin             `json:"origin"`
	AffectedResources       []string           `json:"affected_resources"`
	SignalWindow            SignalWindow       `json:"signal_window"`
	Context                 []Context          `json:"context"`
	Preconditions           []Check            `json:"preconditions"`
	Access                  []Access           `json:"access"`
	MatchExplanation        []string           `json:"match_explanation"`
	RehearsalID             string             `json:"rehearsal_id"`
	RehearsalRevision       int64              `json:"rehearsal_revision"`
	RehearsalReady          bool               `json:"rehearsal_ready"`
	RunbookFindings         []string           `json:"runbook_findings,omitempty"`
	ActivePath              []ProcedureStep    `json:"active_path"`
	OutcomeCriteria         []OutcomeCriterion `json:"outcome_criteria"`
	RunbookUseState         string             `json:"runbook_use_state,omitempty"`
	ApprovedFallbackID      string             `json:"approved_fallback_id,omitempty"`
	ApprovedFallbackVersion int64              `json:"approved_fallback_version,omitempty"`
}
type ProcedureStep struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Title             string   `json:"title"`
	DependsOn         []string `json:"depends_on"`
	ExpectedEvidence  []string `json:"expected_evidence"`
	RequiredAuthority []string `json:"required_authority"`
	OwnerIDs          []string `json:"owner_ids"`
	RollbackCriteria  []string `json:"rollback_criteria"`
	HumanDecision     bool     `json:"human_decision"`
	Optional          bool     `json:"optional"`
	PolicyPermitsSkip bool     `json:"policy_permits_skip"`
}
type OutcomeCriterion struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type Participant struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}
type StepState struct {
	ID               string   `json:"id"`
	State            string   `json:"state"`
	ControllerID     string   `json:"controller_id,omitempty"`
	ApprovedBy       string   `json:"approved_by,omitempty"`
	DelegatedAgentID string   `json:"delegated_agent_id,omitempty"`
	DelegatedMode    string   `json:"delegated_mode,omitempty"`
	Evidence         []string `json:"evidence"`
	Health           string   `json:"health"`
	Cost             float64  `json:"cost"`
	Blocker          string   `json:"blocker,omitempty"`
	RollbackState    string   `json:"rollback_state"`
}
type ScopedCredential struct {
	Reference      string     `json:"reference"`
	StepID         string     `json:"step_id"`
	SubjectID      string     `json:"subject_id"`
	Capabilities   []string   `json:"capabilities"`
	IssuedAt       time.Time  `json:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	SecretRetained bool       `json:"secret_retained"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	StepID    string    `json:"step_id,omitempty"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type ActionReceipt struct {
	ID                  string    `json:"id"`
	IdempotencyKey      string    `json:"idempotency_key"`
	Action              string    `json:"action"`
	StepID              string    `json:"step_id,omitempty"`
	ActorID             string    `json:"actor_id"`
	RunbookVersion      int64     `json:"runbook_version"`
	ExecutionRevision   int64     `json:"execution_revision"`
	CredentialReference string    `json:"credential_reference,omitempty"`
	Evidence            []string  `json:"evidence,omitempty"`
	Cost                float64   `json:"cost"`
	CreatedAt           time.Time `json:"created_at"`
}
type ControlInput struct {
	ExpectedRevision    int64     `json:"expected_revision"`
	IdempotencyKey      string    `json:"idempotency_key"`
	Action              string    `json:"action"`
	StepID              string    `json:"step_id,omitempty"`
	Body                string    `json:"body,omitempty"`
	TargetID            string    `json:"target_id,omitempty"`
	ActorKind           string    `json:"actor_kind,omitempty"`
	Mode                string    `json:"mode,omitempty"`
	Evidence            []string  `json:"evidence,omitempty"`
	Health              string    `json:"health,omitempty"`
	Cost                float64   `json:"cost,omitempty"`
	CredentialExpiresAt time.Time `json:"credential_expires_at,omitempty"`
}
type CriterionResult struct {
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
	Detail   string   `json:"detail,omitempty"`
}
type Feedback struct {
	ParticipantID string `json:"participant_id"`
	Body          string `json:"body"`
}
type EvaluationInput struct {
	ExpectedRevision int64             `json:"expected_revision"`
	IdempotencyKey   string            `json:"idempotency_key"`
	Disposition      string            `json:"disposition"`
	Criteria         []CriterionResult `json:"criteria"`
	Deviations       []string          `json:"deviations"`
	ManualWork       []string          `json:"manual_work"`
	FailedSteps      []string          `json:"failed_steps"`
	AccessGaps       []string          `json:"access_gaps"`
	AgentCorrections []string          `json:"agent_corrections"`
	Feedback         []Feedback        `json:"participant_feedback"`
}
type Finding struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	Detail           string   `json:"detail"`
	Evidence         []string `json:"evidence"`
	Supported        bool     `json:"supported"`
	ImprovementKind  string   `json:"improvement_kind,omitempty"`
	ImprovementRef   string   `json:"improvement_reference,omitempty"`
	ImprovementOwner string   `json:"improvement_owner,omitempty"`
}
type Evaluation struct {
	ID                  string            `json:"id"`
	IdempotencyKey      string            `json:"idempotency_key"`
	Disposition         string            `json:"disposition"`
	Criteria            []CriterionResult `json:"criteria"`
	Deviations          []string          `json:"deviations"`
	ManualWork          []string          `json:"manual_work"`
	FailedSteps         []string          `json:"failed_steps"`
	StepTiming          map[string]string `json:"step_timing"`
	AccessGaps          []string          `json:"access_gaps"`
	AgentCorrections    []string          `json:"agent_corrections"`
	Cost                float64           `json:"cost"`
	ParticipantFeedback []Feedback        `json:"participant_feedback"`
	Findings            []Finding         `json:"findings"`
	OutcomeProven       bool              `json:"outcome_proven"`
	EvaluatedBy         string            `json:"evaluated_by"`
	EvaluatedAt         time.Time         `json:"evaluated_at"`
}
type LearningInput struct {
	ExpectedRevision       int64  `json:"expected_revision"`
	IdempotencyKey         string `json:"idempotency_key"`
	Action                 string `json:"action"`
	FindingID              string `json:"finding_id,omitempty"`
	ImprovementKind        string `json:"improvement_kind,omitempty"`
	ImprovementReference   string `json:"improvement_reference,omitempty"`
	ImprovementOwner       string `json:"improvement_owner,omitempty"`
	ReviewedRunbookVersion int64  `json:"reviewed_runbook_version,omitempty"`
	FreshRehearsalID       string `json:"fresh_rehearsal_id,omitempty"`
	FreshRehearsalRevision int64  `json:"fresh_rehearsal_revision,omitempty"`
	FallbackRunbookID      string `json:"fallback_runbook_id,omitempty"`
	FallbackRunbookVersion int64  `json:"fallback_runbook_version,omitempty"`
	Reason                 string `json:"reason,omitempty"`
}
type LearningReceipt struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Action         string    `json:"action"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type Blocker struct {
	Kind    string   `json:"kind"`
	Subject string   `json:"subject"`
	Detail  string   `json:"detail"`
	Choices []string `json:"choices,omitempty"`
}
type Execution struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	LaunchInput
	ControllerID           string             `json:"controller_id"`
	CreatedAt              time.Time          `json:"created_at"`
	State                  string             `json:"state"`
	Blockers               []Blocker          `json:"blockers"`
	NonAuthority           []string           `json:"non_authority"`
	Participants           []Participant      `json:"participants"`
	Steps                  []StepState        `json:"steps"`
	Credentials            []ScopedCredential `json:"scoped_credentials"`
	Events                 []Event            `json:"events"`
	ActionReceipts         []ActionReceipt    `json:"action_receipts"`
	Health                 string             `json:"health"`
	Cost                   float64            `json:"cost"`
	RollbackState          string             `json:"rollback_state"`
	PredictedNextAction    string             `json:"predicted_next_action"`
	Evaluation             *Evaluation        `json:"evaluation,omitempty"`
	LearningReceipts       []LearningReceipt  `json:"learning_receipts"`
	ReviewedRunbookVersion int64              `json:"reviewed_runbook_version,omitempty"`
	FreshRehearsalID       string             `json:"fresh_rehearsal_id,omitempty"`
	FreshRehearsalRevision int64              `json:"fresh_rehearsal_revision,omitempty"`
	UseState               string             `json:"use_state"`
	FallbackRunbookID      string             `json:"fallback_runbook_id,omitempty"`
	FallbackRunbookVersion int64              `json:"fallback_runbook_version,omitempty"`
	UpdatedAt              time.Time          `json:"updated_at"`
}
type Candidate struct {
	RunbookID        string    `json:"runbook_id"`
	RunbookVersion   int64     `json:"runbook_version"`
	Name             string    `json:"name"`
	Eligible         bool      `json:"eligible"`
	Score            int       `json:"score"`
	MatchExplanation []string  `json:"match_explanation"`
	Blockers         []Blocker `json:"blockers"`
	Choices          []string  `json:"choices,omitempty"`
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
func uid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(s string, xs ...string) bool {
	for _, x := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func unique(xs []string) bool {
	if len(xs) == 0 {
		return false
	}
	m := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func validate(in LaunchInput) bool {
	if in.IdempotencyKey == "" || in.RunbookID == "" || in.RunbookVersion < 1 || !allowed(in.Origin.Kind, "alert", "incident", "deployment", "failed_workflow", "service_objective", "support_thread", "manual_observation") || in.Origin.ResourceID == "" || in.Origin.Revision == "" || in.Origin.TimelineReference == "" || in.Origin.Audience == "" || !unique(in.AffectedResources) || in.SignalWindow.StartedAt.IsZero() || in.SignalWindow.EndedAt.Before(in.SignalWindow.StartedAt) || len(in.Context) == 0 || len(in.Preconditions) == 0 || len(in.Access) == 0 || len(in.MatchExplanation) == 0 {
		return false
	}
	for _, c := range in.Context {
		if c.Kind == "" || c.ResourceID == "" || c.Revision == "" || c.Audience == "" {
			return false
		}
	}
	for _, c := range in.Preconditions {
		if c.ID == "" {
			return false
		}
	}
	for _, a := range in.Access {
		if a.Capability == "" || a.ResourceID == "" {
			return false
		}
	}
	return true
}
func refresh(x *Execution) {
	if x.State == "aborted" {
		x.PredictedNextAction = "preserve evidence and follow the declared rollback or escalation path"
		return
	}
	if x.State == "paused" {
		x.PredictedNextAction = "the current controller can resume or hand off after reviewing current health and blockers"
		return
	}
	if len(x.Steps) == 0 {
		x.PredictedNextAction = "inspect the frozen launch context before beginning the procedure"
		return
	}
	for _, st := range x.Steps {
		if st.State != "completed" && st.State != "skipped" {
			if st.Blocker != "" {
				x.PredictedNextAction = "resolve blocker on " + st.ID + ": " + st.Blocker
			} else if st.ApprovedBy == "" && stepByID(*x, st.ID).HumanDecision {
				x.PredictedNextAction = "record a separate human approval for " + st.ID
			} else {
				x.PredictedNextAction = "perform " + st.ID + " under its declared authority"
			}
			return
		}
	}
	x.State = "completed"
	x.PredictedNextAction = "verify the outcome against health, recovery, and rollback criteria"
}
func stepByID(x Execution, id string) ProcedureStep {
	for _, s := range x.ActivePath {
		if s.ID == id {
			return s
		}
	}
	return ProcedureStep{}
}
func blockers(in LaunchInput) []Blocker {
	out := []Blocker{}
	for _, finding := range in.RunbookFindings {
		out = append(out, Blocker{"runbook_finding", in.RunbookID, finding, []string{"inspect current finding", "select another procedure"}})
	}
	if !in.RehearsalReady {
		out = append(out, Blocker{"stale_or_missing_rehearsal", in.RehearsalID, "selected revision lacks current rehearsal proof", []string{"select another eligible runbook", "rehearse this revision"}})
	}
	if in.RunbookUseState == "suspended" {
		out = append(out, Blocker{"runbook_suspended", in.RunbookID, "current use is suspended after unsafe drift or repeated failure", []string{"use approved fallback " + in.ApprovedFallbackID, "review a corrected revision and fresh rehearsal"}})
	}
	for _, c := range in.Context {
		if !c.Permitted {
			out = append(out, Blocker{"evidence_not_permitted", c.ResourceID, "origin audience does not permit this evidence", nil})
		}
		if !c.Accessible {
			out = append(out, Blocker{"dependency_unavailable", c.ResourceID, "bound context is currently unavailable", []string{"continue without this evidence", "wait for dependency"}})
		}
	}
	for _, c := range in.Preconditions {
		if !c.Satisfied {
			out = append(out, Blocker{"precondition_failed", c.ID, c.Detail, []string{"resolve precondition", "choose another procedure"}})
		}
	}
	for _, a := range in.Access {
		if !a.Granted || a.AuthorityReference == "" {
			out = append(out, Blocker{"access_unavailable", a.Capability + ":" + a.ResourceID, "current authority was not verified", []string{"request ordinary access", "choose a diagnostic-only procedure"}})
		}
	}
	return out
}
func (s *Store) Create(repo, actor string, in LaunchInput) (Execution, error) {
	if repo == "" || actor == "" || !validate(in) {
		return Execution{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Execution{}, e
	}
	for _, x := range xs {
		if x.IdempotencyKey == in.IdempotencyKey {
			return x, nil
		}
		if x.State == "ready" && x.RunbookID == in.RunbookID && x.RunbookVersion == in.RunbookVersion && x.Origin.Kind == in.Origin.Kind && x.Origin.ResourceID == in.Origin.ResourceID && x.Origin.Revision == in.Origin.Revision {
			return x, ErrConflict
		}
	}
	bs := blockers(in)
	state := "ready"
	if len(bs) > 0 {
		state = "blocked"
	}
	now := s.now().UTC()
	steps := []StepState{}
	for _, p := range in.ActivePath {
		steps = append(steps, StepState{ID: p.ID, State: "pending", Health: "unknown", RollbackState: "not_required", Evidence: []string{}})
	}
	useState := in.RunbookUseState
	if useState == "" {
		useState = "approved"
	}
	x := Execution{ID: uid(), RepositoryID: repo, Revision: 1, LaunchInput: in, ControllerID: actor, CreatedAt: now, State: state, Blockers: bs, NonAuthority: []string{"Runbook execution controls and learning records coordinate already-authorized work; they grant no repository, secret, workflow, agent, communication, incident, deployment, environment, credential, or operational authority."}, Participants: []Participant{{actor, "human", "controller", now}}, Steps: steps, Credentials: []ScopedCredential{}, Events: []Event{{1, "launched", actor, "", "exact procedure and context frozen", now}}, ActionReceipts: []ActionReceipt{}, LearningReceipts: []LearningReceipt{}, Health: "unknown", RollbackState: "not_required", UseState: useState, FallbackRunbookID: in.ApprovedFallbackID, FallbackRunbookVersion: in.ApprovedFallbackVersion, UpdatedAt: now}
	if state == "ready" && len(steps) > 0 {
		x.State = "active"
	}
	refresh(&x)
	return x, s.write(x)
}

func (s *Store) Evaluate(repo, id, actor string, in EvaluationInput) (Execution, error) {
	if actor == "" || in.IdempotencyKey == "" || !allowed(in.Disposition, "completed", "abandoned") || len(in.Criteria) == 0 {
		return Execution{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	if x.Evaluation != nil {
		if x.Evaluation.IdempotencyKey == in.IdempotencyKey {
			return x, nil
		}
		return x, ErrConflict
	}
	if (x.State == "completed" && in.Disposition != "completed") || (x.State == "aborted" && in.Disposition != "abandoned") {
		return x, ErrInvalid
	}
	if x.Revision != in.ExpectedRevision || !terminal(x.State) {
		return x, ErrBlocked
	}
	wanted := map[string]bool{}
	for _, c := range x.OutcomeCriteria {
		wanted[c.Kind] = true
	}
	seen, proven := map[string]bool{}, in.Disposition == "completed"
	for _, c := range in.Criteria {
		if !wanted[c.Kind] || seen[c.Kind] || !allowed(c.Status, "met", "unmet", "unknown") || len(c.Evidence) == 0 || !unique(c.Evidence) {
			return x, ErrInvalid
		}
		seen[c.Kind] = true
		if c.Status != "met" {
			proven = false
		}
	}
	if len(seen) != len(wanted) {
		return x, ErrInvalid
	}
	timing := map[string]string{}
	for _, st := range x.Steps {
		var first, last time.Time
		for _, ev := range x.Events {
			if ev.StepID == st.ID {
				if first.IsZero() {
					first = ev.CreatedAt
				}
				last = ev.CreatedAt
			}
		}
		if !first.IsZero() {
			timing[st.ID] = last.Sub(first).String()
		}
	}
	findings := []Finding{}
	add := func(k, d string, evidence []string) {
		if strings.TrimSpace(d) != "" {
			findings = append(findings, Finding{ID: uid(), Kind: k, Detail: d, Evidence: append([]string{}, evidence...), Supported: len(evidence) > 0})
		}
	}
	for _, c := range in.Criteria {
		if c.Status != "met" {
			add("criterion_"+c.Status, c.Kind+": "+c.Detail, c.Evidence)
		}
	}
	for _, d := range in.Deviations {
		add("deviation", d, []string{"execution:" + x.ID})
	}
	for _, d := range in.ManualWork {
		add("manual_work", d, []string{"execution:" + x.ID})
	}
	for _, d := range in.AccessGaps {
		add("access_gap", d, []string{"execution:" + x.ID})
	}
	for _, d := range in.AgentCorrections {
		add("agent_correction", d, []string{"execution:" + x.ID})
	}
	for _, d := range in.FailedSteps {
		add("failed_step", d, []string{"execution:" + x.ID + "#" + d})
	}
	now := s.now().UTC()
	x.Evaluation = &Evaluation{ID: uid(), IdempotencyKey: in.IdempotencyKey, Disposition: in.Disposition, Criteria: in.Criteria, Deviations: in.Deviations, ManualWork: in.ManualWork, FailedSteps: in.FailedSteps, StepTiming: timing, AccessGaps: in.AccessGaps, AgentCorrections: in.AgentCorrections, Cost: x.Cost, ParticipantFeedback: in.Feedback, Findings: findings, OutcomeProven: proven, EvaluatedBy: actor, EvaluatedAt: now}
	x.Revision++
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "outcome_evaluated", actor, "", in.Disposition, now})
	x.PredictedNextAction = "review supported findings and keep the frozen procedure evidence"
	return x, s.write(x)
}

func (s *Store) Learn(repo, id, actor string, in LearningInput) (Execution, error) {
	if actor == "" || in.IdempotencyKey == "" {
		return Execution{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	for _, r := range x.LearningReceipts {
		if r.IdempotencyKey == in.IdempotencyKey {
			return x, nil
		}
	}
	if x.Revision != in.ExpectedRevision || x.Evaluation == nil {
		return x, ErrConflict
	}
	now := s.now().UTC()
	switch in.Action {
	case "link_improvement":
		if !allowed(in.ImprovementKind, "documentation", "workflow", "policy", "infrastructure", "code") || in.ImprovementReference == "" || in.ImprovementOwner == "" {
			return x, ErrInvalid
		}
		found := false
		for i := range x.Evaluation.Findings {
			if x.Evaluation.Findings[i].ID == in.FindingID && x.Evaluation.Findings[i].Supported {
				x.Evaluation.Findings[i].ImprovementKind = in.ImprovementKind
				x.Evaluation.Findings[i].ImprovementRef = in.ImprovementReference
				x.Evaluation.Findings[i].ImprovementOwner = in.ImprovementOwner
				found = true
			}
		}
		if !found {
			return x, ErrBlocked
		}
	case "record_revision":
		if in.ReviewedRunbookVersion <= x.RunbookVersion {
			return x, ErrInvalid
		}
		x.ReviewedRunbookVersion = in.ReviewedRunbookVersion
		x.FreshRehearsalID = ""
		x.FreshRehearsalRevision = 0
	case "record_fresh_rehearsal":
		if x.ReviewedRunbookVersion == 0 || in.FreshRehearsalID == "" || in.FreshRehearsalRevision < 1 {
			return x, ErrInvalid
		}
		x.FreshRehearsalID = in.FreshRehearsalID
		x.FreshRehearsalRevision = in.FreshRehearsalRevision
	case "suspend":
		if in.Reason == "" || in.FallbackRunbookID == "" || in.FallbackRunbookVersion < 1 {
			return x, ErrInvalid
		}
		x.UseState = "suspended"
		x.FallbackRunbookID = in.FallbackRunbookID
		x.FallbackRunbookVersion = in.FallbackRunbookVersion
	default:
		return x, ErrInvalid
	}
	x.Revision++
	x.UpdatedAt = now
	x.LearningReceipts = append(x.LearningReceipts, LearningReceipt{uid(), in.IdempotencyKey, in.Action, actor, now})
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "learning_" + in.Action, actor, "", in.Reason, now})
	return x, s.write(x)
}

func participant(x Execution, actor string) bool {
	for _, p := range x.Participants {
		if p.ID == actor {
			return true
		}
	}
	return false
}
func terminal(s string) bool { return s == "completed" || s == "aborted" }
func (s *Store) Control(repo, id, actor string, in ControlInput) (Execution, error) {
	if actor == "" || in.IdempotencyKey == "" || in.Cost < 0 {
		return Execution{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return x, e
	}
	for _, r := range x.ActionReceipts {
		if r.IdempotencyKey == in.IdempotencyKey {
			if r.ActorID == actor && r.Action == in.Action && r.StepID == in.StepID {
				return x, nil
			}
			return x, ErrConflict
		}
	}
	if x.Revision != in.ExpectedRevision {
		return x, ErrConflict
	}
	if terminal(x.State) {
		return x, ErrBlocked
	}
	now := s.now().UTC()
	controller := x.ControllerID == actor
	joined := participant(x, actor)
	idx := -1
	for i := range x.Steps {
		if x.Steps[i].ID == in.StepID {
			idx = i
		}
	}
	addEvent := func(k, step, body string) {
		x.Events = append(x.Events, Event{int64(len(x.Events) + 1), k, actor, step, body, now})
	}
	switch in.Action {
	case "join":
		if joined {
			return x, nil
		}
		kind := in.ActorKind
		if kind == "" {
			kind = "human"
		}
		if kind != "human" {
			return x, ErrForbidden
		}
		x.Participants = append(x.Participants, Participant{actor, kind, "participant", now})
		addEvent("joined", "", in.Body)
	case "discuss":
		if !joined || strings.TrimSpace(in.Body) == "" {
			return x, ErrForbidden
		}
		addEvent("discussion", in.StepID, in.Body)
	case "pause":
		if !controller {
			return x, ErrForbidden
		}
		x.State = "paused"
		for i := range x.Credentials {
			if x.Credentials[i].RevokedAt == nil {
				x.Credentials[i].RevokedAt = &now
			}
		}
		addEvent("paused", "", in.Body)
	case "resume":
		if !controller || x.State != "paused" {
			return x, ErrForbidden
		}
		x.State = "active"
		addEvent("resumed", "", in.Body)
	case "handoff":
		if !controller || in.TargetID == "" || !participant(x, in.TargetID) {
			return x, ErrForbidden
		}
		x.ControllerID = in.TargetID
		for i := range x.Participants {
			if x.Participants[i].ID == actor {
				x.Participants[i].Role = "participant"
			}
			if x.Participants[i].ID == in.TargetID {
				x.Participants[i].Role = "controller"
			}
		}
		addEvent("handed_off", "", in.TargetID+": "+in.Body)
	case "abort":
		if !controller {
			return x, ErrForbidden
		}
		x.State = "aborted"
		x.RollbackState = "required"
		for i := range x.Credentials {
			if x.Credentials[i].RevokedAt == nil {
				x.Credentials[i].RevokedAt = &now
			}
		}
		addEvent("aborted", "", in.Body)
	case "approve":
		if idx < 0 || !joined || actor == x.Steps[idx].ControllerID {
			return x, ErrForbidden
		}
		x.Steps[idx].ApprovedBy = actor
		addEvent("approved", in.StepID, in.Body)
	case "delegate":
		if !controller || idx < 0 || in.TargetID == "" || !allowed(in.Mode, "analyze", "execute") {
			return x, ErrForbidden
		}
		x.Steps[idx].DelegatedAgentID = in.TargetID
		x.Steps[idx].DelegatedMode = in.Mode
		addEvent("agent_delegated", in.StepID, in.TargetID+":"+in.Mode)
	case "skip":
		p := stepByID(x, in.StepID)
		if !controller || idx < 0 || !p.Optional || !p.PolicyPermitsSkip {
			return x, ErrForbidden
		}
		x.Steps[idx].State = "skipped"
		addEvent("step_skipped", in.StepID, in.Body)
	case "perform":
		if idx < 0 || x.State != "active" || (!joined && x.Steps[idx].DelegatedAgentID != actor) || (!controller && x.Steps[idx].ControllerID != "" && x.Steps[idx].ControllerID != actor && x.Steps[idx].DelegatedAgentID != actor) {
			return x, ErrForbidden
		}
		p := stepByID(x, in.StepID)
		if x.Steps[idx].DelegatedAgentID == actor && x.Steps[idx].DelegatedMode != "execute" {
			return x, ErrForbidden
		}
		for _, d := range p.DependsOn {
			for _, q := range x.Steps {
				if q.ID == d && q.State != "completed" && q.State != "skipped" {
					return x, ErrBlocked
				}
			}
		}
		if (p.Kind == "decision" || len(p.RequiredAuthority) > 0) && x.Steps[idx].ApprovedBy == "" {
			return x, ErrBlocked
		}
		if x.Steps[idx].ApprovedBy == actor {
			return x, ErrForbidden
		}
		if len(in.Evidence) == 0 || !unique(in.Evidence) || !allowed(in.Health, "healthy", "degraded", "unhealthy", "unknown") {
			return x, ErrInvalid
		}
		expiry := in.CredentialExpiresAt
		if expiry.IsZero() {
			expiry = now.Add(15 * time.Minute)
		}
		if !expiry.After(now) || expiry.After(now.Add(15*time.Minute)) {
			return x, ErrInvalid
		}
		cred := ScopedCredential{"runbook_step_" + uid(), in.StepID, actor, append([]string{}, p.RequiredAuthority...), now, expiry, nil, false}
		cred.RevokedAt = &now
		x.Credentials = append(x.Credentials, cred)
		x.Steps[idx].State = "completed"
		x.Steps[idx].ControllerID = actor
		x.Steps[idx].Evidence = append([]string{}, in.Evidence...)
		x.Steps[idx].Health = in.Health
		x.Steps[idx].Cost = in.Cost
		x.Cost += in.Cost
		x.Health = in.Health
		if in.Health == "unhealthy" {
			x.RollbackState = "required"
			x.Steps[idx].RollbackState = "required"
			x.State = "paused"
			x.Steps[idx].Blocker = "unhealthy evidence requires controller rollback review"
		}
		addEvent("step_performed", in.StepID, in.Body)
	default:
		return x, ErrInvalid
	}
	x.Revision++
	x.UpdatedAt = now
	refresh(&x)
	receipt := ActionReceipt{uid(), in.IdempotencyKey, in.Action, in.StepID, actor, x.RunbookVersion, x.Revision, "", append([]string{}, in.Evidence...), in.Cost, now}
	if len(x.Credentials) > 0 && in.Action == "perform" {
		receipt.CredentialReference = x.Credentials[len(x.Credentials)-1].Reference
	}
	x.ActionReceipts = append(x.ActionReceipts, receipt)
	return x, s.write(x)
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) write(x Execution) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, id string) (Execution, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Execution{}, ErrNotFound
	}
	var x Execution
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Get(repo, id string) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) list(repo string) ([]Execution, error) {
	ps, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	sort.Strings(ps)
	out := []Execution{}
	for _, p := range ps {
		b, x := os.ReadFile(p)
		var v Execution
		if x == nil {
			x = json.Unmarshal(b, &v)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	return out, e
}
func (s *Store) List(repo string) ([]Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repo)
}

func (s *Store) RunbookStatus(repo, runbook string, version int64) (string, string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return "", "", 0, e
	}
	var latest *Execution
	for i := range xs {
		x := &xs[i]
		if x.RunbookID == runbook && x.RunbookVersion == version && x.UseState == "suspended" && (latest == nil || x.UpdatedAt.After(latest.UpdatedAt)) {
			latest = x
		}
	}
	if latest != nil {
		return "suspended", latest.FallbackRunbookID, latest.FallbackRunbookVersion, nil
	}
	return "approved", "", 0, nil
}
