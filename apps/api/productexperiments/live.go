package productexperiments

import (
	"strings"
	"time"
)

// Run is one retained attempt to expose an approved, exact-release variant set.
// Attempts are never reused: a stopped or contained run remains inspectable and
// a retry starts a new run.
type Run struct {
	ID                    string        `json:"id"`
	PlanVersion           int64         `json:"plan_version"`
	AudiencePolicyVersion int64         `json:"audience_policy_version"`
	ReleaseID             string        `json:"release_id"`
	ReleaseCommitID       string        `json:"release_commit_id"`
	EnvironmentID         string        `json:"environment_id"`
	DeploymentID          string        `json:"deployment_id"`
	Status                string        `json:"status"`
	CurrentStage          int           `json:"current_stage"`
	Stages                []RunStage    `json:"stages"`
	Observations          []Observation `json:"observations"`
	Controls              []RunControl  `json:"controls"`
	StartedByID           string        `json:"started_by_id"`
	StartedAt             time.Time     `json:"started_at"`
	EndedAt               *time.Time    `json:"ended_at,omitempty"`
	ContainmentReason     string        `json:"containment_reason,omitempty"`
	ExposureAuthority     bool          `json:"exposure_authority"`
}

type RunStage struct {
	Number      int          `json:"number"`
	Name        string       `json:"name"`
	Allocation  []Allocation `json:"allocation"`
	MaxExposure int          `json:"max_exposure"`
	ActorID     string       `json:"actor_id"`
	Reason      string       `json:"reason"`
	CreatedAt   time.Time    `json:"created_at"`
}

type Observation struct {
	ID                    string             `json:"id"`
	Stage                 int                `json:"stage"`
	ExposureByVariant     map[string]int     `json:"exposure_by_variant"`
	MeasureValues         map[string]float64 `json:"measure_values"`
	Uncertainty           map[string]float64 `json:"uncertainty"`
	DataQuality           string             `json:"data_quality"`
	OperationalHealth     string             `json:"operational_health"`
	InstrumentationHealth string             `json:"instrumentation_health"`
	ConsentHealth         string             `json:"consent_health"`
	SampleImbalance       bool               `json:"sample_imbalance"`
	GuardrailBreached     bool               `json:"guardrail_breached"`
	DeploymentFailed      bool               `json:"deployment_failed"`
	CostUnits             float64            `json:"cost_units"`
	Evidence              []string           `json:"evidence"`
	ActorID               string             `json:"actor_id"`
	ObservedAt            time.Time          `json:"observed_at"`
	ContainmentTriggered  bool               `json:"containment_triggered"`
}

type ObservationInput struct {
	ExposureByVariant     map[string]int     `json:"exposure_by_variant"`
	MeasureValues         map[string]float64 `json:"measure_values"`
	Uncertainty           map[string]float64 `json:"uncertainty"`
	DataQuality           string             `json:"data_quality"`
	OperationalHealth     string             `json:"operational_health"`
	InstrumentationHealth string             `json:"instrumentation_health"`
	ConsentHealth         string             `json:"consent_health"`
	SampleImbalance       bool               `json:"sample_imbalance"`
	GuardrailBreached     bool               `json:"guardrail_breached"`
	DeploymentFailed      bool               `json:"deployment_failed"`
	CostUnits             float64            `json:"cost_units"`
	Evidence              []string           `json:"evidence"`
}

type RunControl struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Launch(repo, eid, actor, environment, deployment string, stages []RunStage) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if v.Cleanup != nil { return ErrConflict }
		resolved := s.resolve(repo, *v)
		if !resolved.Ready || len(resolved.AudiencePolicies) == 0 || environment == "" || deployment == "" || len(stages) == 0 {
			return ErrConflict
		}
		policy := resolved.AudiencePolicies[len(resolved.AudiencePolicies)-1]
		if !policy.Ready || !participant(resolved, actor) {
			return ErrInvalid
		}
		for _, run := range v.Runs {
			if run.Status == "running" || run.Status == "paused" {
				return ErrConflict
			}
		}
		previous := -1
		for i := range stages {
			if !validStage(policy, stages[i], i+1, previous) {
				return ErrInvalid
			}
			stages[i].Number, stages[i].ActorID, stages[i].CreatedAt = i+1, actor, s.now()
			previous = stages[i].MaxExposure
		}
		v.Runs = append(v.Runs, Run{ID: id("run_"), PlanVersion: v.CurrentVersion, AudiencePolicyVersion: policy.Version, ReleaseID: policy.ReleaseID, ReleaseCommitID: policy.ReleaseCommitID, EnvironmentID: environment, DeploymentID: deployment, Status: "running", CurrentStage: 1, Stages: stages, StartedByID: actor, StartedAt: s.now()})
		return nil
	})
}

