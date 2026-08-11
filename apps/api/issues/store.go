// Package issues owns durable, attributable repository problem reports.
package issues

import (
	"crypto/rand"
	"encoding/base64"
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
)

var (
	ErrNotFound = errors.New("issue not found")
	ErrInvalid  = errors.New("invalid issue")
)

type Attachment struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Content   string `json:"content"`
}
type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Issue struct {
	ID                string       `json:"id"`
	RepositoryID      string       `json:"repository_id"`
	ReporterID        string       `json:"reporter_id"`
	Title             string       `json:"title"`
	ExpectedBehavior  string       `json:"expected_behavior"`
	ObservedBehavior  string       `json:"observed_behavior"`
	Severity          string       `json:"severity"`
	Environment       string       `json:"environment"`
	ReproductionSteps []string     `json:"reproduction_steps"`
	AffectedReleaseID string       `json:"affected_release_id,omitempty"`
	AffectedVersion   string       `json:"affected_version,omitempty"`
	AffectedCommitID  string       `json:"affected_commit_id,omitempty"`
	Visibility        string       `json:"visibility"`
	Status            string       `json:"status"`
	Attachments       []Attachment `json:"attachments"`
	Comments          []Comment    `json:"discussion"`
	History           []Event      `json:"history"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}
type CreateInput struct {
	RepositoryID, ReporterID, Title, ExpectedBehavior, ObservedBehavior, Severity, Environment, AffectedReleaseID, AffectedVersion, AffectedCommitID, Visibility string
	ReproductionSteps                                                                                                                                            []string
	Attachments                                                                                                                                                  []Attachment
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	root, _ = filepath.Abs(root)
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}

func validate(in CreateInput) bool {
	if strings.TrimSpace(in.RepositoryID) == "" || strings.TrimSpace(in.ReporterID) == "" || strings.TrimSpace(in.Title) == "" || len(in.Title) > 200 || strings.TrimSpace(in.ExpectedBehavior) == "" || strings.TrimSpace(in.ObservedBehavior) == "" || strings.TrimSpace(in.Environment) == "" {
		return false
	}
	if in.Severity != "low" && in.Severity != "medium" && in.Severity != "high" && in.Severity != "critical" {
		return false
	}
	if in.Visibility != "public" && in.Visibility != "repository" {
		return false
	}
	if len(in.ReproductionSteps) == 0 || len(in.ReproductionSteps) > 50 || len(in.Attachments) > 10 {
		return false
	}
	total := 0
	allowed := map[string]bool{"log": true, "screenshot": true, "trace": true, "sample_input": true}
	media := map[string]map[string]bool{
		"log":          {"text/plain": true, "application/json": true, "application/octet-stream": true},
		"trace":        {"text/plain": true, "application/json": true, "application/octet-stream": true},
		"sample_input": {"text/plain": true, "application/json": true, "application/octet-stream": true},
		"screenshot":   {"image/png": true, "image/jpeg": true, "image/webp": true},
	}
	for _, a := range in.Attachments {
		decoded, err := base64.StdEncoding.DecodeString(a.Content)
		total += len(decoded)
		if err != nil || !allowed[a.Kind] || !media[a.Kind][a.MediaType] || strings.TrimSpace(a.Name) == "" || len(a.Name) > 200 || a.Content == "" || len(decoded) > 1<<20 {
			return false
		}
	}
	return total <= 5<<20
}
func (s *Store) Create(in CreateInput) (Issue, error) {
	if !validate(in) {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Issue{}, err
	}
	now := s.now().UTC()
	for i := range in.Attachments {
		in.Attachments[i].ID, _ = newID()
	}
	item := Issue{ID: id, RepositoryID: in.RepositoryID, ReporterID: in.ReporterID, Title: strings.TrimSpace(in.Title), ExpectedBehavior: strings.TrimSpace(in.ExpectedBehavior), ObservedBehavior: strings.TrimSpace(in.ObservedBehavior), Severity: in.Severity, Environment: strings.TrimSpace(in.Environment), ReproductionSteps: in.ReproductionSteps, AffectedReleaseID: in.AffectedReleaseID, AffectedVersion: in.AffectedVersion, AffectedCommitID: in.AffectedCommitID, Visibility: in.Visibility, Status: "open", Attachments: in.Attachments, Comments: []Comment{}, History: []Event{{Sequence: 1, Type: "issue.opened", ActorID: in.ReporterID, CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
	return item, s.write(item)
}
func (s *Store) Get(repo, id string) (Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(err, fs.ErrNotExist) {
		return []Issue{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Issue{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		v, er := s.read(repo, strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) AddComment(repo, id, actor, body string) (Issue, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 65536 {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, err
	}
	cid, _ := newID()
	now := s.now().UTC()
	v.Comments = append(v.Comments, Comment{ID: cid, AuthorID: actor, Body: body, CreatedAt: now})
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "comment.added", ActorID: actor, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) SetStatus(repo, id, actor, status string) (Issue, error) {
	if status != "open" && status != "closed" {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, err
	}
	if v.Status == status {
		return v, nil
	}
	now := s.now().UTC()
	v.Status = status
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "status." + status, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Issue, error) {
	b, err := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Issue{}, ErrNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	var v Issue
	if json.Unmarshal(b, &v) != nil || v.ID != id || v.RepositoryID != repo {
		return Issue{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Issue) error {
	dir := filepath.Join(s.root, v.RepositoryID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".issue-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Chmod(0640)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, v.ID+".json"))
}
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
