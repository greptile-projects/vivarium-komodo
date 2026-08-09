// Package organizations owns durable group identity, membership, and
// ownership-transfer acceptance records.
package organizations

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("organization not found")
	ErrInvalid   = errors.New("invalid organization")
	ErrConflict  = errors.New("organization conflict")
	ErrForbidden = errors.New("organization action forbidden")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)

type Member struct {
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	InvitedByID string    `json:"invited_by_id,omitempty"`
	InvitedAt   time.Time `json:"invited_at,omitempty"`
	AcceptedAt  time.Time `json:"accepted_at,omitempty"`
}
type TeamMember struct {
	UserID      string    `json:"user_id"`
	Role        string    `json:"role"`
	InvitedByID string    `json:"invited_by_id"`
	InvitedAt   time.Time `json:"invited_at"`
	AcceptedAt  time.Time `json:"accepted_at,omitempty"`
}
type Responsibility struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Area         string    `json:"area"`
	Description  string    `json:"description,omitempty"`
	Visibility   string    `json:"visibility"`
	CreatedByID  string    `json:"created_by_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Team struct {
	ID               string           `json:"id"`
	Slug             string           `json:"slug"`
	Name             string           `json:"name"`
	Description      string           `json:"description,omitempty"`
	ParentID         string           `json:"parent_id,omitempty"`
	Visibility       string           `json:"visibility"`
	Version          int64            `json:"version"`
	Members          []TeamMember     `json:"members"`
	Responsibilities []Responsibility `json:"responsibilities"`
	CreatedByID      string           `json:"created_by_id"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}
