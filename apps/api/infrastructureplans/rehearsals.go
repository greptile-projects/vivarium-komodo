package infrastructureplans

import (
	"sort"
	"strings"
	"time"
)

// Rehearsal is an isolated, non-authoritative execution contract for one exact plan.
type Rehearsal struct {
	ID                     string               `json:"id"`
	Title                  string               `json:"title"`
	Environment            RehearsalEnvironment `json:"environment"`
	Credential             CredentialBoundary   `json:"credential"`
	State                  StateBoundary        `json:"state"`
	Resources              []RehearsalResource  `json:"resources"`
	Checks                 []RehearsalCheck     `json:"checks"`
	MaximumDurationSeconds int64                `json:"maximum_duration_seconds"`
	MaximumCost            float64              `json:"maximum_cost"`
	Currency               string               `json:"currency"`
	CreatedByID            string               `json:"created_by_id"`
	CreatedAt              time.Time            `json:"created_at"`
	Attempts               []RehearsalAttempt   `json:"attempts"`
	Current                bool                 `json:"current"`
	Ready                  bool                 `json:"ready"`
	Blockers               []string             `json:"blockers"`
	UnsupportedResources   []string             `json:"unsupported_resources"`
	UntestableEffects      []string             `json:"untestable_effects"`
	NonAuthority           []string             `json:"non_authority"`
}

type RehearsalEnvironment struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	PolicyID        string   `json:"policy_id,omitempty"`
	PolicyRevision  string   `json:"policy_revision,omitempty"`
	Regions         []string `json:"regions"`
	NetworkBoundary string   `json:"network_boundary"`
}

type CredentialBoundary struct {
	Reference      string    `json:"reference"`
	Provider       string    `json:"provider"`
	Scope          []string  `json:"scope"`
	EnvironmentIDs []string  `json:"environment_ids"`
	ExpiresAt      time.Time `json:"expires_at"`
	SecretRetained bool      `json:"secret_retained"`
}

type StateBoundary struct {
	Kind           string `json:"kind"`
	Reference      string `json:"reference"`
	PrivacyMethod  string `json:"privacy_method,omitempty"`
	ProductionData bool   `json:"production_data"`
}

type RehearsalResource struct {
	ResourceID string `json:"resource_id"`
	Support    string `json:"support"`
	Reason     string `json:"reason,omitempty"`
}

type RehearsalCheck struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Command     string   `json:"command"`
	Expected    string   `json:"expected"`
	ResourceIDs []string `json:"resource_ids"`
	Destructive bool     `json:"destructive"`
}

type CheckResult struct {
	CheckID         string   `json:"check_id"`
	Status          string   `json:"status"`
	Summary         string   `json:"summary"`
	SanitizedLog    string   `json:"sanitized_log,omitempty"`
	ArtifactDigests []string `json:"artifact_digests,omitempty"`
	DurationMillis  int64    `json:"duration_millis"`
}

type ResourceGraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type AgentAction struct {
	AgentID    string `json:"agent_id"`
	Action     string `json:"action"`
	ResourceID string `json:"resource_id,omitempty"`
	Summary    string `json:"summary"`
}

type RehearsalAttempt struct {
	ID                  string              `json:"id"`
	RunnerAttestation   string              `json:"runner_attestation"`
	StartedAt           time.Time           `json:"started_at"`
	CompletedAt         time.Time           `json:"completed_at"`
	Results             []CheckResult       `json:"results"`
	ResourceGraph       []ResourceGraphEdge `json:"resource_graph"`
	AgentActions        []AgentAction       `json:"agent_actions"`
	EstimatedCost       float64             `json:"estimated_cost"`
	TeardownStatus      string              `json:"teardown_status"`
	TeardownAttestation string              `json:"teardown_attestation"`
	RecoveryStatus      string              `json:"recovery_status"`
	RecoveryAttestation string              `json:"recovery_attestation"`
	ActorID             string              `json:"actor_id"`
	CreatedAt           time.Time           `json:"created_at"`
	Passed              bool                `json:"passed"`
}

type RehearsalInput struct {
	Title                  string               `json:"title"`
	Environment            RehearsalEnvironment `json:"environment"`
	Credential             CredentialBoundary   `json:"credential"`
	State                  StateBoundary        `json:"state"`
	Resources              []RehearsalResource  `json:"resources"`
	Checks                 []RehearsalCheck     `json:"checks"`
	MaximumDurationSeconds int64                `json:"maximum_duration_seconds"`
	MaximumCost            float64              `json:"maximum_cost"`
	Currency               string               `json:"currency"`
}

