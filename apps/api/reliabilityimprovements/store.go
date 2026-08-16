// Package reliabilityimprovements retains the accountable path from measured harm to verified recovery.
package reliabilityimprovements

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

var ErrNotFound = errors.New("reliability improvement not found")
var ErrInvalid = errors.New("invalid reliability improvement")

type Source struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	EntryID    string `json:"entry_id,omitempty"`
}
type Baseline struct {
	Indicator  string  `json:"indicator"`
	Window     string  `json:"window"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	EvidenceID string  `json:"evidence_id"`
}
type TaskInput struct {
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	Risk               string   `json:"risk,omitempty"`
	DependsOn          []int    `json:"depends_on,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	EvidenceIDs        []string `json:"evidence_ids,omitempty"`
	DependencyContext  []string `json:"dependency_context,omitempty"`
}
type DeliveryLink struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	Revision   string    `json:"revision"`
	TaskID     string    `json:"task_id,omitempty"`
	Summary    string    `json:"summary"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Measurement struct {
	Indicator  string  `json:"indicator"`
	Window     string  `json:"window"`
	Unit       string  `json:"unit"`
	EvidenceID string  `json:"evidence_id"`
	Value      float64 `json:"value"`
	Passed     bool    `json:"passed"`
}
type Rollout struct {
	ID             string        `json:"id"`
	DeploymentID   string        `json:"deployment_id"`
	ReleaseID      string        `json:"release_id"`
	Revision       string        `json:"revision"`
	Environment    string        `json:"environment"`
	Stage          string        `json:"stage"`
	State          string        `json:"state"`
	RequiredAction string        `json:"required_action"`
	Rationale      string        `json:"rationale"`
	ActorID        string        `json:"actor_id"`
	Measurements   []Measurement `json:"measurements"`
	CreatedAt      time.Time     `json:"created_at"`
}
type Improvement struct {
	ID                  string         `json:"id"`
	RepositoryID        string         `json:"repository_id"`
	ObjectiveID         string         `json:"objective_id"`
	ProposalID          string         `json:"proposal_id"`
	BaseRevision        string         `json:"base_revision"`
	Title               string         `json:"title"`
	CreatorID           string         `json:"creator_id"`
	ObjectiveVersion    int64          `json:"objective_version"`
	Source              Source         `json:"source"`
	AffectedRevisions   []string       `json:"affected_revisions"`
	JourneyIDs          []string       `json:"journey_ids"`
	EvidenceIDs         []string       `json:"evidence_ids"`
	DependencyContext   []string       `json:"dependency_context"`
	AcceptanceCriteria  []string       `json:"acceptance_criteria"`
	TaskIDs             []string       `json:"task_ids"`
	Baseline            Baseline       `json:"baseline"`
	DeliveryLinks       []DeliveryLink `json:"delivery_links"`
	Rollouts            []Rollout      `json:"rollouts"`
	State               string         `json:"state"`
	BudgetState         string         `json:"budget_state"`
	PriorImpactRetained bool           `json:"prior_impact_retained"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}
