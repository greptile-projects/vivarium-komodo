// Package repositoryrestructuring retains revision-exact, reviewable plans for
// changing project repository boundaries before any repository identity moves.
package repositoryrestructuring

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("repository restructuring plan not found")
var ErrInvalid = errors.New("invalid repository restructuring plan")
var ErrForbidden = errors.New("repository restructuring action forbidden")
var ErrConflict = errors.New("repository restructuring revision conflict")

var resourceKinds = map[string]bool{"ref": true, "pull_request": true, "issue": true, "task": true, "release": true, "package": true, "documentation": true, "policy": true, "workspace": true, "automation": true, "consumer": true, "federated_relationship": true}
var dispositions = map[string]bool{"move": true, "remain": true, "copy": true, "split": true, "redirect": true, "retire": true, "unresolved": true}
var accessStates = map[string]bool{"accessible": true, "inaccessible": true, "ambiguous": true, "shared": true}
var historyModes = map[string]bool{"full": true, "path_history": true, "selected_commits": true, "squash": true, "none": true}
var preservationStates = map[string]bool{"preserved": true, "changed": true, "missing": true, "not_applicable": true}
var rehearsalDomains = map[string]bool{"git_clone": true, "git_fetch": true, "git_push": true, "build": true, "checks": true, "package_resolution": true, "api_resolution": true, "documentation": true, "workspaces": true, "consumer_journey": true}
var resultStates = map[string]bool{"passed": true, "failed": true, "blocked": true, "not_run": true}
var workKinds = map[string]bool{"branch": true, "pull_request": true, "issue": true, "proposal": true, "task": true, "decision": true, "check": true, "session": true, "workspace": true, "queue": true}
var workOutcomes = map[string]bool{"continued": true, "blocked": true, "archived": true}
var migrationKinds = map[string]bool{"clone": true, "fork": true, "package": true, "api": true, "dependency": true, "extension": true, "workflow": true, "documentation": true, "deployment": true, "federated_follower": true}
var migrationStates = map[string]bool{"planned": true, "redirect_ready": true, "propagating": true, "adopted": true, "blocked": true, "rejected": true, "unavailable": true, "unmigrated": true}
var cutoverStageKinds = map[string]bool{"pause_writes": true, "activate_destinations": true, "transfer_ownership_policies": true, "publish_refs_redirects": true, "verify_topology": true, "retire_sources": true}
var cutoverSignalKinds = map[string]bool{"source_health": true, "destination_health": true, "dependency_adoption": true, "git_traffic": true, "collaboration_migration": true, "build": true, "release": true, "permissions": true, "links": true, "supported_consumers": true, "ordinary_contribution": true, "late_write": true, "residual_use": true}
var verificationSignalKinds = []string{"build", "release", "permissions", "links", "supported_consumers", "ordinary_contribution"}

type Source struct {
	RepositoryID string   `json:"repository_id"`
	Revision     string   `json:"revision"`
	OwnerIDs     []string `json:"owner_ids"`
	Role         string   `json:"role"`
}

type Destination struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	OwnerIDs           []string `json:"owner_ids"`
	Visibility         string   `json:"visibility"`
	DefaultBranch      string   `json:"default_branch"`
	RetainedIdentities []string `json:"retained_identities,omitempty"`
}

type Mapping struct {
	ID                 string   `json:"id"`
	SourceRepositoryID string   `json:"source_repository_id"`
	SourceRevision     string   `json:"source_revision"`
	SourcePaths        []string `json:"source_paths"`
	DestinationID      string   `json:"destination_id,omitempty"`
	DestinationPaths   []string `json:"destination_paths,omitempty"`
	HistoryMode        string   `json:"history_mode"`
	IncludeRefs        []string `json:"include_refs,omitempty"`
	ExcludeRefs        []string `json:"exclude_refs,omitempty"`
	Disposition        string   `json:"disposition"`
	Rationale          string   `json:"rationale"`
}

type InventoryItem struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	RepositoryID   string   `json:"repository_id"`
	Reference      string   `json:"reference"`
	Revision       string   `json:"revision"`
	OwnerIDs       []string `json:"owner_ids,omitempty"`
	Access         string   `json:"access"`
	Disposition    string   `json:"disposition"`
	DestinationIDs []string `json:"destination_ids,omitempty"`
	SharedWith     []string `json:"shared_with,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type RollbackLimits struct {
	LatestTime         time.Time `json:"latest_time"`
	IrreversibleAfter  string    `json:"irreversible_after"`
	MaximumDataLoss    string    `json:"maximum_data_loss"`
	RequiredRetentions []string  `json:"required_retentions"`
}

type Input struct {
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	Sources         []Source        `json:"sources"`
	Destinations    []Destination   `json:"destinations"`
	Mappings        []Mapping       `json:"mappings"`
	Inventory       []InventoryItem `json:"inventory"`
	Deadline        time.Time       `json:"deadline"`
	SuccessCriteria []string        `json:"success_criteria"`
	RollbackLimits  RollbackLimits  `json:"rollback_limits"`
}

type Citation struct {
	RepositoryID string `json:"repository_id"`
	Reference    string `json:"reference"`
	Revision     string `json:"revision"`
	Path         string `json:"path,omitempty"`
}

type FindingInput struct {
	ActorKind        string     `json:"actor_kind"`
	Summary          string     `json:"summary"`
	Impact           string     `json:"impact"`
	AffectedItemIDs  []string   `json:"affected_item_ids"`
	AffectedOwnerIDs []string   `json:"affected_owner_ids,omitempty"`
	Uncertainty      string     `json:"uncertainty,omitempty"`
	Citations        []Citation `json:"citations"`
}

type Finding struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id"`
	FindingInput
	CreatedAt time.Time `json:"created_at"`
}

type PreservationEvidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Source    string `json:"source,omitempty"`
	Candidate string `json:"candidate,omitempty"`
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Detail    string `json:"detail"`
}

