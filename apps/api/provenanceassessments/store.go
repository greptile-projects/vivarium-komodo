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
	CreatedAt time.Time `json:"created_at"`
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
		if actor == "" || !oneOf(actorKind, "human", "agent") || !oneOf(a.Kind, "challenge", "origin_evidence") || strings.TrimSpace(a.Body) == "" || strings.TrimSpace(a.Citation) == "" {
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
