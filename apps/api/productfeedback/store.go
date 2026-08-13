// Package productfeedback owns consent-bound product need reports.
package productfeedback

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

var ErrNotFound = errors.New("feedback not found")
var ErrInvalid = errors.New("invalid feedback")

type Context struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Label      string `json:"label,omitempty"`
}
type Consent struct {
	Research       bool       `json:"research"`
	ProductUpdates bool       `json:"product_updates"`
	GrantedAt      time.Time  `json:"granted_at"`
	WithdrawnAt    *time.Time `json:"withdrawn_at,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	MediaType  string `json:"media_type"`
	Content    string `json:"content,omitempty"`
	Visibility string `json:"visibility"`
	Redacted   bool   `json:"redacted"`
}
type Link struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	AddedByID  string    `json:"added_by_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Feedback struct {
	ID                 string     `json:"id"`
	RepositoryID       string     `json:"repository_id"`
	ReporterID         string     `json:"reporter_id,omitempty"`
	OrganizationID     string     `json:"organization_id,omitempty"`
	Context            Context    `json:"context"`
	Need               string     `json:"need"`
	DesiredOutcome     string     `json:"desired_outcome"`
	Frequency          string     `json:"frequency"`
	Impact             string     `json:"impact"`
	Audience           string     `json:"audience"`
	IdentityVisibility string     `json:"identity_visibility"`
	ContactPreference  string     `json:"contact_preference"`
	ContactValue       string     `json:"contact_value,omitempty"`
	Consent            Consent    `json:"consent"`
	Evidence           []Evidence `json:"evidence"`
	Discussion         []Comment  `json:"discussion"`
	Links              []Link     `json:"links"`
	History            []Event    `json:"history"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
type Input struct {
	OrganizationID     string     `json:"organization_id"`
	Context            Context    `json:"context"`
	Need               string     `json:"need"`
	DesiredOutcome     string     `json:"desired_outcome"`
	Frequency          string     `json:"frequency"`
	Impact             string     `json:"impact"`
	Audience           string     `json:"audience"`
	IdentityVisibility string     `json:"identity_visibility"`
	ContactPreference  string     `json:"contact_preference"`
	ContactValue       string     `json:"contact_value"`
	Consent            Consent    `json:"consent"`
	Evidence           []Evidence `json:"evidence"`
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
	root, _ = filepath.Abs(root)
	if e := os.MkdirAll(root, 0750); e != nil {
		return nil, e
	}
	return &Store{root: root, now: time.Now}, nil
}
func valid(in Input) bool {
	trim := func(v string, max int) bool { return strings.TrimSpace(v) != "" && len(v) <= max }
	if !trim(in.Need, 65536) || !trim(in.DesiredOutcome, 65536) || !trim(in.Impact, 65536) || !map[string]bool{"once": true, "rarely": true, "monthly": true, "weekly": true, "daily": true, "continuous": true}[in.Frequency] {
		return false
	}
	if !map[string]bool{"project": true, "release": true, "journey": true, "preview": true}[in.Context.Kind] || (in.Context.Kind != "project" && in.Context.ResourceID == "") {
		return false
	}
	if !map[string]bool{"public": true, "repository": true, "organization": true}[in.Audience] || (in.Audience == "organization") != (in.OrganizationID != "") {
		return false
	}
	if !map[string]bool{"audience": true, "maintainers": true}[in.IdentityVisibility] || !map[string]bool{"none": true, "discussion": true, "email": true}[in.ContactPreference] || (in.ContactPreference == "email") != (strings.TrimSpace(in.ContactValue) != "") {
		return false
	}
	if len(in.Evidence) > 10 {
		return false
	}
	total := 0
	for _, e := range in.Evidence {
		total += len(e.Content)
		if !e.Redacted || !map[string]bool{"quote": true, "screenshot": true, "log": true, "document": true}[e.Kind] || !map[string]bool{"audience": true, "maintainers": true}[e.Visibility] || !trim(e.Name, 200) || !trim(e.MediaType, 100) || e.Content == "" || len(e.Content) > 1<<20 {
			return false
		}
	}
	return total <= 5<<20
}
func (s *Store) Create(repo, actor string, in Input) (Feedback, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Feedback{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	id := newID()
	for i := range in.Evidence {
		in.Evidence[i].ID = newID()
	}
	in.Consent.GrantedAt = now
	v := Feedback{ID: id, RepositoryID: repo, ReporterID: actor, OrganizationID: in.OrganizationID, Context: in.Context, Need: strings.TrimSpace(in.Need), DesiredOutcome: strings.TrimSpace(in.DesiredOutcome), Frequency: in.Frequency, Impact: strings.TrimSpace(in.Impact), Audience: in.Audience, IdentityVisibility: in.IdentityVisibility, ContactPreference: in.ContactPreference, ContactValue: strings.TrimSpace(in.ContactValue), Consent: in.Consent, Evidence: in.Evidence, Discussion: []Comment{}, Links: []Link{}, History: []Event{{Sequence: 1, Type: "feedback.submitted", ActorID: actor, CreatedAt: now}}, Status: "open", CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Feedback{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Feedback{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Comment(repo, id, actor, body string) (Feedback, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 65536 {
		return Feedback{}, ErrInvalid
	}
	return s.mutate(repo, id, func(v *Feedback) {
		now := s.now().UTC()
		v.Discussion = append(v.Discussion, Comment{ID: newID(), AuthorID: actor, Body: body, CreatedAt: now})
		v.event("comment.added", actor, now)
	})
}
func (s *Store) Link(repo, id, actor, kind, resource string) (Feedback, error) {
	if !map[string]bool{"issue": true, "experiment": true}[kind] || resource == "" {
		return Feedback{}, ErrInvalid
	}
	return s.mutate(repo, id, func(v *Feedback) {
		now := s.now().UTC()
		v.Links = append(v.Links, Link{Kind: kind, ResourceID: resource, AddedByID: actor, CreatedAt: now})
		v.event("link.added", actor, now)
	})
}
func (s *Store) Withdraw(repo, id, actor string) (Feedback, error) {
	return s.mutate(repo, id, func(v *Feedback) {
		now := s.now().UTC()
		v.Consent.WithdrawnAt = &now
		v.ContactPreference = "none"
		v.ContactValue = ""
		v.event("consent.withdrawn", actor, now)
	})
}
func (v *Feedback) event(kind, actor string, now time.Time) {
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: kind, ActorID: actor, CreatedAt: now})
}
func (s *Store) mutate(repo, id string, fn func(*Feedback)) (Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	fn(&v)
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Feedback, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Feedback{}, ErrNotFound
	}
	var v Feedback
	if e != nil || json.Unmarshal(b, &v) != nil || v.ID != id || v.RepositoryID != repo {
		return Feedback{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Feedback) error {
	d := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".feedback-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, e = tmp.Write(b)
	if e == nil {
		e = tmp.Chmod(0600)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(d, v.ID+".json"))
}
func newID() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