type AttemptInput struct {
	RunnerAttestation   string              `json:"runner_attestation"`
	StartedAt           time.Time           `json:"started_at"`
	CompletedAt         time.Time           `json:"completed_at"`
	Results             []CheckResult       `json:"results"`
	ResourceGraph       []ResourceGraphEdge `json:"resource_graph"`
	AgentActions        []AgentAction       `json:"agent_actions"`
	EstimatedCost       float64             `json:"estimated_cost"`
	TeardownStatus      string              `json:"teardown_status"`
	TeardownAttestation string              `json:"teardown_attestation"`
	RecoveryStatus      string              `json:"recovery_status"`
	RecoveryAttestation string              `json:"recovery_attestation"`
}

var checkKinds = map[string]bool{"provisioning": true, "connectivity": true, "access_boundary": true, "policy": true, "service_journey": true, "failure_behavior": true, "cost_estimate": true, "teardown": true, "recovery": true}

func validRehearsal(in RehearsalInput, p Plan, now time.Time) bool {
	if p.Stale || strings.TrimSpace(in.Title) == "" || in.MaximumDurationSeconds < 1 || in.MaximumCost < 0 || strings.TrimSpace(in.Currency) == "" || len(in.Resources) == 0 || len(in.Checks) == 0 {
		return false
	}
	if in.Environment.ID == "" || !map[string]bool{"isolated": true, "policy_approved_ephemeral": true}[in.Environment.Kind] || in.Environment.NetworkBoundary == "" {
		return false
	}
	if in.Environment.Kind == "policy_approved_ephemeral" && (in.Environment.PolicyID == "" || in.Environment.PolicyRevision == "") {
		return false
	}
	if in.Credential.Reference == "" || in.Credential.Provider == "" || len(in.Credential.Scope) == 0 || len(in.Credential.EnvironmentIDs) == 0 || !in.Credential.ExpiresAt.After(now) || in.Credential.SecretRetained {
		return false
	}
	if secretShaped(in.Credential.Reference + in.Credential.Provider + strings.Join(in.Credential.Scope, "") + strings.Join(in.Credential.EnvironmentIDs, "")) {
		return false
	}
	foundEnv := false
	for _, e := range in.Credential.EnvironmentIDs {
		if e == in.Environment.ID {
			foundEnv = true
		}
	}
	if !foundEnv {
		return false
	}
	if !map[string]bool{"synthetic": true, "permitted_representative": true}[in.State.Kind] || in.State.Reference == "" || in.State.ProductionData || (in.State.Kind == "permitted_representative" && in.State.PrivacyMethod == "") {
		return false
	}
	known := map[string]bool{}
	actions := map[string]string{}
	for _, c := range p.Input.Changes {
		known[c.ResourceID] = true
		actions[c.ResourceID] = c.Action
	}
	classified := map[string]bool{}
	for _, r := range in.Resources {
		if !known[r.ResourceID] || classified[r.ResourceID] || !map[string]bool{"supported": true, "unsupported": true, "untestable_destructive": true}[r.Support] || (r.Support != "supported" && r.Reason == "") {
			return false
		}
		classified[r.ResourceID] = true
		if actions[r.ResourceID] == "destroy" && r.Support == "supported" {
			return false
		}
	}
	if len(classified) != len(known) {
		return false
	}
	seen := map[string]bool{}
	kinds := map[string]bool{}
	for _, c := range in.Checks {
		if c.ID == "" || seen[c.ID] || !checkKinds[c.Kind] || c.Command == "" || c.Expected == "" || len(c.ResourceIDs) == 0 || secretShaped(c.Command+c.Expected) {
			return false
		}
		seen[c.ID] = true
		kinds[c.Kind] = true
		for _, r := range c.ResourceIDs {
			if !known[r] {
				return false
			}
			if actions[r] == "destroy" && c.Destructive {
				return false
			}
		}
	}
	for k := range checkKinds {
		if !kinds[k] {
			return false
		}
	}
	return true
}

func deriveRehearsal(r *Rehearsal, stale bool) {
	r.Current = !stale
	r.Blockers = nil
	r.UnsupportedResources = nil
	r.UntestableEffects = nil
	for _, x := range r.Resources {
		if x.Support == "unsupported" {
			r.UnsupportedResources = append(r.UnsupportedResources, x.ResourceID+": "+x.Reason)
		}
		if x.Support == "untestable_destructive" {
			r.UntestableEffects = append(r.UntestableEffects, x.ResourceID+": "+x.Reason)
		}
	}
	if stale {
		r.Blockers = append(r.Blockers, "plan_stale")
	}
	if len(r.Attempts) == 0 {
		r.Blockers = append(r.Blockers, "not_run")
	} else if !r.Attempts[len(r.Attempts)-1].Passed {
		r.Blockers = append(r.Blockers, "latest_attempt_failed")
	}
	if len(r.UnsupportedResources) > 0 {
		r.Blockers = append(r.Blockers, "unsupported_resources")
	}
	if len(r.UntestableEffects) > 0 {
		r.Blockers = append(r.Blockers, "untestable_destructive_effects")
	}
	sort.Strings(r.Blockers)
	r.Ready = len(r.Blockers) == 0
	r.NonAuthority = []string{"rehearsal evidence grants no provider, credential, deployment, environment, approval, or production authority"}
}

