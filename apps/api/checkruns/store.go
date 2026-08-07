// Package checkruns owns durable, commit-bound executions of repository-defined checks.
package checkruns

import (
	"crypto/rand"
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
)

type Definition struct {
	Name             string            `json:"name"`
	Command          string            `json:"command"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
	Environment      map[string]string `json:"environment,omitempty"`
}

type Run struct {
	ID            string     `json:"id"`
	RepositoryID  string     `json:"repository_id"`
	PullRequestID string     `json:"pull_request_id"`
	CommitID      string     `json:"commit_id"`
	Definition    Definition `json:"definition"`
	State         State      `json:"state"`
	ExitCode      *int       `json:"exit_code,omitempty"`
	Error         string     `json:"error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
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
	s.mu.Lock()
	defer s.mu.Unlock()
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Run{}, err
	}
	run := Run{ID: hex.EncodeToString(idBytes), RepositoryID: repositoryID, PullRequestID: pullRequestID, CommitID: commitID, Definition: definition, State: Queued, CreatedAt: s.now().UTC()}
	return run, s.write(run)
}

func (s *Store) Start(id string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil || run.State != Queued {
		return Run{}, err
	}
	now := s.now().UTC()
	run.State, run.StartedAt = Running, &now
	return run, s.write(run)
}

func (s *Store) Complete(id string, exitCode int, message string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, err := s.read(id)
	if err != nil {
		return Run{}, err
	}
	now := s.now().UTC()
	run.ExitCode, run.CompletedAt, run.Error = &exitCode, &now, message
	if exitCode == 0 {
		run.State = Succeeded
	} else {
		run.State = Failed
	}
	return run, s.write(run)
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