type Agent struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Capabilities []string  `json:"capabilities"`
	OperatorIDs  []string  `json:"operator_ids"`
	Visibility   string    `json:"visibility"`
	Version      int64     `json:"version"`
	CreatedByID  string    `json:"created_by_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ResourceRef names one authority boundary without implying authority over the
// rest of the organization portfolio. RepositoryID is retained for nested
// resources so effective access can always explain its ownership boundary.
type ResourceRef struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id,omitempty"`
}
type RoleGrant struct {
	ID              string            `json:"id"`
	PrincipalKind   string            `json:"principal_kind"`
	PrincipalID     string            `json:"principal_id"`
	Role            string            `json:"role"`
	Resources       []ResourceRef     `json:"resources"`
	Exceptions      []string          `json:"exceptions"`
	Reason          string            `json:"reason"`
	CreatedByID     string            `json:"created_by_id"`
	CreatedAt       time.Time         `json:"created_at"`
	ExpiresAt       time.Time         `json:"expires_at"`
	RevokedAt       *time.Time        `json:"revoked_at,omitempty"`
	RevokedByID     string            `json:"revoked_by_id,omitempty"`
	CredentialIDs   []string          `json:"credential_ids,omitempty"`
	CredentialUsers map[string]string `json:"credential_users,omitempty"`
}
type AccessRequest struct {
	ID            string        `json:"id"`
	RequestedByID string        `json:"requested_by_id"`
	PrincipalKind string        `json:"principal_kind"`
	PrincipalID   string        `json:"principal_id"`
	Role          string        `json:"role"`
	Resources     []ResourceRef `json:"resources"`
	Exceptions    []string      `json:"exceptions"`
	Reason        string        `json:"reason"`
	ExpiresAt     time.Time     `json:"expires_at"`
	State         string        `json:"state"`
	CreatedAt     time.Time     `json:"created_at"`
	ResolvedAt    *time.Time    `json:"resolved_at,omitempty"`
	ResolvedByID  string        `json:"resolved_by_id,omitempty"`
	GrantID       string        `json:"grant_id,omitempty"`
}
type Transfer struct {
	ID            string     `json:"id"`
	RepositoryID  string     `json:"repository_id"`
	FromKind      string     `json:"from_kind"`
	FromID        string     `json:"from_id"`
	ToKind        string     `json:"to_kind"`
	ToID          string     `json:"to_id"`
	RequestedByID string     `json:"requested_by_id"`
	State         string     `json:"state"`
	CreatedAt     time.Time  `json:"created_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	ResolvedByID  string     `json:"resolved_by_id,omitempty"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PolicyRule is one independently explainable organization requirement. Config
// remains domain-specific while Enforcement gives clients a stable way to
// distinguish mandatory requirements from advisory guidance.
type PolicyRule struct {
	ID          string          `json:"id"`
	Domain      string          `json:"domain"`
	Enforcement string          `json:"enforcement"`
	Config      json.RawMessage `json:"config"`
}
type PolicyTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type PolicyVersion struct {
	ID            string         `json:"id"`
	Version       int64          `json:"version"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	State         string         `json:"state"`
	Rules         []PolicyRule   `json:"rules"`
	Targets       []PolicyTarget `json:"targets"`
	CreatedByID   string         `json:"created_by_id"`
	CreatedAt     time.Time      `json:"created_at"`
	ActivatedByID string         `json:"activated_by_id,omitempty"`
	ActivatedAt   *time.Time     `json:"activated_at,omitempty"`
}
type PolicyException struct {
	ID            string     `json:"id"`
	PolicyID      string     `json:"policy_id"`
	PolicyVersion int64      `json:"policy_version"`
	RuleID        string     `json:"rule_id"`
	RepositoryID  string     `json:"repository_id"`
	Reason        string     `json:"reason"`
	RequestedByID string     `json:"requested_by_id"`
	RequestedAt   time.Time  `json:"requested_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	State         string     `json:"state"`
	ResolvedByID  string     `json:"resolved_by_id,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}
type EffectiveRule struct {
	PolicyID      string           `json:"policy_id"`
	PolicyVersion int64            `json:"policy_version"`
	PolicyName    string           `json:"policy_name"`
	Target        PolicyTarget     `json:"target"`
	Rule          PolicyRule       `json:"rule"`
	Exception     *PolicyException `json:"exception,omitempty"`
}

// Initiative coordinates an outcome without copying the source records that
// explain why the work exists. Items may span organization repositories while
// retaining their own accountable principal and delivery evidence.
type Initiative struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Outcome     string           `json:"outcome"`
	State       string           `json:"state"`
	Sources     []ResourceRef    `json:"sources"`
	Items       []InitiativeItem `json:"items"`
	CreatedByID string           `json:"created_by_id"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}
type InitiativeItem struct {
	ID                 string        `json:"id"`
	Title              string        `json:"title"`
	Outcome            string        `json:"outcome"`
	Position           int           `json:"position"`
	State              string        `json:"state"`
	RepositoryID       string        `json:"repository_id"`
	Source             ResourceRef   `json:"source"`
	DependsOn          []string      `json:"depends_on"`
	AssigneeKind       string        `json:"assignee_kind"`
	AssigneeID         string        `json:"assignee_id"`
	Contributions      []ResourceRef `json:"contributions"`
	UpcomingReleaseIDs []string      `json:"upcoming_release_ids"`
	PolicyExceptionIDs []string      `json:"policy_exception_ids"`
	BlockedBy          []string      `json:"blocked_by"`
	NeedsReassignment  bool          `json:"needs_reassignment"`
	NextDecision       string        `json:"next_decision,omitempty"`
	UpdatedByID        string        `json:"updated_by_id"`
	UpdatedAt          time.Time     `json:"updated_at"`
}
type Organization struct {
	ID               string            `json:"id"`
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Members          []Member          `json:"members"`
	Transfers        []Transfer        `json:"transfers"`
	Events           []Event           `json:"events"`
	Teams            []Team            `json:"teams"`
	Agents           []Agent           `json:"agents"`
	RoleGrants       []RoleGrant       `json:"role_grants"`
	AccessRequests   []AccessRequest   `json:"access_requests"`
	Policies         []PolicyVersion   `json:"policies"`
	PolicyExceptions []PolicyException `json:"policy_exceptions"`
	Initiatives      []Initiative      `json:"initiatives"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
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
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}

func (s *Store) Create(actor, slug, name, description string) (Organization, error) {
	slug, name, description = strings.ToLower(strings.TrimSpace(slug)), strings.TrimSpace(name), strings.TrimSpace(description)
	if actor == "" || !slugPattern.MatchString(slug) || name == "" || len(name) > 100 || len(description) > 1000 {
		return Organization{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.list()
	if err != nil {
		return Organization{}, err
	}
	for _, x := range items {
		if x.Slug == slug {
			return Organization{}, ErrConflict
		}
	}
	id, err := newID()
	if err != nil {
		return Organization{}, err
	}
	now := s.now().UTC()
	o := Organization{ID: id, Slug: slug, Name: name, Description: description, Members: []Member{{UserID: actor, Role: "owner", AcceptedAt: now}}, Transfers: []Transfer{}, Teams: []Team{}, Agents: []Agent{}, RoleGrants: []RoleGrant{}, AccessRequests: []AccessRequest{}, Policies: []PolicyVersion{}, PolicyExceptions: []PolicyException{}, Initiatives: []Initiative{}, Events: []Event{{Sequence: 1, Type: "organization.created", ActorID: actor, CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
	return o, s.write(o)
}
func (s *Store) Get(id string) (Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}
func (s *Store) ListFor(user string) ([]Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return nil, e
	}
	out := []Organization{}
	for _, o := range all {
		if _, ok := membership(o, user, false); ok {
			out = append(out, o)
		}
	}
	return out, nil
}
func (s *Store) IsMember(id, user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(id)
	if e != nil {
		return false
	}
	_, ok := membership(o, user, true)
	return ok
}
func (s *Store) IsOwner(id, user string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(id)
	if e != nil {
		return false
	}
	m, ok := membership(o, user, true)
	return ok && m.Role == "owner"
}
func (s *Store) Invite(id, actor, user string) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		m, ok := membership(*o, actor, true)
		if !ok || m.Role != "owner" {
			return ErrForbidden
		}
		if user == "" || user == actor {
			return ErrInvalid
		}
		if _, exists := membership(*o, user, false); exists {
			return ErrConflict
		}
		now := s.now().UTC()
		o.Members = append(o.Members, Member{UserID: user, Role: "member", InvitedByID: actor, InvitedAt: now})
		event(o, "member.invited", actor, user, now)
		return nil
	})
}
func (s *Store) Accept(id, user string) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		for i := range o.Members {
			if o.Members[i].UserID == user && o.Members[i].AcceptedAt.IsZero() {
				now := s.now().UTC()
				o.Members[i].AcceptedAt = now
				event(o, "member.accepted", user, user, now)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) Remove(id, actor, user string) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		a, ok := membership(*o, actor, true)
		if !ok || a.Role != "owner" {
			return ErrForbidden
		}
		for i, m := range o.Members {
			if m.UserID == user {
				if m.Role == "owner" {
					return ErrConflict
				}
				o.Members = append(o.Members[:i], o.Members[i+1:]...)
				for ti := range o.Teams {
					for mi := len(o.Teams[ti].Members) - 1; mi >= 0; mi-- {
						if o.Teams[ti].Members[mi].UserID == user {
							o.Teams[ti].Members = append(o.Teams[ti].Members[:mi], o.Teams[ti].Members[mi+1:]...)
							o.Teams[ti].Version++
						}
					}
				}
				for ai := range o.Agents {
					filtered := o.Agents[ai].OperatorIDs[:0]
					for _, x := range o.Agents[ai].OperatorIDs {
						if x != user {
							filtered = append(filtered, x)
						}
					}
					if len(filtered) != len(o.Agents[ai].OperatorIDs) {
						o.Agents[ai].OperatorIDs = filtered
						o.Agents[ai].Version++
					}
				}
				event(o, "member.removed", actor, user, s.now().UTC())
				return nil
			}
		}
		return ErrNotFound
	})
}

func owner(o Organization, actor string) bool {
	m, ok := membership(o, actor, true)
	return ok && m.Role == "owner"
}
func validVisibility(v string) bool { return v == "public" || v == "internal" }
func findTeam(o *Organization, id string) (*Team, bool) {
	for i := range o.Teams {
		if o.Teams[i].ID == id {
			return &o.Teams[i], true
		}
	}
	return nil, false
}
func teamMaintainer(o Organization, team, user string) bool {
	for team != "" {
		t, ok := teamByID(o, team)
		if !ok {
			return false
		}
		if m, yes := teamMember(t, user, true); yes && m.Role == "maintainer" {
			return true
		}
		team = t.ParentID
	}
	return false
}

func (s *Store) CreateTeam(id, actor string, in Team) (Organization, Team, error) {
	var made Team
	o, e := s.change(id, func(o *Organization) error {
		if !owner(*o, actor) {
			return ErrForbidden
		}
		in.Slug, in.Name, in.Description = strings.ToLower(strings.TrimSpace(in.Slug)), strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
		if !slugPattern.MatchString(in.Slug) || in.Name == "" || len(in.Name) > 100 || !validVisibility(in.Visibility) {
			return ErrInvalid
		}
		for _, t := range o.Teams {
			if t.Slug == in.Slug {
				return ErrConflict
			}
		}
		if in.ParentID != "" {
			if _, ok := findTeam(o, in.ParentID); !ok {
				return ErrInvalid
			}
		}
		uid, er := newID()
		if er != nil {
			return er
		}
		now := s.now().UTC()
		in.ID, in.Version, in.CreatedByID, in.CreatedAt, in.UpdatedAt = uid, 1, actor, now, now
		in.Members, in.Responsibilities = []TeamMember{}, []Responsibility{}
		o.Teams = append(o.Teams, in)
		made = in
		event(o, "team.created", actor, uid, now)
		return nil
	})
	return o, made, e
}
func (s *Store) InviteTeamMember(id, team, actor, user, role string, version int64) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		t, ok := findTeam(o, team)
		if !ok {
			return ErrNotFound
		}
		if t.Version != version {
			return ErrConflict
		}
		if !owner(*o, actor) && !teamMaintainer(*o, team, actor) {
			return ErrForbidden
		}
		if _, yes := membership(*o, user, true); !yes {
			return ErrInvalid
		}
		if role != "member" && role != "maintainer" {
			return ErrInvalid
		}
		if _, yes := teamMember(*t, user, false); yes {
			return ErrConflict
		}
		now := s.now().UTC()
		t.Members = append(t.Members, TeamMember{UserID: user, Role: role, InvitedByID: actor, InvitedAt: now})
		t.Version++
		t.UpdatedAt = now
		event(o, "team.member_invited", actor, user, now)
		return nil
	})
}
func teamMember(t Team, user string, accepted bool) (TeamMember, bool) {
	for _, m := range t.Members {
		if m.UserID == user && (!accepted || !m.AcceptedAt.IsZero()) {
			return m, true
		}
	}
	return TeamMember{}, false
}
func (s *Store) AcceptTeam(id, team, user string, version int64) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		t, ok := findTeam(o, team)
		if !ok {
			return ErrNotFound
		}
		if t.Version != version {
			return ErrConflict
		}
		for i := range t.Members {
			if t.Members[i].UserID == user && t.Members[i].AcceptedAt.IsZero() {
				now := s.now().UTC()
				t.Members[i].AcceptedAt = now
				t.Version++
				t.UpdatedAt = now
				event(o, "team.member_accepted", user, user, now)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) RemoveTeamMember(id, team, actor, user string, version int64) (Organization, error) {
	return s.change(id, func(o *Organization) error {
		t, ok := findTeam(o, team)
		if !ok {
			return ErrNotFound
		}
		if t.Version != version {
			return ErrConflict
		}
		if !owner(*o, actor) && !teamMaintainer(*o, team, actor) {
			return ErrForbidden
		}
		for i, m := range t.Members {
			if m.UserID == user {
				t.Members = append(t.Members[:i], t.Members[i+1:]...)
				t.Version++
				t.UpdatedAt = s.now().UTC()
				event(o, "team.member_removed", actor, user, t.UpdatedAt)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) AddResponsibility(id, team, actor string, in Responsibility, version int64) (Organization, Responsibility, error) {
	var made Responsibility
	o, e := s.change(id, func(o *Organization) error {
		t, ok := findTeam(o, team)
		if !ok {
			return ErrNotFound
		}
		if t.Version != version {
			return ErrConflict
		}
		if !owner(*o, actor) && !teamMaintainer(*o, team, actor) {
			return ErrForbidden
		}
		in.Area = strings.TrimSpace(in.Area)
		if in.RepositoryID == "" || in.Area == "" || !validVisibility(in.Visibility) {
			return ErrInvalid
		}
		uid, er := newID()
		if er != nil {
			return er
		}
		now := s.now().UTC()
		in.ID, in.CreatedByID, in.CreatedAt = uid, actor, now
		t.Responsibilities = append(t.Responsibilities, in)
		t.Version++
		t.UpdatedAt = now
		made = in
		event(o, "team.responsibility_added", actor, uid, now)
		return nil
	})
	return o, made, e
}
func (s *Store) RegisterAgent(id, actor string, in Agent) (Organization, Agent, error) {
	var made Agent
	o, e := s.change(id, func(o *Organization) error {
		if !owner(*o, actor) {
			return ErrForbidden
		}
		in.Slug, in.Name = strings.ToLower(strings.TrimSpace(in.Slug)), strings.TrimSpace(in.Name)
		if !slugPattern.MatchString(in.Slug) || in.Name == "" || !validVisibility(in.Visibility) || len(in.Capabilities) == 0 || len(in.OperatorIDs) == 0 {
			return ErrInvalid
		}
		for _, a := range o.Agents {
			if a.Slug == in.Slug {
				return ErrConflict
			}
		}
		for _, u := range in.OperatorIDs {
			if _, ok := membership(*o, u, true); !ok {
				return ErrInvalid
			}
		}
		uid, er := newID()
		if er != nil {
			return er
		}
		now := s.now().UTC()
		in.ID, in.Version, in.CreatedByID, in.CreatedAt, in.UpdatedAt = uid, 1, actor, now, now
		o.Agents = append(o.Agents, in)
		made = in
		event(o, "agent.registered", actor, uid, now)
		return nil
	})
	return o, made, e
}

func validRole(role string) bool {
	return role == "viewer" || role == "contributor" || role == "maintainer" || role == "operator"
}
func validResources(resources []ResourceRef) bool {
	if len(resources) == 0 || len(resources) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, r := range resources {
		if r.ID == "" || (r.Kind != "repository" && r.Kind != "package" && r.Kind != "environment" && r.Kind != "collaboration") {
			return false
		}
		if r.Kind != "repository" && r.RepositoryID == "" {
			return false
		}
		key := r.Kind + "\x00" + r.ID
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}
func validPrincipal(o Organization, kind, id string) bool {
	if kind == "team" {
		_, ok := teamByID(o, id)
		return ok
	}
	if kind == "agent" {
		for _, a := range o.Agents {
			if a.ID == id {
				return true
			}
		}
	}
	return false
}
func normalizeExceptions(in []string) ([]string, bool) {
	if len(in) > 50 {
		return nil, false
	}
	out, seen := []string{}, map[string]bool{}
	for _, x := range in {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > 100 || seen[x] {
			return nil, false
		}
		seen[x] = true
		out = append(out, x)
	}
	sort.Strings(out)
	return out, true
}
func validateAuthority(o Organization, principalKind, principalID, role, reason string, resources []ResourceRef, exceptions []string, expires time.Time, now time.Time) ([]string, error) {
	clean, ok := normalizeExceptions(exceptions)
	if !validPrincipal(o, principalKind, principalID) || !validRole(role) || !validResources(resources) || strings.TrimSpace(reason) == "" || len(reason) > 1000 || !ok || !expires.After(now) || expires.After(now.Add(30*24*time.Hour)) {
		return nil, ErrInvalid
	}
	return clean, nil
}

func (s *Store) GrantRole(id, actor string, in RoleGrant) (Organization, RoleGrant, error) {
	var made RoleGrant
	o, e := s.change(id, func(o *Organization) error {
		if !owner(*o, actor) {
			return ErrForbidden
		}
		now := s.now().UTC()
		clean, err := validateAuthority(*o, in.PrincipalKind, in.PrincipalID, in.Role, in.Reason, in.Resources, in.Exceptions, in.ExpiresAt, now)
		if err != nil {
			return err
		}
		uid, err := newID()
		if err != nil {
			return err
		}
		in.ID, in.CreatedByID, in.CreatedAt, in.Exceptions, in.Reason = uid, actor, now, clean, strings.TrimSpace(in.Reason)
		in.CredentialIDs, in.CredentialUsers = []string{}, map[string]string{}
		o.RoleGrants = append(o.RoleGrants, in)
		made = in
		event(o, "access.granted", actor, uid, now)
		return nil
	})
	return o, made, e
}
func (s *Store) RequestAccess(id, actor string, in AccessRequest) (Organization, AccessRequest, error) {
	var made AccessRequest
	o, e := s.change(id, func(o *Organization) error {
		if _, ok := membership(*o, actor, true); !ok {
			return ErrForbidden
		}
		now := s.now().UTC()
		clean, err := validateAuthority(*o, in.PrincipalKind, in.PrincipalID, in.Role, in.Reason, in.Resources, in.Exceptions, in.ExpiresAt, now)
		if err != nil {
			return err
		}
		if in.PrincipalKind == "team" && !userInTeam(*o, in.PrincipalID, actor) {
			return ErrForbidden
		}
		if in.PrincipalKind == "agent" && !agentOperator(*o, in.PrincipalID, actor) {
			return ErrForbidden
		}
		uid, err := newID()
		if err != nil {
			return err
		}
		in.ID, in.RequestedByID, in.CreatedAt, in.State, in.Exceptions, in.Reason = uid, actor, now, "pending", clean, strings.TrimSpace(in.Reason)
		o.AccessRequests = append(o.AccessRequests, in)
		made = in
		event(o, "access.requested", actor, uid, now)
		return nil
	})
	return o, made, e
}
func (s *Store) ResolveAccessRequest(id, request, actor, decision string) (Organization, AccessRequest, RoleGrant, error) {
	var resolved AccessRequest
	var grant RoleGrant
	o, e := s.change(id, func(o *Organization) error {
		if !owner(*o, actor) {
			return ErrForbidden
		}
		if decision != "approved" && decision != "denied" {
			return ErrInvalid
		}
		for i := range o.AccessRequests {
			x := &o.AccessRequests[i]
			if x.ID != request {
				continue
			}
			if x.State != "pending" {
				return ErrConflict
			}
			now := s.now().UTC()
			if !x.ExpiresAt.After(now) {
				return ErrConflict
			}
			x.State, x.ResolvedByID, x.ResolvedAt = decision, actor, &now
			if decision == "approved" {
				uid, err := newID()
				if err != nil {
					return err
				}
				grant = RoleGrant{ID: uid, PrincipalKind: x.PrincipalKind, PrincipalID: x.PrincipalID, Role: x.Role, Resources: append([]ResourceRef(nil), x.Resources...), Exceptions: append([]string(nil), x.Exceptions...), Reason: x.Reason, CreatedByID: actor, CreatedAt: now, ExpiresAt: x.ExpiresAt, CredentialIDs: []string{}, CredentialUsers: map[string]string{}}
				o.RoleGrants = append(o.RoleGrants, grant)
				x.GrantID = uid
			}
			resolved = *x
			event(o, "access.request_"+decision, actor, x.ID, now)
			return nil
		}
		return ErrNotFound
	})
	return o, resolved, grant, e
}
func (s *Store) RevokeRole(id, grant, actor string) (Organization, RoleGrant, error) {
	var revoked RoleGrant
	o, e := s.change(id, func(o *Organization) error {
		if !owner(*o, actor) {
			return ErrForbidden
		}
		for i := range o.RoleGrants {
			x := &o.RoleGrants[i]
			if x.ID != grant {
				continue
			}
			if x.RevokedAt == nil {
				now := s.now().UTC()
				x.RevokedAt, x.RevokedByID = &now, actor
				event(o, "access.revoked", actor, x.ID, now)
			}
			revoked = *x
			return nil
		}
		return ErrNotFound
	})
	return o, revoked, e
}
func (s *Store) AttachCredential(id, grant, actor, credential string) (Organization, RoleGrant, error) {
	var updated RoleGrant
	o, e := s.change(id, func(o *Organization) error {
		for i := range o.RoleGrants {
			x := &o.RoleGrants[i]
			if x.ID != grant {
				continue
			}
			if x.RevokedAt != nil || !s.now().Before(x.ExpiresAt) || !principalIncludes(*o, *x, actor) {
				return ErrForbidden
			}
			for _, c := range x.CredentialIDs {
				if c == credential {
					return ErrConflict
				}
			}
			x.CredentialIDs = append(x.CredentialIDs, credential)
			if x.CredentialUsers == nil {
				x.CredentialUsers = map[string]string{}
			}
			x.CredentialUsers[credential] = actor
			updated = *x
			event(o, "access.credential_issued", actor, credential, s.now().UTC())
			return nil
		}
		return ErrNotFound
	})
	return o, updated, e
}
func userInTeam(o Organization, team, user string) bool {
	t, ok := teamByID(o, team)
	if !ok {
		return false
	}
	if _, ok := teamMember(t, user, true); ok {
		return true
	}
	for _, child := range o.Teams {
		if child.ParentID == team && userInTeam(o, child.ID, user) {
			return true
		}
	}
	return false
}
func agentOperator(o Organization, agent, user string) bool {
	for _, a := range o.Agents {
		if a.ID == agent {
			for _, u := range a.OperatorIDs {
				if u == user {
					return true
				}
			}
		}
	}
	return false
}
func principalIncludes(o Organization, grant RoleGrant, user string) bool {
	if grant.PrincipalKind == "team" {
		return userInTeam(o, grant.PrincipalID, user)
	}
	return agentOperator(o, grant.PrincipalID, user)
}
func (s *Store) EffectiveAccess(id, user string) ([]RoleGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(id)
	if e != nil {
		return nil, e
	}
	now := s.now()
	out := []RoleGrant{}
	for _, g := range o.RoleGrants {
		if g.RevokedAt == nil && now.Before(g.ExpiresAt) && principalIncludes(o, g, user) {
			out = append(out, g)
		}
	}
	return out, nil
}

type EffectiveMember struct {
	UserID     string   `json:"user_id"`
	Role       string   `json:"role"`
	ViaTeamIDs []string `json:"via_team_ids"`
}
type Directory struct {
	Teams            []Team                       `json:"teams"`
	Agents           []Agent                      `json:"agents"`
	EffectiveMembers map[string][]EffectiveMember `json:"effective_members"`
}

func DirectoryFor(o Organization, internal bool) Directory {
	d := Directory{Teams: []Team{}, Agents: []Agent{}, EffectiveMembers: map[string][]EffectiveMember{}}
	visibleTeams := map[string]bool{}
	for _, t := range o.Teams {
		if internal || t.Visibility == "public" {
			visibleTeams[t.ID] = true
			copy := t
			if !internal {
				copy.Members = []TeamMember{}
				copy.Responsibilities = []Responsibility{}
				for _, r := range t.Responsibilities {
					if r.Visibility == "public" {
						copy.Responsibilities = append(copy.Responsibilities, r)
					}
				}
			}
			d.Teams = append(d.Teams, copy)
		}
	}
	for _, a := range o.Agents {
		if internal || a.Visibility == "public" {
			d.Agents = append(d.Agents, a)
		}
	}
	for _, t := range d.Teams {
		seen := map[string]EffectiveMember{}
		var walk func(string, []string)
		walk = func(id string, path []string) {
			x, ok := teamByID(o, id)
			if !ok {
				return
			}
			for _, m := range x.Members {
				if m.AcceptedAt.IsZero() {
					continue
				}
				p := append(append([]string{}, path...), id)
				_, yes := seen[m.UserID]
				if !yes || m.Role == "maintainer" {
					seen[m.UserID] = EffectiveMember{UserID: m.UserID, Role: m.Role, ViaTeamIDs: p}
				}
			}
			for _, child := range o.Teams {
				if child.ParentID == id && visibleTeams[child.ID] {
					walk(child.ID, append(path, id))
				}
			}
		}
		walk(t.ID, nil)
		for _, m := range seen {
			d.EffectiveMembers[t.ID] = append(d.EffectiveMembers[t.ID], m)
		}
		sort.Slice(d.EffectiveMembers[t.ID], func(i, j int) bool { return d.EffectiveMembers[t.ID][i].UserID < d.EffectiveMembers[t.ID][j].UserID })
	}
	return d
}
func teamByID(o Organization, id string) (Team, bool) {
	for _, t := range o.Teams {
		if t.ID == id {
			return t, true
		}
	}
	return Team{}, false
}
func (s *Store) RequestTransfer(id, actor string, t Transfer) (Organization, Transfer, error) {
	var made Transfer
	o, e := s.change(id, func(o *Organization) error {
		if !authorizedEndpoint(*o, actor, t.FromKind, t.FromID) {
			return ErrForbidden
		}
		for _, x := range o.Transfers {
			if x.RepositoryID == t.RepositoryID && x.State == "pending" {
				return ErrConflict
			}
		}
		tid, er := newID()
		if er != nil {
			return er
		}
		now := s.now().UTC()
		t.ID = tid
		t.RequestedByID = actor
		t.State = "pending"
		t.CreatedAt = now
		o.Transfers = append(o.Transfers, t)
		made = t
		event(o, "repository.transfer_requested", actor, t.RepositoryID, now)
		return nil
	})
	return o, made, e
}
func (s *Store) ResolveTransfer(id, transfer, actor, state string) (Organization, Transfer, error) {
	var found Transfer
	o, e := s.change(id, func(o *Organization) error {
		for i := range o.Transfers {
			x := &o.Transfers[i]
			if x.ID != transfer {
				continue
			}
			if x.State != "pending" {
				return ErrConflict
			}
			if !authorizedEndpoint(*o, actor, x.ToKind, x.ToID) {
				return ErrForbidden
			}
			if state != "accepted" && state != "declined" {
				return ErrInvalid
			}
			now := s.now().UTC()
			x.State = state
			x.ResolvedAt = &now
			x.ResolvedByID = actor
			found = *x
			event(o, "repository.transfer_"+state, actor, x.RepositoryID, now)
			return nil
		}
		return ErrNotFound
	})
	return o, found, e
}

var policyDomains = map[string]bool{"repository_visibility": true, "reviews": true, "required_checks": true, "integration": true, "release_provenance": true, "dependency_use": true, "environment_promotion": true, "agent_authority": true}

// DraftPolicy appends a version. Supplying policyID creates the next immutable
// version in that lineage; an empty policyID starts a new policy.
func (s *Store) DraftPolicy(id, actor, policyID string, in PolicyVersion) (Organization, PolicyVersion, error) {
	var made PolicyVersion
	o, err := s.change(id, func(o *Organization) error {
		m, ok := membership(*o, actor, true)
		if !ok || m.Role != "owner" {
			return ErrForbidden
		}
		in.Name, in.Description = strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)
		if in.Name == "" || len(in.Name) > 100 || len(in.Description) > 1000 || len(in.Rules) == 0 || len(in.Targets) == 0 {
			return ErrInvalid
		}
		version := int64(1)
		if policyID == "" {
			var e error
			policyID, e = newID()
			if e != nil {
				return e
			}
		} else {
			found := false
			for _, p := range o.Policies {
				if p.ID == policyID {
					found = true
					if p.Version >= version {
						version = p.Version + 1
					}
				}
			}
			if !found {
				return ErrNotFound
			}
		}
		seenRules := map[string]bool{}
		for i := range in.Rules {
			r := &in.Rules[i]
			if !policyDomains[r.Domain] || (r.Enforcement != "required" && r.Enforcement != "advisory") || len(r.Config) == 0 || !json.Valid(r.Config) {
				return ErrInvalid
			}
			if r.ID == "" {
				r.ID = r.Domain
			}
			if seenRules[r.ID] {
				return ErrInvalid
			}
			seenRules[r.ID] = true
		}
		seenTargets := map[string]bool{}
		for _, target := range in.Targets {
			if target.Kind != "organization" && target.Kind != "team" && target.Kind != "repository" {
				return ErrInvalid
			}
			if target.Kind == "organization" && target.ID != "" {
				return ErrInvalid
			}
			if target.Kind != "organization" && target.ID == "" {
				return ErrInvalid
			}
			key := target.Kind + ":" + target.ID
			if seenTargets[key] {
				return ErrInvalid
			}
			seenTargets[key] = true
			if target.Kind == "team" {
				if _, ok := teamByID(*o, target.ID); !ok {
					return ErrInvalid
				}
			}
		}
		now := s.now().UTC()
		in.ID, in.Version, in.State, in.CreatedByID, in.CreatedAt = policyID, version, "draft", actor, now
		in.ActivatedAt, in.ActivatedByID = nil, ""
		o.Policies = append(o.Policies, in)
		made = in
		event(o, "policy.drafted", actor, policyID, now)
		return nil
	})
	return o, made, err
}

func (s *Store) ActivatePolicy(id, actor, policyID string, version int64) (Organization, PolicyVersion, error) {
	var activated PolicyVersion
	o, err := s.change(id, func(o *Organization) error {
		m, ok := membership(*o, actor, true)
		if !ok || m.Role != "owner" {
			return ErrForbidden
		}
		index := -1
		for i := range o.Policies {
			if o.Policies[i].ID == policyID && o.Policies[i].Version == version {
				index = i
			}
		}
		if index < 0 {
			return ErrNotFound
		}
		if o.Policies[index].State != "draft" {
			return ErrConflict
		}
		now := s.now().UTC()
		for i := range o.Policies {
			if o.Policies[i].ID == policyID && o.Policies[i].State == "active" {
				o.Policies[i].State = "superseded"
			}
		}
		o.Policies[index].State, o.Policies[index].ActivatedByID, o.Policies[index].ActivatedAt = "active", actor, &now
		activated = o.Policies[index]
		event(o, "policy.activated", actor, policyID, now)
		return nil
	})
	return o, activated, err
}

func (s *Store) EffectivePolicy(id, repositoryID string, includeDraft *PolicyVersion) ([]EffectiveRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, err := s.read(id)
	if err != nil {
		return nil, err
	}
	policies := o.Policies
	result := []EffectiveRule{}
	for _, p := range policies {
		if includeDraft != nil && p.ID == includeDraft.ID && p.Version != includeDraft.Version {
			continue
		}
		if p.State != "active" && (includeDraft == nil || p.ID != includeDraft.ID || p.Version != includeDraft.Version) {
			continue
		}
		for _, target := range p.Targets {
			if !targetApplies(o, target, repositoryID) {
				continue
			}
			for _, rule := range p.Rules {
				e := EffectiveRule{PolicyID: p.ID, PolicyVersion: p.Version, PolicyName: p.Name, Target: target, Rule: rule}
				for i := range o.PolicyExceptions {
					x := &o.PolicyExceptions[i]
					if x.PolicyID == p.ID && x.PolicyVersion == p.Version && x.RuleID == rule.ID && x.RepositoryID == repositoryID && x.State == "approved" && x.ExpiresAt.After(s.now()) {
						copy := *x
						e.Exception = &copy
					}
				}
				result = append(result, e)
			}
		}
	}
	return result, nil
}

func targetApplies(o Organization, target PolicyTarget, repositoryID string) bool {
	if target.Kind == "organization" || (target.Kind == "repository" && target.ID == repositoryID) {
		return true
	}
	if target.Kind == "team" {
		if team, ok := teamByID(o, target.ID); ok {
			for _, r := range team.Responsibilities {
				if r.RepositoryID == repositoryID {
					return true
				}
			}
		}
	}
	return false
}

func (s *Store) RequestPolicyException(id, actor string, in PolicyException) (Organization, PolicyException, error) {
	var made PolicyException
	o, err := s.change(id, func(o *Organization) error {
		if _, ok := membership(*o, actor, true); !ok {
			return ErrForbidden
		}
		now := s.now().UTC()
		in.Reason = strings.TrimSpace(in.Reason)
		if in.RepositoryID == "" || in.RuleID == "" || in.Reason == "" || !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(30*24*time.Hour)) {
			return ErrInvalid
		}
		found := false
		for _, p := range o.Policies {
			if p.ID == in.PolicyID && p.Version == in.PolicyVersion && p.State == "active" && targetApplies(*o, firstMatchingTarget(p.Targets, *o, in.RepositoryID), in.RepositoryID) {
				for _, r := range p.Rules {
					if r.ID == in.RuleID {
						found = true
					}
				}
			}
		}
		if !found {
			return ErrInvalid
		}
		for _, x := range o.PolicyExceptions {
			if x.RepositoryID == in.RepositoryID && x.PolicyID == in.PolicyID && x.PolicyVersion == in.PolicyVersion && x.RuleID == in.RuleID && (x.State == "pending" || x.State == "approved") && x.ExpiresAt.After(now) {
				return ErrConflict
			}
		}
		var e error
		in.ID, e = newID()
		if e != nil {
			return e
		}
		in.RequestedByID, in.RequestedAt, in.State = actor, now, "pending"
		in.ResolvedAt, in.ResolvedByID = nil, ""
		o.PolicyExceptions = append(o.PolicyExceptions, in)
		made = in
		event(o, "policy.exception_requested", actor, in.ID, now)
		return nil
	})
	return o, made, err
}
func firstMatchingTarget(targets []PolicyTarget, o Organization, repo string) PolicyTarget {
	for _, t := range targets {
		if targetApplies(o, t, repo) {
			return t
		}
	}
	return PolicyTarget{}
}

func (s *Store) ResolvePolicyException(id, exceptionID, actor, decision string) (Organization, PolicyException, error) {
	var resolved PolicyException
	o, err := s.change(id, func(o *Organization) error {
		m, ok := membership(*o, actor, true)
		if !ok || m.Role != "owner" {
			return ErrForbidden
		}
		if decision != "approved" && decision != "denied" {
			return ErrInvalid
		}
		for i := range o.PolicyExceptions {
			x := &o.PolicyExceptions[i]
			if x.ID == exceptionID {
				if x.State != "pending" {
					return ErrConflict
				}
				now := s.now().UTC()
				x.State, x.ResolvedByID, x.ResolvedAt = decision, actor, &now
				resolved = *x
				event(o, "policy.exception_"+decision, actor, x.ID, now)
				return nil
			}
		}
		return ErrNotFound
	})
	return o, resolved, err
}

func (s *Store) CreateInitiative(id, actor string, in Initiative) (Organization, Initiative, error) {
	var made Initiative
	o, err := s.change(id, func(o *Organization) error {
		if _, ok := membership(*o, actor, true); !ok {
			return ErrForbidden
		}
		in.Title, in.Outcome = strings.TrimSpace(in.Title), strings.TrimSpace(in.Outcome)
		if in.Title == "" || in.Outcome == "" || len(in.Title) > 200 || len(in.Outcome) > 4000 || len(in.Sources) == 0 {
			return ErrInvalid
		}
		for _, source := range in.Sources {
			if !validInitiativeResource(source, true) {
				return ErrInvalid
			}
		}
		uid, e := newID()
		if e != nil {
			return e
		}
		now := s.now().UTC()
		in.ID, in.State, in.Items, in.CreatedByID, in.CreatedAt, in.UpdatedAt = uid, "open", []InitiativeItem{}, actor, now, now
		made = in
		o.Initiatives = append(o.Initiatives, in)
		event(o, "initiative.created", actor, uid, now)
		return nil
	})
	return o, made, err
}

func (s *Store) PutInitiativeItem(id, initiativeID, itemID, actor string, in InitiativeItem) (Organization, InitiativeItem, error) {
	var saved InitiativeItem
	o, err := s.change(id, func(o *Organization) error {
		if _, ok := membership(*o, actor, true); !ok {
			return ErrForbidden
		}
		var initiative *Initiative
		for i := range o.Initiatives {
			if o.Initiatives[i].ID == initiativeID {
				initiative = &o.Initiatives[i]
			}
		}
		if initiative == nil {
			return ErrNotFound
		}
		in.Title, in.Outcome, in.NextDecision = strings.TrimSpace(in.Title), strings.TrimSpace(in.Outcome), strings.TrimSpace(in.NextDecision)
		if in.Title == "" || in.Outcome == "" || in.RepositoryID == "" || len(in.Title) > 200 || len(in.Outcome) > 4000 || len(in.NextDecision) > 1000 || !validInitiativeResource(in.Source, false) || !validInitiativeAssignee(*o, in.AssigneeKind, in.AssigneeID) {
			return ErrInvalid
		}
		if in.State == "" {
			in.State = "planned"
		}
		if in.State != "planned" && in.State != "in_progress" && in.State != "completed" && in.State != "canceled" {
			return ErrInvalid
		}
		if itemID == "" {
			var e error
			itemID, e = newID()
			if e != nil {
				return e
			}
		}
		in.ID, in.UpdatedByID, in.UpdatedAt = itemID, actor, s.now().UTC()
		if in.Position < 1 {
			in.Position = len(initiative.Items) + 1
		}
		found := false
		for i := range initiative.Items {
			if initiative.Items[i].ID == itemID {
				initiative.Items[i] = in
				found = true
			}
		}
		if !found {
			initiative.Items = append(initiative.Items, in)
		}
		if !validInitiativeGraph(initiative.Items) {
			return ErrInvalid
		}
		sort.SliceStable(initiative.Items, func(i, j int) bool { return initiative.Items[i].Position < initiative.Items[j].Position })
		initiative.UpdatedAt, saved = in.UpdatedAt, in
		event(o, "initiative.item_saved", actor, itemID, in.UpdatedAt)
		return nil
	})
	return o, saved, err
}

// InitiativeView derives reassignment and dependency blockers from current
// organization membership and repository ownership. Historical assignment is
// retained so work never silently becomes unowned.
func (s *Store) InitiativeView(id string, repositoryOwned func(string) bool) ([]Initiative, error) {
	o, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	out := append([]Initiative(nil), o.Initiatives...)
	for ii := range out {
		out[ii].Items = append([]InitiativeItem(nil), out[ii].Items...)
		state := map[string]string{}
		for _, item := range out[ii].Items {
			state[item.ID] = item.State
		}
		for i := range out[ii].Items {
			item := &out[ii].Items[i]
			item.BlockedBy = nil
			for _, dependency := range item.DependsOn {
				if state[dependency] != "completed" {
					item.BlockedBy = append(item.BlockedBy, dependency)
				}
			}
			valid := repositoryOwned(item.RepositoryID) && validInitiativeAssignee(o, item.AssigneeKind, item.AssigneeID) && initiativePrincipalAccess(o, item, s.now().UTC())
			item.NeedsReassignment = !valid
			if !valid {
				item.BlockedBy = append(item.BlockedBy, "reassignment")
			}
			if len(item.BlockedBy) > 0 && item.NextDecision == "" {
				item.NextDecision = "Resolve the listed dependency or assign an accountable principal with current access."
			}
		}
	}
	return out, nil
}

func initiativePrincipalAccess(o Organization, item *InitiativeItem, now time.Time) bool {
	if item.AssigneeKind == "human" {
		_, ok := membership(o, item.AssigneeID, true)
		return ok
	}
	if item.AssigneeKind == "team" {
		if team, ok := teamByID(o, item.AssigneeID); ok {
			for _, responsibility := range team.Responsibilities {
				if responsibility.RepositoryID == item.RepositoryID {
					return true
				}
			}
		}
	}
	for _, grant := range o.RoleGrants {
		if grant.PrincipalKind != item.AssigneeKind || grant.PrincipalID != item.AssigneeID || grant.RevokedAt != nil || !grant.ExpiresAt.After(now) {
			continue
		}
		for _, resource := range grant.Resources {
			if (resource.Kind == "repository" && resource.ID == item.RepositoryID) || resource.RepositoryID == item.RepositoryID {
				return true
			}
		}
	}
	return false
}

func validInitiativeResource(r ResourceRef, source bool) bool {
	if r.ID == "" || r.RepositoryID == "" {
		return false
	}
	if source {
		return r.Kind == "proposal" || r.Kind == "evolution" || r.Kind == "incident" || r.Kind == "security"
	}
	return r.Kind == "proposal_task" || r.Kind == "evolution_task" || r.Kind == "incident_action" || r.Kind == "security_repair"
}
func validInitiativeAssignee(o Organization, kind, id string) bool {
	if id == "" {
		return false
	}
	if kind == "human" {
		_, ok := membership(o, id, true)
		return ok
	}
	if kind == "team" {
		_, ok := teamByID(o, id)
		return ok
	}
	if kind == "agent" {
		for _, a := range o.Agents {
			if a.ID == id && len(a.OperatorIDs) > 0 {
				return true
			}
		}
		return false
	}
	return false
}
func validInitiativeGraph(items []InitiativeItem) bool {
	byID := map[string]InitiativeItem{}
	for _, item := range items {
		byID[item.ID] = item
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if done[id] {
			return true
		}
		visiting[id] = true
		for _, dep := range byID[id].DependsOn {
			if _, ok := byID[dep]; !ok || !visit(dep) {
				return false
			}
		}
		visiting[id], done[id] = false, true
		return true
	}
	for id := range byID {
		if !visit(id) {
			return false
		}
	}
	return true
}

func authorizedEndpoint(o Organization, user, kind, id string) bool {
	if kind == "user" {
		return user == id
	}
	if kind == "organization" && o.ID == id {
		m, ok := membership(o, user, true)
		return ok && m.Role == "owner"
	}
	return false
}
func membership(o Organization, user string, accepted bool) (Member, bool) {
	for _, m := range o.Members {
		if m.UserID == user && (!accepted || !m.AcceptedAt.IsZero()) {
			return m, true
		}
	}
	return Member{}, false
}
func event(o *Organization, kind, actor, subject string, at time.Time) {
	o.Events = append(o.Events, Event{Sequence: int64(len(o.Events) + 1), Type: kind, ActorID: actor, SubjectID: subject, CreatedAt: at})
}
func (s *Store) change(id string, fn func(*Organization) error) (Organization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(id)
	if e != nil {
		return Organization{}, e
	}
	if e = fn(&o); e != nil {
		return Organization{}, e
	}
	o.UpdatedAt = s.now().UTC()
	e = s.write(o)
	return o, e
}
func (s *Store) list() ([]Organization, error) {
	entries, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Organization{}
	for _, x := range entries {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		o, e := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *Store) read(id string) (Organization, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Organization{}, ErrNotFound
	}
	if e != nil {
		return Organization{}, e
	}
	var o Organization
	if json.Unmarshal(b, &o) != nil || o.ID != id {
		return Organization{}, ErrInvalid
	}
	return o, nil
}
func (s *Store) write(o Organization) error {
	b, e := json.Marshal(o)
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(s.root, ".organization-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0640); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(s.root, o.ID+".json"))
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
