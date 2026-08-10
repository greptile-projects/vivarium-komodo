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
	"strings"
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
var ErrConflict = errors.New("workspace version conflict")

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Port struct {
	Number int    `json:"number"`
	Label  string `json:"label"`
	Path   string `json:"path"`
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
	Ports        []Port         `json:"ports,omitempty"`
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
	Kind      string    `json:"kind,omitempty"`
	Surface   string    `json:"surface,omitempty"`
	Path      string    `json:"path,omitempty"`
	TargetID  string    `json:"target_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Presence struct {
	ActorID    string    `json:"actor_id"`
	Surface    string    `json:"surface"`
	Path       string    `json:"path,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type ControlGrant struct {
	ID          string    `json:"id"`
	SubjectID   string    `json:"subject_id"`
	SubjectKind string    `json:"subject_kind"`
	Mode        string    `json:"mode"`
	Scopes      []string  `json:"scopes"`
	State       string    `json:"state"`
	GrantedBy   string    `json:"granted_by"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type FileChange struct {
	Sequence  int64     `json:"sequence"`
	Path      string    `json:"path"`
	ActorID   string    `json:"actor_id"`
	Digest    string    `json:"digest,omitempty"`
	Deleted   bool      `json:"deleted,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Reproducibility struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Commands     []string `json:"commands,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}
