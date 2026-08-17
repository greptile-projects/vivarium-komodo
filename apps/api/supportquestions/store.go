// Package supportquestions owns durable, audience-scoped developer support threads.
package supportquestions

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
	ErrNotFound = errors.New("support question not found")
	ErrInvalid  = errors.New("invalid support question")
)

type Subject struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Label      string `json:"label,omitempty"`
}
type Contact struct {
	Preference string `json:"preference"`
	Value      string `json:"value,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	MediaType  string `json:"media_type"`
	Content    string `json:"content,omitempty"`
	Visibility string `json:"visibility"`
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
type Related struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type Question struct {
	ID              string     `json:"id"`
	RepositoryID    string     `json:"repository_id"`
	AuthorID        string     `json:"author_id"`
	Title           string     `json:"title"`
	Question        string     `json:"question"`
	Subject         Subject    `json:"subject"`
	SoftwareVersion string     `json:"software_version,omitempty"`
	Environment     string     `json:"environment,omitempty"`
	Goal            string     `json:"goal"`
	AttemptedSteps  []string   `json:"attempted_steps"`
	Urgency         string     `json:"urgency"`
	Audience        string     `json:"audience"`
	Contact         Contact    `json:"contact"`
	Status          string     `json:"status"`
	MissingContext  []string   `json:"missing_context"`
	Evidence        []Evidence `json:"evidence"`
	Discussion      []Comment  `json:"discussion"`
	History         []Event    `json:"history"`
	Related         []Related  `json:"related"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
type Input struct {
	Title           string     `json:"title"`
	Question        string     `json:"question"`
	Subject         Subject    `json:"subject"`
	SoftwareVersion string     `json:"software_version"`
	Environment     string     `json:"environment"`
	Goal            string     `json:"goal"`
	AttemptedSteps  []string   `json:"attempted_steps"`
	Urgency         string     `json:"urgency"`
	Audience        string     `json:"audience"`
	Contact         Contact    `json:"contact"`
	Evidence        []Evidence `json:"evidence"`
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

func missing(in Input) []string {
	out := []string{}
	if strings.TrimSpace(in.SoftwareVersion) == "" {
		out = append(out, "software_version")
	}
	if strings.TrimSpace(in.Environment) == "" {
		out = append(out, "environment")
	}
	if len(cleanSteps(in.AttemptedSteps)) == 0 {
		out = append(out, "attempted_steps")
	}
	return out
}
func cleanSteps(in []string) []string {
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || len(in.Title) > 200 || strings.TrimSpace(in.Question) == "" || strings.TrimSpace(in.Goal) == "" || len(in.AttemptedSteps) > 50 {
		return false
	}
	if !map[string]bool{"repository": true, "package": true, "release": true, "api": true, "journey": true, "error": true}[in.Subject.Kind] || (in.Subject.Kind != "repository" && strings.TrimSpace(in.Subject.ResourceID) == "") {
		return false
	}
	if !map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}[in.Urgency] || !map[string]bool{"public": true, "repository": true}[in.Audience] {
		return false
	}
	if !map[string]bool{"none": true, "thread": true, "email": true}[in.Contact.Preference] || (in.Contact.Preference == "email" && !strings.Contains(in.Contact.Value, "@")) {
		return false
	}
	if len(in.Evidence) > 10 {
		return false
	}
	total := 0
	for _, e := range in.Evidence {
		b, err := base64.StdEncoding.DecodeString(e.Content)
		total += len(b)
		if err != nil || len(b) == 0 || len(b) > 1<<20 || strings.TrimSpace(e.Name) == "" || !map[string]bool{"log": true, "configuration": true, "sample_code": true}[e.Kind] || !map[string]bool{"text/plain": true, "application/json": true, "application/yaml": true, "text/yaml": true, "text/x-go": true, "text/javascript": true, "text/typescript": true}[e.MediaType] || !map[string]bool{"audience": true, "maintainers": true}[e.Visibility] {
			return false
		}
	}
	return total <= 5<<20
}
func (s *Store) Create(repo, actor string, in Input) (Question, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, e := newID()
	if e != nil {
		return Question{}, e
	}
	now := s.now().UTC()
	gaps := missing(in)
	status := "open"
	if len(gaps) > 0 {
		status = "needs_context"
	}
	for i := range in.Evidence {
		in.Evidence[i].ID, _ = newID()
	}
	v := Question{ID: id, RepositoryID: repo, AuthorID: actor, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), Subject: in.Subject, SoftwareVersion: strings.TrimSpace(in.SoftwareVersion), Environment: strings.TrimSpace(in.Environment), Goal: strings.TrimSpace(in.Goal), AttemptedSteps: cleanSteps(in.AttemptedSteps), Urgency: in.Urgency, Audience: in.Audience, Contact: in.Contact, Status: status, MissingContext: gaps, Evidence: in.Evidence, Discussion: []Comment{}, History: []Event{{Sequence: 1, Type: "question.opened", ActorID: actor, CreatedAt: now}}, Related: []Related{}, Version: 1, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Question{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Question{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Comment(repo, id, actor, body string) (Question, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 65536 {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	cid, _ := newID()
	now := s.now().UTC()
	v.Discussion = append(v.Discussion, Comment{ID: cid, AuthorID: actor, Body: body, CreatedAt: now})
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "comment.added", ActorID: actor, CreatedAt: now})
	v.Version++
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Status(repo, id, actor, status string) (Question, error) {
	if !map[string]bool{"open": true, "needs_context": true, "answered": true, "closed": true}[status] {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if v.Status == status {
		return v, nil
	}
	now := s.now().UTC()
	v.Status = status
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "status." + status, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) SetRelated(repo, id string, related []Related) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	v.Related = related
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Question, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Question{}, ErrNotFound
	}
	if e != nil {
		return Question{}, e
	}
	var v Question
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo || v.ID != id {
		return Question{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Question) error {
	dir := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".support-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Chmod(0640)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(dir, v.ID+".json"))
}
func newID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
