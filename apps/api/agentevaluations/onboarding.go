package agentevaluations

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type OnboardingBudget struct {
	MaximumCost float64 `json:"maximum_cost"`
	Currency    string  `json:"currency"`
	MaximumRuns int     `json:"maximum_runs"`
}
type OnboardingSchedule struct {
	StartsAt  time.Time `json:"starts_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
type OnboardingInput struct {
	TrialIDs                  []string           `json:"trial_ids"`
	ProfileID                 string             `json:"profile_id"`
	ProfileVersion            int64              `json:"profile_version"`
	Roles                     []string           `json:"roles"`
	Resources                 []string           `json:"resources"`
	Actions                   []string           `json:"actions"`
	DataBoundaries            []string           `json:"data_boundaries"`
	Budget                    OnboardingBudget   `json:"budget"`
	Schedule                  OnboardingSchedule `json:"schedule"`
	RequiredApproverIDs       []string           `json:"required_approver_ids"`
	OperatorAgreementRequired bool               `json:"operator_agreement_required"`
	HumanSponsorID            string             `json:"human_sponsor_id,omitempty"`
	ConsequentialDecisions    []string           `json:"consequential_decisions"`
	PolicyExceptions          []string           `json:"policy_exceptions,omitempty"`
	ChangeReason              string             `json:"change_reason"`
	ExpectedVersion           int64              `json:"expected_version,omitempty"`
}
type OnboardingDecision struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Note      string    `json:"note,omitempty"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
type OperatorAgreement struct {
	OperatorID string    `json:"operator_id"`
	Terms      string    `json:"terms"`
	Version    int64     `json:"version"`
	AcceptedAt time.Time `json:"accepted_at"`
}
type AuthorityPreview struct {
	Subject                string             `json:"subject"`
	Roles                  []string           `json:"roles"`
	Resources              []string           `json:"resources"`
	Actions                []string           `json:"actions"`
	DataBoundaries         []string           `json:"data_boundaries"`
	Budget                 OnboardingBudget   `json:"budget"`
	Schedule               OnboardingSchedule `json:"schedule"`
	HumanSponsorID         string             `json:"human_sponsor_id,omitempty"`
	ConsequentialDecisions []string           `json:"consequential_decisions"`
	ExplicitlyExcluded     []string           `json:"explicitly_excluded"`
	Blockers               []string           `json:"blockers"`
}
type OnboardingVersion struct {
	Number int64 `json:"number"`
	OnboardingInput
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
type Onboarding struct {
	ID               string               `json:"id"`
	ScopeKind        string               `json:"scope_kind"`
	ScopeID          string               `json:"scope_id"`
	CurrentVersion   int64                `json:"current_version"`
	Versions         []OnboardingVersion  `json:"versions"`
	Decisions        []OnboardingDecision `json:"decisions"`
	Agreement        *OperatorAgreement   `json:"operator_agreement,omitempty"`
	State            string               `json:"state"`
	Identity         string               `json:"identity,omitempty"`
	ActivatedBy      string               `json:"activated_by,omitempty"`
	ActivatedAt      *time.Time           `json:"activated_at,omitempty"`
	RevokedBy        string               `json:"revoked_by,omitempty"`
	RevokedAt        *time.Time           `json:"revoked_at,omitempty"`
	RevocationReason string               `json:"revocation_reason,omitempty"`
	Trust            TrustState           `json:"trust"`
	Preview          AuthorityPreview     `json:"authority_preview"`
}

func onboardingValid(in OnboardingInput, now time.Time) bool {
	return in.ProfileID != "" && in.ProfileVersion > 0 && len(in.TrialIDs) > 0 && len(in.Roles) > 0 && len(in.Resources) > 0 && len(in.Actions) > 0 && len(in.DataBoundaries) > 0 && len(in.RequiredApproverIDs) > 0 && in.ChangeReason != "" && in.Budget.MaximumCost >= 0 && in.Budget.MaximumRuns > 0 && in.Budget.Currency != "" && in.Schedule.ExpiresAt.After(in.Schedule.StartsAt) && in.Schedule.ExpiresAt.After(now) && validList(in.TrialIDs) && validList(in.Roles) && validList(in.Resources) && validList(in.Actions) && validList(in.DataBoundaries) && validList(in.RequiredApproverIDs) && len(in.ConsequentialDecisions) > 0 && (in.HumanSponsorID != "" || len(in.ConsequentialDecisions) == 0)
}
func currentOnboarding(x Onboarding) OnboardingVersion { return x.Versions[len(x.Versions)-1] }
func (s *Store) onboardingPreview(x Onboarding) AuthorityPreview {
	v := currentOnboarding(x)
	p := AuthorityPreview{Subject: "project-agent:" + x.ID, Roles: v.Roles, Resources: v.Resources, Actions: v.Actions, DataBoundaries: v.DataBoundaries, Budget: v.Budget, Schedule: v.Schedule, HumanSponsorID: v.HumanSponsorID, ConsequentialDecisions: v.ConsequentialDecisions, ExplicitlyExcluded: []string{"repository ownership", "merge authority", "secrets", "fund withdrawal", "governance standing", "undeclared resources and actions"}}
	approved := map[string]bool{}
	denied := false
	for _, d := range x.Decisions {
		if d.Version == x.CurrentVersion {
			approved[d.ActorID] = d.Decision == "approved"
			denied = denied || d.Decision == "denied"
		}
	}
	for _, a := range v.RequiredApproverIDs {
		if !approved[a] {
			p.Blockers = append(p.Blockers, "approval_required:"+a)
		}
	}
	if denied {
		p.Blockers = append(p.Blockers, "approval_denied")
	}
	if v.OperatorAgreementRequired && (x.Agreement == nil || x.Agreement.Version != x.CurrentVersion) {
		p.Blockers = append(p.Blockers, "operator_agreement_required")
	}
	if v.Schedule.StartsAt.After(s.now().UTC()) {
		p.Blockers = append(p.Blockers, "schedule_not_started")
	}
	if !v.Schedule.ExpiresAt.After(s.now().UTC()) {
		p.Blockers = append(p.Blockers, "expired")
	}
	sort.Strings(p.Blockers)
	return p
}
func (s *Store) validateTrials(scopeKind, scopeID string, in OnboardingInput) bool {
	for _, tid := range in.TrialIDs {
		var t Trial
		if s.read("trials", tid, &t) != nil || (scopeKind == "repository" && t.RepositoryID != scopeID) || t.ProfileID != in.ProfileID || t.ProfileVersion != in.ProfileVersion || t.Status != "completed" || t.Contamination || len(t.BudgetFailures)+len(t.PolicyFailures) > 0 {
			return false
		}
		passed := false
		for _, d := range t.Decisions {
			passed = passed || d.Verdict == "accept" || d.Verdict == "approve"
		}
		if !passed {
			return false
		}
	}
	return true
}
func (s *Store) CreateOnboarding(kind, scope, actor string, in OnboardingInput) (Onboarding, error) {
	now := s.now().UTC()
	if (kind != "repository" && kind != "organization") || scope == "" || !onboardingValid(in, now) || in.ExpectedVersion != 0 {
		return Onboarding{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.validateTrials(kind, scope, in) {
		return Onboarding{}, ErrInvalid
	}
	x := Onboarding{ID: id("aon_"), ScopeKind: kind, ScopeID: scope, CurrentVersion: 1, State: "draft", Versions: []OnboardingVersion{{Number: 1, OnboardingInput: in, CreatedBy: actor, CreatedAt: now}}, Trust: newTrustState()}
	x.Preview = s.onboardingPreview(x)
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) ReviseOnboarding(kind, scope, oid, actor string, in OnboardingInput) (Onboarding, error) {
	now := s.now().UTC()
	if !onboardingValid(in, now) {
		return Onboarding{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if in.ExpectedVersion != x.CurrentVersion || x.State == "revoked" {
		return x, ErrConflict
	}
	if !s.validateTrials(kind, scope, in) {
		return x, ErrInvalid
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, OnboardingVersion{Number: x.CurrentVersion, OnboardingInput: in, CreatedBy: actor, CreatedAt: now})
	if x.State == "active" {
		x.State = "pending_upgrade"
	}
	x.Preview = s.onboardingPreview(x)
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) GetOnboarding(kind, scope, oid string) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	x.Preview = s.onboardingPreview(x)
	s.trustProjection(&x)
	if x.State == "active" && !x.Preview.Schedule.ExpiresAt.After(s.now().UTC()) {
		x.State = "expired"
	}
	return x, nil
}
func (s *Store) ListOnboardings(kind, scope string) ([]Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "onboardings"))
	if e != nil {
		return nil, e
	}
	out := []Onboarding{}
	for _, f := range es {
		var x Onboarding
		if s.read("onboardings", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.ScopeKind == kind && x.ScopeID == scope {
			x.Preview = s.onboardingPreview(x)
			s.trustProjection(&x)
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Store) DecideOnboarding(kind, scope, oid, actor, decision, note string, version int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if version != x.CurrentVersion {
		return x, ErrConflict
	}
	v := currentOnboarding(x)
	found := false
	for _, a := range v.RequiredApproverIDs {
		found = found || a == actor
	}
	if !found || (decision != "approved" && decision != "denied") {
		return x, ErrInvalid
	}
	now := s.now().UTC()
	x.Decisions = append(x.Decisions, OnboardingDecision{ActorID: actor, Decision: decision, Note: note, Version: version, CreatedAt: now})
	x.Preview = s.onboardingPreview(x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) AgreeOnboarding(kind, scope, oid, operator, terms string, version int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if version != x.CurrentVersion {
		return x, ErrConflict
	}
	if strings.TrimSpace(terms) == "" {
		return x, ErrInvalid
	}
	now := s.now().UTC()
	x.Agreement = &OperatorAgreement{OperatorID: operator, Terms: terms, Version: version, AcceptedAt: now}
	x.Preview = s.onboardingPreview(x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) ActivateOnboarding(kind, scope, oid, actor string, version int64) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if version != x.CurrentVersion {
		return x, ErrConflict
	}
	x.Preview = s.onboardingPreview(x)
	if len(x.Preview.Blockers) > 0 || x.State == "revoked" {
		return x, ErrConflict
	}
	now := s.now().UTC()
	x.State = "active"
	x.Identity = fmt.Sprintf("agent:%s:%s", kind, x.ID)
	x.ActivatedBy = actor
	x.ActivatedAt = &now
	x.Trust.ConsentProfileVersion = currentOnboarding(x).ProfileVersion
	s.trustProjection(&x)
	return x, s.write("onboardings", x.ID, x)
}
func (s *Store) RevokeOnboarding(kind, scope, oid, actor, reason string) (Onboarding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x Onboarding
	if s.read("onboardings", oid, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope {
		return x, ErrNotFound
	}
	if strings.TrimSpace(reason) == "" {
		return x, ErrInvalid
	}
	now := s.now().UTC()
	x.State = "revoked"
	x.RevokedBy = actor
	x.RevokedAt = &now
	x.RevocationReason = reason
	x.Preview.Blockers = append(x.Preview.Blockers, "revoked")
	return x, s.write("onboardings", x.ID, x)
}
