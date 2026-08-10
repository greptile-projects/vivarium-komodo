// Package workspaces owns durable, exact-revision development environments.
package workspaces

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

type State string

const (
	SettingUp State = "setting_up"
	Ready     State = "ready"
	Failed    State = "failed"
	Suspended State = "suspended"
)

var ErrNotFound = errors.New("workspace not found")
var ErrInvalidTransition = errors.New("invalid workspace transition")

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type ResourceLimits struct {
	CPUSeconds          int `json:"cpu_seconds"`
	MemoryMB            int `json:"memory_mb"`
	DiskMB              int `json:"disk_mb"`
	SetupTimeoutSeconds int `json:"setup_timeout_seconds"`
}
type Definition struct {
	Version      int            `json:"version"`
	Tools        []Tool         `json:"tools"`
	Dependencies []string       `json:"dependencies"`
	Setup        []string       `json:"setup"`
	Resources    ResourceLimits `json:"resources"`
}
type SourceContext struct {
	Type     string `json:"type"`
	ID       string `json:"id,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}
type Access struct {
	RepositoryID string `json:"repository_id"`
	ActorID      string `json:"actor_id"`
	Permission   string `json:"permission"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	State     State     `json:"state,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	ActorID   string    `json:"actor_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Workspace struct {
	ID               string        `json:"id"`
	RepositoryID     string        `json:"repository_id"`
	Revision         string        `json:"revision"`
	CreatorID        string        `json:"creator_id"`
	Context          SourceContext `json:"source_context"`
	Access           Access        `json:"effective_access"`
	Definition       Definition    `json:"definition"`
	DefinitionDigest string        `json:"definition_digest"`
	State            State         `json:"state"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	SuspendedAt      *time.Time    `json:"suspended_at,omitempty"`
	ReadyAt          *time.Time    `json:"ready_at,omitempty"`
	Events           []Event       `json:"setup_evidence"`
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
	if err = os.MkdirAll(filepath.Join(abs, "records"), 0o750); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(abs, "environments"), 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}
func (s *Store) Environment(id string) string { return filepath.Join(s.root, "environments", id) }

func (s *Store) Create(repositoryID, revision, creatorID string, context SourceContext, access Access, definition Definition, digest string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Workspace{}, err
	}
	now := s.now().UTC()
	w := Workspace{ID: hex.EncodeToString(b), RepositoryID: repositoryID, Revision: revision, CreatorID: creatorID, Context: context, Access: access, Definition: definition, DefinitionDigest: digest, State: SettingUp, CreatedAt: now, UpdatedAt: now}
	w.append(Event{Type: "state", State: SettingUp, ActorID: creatorID, CreatedAt: now})
	return w, s.write(w)
}
func (s *Store) Get(repositoryID, id string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	return w, nil
}
func (s *Store) List(repositoryID string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, "records"))
	if err != nil {
		return nil, err
	}
	out := []Workspace{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		w, e := s.read(entry.Name()[:len(entry.Name())-5])
		if e == nil && w.RepositoryID == repositoryID {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Append(id string, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil {
		return err
	}
	if event.Type == "log" {
		const maximumLogBytes = 10 << 20
		used := 0
		for _, existing := range w.Events {
			if existing.Type == "log" {
				used += len(existing.Message)
			}
		}
		if used >= maximumLogBytes {
			return nil
		}
		if len(event.Message) > maximumLogBytes-used {
			event.Message = event.Message[:maximumLogBytes-used]
		}
	}
	event.CreatedAt = s.now().UTC()
	w.append(event)
	w.UpdatedAt = event.CreatedAt
	return s.write(w)
}
func (s *Store) Finish(id string, success bool, message string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil {
		return Workspace{}, err
	}
	if w.State != SettingUp {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	if success {
		w.State = Ready
		w.ReadyAt = &now
	} else {
		w.State = Failed
	}
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: w.State, Message: message, CreatedAt: now})
	return w, s.write(w)
}
func (s *Store) Suspend(repositoryID, id, actor string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	w.State = Suspended
	w.SuspendedAt = &now
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: Suspended, ActorID: actor, CreatedAt: now})
	return w, s.write(w)
}
func (s *Store) Resume(repositoryID, id, actor, digest string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Suspended || w.DefinitionDigest != digest {
		return Workspace{}, ErrInvalidTransition
	}
	if info, e := os.Stat(s.Environment(id)); e != nil || !info.IsDir() {
		return Workspace{}, ErrInvalidTransition
	}
	now := s.now().UTC()
	w.State = Ready
	w.SuspendedAt = nil
	w.UpdatedAt = now
	w.append(Event{Type: "state", State: Ready, ActorID: actor, Message: "resumed retained exact-revision environment", CreatedAt: now})
	return w, s.write(w)
}
func (w *Workspace) append(e Event) {
	e.Sequence = int64(len(w.Events) + 1)
	w.Events = append(w.Events, e)
}
func (s *Store) read(id string) (Workspace, error) {
	data, err := os.ReadFile(filepath.Join(s.root, "records", id+".json"))
	if os.IsNotExist(err) {
		return Workspace{}, ErrNotFound
	}
	var w Workspace
	if err == nil {
		err = json.Unmarshal(data, &w)
	}
	return w, err
}
func (s *Store) write(w Workspace) error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.root, "records", w.ID+".json")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".workspace-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0o640)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	return err
}
