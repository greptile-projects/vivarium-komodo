// Package questions owns durable, revision-pinned agent explanations.
package questions

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

var ErrNotFound = errors.New("conversation not found")

type Context struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	Path string `json:"path,omitempty"`
}

type Citation struct {
	RepositoryID string `json:"repository_id"`
	CommitID     string `json:"commit_id"`
	Kind         string `json:"kind"`
	Path         string `json:"path,omitempty"`
	LineStart    int    `json:"line_start,omitempty"`
	LineEnd      int    `json:"line_end,omitempty"`
	ObjectID     string `json:"object_id,omitempty"`
	Label        string `json:"label,omitempty"`
}

type Claim struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	Mode        string     `json:"mode"` // evidence, inference, or uncertainty
	Citations   []Citation `json:"citations"`
	Uncertainty string     `json:"uncertainty,omitempty"`
}

type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Text      string    `json:"text,omitempty"`
	Claim     *Claim    `json:"claim,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Conversation struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Revision     string    `json:"revision"`
	CommitID     string    `json:"commit_id"`
	ActorID      string    `json:"actor_id"`
	Agent        string    `json:"agent"`
	Question     string    `json:"question"`
	Context      Context   `json:"context"`
	Status       string    `json:"status"`
	Answer       string    `json:"answer"`
	Claims       []Claim   `json:"claims"`
	Events       []Event   `json:"events"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at"`
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

func id() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) Create(c Conversation) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	c.ID, err = id()
	if err != nil {
		return Conversation{}, err
	}
	c.CreatedAt = s.now().UTC()
	c.Status = "completed"
	c.Agent = "codex"
	c.CompletedAt = c.CreatedAt
	for i := range c.Events {
		c.Events[i].Sequence = int64(i + 1)
		c.Events[i].CreatedAt = c.CreatedAt
	}
	return c, s.write(c)
}

func (s *Store) Get(repositoryID, conversationID string) (Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.read(conversationID)
	if err != nil || c.RepositoryID != repositoryID {
		return Conversation{}, ErrNotFound
	}
	return c, nil
}

func (s *Store) List(repositoryID string) ([]Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Conversation{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		c, er := s.read(e.Name()[:len(e.Name())-5])
		if er == nil && c.RepositoryID == repositoryID {
			c.Events = nil
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) read(id string) (Conversation, error) {
	b, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(err) {
		return Conversation{}, ErrNotFound
	}
	var c Conversation
	if err == nil {
		err = json.Unmarshal(b, &c)
	}
	return c, err
}
func (s *Store) write(c Conversation) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.root, "."+c.ID+".tmp")
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.root, c.ID+".json"))
}
