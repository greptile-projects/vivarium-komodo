// Package projectboundaries owns activation-gated project bootstrap manifests.
package projectboundaries

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
	ErrNotFound  = errors.New("project boundary not found")
	ErrInvalid   = errors.New("invalid project boundary")
	ErrForbidden = errors.New("project boundary forbidden")
	ErrConflict  = errors.New("project boundary conflict")
)
var requiredKinds = []string{"organization", "repository", "team", "package", "agent_role", "contributor_pathway", "documentation", "environment", "review_policy", "security_policy", "privacy_policy", "quality_policy", "release_policy"}

type Access struct {
	SubjectID string `json:"subject_id"`
	Role      string `json:"role"`
	Source    string `json:"source"`
}
type GeneratedContent struct {
	Path          string   `json:"path"`
	Template      string   `json:"template"`
	Source        string   `json:"source"`
	ApprovedByIDs []string `json:"approved_by_ids"`
}
type Policy struct {
	Kind          string `json:"kind"`
	Source        string `json:"source"`
	InheritedFrom string `json:"inherited_from,omitempty"`
	Summary       string `json:"summary"`
}
type Resource struct {
	Kind        string             `json:"kind"`
	Mode        string             `json:"mode"`
	Name        string             `json:"name"`
	ConnectedID string             `json:"connected_id,omitempty"`
	OwnerIDs    []string           `json:"owner_ids"`
	Access      []Access           `json:"effective_access"`
	MonthlyCost float64            `json:"monthly_cost"`
	Generated   []GeneratedContent `json:"generated_content,omitempty"`
	Policies    []Policy           `json:"policies,omitempty"`
	Handle      string             `json:"handle,omitempty"`
	State       string             `json:"state"`
}
type Approval struct {
	OwnerID   string    `json:"owner_id"`
	Revision  int64     `json:"revision"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
type Attempt struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	Revision        int64     `json:"revision"`
	Outcome         string    `json:"outcome"`
	ActorID         string    `json:"actor_id"`
	ResourceHandles []string  `json:"resource_handles"`
	CreatedAt       time.Time `json:"created_at"`
}
type Input struct {
	IncubatorID        string     `json:"incubator_id"`
	AlternativeID      string     `json:"alternative_id"`
	Title              string     `json:"title"`
	Visibility         string     `json:"visibility"`
	OwnerIDs           []string   `json:"owner_ids"`
	Resources          []Resource `json:"resources"`
	RecurringCostLimit float64    `json:"recurring_cost_limit"`
}
type Boundary struct {
	ID string `json:"id"`
	Input
	Revision           int64      `json:"revision"`
	State              string     `json:"state"`
	MissingKinds       []string   `json:"missing_resource_kinds"`
	TotalMonthlyCost   float64    `json:"total_monthly_cost"`
	Approvals          []Approval `json:"approvals"`
	Attempts           []Attempt  `json:"attempts"`
	ActivationBlockers []string   `json:"activation_blockers"`
	AuthorityGranted   bool       `json:"authority_granted"`
	CreatedByID        string     `json:"created_by_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
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
func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func valid(in Input) bool {
	if in.IncubatorID == "" || in.AlternativeID == "" || strings.TrimSpace(in.Title) == "" || len(in.OwnerIDs) == 0 || (in.Visibility != "public" && in.Visibility != "owners") || in.RecurringCostLimit < 0 || len(in.Resources) > 100 {
		return false
	}
	for _, r := range in.Resources {
		if r.Kind == "" || r.Name == "" || len(r.OwnerIDs) == 0 || (r.Mode != "create" && r.Mode != "connect") || (r.Mode == "connect" && r.ConnectedID == "") || r.MonthlyCost < 0 {
			return false
		}
		for _, g := range r.Generated {
			if g.Path == "" || g.Template == "" || g.Source == "" || len(g.ApprovedByIDs) == 0 {
				return false
			}
		}
	}
	return true
}
func derive(v *Boundary) {
	seen := map[string]bool{}
	v.TotalMonthlyCost = 0
	for i := range v.Resources {
		r := &v.Resources[i]
		seen[r.Kind] = true
		v.TotalMonthlyCost += r.MonthlyCost
		if r.State == "" {
			r.State = "planned"
		}
	}
	v.MissingKinds = nil
	for _, k := range requiredKinds {
		if !seen[k] {
			v.MissingKinds = append(v.MissingKinds, k)
		}
	}
	v.ActivationBlockers = nil
	if len(v.MissingKinds) > 0 {
		v.ActivationBlockers = append(v.ActivationBlockers, "required resources are missing")
	}
	if v.TotalMonthlyCost > v.RecurringCostLimit {
		v.ActivationBlockers = append(v.ActivationBlockers, "recurring cost exceeds approved limit")
	}
	approved := map[string]bool{}
	for _, a := range v.Approvals {
		if a.Revision == v.Revision && a.Decision == "approved" {
			approved[a.OwnerID] = true
		}
	}
	for _, o := range v.OwnerIDs {
		if !approved[o] {
			v.ActivationBlockers = append(v.ActivationBlockers, "approval required from "+o)
		}
	}
}
func (s *Store) Create(actor string, in Input) (Boundary, error) {
	if !valid(in) || !has(in.OwnerIDs, actor) {
		return Boundary{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Boundary{ID: id("prj_"), Input: in, Revision: 1, State: "draft", Approvals: []Approval{}, Attempts: []Attempt{}, AuthorityGranted: false, CreatedByID: actor, CreatedAt: now, UpdatedAt: now}
	derive(&v)
	return v, s.write(v)
}
func (s *Store) Get(pid, actor string, public bool) (Boundary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e == nil && v.Visibility != "public" && !has(v.OwnerIDs, actor) {
		return Boundary{}, ErrNotFound
	}
	if e == nil && public && v.Visibility != "public" {
		return Boundary{}, ErrNotFound
	}
	derive(&v)
	return v, e
}
func (s *Store) Decide(pid, actor, decision, reason string, revision int64) (Boundary, error) {
	return s.mutate(pid, actor, func(v *Boundary) error {
		if !has(v.OwnerIDs, actor) {
			return ErrForbidden
		}
		if revision != v.Revision {
			return ErrConflict
		}
		if decision != "approved" && decision != "rejected" {
			return ErrInvalid
		}
		v.Approvals = append(v.Approvals, Approval{actor, revision, decision, reason, s.now().UTC()})
		return nil
	})
}
func (s *Store) Activate(pid, actor string, revision int64) (Boundary, error) {
	return s.mutate(pid, actor, func(v *Boundary) error {
		if !has(v.OwnerIDs, actor) {
			return ErrForbidden
		}
		if revision != v.Revision {
			return ErrConflict
		}
		derive(v)
		if len(v.ActivationBlockers) > 0 {
			return ErrConflict
		}
		if v.State == "active" {
			return nil
		}
		handles := []string{}
		for i := range v.Resources {
			r := &v.Resources[i]
			if r.Mode == "connect" {
				r.Handle = r.ConnectedID
			} else {
				r.Handle = v.ID + ":" + r.Kind + ":" + r.Name
			}
			r.State = "active"
			handles = append(handles, r.Handle)
		}
		v.State = "active"
		v.Attempts = append(v.Attempts, Attempt{id("run_"), "activation", revision, "committed", actor, handles, s.now().UTC()})
		return nil
	})
}
func (s *Store) Rollback(pid, actor, reason string, revision int64) (Boundary, error) {
	return s.mutate(pid, actor, func(v *Boundary) error {
		if !has(v.OwnerIDs, actor) {
			return ErrForbidden
		}
		if revision != v.Revision || v.State != "active" || strings.TrimSpace(reason) == "" {
			return ErrConflict
		}
		old := []string{}
		for i := range v.Resources {
			old = append(old, v.Resources[i].Handle)
			if v.Resources[i].Mode == "connect" {
				v.Resources[i].State = "connected"
				continue
			}
			v.Resources[i].Handle = ""
			v.Resources[i].State = "rolled_back"
		}
		v.State = "rolled_back"
		v.Attempts = append(v.Attempts, Attempt{id("run_"), "rollback", revision, "committed", actor, old, s.now().UTC()})
		return nil
	})
}
func (s *Store) mutate(pid, actor string, fn func(*Boundary) error) (Boundary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e != nil {
		return v, e
	}
	if !has(v.OwnerIDs, actor) {
		return Boundary{}, ErrForbidden
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	derive(&v)
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) path(x string) string { return filepath.Join(s.root, x+".json") }
func (s *Store) write(v Boundary) error {
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
func (s *Store) read(x string) (Boundary, error) {
	var v Boundary
	b, e := os.ReadFile(s.path(x))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) ListPublic() ([]Boundary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Boundary{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			if v.Visibility == "public" {
				derive(&v)
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
