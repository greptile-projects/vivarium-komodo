// Package federatedagents owns home-instance execution records for agents collaborating on remote pull requests.
package federatedagents

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
	ErrNotFound = errors.New("federated agent session not found")
	ErrInvalid  = errors.New("invalid federated agent session")
)

type Context struct {
	TargetPullReference       string   `json:"target_pull_reference"`
	SourcePullReference       string   `json:"source_pull_reference"`
	RemoteRepositoryReference string   `json:"remote_repository_reference"`
	Revision                  string   `json:"revision"`
	Branch                    string   `json:"branch"`
	Paths                     []string `json:"paths"`
	Evidence                  []string `json:"evidence,omitempty"`
}
type Event struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}
type Publication struct {
	Summary          string            `json:"summary"`
	Commands         []string          `json:"commands"`
	Evidence         []string          `json:"evidence"`
	Costs            map[string]string `json:"costs,omitempty"`
	ResidualConcerns []string          `json:"residual_concerns"`
	CommitIDs        []string          `json:"commit_ids"`
	ChangedFiles     []string          `json:"changed_files"`
	SourceCommitID   string            `json:"source_commit_id"`
	PublishedAt      time.Time         `json:"published_at"`
}
type Session struct {
	ID                  string       `json:"id"`
	RepositoryID        string       `json:"repository_id"`
	InitiatorID         string       `json:"initiator_id"`
	Agent               string       `json:"agent"`
	Purpose             string       `json:"purpose"`
	Instructions        string       `json:"instructions"`
	Context             Context      `json:"context"`
	CredentialGrantID   string       `json:"credential_grant_id"`
	CredentialExpiresAt time.Time    `json:"credential_expires_at"`
	CredentialRevokedAt *time.Time   `json:"credential_revoked_at,omitempty"`
	State               string       `json:"state"`
	Events              []Event      `json:"events,omitempty"`
	Publication         *Publication `json:"publication,omitempty"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}
type CreateParams struct {
	RepositoryID, InitiatorID, Agent, Purpose, Instructions, CredentialGrantID string
	CredentialExpiresAt                                                        time.Time
	Context                                                                    Context
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("federated agent session root is required")
	}
	a, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(a, 0750); e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now}, nil
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return "fas_" + hex.EncodeToString(b), nil
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) write(v Session) error {
	if e := os.MkdirAll(filepath.Dir(s.path(v.RepositoryID, v.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.RepositoryID, v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, s.path(v.RepositoryID, v.ID))
}
func (s *Store) read(repo, id string) (Session, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Session{}, ErrNotFound
	}
	if e != nil {
		return Session{}, e
	}
	var v Session
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) Create(p CreateParams) (Session, error) {
	if p.RepositoryID == "" || p.InitiatorID == "" || p.Agent == "" || strings.TrimSpace(p.Instructions) == "" || p.Context.Revision == "" || p.Context.Branch == "" || p.Context.TargetPullReference == "" || p.CredentialGrantID == "" {
		return Session{}, ErrInvalid
	}
	id, e := newID()
	if e != nil {
		return Session{}, e
	}
	now := s.now().UTC()
	v := Session{ID: id, RepositoryID: p.RepositoryID, InitiatorID: p.InitiatorID, Agent: p.Agent, Purpose: p.Purpose, Instructions: strings.TrimSpace(p.Instructions), Context: p.Context, CredentialGrantID: p.CredentialGrantID, CredentialExpiresAt: p.CredentialExpiresAt, State: "delegated", CreatedAt: now, UpdatedAt: now, Events: []Event{{ID: id + "-created", Type: "session.delegated", CreatedAt: now}}}
	s.mu.Lock()
	defer s.mu.Unlock()
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Session{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Session{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *Store) Event(repo, id, typ string, metadata map[string]string) (Session, error) {
	if typ != "run.started" && typ != "finding" && typ != "command.completed" && typ != "evidence.observed" && typ != "run.failed" {
		return Session{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if v.State == "revoked" || v.State == "published" || v.State == "failed" {
		return Session{}, ErrInvalid
	}
	eid, e := newID()
	if e != nil {
		return v, e
	}
	now := s.now().UTC()
	v.Events = append(v.Events, Event{ID: eid, Type: typ, Metadata: metadata, CreatedAt: now})
	if typ == "run.started" {
		v.State = "running"
	}
	if typ == "run.failed" {
		v.State = "failed"
	}
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Publish(repo, id string, p Publication) (Session, error) {
	if strings.TrimSpace(p.Summary) == "" || p.SourceCommitID == "" || len(p.CommitIDs) == 0 || len(p.Commands) > 100 || len(p.Evidence) > 100 || len(p.ResidualConcerns) > 100 {
		return Session{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if v.State == "revoked" || v.Publication != nil {
		return Session{}, ErrInvalid
	}
	p.PublishedAt = s.now().UTC()
	v.Publication = &p
	v.State = "published"
	v.UpdatedAt = p.PublishedAt
	return v, s.write(v)
}
func (s *Store) Revoke(repo, id string, at time.Time) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	t := at.UTC()
	if v.Publication == nil {
		v.State = "revoked"
	}
	v.CredentialRevokedAt = &t
	v.UpdatedAt = t
	return v, s.write(v)
}