type CheckpointChange struct {
	Path       string `json:"path"`
	Operation  string `json:"operation"`
	BaseDigest string `json:"base_digest,omitempty"`
	Digest     string `json:"digest,omitempty"`
	BlobDigest string `json:"-"`
	Patch      string `json:"patch,omitempty"`
	Binary     bool   `json:"binary,omitempty"`
	Size       int64  `json:"size,omitempty"`
}
type CheckpointStatus struct {
	Reproducible        bool     `json:"reproducible"`
	Diverged            bool     `json:"diverged"`
	Conflicts           []string `json:"conflicts"`
	MissingDependencies []string `json:"missing_dependencies"`
	Reasons             []string `json:"reasons"`
}
type Checkpoint struct {
	ID               string             `json:"id"`
	WorkspaceID      string             `json:"workspace_id"`
	RepositoryID     string             `json:"repository_id"`
	ParentID         string             `json:"parent_id,omitempty"`
	CreatorID        string             `json:"creator_id"`
	BaseRevision     string             `json:"base_revision"`
	Definition       Definition         `json:"environment_definition"`
	DefinitionDigest string             `json:"definition_digest"`
	Summary          string             `json:"summary"`
	Reproducibility  Reproducibility    `json:"reproducibility"`
	Changes          []CheckpointChange `json:"changes"`
	Status           CheckpointStatus   `json:"status"`
	CreatedAt        time.Time          `json:"created_at"`
}
type Workspace struct {
	ID               string         `json:"id"`
	RepositoryID     string         `json:"repository_id"`
	Revision         string         `json:"revision"`
	CreatorID        string         `json:"creator_id"`
	Context          SourceContext  `json:"source_context"`
	Access           Access         `json:"effective_access"`
	Definition       Definition     `json:"definition"`
	DefinitionDigest string         `json:"definition_digest"`
	State            State          `json:"state"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	SuspendedAt      *time.Time     `json:"suspended_at,omitempty"`
	ReadyAt          *time.Time     `json:"ready_at,omitempty"`
	Events           []Event        `json:"setup_evidence"`
	Activity         []Event        `json:"activity"`
	Changes          []FileChange   `json:"changes"`
	Presence         []Presence     `json:"presence"`
	Controls         []ControlGrant `json:"controls"`
	Checkpoints      []Checkpoint   `json:"checkpoints"`
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Observe refreshes an intentionally short presence lease. Disconnects disappear
// predictably without rewriting the durable activity ledger.
func (s *Store) Observe(repositoryID, id, actor, surface, path string) (Workspace, error) {
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
	kept := w.Presence[:0]
	for _, p := range w.Presence {
		if p.ExpiresAt.After(now) && p.ActorID != actor {
			kept = append(kept, p)
		}
	}
	w.Presence = append(kept, Presence{ActorID: actor, Surface: surface, Path: path, ObservedAt: now, ExpiresAt: now.Add(45 * time.Second)})
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "presence", Kind: "observation", Surface: surface, Path: path, ActorID: actor, CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) AddMessage(repositoryID, id, actor, message string) (Workspace, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4000 {
		return Workspace{}, ErrConflict
	}
	return s.RecordActivity(repositoryID, id, Event{Type: "message", Kind: "instruction", Surface: "discussion", Message: message, ActorID: actor})
}

func (s *Store) Grant(repositoryID, id, actor, subject, kind, mode string, scopes []string) (Workspace, error) {
	validMode := mode == "observe" || mode == "edit" || mode == "execute"
	if subject == "" || (kind != "human" && kind != "approved_agent") || !validMode || len(scopes) == 0 {
		return Workspace{}, ErrConflict
	}
	allowed := map[string]bool{"files": true, "terminal": true, "preview": true}
	seen := map[string]bool{}
	for _, scope := range scopes {
		if !allowed[scope] || seen[scope] {
			return Workspace{}, ErrConflict
		}
		seen[scope] = true
	}
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
	grantID, err := newID()
	if err != nil {
		return Workspace{}, err
	}
	g := ControlGrant{ID: grantID, SubjectID: subject, SubjectKind: kind, Mode: mode, Scopes: append([]string(nil), scopes...), State: "active", GrantedBy: actor, Version: 1, CreatedAt: now, UpdatedAt: now}
	w.Controls = append(w.Controls, g)
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "control_granted", Kind: "instruction", ActorID: actor, TargetID: subject, Message: mode + ":" + strings.Join(scopes, ","), CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) Intervene(repositoryID, id, actor, grantID, action, message string, version int64) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	index := -1
	for i := range w.Controls {
		if w.Controls[i].ID == grantID {
			index = i
			break
		}
	}
	if index < 0 {
		return Workspace{}, ErrNotFound
	}
	g := &w.Controls[index]
	if g.Version != version || g.State == "revoked" {
		return Workspace{}, ErrConflict
	}
	switch action {
	case "guide":
		if strings.TrimSpace(message) == "" {
			return Workspace{}, ErrConflict
		}
	case "pause":
		g.State = "paused"
	case "resume", "take_over":
		g.State = "active"
	case "revoke":
		g.State = "revoked"
	default:
		return Workspace{}, ErrConflict
	}
	now := s.now().UTC()
	g.Version++
	g.UpdatedAt = now
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: action, Kind: "instruction", ActorID: actor, TargetID: g.SubjectID, Message: strings.TrimSpace(message), CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
}

func (s *Store) RecordActivity(repositoryID, id string, event Event) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(id)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	event.Sequence = int64(len(w.Activity) + 1)
	event.CreatedAt = s.now().UTC()
	if len(event.Message) > 1<<20 {
		event.Message = event.Message[:1<<20]
	}
	w.Activity = append(w.Activity, event)
	w.UpdatedAt = event.CreatedAt
	return w, s.write(w)
}

func (s *Store) RecordChange(repositoryID, id, actor, path, digest string, deleted bool) (Workspace, error) {
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
	w.Changes = append(w.Changes, FileChange{Sequence: int64(len(w.Changes) + 1), Path: path, ActorID: actor, Digest: digest, Deleted: deleted, CreatedAt: now})
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "file", Kind: "authorship", Surface: "files", Path: path, ActorID: actor, Message: map[bool]string{true: "deleted", false: "saved"}[deleted], CreatedAt: now})
	w.UpdatedAt = now
	return w, s.write(w)
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
	if err = os.MkdirAll(filepath.Join(abs, "checkpoint-blobs"), 0o750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) AddCheckpoint(repositoryID, workspaceID string, checkpoint Checkpoint, blobs map[string][]byte) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, err := s.read(workspaceID)
	if err != nil || w.RepositoryID != repositoryID {
		return Workspace{}, ErrNotFound
	}
	if w.State != Ready {
		return Workspace{}, ErrInvalidTransition
	}
	for _, existing := range w.Checkpoints {
		if existing.ID == checkpoint.ParentID {
			checkpoint.ParentID = existing.ID
			break
		}
	}
	if checkpoint.ParentID != "" {
		found := false
		for _, existing := range w.Checkpoints {
			found = found || existing.ID == checkpoint.ParentID
		}
		if !found {
			return Workspace{}, ErrConflict
		}
	}
	for digest, data := range blobs {
		path := filepath.Join(s.root, "checkpoint-blobs", digest)
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			if err = os.WriteFile(path, data, 0o640); err != nil {
				return Workspace{}, err
			}
		}
	}
	w.Checkpoints = append(w.Checkpoints, checkpoint)
	now := s.now().UTC()
	w.UpdatedAt = now
	w.Activity = append(w.Activity, Event{Sequence: int64(len(w.Activity) + 1), Type: "checkpoint", Kind: "authorship", ActorID: checkpoint.CreatorID, TargetID: checkpoint.ID, Message: checkpoint.Summary, CreatedAt: now})
	return w, s.write(w)
}

func (s *Store) Blob(digest string) ([]byte, error) {
	if len(digest) != 64 {
		return nil, ErrNotFound
	}
	return os.ReadFile(filepath.Join(s.root, "checkpoint-blobs", digest))
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
	w.Presence = activePresence(w.Presence, s.now().UTC())
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
			w.Presence = activePresence(w.Presence, s.now().UTC())
			out = append(out, w)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func activePresence(items []Presence, now time.Time) []Presence {
	out := make([]Presence, 0, len(items))
	for _, item := range items {
		if item.ExpiresAt.After(now) {
			out = append(out, item)
		}
	}
	return out
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
