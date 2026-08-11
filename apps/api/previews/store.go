// Package previews owns durable, exact-revision pull request preview attempts.
package previews

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

const ManifestPath = ".komodo/previews.json"

type Resources struct {
	CPUSeconds          int `json:"cpu_seconds"`
	MemoryMB            int `json:"memory_mb"`
	DiskMB              int `json:"disk_mb"`
	BuildTimeoutSeconds int `json:"build_timeout_seconds"`
	LifetimeMinutes     int `json:"lifetime_minutes"`
}
type Definition struct {
	Version       int            `json:"version"`
	Build         []string       `json:"build"`
	Start         string         `json:"start"`
	Port          int            `json:"port"`
	Configuration []string       `json:"configuration,omitempty"`
	Resources     Resources      `json:"resources"`
	Audience      AudiencePolicy `json:"audience"`
}
type AudiencePolicy struct {
	Network  string   `json:"network"`
	Data     string   `json:"data"`
	Identity string   `json:"identity"`
	Actions  []string `json:"actions"`
}
type Invitation struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Role        string     `json:"role"`
	SourceKind  string     `json:"source_kind"`
	SourceID    string     `json:"source_id,omitempty"`
	InvitedByID string     `json:"invited_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	RevokedByID string     `json:"revoked_by_id,omitempty"`
}
type AccessEvent struct {
	Sequence     int64     `json:"sequence"`
	Type         string    `json:"type"`
	ActorID      string    `json:"actor_id"`
	InvitationID string    `json:"invitation_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Attestation struct {
	CommitID            string `json:"commit_id"`
	DefinitionDigest    string `json:"definition_digest"`
	ConfigurationDigest string `json:"configuration_digest"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	State     string    `json:"state,omitempty"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  *int      `json:"exit_code,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Preview struct {
	ID                 string            `json:"id"`
	RepositoryID       string            `json:"repository_id"`
	SourceRepositoryID string            `json:"source_repository_id"`
	PullRequestID      string            `json:"pull_request_id"`
	Revision           string            `json:"revision"`
	CreatorID          string            `json:"creator_id"`
	Definition         Definition        `json:"definition"`
	Attestation        Attestation       `json:"build_attestation"`
	Configuration      map[string]string `json:"-"`
	State              string            `json:"state"`
	URL                string            `json:"url,omitempty"`
	Stale              bool              `json:"stale"`
	Failure            string            `json:"failure,omitempty"`
	Events             []Event           `json:"events"`
	CreatedAt          time.Time         `json:"created_at"`
	ReadyAt            *time.Time        `json:"ready_at,omitempty"`
	StoppedAt          *time.Time        `json:"stopped_at,omitempty"`
	ExpiresAt          time.Time         `json:"expires_at"`
	LocalPort          int               `json:"local_port,omitempty"`
	Invitations        []Invitation      `json:"invitations"`
	AccessEvents       []AccessEvent     `json:"access_events"`
}

var ErrNotFound = errors.New("preview not found")

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("preview root required")
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	if e != nil {
		return nil, e
	}
	return &Store{root: p, now: time.Now}, nil
}
func (s *Store) Environment(id string) string { return filepath.Join(s.root, "environments", id) }
func (s *Store) Create(p Preview) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return p, e
	}
	now := s.now().UTC()
	p.ID = hex.EncodeToString(b)
	p.State = "setting_up"
	p.CreatedAt = now
	p.ExpiresAt = now.Add(time.Duration(p.Definition.Resources.LifetimeMinutes) * time.Minute)
	p.Events = []Event{{Sequence: 1, Type: "state", State: p.State, CreatedAt: now}}
	return p, s.write(p)
}
func (s *Store) Get(repo, pull, id string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	return p, nil
}
func (s *Store) GetByID(id string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}
func (s *Store) List(repo, pull string) ([]Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Preview{}
	for _, v := range entries {
		if v.IsDir() || filepath.Ext(v.Name()) != ".json" {
			continue
		}
		p, er := s.read(strings.TrimSuffix(v.Name(), ".json"))
		if er == nil && p.RepositoryID == repo && p.PullRequestID == pull {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Append(id string, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, er := s.read(id)
	if er != nil {
		return er
	}
	e.Sequence = int64(len(p.Events) + 1)
	e.CreatedAt = s.now().UTC()
	p.Events = append(p.Events, e)
	return s.write(p)
}
func (s *Store) Transition(id, state, url, failure string, port int) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil {
		return p, e
	}
	now := s.now().UTC()
	p.State = state
	p.URL = url
	p.Failure = failure
	p.LocalPort = port
	if state == "ready" {
		p.ReadyAt = &now
	}
	if state == "failed" || state == "stopped" || state == "expired" {
		p.StoppedAt = &now
	}
	p.Events = append(p.Events, Event{Sequence: int64(len(p.Events) + 1), Type: "state", State: state, Message: failure, CreatedAt: now})
	return p, s.write(p)
}
func (s *Store) Invite(repo, pull, id, actor string, in Invitation) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	if in.UserID == "" || (in.Role != "view" && in.Role != "test" && in.Role != "feedback") || in.ExpiresAt.Before(s.now()) || in.ExpiresAt.After(p.ExpiresAt) {
		return Preview{}, errors.New("invalid invitation")
	}
	b := make([]byte, 12)
	if _, e = rand.Read(b); e != nil {
		return Preview{}, e
	}
	now := s.now().UTC()
	in.ID, in.InvitedByID, in.CreatedAt = hex.EncodeToString(b), actor, now
	p.Invitations = append(p.Invitations, in)
	p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "invited", ActorID: actor, InvitationID: in.ID, CreatedAt: now})
	return p, s.write(p)
}
func (s *Store) Revoke(repo, pull, id, invitation, actor string) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, ErrNotFound
	}
	now := s.now().UTC()
	for i := range p.Invitations {
		if p.Invitations[i].ID == invitation && p.Invitations[i].RevokedAt == nil {
			p.Invitations[i].RevokedAt = &now
			p.Invitations[i].RevokedByID = actor
			p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "revoked", ActorID: actor, InvitationID: invitation, CreatedAt: now})
			return p, s.write(p)
		}
	}
	return Preview{}, ErrNotFound
}
func (s *Store) Authorize(repo, pull, id, user string) (Preview, Invitation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(id)
	if e != nil || p.RepositoryID != repo || p.PullRequestID != pull {
		return Preview{}, Invitation{}, ErrNotFound
	}
	now := s.now().UTC()
	for _, in := range p.Invitations {
		if in.UserID == user && in.RevokedAt == nil && now.Before(in.ExpiresAt) {
			p.AccessEvents = append(p.AccessEvents, AccessEvent{Sequence: int64(len(p.AccessEvents) + 1), Type: "entered", ActorID: user, InvitationID: in.ID, CreatedAt: now})
			return p, in, s.write(p)
		}
	}
	return Preview{}, Invitation{}, ErrNotFound
}
func (s *Store) read(id string) (Preview, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(e) {
		return Preview{}, ErrNotFound
	}
	var p Preview
	if e != nil || json.Unmarshal(b, &p) != nil || p.ID != id {
		return Preview{}, ErrNotFound
	}
	return p, nil
}
func (s *Store) write(p Preview) error {
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+p.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, filepath.Join(s.root, p.ID+".json"))
	}
	return e
}
