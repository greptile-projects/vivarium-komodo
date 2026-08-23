package workflowdefinitions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var errExecutionUnchanged = errors.New("execution unchanged")

type TriggeringEvent struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Revision   string    `json:"revision"`
	OccurredAt time.Time `json:"occurred_at"`
}
type ExecutionActor struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type ResourceRevision struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
}
type PolicyDecision struct {
	Repository   string `json:"repository"`
	Organization string `json:"organization"`
	Agent        string `json:"agent"`
	Embargo      string `json:"embargo"`
	Environment  string `json:"environment"`
	Approval     string `json:"approval"`
}
type InvokeInput struct {
	IdempotencyKey     string             `json:"idempotency_key"`
	WorkflowVersion    int64              `json:"workflow_version"`
	Event              TriggeringEvent    `json:"event"`
	Actor              ExecutionActor     `json:"actor"`
	Inputs             map[string]any     `json:"inputs"`
	PermittedResources []ResourceRevision `json:"permitted_resources"`
	Policy             PolicyDecision     `json:"policy"`
}
type StepCredential struct {
	Reference      string           `json:"reference"`
	Subject        string           `json:"subject"`
	Capabilities   []string         `json:"capabilities"`
	Resource       ResourceRevision `json:"resource"`
	IssuedAt       time.Time        `json:"issued_at"`
	ExpiresAt      time.Time        `json:"expires_at"`
	SecretRetained bool             `json:"secret_retained"`
	RevokedAt      *time.Time       `json:"revoked_at,omitempty"`
}
type OutputValue struct {
	Value      any    `json:"value,omitempty"`
	Accessible bool   `json:"accessible"`
	Secret     bool   `json:"secret"`
	Digest     string `json:"digest,omitempty"`
}
type StepAttempt struct {
	Number              int                    `json:"number"`
	IdempotencyKey      string                 `json:"idempotency_key"`
	State               string                 `json:"state"`
	CredentialReference string                 `json:"credential_reference"`
	Outputs             map[string]OutputValue `json:"outputs,omitempty"`
	Cost                float64                `json:"cost"`
	Failure             string                 `json:"failure,omitempty"`
	Logs                []ExecutionLog         `json:"logs"`
	Artifacts           []ExecutionArtifact    `json:"artifacts"`
	AgentSession        *AgentSession          `json:"agent_session,omitempty"`
	StartedAt           time.Time              `json:"started_at"`
	CompletedAt         *time.Time             `json:"completed_at,omitempty"`
}
type ExecutionLog struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
type ExecutionArtifact struct {
	Name       string `json:"name"`
	Digest     string `json:"digest"`
	MediaType  string `json:"media_type"`
	Accessible bool   `json:"accessible"`
	Redacted   bool   `json:"redacted"`
}
type AgentSession struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	State    string `json:"state"`
}
type WaitingApproval struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	OwnerIDs []string `json:"owner_ids"`
}
type ExecutionStep struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	State               string                 `json:"state"`
	Needs               []string               `json:"needs"`
	Optional            bool                   `json:"optional"`
	InvocationKind      string                 `json:"invocation_kind"`
	Credential          *StepCredential        `json:"credential,omitempty"`
	Attempts            []StepAttempt          `json:"attempts"`
	AvailableInputs     map[string]OutputValue `json:"available_inputs,omitempty"`
	Blocker             string                 `json:"blocker,omitempty"`
	WaitingApproval     *WaitingApproval       `json:"waiting_approval,omitempty"`
	RequestedInput      string                 `json:"requested_input,omitempty"`
	ProvidedInputs      map[string]any         `json:"provided_inputs,omitempty"`
	ManualActorID       string                 `json:"manual_actor_id,omitempty"`
	PredictedNextAction string                 `json:"predicted_next_action"`
}
type ExecutionEvent struct {
	Sequence  int64     `json:"sequence"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	StepID    string    `json:"step_id,omitempty"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
type Execution struct {
	ID                         string             `json:"id"`
	RepositoryID               string             `json:"repository_id"`
	WorkflowID                 string             `json:"workflow_id"`
	Revision                   int64              `json:"revision"`
	WorkflowVersion            int64              `json:"workflow_version"`
	WorkflowRepositoryRevision string             `json:"workflow_repository_revision"`
	DefinitionPath             string             `json:"definition_path"`
	IdempotencyKey             string             `json:"idempotency_key"`
	Event                      TriggeringEvent    `json:"event"`
	Actor                      ExecutionActor     `json:"actor"`
	Inputs                     map[string]any     `json:"inputs"`
	PermittedResources         []ResourceRevision `json:"permitted_resources"`
	Policy                     PolicyDecision     `json:"policy"`
	State                      string             `json:"state"`
	Steps                      []ExecutionStep    `json:"steps"`
	Cost                       float64            `json:"cost"`
	MaximumCost                float64            `json:"maximum_cost"`
	Currency                   string             `json:"currency"`
	Blockers                   []string           `json:"blockers"`
	Events                     []ExecutionEvent   `json:"events"`
	CreatedAt                  time.Time          `json:"created_at"`
	UpdatedAt                  time.Time          `json:"updated_at"`
	CompletedAt                *time.Time         `json:"completed_at,omitempty"`
	PredictedNextActions       []string           `json:"predicted_next_actions"`
}
type ExecutionCatalog struct {
	Items []Execution `json:"items"`
}
type DispatchInput struct {
	ExpectedRevision    int64     `json:"expected_revision"`
	IdempotencyKey      string    `json:"idempotency_key"`
	CredentialExpiresAt time.Time `json:"credential_expires_at"`
}
type ResultInput struct {
	ExpectedRevision    int64                  `json:"expected_revision"`
	IdempotencyKey      string                 `json:"idempotency_key"`
	CredentialReference string                 `json:"credential_reference"`
	State               string                 `json:"state"`
	Outputs             map[string]OutputValue `json:"outputs"`
	Cost                float64                `json:"cost"`
	Failure             string                 `json:"failure"`
	Logs                []ExecutionLog         `json:"logs"`
	Artifacts           []ExecutionArtifact    `json:"artifacts"`
	AgentSession        *AgentSession          `json:"agent_session,omitempty"`
	WaitingApproval     *WaitingApproval       `json:"waiting_approval,omitempty"`
	RequestedInput      string                 `json:"requested_input,omitempty"`
}
type ControlInput struct {
	ExpectedRevision int64           `json:"expected_revision"`
	Action           string          `json:"action"`
	StepID           string          `json:"step_id"`
	Reason           string          `json:"reason"`
	Policy           *PolicyDecision `json:"policy,omitempty"`
	Inputs           map[string]any  `json:"inputs,omitempty"`
}

func (s *Store) executionDir(repo, workflow string) string {
	return filepath.Join(s.root, repo, workflow+".executions")
}
func (s *Store) executionPath(repo, workflow, execution string) string {
	return filepath.Join(s.executionDir(repo, workflow), execution+".json")
}
func executionTerminal(state string) bool {
	return state == "completed" || state == "failed" || state == "cancelled"
}
func executionConsumesConcurrency(x Execution) bool {
	return !executionTerminal(x.State) && !(x.State == "blocked" && len(x.Blockers) == 1 && x.Blockers[0] == "policy_denied")
}
func policyAllowed(p PolicyDecision) bool {
	return p.Repository == "allowed" && p.Organization == "allowed" && p.Agent == "allowed" && p.Embargo == "allowed" && p.Environment == "allowed" && p.Approval == "allowed"
}
func safeValue(v any) bool {
	b, e := json.Marshal(v)
	if e != nil {
		return false
	}
	q := strings.ToLower(string(b))
	for _, x := range []string{"password", "private_key", "access_token", "secret=", "authorization:", "terminal_input"} {
		if strings.Contains(q, x) {
			return false
		}
	}
	return len(b) <= 64<<10
}
func safeText(v string) bool { return strings.TrimSpace(v) != "" && safeValue(v) }

func refreshExecution(x *Execution) {
	x.PredictedNextActions = nil
	for i := range x.Steps {
		s := &x.Steps[i]
		s.PredictedNextAction = "No further action; this step is retained as " + s.State + "."
		switch s.State {
		case "pending":
			s.PredictedNextAction = "Wait for dependencies: " + strings.Join(s.Needs, ", ") + "."
		case "ready":
			s.PredictedNextAction = "The scheduler can dispatch this step."
		case "running":
			s.PredictedNextAction = "Wait for the current attempt to report a result."
		case "paused":
			s.PredictedNextAction = "An authorized collaborator can resume the execution."
		case "retry_ready":
			s.PredictedNextAction = "An authorized collaborator can retry this step."
		case "waiting_input":
			s.PredictedNextAction = "Provide the requested non-secret input: " + s.RequestedInput
		case "waiting_approval":
			s.PredictedNextAction = "A repository writer can approve or cancel this request."
		case "manual_ready":
			s.PredictedNextAction = "An authorized collaborator can take over this manual step."
		case "failed":
			s.PredictedNextAction = "Inspect retained attempts and start a new workflow execution if correction is needed."
		}
		if !executionTerminal(x.State) && s.State != "succeeded" && s.State != "skipped" && s.State != "cancelled" {
			x.PredictedNextActions = append(x.PredictedNextActions, s.ID+": "+s.PredictedNextAction)
		}
	}
}
func valueMatches(kind string, value any) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		if ok {
			return true
		}
		_, ok = value.(int)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}
