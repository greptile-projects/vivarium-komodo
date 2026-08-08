// Package releases owns immutable, versioned release candidates.
package releases

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound        = errors.New("release candidate not found")
	ErrInvalid         = errors.New("invalid release candidate")
	ErrVersionConflict = errors.New("release version already exists")
)

type Status string

const Candidate Status = "candidate"

type PullRequestLink struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	AuthorID      string `json:"author_id"`
	MergeCommitID string `json:"merge_commit_id"`
}

// Release is an immutable source and inclusion snapshot. Later delivery rungs
// may evolve Status while preserving this captured definition.
type Release struct {
	ID             string            `json:"id"`
	RepositoryID   string            `json:"repository_id"`
	Version        string            `json:"version"`
	Notes          string            `json:"notes"`
	CommitID       string            `json:"commit_id"`
	PriorReleaseID string            `json:"prior_release_id,omitempty"`
	PriorCommitID  string            `json:"prior_commit_id,omitempty"`
	Status         Status            `json:"status"`
	CreatedByID    string            `json:"created_by_id"`
	CreatedAt      time.Time         `json:"created_at"`
	PullRequests   []PullRequestLink `json:"pull_requests"`
	ProposalIDs    []string          `json:"proposal_ids"`
	TaskIDs        []string          `json:"task_ids"`
	ContributorIDs []string          `json:"contributor_ids"`
}

type CreateParams struct {
	RepositoryID, Version, Notes, CommitID, PriorReleaseID, PriorCommitID, CreatedByID string
	PullRequests                                                                       []PullRequestLink
	ProposalIDs, TaskIDs, ContributorIDs                                               []string
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("release storage root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create release root: %w", err)
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(p CreateParams) (Release, error) {
	p.Version, p.Notes = strings.TrimSpace(p.Version), strings.TrimSpace(p.Notes)
	if p.RepositoryID == "" || p.CreatedByID == "" || p.CommitID == "" || p.Version == "" || len(p.Version) > 100 || len(p.Notes) > 65536 || (p.PriorReleaseID == "") != (p.PriorCommitID == "") {
		return Release{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.list(p.RepositoryID)
	if err != nil {
		return Release{}, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Version, p.Version) {
			return Release{}, ErrVersionConflict
		}
	}
	id, err := newID()
	if err != nil {
		return Release{}, err
	}
	item := Release{ID: id, RepositoryID: p.RepositoryID, Version: p.Version, Notes: p.Notes, CommitID: p.CommitID, PriorReleaseID: p.PriorReleaseID, PriorCommitID: p.PriorCommitID, Status: Candidate, CreatedByID: p.CreatedByID, CreatedAt: s.now().UTC(), PullRequests: nonNil(p.PullRequests), ProposalIDs: unique(p.ProposalIDs), TaskIDs: unique(p.TaskIDs), ContributorIDs: unique(p.ContributorIDs)}
	if err := s.write(item); err != nil {
		return Release{}, err
	}
	return item, nil
}

func (s *Store) Get(repositoryID, id string) (Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, id)
}
func (s *Store) List(repositoryID string) ([]Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repositoryID)
}

func (s *Store) list(repositoryID string) ([]Release, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if errors.Is(err, fs.ErrNotExist) {
		return []Release{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Release{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.read(repositoryID, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}
func (s *Store) read(repositoryID, id string) (Release, error) {
	data, err := os.ReadFile(filepath.Join(s.root, repositoryID, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Release{}, ErrNotFound
	}
	if err != nil {
		return Release{}, err
	}
	var item Release
	if json.Unmarshal(data, &item) != nil || item.RepositoryID != repositoryID || item.ID != id {
		return Release{}, ErrNotFound
	}
	return item, nil
}
func (s *Store) write(item Release) error {
	dir := filepath.Join(s.root, item.RepositoryID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".release-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err = tmp.Write(append(data, '\n')); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, item.ID+".json"))
}
func unique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
func nonNil(v []PullRequestLink) []PullRequestLink {
	if v == nil {
		return []PullRequestLink{}
	}
	return v
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(b)
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:], nil
}
