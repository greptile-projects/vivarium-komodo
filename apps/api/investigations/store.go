// Package investigations owns shared, revision-exact code inquiry canvases.
package investigations

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

var ErrNotFound = errors.New("investigation not found")
var ErrConflict = errors.New("investigation conflict")

type Citation struct {
	RepositoryID      string `json:"repository_id"`
	CommitID          string `json:"commit_id,omitempty"`
	Kind              string `json:"kind"`
	Path              string `json:"path,omitempty"`
	LineStart         int    `json:"line_start,omitempty"`
	LineEnd           int    `json:"line_end,omitempty"`
	ObjectID          string `json:"object_id,omitempty"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	WorkspaceSequence int64  `json:"workspace_sequence,omitempty"`
	Label             string `json:"label,omitempty"`
}

type Entry struct {
	ID         string     `json:"id"`
	Sequence   int64      `json:"sequence"`
	Type       string     `json:"type"`
	Body       string     `json:"body"`
	ActorID    string     `json:"actor_id"`
	Agent      string     `json:"agent,omitempty"`
	CommitID   string     `json:"commit_id"`
	Citations  []Citation `json:"citations,omitempty"`
	Challenges string     `json:"challenges,omitempty"`
	Supersedes string     `json:"supersedes,omitempty"`
	Stale      bool       `json:"stale"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Run struct {
	ID        string    `json:"id"`
	Revision  string    `json:"revision"`
	CommitID  string    `json:"commit_id"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Investigation struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Title        string    `json:"title"`
	Question     string    `json:"question"`
	CreatorID    string    `json:"creator_id"`
	Participants []string  `json:"participants"`
	Revision     string    `json:"revision"`
	CommitID     string    `json:"commit_id"`
	Runs         []Run     `json:"runs"`
	Entries      []Entry   `json:"entries"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) Create(repositoryID, title, question, revision, commitID, actor string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Investigation{}, err
	}
	runID, err := newID()
	if err != nil {
		return Investigation{}, err
	}
	now := s.now().UTC()
	v := Investigation{ID: id, RepositoryID: repositoryID, Title: strings.TrimSpace(title), Question: strings.TrimSpace(question), CreatorID: actor, Participants: []string{actor}, Revision: revision, CommitID: commitID, CreatedAt: now, UpdatedAt: now, Runs: []Run{{ID: runID, Revision: revision, CommitID: commitID, ActorID: actor, CreatedAt: now}}}
	return v, s.write(v)
}

func (s *Store) Get(repositoryID, id string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(id)
	if err != nil || v.RepositoryID != repositoryID {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repositoryID string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Investigation{}
	for _, e := range es {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		v, x := s.read(strings.TrimSuffix(e.Name(), ".json"))
		if x == nil && v.RepositoryID == repositoryID {
			v.Entries = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *Store) Invite(repositoryID, id, actor, participant string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(id)
	if err != nil || v.RepositoryID != repositoryID {
		return Investigation{}, ErrNotFound
	}
	if !contains(v.Participants, actor) {
		return Investigation{}, ErrNotFound
	}
	if !contains(v.Participants, participant) {
		v.Participants = append(v.Participants, participant)
		sort.Strings(v.Participants)
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}

func (s *Store) Add(repositoryID, id, actor string, e Entry) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(id)
	if err != nil || v.RepositoryID != repositoryID || !contains(v.Participants, actor) {
		return Investigation{}, ErrNotFound
	}
	e.Body = strings.TrimSpace(e.Body)
	if e.Body == "" || len(e.Body) > 10000 || !validType(e.Type) {
		return Investigation{}, ErrConflict
	}
	if e.Challenges != "" && !hasEntry(v.Entries, e.Challenges) || e.Supersedes != "" && !hasEntry(v.Entries, e.Supersedes) {
		return Investigation{}, ErrConflict
	}
	e.ID, err = newID()
	if err != nil {
		return Investigation{}, err
	}
	e.Sequence = int64(len(v.Entries) + 1)
	e.ActorID = actor
	e.CommitID = v.CommitID
	e.CreatedAt = s.now().UTC()
	v.Entries = append(v.Entries, e)
	v.UpdatedAt = e.CreatedAt
	return v, s.write(v)
}

func (s *Store) Rerun(repositoryID, id, actor, revision, commitID, reason string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(id)
	if err != nil || v.RepositoryID != repositoryID || !contains(v.Participants, actor) {
		return Investigation{}, ErrNotFound
	}
	runID, err := newID()
	if err != nil {
		return Investigation{}, err
	}
	now := s.now().UTC()
	for i := range v.Entries {
		if v.Entries[i].CommitID != "" && v.Entries[i].CommitID != commitID {
			v.Entries[i].Stale = true
		}
	}
	v.Revision = revision
	v.CommitID = commitID
	v.Runs = append(v.Runs, Run{ID: runID, Revision: revision, CommitID: commitID, ActorID: actor, Reason: strings.TrimSpace(reason), CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}

func validType(v string) bool {
	switch v {
	case "code_reference", "query", "runtime_observation", "hypothesis", "agent_finding", "conclusion", "challenge":
		return true
	}
	return false
}
func contains(v []string, x string) bool {
	for _, item := range v {
		if item == x {
			return true
		}
	}
	return false
}
func hasEntry(v []Entry, x string) bool {
	for _, item := range v {
		if item.ID == x {
			return true
		}
	}
	return false
}
func (s *Store) read(id string) (Investigation, error) {
	b, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(err) {
		return Investigation{}, ErrNotFound
	}
	var v Investigation
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, err
}
func (s *Store) write(v Investigation) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