func resourceKey(k, r, v string) string { return k + ":" + r + "@" + v }
func stepIndex(x Execution, id string) int {
	for i := range x.Steps {
		if x.Steps[i].ID == id {
			return i
		}
	}
	return -1
}
func (x *Execution) event(kind, actor, step, detail string, now time.Time) {
	x.Events = append(x.Events, ExecutionEvent{int64(len(x.Events) + 1), kind, actor, step, detail, now})
}
func (s *Store) saveExecution(x Execution) error {
	if e := os.MkdirAll(s.executionDir(x.RepositoryID, x.WorkflowID), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.executionPath(x.RepositoryID, x.WorkflowID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) readExecution(repo, workflow, execution string) (Execution, error) {
	var x Execution
	b, e := os.ReadFile(s.executionPath(repo, workflow, execution))
	if e != nil {
		return x, ErrNotFound
	}
	if json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.WorkflowID != workflow || x.ID != execution {
		return Execution{}, ErrNotFound
	}
	refreshExecution(&x)
	return x, nil
}
func (s *Store) listExecutions(repo, workflow string) ([]Execution, error) {
	es, e := os.ReadDir(s.executionDir(repo, workflow))
	if os.IsNotExist(e) {
		return []Execution{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Execution{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.readExecution(repo, workflow, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) Invoke(repo, workflow, requestActor string, in InvokeInput) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	w, e := s.read(repo, workflow)
	if e != nil {
		return Execution{}, e
	}
	if w.State != "active" || w.Activation == nil || in.WorkflowVersion != w.CurrentVersion || in.Actor.ID == "" || in.Actor.ID != requestActor || (in.Actor.Kind != "human" && in.Actor.Kind != "agent" && in.Actor.Kind != "system") || in.IdempotencyKey == "" || in.Event.ID == "" || in.Event.Revision == "" || in.Event.OccurredAt.IsZero() || in.Event.OccurredAt.After(now) {
		return Execution{}, ErrInvalid
	}
	v := w.Versions[len(w.Versions)-1]
	matched := false
	for _, t := range v.Triggers {
		if t.Type == in.Event.Type && t.Event == in.Event.Name {
			matched = true
		}
	}
	if !matched {
		return Execution{}, ErrInvalid
	}
	declaredInputs := map[string]bool{}
	for _, f := range v.Inputs {
		declaredInputs[f.Name] = true
		q, ok := in.Inputs[f.Name]
		if f.Required && !ok {
			return Execution{}, ErrInvalid
		}
		if ok && (!safeValue(q) || !valueMatches(f.Type, q)) {
			return Execution{}, ErrInvalid
		}
	}
	for name := range in.Inputs {
		if !declaredInputs[name] {
			return Execution{}, ErrInvalid
		}
	}
	want := map[string]bool{}
	for _, st := range v.Steps {
		want[resourceKey(st.Invocation.Kind, st.Invocation.Reference, st.Invocation.Revision)] = true
	}
	got := map[string]bool{}
	for _, r := range in.PermittedResources {
		got[resourceKey(r.Kind, r.Reference, r.Revision)] = true
	}
	if len(want) != len(got) {
		return Execution{}, ErrInvalid
	}
	for k := range want {
		if !got[k] {
			return Execution{}, ErrInvalid
		}
	}
	xs, e := s.listExecutions(repo, workflow)
	if e != nil {
		return Execution{}, e
	}
	active := 0
	recent := 0
	for _, x := range xs {
		if x.IdempotencyKey == in.IdempotencyKey {
			if x.Event.ID == in.Event.ID && x.WorkflowVersion == in.WorkflowVersion {
				return x, nil
			}
			return Execution{}, ErrConflict
		}
		if executionConsumesConcurrency(x) {
			active++
		}
		if v.RateLimit.WindowSeconds > 0 && x.CreatedAt.After(now.Add(-time.Duration(v.RateLimit.WindowSeconds)*time.Second)) {
			recent++
		}
	}
	limit := v.MaximumConcurrency
	if limit == 0 {
		limit = 1
	}
	if active >= limit {
		return Execution{}, ErrBlocked
	}
	if v.RateLimit.MaximumInvocations > 0 && recent >= v.RateLimit.MaximumInvocations {
		return Execution{}, ErrBlocked
	}
	steps := make([]ExecutionStep, 0, len(v.Steps))
	for _, st := range v.Steps {
		state := "pending"
		if len(st.Needs) == 0 {
			state = "ready"
		}
		if st.Invocation.Kind == "manual" && state == "ready" {
			state = "manual_ready"
		}
		steps = append(steps, ExecutionStep{ID: st.ID, Name: st.Name, State: state, Needs: append([]string{}, st.Needs...), Optional: st.Optional, InvocationKind: st.Invocation.Kind, Attempts: []StepAttempt{}})
	}
	state := "running"
	blockers := []string{}
	if !policyAllowed(in.Policy) {
		state = "blocked"
		blockers = append(blockers, "policy_denied")
	}
	x := Execution{ID: newID(), RepositoryID: repo, WorkflowID: workflow, Revision: 1, WorkflowVersion: v.Number, WorkflowRepositoryRevision: v.RepositoryRevision, DefinitionPath: v.DefinitionPath, IdempotencyKey: in.IdempotencyKey, Event: in.Event, Actor: in.Actor, Inputs: in.Inputs, PermittedResources: in.PermittedResources, Policy: in.Policy, State: state, Steps: steps, MaximumCost: v.MaximumCost, Currency: v.Currency, Blockers: blockers, CreatedAt: now, UpdatedAt: now}
	kind := "invoked"
	if state == "blocked" {
		kind = "policy_blocked"
	}
	x.event(kind, requestActor, "", "exact workflow, event, actor, policy, and resource revisions bound", now)
	refreshExecution(&x)
	return x, s.saveExecution(x)
}
func (s *Store) GetExecution(repo, workflow, execution string) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readExecution(repo, workflow, execution)
}
func (s *Store) Executions(repo, workflow string) (ExecutionCatalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.listExecutions(repo, workflow)
	return ExecutionCatalog{x}, e
}
func (s *Store) mutateExecution(repo, workflow, execution string, expected int64, fn func(*Execution, time.Time) error) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.readExecution(repo, workflow, execution)
	if e != nil {
		return x, e
	}
	if x.Revision != expected {
		return x, ErrConflict
	}
	now := s.now().UTC()
	if e = fn(&x, now); e != nil {
		if errors.Is(e, errExecutionUnchanged) {
			return x, nil
		}
		return x, e
	}
	x.Revision++
	x.UpdatedAt = now
	refreshExecution(&x)
	return x, s.saveExecution(x)
}

func (s *Store) Dispatch(repo, workflow, execution, actor, step string, in DispatchInput) (Execution, error) {
	return s.mutateExecution(repo, workflow, execution, in.ExpectedRevision, func(x *Execution, now time.Time) error {
		if x.State != "running" || in.IdempotencyKey == "" || !in.CredentialExpiresAt.After(now) {
			return ErrInvalid
		}
		i := stepIndex(*x, step)
		if i < 0 {
			return ErrNotFound
		}
		q := &x.Steps[i]
		for _, a := range q.Attempts {
			if a.IdempotencyKey == in.IdempotencyKey {
				return errExecutionUnchanged
			}
		}
		if q.State != "ready" {
			return ErrInvalid
		}
		w, _ := s.read(repo, workflow)
		st := w.Versions[x.WorkflowVersion-1].Steps[0]
		for _, candidate := range w.Versions[x.WorkflowVersion-1].Steps {
			if candidate.ID == step {
				st = candidate
			}
		}
		maximumExpiry := now.Add(time.Duration(st.TimeoutSeconds) * time.Second)
		if maximumExpiry.After(now.Add(15 * time.Minute)) {
			maximumExpiry = now.Add(15 * time.Minute)
		}
		if in.CredentialExpiresAt.After(maximumExpiry) {
			return ErrInvalid
		}
		r := ResourceRevision{st.Invocation.Kind, st.Invocation.Reference, st.Invocation.Revision}
		cred := StepCredential{Reference: "wfstep_" + newID(), Subject: fmt.Sprintf("workflow:%s/execution:%s/step:%s", workflow, x.ID, step), Capabilities: append([]string{}, st.Invocation.Capabilities...), Resource: r, IssuedAt: now, ExpiresAt: in.CredentialExpiresAt, SecretRetained: false}
		q.Credential = &cred
		q.State = "running"
		q.Attempts = append(q.Attempts, StepAttempt{Number: len(q.Attempts) + 1, IdempotencyKey: in.IdempotencyKey, State: "running", CredentialReference: cred.Reference, StartedAt: now})
		x.event("step_dispatched", actor, step, "scoped short-lived credential reference issued", now)
		return nil
	})
}
func (s *Store) RecordResult(repo, workflow, execution, actor, step string, in ResultInput) (Execution, error) {
	return s.mutateExecution(repo, workflow, execution, in.ExpectedRevision, func(x *Execution, now time.Time) error {
		if x.State != "running" || in.IdempotencyKey == "" || in.Cost < 0 {
			return ErrInvalid
		}
		i := stepIndex(*x, step)
		if i < 0 {
			return ErrNotFound
		}
		q := &x.Steps[i]
		manual := q.InvocationKind == "manual" && q.State == "running" && q.ManualActorID == actor
		if !manual && (q.State != "running" || q.Credential == nil || q.Credential.Reference != in.CredentialReference || !q.Credential.ExpiresAt.After(now)) {
			return ErrInvalid
		}
		a := &q.Attempts[len(q.Attempts)-1]
		if a.IdempotencyKey != in.IdempotencyKey {
			return ErrConflict
		}
		if a.State != "running" {
			return errExecutionUnchanged
		}
		for _, log := range in.Logs {
			if (log.Level != "debug" && log.Level != "info" && log.Level != "warning" && log.Level != "error") || !safeText(log.Message) {
				return ErrInvalid
			}
			log.CreatedAt = now
			a.Logs = append(a.Logs, log)
		}
		for _, artifact := range in.Artifacts {
			if artifact.Name == "" || artifact.Digest == "" || artifact.MediaType == "" || !safeText(artifact.Name) || !safeText(artifact.Digest) {
				return ErrInvalid
			}
			if !artifact.Accessible {
				artifact.Name, artifact.MediaType, artifact.Redacted = "restricted artifact", "application/octet-stream", true
			}
			a.Artifacts = append(a.Artifacts, artifact)
		}
		if in.AgentSession != nil {
			if !identifier(in.AgentSession.ID) || in.AgentSession.Revision == "" || !safeText(in.AgentSession.Revision) || (in.AgentSession.State != "running" && in.AgentSession.State != "completed" && in.AgentSession.State != "failed") {
				return ErrInvalid
			}
			a.AgentSession = in.AgentSession
		}
		w, _ := s.read(repo, workflow)
		var def Step
		for _, candidate := range w.Versions[x.WorkflowVersion-1].Steps {
			if candidate.ID == step {
				def = candidate
			}
		}
		if x.Cost+in.Cost > x.MaximumCost || in.Cost > def.MaximumCost {
			a.State = "blocked"
			a.Failure = "budget_exceeded"
			a.CompletedAt = &now
			q.State = "blocked"
			q.Blocker = "budget_exceeded"
			if q.Credential != nil {
				q.Credential.RevokedAt = &now
			}
			x.State = "blocked"
			x.Blockers = []string{"budget_exceeded"}
			x.event("budget_blocked", actor, step, "cost exceeded declared boundary", now)
			return nil
		}
		a.Cost = in.Cost
		a.CompletedAt = &now
		x.Cost += in.Cost
		switch in.State {
		case "succeeded":
			allowed := map[string]bool{}
			for _, f := range def.Outputs {
				allowed[f.Name] = true
			}
			clean := map[string]OutputValue{}
			fieldTypes := map[string]string{}
			for _, f := range def.Outputs {
				fieldTypes[f.Name] = f.Type
			}
			for name, value := range in.Outputs {
				if !allowed[name] || value.Secret || !value.Accessible || !safeValue(value.Value) || !valueMatches(fieldTypes[name], value.Value) {
					continue
				}
				clean[name] = value
			}
			for _, f := range def.Outputs {
				if f.Required {
					if _, ok := clean[f.Name]; !ok {
						return ErrInvalid
					}
				}
			}
			a.State = "succeeded"
			a.Outputs = clean
			q.State = "succeeded"
			if q.Credential != nil {
				q.Credential.RevokedAt = &now
			}
			advanceExecution(x, now)
			x.event("step_succeeded", actor, step, "declared accessible outputs retained", now)
		case "waiting_input":
			if !safeText(in.RequestedInput) {
				return ErrInvalid
			}
			a.State, a.CompletedAt, q.State, q.RequestedInput = "waiting_input", &now, "waiting_input", in.RequestedInput
			if q.Credential != nil {
				q.Credential.RevokedAt = &now
			}
			x.State = "waiting"
			x.event("input_requested", actor, step, in.RequestedInput, now)
		case "waiting_approval":
			if in.WaitingApproval == nil || !identifier(in.WaitingApproval.ID) || !safeText(in.WaitingApproval.Summary) || len(in.WaitingApproval.OwnerIDs) == 0 {
				return ErrInvalid
			}
			a.State, a.CompletedAt, q.State, q.WaitingApproval = "waiting_approval", &now, "waiting_approval", in.WaitingApproval
			if q.Credential != nil {
				q.Credential.RevokedAt = &now
			}
			x.State = "waiting"
			x.event("approval_requested", actor, step, in.WaitingApproval.Summary, now)
		case "failed", "interrupted":
			a.State = in.State
			a.Failure = in.Failure
			q.Credential.RevokedAt = &now
			if len(q.Attempts) < def.Retry.MaximumAttempts {
				q.State = "retry_ready"
				q.Blocker = in.State
			} else {
				q.State = "failed"
				x.State = "failed"
				x.CompletedAt = &now
			}
			x.event("step_"+in.State, actor, step, in.Failure, now)
		default:
			return ErrInvalid
		}
		return nil
	})
}

func advanceExecution(x *Execution, now time.Time) {
	for j := range x.Steps {
		if x.Steps[j].State != "pending" {
			continue
		}
		ready, available := true, map[string]OutputValue{}
		for _, need := range x.Steps[j].Needs {
			k := stepIndex(*x, need)
			if k < 0 || (x.Steps[k].State != "succeeded" && x.Steps[k].State != "skipped") {
				ready = false
				break
			}
			if x.Steps[k].State == "succeeded" && len(x.Steps[k].Attempts) > 0 {
				for n, v := range x.Steps[k].Attempts[len(x.Steps[k].Attempts)-1].Outputs {
					available[need+"."+n] = v
				}
			}
		}
		if ready {
			x.Steps[j].State, x.Steps[j].AvailableInputs = "ready", available
			if x.Steps[j].InvocationKind == "manual" {
				x.Steps[j].State = "manual_ready"
			}
		}
	}
	done := true
	for _, st := range x.Steps {
		if st.State != "succeeded" && st.State != "skipped" {
			done = false
		}
	}
	if done {
		x.State, x.CompletedAt = "completed", &now
	}
}
func (s *Store) Control(repo, workflow, execution, actor string, in ControlInput) (Execution, error) {
	return s.mutateExecution(repo, workflow, execution, in.ExpectedRevision, func(x *Execution, now time.Time) error {
		if in.Reason == "" {
			return ErrInvalid
		}
		switch in.Action {
		case "pause":
			if x.State != "running" && x.State != "waiting" {
				return ErrInvalid
			}
			x.State = "paused"
			for i := range x.Steps {
				if x.Steps[i].State == "running" {
					x.Steps[i].State = "paused"
					if x.Steps[i].Credential != nil && x.Steps[i].Credential.RevokedAt == nil {
						x.Steps[i].Credential.RevokedAt = &now
					}
					if len(x.Steps[i].Attempts) > 0 {
						a := &x.Steps[i].Attempts[len(x.Steps[i].Attempts)-1]
						a.State, a.Failure, a.CompletedAt = "interrupted", "collaborator_pause", &now
					}
				}
			}
		case "cancel":
			if executionTerminal(x.State) {
				return ErrInvalid
			}
			x.State = "cancelled"
			x.CompletedAt = &now
			for i := range x.Steps {
				if x.Steps[i].Credential != nil && x.Steps[i].Credential.RevokedAt == nil {
					x.Steps[i].Credential.RevokedAt = &now
				}
				if x.Steps[i].State == "pending" || x.Steps[i].State == "ready" || x.Steps[i].State == "running" {
					x.Steps[i].State = "cancelled"
				}
			}
		case "retry":
			i := stepIndex(*x, in.StepID)
			if i < 0 || x.Steps[i].State != "retry_ready" {
				return ErrInvalid
			}
			x.Steps[i].State = "ready"
			x.Steps[i].Blocker = ""
		case "resume":
			if x.State == "paused" {
				x.State = "running"
				for i := range x.Steps {
					if x.Steps[i].State == "paused" {
						x.Steps[i].State = "ready"
					}
				}
				for i := range x.Steps {
					if x.Steps[i].State == "waiting_input" || x.Steps[i].State == "waiting_approval" {
						x.State = "waiting"
					}
				}
			} else {
				if x.State != "blocked" || len(x.Blockers) == 0 || x.Blockers[0] == "budget_exceeded" || in.Policy == nil || !policyAllowed(*in.Policy) {
					return ErrInvalid
				}
				x.Policy, x.State, x.Blockers = *in.Policy, "running", nil
			}
		case "skip":
			i := stepIndex(*x, in.StepID)
			if i < 0 || !x.Steps[i].Optional || (x.Steps[i].State != "pending" && x.Steps[i].State != "ready" && x.Steps[i].State != "retry_ready" && x.Steps[i].State != "waiting_input" && x.Steps[i].State != "waiting_approval" && x.Steps[i].State != "manual_ready") {
				return ErrInvalid
			}
			x.Steps[i].State, x.Steps[i].Blocker, x.Steps[i].WaitingApproval, x.Steps[i].RequestedInput = "skipped", "", nil, ""
			advanceExecution(x, now)
		case "provide_input":
			i := stepIndex(*x, in.StepID)
			if i < 0 || x.Steps[i].State != "waiting_input" || len(in.Inputs) == 0 {
				return ErrInvalid
			}
			for k, v := range in.Inputs {
				if !identifier(k) || !safeValue(v) {
					return ErrInvalid
				}
			}
			x.Steps[i].ProvidedInputs, x.Steps[i].RequestedInput, x.Steps[i].State, x.State = in.Inputs, "", "ready", "running"
		case "approve":
			i := stepIndex(*x, in.StepID)
			if i < 0 || x.Steps[i].State != "waiting_approval" || x.Steps[i].WaitingApproval == nil || !contains(x.Steps[i].WaitingApproval.OwnerIDs, actor) {
				return ErrConflict
			}
			x.Steps[i].WaitingApproval, x.Steps[i].State, x.State = nil, "ready", "running"
		case "take_over":
			i := stepIndex(*x, in.StepID)
			if i < 0 || x.Steps[i].State != "manual_ready" || x.Steps[i].InvocationKind != "manual" {
				return ErrInvalid
			}
			x.Steps[i].ManualActorID, x.Steps[i].State, x.State = actor, "running", "running"
			x.Steps[i].Attempts = append(x.Steps[i].Attempts, StepAttempt{Number: len(x.Steps[i].Attempts) + 1, IdempotencyKey: "manual-" + newID(), State: "running", StartedAt: now})
		case "revoke_access", "stale_inputs":
			if executionTerminal(x.State) {
				return ErrInvalid
			}
			x.State = "blocked"
			x.Blockers = []string{in.Action}
			for i := range x.Steps {
				if x.Steps[i].Credential != nil && x.Steps[i].Credential.RevokedAt == nil {
					x.Steps[i].Credential.RevokedAt = &now
				}
				if x.Steps[i].State == "running" {
					x.Steps[i].State = "blocked"
					x.Steps[i].Blocker = in.Action
				}
			}
		default:
			return ErrInvalid
		}
		x.event(in.Action, actor, in.StepID, in.Reason, now)
		return nil
	})
}
