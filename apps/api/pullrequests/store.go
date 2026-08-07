// Package pullrequests owns durable requests to merge one repository branch into another.
package pullrequests

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
	ErrNotFound       = errors.New("pull request not found")
	ErrInvalid        = errors.New("invalid pull request")
	ErrInvalidComment = errors.New("invalid pull request comment")
)

type Status string

const Open Status = "open"

type PullRequest struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	ProposalID     string    `json:"proposal_id,omitempty"`
	AuthorID       string    `json:"author_id"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	SourceBranch   string    `json:"source_branch"`
	TargetBranch   string    `json:"target_branch"`
	SourceCommitID string    `json:"source_commit_id"`
	TargetCommitID string    `json:"target_commit_id"`
	Status         Status    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateParams struct {
	RepositoryID   string
	ProposalID     string
	AuthorID       string
	Title          string
	Body           string
	SourceBranch   string
	TargetBranch   string
	SourceCommitID string
	TargetCommitID string
}

type Comment struct {
	ID            string    `json:"id"`
	PullRequestID string    `json:"pull_request_id"`
	AuthorID      string    `json:"author_id"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("pull request storage root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve pull request root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create pull request root: %w", err)
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(params CreateParams) (PullRequest, error) {
	params.Title = strings.TrimSpace(params.Title)
	params.Body = strings.TrimSpace(params.Body)
	if params.RepositoryID == "" || params.AuthorID == "" || params.Title == "" || len(params.Title) > 200 || len(params.Body) > 65536 || params.SourceBranch == "" || params.TargetBranch == "" || params.SourceBranch == params.TargetBranch || params.SourceCommitID == "" || params.TargetCommitID == "" {
		return PullRequest{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return PullRequest{}, err
	}
	now := s.now().UTC()
	item := PullRequest{ID: id, RepositoryID: params.RepositoryID, ProposalID: params.ProposalID, AuthorID: params.AuthorID, Title: params.Title, Body: params.Body, SourceBranch: params.SourceBranch, TargetBranch: params.TargetBranch, SourceCommitID: params.SourceCommitID, TargetCommitID: params.TargetCommitID, Status: Open, CreatedAt: now, UpdatedAt: now}
	if err := s.write(item); err != nil {
		return PullRequest{}, err
	}
	return item, nil
}

func (s *Store) Get(repositoryID, id string) (PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, id)
}

func (s *Store) List(repositoryID string) ([]PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repositoryID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []PullRequest{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []PullRequest{}
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
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Store) AddComment(repositoryID, pullRequestID, authorID, body string) (Comment, error) {
	body = strings.TrimSpace(body)
	if authorID == "" || body == "" || len(body) > 65536 {
		return Comment{}, ErrInvalidComment
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(repositoryID, pullRequestID); err != nil {
		return Comment{}, err
	}
	id, err := newID()
	if err != nil {
		return Comment{}, err
	}
	comment := Comment{ID: id, PullRequestID: pullRequestID, AuthorID: authorID, Body: body, CreatedAt: s.now().UTC()}
	if err := s.writeJSON(s.commentPath(repositoryID, pullRequestID, id), comment); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

func (s *Store) ListComments(repositoryID, pullRequestID string) ([]Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(repositoryID, pullRequestID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root, repositoryID, pullRequestID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Comment{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Comment{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var comment Comment
		if json.Unmarshal(data, &comment) != nil || !validID(comment.ID) || comment.PullRequestID != pullRequestID || comment.AuthorID == "" || comment.Body == "" || comment.CreatedAt.IsZero() {
			return nil, errors.New("invalid stored pull request comment")
		}
		items = append(items, comment)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Store) read(repositoryID, id string) (PullRequest, error) {
	if !validID(id) {
		return PullRequest{}, ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(s.root, repositoryID, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return PullRequest{}, ErrNotFound
	}
	if err != nil {
		return PullRequest{}, err
	}
	var item PullRequest
	if json.Unmarshal(data, &item) != nil || item.ID != id || item.RepositoryID != repositoryID || item.AuthorID == "" || item.Title == "" || item.SourceBranch == "" || item.TargetBranch == "" || item.SourceCommitID == "" || item.TargetCommitID == "" || item.Status != Open || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return PullRequest{}, errors.New("invalid stored pull request")
	}
	return item, nil
}

func (s *Store) write(item PullRequest) error {
	return s.writeJSON(filepath.Join(s.root, item.RepositoryID, item.ID+".json"), item)
}

func (s *Store) commentPath(repositoryID, pullRequestID, id string) string {
	return filepath.Join(s.root, repositoryID, pullRequestID, id+".json")
}

func (s *Store) writeJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".pull-request-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o640); err == nil {
		_, err = temp.Write(append(data, '\n'))
	}
	if syncErr := temp.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
