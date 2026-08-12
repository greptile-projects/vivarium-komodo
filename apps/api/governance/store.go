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
type Evidence struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Summary    string `json:"summary"`
	VerifiedBy string `json:"verified_by,omitempty"`
}
type StandingEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Standing struct {
	ID                   string          `json:"id"`
	PrincipalID          string          `json:"principal_id"`
	Role                 string          `json:"role"`
	CharterVersion       int64           `json:"charter_version"`
	State                string          `json:"state"`
	Responsibilities     []string        `json:"responsibilities"`
	Evidence             []Evidence      `json:"evidence"`
	Nominations          []string        `json:"available_nominations"`
	Appeals              []string        `json:"available_appeals"`
	ConflictDisclosure   string          `json:"conflict_of_interest,omitempty"`
	OperationalAuthority []string        `json:"operational_authority"`
	InvitedByID          string          `json:"invited_by_id"`
	InvitedAt            time.Time       `json:"invited_at"`
	AcceptedAt           *time.Time      `json:"accepted_at,omitempty"`
	TermStartsAt         *time.Time      `json:"term_starts_at,omitempty"`
	TermEndsAt           *time.Time      `json:"term_ends_at,omitempty"`
	Events               []StandingEvent `json:"events"`
}
type StandingInput struct {
	PrincipalID        string     `json:"principal_id"`
	Role               string     `json:"role"`
	Evidence           []Evidence `json:"evidence"`
	Nominations        []string   `json:"available_nominations"`
	Appeals            []string   `json:"available_appeals"`
	ConflictDisclosure string     `json:"conflict_of_interest,omitempty"`
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
	ID            string            `json:"id"`
	ScopeType     string            `json:"scope_type"`
	ScopeID       string            `json:"scope_id"`
	ActiveVersion int64             `json:"active_version,omitempty"`
	Current       Revision          `json:"current"`
	History       []Revision        `json:"history"`
	Approvals     []Approval        `json:"approvals"`
	Exceptions    []Exception       `json:"exceptions"`
	Standings     []Standing        `json:"standings"`
	Stewardship   []StewardshipCase `json:"stewardship"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

var evidenceKinds = map[string]bool{"contribution": true, "review": true, "support": true, "ownership": true, "membership": true}

func roleFor(v Charter, name string) (Role, bool) {
	for _, r := range v.Current.Roles {
		if r.Name == name {
			return r, true
		}
	}
	return Role{}, false
}

// Invite records charter-bounded eligibility. Governance standing deliberately
// carries an empty operational-authority set; repository policy remains separate.
func (s *Store) Invite(t, scope, actor string, version int64, in StandingInput) (Charter, error) {
	if !clean(in.PrincipalID) || !clean(in.Role) || len(in.Evidence) == 0 {
		return Charter{}, ErrInvalid
	}
	for _, e := range in.Evidence {
		if !evidenceKinds[e.Kind] || !clean(e.Reference) || !clean(e.Summary) {
			return Charter{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	role, ok := roleFor(v, in.Role)
	if !ok || version != v.ActiveVersion || v.Current.State != "active" {
		return v, ErrConflict
	}
	for _, x := range v.Standings {
		if x.PrincipalID == in.PrincipalID && x.Role == in.Role && x.State != "expired" && x.State != "revoked" {
			return v, ErrConflict
		}
	}
	now := s.now().UTC()
	evidence := append([]Evidence(nil), in.Evidence...)
	for i := range evidence {
		evidence[i].VerifiedBy = actor
	}
	x := Standing{ID: id(), PrincipalID: in.PrincipalID, Role: in.Role, CharterVersion: version, State: "invited", Responsibilities: append([]string(nil), role.Responsibilities...), Evidence: evidence, Nominations: in.Nominations, Appeals: in.Appeals, ConflictDisclosure: strings.TrimSpace(in.ConflictDisclosure), OperationalAuthority: []string{}, InvitedByID: actor, InvitedAt: now}
	x.Events = append(x.Events, StandingEvent{Sequence: 1, Type: "invited", ActorID: actor, CreatedAt: now})
	v.Standings = append(v.Standings, x)
	v.UpdatedAt = now
	e = s.write(v)
	return v, e
}

func (s *Store) Transition(t, scope, standingID, actor, action, reason string) (Charter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(t, scope)
	if e != nil {
		return v, e
	}
	for i := range v.Standings {
		x := &v.Standings[i]
		if x.ID != standingID {
			continue
		}
		now := s.now().UTC()
		switch action {
		case "accept":
			if actor != x.PrincipalID || x.State != "invited" {
				return v, ErrConflict
			}
			role, ok := roleFor(v, x.Role)
			if !ok {
				return v, ErrConflict
			}
			x.State = "active"
			x.AcceptedAt = &now
			x.TermStartsAt = &now
			if role.TermDays > 0 {
				end := now.Add(time.Duration(role.TermDays) * 24 * time.Hour)
				x.TermEndsAt = &end
			}
		case "recuse":
			if actor != x.PrincipalID || x.State != "active" {
				return v, ErrConflict
			}
			x.State = "recused"
		case "resume":
			if actor != x.PrincipalID || x.State != "recused" {
				return v, ErrConflict
			}
			x.State = "active"
		case "suspend", "revoke_identity", "revoke_federation_trust":
			if !clean(reason) {
				return v, ErrInvalid
			}
			x.State = map[string]string{"suspend": "suspended", "revoke_identity": "revoked_identity", "revoke_federation_trust": "revoked_federation_trust"}[action]
		case "expire":
			if x.TermEndsAt == nil || now.Before(*x.TermEndsAt) {
				return v, ErrConflict
			}
			x.State = "expired"
		default:
			return v, ErrInvalid
		}
		x.Events = append(x.Events, StandingEvent{Sequence: int64(len(x.Events) + 1), Type: action, ActorID: actor, Reason: strings.TrimSpace(reason), CreatedAt: now})
		v.UpdatedAt = now
		e = s.write(v)
		return v, e
	}
	return v, ErrNotFound
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
