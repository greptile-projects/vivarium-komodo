// Package debuggingworkspaces retains the attributable, revision-exact starting
// context for collaborative production debugging. It deliberately stores only
// references to permitted evidence, never runtime evidence or credentials.
package debuggingworkspaces

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

var ErrNotFound = errors.New("debugging workspace not found")
var ErrInvalid = errors.New("invalid debugging workspace")

type Origin struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Summary    string `json:"summary"`
}
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type Binding struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Path       string `json:"path,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}
type EvidencePermission struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Audience   string `json:"audience"`
	Access     string `json:"access"`
	Retention  string `json:"retention,omitempty"`
	Reason     string `json:"reason,omitempty"`
}
type Hypothesis struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Status      string    `json:"status"`
	EvidenceIDs []string  `json:"evidence_ids,omitempty"`
	Uncertainty string    `json:"uncertainty,omitempty"`
	ActorID     string    `json:"actor_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Event struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Detail    string    `json:"detail"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Workspace struct {
	ID                 string               `json:"id"`
	RepositoryID       string               `json:"repository_id"`
	Title              string               `json:"title"`
	Origin             Origin               `json:"origin"`
	ReleaseID          string               `json:"release_id"`
	ReleaseRevision    string               `json:"release_revision"`
	Environment        string               `json:"environment"`
	Window             TimeWindow           `json:"time_window"`
	UserJourney        string               `json:"user_journey"`
	OwnerIDs           []string             `json:"owner_ids"`
	Severity           string               `json:"severity"`
	SourceRevision     string               `json:"source_revision"`
	Bindings           []Binding            `json:"bindings"`
	PermittedEvidence  []EvidencePermission `json:"permitted_evidence"`
	Audience           string               `json:"audience"`
	ParticipantIDs     []string             `json:"participant_ids"`
	Status             string               `json:"status"`
	Hypotheses         []Hypothesis         `json:"hypotheses"`
	History            []Event              `json:"history"`
	UnavailableContext []Binding            `json:"unavailable_context"`
	Authority          []string             `json:"authority"`
	CreatorID          string               `json:"creator_id"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}
type CreateInput struct {
	Title             string               `json:"title"`
	Origin            Origin               `json:"origin"`
	ReleaseID         string               `json:"release_id"`
	ReleaseRevision   string               `json:"release_revision"`
	Environment       string               `json:"environment"`
	Window            TimeWindow           `json:"time_window"`
	UserJourney       string               `json:"user_journey"`
	OwnerIDs          []string             `json:"owner_ids"`
	Severity          string               `json:"severity"`
	SourceRevision    string               `json:"source_revision"`
	Bindings          []Binding            `json:"bindings"`
	PermittedEvidence []EvidencePermission `json:"permitted_evidence"`
	Audience          string               `json:"audience"`
	ParticipantIDs    []string             `json:"participant_ids"`
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
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func unique(in []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range in {
		x = strings.TrimSpace(x)
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func validBinding(b Binding) bool {
	return map[string]bool{"package": true, "configuration": true, "infrastructure": true}[b.Kind] && ((b.Status == "available" && b.ResourceID != "" && b.Revision != "") || (b.Status == "unavailable" && b.Reason != "" && b.Revision == ""))
}
func (s *Store) Create(repo, actor string, in CreateInput) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || strings.TrimSpace(in.Title) == "" || !map[string]bool{"issue": true, "incident": true, "support_thread": true, "deployment": true, "service_objective": true, "trace": true, "manual_observation": true}[in.Origin.Kind] || in.Origin.Summary == "" || (in.Origin.Kind != "manual_observation" && in.Origin.ResourceID == "") || in.ReleaseID == "" || in.ReleaseRevision == "" || in.Environment == "" || in.Window.Start.IsZero() || !in.Window.End.After(in.Window.Start) || in.UserJourney == "" || len(in.OwnerIDs) == 0 || !map[string]bool{"critical": true, "high": true, "medium": true, "low": true}[in.Severity] || in.SourceRevision == "" || !map[string]bool{"repository": true, "participants": true}[in.Audience] || len(in.PermittedEvidence) == 0 {
		return Workspace{}, ErrInvalid
	}
	kinds := map[string]bool{}
	unavailable := []Binding{}
	for _, b := range in.Bindings {
		if !validBinding(b) {
			return Workspace{}, ErrInvalid
		}
		kinds[b.Kind] = true
		if b.Status == "unavailable" {
			unavailable = append(unavailable, b)
		}
	}
	for _, k := range []string{"package", "configuration", "infrastructure"} {
		if !kinds[k] {
			return Workspace{}, ErrInvalid
		}
	}
	for _, e := range in.PermittedEvidence {
		if e.Kind == "" || !map[string]bool{"repository": true, "participants": true}[e.Audience] || !map[string]bool{"permitted": true, "unavailable": true, "denied": true}[e.Access] || (e.Access != "permitted" && e.Reason == "") {
			return Workspace{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	participants := unique(append(in.ParticipantIDs, actor))
	owners := unique(in.OwnerIDs)
	v := Workspace{ID: newID(), RepositoryID: repo, Title: strings.TrimSpace(in.Title), Origin: in.Origin, ReleaseID: in.ReleaseID, ReleaseRevision: in.ReleaseRevision, Environment: in.Environment, Window: in.Window, UserJourney: in.UserJourney, OwnerIDs: owners, Severity: in.Severity, SourceRevision: in.SourceRevision, Bindings: in.Bindings, PermittedEvidence: in.PermittedEvidence, Audience: in.Audience, ParticipantIDs: participants, Status: "open", UnavailableContext: unavailable, Authority: []string{}, CreatorID: actor, CreatedAt: now, UpdatedAt: now}
	v.History = []Event{{ID: newID(), Kind: "opened", Detail: "Debugging context established; no runtime, deployment, credential, or mutation authority granted.", ActorID: actor, CreatedAt: now}}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Workspace{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Workspace{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if e == nil && v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) AddHypothesis(repo, id, actor string, in Hypothesis) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Workspace{}, ErrNotFound
	}
	if !contains(v.ParticipantIDs, actor) || strings.TrimSpace(in.Summary) == "" || !map[string]bool{"proposed": true, "supported": true, "disputed": true, "rejected": true}[in.Status] {
		return Workspace{}, ErrInvalid
	}
	in.ID = newID()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	v.Hypotheses = append(v.Hypotheses, in)
	v.UpdatedAt = in.CreatedAt
	v.History = append(v.History, Event{ID: newID(), Kind: "hypothesis_added", Detail: in.ID, ActorID: actor, CreatedAt: in.CreatedAt})
	return v, s.write(v)
}
func (s *Store) Control(repo, id, actor, status string, participants []string) (Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Workspace{}, ErrNotFound
	}
	if !contains(v.ParticipantIDs, actor) || !map[string]bool{"open": true, "paused": true, "resolved": true, "closed": true}[status] {
		return Workspace{}, ErrInvalid
	}
	for _, p := range participants {
		if p == "" {
			return Workspace{}, ErrInvalid
		}
	}
	if participants != nil {
		v.ParticipantIDs = unique(append(participants, v.CreatorID))
	}
	v.Status = status
	v.UpdatedAt = s.now().UTC()
	v.History = append(v.History, Event{ID: newID(), Kind: "status_changed", Detail: status, ActorID: actor, CreatedAt: v.UpdatedAt})
	return v, s.write(v)
}
func (s *Store) read(id string) (Workspace, error) {
	var v Workspace
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Workspace) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
