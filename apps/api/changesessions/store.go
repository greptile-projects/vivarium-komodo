// Package changesessions owns durable, pull-request-scoped agent collaboration sessions.
package changesessions

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
)

var (
	ErrNotFound = errors.New("change session not found")
	ErrInvalid  = errors.New("invalid change session")
)

type State string

const AwaitingInstructions State = "awaiting_instructions"
const Delegated State = "delegated"

type RunState string

const Queued RunState = "queued"
const Running RunState = "running"
const Paused RunState = "paused"
const Succeeded RunState = "succeeded"
const Failed RunState = "failed"
const Canceled RunState = "canceled"

type Run struct {
	ID                  string       `json:"id"`
	InitiatorID         string       `json:"initiator_id"`
	Agent               string       `json:"agent"`
	Instructions        string       `json:"instructions"`
	RevisionID          string       `json:"revision_id"`
	ContextPaths        []string     `json:"context_paths"`
	WorkingBranch       string       `json:"working_branch"`
	CredentialGrantID   string       `json:"credential_grant_id"`
	CredentialExpiresAt time.Time    `json:"credential_expires_at"`
	CredentialRevokedAt *time.Time   `json:"credential_revoked_at,omitempty"`
	State               RunState     `json:"state"`
	Publication         *Publication `json:"publication,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
}

// Publication is the durable, review-facing outcome of a successful run.
// Commit and file identities are derived from repository state by the API.
type Publication struct {
	Summary        string    `json:"summary"`
	CommitIDs      []string  `json:"commit_ids"`
	ChangedFiles   []string  `json:"changed_files"`
	Checks         []string  `json:"checks"`
	Concerns       []string  `json:"concerns"`
	SourceCommitID string    `json:"source_commit_id"`
	PublishedAt    time.Time `json:"published_at"`
}

type Event struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	ActorID     string            `json:"actor_id"`
	RunID       string            `json:"run_id,omitempty"`
	InitiatorID string            `json:"initiator_id,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	RevisionID  string            `json:"revision_id,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

var runEventTypes = map[string]RunState{
	"run.started": Running, "agent.message": "", "tool.started": "", "tool.completed": "",
	"artifact.produced": "", "branch.updated": "", "run.failed": Failed, "run.completed": Succeeded,
}

var interventionTypes = map[string]bool{
	"guidance": true, "answer": true, "pause": true, "resume": true, "cancel": true,
}

// Intervene records a collaborator command and applies its control transition
// before returning, so workers and reconnecting clients observe one ordering.
func (s *Store) Intervene(repositoryID, pullRequestID, sessionID, runID, actorID, kind, message string) (Event, Run, error) {
	kind, message = strings.TrimSpace(kind), strings.TrimSpace(message)
	if actorID == "" || !interventionTypes[kind] || ((kind == "guidance" || kind == "answer") && message == "") || len(message) > 10000 {
		return Event{}, Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, pullRequestID, sessionID)
	if err != nil {
		return Event{}, Run{}, err
	}
	for i := range item.Runs {
		run := &item.Runs[i]
		if run.ID != runID {
			continue
		}
		if run.State == Succeeded || run.State == Failed || run.State == Canceled {
			return Event{}, Run{}, ErrInvalid
		}
		switch kind {
		case "pause":
			if run.State != Queued && run.State != Running {
				return Event{}, Run{}, ErrInvalid
			}
			run.State = Paused
		case "resume":
			if run.State != Paused {
				return Event{}, Run{}, ErrInvalid
			}
			run.State = Running
		case "cancel":
			run.State = Canceled
		}
		id, err := newID()
		if err != nil {
			return Event{}, Run{}, err
		}
		now := s.now().UTC()
		metadata := map[string]string{"action": kind}
		if message != "" {
			metadata["message"] = message
		}
		event := Event{ID: id, Type: "run.intervention", ActorID: actorID, RunID: run.ID, InitiatorID: run.InitiatorID, Agent: run.Agent, RevisionID: run.RevisionID, Metadata: metadata, CreatedAt: now}
		item.Events = append(item.Events, event)
		item.UpdatedAt = now
		if err := s.write(item); err != nil {
			return Event{}, Run{}, err
		}
		return event, *run, nil
	}
	return Event{}, Run{}, ErrNotFound
}

// AppendRunEvent adds a worker-reported public record and applies terminal run state.
// Attribution is copied from the durable run rather than trusted from worker input.
func (s *Store) AppendRunEvent(repositoryID, pullRequestID, sessionID, runID, eventType string, metadata map[string]string) (Event, error) {
	nextState, ok := runEventTypes[eventType]
	if !ok || len(metadata) > 20 {
		return Event{}, ErrInvalid
	}
	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || len(key) > 50 || len(value) > 10000 {
			return Event{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, pullRequestID, sessionID)
	if err != nil {
		return Event{}, err
	}
	for i := range item.Runs {
		run := &item.Runs[i]
		if run.ID != runID {
			continue
		}
		if run.State == Paused || run.State == Succeeded || run.State == Failed || run.State == Canceled || (eventType == "run.started" && run.State != Queued) {
			return Event{}, ErrInvalid
		}
		id, err := newID()
		if err != nil {
			return Event{}, err
		}
		now := s.now().UTC()
		event := Event{ID: id, Type: eventType, ActorID: run.InitiatorID, RunID: run.ID, InitiatorID: run.InitiatorID, Agent: run.Agent, RevisionID: run.RevisionID, Metadata: metadata, CreatedAt: now}
		item.Events = append(item.Events, event)
		if nextState != "" {
			run.State = nextState
		}
		item.UpdatedAt = now
		if err := s.write(item); err != nil {
			return Event{}, err
		}
		return event, nil
	}
	return Event{}, ErrNotFound
}

func (s *Store) Publish(repositoryID, pullRequestID, sessionID, runID string, publication Publication) (Event, Run, error) {
	publication.Summary = strings.TrimSpace(publication.Summary)
	if publication.Summary == "" || len(publication.Summary) > 10000 || publication.SourceCommitID == "" || len(publication.CommitIDs) == 0 || len(publication.ChangedFiles) > 1000 || !validSummaries(publication.Checks, 100) || !validSummaries(publication.Concerns, 100) {
		return Event{}, Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, pullRequestID, sessionID)
	if err != nil {
		return Event{}, Run{}, err
	}
	for i := range item.Runs {
		run := &item.Runs[i]
		if run.ID != runID {
			continue
		}
		if run.State != Running || run.Publication != nil {
			return Event{}, Run{}, ErrInvalid
		}
		id, err := newID()
		if err != nil {
			return Event{}, Run{}, err
		}
		publication.PublishedAt = s.now().UTC()
		run.Publication = &publication
		run.State = Succeeded
		event := Event{ID: id, Type: "run.published", ActorID: run.InitiatorID, RunID: run.ID, InitiatorID: run.InitiatorID, Agent: run.Agent, RevisionID: run.RevisionID, Metadata: map[string]string{"summary": publication.Summary, "source_commit_id": publication.SourceCommitID}, CreatedAt: publication.PublishedAt}
		item.Events = append(item.Events, event)
		item.UpdatedAt = publication.PublishedAt
		if err := s.write(item); err != nil {
			return Event{}, Run{}, err
		}
		return event, *run, nil
	}
	return Event{}, Run{}, ErrNotFound
}

func validSummaries(items []string, maximum int) bool {
	if len(items) > maximum {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item) == "" || len(item) > 1000 {
			return false
		}
	}
	return true
}

type Session struct {
	ID                        string             `json:"id"`
	RepositoryID              string             `json:"repository_id"`
	PullRequestID             string             `json:"pull_request_id"`
	InitiatorID               string             `json:"initiator_id"`
	SourceCommitID            string             `json:"source_commit_id"`
	CheckFailure              *CheckFailure      `json:"check_failure,omitempty"`
	TaskContext               *TaskContext       `json:"task_context,omitempty"`
	DeploymentFailure         *DeploymentFailure `json:"deployment_failure,omitempty"`
	ContributionPullRequestID string             `json:"contribution_pull_request_id,omitempty"`
	State                     State              `json:"state"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
	Events                    []Event            `json:"events,omitempty"`
	Runs                      []Run              `json:"runs,omitempty"`
}

// DeploymentFailure captures only public deployment evidence. It deliberately
// contains no environment credential values or authority to operate delivery.
type DeploymentFailure struct {
	DeploymentID   string                    `json:"deployment_id"`
	ReleaseID      string                    `json:"release_id"`
	EnvironmentID  string                    `json:"environment_id"`
	BuildRunID     string                    `json:"build_run_id"`
	ArtifactID     string                    `json:"artifact_id"`
	ArtifactPath   string                    `json:"artifact_path"`
	ArtifactSHA256 string                    `json:"artifact_sha256"`
	SourceCommitID string                    `json:"source_commit_id"`
	State          string                    `json:"state"`
	CurrentStage   string                    `json:"current_stage,omitempty"`
	Events         []DeploymentEvidenceEvent `json:"events"`
}
type DeploymentEvidenceEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	Stage     string    `json:"stage,omitempty"`
	Signal    string    `json:"signal,omitempty"`
	Outcome   string    `json:"outcome,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// LinkTaskContribution gives pre-review execution evidence a durable backlink
// to the ordinary pull request created from it.
func (s *Store) LinkTaskContribution(repositoryID, scopeID, sessionID, pullRequestID string) (Session, error) {
	if pullRequestID == "" {
		return Session{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, scopeID, sessionID)
	if err != nil {
		return Session{}, err
	}
	if item.TaskContext == nil || (item.ContributionPullRequestID != "" && item.ContributionPullRequestID != pullRequestID) {
		return Session{}, ErrInvalid
	}
	item.ContributionPullRequestID, item.UpdatedAt = pullRequestID, s.now().UTC()
	if err := s.write(item); err != nil {
		return Session{}, err
	}
	return item, nil
}

// TaskContext is the immutable shared intent captured when an assigned plan
// task starts before a pull request exists.
type TaskContext struct {
	ProposalID          string             `json:"proposal_id"`
	ProposalTitle       string             `json:"proposal_title"`
	ProposalDescription string             `json:"proposal_description"`
	TaskID              string             `json:"task_id"`
	TaskTitle           string             `json:"task_title"`
	TaskOutcome         string             `json:"task_outcome"`
	Mandate             string             `json:"mandate"`
	Dependencies        []TaskDependency   `json:"dependencies"`
	Repository          RepositoryContext  `json:"repository"`
	ReasoningContext    *reasoning.Context `json:"reasoning_context,omitempty"`
}

type TaskDependency struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Outcome string `json:"outcome"`
	Status  string `json:"status"`
}

type RepositoryContext struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"default_branch"`
	BaseRevision  string `json:"base_revision"`
	WorkingBranch string `json:"working_branch"`
}

// CheckFailure is an immutable evidence snapshot used to start an informed
// repair. Artifact bytes remain in check-run storage and are addressed by ID.
type CheckFailure struct {
	RunID             string            `json:"run_id"`
	CommitID          string            `json:"commit_id"`
	Name              string            `json:"name"`
	Command           string            `json:"command"`
	WorkingDirectory  string            `json:"working_directory,omitempty"`
	TimeoutSeconds    int               `json:"timeout_seconds"`
	Environment       map[string]string `json:"environment,omitempty"`
	DeclaredArtifacts []string          `json:"declared_artifacts,omitempty"`
	Logs              []CheckLog        `json:"logs"`
	Artifacts         []CheckArtifact   `json:"artifacts"`
	ExitCode          int               `json:"exit_code"`
	TimedOut          bool              `json:"timed_out"`
	Error             string            `json:"error,omitempty"`
}

type CheckLog struct {
	Sequence int64  `json:"sequence"`
	Stream   string `json:"stream"`
	Message  string `json:"message"`
}

type CheckArtifact struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

type DelegateParams struct {
	InitiatorID, Agent, Instructions, RevisionID, WorkingBranch, CredentialGrantID string
	ContextPaths                                                                   []string
	CredentialExpiresAt                                                            time.Time
}

func (s *Store) Delegate(repositoryID, pullRequestID, id string, params DelegateParams) (Run, error) {
	if strings.TrimSpace(params.Instructions) == "" || len(params.Instructions) > 10000 || params.InitiatorID == "" || params.Agent == "" || params.RevisionID == "" || params.WorkingBranch == "" || params.CredentialGrantID == "" {
		return Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, pullRequestID, id)
	if err != nil {
		return Run{}, err
	}
	if item.State != AwaitingInstructions || params.RevisionID != item.SourceCommitID {
		return Run{}, ErrInvalid
	}
	runID, err := newID()
	if err != nil {
		return Run{}, err
	}
	eventID, err := newID()
	if err != nil {
		return Run{}, err
	}
	now := s.now().UTC()
	run := Run{ID: runID, InitiatorID: params.InitiatorID, Agent: params.Agent, Instructions: strings.TrimSpace(params.Instructions), RevisionID: params.RevisionID, ContextPaths: append([]string(nil), params.ContextPaths...), WorkingBranch: params.WorkingBranch, CredentialGrantID: params.CredentialGrantID, CredentialExpiresAt: params.CredentialExpiresAt, State: Queued, CreatedAt: now}
	item.Runs = append(item.Runs, run)
	item.State = Delegated
	item.UpdatedAt = now
	item.Events = append(item.Events, Event{ID: eventID, Type: "run.delegated", ActorID: params.InitiatorID, Metadata: map[string]string{"run_id": run.ID, "agent": run.Agent, "revision_id": run.RevisionID, "working_branch": run.WorkingBranch}, CreatedAt: now})
	if err := s.write(item); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *Store) RevokeRunCredential(repositoryID, pullRequestID, sessionID, runID string, at time.Time) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, pullRequestID, sessionID)
	if err != nil {
		return Run{}, err
	}
	for i := range item.Runs {
		if item.Runs[i].ID == runID {
			if item.Runs[i].CredentialRevokedAt == nil {
				v := at.UTC()
				item.Runs[i].CredentialRevokedAt = &v
				item.UpdatedAt = v
				if err := s.write(item); err != nil {
					return Run{}, err
				}
			}
			return item.Runs[i], nil
		}
	}
	return Run{}, ErrNotFound
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("change session root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(repositoryID, pullRequestID, initiatorID, sourceCommitID string) (Session, error) {
	return s.CreateWithCheckFailure(repositoryID, pullRequestID, initiatorID, sourceCommitID, nil)
}

func (s *Store) CreateForTask(repositoryID, scopeID, initiatorID, sourceCommitID string, context TaskContext) (Session, error) {
	return s.create(repositoryID, scopeID, initiatorID, sourceCommitID, nil, &context)
}

func (s *Store) CreateWithCheckFailure(repositoryID, pullRequestID, initiatorID, sourceCommitID string, failure *CheckFailure) (Session, error) {
	return s.create(repositoryID, pullRequestID, initiatorID, sourceCommitID, failure, nil)
}

func (s *Store) CreateWithDeploymentFailure(repositoryID, pullRequestID, initiatorID, sourceCommitID string, failure *DeploymentFailure) (Session, error) {
	item, err := s.create(repositoryID, pullRequestID, initiatorID, sourceCommitID, nil, nil)
	if err != nil {
		return item, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err = s.read(repositoryID, pullRequestID, item.ID)
	if err != nil {
		return item, err
	}
	item.DeploymentFailure = failure
	item.Events[0].Metadata["deployment_id"] = failure.DeploymentID
	item.Events[0].Metadata["release_id"] = failure.ReleaseID
	err = s.write(item)
	return item, err
}

func (s *Store) create(repositoryID, pullRequestID, initiatorID, sourceCommitID string, failure *CheckFailure, taskContext *TaskContext) (Session, error) {
	if repositoryID == "" || pullRequestID == "" || initiatorID == "" || sourceCommitID == "" {
		return Session{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	eventID, err := newID()
	if err != nil {
		return Session{}, err
	}
	now := s.now().UTC()
	metadata := map[string]string{"source_commit_id": sourceCommitID}
	if failure != nil {
		metadata["check_run_id"] = failure.RunID
		metadata["check_name"] = failure.Name
	}
	if taskContext != nil {
		metadata["proposal_id"] = taskContext.ProposalID
		metadata["task_id"] = taskContext.TaskID
		metadata["working_branch"] = taskContext.Repository.WorkingBranch
	}
	item := Session{ID: id, RepositoryID: repositoryID, PullRequestID: pullRequestID, InitiatorID: initiatorID, SourceCommitID: sourceCommitID, CheckFailure: failure, TaskContext: taskContext, State: AwaitingInstructions, CreatedAt: now, UpdatedAt: now, Events: []Event{{ID: eventID, Type: "session.started", ActorID: initiatorID, Metadata: metadata, CreatedAt: now}}}
	if err := s.write(item); err != nil {
		return Session{}, err
	}
	return item, nil
}

func (s *Store) Get(repositoryID, pullRequestID, id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, pullRequestID, id)
}

func (s *Store) List(repositoryID, pullRequestID string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repositoryID, pullRequestID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Session{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.read(repositoryID, pullRequestID, entry.Name()[:len(entry.Name())-5])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Store) Events(repositoryID, pullRequestID, id string) ([]Event, error) {
	item, err := s.Get(repositoryID, pullRequestID, id)
	if err != nil {
		return nil, err
	}
	return item.Events, nil
}

func (s *Store) read(repositoryID, pullRequestID, id string) (Session, error) {
	data, err := os.ReadFile(s.path(repositoryID, pullRequestID, id))
	if errors.Is(err, fs.ErrNotExist) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	var item Session
	if json.Unmarshal(data, &item) != nil || item.RepositoryID != repositoryID || item.PullRequestID != pullRequestID || item.ID != id {
		return Session{}, ErrNotFound
	}
	return item, nil
}
func (s *Store) write(item Session) error {
	dir := filepath.Dir(s.path(item.RepositoryID, item.PullRequestID, item.ID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0o640); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.path(item.RepositoryID, item.PullRequestID, item.ID))
}
func (s *Store) path(repositoryID, pullRequestID, id string) string {
	return filepath.Join(s.root, repositoryID, pullRequestID, id+".json")
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
