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
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("change session not found")
	ErrInvalid  = errors.New("invalid change session")
)

type State string

const AwaitingInstructions State = "awaiting_instructions"

type Event struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	ActorID   string            `json:"actor_id"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

type Session struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	PullRequestID  string    `json:"pull_request_id"`
	InitiatorID    string    `json:"initiator_id"`
	SourceCommitID string    `json:"source_commit_id"`
	State          State     `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Events         []Event   `json:"events,omitempty"`
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
	item := Session{ID: id, RepositoryID: repositoryID, PullRequestID: pullRequestID, InitiatorID: initiatorID, SourceCommitID: sourceCommitID, State: AwaitingInstructions, CreatedAt: now, UpdatedAt: now, Events: []Event{{ID: eventID, Type: "session.started", ActorID: initiatorID, Metadata: map[string]string{"source_commit_id": sourceCommitID}, CreatedAt: now}}}
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
