// Package checkruns owns durable, commit-bound executions of repository-defined checks.
package checkruns

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type State string

const (
	Queued    State = "queued"
	Running   State = "running"
	Succeeded State = "succeeded"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

var ErrInvalidTransition = errors.New("invalid check run transition")

type Definition struct {
	Name             string             `json:"name"`
	Kind             string             `json:"kind,omitempty"`
	Command          string             `json:"command"`
	WorkingDirectory string             `json:"working_directory,omitempty"`
	TimeoutSeconds   int                `json:"timeout_seconds"`
	Environment      map[string]string  `json:"environment,omitempty"`
	Artifacts        []string           `json:"artifacts,omitempty"`
	Dependencies     []string           `json:"dependencies,omitempty"`
	Documentation    *DocumentationSpec `json:"documentation,omitempty"`
	Accessibility    *AccessibilitySpec `json:"accessibility,omitempty"`
}

// AccessibilitySpec describes exactly which behavior automation can inspect;
// anything outside Evaluations remains visible as a human-evaluation gap.
type AccessibilitySpec struct {
	ScenarioIDs             []string `json:"scenario_ids"`
	Evaluations             []string `json:"evaluations"`
	Inputs                  []string `json:"inputs"`
	AffectedAudiences       []string `json:"affected_audiences"`
	RequiresHumanEvaluation []string `json:"requires_human_evaluation,omitempty"`
	InputDigest             string   `json:"input_digest,omitempty"`
	ReusedFromRunID         string   `json:"reused_from_run_id,omitempty"`
}

// DocumentationSpec makes the evidence behind a documentation check
// inspectable without requiring reviewers to interpret its command or logs.
type DocumentationSpec struct {
	Kind            string                 `json:"kind"`
	CollectionID    string                 `json:"collection_id"`
	Inputs          []string               `json:"inputs"`
	Pages           []string               `json:"pages"`
	Symbols         []string               `json:"symbols,omitempty"`
	Links           []string               `json:"links,omitempty"`
	Versions        []DocumentationVersion `json:"versions"`
	ExpectedOutput  string                 `json:"expected_output,omitempty"`
	Coverage        map[string]int         `json:"coverage,omitempty"`
	InputDigest     string                 `json:"input_digest,omitempty"`
	ReusedFromRunID string                 `json:"reused_from_run_id,omitempty"`
}

type DocumentationVersion struct {
	Label        string `json:"label"`
	SourceCommit string `json:"source_commit,omitempty"`
	Package      string `json:"package,omitempty"`
	ReleaseID    string `json:"release_id,omitempty"`
}

type Revision struct {
	RepositoryID string `json:"repository_id"`
	CommitID     string `json:"commit_id"`
}

type Event struct {
	Sequence  int64           `json:"sequence"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Status    State           `json:"status,omitempty"`
	Stream    string          `json:"stream,omitempty"`
	Message   string          `json:"message,omitempty"`
	Outcome   *CommandOutcome `json:"outcome,omitempty"`
	Artifact  *Artifact       `json:"artifact,omitempty"`
	ActorID   string          `json:"actor_id,omitempty"`
}

type CommandOutcome struct {
	ExitCode int  `json:"exit_code"`
	TimedOut bool `json:"timed_out"`
}

type Artifact struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
}

type Run struct {
	ID                 string     `json:"id"`
	RepositoryID       string     `json:"repository_id"`
	SourceRepositoryID string     `json:"source_repository_id"`
	PullRequestID      string     `json:"pull_request_id"`
	CommitID           string     `json:"commit_id"`
	Definition         Definition `json:"definition"`
	Revisions          []Revision `json:"revisions,omitempty"`
	TriggeredByID      string     `json:"triggered_by_id,omitempty"`
	RetryOfID          string     `json:"retry_of_id,omitempty"`
	CanceledByID       string     `json:"canceled_by_id,omitempty"`
	State              State      `json:"state"`
	ExitCode           *int       `json:"exit_code,omitempty"`
	Error              string     `json:"error,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	Events             []Event    `json:"events"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("check run root is required")
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

func (s *Store) Create(repositoryID, pullRequestID, commitID string, definition Definition) (Run, error) {
	return s.CreateForSource(repositoryID, repositoryID, pullRequestID, commitID, definition)
}

func (s *Store) CreateForSource(repositoryID, sourceRepositoryID, pullRequestID, commitID string, definition Definition) (Run, error) {
	return s.createAttempt(repositoryID, sourceRepositoryID, pullRequestID, commitID, definition, "", "")
}

func (s *Store) CreateAttempt(repositoryID, pullRequestID, commitID string, definition Definition, actorID, retryOfID string) (Run, error) {
	return s.createAttempt(repositoryID, repositoryID, pullRequestID, commitID, definition, actorID, retryOfID)
}

func (s *Store) createAttempt(repositoryID, sourceRepositoryID, pullRequestID, commitID string, definition Definition, actorID, retryOfID string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Run{}, err
	}
	now := s.now().UTC()
	run := Run{ID: hex.EncodeToString(idBytes), RepositoryID: repositoryID, SourceRepositoryID: sourceRepositoryID, PullRequestID: pullRequestID, CommitID: commitID, Definition: definition, State: Queued, TriggeredByID: actorID, RetryOfID: retryOfID, CreatedAt: now}
	run.Events = []Event{{Sequence: 1, Type: "status", Timestamp: now, Status: Queued, ActorID: actorID}}
	return run, s.write(run)
}

func (s *Store) Start(id string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return Run{}, err
	}
	if run.State != Queued {
		return Run{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	run.State, run.StartedAt = Running, &now
	run.append(Event{Type: "status", Timestamp: now, Status: Running})
	return run, s.write(run)
}

func (s *Store) AppendLog(id, stream, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return err
	}
	if run.State != Running {
		return ErrInvalidTransition
	}
	if stream != "stdout" && stream != "stderr" {
		return errors.New("invalid log stream")
	}
	const maximumLogBytes = 10 << 20
	used := 0
	for _, event := range run.Events {
		if event.Type == "log" {
			used += len(event.Message)
		}
	}
	if used >= maximumLogBytes {
		return nil
	}
	if len(message) > maximumLogBytes-used {
		message = message[:maximumLogBytes-used]
	}
	run.append(Event{Type: "log", Timestamp: s.now().UTC(), Stream: stream, Message: message})
	return s.write(run)
}

func (s *Store) Complete(id string, exitCode int, timedOut bool, message string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return Run{}, err
	}
	if run.State != Running {
		return Run{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	run.ExitCode, run.CompletedAt, run.Error = &exitCode, &now, message
	run.append(Event{Type: "command", Timestamp: now, Outcome: &CommandOutcome{ExitCode: exitCode, TimedOut: timedOut}})
	if exitCode == 0 {
		run.State = Succeeded
	} else {
		run.State = Failed
	}
	run.append(Event{Type: "status", Timestamp: now, Status: run.State, Message: message})
	return run, s.write(run)
}

func (s *Store) Cancel(id, actorID string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return Run{}, err
	}
	if run.State != Queued && run.State != Running {
		return Run{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	run.State, run.CanceledByID, run.CompletedAt = Canceled, actorID, &now
	run.append(Event{Type: "status", Timestamp: now, Status: Canceled, ActorID: actorID, Message: "check canceled"})
	return run, s.write(run)
}

func (s *Store) AddArtifact(id, path, mediaType string, content []byte) (Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return Artifact{}, err
	}
	if run.State != Running {
		return Artifact{}, ErrInvalidTransition
	}
	contentDigest := sha256.Sum256(content)
	identity := sha256.New()
	_, _ = identity.Write([]byte(path))
	_, _ = identity.Write([]byte{0})
	_, _ = identity.Write(content)
	artifact := Artifact{ID: hex.EncodeToString(identity.Sum(nil)), Path: path, Size: int64(len(content)), SHA256: hex.EncodeToString(contentDigest[:]), MediaType: mediaType}
	dir := filepath.Join(s.root, "artifacts", id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return Artifact{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, artifact.ID), content, 0o640); err != nil {
		return Artifact{}, err
	}
	run.append(Event{Type: "artifact", Timestamp: s.now().UTC(), Artifact: &artifact})
	return artifact, s.write(run)
}

func (s *Store) Get(repositoryID, pullRequestID, id string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err == nil && (run.RepositoryID != repositoryID || run.PullRequestID != pullRequestID) {
		err = os.ErrNotExist
	}
	return run, err
}

func (s *Store) OpenArtifact(repositoryID, pullRequestID, runID, artifactID string) (Artifact, *os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(runID)
	if err != nil || run.RepositoryID != repositoryID || run.PullRequestID != pullRequestID {
		return Artifact{}, nil, os.ErrNotExist
	}
	for _, event := range run.Events {
		if event.Artifact != nil && event.Artifact.ID == artifactID {
			file, err := os.Open(filepath.Join(s.root, "artifacts", runID, artifactID))
			return *event.Artifact, file, err
		}
	}
	return Artifact{}, nil, os.ErrNotExist
}

func (r *Run) append(event Event) {
	event.Sequence = int64(len(r.Events) + 1)
	r.Events = append(r.Events, event)
}

func (s *Store) List(repositoryID, pullRequestID string) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	items := []Run{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		run, err := s.read(entry.Name()[:len(entry.Name())-5])
		if err != nil {
			return nil, err
		}
		if run.RepositoryID == repositoryID && run.PullRequestID == pullRequestID {
			items = append(items, run)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }
func (s *Store) read(id string) (Run, error) {
	var run Run
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return run, err
	}
	err = json.Unmarshal(data, &run)
	// Runs created before fork-based verification sourced their snapshot from
	// the repository that owns the pull request.
	if run.SourceRepositoryID == "" {
		run.SourceRepositoryID = run.RepositoryID
	}
	return run, err
}
func (s *Store) write(run Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(s.root, ".run-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err = temp.Chmod(0o640); err == nil {
		_, err = temp.Write(data)
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.path(run.ID))
}
