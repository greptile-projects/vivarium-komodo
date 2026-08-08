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
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("queue entry not found")
	ErrConflict = errors.New("pull request is already queued")
)

type Entry struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	PullRequestID  string    `json:"pull_request_id"`
	TargetBranch   string    `json:"target_branch"`
	SourceCommitID string    `json:"source_commit_id"`
	TargetCommitID string    `json:"target_commit_id"`
	EnqueuedByID   string    `json:"enqueued_by_id"`
	Position       int       `json:"position"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"created_at"`
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
func (s *Store) Enqueue(repositoryID, pullID, branch, source, target, actor string) (Entry, error) {
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
	e := Entry{ID: hex.EncodeToString(b), RepositoryID: repositoryID, PullRequestID: pullID, TargetBranch: branch, SourceCommitID: source, TargetCommitID: target, EnqueuedByID: actor, Position: len(items) + 1, State: "queued", CreatedAt: s.now().UTC()}
	dir := filepath.Join(s.root, repositoryID)
	if err = os.MkdirAll(dir, 0750); err != nil {
		return Entry{}, err
	}
	data, _ := json.MarshalIndent(e, "", "  ")
	tmp, err := os.CreateTemp(dir, ".queue-")
	if err != nil {
		return Entry{}, err
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
	return e, err
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
		if json.Unmarshal(data, &v) == nil && v.RepositoryID == repositoryID && v.TargetBranch == branch && v.State == "queued" {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	for i := range out {
		out[i].Position = i + 1
	}
	return out, nil
}
