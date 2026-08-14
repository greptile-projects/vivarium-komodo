// Package datacommitments owns versioned declarations of permitted product data use.
package datacommitments

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

var (
	ErrNotFound = errors.New("data commitment not found")
	ErrInvalid  = errors.New("invalid data commitment")
	ErrConflict = errors.New("data commitment version conflict")
)

type Scope struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Name       string `json:"name"`
}
type DataUse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Purposes   []string `json:"purposes"`
	Subjects   []string `json:"subjects"`
	Collection string   `json:"collection"`
	Processing []string `json:"processing"`
	Sharing    []string `json:"sharing"`
	Retention  string   `json:"retention"`
	Residency  []string `json:"residency"`
	Deletion   string   `json:"deletion"`
	Consent    string   `json:"consent"`
	OwnerIDs   []string `json:"owner_ids"`
}
type Guarantee struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Rationale   string `json:"rationale,omitempty"`
}
type Link struct {
	Kind  string `json:"kind"`
	URL   string `json:"url"`
	Label string `json:"label,omitempty"`
}
type Exception struct {
	ID           string    `json:"id"`
	DataUseIDs   []string  `json:"data_use_ids"`
	GuaranteeIDs []string  `json:"guarantee_ids,omitempty"`
	Reason       string    `json:"reason"`
	ApprovedBy   string    `json:"approved_by"`
	ExpiresAt    time.Time `json:"expires_at"`
}
type VersionInput struct {
	Title        string      `json:"title"`
	Scopes       []Scope     `json:"scopes"`
	DataUses     []DataUse   `json:"data_uses"`
	Guarantees   []Guarantee `json:"guarantees"`
	OwnerIDs     []string    `json:"owner_ids"`
	Links        []Link      `json:"links"`
	Exceptions   []Exception `json:"exceptions"`
	ChangeReason string      `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	DataUseID    string `json:"data_use_id,omitempty"`
	GuaranteeID  string `json:"guarantee_id,omitempty"`
	ExceptionID  string `json:"exception_id,omitempty"`
	CommitmentID string `json:"commitment_id,omitempty"`
	Detail       string `json:"detail"`
}
type Commitment struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Blockers       []Blocker `json:"blockers"`
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
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newID() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func listOK(xs []string, required bool) bool {
	if (required && len(xs) == 0) || len(xs) > 100 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if strings.TrimSpace(in.Title) == "" || len(in.Scopes) == 0 || len(in.DataUses) == 0 || !listOK(in.OwnerIDs, false) || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	scopes, uses, guarantees := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Scopes {
		if !map[string]bool{"repository": true, "release": true, "extension": true, "experiment": true, "environment": true}[x.Kind] || x.Name == "" {
			return false
		}
		key := x.Kind + ":" + x.ResourceID
		if scopes[key] {
			return false
		}
		scopes[key] = true
	}
	for _, x := range in.DataUses {
		if x.ID == "" || x.Name == "" || uses[x.ID] || !listOK(x.Categories, true) || !listOK(x.Purposes, true) || !listOK(x.Subjects, true) || x.Collection == "" || !listOK(x.Processing, true) || !listOK(x.Sharing, true) || x.Retention == "" || !listOK(x.Residency, true) || x.Deletion == "" || x.Consent == "" || !listOK(x.OwnerIDs, false) {
			return false
		}
		uses[x.ID] = true
	}
	for _, x := range in.Guarantees {
		if x.ID == "" || x.Description == "" || guarantees[x.ID] || !map[string]bool{"supported": true, "unsupported": true, "partial": true}[x.Status] {
			return false
		}
		if x.Status != "supported" && x.Rationale == "" {
			return false
		}
		guarantees[x.ID] = true
	}
	policy, notice := false, false
	for _, x := range in.Links {
		if !map[string]bool{"policy": true, "notice": true}[x.Kind] || strings.TrimSpace(x.URL) == "" {
			return false
		}
		policy = policy || x.Kind == "policy"
		notice = notice || x.Kind == "notice"
	}
	if !policy || !notice {
		return false
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || x.Reason == "" || x.ApprovedBy == "" || x.ExpiresAt.IsZero() || !listOK(x.DataUseIDs, true) {
			return false
		}
		for _, id := range x.DataUseIDs {
			if !uses[id] {
				return false
			}
		}
		for _, id := range x.GuaranteeIDs {
			if !guarantees[id] {
				return false
			}
		}
	}
	return true
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(c Commitment) error {
	if e := os.MkdirAll(filepath.Dir(s.path(c.RepositoryID, c.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path(c.RepositoryID, c.ID), b, 0640)
}
func (s *Store) raw(repo, id string) (Commitment, error) {
	var c Commitment
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return c, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func (s *Store) Create(repo, actor string, in VersionInput) (Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || !valid(in) {
		return Commitment{}, ErrInvalid
	}
	c := Commitment{ID: newID(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}}}
	return c, s.save(c)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in VersionInput) (Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || !valid(in) {
		return Commitment{}, ErrInvalid
	}
	c, e := s.raw(repo, id)
	if e != nil {
		return c, e
	}
	if c.CurrentVersion != expected {
		return c, ErrConflict
	}
	c.CurrentVersion++
	c.Versions = append(c.Versions, Version{Number: c.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return c, s.save(c)
}
func current(c Commitment) Version { return c.Versions[len(c.Versions)-1] }
func (s *Store) List(repo string) ([]Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Commitment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Commitment{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		c, e := s.raw(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	derive(out, s.now().UTC())
	sort.Slice(out, func(i, j int) bool { return out[i].Versions[0].CreatedAt.After(out[j].Versions[0].CreatedAt) })
	return out, nil
}
func (s *Store) Get(repo, id string) (Commitment, error) {
	all, e := s.List(repo)
	if e != nil {
		return Commitment{}, e
	}
	for _, c := range all {
		if c.ID == id {
			return c, nil
		}
	}
	return Commitment{}, ErrNotFound
}
func derive(all []Commitment, now time.Time) {
	for i := range all {
		c := &all[i]
		c.Blockers = nil
		v := current(*c)
		if len(v.OwnerIDs) == 0 {
			c.Blockers = append(c.Blockers, Blocker{Kind: "missing_ownership", Detail: "commitment has no accountable owner"})
		}
		for _, u := range v.DataUses {
			if len(u.OwnerIDs) == 0 {
				c.Blockers = append(c.Blockers, Blocker{Kind: "missing_ownership", DataUseID: u.ID, Detail: "data use has no accountable owner"})
			}
		}
		for _, g := range v.Guarantees {
			if g.Status != "supported" {
				c.Blockers = append(c.Blockers, Blocker{Kind: "unsupported_guarantee", GuaranteeID: g.ID, Detail: g.Description + " is " + g.Status + ": " + g.Rationale})
			}
		}
		for _, x := range v.Exceptions {
			d := x.ExpiresAt.Sub(now)
			if d <= 0 {
				c.Blockers = append(c.Blockers, Blocker{Kind: "expired_exception", ExceptionID: x.ID, Detail: "exception expired " + x.ExpiresAt.Format(time.RFC3339)})
			} else if d <= 30*24*time.Hour {
				c.Blockers = append(c.Blockers, Blocker{Kind: "expiring_exception", ExceptionID: x.ID, Detail: "exception expires " + x.ExpiresAt.Format(time.RFC3339)})
			}
		}
	}
	for i := range all {
		a := current(all[i])
		for j := i + 1; j < len(all); j++ {
			b := current(all[j])
			shared := false
			for _, x := range a.Scopes {
				for _, y := range b.Scopes {
					if x.Kind == y.Kind && x.ResourceID == y.ResourceID {
						shared = true
					}
				}
			}
			if !shared {
				continue
			}
			for _, x := range a.DataUses {
				for _, y := range b.DataUses {
					if overlap(x.Categories, y.Categories) && overlap(x.Subjects, y.Subjects) && (strings.Join(x.Purposes, "\x00") != strings.Join(y.Purposes, "\x00") || x.Retention != y.Retention || x.Deletion != y.Deletion || x.Consent != y.Consent) {
						all[i].Blockers = append(all[i].Blockers, Blocker{Kind: "conflicting_commitment", DataUseID: x.ID, CommitmentID: all[j].ID, Detail: "overlapping category and subject have different permitted terms"})
						all[j].Blockers = append(all[j].Blockers, Blocker{Kind: "conflicting_commitment", DataUseID: y.ID, CommitmentID: all[i].ID, Detail: "overlapping category and subject have different permitted terms"})
					}
				}
			}
		}
	}
}
func overlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if strings.EqualFold(x, y) {
				return true
			}
		}
	}
	return false
}