type CandidateRepository struct {
	DestinationID string                 `json:"destination_id"`
	ObjectDigest  string                 `json:"object_digest"`
	DefaultRef    string                 `json:"default_ref"`
	DefaultCommit string                 `json:"default_commit"`
	ObjectCount   int                    `json:"object_count"`
	SizeBytes     int64                  `json:"size_bytes"`
	Evidence      []PreservationEvidence `json:"evidence"`
}

type CandidateInput struct {
	MappingIDs           []string               `json:"mapping_ids"`
	Repositories         []CandidateRepository  `json:"repositories"`
	CrossRepositoryLinks []PreservationEvidence `json:"cross_repository_links"`
	Issues               []PreservationEvidence `json:"issues"`
	AssemblyCost         int64                  `json:"assembly_cost"`
	RequiredDecisions    []string               `json:"required_decisions"`
}

type Candidate struct {
	ID string `json:"id"`
	CandidateInput
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
	AuthorityGranted []string  `json:"authority_granted"`
}

type RehearsalCheck struct {
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	Command   string `json:"command"`
	Reference string `json:"reference"`
	Digest    string `json:"digest,omitempty"`
	Summary   string `json:"summary"`
	Cost      int64  `json:"cost"`
}

type RehearsalInput struct {
	CandidateID       string                 `json:"candidate_id"`
	Environment       string                 `json:"environment"`
	Budget            int64                  `json:"budget"`
	ObservedCost      int64                  `json:"observed_cost"`
	Checks            []RehearsalCheck       `json:"checks"`
	Issues            []PreservationEvidence `json:"issues"`
	RequiredDecisions []string               `json:"required_decisions"`
}

type Rehearsal struct {
	ID string `json:"id"`
	RehearsalInput
	Status           string    `json:"status"`
	Blockers         []Blocker `json:"blockers"`
	RecordedBy       string    `json:"recorded_by"`
	RecordedAt       time.Time `json:"recorded_at"`
	AuthorityGranted []string  `json:"authority_granted"`
}

// WorkMapping carries an open collaboration object across one or more new
// boundaries without treating the restructuring owner as its owner.
type WorkMappingInput struct {
	InventoryItemID    string            `json:"inventory_item_id"`
	SourceRevision     string            `json:"source_revision"`
	Kind               string            `json:"kind"`
	Authorship         []string          `json:"authorship"`
	Discussion         []string          `json:"discussion"`
	Reviews            []WorkReview      `json:"reviews"`
	Dependencies       []string          `json:"dependencies"`
	AcceptanceCriteria []string          `json:"acceptance_criteria"`
	ContextAudience    string            `json:"context_audience"`
	Destinations       []WorkDestination `json:"destinations"`
}

type WorkReview struct {
	ActorID   string `json:"actor_id"`
	Revision  string `json:"revision"`
	Decision  string `json:"decision"`
	Reference string `json:"reference"`
}
type WorkDestination struct {
	DestinationID  string   `json:"destination_id"`
	Kind           string   `json:"kind"`
	Reference      string   `json:"reference"`
	Revision       string   `json:"revision"`
	ContributionID string   `json:"contribution_id,omitempty"`
	DependsOn      []string `json:"depends_on,omitempty"`
}
type WorkDecision struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Revision  int64     `json:"revision"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
type WorkOutcome struct {
	DestinationID string    `json:"destination_id"`
	ActorID       string    `json:"actor_id"`
	Status        string    `json:"status"`
	Revision      string    `json:"revision,omitempty"`
	Reference     string    `json:"reference,omitempty"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}
type WorkMapping struct {
	ID string `json:"id"`
	WorkMappingInput
	Version          int64          `json:"version"`
	Status           string         `json:"status"`
	Blockers         []Blocker      `json:"blockers"`
	Decisions        []WorkDecision `json:"decisions"`
	Outcomes         []WorkOutcome  `json:"outcomes"`
	CreatedBy        string         `json:"created_by"`
	CreatedAt        time.Time      `json:"created_at"`
	AuthorityGranted []string       `json:"authority_granted"`
}

// MigrationTarget is one independently governed downstream use of an old
// location. Redirect and mapping metadata is descriptive; only its owner can
// report propagation or adoption.
type MigrationTarget struct {
	ID                  string            `json:"id"`
	Kind                string            `json:"kind"`
	OwnerIDs            []string          `json:"owner_ids"`
	Audience            string            `json:"audience"`
	CurrentLocation     string            `json:"current_location"`
	ReplacementLocation string            `json:"replacement_location"`
	ReplacementRemote   string            `json:"replacement_remote,omitempty"`
	RedirectSignature   string            `json:"redirect_signature,omitempty"`
	Mappings            map[string]string `json:"mappings"`
	Synchronization     []string          `json:"synchronization"`
	CompatibilityUntil  time.Time         `json:"compatibility_until"`
	State               string            `json:"state"`
	NextAction          string            `json:"next_action"`
	CredentialReference string            `json:"credential_reference,omitempty"`
	CredentialExpiresAt time.Time         `json:"credential_expires_at,omitempty"`
}

type MigrationPlanInput struct {
	CandidateID string            `json:"candidate_id"`
	Revision    string            `json:"revision"`
	Targets     []MigrationTarget `json:"targets"`
}

type MigrationEvent struct {
	ID                   string            `json:"id"`
	TargetID             string            `json:"target_id"`
	ActorID              string            `json:"actor_id"`
	State                string            `json:"state"`
	Revision             string            `json:"revision,omitempty"`
	PullRequestReference string            `json:"pull_request_reference,omitempty"`
	ReleaseReference     string            `json:"release_reference,omitempty"`
	Evidence             map[string]string `json:"evidence,omitempty"`
	NextAction           string            `json:"next_action"`
	CreatedAt            time.Time         `json:"created_at"`
}

type MigrationPlan struct {
	ID string `json:"id"`
	MigrationPlanInput
	Events           []MigrationEvent `json:"events"`
	Blockers         []Blocker        `json:"blockers"`
	CreatedBy        string           `json:"created_by"`
	CreatedAt        time.Time        `json:"created_at"`
	AuthorityGranted []string         `json:"authority_granted"`
}

