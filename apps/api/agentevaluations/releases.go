package agentevaluations

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var releaseApprovalKinds = []string{"domain_review", "pilot_acceptance", "data_policy", "resource_approval"}

type AgentReleaseInput struct {
	OnboardingID       string   `json:"onboarding_id"`
	TrialIDs           []string `json:"trial_ids"`
	PilotID            string   `json:"pilot_id"`
	BehaviorContractID string   `json:"behavior_contract_id"`
	BehaviorVersion    int64    `json:"behavior_version"`
	RepositoryRevision string   `json:"repository_revision"`
	ModelVersion       string   `json:"model_version"`
	ToolVersions       []string `json:"tool_versions"`
	OperatorTerms      string   `json:"operator_terms"`
	ChangeReason       string   `json:"change_reason"`
}
type ReleaseApproval struct {
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	At        time.Time `json:"at"`
}
type AgentRelease struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	AgentReleaseInput
	ProfileID         string            `json:"profile_id"`
	ProfileVersion    int64             `json:"profile_version"`
	Identity          string            `json:"identity"`
	Approvals         []ReleaseApproval `json:"approvals"`
	State             string            `json:"state"`
	Blockers          []string          `json:"blockers"`
	AttestationDigest string            `json:"attestation_digest,omitempty"`
	PublishedBy       string            `json:"published_by,omitempty"`
	PublishedAt       *time.Time        `json:"published_at,omitempty"`
	Deployments       []AgentDeployment `json:"deployments"`
	CreatedBy         string            `json:"created_by"`
	CreatedAt         time.Time         `json:"created_at"`
}
type AgentDeploymentInput struct {
	Roles                []string `json:"roles"`
	Resources            []string `json:"resources"`
	Actions              []string `json:"actions"`
	CredentialReferences []string `json:"credential_references"`
	MaximumCost          float64  `json:"maximum_cost"`
	Currency             string   `json:"currency"`
	MaximumLatencyMS     int64    `json:"maximum_latency_ms"`
	RollbackReleaseID    string   `json:"rollback_release_id"`
	ExpectedVersion      int64    `json:"expected_version"`
}
type ReleaseSignal struct {
	ID        string              `json:"id"`
	Kind      string              `json:"kind"`
	Summary   string              `json:"summary"`
	Cost      float64             `json:"cost"`
	Currency  string              `json:"currency"`
	LatencyMS int64               `json:"latency_ms"`
	Evidence  []EvidenceReference `json:"evidence"`
	ActorID   string              `json:"actor_id"`
	At        time.Time           `json:"at"`
}
type DeploymentControl struct {
	Action   string    `json:"action"`
	Reason   string    `json:"reason"`
	WorkKind string    `json:"work_kind,omitempty"`
	WorkID   string    `json:"work_id,omitempty"`
	OwnerID  string    `json:"owner_id,omitempty"`
	ActorID  string    `json:"actor_id"`
	At       time.Time `json:"at"`
}
type AgentDeployment struct {
	ID string `json:"id"`
	AgentDeploymentInput
	Version   int64               `json:"version"`
	State     string              `json:"state"`
	Signals   []ReleaseSignal     `json:"signals"`
	Controls  []DeploymentControl `json:"controls"`
	CreatedBy string              `json:"created_by"`
	CreatedAt time.Time           `json:"created_at"`
}

