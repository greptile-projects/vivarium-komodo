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
	ErrInvalidReview  = errors.New("invalid pull request review")
)

type Status string

const (
	Open   Status = "open"
	Merged Status = "merged"
)

type PullRequest struct {
	ID             string     `json:"id"`
	RepositoryID   string     `json:"repository_id"`
	ProposalID     string     `json:"proposal_id,omitempty"`
	AuthorID       string     `json:"author_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	SourceBranch   string     `json:"source_branch"`
	TargetBranch   string     `json:"target_branch"`
	SourceCommitID string     `json:"source_commit_id"`
	TargetCommitID string     `json:"target_commit_id"`
	Status         Status     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	MergedAt       *time.Time `json:"merged_at,omitempty"`
	MergedByID     string     `json:"merged_by_id,omitempty"`
	MergeCommitID  string     `json:"merge_commit_id,omitempty"`
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

type ReviewDecision string

const (
	Approve        ReviewDecision = "approve"
	RequestChanges ReviewDecision = "request_changes"
)

// Review is the reviewer's current decision and the exact source commit it evaluates.
type Review struct {
	PullRequestID string         `json:"pull_request_id"`
	ReviewerID    string         `json:"reviewer_id"`
	Decision      ReviewDecision `json:"decision"`
	CommitID      string         `json:"commit_id"`
	SubmittedAt   time.Time      `json:"submitted_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
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

func (s *Store) PutReview(repositoryID, pullRequestID, reviewerID string, decision ReviewDecision, commitID string) (Review, error) {
	if !validPathKey(reviewerID) || commitID == "" || (decision != Approve && decision != RequestChanges) {
		return Review{}, ErrInvalidReview
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(repositoryID, pullRequestID); err != nil {
		return Review{}, err
	}
	now := s.now().UTC()
	review := Review{PullRequestID: pullRequestID, ReviewerID: reviewerID, Decision: decision, CommitID: commitID, SubmittedAt: now, UpdatedAt: now}
	path := s.reviewPath(repositoryID, pullRequestID, reviewerID)
	if previous, err := s.readReview(path, pullRequestID, reviewerID); err == nil {
		review.SubmittedAt = previous.SubmittedAt
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Review{}, err
	}
	if err := s.writeJSON(path, review); err != nil {
		return Review{}, err
	}
	return review, nil
}

func (s *Store) DeleteReview(repositoryID, pullRequestID, reviewerID string) error {
	if !validPathKey(reviewerID) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(repositoryID, pullRequestID); err != nil {
		return err
	}
	if err := os.Remove(s.reviewPath(repositoryID, pullRequestID, reviewerID)); errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	} else {
		return err
	}
}

func (s *Store) ListReviews(repositoryID, pullRequestID string) ([]Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(repositoryID, pullRequestID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root, repositoryID, pullRequestID, "reviews")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Review{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Review, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		reviewerID := strings.TrimSuffix(entry.Name(), ".json")
		review, err := s.readReview(filepath.Join(dir, entry.Name()), pullRequestID, reviewerID)
		if err != nil {
			return nil, err
		}
		items = append(items, review)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ReviewerID < items[j].ReviewerID })
	return items, nil
}

// MarkMerged records the terminal outcome after the target reference has been
// advanced. Repeating the operation with the same result is idempotent.
func (s *Store) MarkMerged(repositoryID, id, actorID, commitID string) (PullRequest, error) {
	if actorID == "" || commitID == "" {
		return PullRequest{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, err := s.read(repositoryID, id)
	if err != nil {
		return PullRequest{}, err
	}
	if item.Status == Merged {
		if item.MergedByID == actorID && item.MergeCommitID == commitID {
			return item, nil
		}
		return PullRequest{}, ErrInvalid
	}
	now := s.now().UTC()
	item.Status, item.MergedAt, item.MergedByID, item.MergeCommitID, item.UpdatedAt = Merged, &now, actorID, commitID, now
	if err := s.write(item); err != nil {
		return PullRequest{}, err
	}
	return item, nil
}

func (s *Store) readReview(path, pullRequestID, reviewerID string) (Review, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Review{}, err
	}
	var review Review
	if json.Unmarshal(data, &review) != nil || review.PullRequestID != pullRequestID || review.ReviewerID != reviewerID || review.CommitID == "" || (review.Decision != Approve && review.Decision != RequestChanges) || review.SubmittedAt.IsZero() || review.UpdatedAt.IsZero() {
		return Review{}, errors.New("invalid stored pull request review")
	}
	return review, nil
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
	if json.Unmarshal(data, &item) != nil || item.ID != id || item.RepositoryID != repositoryID || item.AuthorID == "" || item.Title == "" || item.SourceBranch == "" || item.TargetBranch == "" || item.SourceCommitID == "" || item.TargetCommitID == "" || (item.Status != Open && item.Status != Merged) || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() || (item.Status == Merged && (item.MergedAt == nil || item.MergedByID == "" || item.MergeCommitID == "")) {
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

func (s *Store) reviewPath(repositoryID, pullRequestID, reviewerID string) string {
	return filepath.Join(s.root, repositoryID, pullRequestID, "reviews", reviewerID+".json")
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

func validPathKey(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`)
}