// Cutover is an owner-controlled, optimistic execution record. It coordinates
// already-authorized repository operations but never grants that authority.
type CutoverStage struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	OwnerIDs       []string  `json:"owner_ids"`
	DependsOn      []string  `json:"depends_on,omitempty"`
	AtomicGroup    string    `json:"atomic_group,omitempty"`
	SourceIDs      []string  `json:"source_ids,omitempty"`
	DestinationIDs []string  `json:"destination_ids,omitempty"`
	State          string    `json:"state"`
	Summary        string    `json:"summary,omitempty"`
	Evidence       []string  `json:"evidence,omitempty"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	CompletedAt    time.Time `json:"completed_at,omitempty"`
}

type CutoverInput struct {
	CandidateID       string            `json:"candidate_id"`
	RehearsalID       string            `json:"rehearsal_id"`
	MigrationPlanID   string            `json:"migration_plan_id"`
	SourceRevisions   map[string]string `json:"source_revisions"`
	RequiredOwnerIDs  []string          `json:"required_owner_ids"`
	WriteBoundary     string            `json:"write_boundary"`
	Stages            []CutoverStage    `json:"stages"`
	SourceDisposition string            `json:"source_disposition"`
}

type CutoverApproval struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason"`
	Revision  int64     `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
}
type CutoverSignal struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Status     string    `json:"status"`
	Revision   string    `json:"revision,omitempty"`
	Value      int64     `json:"value,omitempty"`
	Summary    string    `json:"summary"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type CutoverControl struct {
	ActorID   string    `json:"actor_id"`
	Kind      string    `json:"kind"`
	Reason    string    `json:"reason"`
	Revision  int64     `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
}
type Cutover struct {
	ID string `json:"id"`
	CutoverInput
	Version          int64             `json:"version"`
	State            string            `json:"state"`
	ActiveStageID    string            `json:"active_stage_id,omitempty"`
	Approvals        []CutoverApproval `json:"approvals"`
	Signals          []CutoverSignal   `json:"signals"`
	Controls         []CutoverControl  `json:"controls"`
	Blockers         []Blocker         `json:"blockers"`
	RollbackOptions  []string          `json:"rollback_options"`
	CreatedBy        string            `json:"created_by"`
	CreatedAt        time.Time         `json:"created_at"`
	AuthorityGranted []string          `json:"authority_granted"`
}

type Blocker struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Detail     string `json:"detail"`
}

type Plan struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatorID    string `json:"creator_id"`
	Input
	Findings         []Finding       `json:"findings"`
	Candidates       []Candidate     `json:"candidates"`
	Rehearsals       []Rehearsal     `json:"rehearsals"`
	WorkMappings     []WorkMapping   `json:"work_mappings"`
	MigrationPlans   []MigrationPlan `json:"migration_plans"`
	Cutovers         []Cutover       `json:"cutovers"`
	Blockers         []Blocker       `json:"blockers"`
	AuthorityGranted []string        `json:"authority_granted"`
	CreatedAt        time.Time       `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Create(repositoryID, actorID string, in Input) (*Plan, error) {
	if !valid(in) {
		return nil, ErrInvalid
	}
	p := &Plan{ID: id("rsp"), RepositoryID: repositoryID, CreatorID: actorID, Input: in, Findings: []Finding{}, Candidates: []Candidate{}, Rehearsals: []Rehearsal{}, WorkMappings: []WorkMapping{}, MigrationPlans: []MigrationPlan{}, Cutovers: []Cutover{}, AuthorityGranted: []string{}, CreatedAt: time.Now().UTC()}
	p.Blockers = blockers(in.Inventory)
	if err := s.write(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) AddMigrationPlan(repositoryID, planID, actorID string, in MigrationPlanInput) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	if in.Revision == "" || len(in.Targets) == 0 || !candidateExists(*p, in.CandidateID) {
		return nil, ErrInvalid
	}
	seen, locations := map[string]bool{}, map[string]string{}
	redirects := map[string]string{}
	blockers := []Blocker{}
	for i := range in.Targets {
		t := &in.Targets[i]
		if t.ID == "" || seen[t.ID] || !migrationKinds[t.Kind] || len(t.OwnerIDs) == 0 || t.CurrentLocation == "" || t.ReplacementLocation == "" || len(t.Mappings) == 0 || len(t.Synchronization) == 0 || t.CompatibilityUntil.IsZero() || !migrationStates[t.State] || t.NextAction == "" || (t.Audience != "public" && t.Audience != "repository" && t.Audience != "owner") {
			return nil, ErrInvalid
		}
		seen[t.ID] = true
		if t.CurrentLocation == t.ReplacementLocation {
			blockers = append(blockers, Blocker{Kind: "redirect_loop", ResourceID: t.ID, Detail: "replacement resolves to the current location"})
		} else {
			redirects[t.CurrentLocation] = t.ReplacementLocation
		}
		if prior := locations[t.ReplacementLocation]; prior != "" {
			blockers = append(blockers, Blocker{Kind: "namespace_collision", ResourceID: t.ID, Detail: "replacement location is also claimed by " + prior})
		} else {
			locations[t.ReplacementLocation] = t.ID
		}
		if (t.Kind == "clone" || t.Kind == "fork") && t.RedirectSignature == "" && t.ReplacementRemote == "" {
			return nil, ErrInvalid
		}
		if !t.CredentialExpiresAt.IsZero() && !t.CredentialExpiresAt.After(time.Now().UTC()) {
			blockers = append(blockers, Blocker{Kind: "stale_credential", ResourceID: t.ID, Detail: "propagation credential reference is expired"})
		}
		if t.State == "unavailable" || t.State == "rejected" || t.State == "unmigrated" || t.State == "blocked" {
			blockers = append(blockers, Blocker{Kind: t.State, ResourceID: t.ID, Detail: t.NextAction})
		}
	}
	for _, t := range in.Targets {
		if t.CurrentLocation == t.ReplacementLocation {
			continue
		}
		visited := map[string]bool{t.CurrentLocation: true}
		for at := t.ReplacementLocation; redirects[at] != ""; at = redirects[at] {
			if visited[at] {
				blockers = append(blockers, Blocker{Kind: "redirect_loop", ResourceID: t.ID, Detail: "replacement locations form a redirect loop"})
				break
			}
			visited[at] = true
		}
	}
	p.MigrationPlans = append(p.MigrationPlans, MigrationPlan{ID: id("mig"), MigrationPlanInput: in, Events: []MigrationEvent{}, Blockers: blockers, CreatedBy: actorID, CreatedAt: time.Now().UTC(), AuthorityGranted: []string{}})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) RecordMigrationEvent(repositoryID, planID, migrationID, actorID string, in MigrationEvent) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	for mi := range p.MigrationPlans {
		m := &p.MigrationPlans[mi]
		if m.ID != migrationID {
			continue
		}
		for ti := range m.Targets {
			t := &m.Targets[ti]
			if t.ID != in.TargetID {
				continue
			}
			if !contains(t.OwnerIDs, actorID) {
				return nil, ErrForbidden
			}
			if !migrationStates[in.State] || in.NextAction == "" {
				return nil, ErrInvalid
			}
			if in.State == "adopted" && (in.Revision == "" || (in.PullRequestReference == "" && in.ReleaseReference == "")) {
				return nil, ErrInvalid
			}
			in.ID = id("evt")
			in.ActorID = actorID
			in.CreatedAt = time.Now().UTC()
			t.State = in.State
			t.NextAction = in.NextAction
			m.Events = append(m.Events, in)
			filtered := m.Blockers[:0]
			for _, b := range m.Blockers {
				if b.ResourceID != t.ID {
					filtered = append(filtered, b)
				}
			}
			m.Blockers = filtered
			if in.State == "blocked" || in.State == "rejected" || in.State == "unavailable" || in.State == "unmigrated" {
				m.Blockers = append(m.Blockers, Blocker{Kind: in.State, ResourceID: t.ID, Detail: in.NextAction})
			}
			if err = s.writeUnlocked(p); err != nil {
				return nil, err
			}
			return p, nil
		}
		return nil, ErrInvalid
	}
	return nil, ErrNotFound
}

