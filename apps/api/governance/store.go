// Package governance owns versioned project charters. Charters describe
// legitimate decision-making but never grant operational authority.
package governance

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

var ErrNotFound = errors.New("governance charter not found")
var ErrInvalid = errors.New("invalid governance charter")
var ErrConflict = errors.New("governance charter conflict")

type Role struct {
	Name             string   `json:"name"`
	Purpose          string   `json:"purpose"`
	Eligibility      []string `json:"eligibility"`
	Responsibilities []string `json:"responsibilities"`
	TermDays         int      `json:"term_days,omitempty"`
	MinimumMembers   int      `json:"minimum_members"`
}
type DecisionClass struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	EligibleRoles      []string `json:"eligible_roles"`
	Participation      string   `json:"participation"`
	Quorum             int      `json:"quorum"`
	Threshold          int      `json:"threshold"`
	ProtectedResources []string `json:"protected_resources"`
}
type Procedures struct {
	Removal    string `json:"removal"`
	Succession string `json:"succession"`
	Vacancy    string `json:"vacancy"`
}
type AmendmentPolicy struct {
	EligibleRoles []string `json:"eligible_roles"`
	NoticeDays    int      `json:"notice_days"`
	Quorum        int      `json:"quorum"`
	Threshold     int      `json:"threshold"`
}
type Input struct {
	Title              string          `json:"title"`
	Purpose            string          `json:"purpose"`
	Roles              []Role          `json:"roles"`
	DecisionClasses    []DecisionClass `json:"decision_classes"`
	ParticipationRules []string        `json:"participation_rules"`
	ProtectedResources []string        `json:"protected_resources"`
	Procedures         Procedures      `json:"procedures"`
	AmendmentPolicy    AmendmentPolicy `json:"amendment_policy"`
	ChangeReason       string          `json:"change_reason"`
}
type PreviewItem struct {
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
}
type Preview struct {
	GeneratedAt      time.Time     `json:"generated_at"`
	Items            []PreviewItem `json:"items"`
	Blockers         []string      `json:"blockers"`
	AuthorityGranted bool          `json:"authority_granted"`
}
type Approval struct {
	ActorID   string    `json:"actor_id"`
	Version   int64     `json:"version"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Exception struct {
	ID        string     `json:"id"`
	Version   int64      `json:"version"`
	Scope     string     `json:"scope"`
	Reason    string     `json:"reason"`
	ExpiresAt time.Time  `json:"expires_at"`
	ActorID   string     `json:"actor_id"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
type Revision struct {
	Version int64 `json:"version"`
	Input
	State         string     `json:"state"`
	Preview       Preview    `json:"preview"`
	AuthorID      string     `json:"author_id"`
	CreatedAt     time.Time  `json:"created_at"`
	ActivatedByID string     `json:"activated_by_id,omitempty"`
	ActivatedAt   *time.Time `json:"activated_at,omitempty"`
}
type Charter struct {
	ID            string      `json:"id"`
	ScopeType     string      `json:"scope_type"`
	ScopeID       string      `json:"scope_id"`
	ActiveVersion int64       `json:"active_version,omitempty"`
	Current       Revision    `json:"current"`
	History       []Revision  `json:"history"`
	Approvals     []Approval  `json:"approvals"`
	Exceptions    []Exception `json:"exceptions"`
	UpdatedAt     time.Time   `json:"updated_at"`
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
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string                             { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) path(t, scope string) string { return filepath.Join(s.root, t, scope+".json") }
func (s *Store) read(t, scope string) (Charter, error) {
	b, e := os.ReadFile(s.path(t, scope))
	if errors.Is(e, os.ErrNotExist) {
		return Charter{}, ErrNotFound
	}
	if e != nil {
		return Charter{}, e
	}
	var v Charter
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Charter) error {
	p := s.path(v.ScopeType, v.ScopeID)
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func clean(v string) bool { return strings.TrimSpace(v) != "" && len(v) <= 2000 }
func validate(in Input) error {
	if !clean(in.Title) || !clean(in.Purpose) || !clean(in.ChangeReason) || len(in.Roles) == 0 || len(in.DecisionClasses) == 0 || len(in.ProtectedResources) == 0 || !clean(in.Procedures.Removal) || !clean(in.Procedures.Succession) || !clean(in.Procedures.Vacancy) || len(in.AmendmentPolicy.EligibleRoles) == 0 || in.AmendmentPolicy.Quorum < 1 || in.AmendmentPolicy.Threshold < 1 || in.AmendmentPolicy.Threshold > 100 {
		return ErrInvalid
	}
	roles := map[string]bool{}
	for _, r := range in.Roles {
		if !clean(r.Name) || !clean(r.Purpose) || len(r.Eligibility) == 0 || len(r.Responsibilities) == 0 || r.MinimumMembers < 1 {
			return ErrInvalid
		}
		roles[r.Name] = true
	}
	for _, d := range in.DecisionClasses {
		if !clean(d.Name) || len(d.EligibleRoles) == 0 || d.Quorum < 1 || d.Threshold < 1 || d.Threshold > 100 || len(d.ProtectedResources) == 0 {
			return ErrInvalid
		}
		for _, r := range d.EligibleRoles {
			if !roles[r] {
				return ErrInvalid
			}
		}
	}
	for _, r := range in.AmendmentPolicy.EligibleRoles {
		if !roles[r] {
			return ErrInvalid
		}
	}
	return nil
}
func (s *Store) Publish(t, scope, actor string, expected int64, in Input, p Preview) (Charter, error) {
	if (t != "repository" && t != "organization") || !clean(scope) || !clean(actor) || validate(in) != nil {
		return Charter{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if errors.Is(e, ErrNotFound) {
		if expected != 0 {
			return Charter{}, ErrConflict
		}
		v = Charter{ID: id(), ScopeType: t, ScopeID: scope}
	} else if e != nil {
		return Charter{}, e
	} else if v.Current.Version != expected {
		return Charter{}, ErrConflict
	}
	now := s.now().UTC()
	if v.Current.Version > 0 {
		v.History = append(v.History, v.Current)
	}
	v.Current = Revision{Version: expected + 1, Input: in, State: "draft", Preview: p, AuthorID: actor, CreatedAt: now}
	v.UpdatedAt = now
	if e = s.write(v); e != nil {
		return Charter{}, e
	}
	return v, nil
}
func (s *Store) Get(t, scope string) (Charter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(t, scope)
}
func (s *Store) Approve(t, scope, actor, note string, version int64) (Charter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	if version != v.Current.Version {
		return v, ErrConflict
	}
	for _, a := range v.Approvals {
		if a.Version == version && a.ActorID == actor {
			return v, nil
		}
	}
	v.Approvals = append(v.Approvals, Approval{ActorID: actor, Version: version, Note: strings.TrimSpace(note), CreatedAt: s.now().UTC()})
	e = s.write(v)
	return v, e
}
func (s *Store) Activate(t, scope, actor string, version int64, p Preview) (Charter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	if version != v.Current.Version {
		return v, ErrConflict
	}
	approved := false
	for _, a := range v.Approvals {
		approved = approved || (a.Version == version)
	}
	if !approved || len(p.Blockers) > 0 {
		return v, ErrConflict
	}
	now := s.now().UTC()
	v.Current.State = "active"
	v.Current.Preview = p
	v.Current.ActivatedByID = actor
	v.Current.ActivatedAt = &now
	v.ActiveVersion = version
	v.UpdatedAt = now
	e = s.write(v)
	return v, e
}
func (s *Store) Except(t, scope, actor string, version int64, x Exception) (Charter, error) {
	if !clean(x.Scope) || !clean(x.Reason) || !x.ExpiresAt.After(s.now()) {
		return Charter{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	if version < 1 || version > v.Current.Version {
		return v, ErrConflict
	}
	x.ID = id()
	x.Version = version
	x.ActorID = actor
	x.CreatedAt = s.now().UTC()
	v.Exceptions = append(v.Exceptions, x)
	e = s.write(v)
	return v, e
}
func (s *Store) List(t string) ([]Charter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, t))
	if errors.Is(e, os.ErrNotExist) {
		return []Charter{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Charter{}
	for _, x := range entries {
		if x.IsDir() {
			continue
		}
		v, e := s.read(t, strings.TrimSuffix(x.Name(), ".json"))
		if e == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
