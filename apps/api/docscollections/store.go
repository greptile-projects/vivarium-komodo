// Package documentation owns repository-backed documentation collection contracts.
package docscollections

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("documentation collection not found")
	ErrInvalid  = errors.New("invalid documentation collection")
	ErrConflict = errors.New("documentation collection changed")
)

type TaskOrigin struct {
	Kind           string `json:"kind"`
	ResourceID     string `json:"resource_id"`
	ParentID       string `json:"parent_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
}
type CodeReference struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Revision  string `json:"revision"`
	BlobID    string `json:"blob_id"`
	Excerpt   string `json:"excerpt"`
}
type TaskEvent struct {
	Sequence    int64           `json:"sequence"`
	Type        string          `json:"type"`
	ActorID     string          `json:"actor_id"`
	Body        string          `json:"body,omitempty"`
	Draft       string          `json:"draft,omitempty"`
	Rendered    string          `json:"rendered,omitempty"`
	References  []CodeReference `json:"references,omitempty"`
	Citations   []string        `json:"citations,omitempty"`
	Uncertainty string          `json:"uncertainty,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}
type Task struct {
	ID                string      `json:"id"`
	RepositoryID      string      `json:"repository_id"`
	CollectionID      string      `json:"collection_id"`
	CollectionVersion int64       `json:"collection_version"`
	Title             string      `json:"title"`
	Path              string      `json:"path"`
	Origin            TaskOrigin  `json:"origin"`
	Revision          string      `json:"revision"`
	Evidence          []string    `json:"evidence"`
	Mode              string      `json:"mode"`
	Branch            string      `json:"branch,omitempty"`
	WorkspaceID       string      `json:"workspace_id,omitempty"`
	CreatorID         string      `json:"creator_id"`
	Events            []TaskEvent `json:"events"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type VersionMapping struct {
	Label          string `json:"label"`
	SourceRevision string `json:"source_revision"`
	ReleaseID      string `json:"release_id,omitempty"`
}
type Link struct {
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	ResourceID string `json:"resource_id,omitempty"`
	Path       string `json:"path,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
}
type Policy struct {
	Navigation  string `json:"navigation"`
	Renderer    string `json:"renderer"`
	Publication string `json:"publication"`
}
type Version struct {
	Number       int64            `json:"number"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	RootPath     string           `json:"root_path"`
	EntryPaths   []string         `json:"entry_paths"`
	Versions     []VersionMapping `json:"versions"`
	OwnerIDs     []string         `json:"owner_ids"`
	Audiences    []string         `json:"audiences"`
	Policy       Policy           `json:"policy"`
	Links        []Link           `json:"links"`
	AuthorID     string           `json:"author_id"`
	ChangeReason string           `json:"change_reason"`
	CreatedAt    time.Time        `json:"created_at"`
}
type Collection struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	History        []Version `json:"history"`
}
type Input struct {
	ExpectedVersion int64            `json:"expected_version"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	RootPath        string           `json:"root_path"`
	EntryPaths      []string         `json:"entry_paths"`
	Versions        []VersionMapping `json:"versions"`
	OwnerIDs        []string         `json:"owner_ids"`
	Audiences       []string         `json:"audiences"`
	Policy          Policy           `json:"policy"`
	Links           []Link           `json:"links"`
	ChangeReason    string           `json:"change_reason"`
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func validPath(p string) bool {
	p = strings.Trim(p, "/")
	return p != "" && p != "." && !strings.Contains(p, "..") && !strings.Contains(p, "\\")
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || !validPath(in.RootPath) || len(in.EntryPaths) == 0 || len(in.EntryPaths) > 100 || len(in.Versions) == 0 || len(in.Versions) > 30 || len(in.Audiences) == 0 || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	if in.Policy.Navigation != "manual" && in.Policy.Navigation != "path" {
		return false
	}
	if in.Policy.Renderer != "markdown" && in.Policy.Renderer != "plain_text" {
		return false
	}
	if in.Policy.Publication != "maintainer_reviewed" && in.Policy.Publication != "owner_reviewed" {
		return false
	}
	for _, p := range in.EntryPaths {
		if !validPath(p) {
			return false
		}
	}
	for _, v := range in.Versions {
		if strings.TrimSpace(v.Label) == "" || len(v.SourceRevision) != 40 {
			return false
		}
	}
	return true
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in Input) (Collection, error) {
	if repo == "" || actor == "" || in.ExpectedVersion != 0 || !valid(in) {
		return Collection{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c := Collection{ID: id(), RepositoryID: repo}
	return s.add(c, actor, in)
}
func (s *Store) Update(repo, cid, actor string, in Input) (Collection, error) {
	if !valid(in) {
		return Collection{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	if c.CurrentVersion != in.ExpectedVersion {
		return c, ErrConflict
	}
	return s.add(c, actor, in)
}
func (s *Store) add(c Collection, actor string, in Input) (Collection, error) {
	v := Version{Number: c.CurrentVersion + 1, Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), RootPath: strings.Trim(in.RootPath, "/"), EntryPaths: in.EntryPaths, Versions: in.Versions, OwnerIDs: in.OwnerIDs, Audiences: in.Audiences, Policy: in.Policy, Links: in.Links, AuthorID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: s.now().UTC()}
	c.CurrentVersion = v.Number
	c.History = append(c.History, v)
	return c, s.write(c)
}
func (s *Store) Get(repo, id string) (Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Collection{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Collection{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		c, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Store) CreateTask(repo, collection, actor, title, page, revision string, origin TaskOrigin, evidence []string, mode, branch string) (Task, error) {
	if repo == "" || actor == "" || strings.TrimSpace(title) == "" || len(revision) != 40 || (mode != "branch" && mode != "workspace") || !validPath(page) || origin.Kind == "" || origin.ResourceID == "" || mode == "branch" && strings.TrimSpace(branch) == "" {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.read(repo, collection)
	if err != nil {
		return Task{}, err
	}
	cur := c.History[len(c.History)-1]
	full := strings.Trim(strings.Trim(cur.RootPath, "/")+"/"+strings.Trim(page, "/"), "/")
	if !strings.HasPrefix(full+"/", strings.Trim(cur.RootPath, "/")+"/") {
		return Task{}, ErrInvalid
	}
	now := s.now().UTC()
	t := Task{ID: id(), RepositoryID: repo, CollectionID: collection, CollectionVersion: c.CurrentVersion, Title: strings.TrimSpace(title), Path: full, Origin: origin, Revision: revision, Evidence: evidence, Mode: mode, Branch: strings.TrimSpace(branch), CreatorID: actor, CreatedAt: now, UpdatedAt: now, Events: []TaskEvent{{Sequence: 1, Type: "opened", ActorID: actor, Citations: evidence, CreatedAt: now}}}
	return t, s.writeTask(t)
}
func (s *Store) SetTaskWorkspace(repo, task, workspace string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := s.readTask(repo, task)
	if e != nil {
		return t, e
	}
	t.WorkspaceID = workspace
	t.UpdatedAt = s.now().UTC()
	return t, s.writeTask(t)
}
func (s *Store) GetTask(repo, task string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readTask(repo, task)
}
func (s *Store) ListTasks(repo, collection string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, "tasks")
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []Task{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Task{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		t, e := s.readTask(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		if collection == "" || t.CollectionID == collection {
			out = append(out, t)
		}
	}
	return out, nil
}
func (s *Store) AddTaskEvent(repo, task, actor string, event TaskEvent) (Task, error) {
	if actor == "" || (event.Type != "discussion" && event.Type != "suggestion" && event.Type != "draft") || strings.TrimSpace(event.Body) == "" && strings.TrimSpace(event.Draft) == "" {
		return Task{}, ErrInvalid
	}
	if event.Type == "suggestion" && len(event.Citations) == 0 {
		return Task{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := s.readTask(repo, task)
	if e != nil {
		return t, e
	}
	event.Sequence = int64(len(t.Events) + 1)
	event.ActorID = actor
	event.CreatedAt = s.now().UTC()
	t.Events = append(t.Events, event)
	t.UpdatedAt = event.CreatedAt
	return t, s.writeTask(t)
}
func (s *Store) readTask(repo, task string) (Task, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, "tasks", task+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Task{}, ErrNotFound
	}
	var t Task
	if e == nil {
		e = json.Unmarshal(b, &t)
	}
	return t, e
}
func (s *Store) writeTask(t Task) error {
	d := filepath.Join(s.root, t.RepositoryID, "tasks")
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(t, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".task-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, t.ID+".json"))
	}
	return e
}
func (s *Store) read(repo, id string) (Collection, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Collection{}, ErrNotFound
	}
	var c Collection
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func (s *Store) write(c Collection) error {
	d := filepath.Join(s.root, c.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".docs-")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, c.ID+".json"))
	}
	return e
}