func candidateExists(p Plan, id string) bool {
	for _, c := range p.Candidates {
		if c.ID == id {
			return true
		}
	}
	return false
}

func (s *Store) AddCutover(repositoryID, planID, actorID string, in CutoverInput) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	if !validCutover(*p, in) {
		return nil, ErrInvalid
	}
	bs := cutoverPrerequisiteBlockers(*p, in)
	state := "awaiting_approvals"
	if len(bs) > 0 {
		state = "blocked"
	}
	for i := range in.Stages {
		in.Stages[i].State = "pending"
		in.Stages[i].Summary = ""
		in.Stages[i].Evidence = nil
	}
	c := Cutover{ID: id("cut"), CutoverInput: in, Version: 1, State: state, Approvals: []CutoverApproval{}, Signals: []CutoverSignal{}, Controls: []CutoverControl{}, Blockers: bs, RollbackOptions: []string{"resume source writes before authority publication", "restore prior refs, redirects, ownership, and policies before source retirement"}, CreatedBy: actorID, CreatedAt: time.Now().UTC(), AuthorityGranted: []string{}}
	p.Cutovers = append(p.Cutovers, c)
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) DecideCutover(repositoryID, planID, cutoverID, actorID, decision, reason string, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	c := cutoverByID(p, cutoverID)
	if c == nil {
		return nil, ErrNotFound
	}
	if expected != c.Version {
		return nil, ErrConflict
	}
	if !contains(c.RequiredOwnerIDs, actorID) {
		return nil, ErrForbidden
	}
	if (decision != "approved" && decision != "rejected") || strings.TrimSpace(reason) == "" {
		return nil, ErrInvalid
	}
	c.Version++
	c.Approvals = append(c.Approvals, CutoverApproval{ActorID: actorID, Decision: decision, Reason: reason, Revision: c.Version, CreatedAt: time.Now().UTC()})
	c.Blockers = cutoverPrerequisiteBlockers(*p, c.CutoverInput)
	if decision == "rejected" {
		c.Blockers = append(c.Blockers, Blocker{Kind: "owner_rejected", ResourceID: actorID, Detail: reason})
		c.State = "blocked"
	} else if allCutoverOwnersApproved(c.RequiredOwnerIDs, c.Approvals) && len(c.Blockers) == 0 {
		c.State = "ready"
	}
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) ControlCutover(repositoryID, planID, cutoverID, actorID, kind, reason string, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	c := cutoverByID(p, cutoverID)
	if c == nil {
		return nil, ErrNotFound
	}
	if expected != c.Version {
		return nil, ErrConflict
	}
	if !contains(c.RequiredOwnerIDs, actorID) {
		return nil, ErrForbidden
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrInvalid
	}
	switch kind {
	case "start":
		if c.State != "ready" || !allCutoverOwnersApproved(c.RequiredOwnerIDs, c.Approvals) {
			return nil, ErrInvalid
		}
		c.State = "running"
	case "pause":
		if c.State != "running" {
			return nil, ErrInvalid
		}
		c.State = "paused"
	case "resume":
		if c.State != "paused" {
			return nil, ErrInvalid
		}
		c.State = "running"
	case "rollback":
		if c.State != "running" && c.State != "paused" && c.State != "failed" {
			return nil, ErrInvalid
		}
		if stageSucceeded(c.Stages, "retire_sources") {
			return nil, ErrInvalid
		}
		c.State = "rolled_back"
		c.ActiveStageID = ""
		for i := range c.Stages {
			if c.Stages[i].State == "active" || c.Stages[i].State == "succeeded" {
				c.Stages[i].State = "rolled_back"
			}
		}
	default:
		return nil, ErrInvalid
	}
	c.Version++
	c.Controls = append(c.Controls, CutoverControl{ActorID: actorID, Kind: kind, Reason: reason, Revision: c.Version, CreatedAt: time.Now().UTC()})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) UpdateCutoverStage(repositoryID, planID, cutoverID, stageID, actorID, state, summary string, evidence []string, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	c := cutoverByID(p, cutoverID)
	if c == nil {
		return nil, ErrNotFound
	}
	if expected != c.Version {
		return nil, ErrConflict
	}
	if c.State != "running" || strings.TrimSpace(summary) == "" || (state != "active" && state != "succeeded" && state != "failed") {
		return nil, ErrInvalid
	}
	for i := range c.Stages {
		stage := &c.Stages[i]
		if stage.ID != stageID {
			continue
		}
		if !contains(stage.OwnerIDs, actorID) {
			return nil, ErrForbidden
		}
		if state == "active" && (stage.State != "pending" || !dependenciesSucceeded(c.Stages, stage.DependsOn)) {
			return nil, ErrInvalid
		}
		if (state == "succeeded" || state == "failed") && stage.State != "active" {
			return nil, ErrInvalid
		}
		if state == "succeeded" && len(evidence) == 0 {
			return nil, ErrInvalid
		}
		if stage.Kind == "retire_sources" && state == "active" {
			c.Blockers = cutoverCompletionBlockers(*p, *c)
			if len(c.Blockers) > 0 {
				return nil, ErrInvalid
			}
		}
		now := time.Now().UTC()
		stage.State = state
		stage.Summary = summary
		stage.Evidence = evidence
		if state == "active" {
			stage.StartedAt = now
			c.ActiveStageID = stage.ID
		} else {
			stage.CompletedAt = now
			c.ActiveStageID = ""
		}
		if state == "failed" {
			c.State = "failed"
			c.Blockers = append(c.Blockers, Blocker{Kind: "stage_failed", ResourceID: stage.ID, Detail: summary})
		}
		if state == "succeeded" && stage.Kind == "retire_sources" {
			c.State = "completed"
		}
		c.Version++
		if err = s.writeUnlocked(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, ErrNotFound
}

func (s *Store) RecordCutoverSignal(repositoryID, planID, cutoverID, actorID string, in CutoverSignal, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	c := cutoverByID(p, cutoverID)
	if c == nil {
		return nil, ErrNotFound
	}
	if expected != c.Version || !cutoverSignalKinds[in.Kind] || in.ResourceID == "" || in.Status == "" || strings.TrimSpace(in.Summary) == "" {
		return nil, ErrInvalid
	}
	if !isPlanOwner(*p, actorID) {
		return nil, ErrForbidden
	}
	in.ID = id("sig")
	in.ActorID = actorID
	in.CreatedAt = time.Now().UTC()
	c.Signals = append(c.Signals, in)
	c.Version++
	c.Blockers = cutoverCompletionBlockers(*p, *c)
	if (in.Kind == "late_write" || in.Kind == "residual_use") && (in.Value > 0 || in.Status == "failed") {
		c.State = "paused"
		c.ActiveStageID = ""
	}
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func validCutover(p Plan, in CutoverInput) bool {
	if in.CandidateID == "" || in.RehearsalID == "" || in.MigrationPlanID == "" || len(in.SourceRevisions) != len(p.Sources) || len(in.RequiredOwnerIDs) == 0 || in.WriteBoundary == "" || len(in.Stages) == 0 || (in.SourceDisposition != "read_only" && in.SourceDisposition != "archived" && in.SourceDisposition != "removed") {
		return false
	}
	ids, kinds := map[string]bool{}, map[string]bool{}
	knownOwners, knownSources, knownDestinations := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, source := range p.Sources {
		knownSources[source.RepositoryID] = true
		for _, owner := range source.OwnerIDs {
			knownOwners[owner] = true
			if !contains(in.RequiredOwnerIDs, owner) {
				return false
			}
		}
	}
	for _, destination := range p.Destinations {
		knownDestinations[destination.ID] = true
		for _, owner := range destination.OwnerIDs {
			knownOwners[owner] = true
			if !contains(in.RequiredOwnerIDs, owner) {
				return false
			}
		}
	}
	for _, owner := range in.RequiredOwnerIDs {
		if !knownOwners[owner] {
			return false
		}
	}
	for _, s := range in.Stages {
		if s.ID == "" || ids[s.ID] || !cutoverStageKinds[s.Kind] || len(s.OwnerIDs) == 0 || (s.Kind == "publish_refs_redirects" && s.AtomicGroup == "") {
			return false
		}
		for _, owner := range s.OwnerIDs {
			if !knownOwners[owner] {
				return false
			}
		}
		for _, source := range s.SourceIDs {
			if !knownSources[source] {
				return false
			}
		}
		for _, destination := range s.DestinationIDs {
			if !knownDestinations[destination] {
				return false
			}
		}
		ids[s.ID] = true
		kinds[s.Kind] = true
	}
	for _, s := range in.Stages {
		for _, d := range s.DependsOn {
			if !ids[d] || d == s.ID {
				return false
			}
		}
	}
	for _, s := range in.Stages {
		if cutoverDependencyCycle(in.Stages, s.ID, map[string]bool{}) {
			return false
		}
	}
	for k := range cutoverStageKinds {
		if !kinds[k] {
			return false
		}
	}
	return true
}
func cutoverDependencyCycle(stages []CutoverStage, id string, path map[string]bool) bool {
	if path[id] {
		return true
	}
	next := map[string]bool{}
	for k, v := range path {
		next[k] = v
	}
	next[id] = true
	for _, s := range stages {
		if s.ID == id {
			for _, d := range s.DependsOn {
				if cutoverDependencyCycle(stages, d, next) {
					return true
				}
			}
		}
	}
	return false
}

func cutoverPrerequisiteBlockers(p Plan, in CutoverInput) []Blocker {
	bs := []Blocker{}
	for _, s := range p.Sources {
		if in.SourceRevisions[s.RepositoryID] != s.Revision {
			bs = append(bs, Blocker{Kind: "stale_source", ResourceID: s.RepositoryID, Detail: "cutover does not bind the planned source revision"})
		}
	}
	found := false
	for _, r := range p.Rehearsals {
		if r.ID == in.RehearsalID && r.CandidateID == in.CandidateID {
			found = true
			if r.Status != "passed" {
				bs = append(bs, Blocker{Kind: "rehearsal_not_passing", ResourceID: r.ID, Detail: "current candidate rehearsal is blocked"})
			}
		}
	}
	if !found {
		bs = append(bs, Blocker{Kind: "missing_rehearsal", ResourceID: in.RehearsalID, Detail: "candidate-bound rehearsal was not found"})
	}
	mf := false
	for _, m := range p.MigrationPlans {
		if m.ID == in.MigrationPlanID && m.CandidateID == in.CandidateID {
			mf = true
		}
	}
	if !mf {
		bs = append(bs, Blocker{Kind: "missing_migration_plan", ResourceID: in.MigrationPlanID, Detail: "candidate-bound downstream plan was not found"})
	}
	return bs
}
func cutoverCompletionBlockers(p Plan, c Cutover) []Blocker {
	bs := cutoverPrerequisiteBlockers(p, c.CutoverInput)
	for _, k := range verificationSignalKinds {
		if !latestSignalPassed(c.Signals, k) {
			bs = append(bs, Blocker{Kind: "missing_verification", ResourceID: k, Detail: "current passing topology verification is required"})
		}
	}
	for i, s := range c.Signals {
		if (s.Kind == "late_write" || s.Kind == "residual_use") && signalIsLatest(c.Signals, i) && (s.Value > 0 || s.Status == "failed") {
			bs = append(bs, Blocker{Kind: s.Kind, ResourceID: s.ResourceID, Detail: s.Summary})
		}
	}
	for _, m := range p.MigrationPlans {
		if m.ID == c.MigrationPlanID {
			for _, t := range m.Targets {
				if t.State != "adopted" {
					bs = append(bs, Blocker{Kind: "residual_use", ResourceID: t.ID, Detail: "downstream target has not adopted the new topology"})
				}
			}
		}
	}
	for _, w := range p.WorkMappings {
		if w.Status != "continued" && w.Status != "archived" {
			bs = append(bs, Blocker{Kind: "collaboration_not_migrated", ResourceID: w.ID, Detail: "open work has not reached its declared destination outcome"})
		}
	}
	return bs
}
func signalIsLatest(signals []CutoverSignal, at int) bool {
	for i := at + 1; i < len(signals); i++ {
		if signals[i].Kind == signals[at].Kind && signals[i].ResourceID == signals[at].ResourceID {
			return false
		}
	}
	return true
}
func cutoverByID(p *Plan, id string) *Cutover {
	for i := range p.Cutovers {
		if p.Cutovers[i].ID == id {
			return &p.Cutovers[i]
		}
	}
	return nil
}
func allCutoverOwnersApproved(owners []string, ds []CutoverApproval) bool {
	for _, o := range owners {
		ok := false
		for i := len(ds) - 1; i >= 0; i-- {
			if ds[i].ActorID == o {
				ok = ds[i].Decision == "approved"
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func dependenciesSucceeded(stages []CutoverStage, ids []string) bool {
	for _, id := range ids {
		ok := false
		for _, s := range stages {
			if s.ID == id && s.State == "succeeded" {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func stageSucceeded(stages []CutoverStage, kind string) bool {
	for _, s := range stages {
		if s.Kind == kind && s.State == "succeeded" {
			return true
		}
	}
	return false
}
func latestSignalPassed(signals []CutoverSignal, kind string) bool {
	for i := len(signals) - 1; i >= 0; i-- {
		if signals[i].Kind == kind {
			return signals[i].Status == "passed" || signals[i].Status == "healthy"
		}
	}
	return false
}
func isPlanOwner(p Plan, actor string) bool {
	for _, s := range p.Sources {
		if contains(s.OwnerIDs, actor) {
			return true
		}
	}
	for _, d := range p.Destinations {
		if contains(d.OwnerIDs, actor) {
			return true
		}
	}
	return false
}

func (s *Store) AddWorkMapping(repositoryID, planID, actorID string, in WorkMappingInput) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	item, ok := inventoryByID(p, in.InventoryItemID)
	if !ok || !validWorkMapping(*p, item, in) {
		return nil, ErrInvalid
	}
	bs := []Blocker{}
	if item.Revision != in.SourceRevision {
		bs = append(bs, Blocker{Kind: "stale_revision", ResourceID: item.ID, Detail: "source work changed after the restructuring inventory"})
	}
	if item.Access == "inaccessible" {
		bs = append(bs, Blocker{Kind: "removed_access", ResourceID: item.ID, Detail: item.Reason})
	}
	if in.ContextAudience == "embargoed" {
		bs = append(bs, Blocker{Kind: "embargoed_context", ResourceID: item.ID, Detail: "context cannot be projected to every destination"})
	}
	if item.Disposition == "retire" || item.Disposition == "unresolved" {
		bs = append(bs, Blocker{Kind: "cannot_migrate", ResourceID: item.ID, Detail: item.Reason})
	}
	status := "proposed"
	if len(bs) > 0 {
		status = "blocked"
	}
	p.WorkMappings = append(p.WorkMappings, WorkMapping{ID: id("wrk"), WorkMappingInput: in, Version: 1, Status: status, Blockers: bs, Decisions: []WorkDecision{}, Outcomes: []WorkOutcome{}, CreatedBy: actorID, CreatedAt: time.Now().UTC(), AuthorityGranted: []string{}})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) DecideWorkMapping(repositoryID, planID, mappingID, actorID, decision, reason string, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	for i := range p.WorkMappings {
		m := &p.WorkMappings[i]
		if m.ID != mappingID {
			continue
		}
		item, _ := inventoryByID(p, m.InventoryItemID)
		if !contains(item.OwnerIDs, actorID) {
			return nil, ErrForbidden
		}
		if expected != m.Version {
			return nil, ErrConflict
		}
		if decision != "approved" && decision != "rejected" || strings.TrimSpace(reason) == "" {
			return nil, ErrInvalid
		}
		m.Version++
		m.Decisions = append(m.Decisions, WorkDecision{ActorID: actorID, Decision: decision, Revision: m.Version, Reason: reason, CreatedAt: time.Now().UTC()})
		if decision == "rejected" {
			m.Status = "blocked"
			m.Blockers = append(m.Blockers, Blocker{Kind: "owner_rejected", ResourceID: item.ID, Detail: reason})
		} else if len(m.Blockers) == 0 && allOwnersApproved(item.OwnerIDs, m.Decisions) {
			m.Status = "approved"
		}
		if err = s.writeUnlocked(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, ErrNotFound
}

func (s *Store) RecordWorkOutcome(repositoryID, planID, mappingID, actorID string, in WorkOutcome, expected int64) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	for i := range p.WorkMappings {
		m := &p.WorkMappings[i]
		if m.ID != mappingID {
			continue
		}
		if expected != m.Version {
			return nil, ErrConflict
		}
		item, _ := inventoryByID(p, m.InventoryItemID)
		d, ok := destinationByID(p, in.DestinationID)
		if !ok || !workOutcomes[in.Status] || strings.TrimSpace(in.Reason) == "" {
			return nil, ErrInvalid
		}
		if !contains(item.OwnerIDs, actorID) && !contains(d.OwnerIDs, actorID) {
			return nil, ErrForbidden
		}
		if in.Status == "continued" && (m.Status != "approved" || in.Reference == "" || in.Revision == "") {
			return nil, ErrInvalid
		}
		in.ActorID = actorID
		in.CreatedAt = time.Now().UTC()
		m.Outcomes = append(m.Outcomes, in)
		m.Version++
		if in.Status != "continued" {
			m.Status = in.Status
			m.Blockers = append(m.Blockers, Blocker{Kind: in.Status, ResourceID: in.DestinationID, Detail: in.Reason})
		} else if allDestinationsContinued(m.Destinations, m.Outcomes) {
			m.Status = "continued"
		}
		if err = s.writeUnlocked(p); err != nil {
			return nil, err
		}
		return p, nil
	}
	return nil, ErrNotFound
}

func (s *Store) AddCandidate(repositoryID, planID, actorID string, in CandidateInput) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	if !validCandidate(*p, in) {
		return nil, ErrInvalid
	}
	p.Candidates = append(p.Candidates, Candidate{ID: id("cand"), CandidateInput: in, CreatedBy: actorID, CreatedAt: time.Now().UTC(), AuthorityGranted: []string{}})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) AddRehearsal(repositoryID, planID, actorID string, in RehearsalInput) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	var candidate *Candidate
	for _, c := range p.Candidates {
		if c.ID == in.CandidateID {
			copy := c
			candidate = &copy
		}
	}
	if candidate == nil || strings.TrimSpace(in.Environment) == "" || in.Budget < 0 || in.ObservedCost < 0 || len(in.Checks) == 0 {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	blockers := []Blocker{}
	for _, c := range in.Checks {
		if !rehearsalDomains[c.Domain] || seen[c.Domain] || !resultStates[c.Status] || c.Command == "" || c.Reference == "" || c.Summary == "" || c.Cost < 0 {
			return nil, ErrInvalid
		}
		seen[c.Domain] = true
		if c.Status != "passed" {
			blockers = append(blockers, Blocker{Kind: c.Status, ResourceID: c.Domain, Detail: c.Summary})
		}
	}
	for domain := range rehearsalDomains {
		if !seen[domain] {
			blockers = append(blockers, Blocker{Kind: "missing_domain", ResourceID: domain, Detail: "required rehearsal domain was not exercised"})
		}
	}
	if in.ObservedCost > in.Budget {
		blockers = append(blockers, Blocker{Kind: "budget_exceeded", ResourceID: in.CandidateID, Detail: "observed rehearsal cost exceeded its bound"})
	}
	issues := append(append(append([]PreservationEvidence{}, candidate.Issues...), candidate.CrossRepositoryLinks...), in.Issues...)
	for _, issue := range issues {
		if !validEvidence(issue) {
			return nil, ErrInvalid
		}
		if issue.Status != "preserved" && issue.Status != "not_applicable" {
			blockers = append(blockers, Blocker{Kind: issue.Kind, ResourceID: issue.Reference, Detail: issue.Detail})
		}
	}
	status := "passed"
	if len(blockers) > 0 {
		status = "blocked"
	}
	p.Rehearsals = append(p.Rehearsals, Rehearsal{ID: id("rhs"), RehearsalInput: in, Status: status, Blockers: blockers, RecordedBy: actorID, RecordedAt: time.Now().UTC(), AuthorityGranted: []string{}})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) AddFinding(repositoryID, planID, actorID string, in FindingInput) (*Plan, error) {
	if strings.TrimSpace(in.Summary) == "" || strings.TrimSpace(in.Impact) == "" || len(in.Citations) == 0 {
		return nil, ErrInvalid
	}
	for _, c := range in.Citations {
		if c.RepositoryID == "" || c.Reference == "" || c.Revision == "" {
			return nil, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.read(repositoryID, planID)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, x := range p.Inventory {
		known[x.ID] = true
	}
	for _, x := range in.AffectedItemIDs {
		if !known[x] {
			return nil, ErrInvalid
		}
	}
	p.Findings = append(p.Findings, Finding{ID: id("fnd"), ActorID: actorID, FindingInput: in, CreatedAt: time.Now().UTC()})
	if err = s.writeUnlocked(p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) Get(repositoryID, planID string) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, planID)
}
func (s *Store) List(repositoryID string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if os.IsNotExist(err) {
		return []Plan{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p, er := s.read(repositoryID, strings.TrimSuffix(e.Name(), ".json"))
		if er == nil {
			out = append(out, *p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Summary) == "" || len(in.Sources) == 0 || len(in.Destinations) == 0 || len(in.Mappings) == 0 || len(in.Inventory) == 0 || len(in.SuccessCriteria) == 0 || in.Deadline.IsZero() || in.RollbackLimits.LatestTime.IsZero() {
		return false
	}
	sources := map[string]string{}
	for _, x := range in.Sources {
		if x.RepositoryID == "" || x.Revision == "" || len(x.OwnerIDs) == 0 {
			return false
		}
		sources[x.RepositoryID] = x.Revision
	}
	dests := map[string]bool{}
	for _, x := range in.Destinations {
		if x.ID == "" || x.Name == "" || len(x.OwnerIDs) == 0 || x.DefaultBranch == "" || (x.Visibility != "public" && x.Visibility != "private" && x.Visibility != "internal") || dests[x.ID] {
			return false
		}
		dests[x.ID] = true
	}
	maps := map[string]bool{}
	for _, x := range in.Mappings {
		if x.ID == "" || maps[x.ID] || sources[x.SourceRepositoryID] != "" && sources[x.SourceRepositoryID] != x.SourceRevision || sources[x.SourceRepositoryID] == "" || !dispositions[x.Disposition] || !historyModes[x.HistoryMode] || len(x.SourcePaths) == 0 {
			return false
		}
		if x.DestinationID != "" && !dests[x.DestinationID] {
			return false
		}
		maps[x.ID] = true
	}
	items := map[string]bool{}
	for _, x := range in.Inventory {
		if x.ID == "" || items[x.ID] || !resourceKinds[x.Kind] || sources[x.RepositoryID] == "" || x.Revision == "" || !accessStates[x.Access] || !dispositions[x.Disposition] {
			return false
		}
		for _, d := range x.DestinationIDs {
			if !dests[d] {
				return false
			}
		}
		items[x.ID] = true
	}
	return true
}

func validEvidence(x PreservationEvidence) bool {
	return x.Kind != "" && x.Reference != "" && preservationStates[x.Status] && strings.TrimSpace(x.Detail) != ""
}

func validCandidate(p Plan, in CandidateInput) bool {
	if len(in.MappingIDs) == 0 || len(in.Repositories) == 0 || in.AssemblyCost < 0 {
		return false
	}
	mappings := map[string]bool{}
	destinations := map[string]bool{}
	for _, x := range p.Mappings {
		mappings[x.ID] = true
	}
	for _, x := range p.Destinations {
		destinations[x.ID] = true
	}
	seen := map[string]bool{}
	for _, x := range in.MappingIDs {
		if !mappings[x] || seen[x] {
			return false
		}
		seen[x] = true
	}
	seen = map[string]bool{}
	for _, r := range in.Repositories {
		if !destinations[r.DestinationID] || seen[r.DestinationID] || r.ObjectDigest == "" || r.DefaultRef == "" || r.DefaultCommit == "" || r.ObjectCount < 1 || r.SizeBytes < 0 || len(r.Evidence) == 0 {
			return false
		}
		seen[r.DestinationID] = true
		for _, e := range r.Evidence {
			if !validEvidence(e) {
				return false
			}
		}
	}
	for _, xs := range [][]PreservationEvidence{in.CrossRepositoryLinks, in.Issues} {
		for _, e := range xs {
			if !validEvidence(e) {
				return false
			}
		}
	}
	return true
}

func inventoryByID(p *Plan, id string) (InventoryItem, bool) {
	for _, x := range p.Inventory {
		if x.ID == id {
			return x, true
		}
	}
	return InventoryItem{}, false
}
func destinationByID(p *Plan, id string) (Destination, bool) {
	for _, x := range p.Destinations {
		if x.ID == id {
			return x, true
		}
	}
	return Destination{}, false
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func validWorkMapping(p Plan, item InventoryItem, in WorkMappingInput) bool {
	if item.ID == "" || !workKinds[in.Kind] || in.SourceRevision == "" || len(in.Authorship) == 0 || len(in.AcceptanceCriteria) == 0 || len(in.Destinations) == 0 {
		return false
	}
	if in.ContextAudience != "public" && in.ContextAudience != "repository" && in.ContextAudience != "owners" && in.ContextAudience != "embargoed" {
		return false
	}
	seen := map[string]bool{}
	for _, d := range in.Destinations {
		dest, ok := destinationByID(&p, d.DestinationID)
		_ = dest
		if !ok || seen[d.DestinationID] || !workKinds[d.Kind] || d.Reference == "" || d.Revision == "" {
			return false
		}
		seen[d.DestinationID] = true
	}
	for _, r := range in.Reviews {
		if r.ActorID == "" || r.Revision == "" || r.Decision == "" || r.Reference == "" {
			return false
		}
	}
	return true
}
func allOwnersApproved(owners []string, ds []WorkDecision) bool {
	for _, o := range owners {
		ok := false
		for i := len(ds) - 1; i >= 0; i-- {
			if ds[i].ActorID == o {
				ok = ds[i].Decision == "approved"
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func allDestinationsContinued(ds []WorkDestination, os []WorkOutcome) bool {
	for _, d := range ds {
		ok := false
		for i := len(os) - 1; i >= 0; i-- {
			if os[i].DestinationID == d.DestinationID {
				ok = os[i].Status == "continued"
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func blockers(items []InventoryItem) []Blocker {
	out := []Blocker{}
	for _, x := range items {
		if x.Access != "accessible" {
			out = append(out, Blocker{Kind: x.Access, ResourceID: x.ID, Detail: x.Reason})
		}
		if x.Disposition == "unresolved" {
			out = append(out, Blocker{Kind: "unresolved_mapping", ResourceID: x.ID, Detail: x.Reason})
		}
	}
	return out
}
func id(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}
func (s *Store) path(r, p string) string { return filepath.Join(s.root, r, p+".json") }
func (s *Store) write(p *Plan) error     { s.mu.Lock(); defer s.mu.Unlock(); return s.writeUnlocked(p) }
func (s *Store) writeUnlocked(p *Plan) error {
	d := filepath.Dir(s.path(p.RepositoryID, p.ID))
	if err := os.MkdirAll(d, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(p.RepositoryID, p.ID) + ".tmp"
	if err = os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(p.RepositoryID, p.ID))
}
func (s *Store) read(r, p string) (*Plan, error) {
	b, err := os.ReadFile(s.path(r, p))
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var x Plan
	if json.Unmarshal(b, &x) != nil {
		return nil, ErrInvalid
	}
	if x.WorkMappings == nil {
		x.WorkMappings = []WorkMapping{}
	}
	if x.MigrationPlans == nil {
		x.MigrationPlans = []MigrationPlan{}
	}
	return &x, nil
}
