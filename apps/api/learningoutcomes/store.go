// Package learningoutcomes keeps consent-bounded evidence and reviewed curriculum improvements.
package learningoutcomes

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("learning outcomes not found")
var ErrInvalid = errors.New("invalid learning outcome")

type Observation struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	ModuleID           string     `json:"module_id,omitempty"`
	PathwayVersion     int64      `json:"pathway_version"`
	ProjectRevision    string     `json:"project_revision"`
	Audience           string     `json:"audience"`
	Consent            string     `json:"consent"`
	Count              int        `json:"count"`
	Summary            string     `json:"summary"`
	EvidenceReferences []string   `json:"evidence_references"`
	RetainUntil        *time.Time `json:"retain_until,omitempty"`
	AuthorID           string     `json:"author_id"`
	CreatedAt          time.Time  `json:"created_at"`
}
type Finding struct {
	ID             string    `json:"id"`
	Kind           string    `json:"kind"`
	ModuleID       string    `json:"module_id,omitempty"`
	Summary        string    `json:"summary"`
	ObservationIDs []string  `json:"observation_ids"`
	Confidence     string    `json:"confidence"`
	AuthorKind     string    `json:"author_kind"`
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type Impact struct {
	LearnerID            string `json:"learner_id"`
	CompletionEvidenceID string `json:"completion_evidence_id"`
	PriorPathwayVersion  int64  `json:"prior_pathway_version"`
	Status               string `json:"status"`
	Reason               string `json:"reason"`
}
type Improvement struct {
	ID                   string    `json:"id"`
	FindingIDs           []string  `json:"finding_ids"`
	Kind                 string    `json:"kind"`
	Summary              string    `json:"summary"`
	BasePathwayVersion   int64     `json:"base_pathway_version"`
	TargetPathwayVersion int64     `json:"target_pathway_version"`
	ProjectRevision      string    `json:"project_revision"`
	DeliveryKind         string    `json:"delivery_kind"`
	DeliveryID           string    `json:"delivery_id"`
	DeliveryRevision     string    `json:"delivery_revision"`
	ReviewStatus         string    `json:"review_status"`
	ReviewerID           string    `json:"reviewer_id,omitempty"`
	Material             bool      `json:"material"`
	RequirementChanges   []string  `json:"requirement_changes,omitempty"`
	AffectedLearners     []Impact  `json:"affected_learners,omitempty"`
	AuthorID             string    `json:"author_id"`
	CreatedAt            time.Time `json:"created_at"`
}
type Revalidation struct {
	ID                   string    `json:"id"`
	ImprovementID        string    `json:"improvement_id"`
	LearnerID            string    `json:"learner_id"`
	CompletionEvidenceID string    `json:"completion_evidence_id"`
	FromPathwayVersion   int64     `json:"from_pathway_version"`
	ToPathwayVersion     int64     `json:"to_pathway_version"`
	EvidenceReferences   []string  `json:"evidence_references"`
	ActorID              string    `json:"actor_id"`
	CreatedAt            time.Time `json:"created_at"`
}
type Record struct {
	RepositoryID    string         `json:"repository_id"`
	PathwayID       string         `json:"pathway_id"`
	Observations    []Observation  `json:"observations"`
	Findings        []Finding      `json:"findings"`
	Improvements    []Improvement  `json:"improvements"`
	Revalidations   []Revalidation `json:"revalidations"`
	GrantsAuthority bool           `json:"grants_authority"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func text(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 4000 }
func one(s string, xs ...string) bool {
	for _, x := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func id(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func (s *Store) mutate(repo, pathway string, fn func(*Record) error) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, e := s.read(repo, pathway)
	if errors.Is(e, ErrNotFound) {
		r = Record{RepositoryID: repo, PathwayID: pathway, Observations: []Observation{}, Findings: []Finding{}, Improvements: []Improvement{}, Revalidations: []Revalidation{}, GrantsAuthority: false}
	} else if e != nil {
		return r, e
	}
	if e = fn(&r); e != nil {
		return r, e
	}
	return r, s.write(r)
}
func (s *Store) Observe(repo, pathway, actor string, v Observation) (Record, error) {
	return s.mutate(repo, pathway, func(r *Record) error {
		if !id(v.ID) || !one(v.Kind, "module_completion", "recurring_question", "setup_failure", "assessment_gap", "mentor_load", "contribution_outcome", "reviewer_correction", "retention") || v.PathwayVersion < 1 || !text(v.ProjectRevision) || !one(v.Audience, "maintainers", "repository", "public") || v.Consent != "granted" || v.Count < 1 || !text(v.Summary) || len(v.EvidenceReferences) == 0 {
			return ErrInvalid
		}
		for _, x := range r.Observations {
			if x.ID == v.ID {
				return ErrInvalid
			}
		}
		v.AuthorID = actor
		v.CreatedAt = s.now().UTC()
		r.Observations = append(r.Observations, v)
		return nil
	})
}
func (s *Store) Find(repo, pathway, actor, actorKind string, v Finding) (Record, error) {
	return s.mutate(repo, pathway, func(r *Record) error {
		if !id(v.ID) || !one(v.Kind, "curriculum_gap", "setup_gap", "assessment_gap", "support_load", "contribution_gap", "retention_gap") || !text(v.Summary) || len(v.ObservationIDs) == 0 || !one(v.Confidence, "supported", "uncertain") || !one(actorKind, "human", "agent") {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, o := range r.Observations {
			seen[o.ID] = true
		}
		for _, x := range v.ObservationIDs {
			if !seen[x] {
				return ErrInvalid
			}
		}
		v.AuthorID = actor
		v.AuthorKind = actorKind
		v.CreatedAt = s.now().UTC()
		r.Findings = append(r.Findings, v)
		return nil
	})
}
func (s *Store) Improve(repo, pathway, actor string, v Improvement) (Record, error) {
	return s.mutate(repo, pathway, func(r *Record) error {
		if !id(v.ID) || len(v.FindingIDs) == 0 || !one(v.Kind, "documentation", "exercise", "workspace", "pathway", "code", "policy") || !text(v.Summary) || v.BasePathwayVersion < 1 || v.TargetPathwayVersion < v.BasePathwayVersion || !text(v.ProjectRevision) || !one(v.DeliveryKind, "pull_request", "proposal") || !text(v.DeliveryID) || !text(v.DeliveryRevision) || !one(v.ReviewStatus, "pending", "approved", "rejected") || v.Material && (v.TargetPathwayVersion <= v.BasePathwayVersion || len(v.RequirementChanges) == 0) {
			return ErrInvalid
		}
		found := map[string]bool{}
		for _, f := range r.Findings {
			found[f.ID] = true
		}
		for _, x := range v.FindingIDs {
			if !found[x] {
				return ErrInvalid
			}
		}
		if v.ReviewStatus == "approved" && v.ReviewerID == "" {
			return ErrInvalid
		}
		for i := range v.AffectedLearners {
			if !text(v.AffectedLearners[i].LearnerID) || !text(v.AffectedLearners[i].CompletionEvidenceID) || v.AffectedLearners[i].PriorPathwayVersion < 1 || !text(v.AffectedLearners[i].Reason) {
				return ErrInvalid
			}
			if v.Material {
				v.AffectedLearners[i].Status = "revalidation_required"
			} else {
				v.AffectedLearners[i].Status = "achievement_preserved"
			}
		}
		v.AuthorID = actor
		v.CreatedAt = s.now().UTC()
		r.Improvements = append(r.Improvements, v)
		return nil
	})
}
func (s *Store) Revalidate(repo, pathway, actor string, v Revalidation) (Record, error) {
	return s.mutate(repo, pathway, func(r *Record) error {
		if !id(v.ID) || !text(v.ImprovementID) || !text(v.LearnerID) || !text(v.CompletionEvidenceID) || v.FromPathwayVersion < 1 || v.ToPathwayVersion <= v.FromPathwayVersion || len(v.EvidenceReferences) == 0 {
			return ErrInvalid
		}
		ok := false
		for _, x := range r.Improvements {
			if x.ID == v.ImprovementID && x.Material && x.ReviewStatus == "approved" && x.TargetPathwayVersion == v.ToPathwayVersion {
				ok = true
			}
		}
		if !ok {
			return ErrInvalid
		}
		v.ActorID = actor
		v.CreatedAt = s.now().UTC()
		r.Revalidations = append(r.Revalidations, v)
		return nil
	})
}
func (s *Store) Get(repo, pathway string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pathway)
}
func (s *Store) read(repo, pathway string) (Record, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, pathway+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	var r Record
	if e == nil {
		e = json.Unmarshal(b, &r)
	}
	return r, e
}
func (s *Store) write(r Record) error {
	d := filepath.Join(s.root, r.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(r, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "outcomes-*.tmp")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if c := tmp.Close(); e == nil {
		e = c
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, r.PathwayID+".json"))
	}
	return e
}
