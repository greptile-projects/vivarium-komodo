package relationships

import (
	"strings"
	"time"
)

// EvolutionRollout is the durable coordination layer over the platform's
// existing queue, release, deployment, and recovery resources. It records
// evidence links; those systems continue to own execution and authorization.
type EvolutionRollout struct {
	VerificationID string                  `json:"verification_id"`
	State          string                  `json:"state"`
	Phases         []EvolutionRolloutPhase `json:"phases"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type EvolutionRolloutPhase struct {
	ID        string                     `json:"id"`
	Position  int                        `json:"position"`
	Name      string                     `json:"name"`
	Gates     []string                   `json:"compatibility_gates"`
	Steps     []EvolutionRolloutStep     `json:"steps"`
	State     string                     `json:"state"`
	Approvals []EvolutionRolloutApproval `json:"approvals"`
	Outcomes  []EvolutionRolloutOutcome  `json:"outcomes"`
}

type EvolutionRolloutStep struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Kind         string `json:"kind"`
	ResourceID   string `json:"resource_id"`
	State        string `json:"state"`
}

type EvolutionRolloutApproval struct {
	RepositoryID string    `json:"repository_id"`
	ActorID      string    `json:"actor_id"`
	Decision     string    `json:"decision"`
	Note         string    `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type EvolutionRolloutOutcome struct {
	StepID       string    `json:"step_id"`
	RepositoryID string    `json:"repository_id"`
	Kind         string    `json:"kind"`
	ResourceID   string    `json:"resource_id"`
	State        string    `json:"state"`
	ActorID      string    `json:"actor_id"`
	Note         string    `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type EvolutionRolloutPhaseInput struct {
	Name  string                 `json:"name"`
	Gates []string               `json:"compatibility_gates"`
	Steps []EvolutionRolloutStep `json:"steps"`
}

func (s *Store) ConfigureEvolutionRollout(planID, actor, verificationID string, inputs []EvolutionRolloutPhaseInput) (EvolutionPlan, error) {
	if actor == "" || verificationID == "" || len(inputs) == 0 || len(inputs) > 25 {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	if v.Rollout != nil {
		return v, ErrConflict
	}
	found := false
	for _, verification := range v.Verifications {
		if verification.ID == verificationID {
			found = true
		}
	}
	if !found {
		return v, ErrInvalid
	}
	allowed := map[string]bool{v.RepositoryID: true}
	for _, consumer := range v.AffectedConsumers {
		allowed[consumer.RepositoryID] = true
	}
	rollout := &EvolutionRollout{VerificationID: verificationID, State: "awaiting_approval", Phases: []EvolutionRolloutPhase{}, UpdatedAt: s.now().UTC()}
	for i, input := range inputs {
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len(input.Steps) == 0 || len(input.Gates) == 0 {
			return v, ErrInvalid
		}
		phaseID, _ := newID()
		phase := EvolutionRolloutPhase{ID: phaseID, Position: i + 1, Name: input.Name, Gates: []string{}, Steps: []EvolutionRolloutStep{}, State: "pending", Approvals: []EvolutionRolloutApproval{}, Outcomes: []EvolutionRolloutOutcome{}}
		for _, gate := range input.Gates {
			gate = strings.TrimSpace(gate)
			if gate == "" {
				return v, ErrInvalid
			}
			phase.Gates = append(phase.Gates, gate)
		}
		seen := map[string]bool{}
		for _, step := range input.Steps {
			step.RepositoryID, step.Kind = strings.TrimSpace(step.RepositoryID), strings.ToLower(strings.TrimSpace(step.Kind))
			if !allowed[step.RepositoryID] || !oneOfEvolution(step.Kind, "queue", "release", "deployment", "rollback", "repair") || seen[step.RepositoryID+":"+step.Kind] {
				return v, ErrInvalid
			}
			seen[step.RepositoryID+":"+step.Kind] = true
			step.ID, _ = newID()
			step.State = "pending"
			step.ResourceID = ""
			phase.Steps = append(phase.Steps, step)
		}
		rollout.Phases = append(rollout.Phases, phase)
	}
	v.Rollout = rollout
	v.UpdatedAt = rollout.UpdatedAt
	return v, s.write("evolutions", v.ID, v)
}

func (s *Store) ApproveEvolutionRollout(planID, phaseID, repositoryID, actor, decision, note string) (EvolutionPlan, error) {
	decision, note = strings.ToLower(strings.TrimSpace(decision)), strings.TrimSpace(note)
	if actor == "" || repositoryID == "" || !oneOfEvolution(decision, "approve", "reject") {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	phase := rolloutPhase(&v, phaseID)
	if phase == nil {
		return v, ErrNotFound
	}
	needed := false
	for _, step := range phase.Steps {
		if step.RepositoryID == repositoryID {
			needed = true
		}
	}
	if !needed || oneOfEvolution(phase.State, "completed", "failed") {
		return v, ErrConflict
	}
	a := EvolutionRolloutApproval{RepositoryID: repositoryID, ActorID: actor, Decision: decision, Note: note, CreatedAt: s.now().UTC()}
	replaced := false
	for i := range phase.Approvals {
		if phase.Approvals[i].RepositoryID == repositoryID {
			phase.Approvals[i] = a
			replaced = true
		}
	}
	if !replaced {
		phase.Approvals = append(phase.Approvals, a)
	}
	deriveRollout(&v)
	v.UpdatedAt = a.CreatedAt
	v.Rollout.UpdatedAt = a.CreatedAt
	return v, s.write("evolutions", v.ID, v)
}

func (s *Store) RecordEvolutionRolloutOutcome(planID, phaseID, stepID, actor, resourceID, state, note string) (EvolutionPlan, error) {
	state, resourceID, note = strings.ToLower(strings.TrimSpace(state)), strings.TrimSpace(resourceID), strings.TrimSpace(note)
	if actor == "" || resourceID == "" || !oneOfEvolution(state, "pending", "running", "succeeded", "failed", "rolled_back", "repairing") {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	phase := rolloutPhase(&v, phaseID)
	if phase == nil {
		return v, ErrNotFound
	}
	if phase.State != "ready" && phase.State != "running" && phase.State != "paused" {
		return v, ErrConflict
	}
	var step *EvolutionRolloutStep
	for i := range phase.Steps {
		if phase.Steps[i].ID == stepID {
			step = &phase.Steps[i]
		}
	}
	if step == nil {
		return v, ErrNotFound
	}
	step.ResourceID, step.State = resourceID, state
	now := s.now().UTC()
	phase.Outcomes = append(phase.Outcomes, EvolutionRolloutOutcome{StepID: step.ID, RepositoryID: step.RepositoryID, Kind: step.Kind, ResourceID: resourceID, State: state, ActorID: actor, Note: note, CreatedAt: now})
	deriveRollout(&v)
	v.UpdatedAt = now
	v.Rollout.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}

func rolloutPhase(v *EvolutionPlan, id string) *EvolutionRolloutPhase {
	if v.Rollout == nil {
		return nil
	}
	for i := range v.Rollout.Phases {
		if v.Rollout.Phases[i].ID == id {
			return &v.Rollout.Phases[i]
		}
	}
	return nil
}

func deriveRollout(v *EvolutionPlan) {
	if v.Rollout == nil {
		return
	}
	previousComplete := true
	allComplete := true
	for i := range v.Rollout.Phases {
		p := &v.Rollout.Phases[i]
		approved := map[string]bool{}
		rejected := false
		for _, a := range p.Approvals {
			approved[a.RepositoryID] = a.Decision == "approve"
			rejected = rejected || a.Decision == "reject"
		}
		allApproved := true
		anyStarted := false
		allSucceeded := true
		failed := false
		for _, step := range p.Steps {
			allApproved = allApproved && approved[step.RepositoryID]
			anyStarted = anyStarted || step.State != "pending"
			allSucceeded = allSucceeded && oneOfEvolution(step.State, "succeeded", "rolled_back")
			failed = failed || step.State == "failed"
		}
		switch {
		case failed:
			p.State = "paused"
		case rejected:
			p.State = "blocked"
		case allSucceeded:
			p.State = "completed"
		case anyStarted:
			p.State = "running"
		case previousComplete && allApproved:
			p.State = "ready"
		default:
			p.State = "pending"
		}
		previousComplete = p.State == "completed"
		allComplete = allComplete && previousComplete
	}
	if allComplete {
		v.Rollout.State = "completed"
	} else {
		v.Rollout.State = "active"
		for _, p := range v.Rollout.Phases {
			if p.State == "paused" {
				v.Rollout.State = "paused"
			}
			if p.State == "blocked" && v.Rollout.State != "paused" {
				v.Rollout.State = "blocked"
			}
		}
	}
}
