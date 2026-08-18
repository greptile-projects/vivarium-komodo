package infrastructureplans

import (
	"sort"
	"strings"
	"time"
)

// Execution is the authoritative, environment-owned application of one exact
// merged plan. It stores only a reference to a provider lease, never its value.
type Execution struct {
	ID                 string              `json:"id"`
	EnvironmentID      string              `json:"environment_id"`
	PlanRevision       string              `json:"plan_revision"`
	MergedRevision     string              `json:"merged_revision"`
	Credential         ExecutionCredential `json:"credential"`
	Budget             ExecutionBudget     `json:"budget"`
	State              string              `json:"state"`
	ActiveControllerID string              `json:"active_controller_id"`
	InitiatedByID      string              `json:"initiated_by_id"`
	CreatedAt          time.Time           `json:"created_at"`
	StartedAt          *time.Time          `json:"started_at,omitempty"`
	CompletedAt        *time.Time          `json:"completed_at,omitempty"`
	Approvals          []ExecutionApproval `json:"approvals"`
	Steps              []ExecutionStep     `json:"steps"`
	Delegations        []StepDelegation    `json:"delegations"`
	Events             []ExecutionEvent    `json:"events"`
	CurrentStepID      string              `json:"current_step_id,omitempty"`
	Spent              float64             `json:"spent"`
	Health             string              `json:"health"`
	Blockers           []string            `json:"blockers"`
	NextActions        []string            `json:"next_actions"`
	SafeControls       []string            `json:"safe_controls"`
	NonAuthority       []string            `json:"non_authority"`
}
type ExecutionCredential struct {
	Reference      string    `json:"reference"`
	Provider       string    `json:"provider"`
	Scopes         []string  `json:"scopes"`
	EnvironmentID  string    `json:"environment_id"`
	ExpiresAt      time.Time `json:"expires_at"`
	SecretRetained bool      `json:"secret_retained"`
}
type ExecutionBudget struct {
	MaximumCost float64 `json:"maximum_cost"`
	Currency    string  `json:"currency"`
}
type ExecutionApproval struct {
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type StepDelegation struct {
	StepID    string    `json:"step_id"`
	AgentID   string    `json:"agent_id"`
	Actions   []string  `json:"actions"`
	ExpiresAt time.Time `json:"expires_at"`
}
type ExecutionStep struct {
	ResourceID       string     `json:"resource_id"`
	Action           string     `json:"action"`
	Position         int        `json:"position"`
	State            string     `json:"state"`
	SafetyPoint      bool       `json:"safety_point"`
	ControllerID     string     `json:"controller_id,omitempty"`
	ProviderResponse string     `json:"provider_response,omitempty"`
	Health           string     `json:"health"`
	Cost             float64    `json:"cost"`
	Blocker          string     `json:"blocker,omitempty"`
	NextAction       string     `json:"next_action,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
}
type ExecutionEvent struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	StepID    string    `json:"step_id,omitempty"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}
type ExecutionInput struct {
	EnvironmentID string              `json:"environment_id"`
	Credential    ExecutionCredential `json:"credential"`
	Budget        ExecutionBudget     `json:"budget"`
	ControllerID  string              `json:"controller_id"`
	Delegations   []StepDelegation    `json:"delegations"`
}
type StepUpdate struct {
	State            string  `json:"state"`
	ProviderResponse string  `json:"provider_response"`
	Health           string  `json:"health"`
	Cost             float64 `json:"cost"`
	Blocker          string  `json:"blocker"`
	NextAction       string  `json:"next_action"`
	SafetyPoint      bool    `json:"safety_point"`
}

func (s *Store) StartExecution(repo, pull, plan, actor string, in ExecutionInput) (Plan, error) {
	now := s.now().UTC()
	if actor == "" || in.ControllerID != actor || in.EnvironmentID == "" || in.Credential.Reference == "" || in.Credential.Provider == "" || len(in.Credential.Scopes) == 0 || in.Credential.EnvironmentID != in.EnvironmentID || !in.Credential.ExpiresAt.After(now) || in.Credential.ExpiresAt.After(now.Add(24*time.Hour)) || in.Credential.SecretRetained || in.Budget.MaximumCost <= 0 || in.Budget.Currency == "" || secretShaped(in.Credential.Reference+in.Credential.Provider+strings.Join(in.Credential.Scopes, "")) {
		return Plan{}, ErrInvalid
	}
	outcome, ok := s.pulls.(PullOutcome)
	if !ok || s.environments == nil {
		return Plan{}, ErrInvalid
	}
	source, mergedRevision, merged, err := outcome.MergedRevision(repo, pull)
	if err != nil || !merged {
		return Plan{}, ErrInvalid
	}
	required, exists := s.environments.ExecutionEnvironment(repo, in.EnvironmentID)
	if !exists {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		current := s.derive(*p)
		if current.Stale || current.Input.Revision != source {
			return ErrInvalid
		}
		for _, c := range current.Input.Changes {
			found := false
			for _, e := range c.EnvironmentIDs {
				if e == in.EnvironmentID {
					found = true
				}
			}
			if !found {
				continue
			}
			if len(c.OwnerIDs) == 0 {
				return ErrInvalid
			}
			for _, owner := range c.OwnerIDs {
				acknowledged := false
				for _, a := range current.Acknowledgements {
					if a.OwnerID == owner && a.Decision == "acknowledged" && a.Current {
						acknowledged = true
					}
				}
				if !acknowledged {
					return ErrInvalid
				}
			}
		}
		for _, effect := range current.Input.PolicyEffects {
			if effect.Effect != "satisfy" {
				return ErrInvalid
			}
		}
		ready := false
		for _, rehearsal := range current.Rehearsals {
			if rehearsal.Current && rehearsal.Ready {
				ready = true
			}
		}
		if !ready {
			return ErrInvalid
		}
		steps := []ExecutionStep{}
		pos := 0
		for _, resource := range p.DependencyOrder {
			for _, c := range p.Input.Changes {
				if c.ResourceID != resource {
					continue
				}
				affected := false
				for _, e := range c.EnvironmentIDs {
					if e == in.EnvironmentID {
						affected = true
					}
				}
				if affected {
					pos++
					steps = append(steps, ExecutionStep{ResourceID: c.ResourceID, Action: c.Action, Position: pos, State: "pending", SafetyPoint: true, Health: "unknown"})
				}
			}
		}
		if len(steps) == 0 {
			return ErrInvalid
		}
		known := map[string]bool{}
		for _, st := range steps {
			known[st.ResourceID] = true
		}
		for _, d := range in.Delegations {
			if !known[d.StepID] || d.AgentID == "" || len(d.Actions) == 0 || !d.ExpiresAt.After(now) || d.ExpiresAt.After(in.Credential.ExpiresAt) {
				return ErrInvalid
			}
			for _, a := range d.Actions {
				if a != "apply" && a != "observe" {
					return ErrInvalid
				}
			}
		}
		state := "ready"
		next := []string{"start execution"}
		if required > 0 {
			state = "awaiting_approvals"
			next = []string{"collect environment approvals"}
		}
		x := Execution{ID: id(), EnvironmentID: in.EnvironmentID, PlanRevision: source, MergedRevision: mergedRevision, Credential: in.Credential, Budget: in.Budget, State: state, ActiveControllerID: in.ControllerID, InitiatedByID: actor, CreatedAt: now, Approvals: []ExecutionApproval{}, Steps: steps, Delegations: in.Delegations, Health: "unknown", NextActions: next, SafeControls: []string{"cancel"}, NonAuthority: []string{"execution grants no secret, approval, destructive, unrelated provider, repository, or deployment authority"}}
		x.Events = append(x.Events, ExecutionEvent{Sequence: 1, Kind: "created", ActorID: actor, Summary: "exact merged plan bound to environment authority", CreatedAt: now})
		p.Executions = append(p.Executions, x)
		return nil
	})
}

