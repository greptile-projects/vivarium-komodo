// Package adoptionworkspaces owns evidence-backed software fit evaluations.
package adoptionworkspaces

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

var ErrNotFound = errors.New("adoption workspace not found")
var ErrInvalid = errors.New("invalid adoption workspace")
var ErrForbidden = errors.New("adoption workspace action forbidden")

var originKinds = map[string]bool{"roadmap_outcome": true, "support_gap": true, "incubator": true, "decision": true, "package": true, "api": true, "federated_repository": true}
var dimensions = map[string]bool{"capability": true, "provenance": true, "support": true, "security": true, "data_use": true, "compatibility": true, "gap": true}
var trialAudiences = map[string]bool{"public": true, "shared": true, "provider": true, "consumer": true}

type Origin struct {
	Kind         string `json:"kind"`
	ResourceID   string `json:"resource_id"`
	Revision     string `json:"revision,omitempty"`
	RepositoryID string `json:"repository_id,omitempty"`
	Label        string `json:"label,omitempty"`
}
type Environment struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Version  string `json:"version,omitempty"`
}
type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
type Input struct {
	Title              string        `json:"title"`
	Outcome            string        `json:"outcome"`
	Origin             Origin        `json:"origin"`
	RequiredJourneys   []string      `json:"required_journeys"`
	Environments       []Environment `json:"environments"`
	Constraints        []string      `json:"constraints"`
	Budget             string        `json:"budget"`
	OwnerIDs           []string      `json:"owner_ids"`
	EvaluationCriteria []Criterion   `json:"evaluation_criteria"`
	Visibility         string        `json:"visibility"`
}
type Participant struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	SubjectID      string     `json:"subject_id"`
	Role           string     `json:"role"`
	EvidenceAccess string     `json:"evidence_access"`
	Consent        string     `json:"consent"`
	InvitedByID    string     `json:"invited_by_id"`
	InvitedAt      time.Time  `json:"invited_at"`
	RespondedAt    *time.Time `json:"responded_at,omitempty"`
}
type Evidence struct {
	ID           string     `json:"id"`
	Dimension    string     `json:"dimension"`
	Claim        string     `json:"claim"`
	Reference    string     `json:"reference,omitempty"`
	Revision     string     `json:"revision"`
	Visibility   string     `json:"visibility"`
	Availability string     `json:"availability"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	AddedByID    string     `json:"added_by_id"`
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"`
	ProofOfFit   bool       `json:"proof_of_fit"`
	Gap          string     `json:"gap,omitempty"`
}
type Candidate struct {
	ID                 string             `json:"id"`
	Project            string             `json:"project"`
	ProviderRepository string             `json:"provider_repository"`
	Version            string             `json:"version"`
	Revision           string             `json:"revision"`
	Evidence           []Evidence         `json:"evidence"`
	Coverage           map[string]string  `json:"coverage"`
	Blockers           []string           `json:"blockers"`
	AddedByID          string             `json:"added_by_id"`
	CreatedAt          time.Time          `json:"created_at"`
	Trials             []Trial            `json:"trials"`
	IntegrationPlans   []IntegrationPlan  `json:"integration_plans"`
	Deliveries         []AdoptionDelivery `json:"deliveries"`
}
type TrialSource struct {
	Kind        string `json:"kind"`
	Reference   string `json:"reference"`
	Revision    string `json:"revision"`
	Attestation string `json:"attestation,omitempty"`
}
type TrialData struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Reference   string `json:"reference,omitempty"`
}
type TrialInput struct {
	Name             string            `json:"name"`
	Source           TrialSource       `json:"source"`
	Packages         []string          `json:"packages"`
	APIs             []string          `json:"apis"`
	Data             []TrialData       `json:"data"`
	JourneyIDs       []string          `json:"journey_ids"`
	Policies         []string          `json:"policies"`
	Setup            []string          `json:"setup"`
	Configuration    map[string]string `json:"configuration"`
	Commands         []string          `json:"commands"`
	Budget           string            `json:"budget"`
	EvidenceAudience string            `json:"evidence_audience"`
}
type TrialCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}
type TrialPreview struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
}
type TrialMeasurement struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type TrialFinding struct {
	Kind              string `json:"kind"`
	Summary           string `json:"summary"`
	EvidenceReference string `json:"evidence_reference,omitempty"`
}
type TrialAttemptInput struct {
	Environment        string             `json:"environment"`
	SourceRevision     string             `json:"source_revision"`
	Configuration      map[string]string  `json:"configuration"`
	Commands           []string           `json:"commands"`
	IntegrationChanges []string           `json:"integration_changes"`
	Checks             []TrialCheck       `json:"checks"`
	Previews           []TrialPreview     `json:"previews"`
	Measurements       []TrialMeasurement `json:"measurements"`
	Cost               float64            `json:"cost"`
	Currency           string             `json:"currency"`
	Findings           []TrialFinding     `json:"findings"`
	Artifacts          []string           `json:"artifacts"`
	ReproductionOf     string             `json:"reproduction_of,omitempty"`
}
type TrialAttempt struct {
	ID string `json:"id"`
	TrialAttemptInput
	Status       string    `json:"status"`
	Reproducible bool      `json:"reproducible"`
	AddedByID    string    `json:"added_by_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type TrialFeedback struct {
	ID        string    `json:"id"`
	AttemptID string    `json:"attempt_id"`
	JourneyID string    `json:"journey_id"`
	Verdict   string    `json:"verdict"`
	Comment   string    `json:"comment"`
	AddedByID string    `json:"added_by_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Trial struct {
	ID string `json:"id"`
	TrialInput
	Attempts  []TrialAttempt  `json:"attempts"`
	Feedback  []TrialFeedback `json:"feedback"`
	Status    string          `json:"status"`
	Blockers  []string        `json:"blockers"`
	AddedByID string          `json:"added_by_id"`
	CreatedAt time.Time       `json:"created_at"`
}
type PlanOwnership struct {
	Decision string `json:"decision"`
	OwnerID  string `json:"owner_id"`
	Side     string `json:"side"`
}
type PlanBoundary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
}
type PlanException struct {
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	Resolution  string `json:"resolution"`
}
type PlanGap struct {
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
}
type CompatibilityPromise struct {
	Promise string `json:"promise"`
	OwnerID string `json:"owner_id"`
}
type IntegrationWork struct {
	Key                string   `json:"key"`
	Scope              string   `json:"scope"`
	Target             string   `json:"target"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	DependsOn          []string `json:"depends_on"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type IntegrationPlanInput struct {
	TrialID                string                 `json:"trial_id"`
	SelectedVersion        string                 `json:"selected_version"`
	SelectedRevision       string                 `json:"selected_revision"`
	Architecture           string                 `json:"architecture"`
	ConfigurationOwnership []PlanOwnership        `json:"configuration_ownership"`
	UpdatePolicy           string                 `json:"update_policy"`
	SupportPolicy          string                 `json:"support_policy"`
	ServiceBoundaries      []PlanBoundary         `json:"service_boundaries"`
	DataBoundaries         []PlanBoundary         `json:"data_boundaries"`
	RequiredExceptions     []PlanException        `json:"required_exceptions"`
	ExitStrategy           string                 `json:"exit_strategy"`
	UnresolvedGaps         []PlanGap              `json:"unresolved_gaps"`
	RecurringCost          string                 `json:"recurring_cost"`
	CompatibilityPromises  []CompatibilityPromise `json:"compatibility_promises"`
	Work                   []IntegrationWork      `json:"work"`
}
type IntegrationPreview struct {
	EffectiveAccess   []string `json:"effective_access"`
	RecurringCost     string   `json:"recurring_cost"`
	AccountableOwners []string `json:"accountable_owners"`
	Blockers          []string `json:"blockers"`
}
type IntegrationPlan struct {
	ID string `json:"id"`
	IntegrationPlanInput
	Preview          IntegrationPreview `json:"preview"`
	RecordedByID     string             `json:"recorded_by_id"`
	CreatedAt        time.Time          `json:"created_at"`
	AuthorityGranted bool               `json:"authority_granted"`
}
type DeliveryChange struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}
type DeliveryEvidence struct {
	Kind         string `json:"kind"`
	Reference    string `json:"reference"`
	Revision     string `json:"revision"`
	Status       string `json:"status"`
	ApprovedByID string `json:"approved_by_id,omitempty"`
}
type RolloutStage struct {
	Name              string  `json:"name"`
	Environment       string  `json:"environment"`
	ReleaseRevision   string  `json:"release_revision"`
	Status            string  `json:"status"`
	Health            string  `json:"health"`
	Cost              float64 `json:"cost"`
	Currency          string  `json:"currency"`
	EvidenceReference string  `json:"evidence_reference"`
}
type AdoptionDeliveryInput struct {
	PlanID              string             `json:"plan_id"`
	ProviderRevision    string             `json:"provider_revision"`
	ConsumerRepository  string             `json:"consumer_repository"`
	ConsumerPullRequest string             `json:"consumer_pull_request"`
	ConsumerRevision    string             `json:"consumer_revision"`
	PinnedDependencies  []string           `json:"pinned_dependencies"`
	Changes             []DeliveryChange   `json:"changes"`
	Evidence            []DeliveryEvidence `json:"evidence"`
	Rollout             []RolloutStage     `json:"rollout"`
}
type DeliveryObservation struct {
	ID                string    `json:"id"`
	ConsumerRevision  string    `json:"consumer_revision"`
	Kind              string    `json:"kind"`
	Status            string    `json:"status"`
	Summary           string    `json:"summary"`
	EvidenceReference string    `json:"evidence_reference"`
	Cost              float64   `json:"cost,omitempty"`
	Currency          string    `json:"currency,omitempty"`
	AddedByID         string    `json:"added_by_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type AdoptionDelivery struct {
	ID string `json:"id"`
	AdoptionDeliveryInput
	Status           string                `json:"status"`
	Blockers         []string              `json:"blockers"`
	Observations     []DeliveryObservation `json:"observations"`
	CreatedByID      string                `json:"created_by_id"`
	CreatedAt        time.Time             `json:"created_at"`
	AuthorityGranted bool                  `json:"authority_granted"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Workspace struct {
	ID string `json:"id"`
	Input
	CreatedByID      string        `json:"created_by_id"`
	Participants     []Participant `json:"participants"`
	Candidates       []Candidate   `json:"candidates"`
	History          []Event       `json:"history"`
	AuthorityGranted bool          `json:"authority_granted"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func listOK(xs []string, required bool) bool {
	if (required && len(xs) == 0) || len(xs) > 50 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func validInput(v Input) bool {
	if v.Title == "" || v.Outcome == "" || !originKinds[v.Origin.Kind] || v.Origin.ResourceID == "" || v.Budget == "" || !listOK(v.RequiredJourneys, true) || !listOK(v.Constraints, false) || !listOK(v.OwnerIDs, true) || (v.Visibility != "public" && v.Visibility != "participants") || len(v.Environments) == 0 || len(v.EvaluationCriteria) == 0 {
		return false
	}
	for _, e := range v.Environments {
		if e.Name == "" || e.Platform == "" {
			return false
		}
	}
	for _, c := range v.EvaluationCriteria {
		if c.ID == "" || c.Description == "" {
			return false
		}
	}
	return true
}
func (s *Store) Create(actor string, in Input) (Workspace, error) {
	if actor == "" || !validInput(in) {
		return Workspace{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Workspace{ID: id("adp_"), Input: in, CreatedByID: actor, Participants: []Participant{{ID: id("par_"), Kind: "human", SubjectID: actor, Role: "adopter_owner", EvidenceAccess: "all", Consent: "accepted", InvitedByID: actor, InvitedAt: now, RespondedAt: &now}}, Candidates: []Candidate{}, History: []Event{}, AuthorityGranted: false, CreatedAt: now, UpdatedAt: now}
	v.event("workspace.opened", actor, "")
	return v, s.write(v)
}
func canRead(v Workspace, actor string) bool {
	if v.Visibility == "public" {
		return true
	}
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func canContribute(v Workspace, actor string) bool {
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" && p.Kind == "human" {
			return true
		}
	}
	return false
}
func canRunTrial(v Workspace, actor string) bool {
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func isAgent(v Workspace, actor string) bool {
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Kind == "agent" {
			return true
		}
	}
	return false
}
func isAffectedUser(v Workspace, actor string) bool {
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Kind == "human" && p.Role == "affected_user" && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func isOwner(v Workspace, actor string) bool {
	if v.CreatedByID == actor {
		return true
	}
	for _, x := range v.OwnerIDs {
		if x == actor {
			return true
		}
	}
	return false
}
func (s *Store) Invite(wid, actor string, p Participant) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !isOwner(*v, actor) {
			return ErrForbidden
		}
		if p.SubjectID == "" || !map[string]bool{"provider_maintainer": true, "affected_user": true, "read_only_agent": true}[p.Role] {
			return ErrInvalid
		}
		if p.Role == "read_only_agent" && p.Kind != "agent" || p.Role != "read_only_agent" && p.Kind != "human" {
			return ErrInvalid
		}
		for _, x := range v.Participants {
			if x.SubjectID == p.SubjectID {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		p.ID = id("par_")
		p.InvitedByID = actor
		p.InvitedAt = now
		p.Consent = "pending"
		if p.EvidenceAccess == "" {
			p.EvidenceAccess = "shared"
		}
		if !map[string]bool{"shared": true, "provider": true}[p.EvidenceAccess] {
			return ErrInvalid
		}
		if p.Kind == "agent" {
			p.Consent = "accepted"
			p.RespondedAt = &now
		}
		v.Participants = append(v.Participants, p)
		v.event("participant.invited", actor, p.ID)
		return nil
	})
}
func (s *Store) Consent(wid, pid, actor, decision string) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if decision != "accepted" && decision != "declined" {
			return ErrInvalid
		}
		for i := range v.Participants {
			p := &v.Participants[i]
			if p.ID == pid && p.SubjectID == actor && p.Kind == "human" && p.Consent == "pending" {
				now := s.now().UTC()
				p.Consent = decision
				p.RespondedAt = &now
				v.event("participant."+decision, actor, p.ID)
				return nil
			}
		}
		return ErrForbidden
	})
}
func (s *Store) AddCandidate(wid, actor string, c Candidate) (Workspace, error) {
	return s.mutate(wid, actor, true, func(v *Workspace) error {
		if c.Project == "" || c.Version == "" || c.Revision == "" || c.ProviderRepository == "" {
			return ErrInvalid
		}
		c.ID = id("can_")
		c.AddedByID = actor
		c.CreatedAt = s.now().UTC()
		c.Evidence = []Evidence{}
		c.Trials = []Trial{}
		c.IntegrationPlans = []IntegrationPlan{}
		c.Deliveries = []AdoptionDelivery{}
		v.Candidates = append(v.Candidates, c)
		v.event("candidate.added", actor, c.ID)
		return nil
	})
}

func canPlan(v Workspace, actor string) bool {
	if isOwner(v, actor) {
		return true
	}
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Kind == "human" && p.Role == "provider_maintainer" && p.Consent == "accepted" {
			return true
		}
	}
	return false
}

func validPlan(in IntegrationPlanInput) bool {
	if in.TrialID == "" || in.SelectedVersion == "" || in.SelectedRevision == "" || in.Architecture == "" || in.UpdatePolicy == "" || in.SupportPolicy == "" || in.ExitStrategy == "" || in.RecurringCost == "" || len(in.ConfigurationOwnership) == 0 || len(in.ServiceBoundaries) == 0 || len(in.DataBoundaries) == 0 || len(in.CompatibilityPromises) == 0 || len(in.Work) == 0 {
		return false
	}
	if !safeStrings([]string{in.Architecture, in.UpdatePolicy, in.SupportPolicy, in.ExitStrategy, in.RecurringCost}) {
		return false
	}
	owners := map[string]bool{}
	for _, x := range in.ConfigurationOwnership {
		if x.Decision == "" || x.OwnerID == "" || !map[string]bool{"consumer": true, "provider": true, "shared": true}[x.Side] {
			return false
		}
		owners[x.OwnerID] = true
	}
	for _, xs := range [][]PlanBoundary{in.ServiceBoundaries, in.DataBoundaries} {
		for _, x := range xs {
			if x.Name == "" || x.Description == "" || x.OwnerID == "" {
				return false
			}
			owners[x.OwnerID] = true
		}
	}
	for _, x := range in.RequiredExceptions {
		if x.Description == "" || x.OwnerID == "" || x.Resolution == "" {
			return false
		}
		owners[x.OwnerID] = true
	}
	for _, x := range in.UnresolvedGaps {
		if x.Description == "" || x.OwnerID == "" {
			return false
		}
		owners[x.OwnerID] = true
	}
	for _, x := range in.CompatibilityPromises {
		if x.Promise == "" || x.OwnerID == "" {
			return false
		}
		owners[x.OwnerID] = true
	}
	seen := map[string]bool{}
	for _, x := range in.Work {
		if x.Key == "" || seen[x.Key] || x.Target == "" || x.OwnerID == "" || !map[string]bool{"human": true, "agent": true}[x.OwnerKind] || !map[string]bool{"consumer_repository": true, "environment": true, "documentation": true, "upstream_fork": true}[x.Scope] || len(x.AcceptanceCriteria) == 0 {
			return false
		}
		for _, dep := range x.DependsOn {
			if !seen[dep] {
				return false
			}
		}
		seen[x.Key], owners[x.OwnerID] = true, true
	}
	return true
}

func (s *Store) AddIntegrationPlan(wid, cid, actor string, in IntegrationPlanInput) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !canPlan(*v, actor) {
			return ErrForbidden
		}
		if !validPlan(in) {
			return ErrInvalid
		}
		for ci := range v.Candidates {
			c := &v.Candidates[ci]
			if c.ID != cid {
				continue
			}
			if in.SelectedVersion != c.Version || in.SelectedRevision != c.Revision {
				return ErrInvalid
			}
			passed := false
			for _, t := range c.Trials {
				passed = passed || (t.ID == in.TrialID && t.Status == "passed")
			}
			if !passed {
				return ErrInvalid
			}
			access := []string{}
			owners := map[string]bool{}
			blockers := []string{}
			for _, x := range in.Work {
				control := "consumer-controlled"
				if x.Scope == "upstream_fork" {
					permitted := false
					for _, p := range v.Participants {
						permitted = permitted || (p.SubjectID == x.OwnerID && p.Role == "provider_maintainer" && p.Kind == "human" && p.Consent == "accepted")
					}
					if !permitted {
						return ErrInvalid
					}
					control = "provider-controlled"
				}
				access = append(access, x.Key+": "+control+"; "+x.OwnerKind+" work grants no repository or operational authority")
				owners[x.OwnerID] = true
			}
			for _, x := range in.ConfigurationOwnership {
				owners[x.OwnerID] = true
			}
			for _, xs := range [][]PlanBoundary{in.ServiceBoundaries, in.DataBoundaries} {
				for _, x := range xs {
					owners[x.OwnerID] = true
				}
			}
			for _, x := range in.CompatibilityPromises {
				owners[x.OwnerID] = true
			}
			for _, x := range in.UnresolvedGaps {
				owners[x.OwnerID] = true
				blockers = append(blockers, "unresolved fit gap: "+x.Description)
			}
			for _, x := range in.RequiredExceptions {
				owners[x.OwnerID] = true
				if x.Resolution != "approved" {
					blockers = append(blockers, "required exception: "+x.Description)
				}
			}
			ownerList := []string{}
			for x := range owners {
				ownerList = append(ownerList, x)
			}
			sort.Strings(ownerList)
			sort.Strings(blockers)
			p := IntegrationPlan{ID: id("ipl_"), IntegrationPlanInput: in, Preview: IntegrationPreview{EffectiveAccess: access, RecurringCost: in.RecurringCost, AccountableOwners: ownerList, Blockers: blockers}, RecordedByID: actor, CreatedAt: s.now().UTC(), AuthorityGranted: false}
			c.IntegrationPlans = append(c.IntegrationPlans, p)
			v.event("integration_plan.recorded", actor, p.ID)
			return nil
		}
		return ErrNotFound
	})
}

var deliveryEvidenceKinds = map[string]bool{"provider_attestation": true, "approval": true, "review": true, "policy": true, "rehearsal": true, "release": true, "support_readiness": true, "user_acceptance": true}
var deliveryChangeKinds = map[string]bool{"dependency": true, "integration": true, "configuration": true, "infrastructure": true, "test": true, "documentation": true}

func deriveDelivery(in AdoptionDeliveryInput) (string, []string) {
	blockers := []string{}
	seen := map[string]bool{}
	for _, e := range in.Evidence {
		seen[e.Kind] = true
		if e.Status != "passed" {
			blockers = append(blockers, e.Kind+": "+e.Status)
		}
	}
	for kind := range deliveryEvidenceKinds {
		if !seen[kind] {
			blockers = append(blockers, kind+": missing evidence")
		}
	}
	for _, stage := range in.Rollout {
		if stage.Status != "passed" || stage.Health != "healthy" {
			blockers = append(blockers, "rollout "+stage.Name+": "+stage.Status+"; health "+stage.Health)
		}
	}
	sort.Strings(blockers)
	if len(blockers) > 0 {
		return "paused", blockers
	}
	return "active", blockers
}

func validDelivery(in AdoptionDeliveryInput) bool {
	if in.PlanID == "" || in.ProviderRevision == "" || in.ConsumerRepository == "" || in.ConsumerPullRequest == "" || in.ConsumerRevision == "" || len(in.PinnedDependencies) == 0 || len(in.Changes) == 0 || len(in.Rollout) == 0 || !safeStrings(in.PinnedDependencies) {
		return false
	}
	for _, p := range in.PinnedDependencies {
		if !strings.ContainsAny(p, "@=") {
			return false
		}
	}
	for _, x := range in.Changes {
		if !deliveryChangeKinds[x.Kind] || x.Path == "" {
			return false
		}
	}
	for _, e := range in.Evidence {
		if !deliveryEvidenceKinds[e.Kind] || e.Reference == "" || e.Revision != in.ConsumerRevision || !map[string]bool{"passed": true, "failed": true, "revoked": true}[e.Status] {
			return false
		}
	}
	for i, x := range in.Rollout {
		if x.Name == "" || x.Environment == "" || x.ReleaseRevision != in.ConsumerRevision || x.EvidenceReference == "" || x.Cost < 0 || (x.Cost > 0 && x.Currency == "") || !map[string]bool{"pending": true, "passed": true, "failed": true}[x.Status] || !map[string]bool{"unknown": true, "healthy": true, "unhealthy": true}[x.Health] {
			return false
		}
		if i > 0 && in.Rollout[i-1].Status != "passed" && x.Status != "pending" {
			return false
		}
	}
	return true
}

func (s *Store) AddDelivery(wid, cid, actor string, in AdoptionDeliveryInput) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !isOwner(*v, actor) || isAgent(*v, actor) {
			return ErrForbidden
		}
		if !validDelivery(in) {
			return ErrInvalid
		}
		for ci := range v.Candidates {
			c := &v.Candidates[ci]
			if c.ID != cid {
				continue
			}
			if c.Revision != in.ProviderRevision {
				return ErrInvalid
			}
			found := false
			for _, p := range c.IntegrationPlans {
				found = found || (p.ID == in.PlanID && p.SelectedRevision == in.ProviderRevision)
			}
			if !found {
				return ErrInvalid
			}
			status, blockers := deriveDelivery(in)
			d := AdoptionDelivery{ID: id("adl_"), AdoptionDeliveryInput: in, Status: status, Blockers: blockers, Observations: []DeliveryObservation{}, CreatedByID: actor, CreatedAt: s.now().UTC(), AuthorityGranted: false}
			c.Deliveries = append(c.Deliveries, d)
			v.event("adoption_delivery.connected", actor, d.ID)
			return nil
		}
		return ErrNotFound
	})
}

func (s *Store) AddDeliveryObservation(wid, cid, did, actor string, in DeliveryObservation) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !canRunTrial(*v, actor) || in.Kind == "" || in.Summary == "" || in.EvidenceReference == "" || !map[string]bool{"health": true, "cost": true, "access": true, "compatibility": true, "criteria": true, "user_acceptance": true, "rollout": true}[in.Kind] || !map[string]bool{"passed": true, "failed": true, "revoked": true, "restored": true}[in.Status] || in.Cost < 0 || (in.Cost > 0 && in.Currency == "") {
			return ErrInvalid
		}
		if in.Status == "restored" && (!isOwner(*v, actor) || isAgent(*v, actor)) {
			return ErrForbidden
		}
		if in.Kind == "user_acceptance" && !isOwner(*v, actor) && !isAffectedUser(*v, actor) {
			return ErrForbidden
		}
		for ci := range v.Candidates {
			if v.Candidates[ci].ID == cid {
				for di := range v.Candidates[ci].Deliveries {
					d := &v.Candidates[ci].Deliveries[di]
					if d.ID == did {
						if in.ConsumerRevision != d.ConsumerRevision {
							return ErrInvalid
						}
						if in.Status == "restored" && d.Status != "paused" {
							return ErrInvalid
						}
						if in.Status == "restored" {
							_, baseBlockers := deriveDelivery(d.AdoptionDeliveryInput)
							failedObservation := false
							for _, prior := range d.Observations {
								failedObservation = failedObservation || prior.Status == "failed" || prior.Status == "revoked"
							}
							if len(baseBlockers) > 0 || !failedObservation {
								return ErrInvalid
							}
						}
						in.ID, in.AddedByID, in.CreatedAt = id("obs_"), actor, s.now().UTC()
						d.Observations = append(d.Observations, in)
						if in.Status == "failed" || in.Status == "revoked" {
							d.Status = "paused"
							d.Blockers = append(d.Blockers, in.Kind+": "+in.Summary)
						}
						if in.Status == "restored" {
							d.Status = "active"
							d.Blockers = []string{}
						}
						v.event("adoption_delivery."+in.Status, actor, in.ID)
						return nil
					}
				}
			}
		}
		return ErrNotFound
	})
}

func hasJourney(v Workspace, journey string) bool {
	for _, x := range v.RequiredJourneys {
		if x == journey {
			return true
		}
	}
	return false
}
func safeMap(v map[string]string) bool {
	for k, x := range v {
		joined := strings.ToLower(k + " " + x)
		if strings.TrimSpace(k) == "" || strings.TrimSpace(x) == "" || strings.Contains(joined, "password") || strings.Contains(joined, "secret") || strings.Contains(joined, "token") || strings.Contains(joined, "credential") {
			return false
		}
	}
	return true
}
func safeStrings(v []string) bool {
	for _, x := range v {
		lower := strings.ToLower(x)
		if strings.TrimSpace(x) == "" || strings.Contains(lower, "password=") || strings.Contains(lower, "secret=") || strings.Contains(lower, "token=") || strings.Contains(lower, "credential=") {
			return false
		}
	}
	return true
}
func (s *Store) AddTrial(wid, cid, actor string, in TrialInput) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !canRunTrial(*v, actor) || in.Name == "" || !map[string]bool{"release": true, "revision": true}[in.Source.Kind] || in.Source.Reference == "" || in.Source.Revision == "" || (in.Source.Kind == "release" && in.Source.Attestation == "") || len(in.JourneyIDs) == 0 || len(in.Policies) == 0 || len(in.Setup) == 0 || len(in.Commands) == 0 || len(in.Data) == 0 || in.Budget == "" || !safeMap(in.Configuration) || !safeStrings(in.Commands) || !trialAudiences[in.EvidenceAudience] || (!listOK(in.Packages, false) || !listOK(in.APIs, false)) || len(in.Packages)+len(in.APIs) == 0 {
			return ErrInvalid
		}
		for _, j := range in.JourneyIDs {
			if !hasJourney(*v, j) {
				return ErrInvalid
			}
		}
		for _, d := range in.Data {
			if !map[string]bool{"synthetic": true, "permitted": true}[d.Kind] || d.Description == "" || (d.Kind == "permitted" && d.Reference == "") {
				return ErrInvalid
			}
		}
		for i := range v.Candidates {
			if v.Candidates[i].ID == cid {
				if in.Source.Revision != v.Candidates[i].Revision {
					return ErrInvalid
				}
				now := s.now().UTC()
				t := Trial{ID: id("trl_"), TrialInput: in, Attempts: []TrialAttempt{}, Feedback: []TrialFeedback{}, Status: "not_run", Blockers: []string{"no attempt evidence"}, AddedByID: actor, CreatedAt: now}
				v.Candidates[i].Trials = append(v.Candidates[i].Trials, t)
				v.event("trial.assembled", actor, t.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func deriveAttempt(in TrialAttemptInput) (string, bool) {
	if len(in.Checks) == 0 {
		return "non_reproducible", false
	}
	for _, c := range in.Checks {
		if c.Name == "" || !map[string]bool{"passed": true, "failed": true, "blocked": true}[c.Status] {
			return "non_reproducible", false
		}
		if c.Status != "passed" {
			return "failed", false
		}
	}
	if len(in.Commands) == 0 || len(in.Artifacts) == 0 {
		return "non_reproducible", false
	}
	return "passed", true
}
func (s *Store) AddTrialAttempt(wid, cid, tid, actor string, in TrialAttemptInput) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !canRunTrial(*v, actor) || in.Environment == "" || in.SourceRevision == "" || !safeMap(in.Configuration) || !safeStrings(in.Commands) || in.Cost < 0 || (in.Cost > 0 && in.Currency == "") {
			return ErrInvalid
		}
		for ci := range v.Candidates {
			c := &v.Candidates[ci]
			if c.ID != cid {
				continue
			}
			for ti := range c.Trials {
				t := &c.Trials[ti]
				if t.ID != tid {
					continue
				}
				if in.SourceRevision != t.Source.Revision {
					return ErrInvalid
				}
				if in.ReproductionOf != "" {
					found := false
					for _, prior := range t.Attempts {
						found = found || prior.ID == in.ReproductionOf
					}
					if !found {
						return ErrInvalid
					}
				}
				status, repro := deriveAttempt(in)
				now := s.now().UTC()
				a := TrialAttempt{ID: id("att_"), TrialAttemptInput: in, Status: status, Reproducible: repro, AddedByID: actor, CreatedAt: now}
				t.Attempts = append(t.Attempts, a)
				t.Status = status
				t.Blockers = []string{}
				if !repro {
					t.Blockers = append(t.Blockers, "latest attempt is "+status)
				}
				v.event("trial.attempted", actor, a.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) AddTrialFeedback(wid, cid, tid, actor string, in TrialFeedback) (Workspace, error) {
	return s.mutate(wid, actor, false, func(v *Workspace) error {
		if !canRunTrial(*v, actor) || isAgent(*v, actor) || in.AttemptID == "" || !hasJourney(*v, in.JourneyID) || !map[string]bool{"meets": true, "does_not_meet": true, "uncertain": true}[in.Verdict] || in.Comment == "" {
			return ErrInvalid
		}
		for ci := range v.Candidates {
			if v.Candidates[ci].ID != cid {
				continue
			}
			for ti := range v.Candidates[ci].Trials {
				t := &v.Candidates[ci].Trials[ti]
				if t.ID != tid {
					continue
				}
				found := false
				for _, a := range t.Attempts {
					found = found || a.ID == in.AttemptID
				}
				if !found {
					return ErrInvalid
				}
				now := s.now().UTC()
				in.ID = id("fbk_")
				in.AddedByID = actor
				in.CreatedAt = now
				t.Feedback = append(t.Feedback, in)
				v.event("trial.feedback_added", actor, in.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) AddEvidence(wid, cid, actor string, e Evidence) (Workspace, error) {
	return s.mutate(wid, actor, true, func(v *Workspace) error {
		if !dimensions[e.Dimension] || e.Claim == "" || e.Revision == "" || !map[string]bool{"public": true, "shared": true, "provider": true}[e.Visibility] || !map[string]bool{"available": true, "unavailable": true}[e.Availability] || (e.Availability == "available" && e.Reference == "") {
			return ErrInvalid
		}
		for i := range v.Candidates {
			if v.Candidates[i].ID == cid {
				e.ID = id("evd_")
				e.AddedByID = actor
				e.CreatedAt = s.now().UTC()
				e.Status = "current"
				e.ProofOfFit = true
				v.Candidates[i].Evidence = append(v.Candidates[i].Evidence, e)
				v.event("evidence.added", actor, e.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) List(actor string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return nil, e
	}
	out := []Workspace{}
	for _, v := range all {
		if canRead(v, actor) {
			out = append(out, s.project(v, actor))
		}
	}
	return out, nil
}
func (s *Store) Get(wid, actor string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(wid)
	if e == nil && !canRead(v, actor) {
		return Workspace{}, ErrNotFound
	}
	if e == nil {
		v = s.project(v, actor)
	}
	return v, e
}
func (s *Store) project(v Workspace, actor string) Workspace {
	provider := false
	participant := false
	for _, p := range v.Participants {
		if p.SubjectID == actor && p.Consent == "accepted" {
			participant = true
			if p.EvidenceAccess == "provider" {
				provider = true
			}
		}
	}
	now := s.now().UTC()
	for ci := range v.Candidates {
		c := &v.Candidates[ci]
		c.Coverage = map[string]string{}
		c.Blockers = []string{}
		for d := range dimensions {
			c.Coverage[d] = "missing"
		}
		for ei := range c.Evidence {
			e := &c.Evidence[ei]
			e.Status = "current"
			e.ProofOfFit = true
			e.Gap = ""
			if e.Dimension == "gap" {
				e.Status = "known_gap"
				e.ProofOfFit = false
				e.Gap = e.Claim
			} else if e.Availability == "unavailable" {
				e.Status = "unavailable"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "evidence unavailable"
			} else if e.Revision != c.Revision {
				e.Status = "stale"
				e.ProofOfFit = false
				e.Gap = "evidence is for a different candidate revision"
			} else if e.ValidUntil != nil && !e.ValidUntil.After(now) {
				e.Status = "expired"
				e.ProofOfFit = false
				e.Gap = "evidence validity expired"
			} else if e.Visibility == "shared" && !participant {
				e.Status = "inaccessible"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "workspace evidence is not accessible to this viewer"
			} else if e.Visibility == "provider" && !provider {
				e.Status = "inaccessible"
				e.ProofOfFit = false
				e.Reference = ""
				e.Gap = "provider evidence is not accessible to this viewer"
			}
			if e.ProofOfFit {
				c.Coverage[e.Dimension] = "supported"
			} else if c.Coverage[e.Dimension] != "supported" {
				c.Coverage[e.Dimension] = e.Status
			}
			if !e.ProofOfFit {
				c.Blockers = append(c.Blockers, e.Dimension+": "+e.Gap)
			}
		}
		for d, status := range c.Coverage {
			if status == "missing" {
				c.Blockers = append(c.Blockers, d+": no evidence")
			}
		}
		for ti := range c.Trials {
			t := &c.Trials[ti]
			accessible := t.EvidenceAudience == "public" || (t.EvidenceAudience == "shared" && participant) || (t.EvidenceAudience == "provider" && provider) || (t.EvidenceAudience == "consumer" && ((participant && !provider) || isOwner(v, actor)))
			if !accessible {
				t.Setup = nil
				t.Configuration = nil
				t.Commands = nil
				t.Data = nil
				t.Attempts = nil
				t.Feedback = nil
				t.Status = "inaccessible"
				t.Blockers = []string{"trial evidence is not accessible to this viewer"}
			}
		}
		sort.Strings(c.Blockers)
	}
	return v
}
func (s *Store) mutate(wid, actor string, contribute bool, fn func(*Workspace) error) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(wid)
	if e != nil {
		return v, e
	}
	if !canRead(v, actor) || (contribute && !canContribute(v, actor)) {
		return Workspace{}, ErrForbidden
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (v *Workspace) event(t, a, subject string) {
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: t, ActorID: a, SubjectID: subject, CreatedAt: time.Now().UTC()})
}
func (s *Store) path(wid string) string { return filepath.Join(s.root, wid+".json") }
func (s *Store) write(v Workspace) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, s.path(v.ID))
	}
	return e
}
func (s *Store) read(wid string) (Workspace, error) {
	var v Workspace
	b, e := os.ReadFile(s.path(wid))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) list() ([]Workspace, error) {
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Workspace{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
