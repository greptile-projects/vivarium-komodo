// Package proposals owns durable repository proposal and discussion records.
package proposals

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
	ErrNotFound       = errors.New("proposal not found")
	ErrInvalid        = errors.New("invalid proposal")
	ErrInvalidComment = errors.New("invalid proposal comment")
)

type State string

const (
	Open   State = "open"
	Closed State = "closed"
)

type Proposal struct {
	ID           string     `json:"id"`
	RepositoryID string     `json:"repository_id"`
	AuthorID     string     `json:"author_id"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	State        State      `json:"state"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	ClosedByID   string     `json:"closed_by_id,omitempty"`
}

type Comment struct {
	ID         string    `json:"id"`
	ProposalID string    `json:"proposal_id"`
	AuthorID   string    `json:"author_id"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("proposal storage root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve proposal root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("create proposal root: %w", err)
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(repositoryID, authorID, title, body string) (Proposal, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if repositoryID == "" || authorID == "" || title == "" || len(title) > 200 || len(body) > 65536 {
		return Proposal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Proposal{}, err
	}
	now := s.now().UTC()
	p := Proposal{ID: id, RepositoryID: repositoryID, AuthorID: authorID, Title: title, Body: body, State: Open, CreatedAt: now, UpdatedAt: now}
	if err := s.writeJSON(s.proposalPath(repositoryID, id), p, true); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

func (s *Store) Get(repositoryID, id string) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readProposal(repositoryID, id)
}

func (s *Store) List(repositoryID string) ([]Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repositoryID)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Proposal{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Proposal{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		item, err := s.readProposal(repositoryID, strings.TrimSuffix(entry.Name(), ".json"))
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

func (s *Store) Update(repositoryID, id, title, body string) (Proposal, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if title == "" || len(title) > 200 || len(body) > 65536 {
		return Proposal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readProposal(repositoryID, id)
	if err != nil {
		return Proposal{}, err
	}
	p.Title, p.Body, p.UpdatedAt = title, body, s.now().UTC()
	if err := s.writeJSON(s.proposalPath(repositoryID, id), p, false); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

func (s *Store) Close(repositoryID, id, actorID string) (Proposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.readProposal(repositoryID, id)
	if err != nil {
		return Proposal{}, err
	}
	if p.State == Closed {
		return p, nil
	}
	now := s.now().UTC()
	p.State, p.ClosedAt, p.ClosedByID, p.UpdatedAt = Closed, &now, actorID, now
	if err := s.writeJSON(s.proposalPath(repositoryID, id), p, false); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

func (s *Store) AddComment(repositoryID, proposalID, authorID, body string) (Comment, error) {
	body = strings.TrimSpace(body)
	if authorID == "" || body == "" || len(body) > 65536 {
		return Comment{}, ErrInvalidComment
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readProposal(repositoryID, proposalID); err != nil {
		return Comment{}, err
	}
	id, err := newID()
	if err != nil {
		return Comment{}, err
	}
	c := Comment{ID: id, ProposalID: proposalID, AuthorID: authorID, Body: body, CreatedAt: s.now().UTC()}
	if err := s.writeJSON(s.commentPath(repositoryID, proposalID, id), c, true); err != nil {
		return Comment{}, err
	}
	return c, nil
}

func (s *Store) ListComments(repositoryID, proposalID string) ([]Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.readProposal(repositoryID, proposalID); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root, repositoryID, proposalID)
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
		var c Comment
		if json.Unmarshal(data, &c) != nil || c.ProposalID != proposalID || c.AuthorID == "" || c.CreatedAt.IsZero() {
			return nil, errors.New("invalid stored proposal comment")
		}
		items = append(items, c)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Store) proposalPath(repositoryID, id string) string {
	return filepath.Join(s.root, repositoryID, id+".json")
}
func (s *Store) commentPath(repositoryID, proposalID, id string) string {
	return filepath.Join(s.root, repositoryID, proposalID, id+".json")
}

func (s *Store) readProposal(repositoryID, id string) (Proposal, error) {
	if !validID(id) {
		return Proposal{}, ErrNotFound
	}
	data, err := os.ReadFile(s.proposalPath(repositoryID, id))
	if errors.Is(err, fs.ErrNotExist) {
		return Proposal{}, ErrNotFound
	}
	if err != nil {
		return Proposal{}, err
	}
	var p Proposal
	if json.Unmarshal(data, &p) != nil || p.ID != id || p.RepositoryID != repositoryID || p.AuthorID == "" || p.Title == "" || (p.State != Open && p.State != Closed) || p.CreatedAt.IsZero() {
		return Proposal{}, errors.New("invalid stored proposal")
	}
	return p, nil
}

func (s *Store) writeJSON(path string, value any, exclusive bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".proposal-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o640); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if exclusive {
		if _, err := os.Stat(path); err == nil {
			return errors.New("proposal already exists")
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
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
