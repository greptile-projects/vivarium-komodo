// Package workspaces owns durable, exact-revision development environments.
package workspaces

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

type State string

const (
	SettingUp State = "setting_up"
	Ready     State = "ready"
	Failed    State = "failed"
	Suspended State = "suspended"
	Stopping  State = "stopped"
	Expired   State = "expired"
)

var ErrNotFound = errors.New("workspace not found")
var ErrInvalidTransition = errors.New("invalid workspace transition")
var ErrConflict = errors.New("workspace version conflict")

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Port struct {
	Number int    `json:"number"`
	Label  string `json:"label"`
	Path   string `json:"path"`
}
type ResourceLimits struct {
	CPUSeconds          int `json:"cpu_seconds"`
	MemoryMB            int `json:"memory_mb"`
	DiskMB              int `json:"disk_mb"`
	SetupTimeoutSeconds int `json:"setup_timeout_seconds"`
}
type Definition struct {
	Version      int            `json:"version"`
	Tools        []Tool         `json:"tools"`
	Dependencies []string       `json:"dependencies"`
	Setup        []string       `json:"setup"`
	Commands     []NamedCommand `json:"commands,omitempty"`
	Ports        []Port         `json:"ports,omitempty"`
	Resources    ResourceLimits `json:"resources"`
}
type NamedCommand struct {
	Name           string `json:"name"`
	Command        string `json:"command"`
	Directory      string `json:"directory,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}
type SourceContext struct {
	Type                 string           `json:"type"`
	ID                   string           `json:"id,omitempty"`
	ParentID             string           `json:"parent_id,omitempty"`
	UpstreamRepositoryID string           `json:"upstream_repository_id,omitempty"`
	GuidanceVersion      int64            `json:"guidance_version,omitempty"`
	Guidance             []string         `json:"guidance,omitempty"`
	Evidence             []string         `json:"evidence,omitempty"`
	AcceptanceCriteria   []string         `json:"acceptance_criteria,omitempty"`
	SampleData           []string         `json:"sample_data,omitempty"`
	Conflict             *ConflictContext `json:"conflict,omitempty"`
}

// ConflictContext freezes the two histories and the audience-safe evidence used
// to create a reconciliation workspace. It is context, never an authority grant.
type ConflictContext struct {
	PullRequestID       string             `json:"pull_request_id"`
	BaseCommitID        string             `json:"base_commit_id"`
	Source              ConflictRevision   `json:"source"`
	Target              ConflictRevision   `json:"target"`
	SourceHistory       []string           `json:"source_history"`
	TargetHistory       []string           `json:"target_history"`
	Evidence            []ConflictEvidence `json:"evidence"`
	OwnerIDs            []string           `json:"owner_ids"`
	PublishRepositoryID string             `json:"publish_repository_id"`
	PublishPermission   string             `json:"publish_permission"`
}
type ConflictRevision struct {
	RepositoryID string `json:"repository_id"`
	Branch       string `json:"branch"`
	CommitID     string `json:"commit_id"`
}
type ConflictEvidence struct {
	Kind   string `json:"kind"`
	Path   string `json:"path,omitempty"`
	Symbol string `json:"symbol,omitempty"`
	Detail string `json:"detail"`
}
type ResolutionEvidence struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
	Path      string `json:"path,omitempty"`
}
type OutcomeImpact struct {
	Kind        string `json:"kind"`
	Outcome     string `json:"outcome"`
	Disposition string `json:"disposition"`
	Rationale   string `json:"rationale"`
}

// ResolutionEntry is an append-only explanation of how a conflict question or
// edit relates to the frozen intentions. It does not apply code or grant access.
type ResolutionEntry struct {
	ID          string               `json:"id"`
	ParentID    string               `json:"parent_id,omitempty"`
	Kind        string               `json:"kind"`
	Summary     string               `json:"summary"`
	Paths       []string             `json:"paths,omitempty"`
	Evidence    []ResolutionEvidence `json:"evidence"`
	Impacts     []OutcomeImpact      `json:"impacts,omitempty"`
	Assumptions []string             `json:"assumptions,omitempty"`
	Uncertainty string               `json:"uncertainty,omitempty"`
	ActorID     string               `json:"actor_id"`
	ActorKind   string               `json:"actor_kind"`
	CreatedAt   time.Time            `json:"created_at"`
}
type Access struct {
	RepositoryID string `json:"repository_id"`
	ActorID      string `json:"actor_id"`
	Permission   string `json:"permission"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	State     State     `json:"state,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ActorID   string    `json:"actor_id,omitempty"`
	Kind      string    `json:"kind,omitempty"`
	Surface   string    `json:"surface,omitempty"`
	Path      string    `json:"path,omitempty"`
	TargetID  string    `json:"target_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Presence struct {
	ActorID    string    `json:"actor_id"`
	Surface    string    `json:"surface"`
	Path       string    `json:"path,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type ControlGrant struct {
	ID          string    `json:"id"`
	SubjectID   string    `json:"subject_id"`
	SubjectKind string    `json:"subject_kind"`
	Mode        string    `json:"mode"`
	Scopes      []string  `json:"scopes"`
	State       string    `json:"state"`
	GrantedBy   string    `json:"granted_by"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type FileChange struct {
	Sequence  int64     `json:"sequence"`
	Path      string    `json:"path"`
	ActorID   string    `json:"actor_id"`
	Digest    string    `json:"digest,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Reproducibility struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Commands     []string `json:"commands,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}
type CheckpointChange struct {
	Path       string `json:"path"`
	Operation  string `json:"operation"`
	BaseDigest string `json:"base_digest,omitempty"`
	Digest     string `json:"digest,omitempty"`
	BlobDigest string `json:"-"`
	Patch      string `json:"patch,omitempty"`
	Binary     bool   `json:"binary,omitempty"`
	Size       int64  `json:"size,omitempty"`
}
type CheckpointStatus struct {
	Reproducible        bool     `json:"reproducible"`
	Diverged            bool     `json:"diverged"`
	Conflicts           []string `json:"conflicts"`
	MissingDependencies []string `json:"missing_dependencies"`
	Reasons             []string `json:"reasons"`
}
type Checkpoint struct {
	ID               string                 `json:"id"`
	WorkspaceID      string                 `json:"workspace_id"`
	RepositoryID     string                 `json:"repository_id"`
	ParentID         string                 `json:"parent_id,omitempty"`
	CreatorID        string                 `json:"creator_id"`
	BaseRevision     string                 `json:"base_revision"`
	Definition       Definition             `json:"environment_definition"`
	DefinitionDigest string                 `json:"definition_digest"`
	Summary          string                 `json:"summary"`
	Reproducibility  Reproducibility        `json:"reproducibility"`
	Changes          []CheckpointChange     `json:"changes"`
	Status           CheckpointStatus       `json:"status"`
	CreatedAt        time.Time              `json:"created_at"`
	Publication      *Publication           `json:"publication,omitempty"`
	Verification     *VerificationCandidate `json:"verification,omitempty"`
}

// VerificationCandidate is the immutable proof surface for a reconciliation
// checkpoint. Attempts and owner decisions are append-only and retain the exact
// input keys they evaluate so unrelated drift does not discard useful proof.
type VerificationCandidate struct {
	Digest    string                  `json:"digest"`
	Inputs    VerificationInputs      `json:"inputs"`
	Criteria  []VerificationCriterion `json:"criteria"`
	Attempts  []VerificationAttempt   `json:"attempts"`
	Decisions []VerificationDecision  `json:"owner_decisions"`
	Status    string                  `json:"status"`
	Blockers  []string                `json:"blockers,omitempty"`
}
type VerificationInputs struct {
	Candidate  string `json:"candidate"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Dependency string `json:"dependency"`
	Policy     string `json:"policy"`
}
type VerificationCriterion struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Origin         string   `json:"origin"`
	AffectedInputs []string `json:"affected_inputs"`
	OwnerIDs       []string `json:"owner_ids,omitempty"`
}
type VerificationArtifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type,omitempty"`
}
type VerificationAttempt struct {
	ID             string                 `json:"id"`
	CriterionIDs   []string               `json:"criterion_ids"`
	Kind           string                 `json:"kind"`
	InputRevisions VerificationInputs     `json:"input_revisions"`
	Commands       []string               `json:"commands"`
	Logs           []string               `json:"logs,omitempty"`
	Artifacts      []VerificationArtifact `json:"artifacts,omitempty"`
	Coverage       []string               `json:"coverage,omitempty"`
	Failures       []string               `json:"failures,omitempty"`
	Cost           float64                `json:"cost"`
	Currency       string                 `json:"currency,omitempty"`
	Status         string                 `json:"status"`
	StaleInputKeys []string               `json:"stale_input_keys,omitempty"`
	ActorID        string                 `json:"actor_id"`
	CreatedAt      time.Time              `json:"created_at"`
}
type VerificationDecision struct {
	ID             string             `json:"id"`
	CriterionIDs   []string           `json:"criterion_ids"`
	InputRevisions VerificationInputs `json:"input_revisions"`
	Decision       string             `json:"decision"`
	Rationale      string             `json:"rationale"`
	StaleInputKeys []string           `json:"stale_input_keys,omitempty"`
	OwnerID        string             `json:"owner_id"`
	CreatedAt      time.Time          `json:"created_at"`
}
type Publication struct {
	CommitID           string    `json:"commit_id"`
	Branch             string    `json:"branch"`
	Mode               string    `json:"mode,omitempty"`
	PullRequestID      string    `json:"pull_request_id,omitempty"`
	PublisherID        string    `json:"publisher_id"`
	ContributorIDs     []string  `json:"contributor_ids"`
	SourceCommitID     string    `json:"source_commit_id,omitempty"`
	TargetCommitID     string    `json:"target_commit_id,omitempty"`
	VerificationDigest string    `json:"verification_digest,omitempty"`
	ResolutionIDs      []string  `json:"resolution_ids,omitempty"`
	ApprovalIDs        []string  `json:"approval_ids,omitempty"`
	Commands           []string  `json:"commands,omitempty"`
	PublishedAt        time.Time `json:"published_at"`
}
type Workspace struct {
	ID                string            `json:"id"`
	RepositoryID      string            `json:"repository_id"`
	Revision          string            `json:"revision"`
	CreatorID         string            `json:"creator_id"`
	Context           SourceContext     `json:"source_context"`
	Access            Access            `json:"effective_access"`
	Definition        Definition        `json:"definition"`
	DefinitionDigest  string            `json:"definition_digest"`
	State             State             `json:"state"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	SuspendedAt       *time.Time        `json:"suspended_at,omitempty"`
	ReadyAt           *time.Time        `json:"ready_at,omitempty"`
	Events            []Event           `json:"setup_evidence"`
	Activity          []Event           `json:"activity"`
	Changes           []FileChange      `json:"changes"`
	Presence          []Presence        `json:"presence"`
	Controls          []ControlGrant    `json:"controls"`
	Checkpoints       []Checkpoint      `json:"checkpoints"`
	Resolutions       []ResolutionEntry `json:"resolutions,omitempty"`
	Policy            Policy            `json:"effective_policy"`
	ExpiresAt         *time.Time        `json:"expires_at,omitempty"`
	ExpiryAnnouncedAt *time.Time        `json:"expiry_announced_at,omitempty"`
	StoppedAt         *time.Time        `json:"stopped_at,omitempty"`
	RebuildRequired   bool              `json:"rebuild_required"`
	RebuildReasons    []string          `json:"rebuild_reasons,omitempty"`
	Consumption       []Consumption     `json:"consumption,omitempty"`
}

func (s *Store) AddResolution(repositoryID, id, actor string, entry ResolutionEntry) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID || w.Context.Conflict == nil {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	if entry.ParentID != "" {
		found := false
		for _, existing := range w.Resolutions {
			found = found || existing.ID == entry.ParentID
		}
		if !found {
			return Workspace{}, ErrConflict
		}
	}
	b := make([]byte, 12)
	if _, err = rand.Read(b); err != nil {
		return Workspace{}, err
	}
	now := s.now().UTC()
	entry.ID, entry.ActorID, entry.CreatedAt = hex.EncodeToString(b), actor, now
	w.Resolutions = append(w.Resolutions, entry)
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "resolution_" + entry.Kind, Kind: "intent", ActorID: actor, TargetID: entry.ID, Message: entry.Summary, CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

// Policy is the complete owner-controlled authority and cost envelope. Zero
// values from old records are upgraded to DefaultPolicy when read.
type Policy struct {
	CPUSeconds        int    `json:"cpu_seconds"`
	MemoryMB          int    `json:"memory_mb"`
	DiskMB            int    `json:"disk_mb"`
	Network           string `json:"network"`
	IdleMinutes       int    `json:"idle_minutes"`
	RetentionDays     int    `json:"retention_days"`
	ExpiryNoticeHours int    `json:"expiry_notice_hours"`
	Sharing           string `json:"sharing"`
	AgentExecution    bool   `json:"agent_execution"`
}
type Consumption struct {
	ActorID    string    `json:"actor_id"`
	Kind       string    `json:"kind"`
	Quantity   int64     `json:"quantity"`
	Unit       string    `json:"unit"`
	RecordedAt time.Time `json:"recorded_at"`
}

func DefaultPolicy() Policy {
	return Policy{CPUSeconds: 300, MemoryMB: 2048, DiskMB: 4096, Network: "none", IdleMinutes: 120, RetentionDays: 30, ExpiryNoticeHours: 24, Sharing: "participants", AgentExecution: true}
}
func (p Policy) Valid() bool {
	return p.CPUSeconds >= 1 && p.CPUSeconds <= 3600 && p.MemoryMB >= 128 && p.MemoryMB <= 32768 && p.DiskMB >= 128 && p.DiskMB <= 102400 && (p.Network == "none" || p.Network == "restricted") && p.IdleMinutes >= 5 && p.IdleMinutes <= 10080 && p.RetentionDays >= 1 && p.RetentionDays <= 365 && p.ExpiryNoticeHours >= 1 && p.ExpiryNoticeHours <= 168 && (p.Sharing == "private" || p.Sharing == "participants")
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Observe refreshes an intentionally short presence lease. Disconnects disappear
// predictably without rewriting the durable activity ledger.
func (s *Store) Observe(repositoryID, id, actor, surface, path string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	kept := w.Presence[:0]
	for _, p := range w.Presence {
		if p.ExpiresAt.After(now) && p.ActorID != actor {
			kept = append(kept, p)
		}
	}
	w.Presence = append(kept, Presence{ActorID: actor, Surface: surface, Path: path, ObservedAt: now, ExpiresAt: now.Add(45 * time.Second)})
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "presence", Kind: "observation", Surface: surface, Path: path, ActorID: actor, CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) AddMessage(repositoryID, id, actor, message string) (Workspace, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4000 {
		return Workspace{}, ErrConflict
	}
	return s.RecordActivity(repositoryID, id, Event{Type: "message", Kind: "instruction", Surface: "discussion", Message: message, ActorID: actor})
}

func (s *Store) Grant(repositoryID, id, actor, subject, kind, mode string, scopes []string) (Workspace, error) {
	validMode := mode == "observe" || mode == "edit" || mode == "execute"
	if subject == "" || (kind != "human" && kind != "approved_agent") || !validMode || len(scopes) == 0 {
		return Workspace{}, ErrConflict
	}
	allowed := map[string]bool{"files": true, "terminal": true, "preview": true}
	seen := map[string]bool{}
	for _, scope := range scopes {
		if !allowed[scope] || seen[scope] {
			return Workspace{}, ErrConflict
		}
		seen[scope] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	if w.Policy.Sharing == "private" && subject != w.CreatorID {
		return Workspace{}, ErrConflict
	}
	if kind == "approved_agent" && !w.Policy.AgentExecution {
		return Workspace{}, ErrConflict
	}
	now := s.now().UTC()
	grantID, err := newID()
	if err != nil {
		return Workspace{}, err
	}
	g := ControlGrant{ID: grantID, SubjectID: subject, SubjectKind: kind, Mode: mode, Scopes: append([]string(nil), scopes...), State: "active", GrantedBy: actor, Version: 1, CreatedAt: now, UpdatedAt: now}
	w.Controls = append(w.Controls, g)
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "control_granted", Kind: "instruction", ActorID: actor, TargetID: subject, Message: mode + ":" + strings.Join(scopes, ","), CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) Intervene(repositoryID, id, actor, grantID, action, message string, version int64) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	index := -1
	for i := range w.Controls {
		if w.Controls[i].ID == grantID {
			index = i
			break
		}
	}
	if index < 0 {
		return Workspace{}, ErrNotFound
	}
	g := &w.Controls[index]
	if g.Version != version || g.State == "revoked" {
		return Workspace{}, ErrConflict
	}
	switch action {
	case "guide":
		if strings.TrimSpace(message) == "" {
			return Workspace{}, ErrConflict
		}
	case "pause":
		g.State = "paused"
	case "resume", "take_over":
		g.State = "active"
	case "revoke":
		g.State = "revoked"
	default:
		return Workspace{}, ErrConflict
	}
	now := s.now().UTC()
	g.Version++
	g.UpdatedAt = now
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: action, Kind: "instruction", ActorID: actor, TargetID: g.SubjectID, Message: strings.TrimSpace(message), CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) RecordActivity(repositoryID, id string, event Event) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	event.Sequence = int64(len(w.Activity) + 1)
	event.CreatedAt = s.now().UTC()
	if len(event.Message) > 1<<20 {
		event.Message = event.Message[:1<<20]
	}
	w.Activity = append(w.Activity, event)
	w.UpdatedAt = event.CreatedAt
	return w, s.write(w)
}

func (s *Store) RecordPublication(repositoryID, id, checkpointID string, publication Publication) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	for i := range w.Checkpoints {
		if w.Checkpoints[i].ID != checkpointID {
			continue
		}
		if w.Checkpoints[i].Publication != nil {
			return Workspace{}, ErrConflict
		}
		w.Checkpoints[i].Publication = &publication
		w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "checkpoint_published", Kind: "authorship", ActorID: publication.PublisherID, TargetID: checkpointID, Message: publication.CommitID, CreatedAt: publication.PublishedAt})
		w.UpdatedAt = publication.PublishedAt
		return w, s.write(w)
	}
	return Workspace{}, ErrNotFound
}

func (s *Store) LinkPublicationPullRequest(repositoryID, id, checkpointID, pullRequestID string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	for i := range w.Checkpoints {
		if w.Checkpoints[i].ID == checkpointID && w.Checkpoints[i].Publication != nil && w.Checkpoints[i].Publication.PullRequestID == "" {
			w.Checkpoints[i].Publication.PullRequestID = pullRequestID
			w.UpdatedAt = s.now().UTC()
			return w, s.write(w)
		}
	}
	return Workspace{}, ErrConflict
}

func (s *Store) RecordChange(repositoryID, id, actor, path, digest string, deleted bool) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	w.Changes = append(w.Changes, FileChange{Sequence: int64(len(w.Changes) + 1), Path: path, ActorID: actor, Digest: digest, Deleted: deleted, CreatedAt: now})
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "file", Kind: "authorship", Surface: "files", Path: path, ActorID: actor, Message: map[bool]string{true: "deleted", false: "saved"}[deleted], CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "records"), 0o750); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "environments"), 0o750); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "checkpoint-blobs"), 0o750); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "policies"), 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) AddCheckpoint(repositoryID, workspaceID string, checkpoint Checkpoint, blobs map[string][]byte) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(workspaceID)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	for _, existing := range w.Checkpoints {
		if existing.ID == checkpoint.ParentID {
			checkpoint.ParentID = existing.ID
			break
		}
	}
	if checkpoint.ParentID != "" {
		found := false
		for _, existing := range w.Checkpoints {
			found = found || existing.ID == checkpoint.ParentID
		}
		if !found {
			return Workspace{}, ErrConflict
		}
	}
	for digest, data := range blobs {
		path := filepath.Join(s.root, "checkpoint-blobs", digest)
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			if err = os.WriteFile(path, data, 0o640); err != nil {
				return Workspace{}, err
			}
		}
	}
	w.Checkpoints = append(w.Checkpoints, checkpoint)
	now := s.now().UTC()
	w.UpdatedAt = now
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "checkpoint", Kind: "authorship", ActorID: checkpoint.CreatorID, TargetID: checkpoint.ID, Message: checkpoint.Summary, CreatedAt: now})
	return w, s.write(w)
}

func verificationStale(expected, got VerificationInputs, keys []string) []string {
	values := map[string][2]string{"candidate": {expected.Candidate, got.Candidate}, "source": {expected.Source, got.Source}, "target": {expected.Target, got.Target}, "dependency": {expected.Dependency, got.Dependency}, "policy": {expected.Policy, got.Policy}}
	out := []string{}
	for _, key := range keys {
		if pair, ok := values[key]; ok && pair[0] != pair[1] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
func refreshVerification(candidate *VerificationCandidate) {
	latest := map[string]string{}
	for i := range candidate.Attempts {
		keys := []string{}
		for _, id := range candidate.Attempts[i].CriterionIDs {
			for _, c := range candidate.Criteria {
				if c.ID == id {
					keys = append(keys, c.AffectedInputs...)
				}
			}
		}
		candidate.Attempts[i].StaleInputKeys = verificationStale(candidate.Inputs, candidate.Attempts[i].InputRevisions, keys)
		if len(candidate.Attempts[i].StaleInputKeys) == 0 {
			for _, id := range candidate.Attempts[i].CriterionIDs {
				latest[id] = candidate.Attempts[i].Status
			}
		}
	}
	for i := range candidate.Decisions {
		keys := []string{}
		for _, id := range candidate.Decisions[i].CriterionIDs {
			for _, criterion := range candidate.Criteria {
				if criterion.ID == id {
					keys = append(keys, criterion.AffectedInputs...)
				}
			}
		}
		candidate.Decisions[i].StaleInputKeys = verificationStale(candidate.Inputs, candidate.Decisions[i].InputRevisions, keys)
	}
	candidate.Blockers = nil
	for _, criterion := range candidate.Criteria {
		if latest[criterion.ID] != "passed" {
			candidate.Blockers = append(candidate.Blockers, "missing current evidence: "+criterion.ID)
		}
	}
	for _, decision := range candidate.Decisions {
		if len(decision.StaleInputKeys) == 0 && decision.Decision == "rejected" {
			candidate.Blockers = append(candidate.Blockers, "owner rejected behavior change: "+decision.ID)
		}
	}
	switch {
	case func() bool {
		for _, status := range latest {
			if status == "failed" {
				return true
			}
		}
		return false
	}():
		candidate.Status = "failed"
	case len(candidate.Blockers) > 0:
		candidate.Status = "blocked"
	default:
		candidate.Status = "passed"
	}
}
func criterionSet(candidate *VerificationCandidate, ids []string) bool {
	if len(ids) == 0 {
		return false
	}
	known := map[string]bool{}
	for _, c := range candidate.Criteria {
		known[c.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func (s *Store) AddVerificationAttempt(repositoryID, workspaceID, checkpointID, actor string, in VerificationAttempt) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(workspaceID)
	if err != nil || w.RepositoryID != repositoryID {
		return Checkpoint{}, ErrNotFound
	}
	for i := range w.Checkpoints {
		c := &w.Checkpoints[i]
		if c.ID != checkpointID || c.Verification == nil {
			continue
		}
		if !criterionSet(c.Verification, in.CriterionIDs) || !map[string]bool{"required_check": true, "reproduction": true, "contract_scenario": true, "schema_scenario": true, "preview_acceptance": true, "conflict_test": true, "conflict_scenario": true, "acceptance": true}[in.Kind] || !map[string]bool{"passed": true, "failed": true, "blocked": true}[in.Status] || len(in.Commands) == 0 || in.Cost < 0 {
			return Checkpoint{}, ErrConflict
		}
		for _, a := range in.Artifacts {
			if a.Name == "" || len(a.Digest) != 64 {
				return Checkpoint{}, ErrConflict
			}
		}
		in.ID, _ = newID()
		in.ActorID = actor
		in.CreatedAt = s.now().UTC()
		c.Verification.Attempts = append(c.Verification.Attempts, in)
		refreshVerification(c.Verification)
		w.UpdatedAt = in.CreatedAt
		w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "verification_attempt", Kind: "evidence", ActorID: actor, TargetID: in.ID, Message: in.Status, CreatedAt: in.CreatedAt})
		if err = s.write(w); err != nil {
			return Checkpoint{}, err
		}
		return *c, nil
	}
	return Checkpoint{}, ErrNotFound
}
func (s *Store) AddVerificationDecision(repositoryID, workspaceID, checkpointID, actor string, in VerificationDecision) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(workspaceID)
	if err != nil || w.RepositoryID != repositoryID || w.Context.Conflict == nil {
		return Checkpoint{}, ErrNotFound
	}
	owner := false
	for _, id := range w.Context.Conflict.OwnerIDs {
		owner = owner || id == actor
	}
	if !owner {
		return Checkpoint{}, ErrConflict
	}
	for i := range w.Checkpoints {
		c := &w.Checkpoints[i]
		if c.ID != checkpointID || c.Verification == nil {
			continue
		}
		if !criterionSet(c.Verification, in.CriterionIDs) || !map[string]bool{"approved": true, "rejected": true}[in.Decision] || strings.TrimSpace(in.Rationale) == "" {
			return Checkpoint{}, ErrConflict
		}
		in.ID, _ = newID()
		in.OwnerID = actor
		in.CreatedAt = s.now().UTC()
		c.Verification.Decisions = append(c.Verification.Decisions, in)
		refreshVerification(c.Verification)
		w.UpdatedAt = in.CreatedAt
		w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "verification_decision", Kind: "decision", ActorID: actor, TargetID: in.ID, Message: in.Decision, CreatedAt: in.CreatedAt})
		if err = s.write(w); err != nil {
			return Checkpoint{}, err
		}
		return *c, nil
	}
	return Checkpoint{}, ErrNotFound
}

func (s *Store) Blob(digest string) ([]byte, error) {
	if len(digest) != 64 {
		return nil, ErrNotFound
	}
	return os.ReadFile(filepath.Join(s.root, "checkpoint-blobs", digest))
}
func (s *Store) Environment(id string) string { return filepath.Join(s.root, "environments", id) }

func (s *Store) Create(repositoryID, revision, creatorID string, context SourceContext, access Access, definition Definition, digest string) (Workspace, error) {
	policy, _ := s.EffectivePolicy(repositoryID, "")
	return s.CreateWithPolicy(repositoryID, revision, creatorID, context, access, definition, digest, policy)
}
func (s *Store) CreateWithPolicy(repositoryID, revision, creatorID string, context SourceContext, access Access, definition Definition, digest string, policy Policy) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Workspace{}, err
	}
	now := s.now().UTC()
	if !policy.Valid() {
		policy = DefaultPolicy()
	}
	definition.Resources.CPUSeconds = min(definition.Resources.CPUSeconds, policy.CPUSeconds)
	definition.Resources.MemoryMB = min(definition.Resources.MemoryMB, policy.MemoryMB)
	definition.Resources.DiskMB = min(definition.Resources.DiskMB, policy.DiskMB)
	w := Workspace{ID: hex.EncodeToString(b), RepositoryID: repositoryID, Revision: revision, CreatorID: creatorID, Context: context, Access: access, Definition: definition, DefinitionDigest: digest, Policy: policy, State: SettingUp, CreatedAt: now, UpdatedAt: now}
	w.append(Event{Type: "state", State: SettingUp, ActorID: creatorID, CreatedAt: now})
	return w, s.write(w)
}

func (s *Store) EffectivePolicy(repositoryID, organizationID string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, err := s.readPolicy("repository", repositoryID); err == nil {
		return p, nil
	}
	if organizationID != "" {
		if p, err := s.readPolicy("organization", organizationID); err == nil {
			return p, nil
		}
	}
	return DefaultPolicy(), nil
}
func (s *Store) Get(repositoryID, id string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if changed := s.applyLifecycle(&w); changed {
		_ = s.write(w)
	}
	w.Presence = activePresence(w.Presence, s.now().UTC())
	return w, nil
}
func (s *Store) List(repositoryID string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "records"))
	if err != nil {
		return nil, err
	}
	out := []Workspace{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		w, e := s.read(entry.Name()[:len(entry.Name())-5])
		if e == nil && w.RepositoryID == repositoryID {
			if changed := s.applyLifecycle(&w); changed {
				_ = s.write(w)
			}
			w.Presence = activePresence(w.Presence, s.now().UTC())
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) applyLifecycle(w *Workspace) bool {
	now := s.now().UTC()
	changed := false
	deadline := w.CreatedAt.Add(time.Duration(w.Policy.RetentionDays) * 24 * time.Hour)
	noticeAt := deadline.Add(-time.Duration(w.Policy.ExpiryNoticeHours) * time.Hour)
	if w.ExpiresAt == nil && (now.After(noticeAt) || now.Equal(noticeAt)) && w.State != Expired && w.State != Stopping {
		at := deadline
		if at.Before(now.Add(time.Duration(w.Policy.ExpiryNoticeHours) * time.Hour)) {
			at = now.Add(time.Duration(w.Policy.ExpiryNoticeHours) * time.Hour)
		}
		w.ExpiresAt = &at
		w.ExpiryAnnouncedAt = &now
		w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "expiry_announced", Kind: "lifecycle", ActorID: "system", Message: at.Format(time.RFC3339), CreatedAt: now})
		changed = true
	}
	if w.ExpiresAt != nil && !now.Before(*w.ExpiresAt) && w.State != Expired && w.State != Stopping {
		w.State = Expired
		w.StoppedAt = &now
		w.Presence = nil
		for i := range w.Controls {
			w.Controls[i].State = "revoked"
			w.Controls[i].Version++
			w.Controls[i].UpdatedAt = now
		}
		w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "expired", Kind: "lifecycle", ActorID: "system", CreatedAt: now})
		changed = true
	} else if w.State == Ready && now.Sub(w.UpdatedAt) >= time.Duration(w.Policy.IdleMinutes)*time.Minute {
		w.State = Suspended
		w.SuspendedAt = &now
		w.Presence = nil
		w.Events = append(w.Events, Event{Sequence: int64(len(w.Events) + 1), Type: "state", State: Suspended, ActorID: "system", Message: "idle policy", CreatedAt: now})
		changed = true
	}
	if changed {
		w.UpdatedAt = now
	}
	return changed
}

func (s *Store) RequireRebuild(repositoryID, id, reason string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	w.RebuildRequired = true
	found := false
	for _, v := range w.RebuildReasons {
		found = found || v == reason
	}
	if !found {
		w.RebuildReasons = append(w.RebuildReasons, reason)
	}
	now := s.now().UTC()
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "rebuild_required", Kind: "lifecycle", ActorID: "system", Message: reason, CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}
func activePresence(items []Presence, now time.Time) []Presence {
	out := make([]Presence, 0, len(items))
	for _, item := range items {
		if item.ExpiresAt.After(now) {
			out = append(out, item)
		}
	}
	return out
}
func (s *Store) Append(id string, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil {
		return err
	}
	if event.Type == "log" {
		const maximumLogBytes = 10 << 20
		used := 0
		for _, existing := range w.Events {
			if existing.Type == "log" {
				used += len(existing.Message)
			}
		}
		if used >= maximumLogBytes {
			return nil
		}
		if len(event.Message) > maximumLogBytes-used {
			event.Message = event.Message[:maximumLogBytes-used]
		}
	}
	event.CreatedAt = s.now().UTC()
	w.append(event)
	w.UpdatedAt = event.CreatedAt
	return s.write(w)
}
func (s *Store) Finish(id string, success bool, message string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil {
		return Workspace{}, err
	}
	if w.State != SettingUp {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	if success {
		w.State = Ready
		w.ReadyAt = &now
	} else {
		w.State = Failed
	}
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: w.State, Message: message, CreatedAt: now})
	return w, s.write(w)
}
func (s *Store) Suspend(repositoryID, id, actor string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	w.State = Suspended
	w.SuspendedAt = &now
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: Suspended, ActorID: actor, CreatedAt: now})
	return w, s.write(w)
}
func (s *Store) Resume(repositoryID, id, actor, digest string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Suspended || w.DefinitionDigest != digest {
		return Workspace{}, ErrInvalidTransition
	}
	if info, e := os.Stat(s.Environment(id)); e != nil || !info.IsDir() {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	w.State = Ready
	w.SuspendedAt = nil
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: Ready, ActorID: actor, Message: "resumed retained exact-revision environment", CreatedAt: now})
	return w, s.write(w)
}
func (w *Workspace) append(e Event) {
	e.Sequence = int64(len(w.Events) + 1)
	w.Events = append(w.Events, e)
}
func (s *Store) read(id string) (Workspace, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "records", id+".json"))
	if os.IsNotExist(err) {
		return Workspace{}, ErrNotFound
	}
	var w Workspace
	if err == nil {
		err = json.Unmarshal(data, &w)
		if !w.Policy.Valid() {
			w.Policy = DefaultPolicy()
		}
	}
	return w, err
}

func (s *Store) SetPolicy(scope, id string, p Policy) (Policy, error) {
	if (scope != "repository" && scope != "organization") || id == "" || !p.Valid() {
		return Policy{}, ErrConflict
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return Policy{}, err
	}
	err = os.WriteFile(filepath.Join(s.root, "policies", scope+"-"+id+".json"), data, 0o640)
	if err == nil && scope == "repository" {
		entries, _ := os.ReadDir(filepath.Join(s.root, "records"))
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			w, e := s.read(strings.TrimSuffix(entry.Name(), ".json"))
			if e != nil || w.RepositoryID != id {
				continue
			}
			if w.Policy != p {
				w.RebuildRequired = true
				w.RebuildReasons = []string{"workspace policy changed"}
				now := s.now().UTC()
				for i := range w.Controls {
					if p.Sharing == "private" && w.Controls[i].SubjectID != w.CreatorID || w.Controls[i].SubjectKind == "approved_agent" && !p.AgentExecution {
						w.Controls[i].State = "revoked"
						w.Controls[i].Version++
						w.Controls[i].UpdatedAt = now
					}
				}
				w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "rebuild_required", Kind: "lifecycle", ActorID: "system", Message: "workspace policy changed", CreatedAt: now})
				_ = s.write(w)
			}
		}
	}
	return p, err
}
func (s *Store) Policy(scope, id string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readPolicy(scope, id)
	if os.IsNotExist(err) {
		return DefaultPolicy(), nil
	}
	return p, err
}
func (s *Store) readPolicy(scope, id string) (Policy, error) {
	var p Policy
	data, err := os.ReadFile(filepath.Join(s.root, "policies", scope+"-"+id+".json"))
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	return p, err
}

func (s *Store) AnnounceExpiry(repositoryID, id, actor string, at time.Time) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	now := s.now().UTC()
	if at.Before(now.Add(time.Duration(w.Policy.ExpiryNoticeHours)*time.Hour)) || w.State == Expired || w.State == Stopping {
		return Workspace{}, ErrConflict
	}
	w.ExpiresAt = &at
	w.ExpiryAnnouncedAt = &now
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "expiry_announced", Kind: "lifecycle", ActorID: actor, Message: at.UTC().Format(time.RFC3339), CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}
func (s *Store) Stop(repositoryID, id, actor, reason string, expire bool) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State == Expired || w.State == Stopping {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	if expire {
		w.State = Expired
	} else {
		w.State = Stopping
	}
	w.StoppedAt = &now
	w.Presence = nil
	for i := range w.Controls {
		if w.Controls[i].State != "revoked" {
			w.Controls[i].State = "revoked"
			w.Controls[i].Version++
			w.Controls[i].UpdatedAt = now
		}
	}
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: string(w.State), Kind: "lifecycle", ActorID: actor, Message: strings.TrimSpace(reason), CreatedAt: now})
	w.UpdatedAt = now
	if err := s.write(w); err != nil {
		return Workspace{}, err
	}
	// Terminal lifecycle decisions retain the record and content-addressed
	// checkpoint evidence, but the mutable runtime must stop consuming storage
	// and must not remain available outside the public state machine.
	if err := os.RemoveAll(s.Environment(id)); err != nil {
		return w, err
	}
	return w, nil
}
func (s *Store) RecordConsumption(repositoryID, id, actor, kind string, quantity int64, unit string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if quantity < 0 {
		return Workspace{}, ErrConflict
	}
	w.Consumption = append(w.Consumption, Consumption{ActorID: actor, Kind: kind, Quantity: quantity, Unit: unit, RecordedAt: s.now().UTC()})
	return w, s.write(w)
}
func (s *Store) write(w Workspace) error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, "records", w.ID+".json")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".workspace-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o640)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	return err
}
