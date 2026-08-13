package productexperiments

import (
	"strings"
	"time"
)

type SegmentEffect struct {
	Segment     string  `json:"segment"`
	VariantID   string  `json:"variant_id"`
	MeasureID   string  `json:"measure_id"`
	Effect      float64 `json:"effect"`
	Uncertainty float64 `json:"uncertainty"`
	SampleSize  int     `json:"sample_size"`
}
type GuardrailOutcome struct {
	MeasureID   string  `json:"measure_id"`
	Status      string  `json:"status"`
	Value       float64 `json:"value"`
	Uncertainty float64 `json:"uncertainty"`
}
type Interpretation struct {
	Summary     string   `json:"summary"`
	ActorKind   string   `json:"actor_kind"`
	ActorID     string   `json:"actor_id"`
	Evidence    []string `json:"evidence"`
	Uncertainty string   `json:"uncertainty"`
}
type Dissent struct {
	ActorID  string   `json:"actor_id"`
	Position string   `json:"position"`
	Evidence []string `json:"evidence"`
}
type AnalysisInput struct {
	RunID              string             `json:"run_id"`
	ObservationID      string             `json:"observation_id"`
	EvidenceState      string             `json:"evidence_state"`
	Summary            string             `json:"summary"`
	SegmentEffects     []SegmentEffect    `json:"segment_effects"`
	Exclusions         []string           `json:"exclusions"`
	Guardrails         []GuardrailOutcome `json:"guardrails"`
	Interpretation     Interpretation     `json:"interpretation"`
	Dissent            []Dissent          `json:"dissent"`
	AggregatedEvidence []string           `json:"aggregated_evidence"`
}
type Analysis struct {
	ID                 string             `json:"id"`
	PlanVersion        int64              `json:"plan_version"`
	RunID              string             `json:"run_id"`
	ObservationID      string             `json:"observation_id"`
	EvidenceState      string             `json:"evidence_state"`
	Summary            string             `json:"summary"`
	SegmentEffects     []SegmentEffect    `json:"segment_effects"`
	Exclusions         []string           `json:"exclusions"`
	Guardrails         []GuardrailOutcome `json:"guardrails"`
	Interpretation     Interpretation     `json:"interpretation"`
	Dissent            []Dissent          `json:"dissent"`
	AggregatedEvidence []string           `json:"aggregated_evidence"`
	AuthorID           string             `json:"author_id"`
	CreatedAt          time.Time          `json:"created_at"`
}
type OutcomeTask struct {
	ID                   string     `json:"id"`
	Kind                 string     `json:"kind"`
	Title                string     `json:"title"`
	OwnerID              string     `json:"owner_id"`
	RequiredActions      []string   `json:"required_actions"`
	Status               string     `json:"status"`
	PullRequestID        string     `json:"pull_request_id,omitempty"`
	ReleaseID            string     `json:"release_id,omitempty"`
	DeploymentID         string     `json:"deployment_id,omitempty"`
	Evidence             []string   `json:"evidence,omitempty"`
	CompletedByID        string     `json:"completed_by_id,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	OperationalAuthority bool       `json:"operational_authority"`
}
type DecisionInput struct {
	ExpectedVersion  int64         `json:"expected_version"`
	AnalysisID       string        `json:"analysis_id"`
	Outcome          string        `json:"outcome"`
	AdoptedVariantID string        `json:"adopted_variant_id"`
	Rationale        string        `json:"rationale"`
	UserProtections  []string      `json:"user_protections"`
	Tasks            []OutcomeTask `json:"tasks"`
	ChangeReason     string        `json:"change_reason"`
}
type OutcomeDecision struct {
	ID                   string        `json:"id"`
	Version              int64         `json:"version"`
	PlanVersion          int64         `json:"plan_version"`
	AnalysisID           string        `json:"analysis_id"`
	Outcome              string        `json:"outcome"`
	AdoptedVariantID     string        `json:"adopted_variant_id,omitempty"`
	Rationale            string        `json:"rationale"`
	UserProtections      []string      `json:"user_protections"`
	Tasks                []OutcomeTask `json:"tasks"`
	ChangeReason         string        `json:"change_reason"`
	AuthorID             string        `json:"author_id"`
	CreatedAt            time.Time     `json:"created_at"`
	Current              bool          `json:"current"`
	Complete             bool          `json:"complete"`
	OperationalAuthority bool          `json:"operational_authority"`
}
type TaskCompletion struct {
	PullRequestID string   `json:"pull_request_id"`
	ReleaseID     string   `json:"release_id"`
	DeploymentID  string   `json:"deployment_id"`
	Evidence      []string `json:"evidence"`
}
type CleanupReceipt struct {
	DecisionID                 string    `json:"decision_id"`
	DecisionVersion            int64     `json:"decision_version"`
	ObsoleteVariantsRemoved    bool      `json:"obsolete_variants_removed"`
	TargetingRulesRemoved      bool      `json:"targeting_rules_removed"`
	CredentialsRevoked         bool      `json:"credentials_revoked"`
	CollectionStopped          bool      `json:"collection_stopped"`
	AggregatedEvidenceRetained []string  `json:"aggregated_evidence_retained"`
	ProvenanceRetained         bool      `json:"provenance_retained"`
	UserProtections            []string  `json:"user_protections"`
	CompletedByID              string    `json:"completed_by_id"`
	CompletedAt                time.Time `json:"completed_at"`
}

func (s *Store) AddAnalysis(repo, eid, actor string, in AnalysisInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if !participant(*v, actor) || !one(in.EvidenceState, "threshold_reached", "stop_condition_reached") || strings.TrimSpace(in.Summary) == "" || len(in.AggregatedEvidence) == 0 {
			return ErrInvalid
		}
		r := findRun(v, in.RunID)
		if r == nil {
			return ErrInvalid
		}
		found := false
		for _, o := range r.Observations {
			if o.ID == in.ObservationID {
				found = true
			}
		}
		if !found {
			return ErrInvalid
		}
		for _, g := range in.Guardrails {
			if !one(g.Status, "passed", "breached", "uncertain") {
				return ErrInvalid
			}
		}
		if in.Interpretation.Summary != "" && (!one(in.Interpretation.ActorKind, "human", "agent") || in.Interpretation.ActorID == "") {
			return ErrInvalid
		}
		v.Analyses = append(v.Analyses, Analysis{ID: id("ana_"), PlanVersion: v.CurrentVersion, RunID: in.RunID, ObservationID: in.ObservationID, EvidenceState: in.EvidenceState, Summary: in.Summary, SegmentEffects: in.SegmentEffects, Exclusions: in.Exclusions, Guardrails: in.Guardrails, Interpretation: in.Interpretation, Dissent: in.Dissent, AggregatedEvidence: in.AggregatedEvidence, AuthorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Decide(repo, eid, actor string, in DecisionInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if !participant(*v, actor) || !one(in.Outcome, "adopt_variant", "retain_control", "extend_test", "inconclusive") || in.Rationale == "" || in.ChangeReason == "" || len(in.Tasks) == 0 {
			return ErrInvalid
		}
		version := int64(1)
		if len(v.Decisions) > 0 {
			version = v.Decisions[len(v.Decisions)-1].Version + 1
		}
		if in.ExpectedVersion != version-1 {
			return ErrConflict
		}
		var a *Analysis
		for i := range v.Analyses {
			if v.Analyses[i].ID == in.AnalysisID {
				a = &v.Analyses[i]
			}
		}
		if a == nil || a.PlanVersion != v.CurrentVersion {
			return ErrInvalid
		}
		if in.Outcome == "adopt_variant" && !declaredVariants(v.Versions[len(v.Versions)-1], []string{in.AdoptedVariantID}) {
			return ErrInvalid
		}
		if in.Outcome != "adopt_variant" && in.AdoptedVariantID != "" {
			return ErrInvalid
		}
		for i := range in.Tasks {
			if !one(in.Tasks[i].Kind, "rollout", "rollback", "follow_up", "cleanup") || in.Tasks[i].Title == "" || in.Tasks[i].OwnerID == "" || len(in.Tasks[i].RequiredActions) == 0 {
				return ErrInvalid
			}
			in.Tasks[i].ID = id("task_")
			in.Tasks[i].Status = "open"
		}
		for i := range v.Decisions {
			v.Decisions[i].Current = false
		}
		v.Decisions = append(v.Decisions, OutcomeDecision{ID: id("out_"), Version: version, PlanVersion: v.CurrentVersion, AnalysisID: a.ID, Outcome: in.Outcome, AdoptedVariantID: in.AdoptedVariantID, Rationale: in.Rationale, UserProtections: in.UserProtections, Tasks: in.Tasks, ChangeReason: in.ChangeReason, AuthorID: actor, CreatedAt: s.now(), Current: true})
		return nil
	})
}
func (s *Store) CompleteOutcomeTask(repo, eid, decision, task, actor string, in TaskCompletion) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if len(v.Decisions) == 0 {
			return ErrNotFound
		}
		d := &v.Decisions[len(v.Decisions)-1]
		if d.ID != decision || !d.Current {
			return ErrConflict
		}
		if !participant(*v, actor) || in.PullRequestID == "" || in.ReleaseID == "" || in.DeploymentID == "" || len(in.Evidence) == 0 {
			return ErrInvalid
		}
		found := false
		for i := range d.Tasks {
			if d.Tasks[i].ID == task {
				if d.Tasks[i].Status == "completed" {
					return nil
				}
				now := s.now()
				d.Tasks[i].Status = "completed"
				d.Tasks[i].PullRequestID = in.PullRequestID
				d.Tasks[i].ReleaseID = in.ReleaseID
				d.Tasks[i].DeploymentID = in.DeploymentID
				d.Tasks[i].Evidence = in.Evidence
				d.Tasks[i].CompletedByID = actor
				d.Tasks[i].CompletedAt = &now
				found = true
			}
		}
		if !found {
			return ErrNotFound
		}
		d.Complete = true
		for _, t := range d.Tasks {
			d.Complete = d.Complete && t.Status == "completed"
		}
		return nil
	})
}
func (s *Store) CompleteCleanup(repo, eid, decision, actor string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if len(v.Decisions) == 0 {
			return ErrNotFound
		}
		d := &v.Decisions[len(v.Decisions)-1]
		if d.ID != decision || !d.Current || !d.Complete {
			return ErrConflict
		}
		if !participant(*v, actor) {
			return ErrInvalid
		}
		var a Analysis
		for _, x := range v.Analyses {
			if x.ID == d.AnalysisID {
				a = x
			}
		}
		v.Cleanup = &CleanupReceipt{DecisionID: d.ID, DecisionVersion: d.Version, ObsoleteVariantsRemoved: true, TargetingRulesRemoved: true, CredentialsRevoked: true, CollectionStopped: true, AggregatedEvidenceRetained: a.AggregatedEvidence, ProvenanceRetained: true, UserProtections: d.UserProtections, CompletedByID: actor, CompletedAt: s.now()}
		return nil
	})
}