func validStage(policy AudiencePolicy, stage RunStage, number, previous int) bool {
	if strings.TrimSpace(stage.Name) == "" || stage.MaxExposure < 1 || stage.MaxExposure > 10000 || (previous >= 0 && stage.MaxExposure < previous) || len(stage.Allocation) != len(policy.Allocation) {
		return false
	}
	total, seen := 0, map[string]bool{}
	for _, a := range stage.Allocation {
		if a.BasisPoints < 0 || seen[a.VariantID] || !contains(policy.VariantIDs, a.VariantID) {
			return false
		}
		seen[a.VariantID], total = true, total+a.BasisPoints
	}
	return total == 10000 && number > 0
}

func (s *Store) Advance(repo, eid, runID, actor, reason string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		r := findRun(v, runID)
		if r == nil || r.Status != "running" || r.CurrentStage >= len(r.Stages) || !participant(*v, actor) || strings.TrimSpace(reason) == "" {
			return ErrInvalid
		}
		r.CurrentStage++
		r.Controls = append(r.Controls, RunControl{ID: id("ctl_"), Action: "advance", Reason: reason, ActorID: actor, CreatedAt: s.now()})
		return nil
	})
}

func (s *Store) Observe(repo, eid, runID, actor string, in ObservationInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		r := findRun(v, runID)
		if r == nil || (r.Status != "running" && r.Status != "paused") || !participant(*v, actor) || !one(in.DataQuality, "healthy", "degraded", "lost") || !one(in.OperationalHealth, "healthy", "degraded", "failed") || !one(in.InstrumentationHealth, "healthy", "degraded", "lost") || !one(in.ConsentHealth, "valid", "revoked") || in.CostUnits < 0 {
			return ErrInvalid
		}
		o := Observation{ID: id("obs_"), Stage: r.CurrentStage, ExposureByVariant: in.ExposureByVariant, MeasureValues: in.MeasureValues, Uncertainty: in.Uncertainty, DataQuality: in.DataQuality, OperationalHealth: in.OperationalHealth, InstrumentationHealth: in.InstrumentationHealth, ConsentHealth: in.ConsentHealth, SampleImbalance: in.SampleImbalance, GuardrailBreached: in.GuardrailBreached, DeploymentFailed: in.DeploymentFailed, CostUnits: in.CostUnits, Evidence: in.Evidence, ActorID: actor, ObservedAt: s.now()}
		reasons := []string{}
		if in.GuardrailBreached {
			reasons = append(reasons, "guardrail breach")
		}
		if in.DeploymentFailed || in.OperationalHealth == "failed" {
			reasons = append(reasons, "deployment failure")
		}
		if in.InstrumentationHealth == "lost" || in.DataQuality == "lost" {
			reasons = append(reasons, "instrumentation loss")
		}
		if in.SampleImbalance {
			reasons = append(reasons, "sample imbalance")
		}
		if in.ConsentHealth == "revoked" {
			reasons = append(reasons, "revoked consent")
		}
		if len(reasons) > 0 && r.Status != "stopped" {
			r.Status = "contained"
			r.ContainmentReason = strings.Join(reasons, "; ")
			now := s.now()
			r.EndedAt = &now
			o.ContainmentTriggered = true
			r.Controls = append(r.Controls, RunControl{ID: id("ctl_"), Action: "contain", Reason: r.ContainmentReason, ActorID: "system", CreatedAt: now})
		}
		r.Observations = append(r.Observations, o)
		return nil
	})
}

func (s *Store) Control(repo, eid, runID, actor, action, reason string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		r := findRun(v, runID)
		if r == nil || !participant(*v, actor) || strings.TrimSpace(reason) == "" || !one(action, "pause", "resume", "stop") {
			return ErrInvalid
		}
		if (action == "pause" && r.Status != "running") || (action == "resume" && r.Status != "paused") || (action == "stop" && r.Status != "running" && r.Status != "paused") {
			return ErrConflict
		}
		if action == "pause" {
			r.Status = "paused"
		}
		if action == "resume" {
			r.Status = "running"
		}
		if action == "stop" {
			r.Status = "stopped"
			now := s.now()
			r.EndedAt = &now
		}
		r.Controls = append(r.Controls, RunControl{ID: id("ctl_"), Action: action, Reason: reason, ActorID: actor, CreatedAt: s.now()})
		return nil
	})
}

func findRun(v *Experiment, id string) *Run {
	for i := range v.Runs {
		if v.Runs[i].ID == id {
			return &v.Runs[i]
		}
	}
	return nil
}
func participant(v Experiment, actor string) bool {
	if len(v.Versions) == 0 {
		return false
	}
	p := v.Versions[len(v.Versions)-1]
	return contains(p.OwnerIDs, actor) || contains(p.ParticipantIDs, actor)
}
