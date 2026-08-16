// Package recoveryimprovements retains accountable work and verification for recovery gaps.
package recoveryimprovements

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

var ErrNotFound = errors.New("recovery improvement not found")
var ErrInvalid = errors.New("invalid recovery improvement")

type TaskInput struct {
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	ContextKind        string   `json:"context_kind,omitempty"`
	ContextID          string   `json:"context_id,omitempty"`
	DependsOn          []int    `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type Link struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision"`
	TaskID     string    `json:"task_id,omitempty"`
	Summary    string    `json:"summary"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Improvement struct {
	ID                     string    `json:"id"`
	RepositoryID           string    `json:"repository_id"`
	InvestigationID        string    `json:"investigation_id"`
	FindingID              string    `json:"finding_id"`
	ExerciseID             string    `json:"exercise_id"`
	PlanID                 string    `json:"plan_id"`
	PlanVersion            int64     `json:"plan_version"`
	Title                  string    `json:"title"`
	BaseRevision           string    `json:"base_revision"`
	ProposalID             string    `json:"proposal_id"`
	TaskIDs                []string  `json:"task_ids"`
	Links                  []Link    `json:"links"`
	VerificationExerciseID string    `json:"verification_exercise_id,omitempty"`
	State                  string    `json:"state"`
	Blockers               []string  `json:"blockers"`
	CreatorID              string    `json:"creator_id"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
type CreateInput struct {
	InvestigationID string      `json:"investigation_id"`
	FindingID       string      `json:"finding_id"`
	Title           string      `json:"title"`
	BaseRevision    string      `json:"base_revision"`
	Tasks           []TaskInput `json:"tasks"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func Valid(in CreateInput) bool {
	if in.InvestigationID == "" || in.FindingID == "" || strings.TrimSpace(in.Title) == "" || in.BaseRevision == "" || len(in.Tasks) == 0 {
		return false
	}
	for i, t := range in.Tasks {
		if t.Title == "" || !map[string]bool{"human": true, "agent": true}[t.OwnerKind] || t.OwnerID == "" || len(t.AcceptanceCriteria) == 0 {
			return false
		}
		if (t.ContextKind == "") != (t.ContextID == "") || !map[string]bool{"": true, "session": true, "workspace": true}[t.ContextKind] {
			return false
		}
		for _, d := range t.DependsOn {
			if d < 1 || d > i {
				return false
			}
		}
	}
	return true
}
func (s *Store) Create(repo, actor, proposal, exercise, plan string, planVersion int64, taskIDs []string, in CreateInput) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || proposal == "" || exercise == "" || plan == "" || planVersion < 1 || len(taskIDs) != len(in.Tasks) || !Valid(in) {
		return Improvement{}, ErrInvalid
	}
	now := s.now().UTC()
	v := Improvement{ID: id(), RepositoryID: repo, InvestigationID: in.InvestigationID, FindingID: in.FindingID, ExerciseID: exercise, PlanID: plan, PlanVersion: planVersion, Title: strings.TrimSpace(in.Title), BaseRevision: in.BaseRevision, ProposalID: proposal, TaskIDs: taskIDs, State: "planned", Blockers: []string{"fresh_exercise_required"}, CreatorID: actor, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) Get(repo, x string) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Improvement{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if er == nil && v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Link(repo, x, actor string, in Link) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	if actor == "" || !map[string]bool{"session": true, "workspace": true, "pull_request": true, "check": true, "integration": true, "release": true, "policy_change": true, "approval": true}[in.Kind] || in.ResourceID == "" || in.Revision == "" || strings.TrimSpace(in.Summary) == "" {
		return Improvement{}, ErrInvalid
	}
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	v.Links = append(v.Links, in)
	v.State = "delivering"
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}
func (s *Store) Verify(repo, x, actor, exercise string, passed bool) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	if actor == "" || exercise == "" || exercise == v.ExerciseID {
		return Improvement{}, ErrInvalid
	}
	v.VerificationExerciseID = exercise
	v.Blockers = nil
	if passed {
		v.State = "verified"
	} else {
		v.State = "verification_failed"
		v.Blockers = []string{"recovery_gap_remains"}
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (s *Store) read(x string) (Improvement, error) {
	b, e := os.ReadFile(filepath.Join(s.root, x+".json"))
	if e != nil {
		return Improvement{}, e
	}
	var v Improvement
	if json.Unmarshal(b, &v) != nil || v.ID != x {
		return Improvement{}, ErrInvalid
	}
	return v, nil
}
func (s *Store) write(v Improvement) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(s.root, ".improvement-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0640); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(s.root, v.ID+".json"))
	}
	return e
}
