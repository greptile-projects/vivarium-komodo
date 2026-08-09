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
type Organization struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Members     []Member   `json:"members"`
	Transfers   []Transfer `json:"transfers"`
	Events      []Event    `json:"events"`
	Teams       []Team     `json:"teams"`
	Agents      []Agent    `json:"agents"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
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
	o := Organization{ID: id, Slug: slug, Name: name, Description: description, Members: []Member{{UserID: actor, Role: "owner", AcceptedAt: now}}, Transfers: []Transfer{}, Teams: []Team{}, Agents: []Agent{}, Events: []Event{{Sequence: 1, Type: "organization.created", ActorID: actor, CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
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
