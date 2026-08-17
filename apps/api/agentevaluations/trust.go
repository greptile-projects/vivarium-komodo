package agentevaluations

import (
	"strings"
	"time"
)

// TrustState is repository-owned evidence about an activated agent. It stores
// references and bounded summaries, never task content, logs, prompts, or secrets.
type TrustState struct {
	Version               int64              `json:"version"`
	Policy                ReevaluationPolicy `json:"reevaluation_policy"`
	Outcomes              []AgentOutcome     `json:"outcomes"`
	Reevaluations         []Reevaluation     `json:"reevaluations"`
	Controls              []AuthorityControl `json:"authority_controls"`
	Handoffs              []AgentHandoff     `json:"handoffs"`
	Notices               []TrustNotice      `json:"notices"`
	EffectiveResources    []string           `json:"effective_resources"`
	EffectiveActions      []string           `json:"effective_actions"`
	AuthorityStatus       string             `json:"authority_status"`
	ConsentProfileVersion int64              `json:"consent_profile_version"`
}
type ReevaluationPolicy struct {
	IntervalDays                   int     `json:"interval_days"`
	RequiredSuiteID                string  `json:"required_suite_id"`
	SuspendOnFailure               bool    `json:"suspend_on_failure"`
	MaximumVerificationFailureRate float64 `json:"maximum_verification_failure_rate"`
	MaximumAverageCost             float64 `json:"maximum_average_cost"`
	Currency                       string  `json:"currency"`
	ExpectedVersion                int64   `json:"expected_version,omitempty"`
}
type EvidenceReference struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
}
type OutcomeInput struct {
	Kind             string              `json:"kind"`
	WorkKind         string              `json:"work_kind"`
	WorkID           string              `json:"work_id"`
	Summary          string              `json:"summary"`
	Evidence         []EvidenceReference `json:"evidence"`
	Cost             float64             `json:"cost"`
	Currency         string              `json:"currency"`
	ResponsivenessMS int64               `json:"responsiveness_ms"`
	OccurredAt       time.Time           `json:"occurred_at"`
}
type AgentOutcome struct {
	ID string `json:"id"`
	OutcomeInput
	RecordedBy string    `json:"recorded_by"`
	RecordedAt time.Time `json:"recorded_at"`
}
type Reevaluation struct {
	TrialID        string    `json:"trial_id"`
	ProfileVersion int64     `json:"profile_version"`
	Result         string    `json:"result"`
	Rationale      string    `json:"rationale"`
	RecordedBy     string    `json:"recorded_by"`
	RecordedAt     time.Time `json:"recorded_at"`
}
type AuthorityControl struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	Resources []string  `json:"resources,omitempty"`
	Actions   []string  `json:"actions,omitempty"`
	Reason    string    `json:"reason"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type HandoffInput struct {
	WorkKind                string              `json:"work_kind"`
	WorkID                  string              `json:"work_id"`
	ReplacementOnboardingID string              `json:"replacement_onboarding_id"`
	Summary                 string              `json:"summary"`
	Completed               []EvidenceReference `json:"completed"`
	Remaining               []string            `json:"remaining"`
	VerificationCriteria    []string            `json:"verification_criteria"`
	ResidualRisks           []string            `json:"residual_risks"`
	ExpectedVersion         int64               `json:"expected_version"`
}
type AgentHandoff struct {
	ID string `json:"id"`
	HandoffInput
	FromIdentity string     `json:"from_identity"`
	ToIdentity   string     `json:"to_identity"`
	State        string     `json:"state"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	AcceptedBy   string     `json:"accepted_by,omitempty"`
	AcceptedAt   *time.Time `json:"accepted_at,omitempty"`
	Verification string     `json:"verification,omitempty"`
}
type TrustNotice struct {
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func newTrustState() TrustState {
	return TrustState{Version: 1, AuthorityStatus: "inactive", Outcomes: []AgentOutcome{}, Reevaluations: []Reevaluation{}, Controls: []AuthorityControl{}, Handoffs: []AgentHandoff{}, Notices: []TrustNotice{}}
}
func outcomeKindOK(k string) bool {
	return map[string]bool{"task_outcome": true, "reviewer_correction": true, "verification_failure": true, "reversion": true, "security_violation": true, "policy_violation": true, "accepted_contribution": true}[k]
}
func safeText(x string, n int) bool {
	x = strings.TrimSpace(x)
	return x != "" && len(x) <= n && !strings.Contains(strings.ToLower(x), "secret")
}
func subset(xs, allowed []string) bool {
	a := map[string]bool{}
	for _, x := range allowed {
		a[x] = true
	}
	for _, x := range xs {
		if !a[x] {
			return false
		}
	}
	return len(xs) > 0 && validList(xs)
}
func (s *Store) trustProjection(x *Onboarding) {
	if x.Trust.Version == 0 {
		x.Trust = newTrustState()
	}
	v := currentOnboarding(*x)
	x.Trust.EffectiveResources = append([]string{}, v.Resources...)
	x.Trust.EffectiveActions = append([]string{}, v.Actions...)
	if x.State == "active" {
		x.Trust.AuthorityStatus = "active"
	} else {
		x.Trust.AuthorityStatus = x.State
	}
	for _, c := range x.Trust.Controls {
		switch c.Action {
		case "narrow":
			x.Trust.EffectiveResources = append([]string{}, c.Resources...)
			x.Trust.EffectiveActions = append([]string{}, c.Actions...)
		case "suspend":
			x.Trust.AuthorityStatus = "suspended"
		case "resume":
			x.Trust.AuthorityStatus = "active"
		case "revoke":
			x.Trust.AuthorityStatus = "revoked"
		}
	}
	if x.Trust.Policy.IntervalDays > 0 {
		last := x.ActivatedAt
		if n := len(x.Trust.Reevaluations); n > 0 {
			last = &x.Trust.Reevaluations[n-1].RecordedAt
		}
		if last != nil && !last.Add(time.Duration(x.Trust.Policy.IntervalDays)*24*time.Hour).After(s.now().UTC()) {
			x.Trust.Notices = upsertNotice(x.Trust.Notices, TrustNotice{Kind: "reevaluation_overdue", Severity: "critical", Message: "Periodic reevaluation is overdue.", Action: "run_required_suite", CreatedAt: s.now().UTC()})
		}
	}
}
func upsertNotice(xs []TrustNotice, n TrustNotice) []TrustNotice {
	for _, x := range xs {
		if x.Kind == n.Kind {
			return xs
		}
	}
	return append(xs, n)
}
func (s *Store) SetTrustPolicy(kind, scope, oid, actor string, in ReevaluationPolicy) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	s.trustProjection(&x)
	if in.ExpectedVersion != x.Trust.Version || in.IntervalDays < 1 || in.RequiredSuiteID == "" || in.MaximumVerificationFailureRate < 0 || in.MaximumVerificationFailureRate > 1 || in.MaximumAverageCost < 0 || in.Currency == "" {
		return x, ErrInvalid
	}
	in.ExpectedVersion = 0
	x.Trust.Policy = in
	x.Trust.Version++
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) RecordOutcome(kind, scope, oid, actor string, in OutcomeInput) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if !outcomeKindOK(in.Kind) || !safeText(in.Summary, 500) || !safeText(in.WorkKind, 64) || !safeText(in.WorkID, 200) || in.Cost < 0 || in.ResponsivenessMS < 0 || in.Currency == "" || in.OccurredAt.IsZero() || in.OccurredAt.After(s.now().UTC().Add(time.Minute)) {
		return x, ErrInvalid
	}
	for _, e := range in.Evidence {
		if !safeText(e.Kind, 64) || !safeText(e.ID, 200) {
			return x, ErrInvalid
		}
	}
	now := s.now().UTC()
	x.Trust.Outcomes = append(x.Trust.Outcomes, AgentOutcome{ID: id("aout_"), OutcomeInput: in, RecordedBy: actor, RecordedAt: now})
	x.Trust.Version++
	if in.Kind == "security_violation" || in.Kind == "policy_violation" {
		x.Trust.Notices = upsertNotice(x.Trust.Notices, TrustNotice{Kind: in.Kind, Severity: "critical", Message: "A reported " + strings.ReplaceAll(in.Kind, "_", " ") + " requires maintainer review.", Action: "suspend_or_revoke", CreatedAt: now})
	}
	failures, total, cost := 0, 0, 0.0
	for _, o := range x.Trust.Outcomes {
		total++
		cost += o.Cost
		if o.Kind == "verification_failure" || o.Kind == "reversion" {
			failures++
		}
	}
	if total >= 3 && float64(failures)/float64(total) > x.Trust.Policy.MaximumVerificationFailureRate {
		x.Trust.Notices = upsertNotice(x.Trust.Notices, TrustNotice{Kind: "deteriorating_results", Severity: "warning", Message: "Verification failures exceed the accepted threshold.", Action: "reevaluate_or_narrow", CreatedAt: now})
	}
	if total > 0 && x.Trust.Policy.MaximumAverageCost > 0 && cost/float64(total) > x.Trust.Policy.MaximumAverageCost {
		x.Trust.Notices = upsertNotice(x.Trust.Notices, TrustNotice{Kind: "anomalous_cost", Severity: "warning", Message: "Average reported cost exceeds the accepted threshold.", Action: "review_budget_or_suspend", CreatedAt: now})
	}
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) RecordReevaluation(kind, scope, oid, actor, trialID, result, rationale string, profileVersion int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	var t Trial
	if s.read("trials", trialID, &t) != nil || (t.Status != "completed" && t.Status != "failed") || t.ProfileID != currentOnboarding(x).ProfileID || t.ProfileVersion != profileVersion || (kind == "repository" && t.RepositoryID != scope) {
		return x, ErrInvalid
	}
	if x.Trust.Policy.RequiredSuiteID != "" && t.SuiteID != x.Trust.Policy.RequiredSuiteID {
		return x, ErrInvalid
	}
	if result != "passed" && result != "failed" || !safeText(rationale, 500) {
		return x, ErrInvalid
	}
	if result == "passed" {
		accepted := false
		for _, d := range t.Decisions {
			accepted = accepted || d.Verdict == "accept" || d.Verdict == "approve"
		}
		if !accepted || t.Contamination || len(t.BudgetFailures)+len(t.PolicyFailures) > 0 {
			return x, ErrInvalid
		}
	}
	now := s.now().UTC()
	x.Trust.Reevaluations = append(x.Trust.Reevaluations, Reevaluation{TrialID: trialID, ProfileVersion: profileVersion, Result: result, Rationale: rationale, RecordedBy: actor, RecordedAt: now})
	x.Trust.Version++
	if result == "failed" {
		x.Trust.Notices = upsertNotice(x.Trust.Notices, TrustNotice{Kind: "reevaluation_failed", Severity: "critical", Message: "The required reevaluation failed.", Action: "replace_or_restrict", CreatedAt: now})
		if x.Trust.Policy.SuspendOnFailure {
			x.Trust.Controls = append(x.Trust.Controls, AuthorityControl{ID: id("actl_"), Action: "suspend", Reason: "required reevaluation failed", ActorID: actor, CreatedAt: now})
		}
	}
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) ControlAuthority(kind, scope, oid, actor, action, reason string, resources, actions []string, expected int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	s.trustProjection(&x)
	if expected != x.Trust.Version || !safeText(reason, 500) || !map[string]bool{"narrow": true, "suspend": true, "resume": true, "revoke": true}[action] {
		return x, ErrConflict
	}
	if action == "narrow" && (!subset(resources, currentOnboarding(x).Resources) || !subset(actions, currentOnboarding(x).Actions)) {
		return x, ErrInvalid
	}
	if action != "narrow" && (len(resources) > 0 || len(actions) > 0) {
		return x, ErrInvalid
	}
	now := s.now().UTC()
	x.Trust.Controls = append(x.Trust.Controls, AuthorityControl{ID: id("actl_"), Action: action, Resources: resources, Actions: actions, Reason: reason, ActorID: actor, CreatedAt: now})
	x.Trust.Version++
	if action == "revoke" {
		x.State = "revoked"
		x.RevokedBy = actor
		x.RevokedAt = &now
		x.RevocationReason = reason
	}
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) CreateHandoff(kind, scope, oid, actor string, in HandoffInput) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x, to Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	s.trustProjection(&x)
	if in.ExpectedVersion != x.Trust.Version || s.read("onboardings", in.ReplacementOnboardingID, &to) != nil || to.ScopeKind != kind || to.ScopeID != scope || to.State != "active" || !safeText(in.WorkKind, 64) || !safeText(in.WorkID, 200) || !safeText(in.Summary, 500) || len(in.Remaining) == 0 || len(in.VerificationCriteria) == 0 {
		return x, ErrInvalid
	}
	now := s.now().UTC()
	in.ExpectedVersion = 0
	x.Trust.Handoffs = append(x.Trust.Handoffs, AgentHandoff{ID: id("ahf_"), HandoffInput: in, FromIdentity: x.Identity, ToIdentity: to.Identity, State: "pending_acceptance", CreatedBy: actor, CreatedAt: now})
	x.Trust.Version++
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) AcceptHandoff(kind, scope, oid, hid, actor, verification string, expected int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	s.trustProjection(&x)
	if expected != x.Trust.Version || !safeText(verification, 500) {
		return x, ErrConflict
	}
	found := false
	now := s.now().UTC()
	for i := range x.Trust.Handoffs {
		if x.Trust.Handoffs[i].ID == hid && x.Trust.Handoffs[i].State == "pending_acceptance" {
			x.Trust.Handoffs[i].State = "accepted"
			x.Trust.Handoffs[i].AcceptedBy = actor
			x.Trust.Handoffs[i].AcceptedAt = &now
			x.Trust.Handoffs[i].Verification = verification
			found = true
		}
	}
	if !found {
		return x, ErrNotFound
	}
	x.Trust.Version++
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
