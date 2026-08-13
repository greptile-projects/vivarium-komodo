// Package roadmapdelivery keeps product intent attached to ordinary delivery evidence.
package roadmapdelivery

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

var ErrNotFound = errors.New("roadmap delivery not found")
var ErrInvalid = errors.New("invalid roadmap delivery")
var ErrConflict = errors.New("roadmap delivery conflict")

type Task struct {
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	EvidenceIDs        []string `json:"evidence_ids"`
	SuccessMeasures    []string `json:"success_measures"`
	DependsOn          []int    `json:"depends_on,omitempty"`
}
type Input struct {
	OutcomeID    string `json:"outcome_id"`
	BaseRevision string `json:"base_revision"`
	Tasks        []Task `json:"tasks"`
}
type Link struct {
	ID                 string            `json:"id"`
	Kind               string            `json:"kind"`
	ResourceID         string            `json:"resource_id"`
	Revision           string            `json:"revision,omitempty"`
	State              string            `json:"state"`
	TaskIDs            []string          `json:"task_ids,omitempty"`
	MeasureResults     map[string]string `json:"measure_results,omitempty"`
	EvidenceIDs        []string          `json:"evidence_ids,omitempty"`
	AssumptionsChanged bool              `json:"assumptions_changed"`
	UnresolvedNeeds    []string          `json:"unresolved_user_needs,omitempty"`
	PolicyConflicts    []string          `json:"policy_conflicts,omitempty"`
	ReportedByID       string            `json:"reported_by_id"`
	ReportedAt         time.Time         `json:"reported_at"`
}
type Revisit struct {
	ID            string    `json:"id"`
	Reason        string    `json:"reason"`
	EvidenceIDs   []string  `json:"evidence_ids"`
	RequestedByID string    `json:"requested_by_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type Delivery struct {
	ID                   string    `json:"id"`
	RepositoryID         string    `json:"repository_id"`
	RoadmapID            string    `json:"roadmap_id"`
	RoadmapVersion       int64     `json:"roadmap_version"`
	OutcomeID            string    `json:"outcome_id"`
	OpportunityID        string    `json:"opportunity_id"`
	OpportunityVersion   int64     `json:"opportunity_version"`
	ProposalID           string    `json:"proposal_id"`
	TaskIDs              []string  `json:"task_ids"`
	SuccessMeasures      []string  `json:"success_measures"`
	EvidenceIDs          []string  `json:"evidence_ids"`
	BaseRevision         string    `json:"base_revision"`
	Links                []Link    `json:"links"`
	Revisits             []Revisit `json:"revisits"`
	State                string    `json:"state"`
	Blockers             []string  `json:"blockers"`
	OperationalAuthority bool      `json:"operational_authority"`
	CreatedByID          string    `json:"created_by_id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
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
	a, _ := filepath.Abs(root)
	if e := os.MkdirAll(a, 0750); e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now}, nil
}
func Validate(in Input, outcomeMeasures []string) bool {
	if in.OutcomeID == "" || in.BaseRevision == "" || len(in.Tasks) == 0 {
		return false
	}
	covered := map[string]bool{}
	for i, t := range in.Tasks {
		if strings.TrimSpace(t.Title) == "" || !map[string]bool{"human": true, "agent": true}[t.OwnerKind] || t.OwnerID == "" || len(t.AcceptanceCriteria) == 0 || len(t.EvidenceIDs) == 0 || len(t.SuccessMeasures) == 0 {
			return false
		}
		for _, d := range t.DependsOn {
			if d < 1 || d > i {
				return false
			}
		}
		for _, m := range t.SuccessMeasures {
			covered[m] = true
		}
	}
	for _, m := range outcomeMeasures {
		if !covered[m] {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, roadmap string, rv int64, outcome, opp string, ov int64, proposal, actor, base string, taskIDs, measures, evidence []string) (Delivery, error) {
	if repo == "" || roadmap == "" || rv < 1 || outcome == "" || proposal == "" || actor == "" || len(taskIDs) == 0 {
		return Delivery{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.now().UTC()
	v := Delivery{ID: id("delivery_"), RepositoryID: repo, RoadmapID: roadmap, RoadmapVersion: rv, OutcomeID: outcome, OpportunityID: opp, OpportunityVersion: ov, ProposalID: proposal, TaskIDs: taskIDs, SuccessMeasures: measures, EvidenceIDs: evidence, BaseRevision: base, Links: []Link{}, Revisits: []Revisit{}, State: "in_progress", Blockers: []string{"measures_not_reported"}, CreatedByID: actor, CreatedAt: n, UpdatedAt: n}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Report(repo, id, actor string, l Link) (Delivery, error) {
	if actor == "" || !map[string]bool{"pull_request": true, "check": true, "preview": true, "integration": true, "release": true, "deployment": true, "experiment": true}[l.Kind] || l.ResourceID == "" || l.State == "" {
		return Delivery{}, ErrInvalid
	}
	return s.mutate(repo, id, func(v *Delivery) error {
		for _, x := range v.Links {
			if x.Kind == l.Kind && x.ResourceID == l.ResourceID && x.Revision == l.Revision {
				return ErrConflict
			}
		}
		l.ID = idfn("link_")
		l.ReportedByID = actor
		l.ReportedAt = s.now().UTC()
		v.Links = append(v.Links, l)
		derive(v)
		return nil
	})
}
func (s *Store) Revisit(repo, id, actor, reason string, evidence []string) (Delivery, error) {
	if actor == "" || strings.TrimSpace(reason) == "" || len(evidence) == 0 {
		return Delivery{}, ErrInvalid
	}
	return s.mutate(repo, id, func(v *Delivery) error {
		v.Revisits = append(v.Revisits, Revisit{ID: idfn("revisit_"), Reason: reason, EvidenceIDs: evidence, RequestedByID: actor, CreatedAt: s.now().UTC()})
		derive(v)
		return nil
	})
}
func derive(v *Delivery) {
	b := []string{}
	results := map[string]string{}
	delivered := false
	for _, l := range v.Links {
		if l.AssumptionsChanged {
			b = append(b, "changed_assumptions")
		}
		if len(l.UnresolvedNeeds) > 0 {
			b = append(b, "unresolved_user_needs")
		}
		if len(l.PolicyConflicts) > 0 {
			b = append(b, "policy_conflict")
		}
		if (l.Kind == "release" || l.Kind == "deployment") && map[string]bool{"succeeded": true, "published": true, "deployed": true}[l.State] {
			delivered = true
		}
		for m, x := range l.MeasureResults {
			results[m] = x
		}
	}
	for _, m := range v.SuccessMeasures {
		switch results[m] {
		case "passed":
		case "failed":
			b = append(b, "failed_measure:"+m)
		default:
			b = append(b, "measure_not_reported:"+m)
		}
	}
	if len(v.Revisits) > 0 {
		b = append(b, "decision_revisit_required")
	}
	sort.Strings(b)
	v.Blockers = dedupe(b)
	v.State = "in_progress"
	if delivered && len(v.Blockers) == 0 {
		v.State = "achieved"
	} else if len(v.Revisits) > 0 {
		v.State = "revisit_required"
	} else if delivered {
		v.State = "delivered_not_achieved"
	}
	v.UpdatedAt = time.Now().UTC()
}
func dedupe(v []string) []string {
	r := []string{}
	for _, x := range v {
		if len(r) == 0 || r[len(r)-1] != x {
			r = append(r, x)
		}
	}
	return r
}
func (s *Store) mutate(repo, id string, fn func(*Delivery) error) (Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return Delivery{}, e
	}
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Delivery, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if e != nil {
		return Delivery{}, ErrNotFound
	}
	var v Delivery
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo || v.ID != id {
		return Delivery{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Delivery) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(d, v.ID+".json"), b, 0600)
}
func id(prefix string) string { return idfn(prefix) }
func idfn(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