func (s *Store) CreateRehearsal(repo, pull, plan, actor string, in RehearsalInput) (Plan, error) {
	if actor == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		d := s.derive(*p)
		if !validRehearsal(in, d, s.now().UTC()) {
			return ErrInvalid
		}
		r := Rehearsal{ID: id(), Title: in.Title, Environment: in.Environment, Credential: in.Credential, State: in.State, Resources: in.Resources, Checks: in.Checks, MaximumDurationSeconds: in.MaximumDurationSeconds, MaximumCost: in.MaximumCost, Currency: in.Currency, CreatedByID: actor, CreatedAt: s.now().UTC()}
		p.Rehearsals = append(p.Rehearsals, r)
		return nil
	})
}

func validAttempt(in AttemptInput, r Rehearsal) bool {
	if in.RunnerAttestation == "" || in.StartedAt.IsZero() || !in.CompletedAt.After(in.StartedAt) || in.CompletedAt.Sub(in.StartedAt) > time.Duration(r.MaximumDurationSeconds)*time.Second || in.EstimatedCost < 0 || in.EstimatedCost > r.MaximumCost || len(in.Results) != len(r.Checks) || !map[string]bool{"passed": true, "failed": true}[in.TeardownStatus] || in.TeardownAttestation == "" || !map[string]bool{"passed": true, "failed": true, "not_applicable": true}[in.RecoveryStatus] || in.RecoveryAttestation == "" {
		return false
	}
	checks := map[string]bool{}
	for _, c := range r.Checks {
		checks[c.ID] = true
	}
	seen := map[string]bool{}
	for _, x := range in.Results {
		if !checks[x.CheckID] || seen[x.CheckID] || !map[string]bool{"passed": true, "failed": true, "unsupported": true}[x.Status] || x.Summary == "" || x.DurationMillis < 0 || secretShaped(x.Summary+x.SanitizedLog+strings.Join(x.ArtifactDigests, "")) {
			return false
		}
		for _, digest := range x.ArtifactDigests {
			if !strings.HasPrefix(digest, "sha256:") || len(strings.TrimPrefix(digest, "sha256:")) == 0 {
				return false
			}
		}
		seen[x.CheckID] = true
	}
	resources := map[string]bool{}
	for _, x := range r.Resources {
		resources[x.ResourceID] = true
	}
	for _, edge := range in.ResourceGraph {
		if !resources[edge.From] || edge.To == "" || secretShaped(edge.To) {
			return false
		}
	}
	for _, a := range in.AgentActions {
		if a.AgentID == "" || a.Action == "" || a.Summary == "" || (a.ResourceID != "" && !resources[a.ResourceID]) || secretShaped(a.AgentID+a.Action+a.Summary) {
			return false
		}
	}
	return !secretShaped(in.RunnerAttestation + in.TeardownAttestation + in.RecoveryAttestation)
}

func (s *Store) RecordRehearsalAttempt(repo, pull, plan, rehearsal, actor string, in AttemptInput) (Plan, error) {
	if actor == "" {
		return Plan{}, ErrInvalid
	}
	return s.mutate(repo, pull, plan, func(p *Plan) error {
		d := s.derive(*p)
		if d.Stale {
			return ErrInvalid
		}
		for i := range p.Rehearsals {
			r := &p.Rehearsals[i]
			if r.ID != rehearsal {
				continue
			}
			if !validAttempt(in, *r) {
				return ErrInvalid
			}
			pass := in.TeardownStatus == "passed" && (in.RecoveryStatus == "passed" || in.RecoveryStatus == "not_applicable")
			for _, x := range in.Results {
				pass = pass && x.Status == "passed"
			}
			r.Attempts = append(r.Attempts, RehearsalAttempt{ID: id(), RunnerAttestation: in.RunnerAttestation, StartedAt: in.StartedAt, CompletedAt: in.CompletedAt, Results: in.Results, ResourceGraph: in.ResourceGraph, AgentActions: in.AgentActions, EstimatedCost: in.EstimatedCost, TeardownStatus: in.TeardownStatus, TeardownAttestation: in.TeardownAttestation, RecoveryStatus: in.RecoveryStatus, RecoveryAttestation: in.RecoveryAttestation, ActorID: actor, CreatedAt: s.now().UTC(), Passed: pass})
			return nil
		}
		return ErrNotFound
	})
}
