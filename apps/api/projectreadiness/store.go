// Package projectreadiness retains revision-exact evidence and owner decisions for an incubated project's first launch.
package projectreadiness

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

var ErrNotFound = errors.New("project readiness not found")
var ErrInvalid = errors.New("invalid project readiness")
var ErrForbidden = errors.New("project readiness owner required")
var ErrConflict = errors.New("project readiness revision conflict")

var RequiredCategories = []string{"ownership", "support_governance", "licensing_provenance", "security_privacy", "accessibility", "documentation", "package_api_adoption", "service_objectives", "continuity", "contributor_setup", "operating_budget", "prototype_debt", "user_validation"}

type Evidence struct {
	Category          string    `json:"category"`
	Reference         string    `json:"reference"`
	Digest            string    `json:"digest"`
	Summary           string    `json:"summary"`
	Outcome           string    `json:"outcome"`
	MaintainerIDs     []string  `json:"maintainer_ids,omitempty"`
	SafeDefaults      bool      `json:"safe_defaults"`
	SupportedPromises bool      `json:"supported_promises"`
	UserValidated     bool      `json:"user_validated"`
	RecordedByID      string    `json:"recorded_by_id"`
	RecordedAt        time.Time `json:"recorded_at"`
}
type Decision struct {
	Category       string    `json:"category"`
	OwnerID        string    `json:"owner_id"`
	EvidenceDigest string    `json:"evidence_digest"`
	Decision       string    `json:"decision"`
	Reason         string    `json:"reason"`
	NarrowedScope  string    `json:"narrowed_scope,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
	FollowUpWork   string    `json:"follow_up_work,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
type Input struct {
	IncubatorID      string              `json:"incubator_id"`
	BoundaryID       string              `json:"boundary_id"`
	BoundaryRevision int64               `json:"boundary_revision"`
	AlternativeID    string              `json:"alternative_id"`
	DeliveryID       string              `json:"delivery_id"`
	DeliveryRevision int64               `json:"delivery_revision"`
	LaunchRevision   string              `json:"launch_revision"`
	DeclaredScope    string              `json:"declared_scope"`
	RequiredOwners   map[string][]string `json:"required_owners"`
}
type Readiness struct {
	ID string `json:"id"`
	Input
	Revision          int64      `json:"revision"`
	Evidence          []Evidence `json:"evidence"`
	Decisions         []Decision `json:"decisions"`
	MissingCategories []string   `json:"missing_categories"`
	Blockers          []string   `json:"blockers"`
	EffectiveScope    string     `json:"effective_scope"`
	Ready             bool       `json:"ready"`
	AuthorityGranted  bool       `json:"authority_granted"`
	CreatedByID       string     `json:"created_by_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return "ready_" + hex.EncodeToString(b[:]) }
func category(x string) bool {
	for _, v := range RequiredCategories {
		if x == v {
			return true
		}
	}
	return false
}
func valid(in Input) bool {
	if in.IncubatorID == "" || in.BoundaryID == "" || in.BoundaryRevision < 1 || in.AlternativeID == "" || in.DeliveryID == "" || in.DeliveryRevision < 1 || in.LaunchRevision == "" || (in.DeclaredScope != "public" && in.DeclaredScope != "limited") {
		return false
	}
	for _, c := range RequiredCategories {
		if len(in.RequiredOwners[c]) == 0 {
			return false
		}
		for _, o := range in.RequiredOwners[c] {
			if strings.TrimSpace(o) == "" {
				return false
			}
		}
	}
	return true
}
func latestEvidence(v *Readiness, c string) *Evidence {
	for i := len(v.Evidence) - 1; i >= 0; i-- {
		if v.Evidence[i].Category == c {
			return &v.Evidence[i]
		}
	}
	return nil
}
func derive(v *Readiness, now time.Time) {
	v.MissingCategories = nil
	v.Blockers = nil
	v.EffectiveScope = v.DeclaredScope
	for _, c := range RequiredCategories {
		e := latestEvidence(v, c)
		if e == nil {
			v.MissingCategories = append(v.MissingCategories, c)
			v.Blockers = append(v.Blockers, "missing current evidence for "+c)
			continue
		}
		if c == "ownership" && len(e.MaintainerIDs) == 0 {
			v.Blockers = append(v.Blockers, "missing maintainers")
		}
		unsafe := !e.SafeDefaults
		unsupported := !e.SupportedPromises
		failedValidation := c == "user_validation" && !e.UserValidated
		accepted := false
		excepted := false
		for i := len(v.Decisions) - 1; i >= 0; i-- {
			d := v.Decisions[i]
			if d.Category == c && d.EvidenceDigest == e.Digest {
				accepted = d.Decision == "accepted"
				excepted = d.Decision == "exception" && d.ExpiresAt.After(now) && d.FollowUpWork != "" && d.NarrowedScope != ""
				break
			}
		}
		if e.Outcome != "passed" || unsafe || unsupported || failedValidation {
			if excepted {
				for i := len(v.Decisions) - 1; i >= 0; i-- {
					if v.Decisions[i].Category == c && v.Decisions[i].EvidenceDigest == e.Digest {
						v.EffectiveScope = v.Decisions[i].NarrowedScope
						break
					}
				}
			} else {
				v.Blockers = append(v.Blockers, "unresolved "+c+" evidence")
			}
		} else if !accepted && !excepted {
			v.Blockers = append(v.Blockers, "owner decision required for "+c)
		} else if excepted {
			for i := len(v.Decisions) - 1; i >= 0; i-- {
				if v.Decisions[i].Category == c && v.Decisions[i].EvidenceDigest == e.Digest {
					v.EffectiveScope = v.Decisions[i].NarrowedScope
					break
				}
			}
		}
	}
	v.Ready = len(v.Blockers) == 0
}
func (s *Store) Create(actor string, in Input) (Readiness, error) {
	if actor == "" || !valid(in) {
		return Readiness{}, ErrInvalid
	}
	n := s.now().UTC()
	v := Readiness{ID: id(), Input: in, Revision: 1, Evidence: []Evidence{}, Decisions: []Decision{}, AuthorityGranted: false, CreatedByID: actor, CreatedAt: n, UpdatedAt: n}
	derive(&v, n)
	s.mu.Lock()
	defer s.mu.Unlock()
	return v, s.write(v)
}
func (s *Store) Get(x string) (Readiness, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e == nil {
		derive(&v, s.now().UTC())
	}
	return v, e
}
func (s *Store) AddEvidence(x, actor string, expected int64, e Evidence) (Readiness, error) {
	return s.mutate(x, expected, func(v *Readiness) error {
		if !isOwner(v, e.Category, actor) {
			return ErrForbidden
		}
		if !category(e.Category) || e.Reference == "" || e.Digest == "" || e.Summary == "" || (e.Outcome != "passed" && e.Outcome != "failed") {
			return ErrInvalid
		}
		e.RecordedByID = actor
		e.RecordedAt = s.now().UTC()
		v.Evidence = append(v.Evidence, e)
		return nil
	})
}
func (s *Store) Decide(x, actor string, expected int64, d Decision) (Readiness, error) {
	return s.mutate(x, expected, func(v *Readiness) error {
		if !isOwner(v, d.Category, actor) {
			return ErrForbidden
		}
		e := latestEvidence(v, d.Category)
		if e == nil || d.EvidenceDigest != e.Digest || d.Reason == "" || (d.Decision != "accepted" && d.Decision != "exception") {
			return ErrInvalid
		}
		if d.Decision == "accepted" && (e.Outcome != "passed" || !e.SafeDefaults || !e.SupportedPromises || (d.Category == "user_validation" && !e.UserValidated)) {
			return ErrInvalid
		}
		if d.Decision == "exception" && (d.FollowUpWork == "" || d.NarrowedScope == "" || d.NarrowedScope == "public" || d.NarrowedScope == v.DeclaredScope || !d.ExpiresAt.After(s.now()) || d.ExpiresAt.After(s.now().Add(90*24*time.Hour))) {
			return ErrInvalid
		}
		d.OwnerID = actor
		d.CreatedAt = s.now().UTC()
		v.Decisions = append(v.Decisions, d)
		return nil
	})
}
func isOwner(v *Readiness, c, a string) bool {
	for _, x := range v.RequiredOwners[c] {
		if x == a {
			return true
		}
	}
	return false
}
func (s *Store) mutate(x string, expected int64, fn func(*Readiness) error) (Readiness, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil {
		return v, e
	}
	if v.Revision != expected {
		return v, ErrConflict
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.Revision++
	v.UpdatedAt = s.now().UTC()
	derive(&v, s.now().UTC())
	return v, s.write(v)
}
func (s *Store) path(x string) string { return filepath.Join(s.root, x+".json") }
func (s *Store) write(v Readiness) error {
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
func (s *Store) read(x string) (Readiness, error) {
	var v Readiness
	b, e := os.ReadFile(s.path(x))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) List() ([]Readiness, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Readiness{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			v, x := s.read(strings.TrimSuffix(f.Name(), ".json"))
			if x != nil {
				return nil, x
			}
			derive(&v, s.now().UTC())
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
