// Package decisions owns durable, attributable technical-decision scope and discussion.
package decisions

import (
	"crypto/rand"
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
	ErrNotFound = errors.New("decision not found")
	ErrInvalid  = errors.New("invalid decision")
)

type Context struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type Resource struct {
	Kind         string `json:"kind"`
	RepositoryID string `json:"repository_id,omitempty"`
	ID           string `json:"id,omitempty"`
	Ref          string `json:"ref,omitempty"`
	Path         string `json:"path,omitempty"`
	Label        string `json:"label"`
}
type Scope struct {
	Version           int        `json:"version"`
	Question          string     `json:"question"`
	Constraints       []string   `json:"constraints"`
	SuccessMeasures   []string   `json:"success_measures"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	AffectedResources []Resource `json:"affected_resources"`
	ParticipantIDs    []string   `json:"participant_ids"`
	OwnerID           string     `json:"owner_id"`
	ChangedByID       string     `json:"changed_by_id"`
	ChangeSummary     string     `json:"change_summary"`
	CreatedAt         time.Time  `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Decision struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	Context      Context   `json:"context"`
	CreatedByID  string    `json:"created_by_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Scope        Scope     `json:"scope"`
	History      []Scope   `json:"history"`
	Comments     []Comment `json:"comments"`
}
type ScopeInput struct {
	Question          string     `json:"question"`
	Constraints       []string   `json:"constraints"`
	SuccessMeasures   []string   `json:"success_measures"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	AffectedResources []Resource `json:"affected_resources"`
	ParticipantIDs    []string   `json:"participant_ids"`
	OwnerID           string     `json:"owner_id"`
	ChangeSummary     string     `json:"change_summary"`
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
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func cleanList(in []string) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || len(v) > 500 {
			return nil, false
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, true
}
func validContext(k string) bool {
	switch k {
	case "repository", "proposal", "investigation", "incident", "evolution_plan", "stewardship_opportunity":
		return true
	}
	return false
}
func scope(in ScopeInput, actor string, version int, now time.Time) (Scope, error) {
	q := strings.TrimSpace(in.Question)
	c, ok := cleanList(in.Constraints)
	if !ok {
		return Scope{}, ErrInvalid
	}
	m, ok := cleanList(in.SuccessMeasures)
	if !ok {
		return Scope{}, ErrInvalid
	}
	p, ok := cleanList(in.ParticipantIDs)
	if !ok || q == "" || len(q) > 4000 || len(c) == 0 || len(m) == 0 || in.OwnerID == "" || len(p) == 0 {
		return Scope{}, ErrInvalid
	}
	if !contains(p, in.OwnerID) || !contains(p, actor) {
		return Scope{}, ErrInvalid
	}
	for i := range in.AffectedResources {
		in.AffectedResources[i].Kind = strings.TrimSpace(in.AffectedResources[i].Kind)
		in.AffectedResources[i].Label = strings.TrimSpace(in.AffectedResources[i].Label)
		if in.AffectedResources[i].Kind == "" || in.AffectedResources[i].Label == "" || len(in.AffectedResources[i].Label) > 300 {
			return Scope{}, ErrInvalid
		}
	}
	if in.Deadline != nil {
		d := in.Deadline.UTC()
		in.Deadline = &d
	}
	return Scope{Version: version, Question: q, Constraints: c, SuccessMeasures: m, Deadline: in.Deadline, AffectedResources: in.AffectedResources, ParticipantIDs: p, OwnerID: in.OwnerID, ChangedByID: actor, ChangeSummary: strings.TrimSpace(in.ChangeSummary), CreatedAt: now}, nil
}
func contains(v []string, x string) bool {
	for _, a := range v {
		if a == x {
			return true
		}
	}
	return false
}
func (s *Store) Create(repo, actor, title string, context Context, in ScopeInput) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	title = strings.TrimSpace(title)
	context.Kind = strings.TrimSpace(context.Kind)
	context.ID = strings.TrimSpace(context.ID)
	if repo == "" || title == "" || len(title) > 200 || !validContext(context.Kind) || (context.Kind != "repository" && context.ID == "") {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	sc, e := scope(in, actor, 1, now)
	if e != nil {
		return Decision{}, e
	}
	v := Decision{ID: newID(), RepositoryID: repo, Title: title, State: "pending", Context: context, CreatedByID: actor, CreatedAt: now, UpdatedAt: now, Scope: sc, History: []Scope{sc}, Comments: []Comment{}}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	return v, nil
}
func (s *Store) List(repo, kind, id string) ([]Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Decision{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Decision{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, z := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if z == nil && (kind == "" || (v.Context.Kind == kind && v.Context.ID == id)) {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) Revise(repo, id, actor, title string, in ScopeInput) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 200 || strings.TrimSpace(in.ChangeSummary) == "" {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	sc, e := scope(in, actor, len(v.History)+1, now)
	if e != nil {
		return Decision{}, e
	}
	v.Title = title
	v.Scope = sc
	v.History = append(v.History, sc)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Comment(repo, id, actor, body string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 65536 {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	v.Comments = append(v.Comments, Comment{ID: newID(), AuthorID: actor, Body: body, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) read(repo, id string) (Decision, error) {
	var v Decision
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo {
		return Decision{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Decision) error {
	if err := os.MkdirAll(filepath.Join(s.root, v.RepositoryID), 0750); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.RepositoryID, v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, s.path(v.RepositoryID, v.ID))
}