func (s *Store) deriveRelease(x *AgentRelease) {
	approved := map[string]bool{}
	for _, a := range x.Approvals {
		if a.Decision == "approved" {
			approved[a.Kind] = true
		} else {
			approved[a.Kind] = false
		}
	}
	x.Blockers = nil
	for _, k := range releaseApprovalKinds {
		if !approved[k] {
			x.Blockers = append(x.Blockers, k+"_required")
		}
	}
	var o Onboarding
	if s.read("onboardings", x.OnboardingID, &o) != nil || o.State != "active" {
		x.Blockers = append(x.Blockers, "current_active_onboarding_required")
	} else {
		s.trustProjection(&o)
		v := currentOnboarding(o)
		if o.Trust.AuthorityStatus != "active" {
			x.Blockers = append(x.Blockers, "current_agent_trust_required")
		}
		if v.ProfileVersion != x.ProfileVersion || o.Trust.ConsentProfileVersion != x.ProfileVersion {
			x.Blockers = append(x.Blockers, "fresh_profile_consent_required")
		}
	}
	sort.Strings(x.Blockers)
}
func (s *Store) CreateRelease(repo, actor string, in AgentReleaseInput) (AgentRelease, error) {
	if repo == "" || actor == "" || in.OnboardingID == "" || len(in.TrialIDs) == 0 || in.PilotID == "" || in.BehaviorContractID == "" || in.BehaviorVersion < 1 || in.RepositoryRevision == "" || in.ModelVersion == "" || !validList(in.ToolVersions) || in.OperatorTerms == "" || in.ChangeReason == "" {
		return AgentRelease{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var o Onboarding
	if s.read("onboardings", in.OnboardingID, &o) != nil || o.ScopeKind != "repository" || o.ScopeID != repo || o.State != "active" {
		return AgentRelease{}, ErrInvalid
	}
	v := currentOnboarding(o)
	if o.Agreement == nil || o.Agreement.Version != o.CurrentVersion || o.Agreement.Terms != in.OperatorTerms {
		return AgentRelease{}, ErrInvalid
	}
	for _, tid := range in.TrialIDs {
		var t Trial
		if s.read("trials", tid, &t) != nil || t.RepositoryID != repo || t.Status != "completed" || t.Contamination || len(t.PolicyFailures)+len(t.BudgetFailures) > 0 {
			return AgentRelease{}, ErrInvalid
		}
	}
	var p Pilot
	if s.read("pilots", in.PilotID, &p) != nil || p.RepositoryID != repo {
		return AgentRelease{}, ErrInvalid
	}
	derivePilot(&p, s.now().UTC())
	if p.State != "active" || len(p.Feedback) == 0 {
		return AgentRelease{}, ErrInvalid
	}
	for _, c := range p.Consents {
		if c.State != "accepted" {
			return AgentRelease{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	x := AgentRelease{ID: id("arl_"), RepositoryID: repo, AgentReleaseInput: in, ProfileID: v.ProfileID, ProfileVersion: v.ProfileVersion, Identity: o.Identity, State: "draft", Approvals: []ReleaseApproval{}, Deployments: []AgentDeployment{}, CreatedBy: actor, CreatedAt: now}
	s.deriveRelease(&x)
	return x, s.write("releases", x.ID, x)
}
func (s *Store) mutateRelease(repo, rid string, fn func(*AgentRelease) error) (AgentRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var x AgentRelease
	if s.read("releases", rid, &x) != nil || x.RepositoryID != repo {
		return x, ErrNotFound
	}
	if e := fn(&x); e != nil {
		return x, e
	}
	s.deriveRelease(&x)
	return x, s.write("releases", x.ID, x)
}
func (s *Store) GetRelease(repo, rid string) (AgentRelease, error) {
	return s.mutateRelease(repo, rid, func(*AgentRelease) error { return nil })
}
func (s *Store) ListReleases(repo string) ([]AgentRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, "releases"))
	if e != nil {
		return nil, e
	}
	out := []AgentRelease{}
	for _, f := range es {
		var x AgentRelease
		if s.read("releases", strings.TrimSuffix(f.Name(), ".json"), &x) == nil && x.RepositoryID == repo {
			s.deriveRelease(&x)
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) DecideRelease(repo, rid, actor, kind, decision, rationale string) (AgentRelease, error) {
	if !contains(releaseApprovalKinds, kind) || (decision != "approved" && decision != "rejected") || !safeText(rationale, 500) {
		return AgentRelease{}, ErrInvalid
	}
	return s.mutateRelease(repo, rid, func(x *AgentRelease) error {
		if x.State != "draft" {
			return ErrConflict
		}
		x.Approvals = append(x.Approvals, ReleaseApproval{Kind: kind, ActorID: actor, Decision: decision, Rationale: rationale, At: s.now().UTC()})
		return nil
	})
}
func (s *Store) PublishRelease(repo, rid, actor string) (AgentRelease, error) {
	return s.mutateRelease(repo, rid, func(x *AgentRelease) error {
		s.deriveRelease(x)
		if x.State != "draft" || len(x.Blockers) > 0 {
			return ErrConflict
		}
		raw := x.RepositoryID + "\n" + x.RepositoryRevision + "\n" + x.Identity + "\n" + x.ProfileID + "\n" + strconv.FormatInt(x.ProfileVersion, 10) + "\n" + strings.Join(x.TrialIDs, "\n") + "\n" + x.PilotID + "\n" + x.BehaviorContractID + "\n" + strconv.FormatInt(x.BehaviorVersion, 10) + "\n" + x.ModelVersion + "\n" + strings.Join(x.ToolVersions, "\n") + "\n" + x.OperatorTerms
		for _, approval := range x.Approvals {
			raw += "\n" + approval.Kind + "\n" + approval.ActorID + "\n" + approval.Decision
		}
		d := sha256.Sum256([]byte(raw))
		x.AttestationDigest = "sha256:" + hex.EncodeToString(d[:])
		now := s.now().UTC()
		x.PublishedAt = &now
		x.PublishedBy = actor
		x.State = "attested"
		return nil
	})
}
func (s *Store) DeployRelease(repo, rid, actor string, in AgentDeploymentInput) (AgentRelease, error) {
	if !validList(in.Roles) || !validList(in.Resources) || !validList(in.Actions) || !validList(in.CredentialReferences) || in.MaximumCost <= 0 || in.Currency == "" || in.MaximumLatencyMS < 1 {
		return AgentRelease{}, ErrInvalid
	}
	for _, ref := range in.CredentialReferences {
		if !strings.HasPrefix(ref, "credential-ref:") || !safeText(ref, 200) {
			return AgentRelease{}, ErrInvalid
		}
	}
	return s.mutateRelease(repo, rid, func(x *AgentRelease) error {
		if x.State != "attested" {
			return ErrConflict
		}
		var o Onboarding
		if s.read("onboardings", x.OnboardingID, &o) != nil {
			return ErrInvalid
		}
		s.trustProjection(&o)
		v := currentOnboarding(o)
		if o.Trust.AuthorityStatus != "active" || !subset(in.Resources, v.Resources) || !subset(in.Actions, v.Actions) || !subset(in.Roles, v.Roles) {
			return ErrInvalid
		}
		if in.RollbackReleaseID != "" {
			var rollback AgentRelease
			if s.read("releases", in.RollbackReleaseID, &rollback) != nil || rollback.RepositoryID != repo || rollback.State != "attested" {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		x.Deployments = append(x.Deployments, AgentDeployment{ID: id("ard_"), AgentDeploymentInput: in, Version: 1, State: "active", Signals: []ReleaseSignal{}, Controls: []DeploymentControl{}, CreatedBy: actor, CreatedAt: now})
		return nil
	})
}
func (s *Store) RecordReleaseSignal(repo, rid, did, actor string, in ReleaseSignal) (AgentRelease, error) {
	if !contains([]string{"outcome", "correction", "cost", "latency", "policy", "safety"}, in.Kind) || !safeText(in.Summary, 500) || in.Cost < 0 || in.LatencyMS < 0 {
		return AgentRelease{}, ErrInvalid
	}
	for _, evidence := range in.Evidence {
		if !safeText(evidence.Kind, 64) || !safeText(evidence.ID, 200) || (evidence.Revision != "" && !safeText(evidence.Revision, 200)) {
			return AgentRelease{}, ErrInvalid
		}
	}
	return s.mutateRelease(repo, rid, func(x *AgentRelease) error {
		for i := range x.Deployments {
			if x.Deployments[i].ID == did {
				in.ID = id("ars_")
				in.ActorID = actor
				in.At = s.now().UTC()
				x.Deployments[i].Signals = append(x.Deployments[i].Signals, in)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) ControlDeployment(repo, rid, did, actor, action, reason, workKind, workID, ownerID string, expected int64) (AgentRelease, error) {
	if !contains([]string{"narrow", "pause", "resume", "rollback", "reopen_finding", "create_repair"}, action) || !safeText(reason, 500) {
		return AgentRelease{}, ErrInvalid
	}
	if contains([]string{"reopen_finding", "create_repair"}, action) && (!safeText(workKind, 64) || !safeText(workID, 200) || !safeText(ownerID, 200)) {
		return AgentRelease{}, ErrInvalid
	}
	return s.mutateRelease(repo, rid, func(x *AgentRelease) error {
		for i := range x.Deployments {
			d := &x.Deployments[i]
			if d.ID != did {
				continue
			}
			if d.Version != expected {
				return ErrConflict
			}
			if action == "resume" {
				s.deriveRelease(x)
				if len(x.Blockers) > 0 {
					return ErrConflict
				}
				d.State = "active"
			} else if action == "narrow" {
				d.State = "narrowed"
			} else if action == "pause" {
				d.State = "paused"
			} else if action == "rollback" {
				d.State = "rolled_back"
			}
			d.Version++
			d.Controls = append(d.Controls, DeploymentControl{Action: action, Reason: reason, WorkKind: workKind, WorkID: workID, OwnerID: ownerID, ActorID: actor, At: s.now().UTC()})
			return nil
		}
		return ErrNotFound
	})
}
