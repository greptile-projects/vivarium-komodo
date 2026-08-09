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
	o := Organization{ID: id, Slug: slug, Name: name, Description: description, Members: []Member{{UserID: actor, Role: "owner", AcceptedAt: now}}, Transfers: []Transfer{}, Events: []Event{{Sequence: 1, Type: "organization.created", ActorID: actor, CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
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
				event(o, "member.removed", actor, user, s.now().UTC())
				return nil
			}
		}
		return ErrNotFound
	})
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
