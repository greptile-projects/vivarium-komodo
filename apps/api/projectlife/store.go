// Package projectlife retains the evidence and accountable decisions of an incubated project in public life.
package projectlife

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

var ErrNotFound = errors.New("project life record not found")
var ErrInvalid = errors.New("invalid project life record")
var ErrForbidden = errors.New("project life collaborator required")
var ErrConflict = errors.New("project life revision conflict")

var PublicationKinds = map[string]bool{"release": true, "documentation": true, "package": true, "api_contract": true, "contributor_opportunity": true, "environment": true}
var SignalKinds = map[string]bool{"adoption": true, "support": true, "reliability": true, "cost": true, "success_measure": true}

type Input struct {
	IncubatorID       string   `json:"incubator_id"`
	AlternativeID     string   `json:"alternative_id"`
	BoundaryID        string   `json:"boundary_id"`
	BoundaryRevision  int64    `json:"boundary_revision"`
	DeliveryID        string   `json:"delivery_id"`
	DeliveryRevision  int64    `json:"delivery_revision"`
	ReadinessID       string   `json:"readiness_id"`
	ReadinessRevision int64    `json:"readiness_revision"`
	LaunchRevision    string   `json:"launch_revision"`
	Audience          string   `json:"audience"`
	OwnerIDs          []string `json:"owner_ids"`
}
type Publication struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Revision      string    `json:"revision"`
	Reference     string    `json:"reference"`
	Digest        string    `json:"digest"`
	Audience      string    `json:"audience"`
	Attestation   string    `json:"attestation"`
	PublishedByID string    `json:"published_by_id"`
	PublishedAt   time.Time `json:"published_at"`
}
type Signal struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Measure           string    `json:"measure"`
	Value             float64   `json:"value"`
	Unit              string    `json:"unit"`
	EvidenceReference string    `json:"evidence_reference"`
	EvidenceDigest    string    `json:"evidence_digest"`
	ObservedAt        time.Time `json:"observed_at"`
	RecordedByID      string    `json:"recorded_by_id"`
	RecordedAt        time.Time `json:"recorded_at"`
}
type Feedback struct {
	ID                string    `json:"id"`
	Audience          string    `json:"audience"`
	Summary           string    `json:"summary"`
	EvidenceReference string    `json:"evidence_reference"`
	SubmittedByID     string    `json:"submitted_by_id"`
	SubmittedAt       time.Time `json:"submitted_at"`
}
type Work struct {
	ID          string    `json:"id"`
	FeedbackID  string    `json:"feedback_id"`
	Kind        string    `json:"kind"`
	OwnerID     string    `json:"owner_id"`
	Title       string    `json:"title"`
	Reference   string    `json:"reference"`
	Status      string    `json:"status"`
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type RoadmapChange struct {
	ID                 string    `json:"id"`
	FeedbackID         string    `json:"feedback_id"`
	Revision           string    `json:"revision"`
	Summary            string    `json:"summary"`
	EvidenceReferences []string  `json:"evidence_references"`
	DecidedByID        string    `json:"decided_by_id"`
	DecidedAt          time.Time `json:"decided_at"`
}
type Obligation struct {
	Kind              string `json:"kind"`
	ResourceReference string `json:"resource_reference"`
	Resolution        string `json:"resolution"`
	EvidenceReference string `json:"evidence_reference"`
}
type Disposition struct {
	State           string       `json:"state"`
	TargetReference string       `json:"target_reference,omitempty"`
	Reason          string       `json:"reason"`
	Obligations     []Obligation `json:"obligations"`
	DecidedByID     string       `json:"decided_by_id"`
	DecidedAt       time.Time    `json:"decided_at"`
}
type Record struct {
	ID string `json:"id"`
	Input
	Revision         int64           `json:"revision"`
	Publications     []Publication   `json:"publications"`
	Signals          []Signal        `json:"signals"`
	Feedback         []Feedback      `json:"feedback"`
	Work             []Work          `json:"work"`
	Roadmap          []RoadmapChange `json:"roadmap"`
	Disposition      *Disposition    `json:"disposition,omitempty"`
	Blockers         []string        `json:"blockers"`
	AuthorityGranted bool            `json:"authority_granted"`
	CreatedByID      string          `json:"created_by_id"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
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
func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func validInput(v Input) bool {
	return v.IncubatorID != "" && v.AlternativeID != "" && v.BoundaryID != "" && v.BoundaryRevision > 0 && v.DeliveryID != "" && v.DeliveryRevision > 0 && v.ReadinessID != "" && v.ReadinessRevision > 0 && v.LaunchRevision != "" && v.Audience != "" && len(v.OwnerIDs) > 0
}
func derive(v *Record) {
	v.Blockers = nil
	if len(v.Publications) == 0 {
		v.Blockers = append(v.Blockers, "first attested publication required")
	}
	if v.Disposition != nil && (v.Disposition.State == "graduated" || v.Disposition.State == "merged" || v.Disposition.State == "archived") {
		for _, o := range v.Disposition.Obligations {
			if o.Kind == "" || o.ResourceReference == "" || o.Resolution == "" || o.EvidenceReference == "" {
				v.Blockers = append(v.Blockers, "unresolved resource or obligation")
			}
		}
	}
}
func (s *Store) Create(actor string, in Input) (Record, error) {
	if actor == "" || !validInput(in) || !contains(in.OwnerIDs, actor) {
		return Record{}, ErrInvalid
	}
	n := s.now().UTC()
	v := Record{ID: newID("life_"), Input: in, Revision: 1, Publications: []Publication{}, Signals: []Signal{}, Feedback: []Feedback{}, Work: []Work{}, Roadmap: []RoadmapChange{}, CreatedByID: actor, CreatedAt: n, UpdatedAt: n}
	derive(&v)
	s.mu.Lock()
	defer s.mu.Unlock()
	return v, s.write(v)
}
func (s *Store) Get(id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e == nil {
		derive(&v)
	}
	return v, e
}
func (s *Store) List() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Record{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			v, x := s.read(strings.TrimSuffix(f.Name(), ".json"))
			if x != nil {
				return nil, x
			}
			derive(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) mutate(id, actor string, rev int64, fn func(*Record) error) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil {
		return v, e
	}
	if !contains(v.OwnerIDs, actor) {
		return v, ErrForbidden
	}
	if v.Revision != rev {
		return v, ErrConflict
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.Revision++
	v.UpdatedAt = s.now().UTC()
	derive(&v)
	return v, s.write(v)
}
func (s *Store) Publish(id, actor string, rev int64, p Publication) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		if !PublicationKinds[p.Kind] || p.Revision != v.LaunchRevision || p.Reference == "" || p.Digest == "" || p.Attestation == "" || p.Audience != v.Audience {
			return ErrInvalid
		}
		p.ID = newID("pub_")
		p.PublishedByID = actor
		p.PublishedAt = s.now().UTC()
		v.Publications = append(v.Publications, p)
		return nil
	})
}
func (s *Store) Observe(id, actor string, rev int64, x Signal) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		if !SignalKinds[x.Kind] || x.Measure == "" || x.Unit == "" || x.EvidenceReference == "" || x.EvidenceDigest == "" || x.ObservedAt.IsZero() {
			return ErrInvalid
		}
		x.ID = newID("signal_")
		x.RecordedByID = actor
		x.RecordedAt = s.now().UTC()
		v.Signals = append(v.Signals, x)
		return nil
	})
}
func (s *Store) AddFeedback(id, actor string, rev int64, x Feedback) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		if x.Summary == "" || x.EvidenceReference == "" || x.Audience == "" {
			return ErrInvalid
		}
		x.ID = newID("feedback_")
		x.SubmittedByID = actor
		x.SubmittedAt = s.now().UTC()
		v.Feedback = append(v.Feedback, x)
		return nil
	})
}
func (s *Store) AddWork(id, actor string, rev int64, x Work) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		found := false
		for _, f := range v.Feedback {
			found = found || f.ID == x.FeedbackID
		}
		if !found || (x.Kind != "human" && x.Kind != "agent") || x.OwnerID == "" || x.Title == "" || x.Reference == "" {
			return ErrInvalid
		}
		x.ID = newID("work_")
		x.Status = "open"
		x.CreatedByID = actor
		x.CreatedAt = s.now().UTC()
		v.Work = append(v.Work, x)
		return nil
	})
}
func (s *Store) ReviseRoadmap(id, actor string, rev int64, x RoadmapChange) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		found := false
		for _, f := range v.Feedback {
			found = found || f.ID == x.FeedbackID
		}
		if !found || x.Revision == "" || x.Summary == "" || len(x.EvidenceReferences) == 0 {
			return ErrInvalid
		}
		x.ID = newID("roadmap_")
		x.DecidedByID = actor
		x.DecidedAt = s.now().UTC()
		v.Roadmap = append(v.Roadmap, x)
		return nil
	})
}
func (s *Store) Decide(id, actor string, rev int64, x Disposition) (Record, error) {
	return s.mutate(id, actor, rev, func(v *Record) error {
		if !map[string]bool{"graduated": true, "experimental": true, "merged": true, "archived": true}[x.State] || x.Reason == "" || ((x.State == "graduated" || x.State == "merged") && x.TargetReference == "") {
			return ErrInvalid
		}
		if x.State != "experimental" {
			if len(x.Obligations) == 0 {
				return ErrInvalid
			}
			for _, o := range x.Obligations {
				if o.Kind == "" || o.ResourceReference == "" || o.Resolution == "" || o.EvidenceReference == "" {
					return ErrInvalid
				}
			}
		}
		x.DecidedByID = actor
		x.DecidedAt = s.now().UTC()
		v.Disposition = &x
		return nil
	})
}
func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }
func (s *Store) write(v Record) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	p := s.path(v.ID) + ".tmp"
	if e = os.WriteFile(p, b, 0640); e == nil {
		e = os.Rename(p, s.path(v.ID))
	}
	return e
}
func (s *Store) read(id string) (Record, error) {
	var v Record
	b, e := os.ReadFile(s.path(id))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
