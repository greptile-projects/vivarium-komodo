// Package provenanceassessments retains candidate-bound provenance and licensing decisions.
package provenanceassessments

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

var ErrNotFound = errors.New("provenance assessment not found")
var ErrInvalid = errors.New("invalid provenance assessment")
var ErrConflict = errors.New("provenance assessment revision conflict")

type InputKey struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
}
type Finding struct {
	ID                  string   `json:"id"`
	Kind                string   `json:"kind"`
	Subject             string   `json:"subject"`
	Detail              string   `json:"detail"`
	Blocking            bool     `json:"blocking"`
	DistributionTargets []string `json:"distribution_targets"`
}
type Annotation struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	FindingID string    `json:"finding_id,omitempty"`
	Body      string    `json:"body"`
	Citation  string    `json:"citation"`
	Origin    string    `json:"origin,omitempty"`
	License   string    `json:"license,omitempty"`
	ActorID   string    `json:"actor_id"`
	ActorKind string    `json:"actor_kind"`
	Audience  string    `json:"audience"`
	CreatedAt time.Time `json:"created_at"`
}
type WorkLink struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision,omitempty"`
}
type RepairProgress struct {
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type RepairDelivery struct {
	Revision            string     `json:"revision"`
	PullRequestID       string     `json:"pull_request_id"`
	CheckRunIDs         []string   `json:"check_run_ids"`
	Links               []WorkLink `json:"links"`
	AuthorshipPreserved bool       `json:"authorship_preserved"`
	Summary             string     `json:"summary"`
	ActorID             string     `json:"actor_id"`
	CreatedAt           time.Time  `json:"created_at"`
}
type Repair struct {
	ID                   string           `json:"id"`
	FindingID            string           `json:"finding_id"`
	Strategy             string           `json:"strategy"`
	AffectedRevision     string           `json:"affected_revision"`
	PolicyID             string           `json:"policy_id"`
	PolicyVersion        int64            `json:"policy_version"`
	Obligations          []string         `json:"obligations"`
	AcceptanceCriteria   []string         `json:"acceptance_criteria"`
	PermittedEvidenceIDs []string         `json:"permitted_evidence_ids"`
	OwnerKind            string           `json:"owner_kind"`
	OwnerID              string           `json:"owner_id"`
	CleanRoom            bool             `json:"clean_room"`
	EvidenceReviewerIDs  []string         `json:"evidence_reviewer_ids,omitempty"`
	Links                []WorkLink       `json:"links"`
	Progress             []RepairProgress `json:"progress"`
	Delivery             *RepairDelivery  `json:"delivery,omitempty"`
	CreatedByID          string           `json:"created_by_id"`
	CreatedAt            time.Time        `json:"created_at"`
}
type Decision struct {
	ID        string     `json:"id"`
	FindingID string     `json:"finding_id"`
	Decision  string     `json:"decision"`
	Rationale string     `json:"rationale"`
	OwnerID   string     `json:"owner_id"`
	ExpiresAt time.Time  `json:"expires_at,omitempty"`
	InputKeys []InputKey `json:"input_keys"`
	CreatedAt time.Time  `json:"created_at"`
}
type Assessment struct {
	ID                  string       `json:"id"`
	RepositoryID        string       `json:"repository_id"`
	CandidateKind       string       `json:"candidate_kind"`
	CandidateID         string       `json:"candidate_id"`
	Revision            string       `json:"revision"`
	DistributionTargets []string     `json:"distribution_targets"`
	GraphID             string       `json:"graph_id"`
	PolicyID            string       `json:"policy_id"`
	PolicyVersion       int64        `json:"policy_version"`
	InputKeys           []InputKey   `json:"input_keys"`
	Findings            []Finding    `json:"findings"`
	Annotations         []Annotation `json:"annotations"`
	Decisions           []Decision   `json:"decisions"`
	Repairs             []Repair     `json:"repairs"`
	RevisionNumber      int64        `json:"revision_number"`
	CreatedByID         string       `json:"created_by_id"`
	CreatedAt           time.Time    `json:"created_at"`
}
type View struct {
	Assessment
	Status         string     `json:"status"`
	Ready          bool       `json:"ready"`
	StaleInputKeys []InputKey `json:"stale_input_keys"`
	Blockers       []Finding  `json:"blockers"`
}

func Project(v View, restricted bool) View {
	if restricted {
		return v
	}
	v.Annotations = append([]Annotation{}, v.Annotations...)
	for i := range v.Annotations {
		if v.Annotations[i].Audience == "restricted" {
			v.Annotations[i].Body, v.Annotations[i].Citation, v.Annotations[i].Origin, v.Annotations[i].License = "", "", "", ""
		}
	}
	return v
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
func ident() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func clean(xs []string) bool {
	seen := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func (s *Store) save(v Assessment) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		e = os.WriteFile(filepath.Join(s.root, v.ID+".json"), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(v Assessment) (Assessment, error) {
	if v.RepositoryID == "" || !oneOf(v.CandidateKind, "pull_request", "stack", "package", "release_candidate") || v.CandidateID == "" || v.Revision == "" || v.GraphID == "" || v.PolicyID == "" || v.PolicyVersion < 1 || v.CreatedByID == "" || len(v.DistributionTargets) == 0 || !clean(v.DistributionTargets) {
		return Assessment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return Assessment{}, e
	}
	for _, x := range all {
		if x.RepositoryID == v.RepositoryID && x.CandidateKind == v.CandidateKind && x.CandidateID == v.CandidateID && x.Revision == v.Revision && x.GraphID == v.GraphID && x.PolicyID == v.PolicyID && x.PolicyVersion == v.PolicyVersion {
			return Assessment{}, ErrConflict
		}
	}
	v.ID = ident()
	v.RevisionNumber = 1
	v.CreatedAt = s.now().UTC()
	v.Annotations = []Annotation{}
	v.Decisions = []Decision{}
	v.Repairs = []Repair{}
	return v, s.save(v)
}
func oneOf(x string, xs ...string) bool {
	for _, v := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func (s *Store) mutate(id string, expected int64, fn func(*Assessment) error) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil {
		return Assessment{}, e
	}
	if v.RevisionNumber != expected {
		return Assessment{}, ErrConflict
	}
	if e = fn(&v); e != nil {
		return Assessment{}, e
	}
	v.RevisionNumber++
	return v, s.save(v)
}
func (s *Store) Annotate(id, actor, actorKind string, expected int64, a Annotation) (Assessment, error) {
	return s.mutate(id, expected, func(v *Assessment) error {
		if a.Audience == "" {
			a.Audience = "repository"
		}
		if actor == "" || !oneOf(actorKind, "human", "agent") || !oneOf(a.Audience, "repository", "restricted") || !oneOf(a.Kind, "challenge", "origin_evidence") || strings.TrimSpace(a.Body) == "" || strings.TrimSpace(a.Citation) == "" {
			return ErrInvalid
		}
		if a.FindingID != "" && !hasFinding(v.Findings, a.FindingID) {
			return ErrInvalid
		}
		a.ID = ident()
		a.ActorID = actor
		a.ActorKind = actorKind
		a.CreatedAt = s.now().UTC()
		v.Annotations = append(v.Annotations, a)
		return nil
	})
}
func (s *Store) CreateRepair(id, actor string, expected int64, x Repair) (Assessment, Repair, error) {
	var made Repair
	v, err := s.mutate(id, expected, func(v *Assessment) error {
		if actor == "" || !hasFinding(v.Findings, x.FindingID) || !oneOf(x.Strategy, "replace", "reimplement", "remove", "obtain_permission", "isolate") || !oneOf(x.OwnerKind, "human", "agent") || x.OwnerID == "" || len(x.AcceptanceCriteria) == 0 || !clean(x.AcceptanceCriteria) || !clean(x.PermittedEvidenceIDs) || len(x.Links) == 0 {
			return ErrInvalid
		}
		for _, prior := range v.Repairs {
			if prior.FindingID == x.FindingID && prior.Delivery == nil {
				return ErrConflict
			}
		}
		annotations := map[string]Annotation{}
		for _, a := range v.Annotations {
			if a.FindingID == x.FindingID {
				annotations[a.ID] = a
			}
		}
		for _, evidence := range x.PermittedEvidenceIDs {
			a, ok := annotations[evidence]
			if !ok || (x.CleanRoom && a.Audience == "restricted") {
				return ErrInvalid
			}
		}
		if x.CleanRoom {
			if x.Strategy != "reimplement" || len(x.EvidenceReviewerIDs) == 0 || !clean(x.EvidenceReviewerIDs) {
				return ErrInvalid
			}
			for _, reviewer := range x.EvidenceReviewerIDs {
				if reviewer == x.OwnerID || reviewer == "" {
					return ErrInvalid
				}
			}
		}
		for _, link := range x.Links {
			if !oneOf(link.Kind, "branch", "fork", "session", "workspace", "task") || link.ResourceID == "" {
				return ErrInvalid
			}
		}
		x.ID, x.AffectedRevision, x.PolicyID, x.PolicyVersion = ident(), v.Revision, v.PolicyID, v.PolicyVersion
		x.Progress, x.Delivery, x.CreatedByID, x.CreatedAt = []RepairProgress{}, nil, actor, s.now().UTC()
		v.Repairs = append(v.Repairs, x)
		made = x
		return nil
	})
	return v, made, err
}
func (s *Store) ProgressRepair(id, repair, actor, status, summary string, expected int64) (Assessment, error) {
	return s.mutate(id, expected, func(v *Assessment) error {
		for i := range v.Repairs {
			if v.Repairs[i].ID == repair {
				if actor == "" || strings.TrimSpace(summary) == "" || !oneOf(status, "started", "blocked", "review", "completed") {
					return ErrInvalid
				}
				v.Repairs[i].Progress = append(v.Repairs[i].Progress, RepairProgress{Status: status, Summary: summary, ActorID: actor, CreatedAt: s.now().UTC()})
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) DeliverRepair(id, repair, actor string, expected int64, d RepairDelivery) (Assessment, error) {
	return s.mutate(id, expected, func(v *Assessment) error {
		for i := range v.Repairs {
			if v.Repairs[i].ID == repair {
				if v.Repairs[i].Delivery != nil {
					return ErrConflict
				}
				if actor == "" || d.Revision == "" || d.PullRequestID == "" || len(d.CheckRunIDs) == 0 || !clean(d.CheckRunIDs) || !d.AuthorshipPreserved || strings.TrimSpace(d.Summary) == "" {
					return ErrInvalid
				}
				d.ActorID, d.CreatedAt = actor, s.now().UTC()
				v.Repairs[i].Delivery = &d
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) Decide(id, actor string, expected int64, d Decision) (Assessment, error) {
	return s.mutate(id, expected, func(v *Assessment) error {
		if actor == "" || d.OwnerID != actor || !hasFinding(v.Findings, d.FindingID) || !oneOf(d.Decision, "acknowledged", "resolved", "exception") || strings.TrimSpace(d.Rationale) == "" {
			return ErrInvalid
		}
		if d.Decision == "exception" && !d.ExpiresAt.After(s.now()) {
			return ErrInvalid
		}
		d.ID = ident()
		d.CreatedAt = s.now().UTC()
		d.InputKeys = append([]InputKey{}, v.InputKeys...)
		v.Decisions = append(v.Decisions, d)
		return nil
	})
}
func hasFinding(xs []Finding, id string) bool {
	for _, x := range xs {
		if x.ID == id {
			return true
		}
	}
	return false
}
func (s *Store) Get(id string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}
func (s *Store) List(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	out := []Assessment{}
	for _, v := range all {
		if v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	return out, e
}
func (s *Store) read(id string) (Assessment, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Assessment{}, ErrNotFound
	}
	var v Assessment
	if e != nil || json.Unmarshal(b, &v) != nil || v.ID != id {
		return Assessment{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) list() ([]Assessment, error) {
	es, e := os.ReadDir(s.root)
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func Derive(v Assessment, current []InputKey, now time.Time) View {
	stale := []InputKey{}
	cm := map[string]string{}
	for _, k := range current {
		cm[k.Kind+"\x00"+k.Reference] = k.Revision
	}
	for _, k := range v.InputKeys {
		if cm[k.Kind+"\x00"+k.Reference] != k.Revision {
			stale = append(stale, k)
		}
	}
	blockers := []Finding{}
	for _, f := range v.Findings {
		if !f.Blocking {
			continue
		}
		resolved := false
		for i := len(v.Decisions) - 1; i >= 0; i-- {
			d := v.Decisions[i]
			if d.FindingID != f.ID {
				continue
			}
			if d.Decision == "resolved" || d.Decision == "acknowledged" || (d.Decision == "exception" && d.ExpiresAt.After(now)) {
				resolved = true
			}
			break
		}
		if !resolved {
			blockers = append(blockers, f)
		}
	}
	status := "ready"
	if len(stale) > 0 {
		status = "stale"
	} else if len(blockers) > 0 {
		status = "blocked"
	}
	return View{v, status, status == "ready", stale, blockers}
}