type CreateInput struct {
	ObjectiveID        string      `json:"objective_id"`
	ObjectiveVersion   int64       `json:"objective_version"`
	Source             Source      `json:"source"`
	BaseRevision       string      `json:"base_revision"`
	Title              string      `json:"title"`
	AffectedRevisions  []string    `json:"affected_revisions"`
	JourneyIDs         []string    `json:"journey_ids"`
	EvidenceIDs        []string    `json:"evidence_ids"`
	DependencyContext  []string    `json:"dependency_context"`
	AcceptanceCriteria []string    `json:"acceptance_criteria"`
	Baseline           Baseline    `json:"baseline"`
	Tasks              []TaskInput `json:"tasks"`
}
type RolloutInput struct {
	DeploymentID string        `json:"deployment_id"`
	ReleaseID    string        `json:"release_id"`
	Revision     string        `json:"revision"`
	Environment  string        `json:"environment"`
	Stage        string        `json:"stage"`
	Rationale    string        `json:"rationale"`
	Measurements []Measurement `json:"measurements"`
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
	if e := os.MkdirAll(root, 0700); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func NewID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func Valid(in CreateInput) bool {
	if in.ObjectiveID == "" || in.ObjectiveVersion < 1 || in.Source.ResourceID == "" || !map[string]bool{"finding": true, "depleted_budget": true}[in.Source.Kind] || in.BaseRevision == "" || strings.TrimSpace(in.Title) == "" || len(in.AffectedRevisions) == 0 || len(in.JourneyIDs) == 0 || len(in.EvidenceIDs) == 0 || len(in.AcceptanceCriteria) == 0 || in.Baseline.Indicator == "" || in.Baseline.Window == "" || in.Baseline.Unit == "" || in.Baseline.EvidenceID == "" || len(in.Tasks) == 0 {
		return false
	}
	for i, t := range in.Tasks {
		if t.Title == "" || !map[string]bool{"human": true, "agent": true}[t.OwnerKind] || t.OwnerID == "" || len(t.AcceptanceCriteria) == 0 {
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
func (s *Store) Create(repo, actor, proposal string, taskIDs []string, in CreateInput) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || proposal == "" || len(taskIDs) != len(in.Tasks) || !Valid(in) {
		return Improvement{}, ErrInvalid
	}
	now := s.now().UTC()
	v := Improvement{ID: NewID(), RepositoryID: repo, ObjectiveID: in.ObjectiveID, ObjectiveVersion: in.ObjectiveVersion, Source: in.Source, ProposalID: proposal, TaskIDs: taskIDs, BaseRevision: in.BaseRevision, Title: strings.TrimSpace(in.Title), AffectedRevisions: in.AffectedRevisions, JourneyIDs: in.JourneyIDs, EvidenceIDs: in.EvidenceIDs, DependencyContext: in.DependencyContext, AcceptanceCriteria: in.AcceptanceCriteria, Baseline: in.Baseline, State: "planned", BudgetState: "depleted", PriorImpactRetained: true, CreatorID: actor, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) List(repo string) ([]Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Improvement{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		v, readErr := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if readErr != nil {
			return nil, readErr
		}
		if v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Link(repo, id, actor string, in DeliveryLink) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	if !map[string]bool{"pull_request": true, "check": true, "release": true, "deployment": true, "decision": true}[in.Kind] || in.ResourceID == "" || in.Revision == "" || actor == "" {
		return Improvement{}, ErrInvalid
	}
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	v.DeliveryLinks = append(v.DeliveryLinks, in)
	v.State = "delivering"
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}
func (s *Store) Rollout(repo, id, actor string, in RolloutInput) (Improvement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Improvement{}, ErrNotFound
	}
	if actor == "" || in.DeploymentID == "" || in.ReleaseID == "" || in.Revision == "" || in.Environment == "" || in.Stage == "" || len(in.Measurements) == 0 {
		return Improvement{}, ErrInvalid
	}
	all := true
	for _, m := range in.Measurements {
		if m.Indicator == "" || m.Window == "" || m.Unit != v.Baseline.Unit || m.EvidenceID == "" {
			return Improvement{}, ErrInvalid
		}
		all = all && m.Passed
	}
	state, action := "failed", "contain"
	if all {
		state, action = "succeeded", "continue"
	} else if strings.Contains(strings.ToLower(in.Rationale), "rollback") {
		action = "rollback"
	} else if strings.Contains(strings.ToLower(in.Rationale), "revisit") {
		action = "revisit_decision"
	}
	now := s.now().UTC()
	v.Rollouts = append(v.Rollouts, Rollout{ID: NewID(), DeploymentID: in.DeploymentID, ReleaseID: in.ReleaseID, Revision: in.Revision, Environment: in.Environment, Stage: in.Stage, State: state, RequiredAction: action, Rationale: in.Rationale, Measurements: in.Measurements, ActorID: actor, CreatedAt: now})
	if all {
		v.State = "verified"
		v.BudgetState = "restored"
	} else {
		v.State = "contained"
		v.BudgetState = "depleted"
	}
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) read(id string) (Improvement, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if e != nil {
		return Improvement{}, e
	}
	var v Improvement
	if json.Unmarshal(b, &v) != nil || v.ID != id {
		return Improvement{}, ErrInvalid
	}
	return v, nil
}
func (s *Store) write(v Improvement) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(s.root, ".improvement-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
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
func Unique(xs []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
