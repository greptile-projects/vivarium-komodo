// Package integrationqueue owns durable, ordered admission to protected target branches.
package integrationqueue

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

var (
	ErrNotFound = errors.New("queue entry not found")
	ErrConflict = errors.New("pull request is already queued")
)

type Entry struct {
	ID                string     `json:"id"`
	RepositoryID      string     `json:"repository_id"`
	PullRequestID     string     `json:"pull_request_id"`
	TargetBranch      string     `json:"target_branch"`
	SourceCommitID    string     `json:"source_commit_id"`
	TargetCommitID    string     `json:"target_commit_id"`
	CandidateCommitID string     `json:"candidate_commit_id"`
	CandidateTreeID   string     `json:"candidate_tree_id"`
	RequiredChecks    []string   `json:"required_checks"`
	EnqueuedByID      string     `json:"enqueued_by_id"`
	Position          int        `json:"position"`
	State             string     `json:"state"`
	Reason            string     `json:"reason,omitempty"`
	Generation        int        `json:"generation"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
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
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}
func (s *Store) Enqueue(repositoryID, pullID, branch, source, target, candidate, tree, actor string, required []string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.list(repositoryID, branch)
	if err != nil {
		return Entry{}, err
	}
	for _, v := range items {
		if v.PullRequestID == pullID {
			return Entry{}, ErrConflict
		}
	}
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return Entry{}, err
	}
	now := s.now().UTC()
	e := Entry{ID: hex.EncodeToString(b), RepositoryID: repositoryID, PullRequestID: pullID, TargetBranch: branch, SourceCommitID: source, TargetCommitID: target, CandidateCommitID: candidate, CandidateTreeID: tree, RequiredChecks: append([]string(nil), required...), EnqueuedByID: actor, Position: len(items) + 1, State: "verifying", Generation: 1, CreatedAt: now, UpdatedAt: now}
	dir := filepath.Join(s.root, repositoryID)
	if err = os.MkdirAll(dir, 0750); err != nil {
		return Entry{}, err
	}
	return e, s.write(e)
}

// ReplaceCandidate supersedes an entry's old evidence with a new generation.
// Check attempts remain durable in the check-run store, while only the current
// generation is eligible to advance the branch.
func (s *Store) ReplaceCandidate(id, target, candidate, tree string, required []string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.read(id)
	if err != nil {
		return Entry{}, err
	}
	e.TargetCommitID, e.CandidateCommitID, e.CandidateTreeID = target, candidate, tree
	e.RequiredChecks, e.State, e.Reason = append([]string(nil), required...), "verifying", ""
	e.Generation++
	e.UpdatedAt, e.CompletedAt = s.now().UTC(), nil
	return e, s.write(e)
}

func (s *Store) Transition(id, state, reason string, terminal bool) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.read(id)
	if err != nil {
		return Entry{}, err
	}
	e.State, e.Reason, e.UpdatedAt = state, reason, s.now().UTC()
	if terminal {
		completed := e.UpdatedAt
		e.CompletedAt = &completed
	}
	return e, s.write(e)
}

func (s *Store) Get(id string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

func (s *Store) ListActive() ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, dir := range root {
		if !dir.IsDir() {
			continue
		}
		files, readErr := os.ReadDir(filepath.Join(s.root, dir.Name()))
		if readErr != nil {
			return nil, readErr
		}
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
				continue
			}
			e, readErr := s.read(strings.TrimSuffix(file.Name(), ".json"))
			if readErr != nil {
				return nil, readErr
			}
			if e.CompletedAt == nil {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func (s *Store) read(id string) (Entry, error) {
	matches, err := filepath.Glob(filepath.Join(s.root, "*", id+".json"))
	if err != nil || len(matches) != 1 {
		return Entry{}, ErrNotFound
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return Entry{}, err
	}
	var e Entry
	if json.Unmarshal(data, &e) != nil {
		return Entry{}, ErrNotFound
	}
	return e, nil
}

func (s *Store) write(e Entry) error {
	dir := filepath.Join(s.root, e.RepositoryID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(e, "", "  ")
	tmp, err := os.CreateTemp(dir, ".queue-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0640); err == nil {
		_, err = tmp.Write(data)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(dir, e.ID+".json"))
	}
	return err
}
func (s *Store) List(repositoryID, branch string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repositoryID, branch)
}
func (s *Store) list(repositoryID, branch string) ([]Entry, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if errors.Is(err, os.ErrNotExist) {
		return []Entry{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Entry{}
	for _, f := range entries {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		data, e := os.ReadFile(filepath.Join(s.root, repositoryID, f.Name()))
		if e != nil {
			return nil, e
		}
		var v Entry
		if json.Unmarshal(data, &v) == nil && v.RepositoryID == repositoryID && v.TargetBranch == branch && v.CompletedAt == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	for i := range out {
		out[i].Position = i + 1
	}
	return out, nil
}