func (s *Store) mutateExecution(repo, pull, plan, execution string, fn func(*Execution) error) (Plan, error) {
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		for i := range p.Executions {
			if p.Executions[i].ID == execution {
				if err := fn(&p.Executions[i]); err != nil {
					return err
				}
				deriveExecution(&p.Executions[i], s.now().UTC())
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) ApproveExecution(repo, pull, plan, execution, actor string) (Plan, error) {
	if actor == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutateExecution(repo, pull, plan, execution, func(x *Execution) error {
		if x.State != "awaiting_approvals" {
			return ErrInvalid
		}
		for _, a := range x.Approvals {
			if a.ActorID == actor {
				return ErrInvalid
			}
		}
		x.Approvals = append(x.Approvals, ExecutionApproval{actor, s.now().UTC()})
		required, _ := s.environments.ExecutionEnvironment(repo, x.EnvironmentID)
		if len(x.Approvals) >= required {
			x.State = "ready"
		}
		x.event("approved", actor, "", "environment approval recorded", s.now().UTC())
		return nil
	})
}
func (s *Store) ControlExecution(repo, pull, plan, execution, actor, action, reason string) (Plan, error) {
	if actor == "" || reason == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutateExecution(repo, pull, plan, execution, func(x *Execution) error {
		now := s.now().UTC()
		switch action {
		case "start":
			if x.State != "ready" || !x.Credential.ExpiresAt.After(now) || x.Spent >= x.Budget.MaximumCost {
				return ErrInvalid
			}
			x.State = "running"
			x.StartedAt = &now
		case "pause":
			if x.State != "running" || !x.atSafetyPoint() {
				return ErrInvalid
			}
			x.State = "paused"
		case "resume":
			if x.State != "paused" || !x.Credential.ExpiresAt.After(now) || x.Spent >= x.Budget.MaximumCost {
				return ErrInvalid
			}
			x.State = "running"
		case "cancel":
			if (x.State != "ready" && x.State != "awaiting_approvals" && x.State != "running" && x.State != "paused") || !x.atSafetyPoint() {
				return ErrInvalid
			}
			x.State = "cancelled"
			x.CompletedAt = &now
		default:
			return ErrInvalid
		}
		x.ActiveControllerID = actor
		x.event(action, actor, "", reason, now)
		return nil
	})
}
func (s *Store) UpdateExecutionStep(repo, pull, plan, execution, step, actor string, in StepUpdate) (Plan, error) {
	if actor == "" || !map[string]bool{"running": true, "succeeded": true, "failed": true}[in.State] || !map[string]bool{"healthy": true, "degraded": true, "unhealthy": true, "unknown": true}[in.Health] || in.ProviderResponse == "" || in.NextAction == "" || in.Cost < 0 || secretShaped(in.ProviderResponse+in.Blocker+in.NextAction) {
		return Plan{}, ErrInvalid
	}
	return s.mutateExecution(repo, pull, plan, execution, func(x *Execution) error {
		if x.State != "running" || x.Credential.ExpiresAt.Before(s.now()) {
			return ErrInvalid
		}
		delegated := false
		if actor == x.ActiveControllerID {
			delegated = true
		} else {
			for _, d := range x.Delegations {
				if d.AgentID == actor && d.StepID == step && d.ExpiresAt.After(s.now()) {
					for _, action := range d.Actions {
						if action == "apply" {
							delegated = true
						}
					}
				}
			}
		}
		if !delegated {
			return ErrInvalid
		}
		for i := range x.Steps {
			st := &x.Steps[i]
			if st.ResourceID != step {
				continue
			}
			for j := 0; j < i; j++ {
				if x.Steps[j].State != "succeeded" {
					return ErrInvalid
				}
			}
			if st.Action == "destroy" && actor != x.ActiveControllerID {
				return ErrInvalid
			}
			delta := in.Cost - st.Cost
			if delta < 0 || x.Spent+delta > x.Budget.MaximumCost {
				return ErrInvalid
			}
			now := s.now().UTC()
			st.State, st.ProviderResponse, st.Health, st.Cost, st.Blocker, st.NextAction, st.SafetyPoint, st.ControllerID, st.UpdatedAt = in.State, in.ProviderResponse, in.Health, in.Cost, in.Blocker, in.NextAction, in.SafetyPoint, actor, &now
			x.Spent += delta
			x.CurrentStepID = step
			x.Health = in.Health
			if in.State == "failed" || in.Health == "unhealthy" {
				x.State = "paused"
			}
			all := true
			for _, q := range x.Steps {
				if q.State != "succeeded" {
					all = false
				}
			}
			if all {
				x.State = "succeeded"
				x.CompletedAt = &now
			}
			x.event("step_"+in.State, actor, step, in.NextAction, now)
			return nil
		}
		return ErrNotFound
	})
}
func (x *Execution) event(kind, actor, step, summary string, now time.Time) {
	x.Events = append(x.Events, ExecutionEvent{Sequence: int64(len(x.Events) + 1), Kind: kind, ActorID: actor, StepID: step, Summary: summary, CreatedAt: now})
}
func (x *Execution) atSafetyPoint() bool {
	if x.CurrentStepID == "" {
		return true
	}
	for _, s := range x.Steps {
		if s.ResourceID == x.CurrentStepID {
			return s.SafetyPoint
		}
	}
	return false
}
func deriveExecution(x *Execution, now time.Time) {
	x.Blockers = []string{}
	x.NextActions = []string{}
	x.SafeControls = []string{}
	if now.After(x.Credential.ExpiresAt) && (x.State == "running" || x.State == "paused" || x.State == "ready") {
		x.Blockers = append(x.Blockers, "credential_expired")
	}
	if x.Spent >= x.Budget.MaximumCost {
		x.Blockers = append(x.Blockers, "budget_exhausted")
	}
	for _, s := range x.Steps {
		if s.Blocker != "" {
			x.Blockers = append(x.Blockers, s.ResourceID+":"+s.Blocker)
		}
	}
	sort.Strings(x.Blockers)
	switch x.State {
	case "awaiting_approvals":
		x.NextActions = []string{"collect environment approvals"}
		x.SafeControls = []string{"cancel"}
	case "ready":
		x.NextActions = []string{"start execution"}
		x.SafeControls = []string{"start", "cancel"}
	case "running":
		x.NextActions = []string{"apply next dependency-ordered step"}
		if x.atSafetyPoint() {
			x.SafeControls = []string{"pause", "cancel"}
		}
	case "paused":
		x.NextActions = []string{"resolve blockers or resume"}
		x.SafeControls = []string{"resume", "cancel"}
	}
}
