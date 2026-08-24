// Package historyremediations owns restricted, pre-inspection history repair scopes.
package historyremediations

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

var ErrNotFound = errors.New("history remediation not found")
var ErrInvalid = errors.New("invalid history remediation")

type Source struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
}
type Object struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Kind         string `json:"kind"`
	ObjectID     string `json:"object_id"`
	Path         string `json:"path,omitempty"`
	Digest       string `json:"digest,omitempty"`
	Match        string `json:"match"`
	Reason       string `json:"reason,omitempty"`
	AttributedTo string `json:"attributed_to"`
}
type Scope struct {
	Kind         string `json:"kind"`
	RepositoryID string `json:"repository_id,omitempty"`
	Reference    string `json:"reference"`
	Revision     string `json:"revision,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision,omitempty"`
	Digest     string `json:"digest"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	RecordedBy string `json:"recorded_by"`
}
type Constraint struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	OwnerID   string `json:"owner_id"`
	Rationale string `json:"rationale"`
}
type Approval struct {
	Kind     string `json:"kind"`
	OwnerID  string `json:"owner_id"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
}
type Input struct {
	Title              string       `json:"title"`
	Source             Source       `json:"source"`
	ContentDescription string       `json:"content_description"`
	Reason             string       `json:"reason"`
	Audience           string       `json:"audience"`
	ResponseOwnerIDs   []string     `json:"response_owner_ids"`
	ParticipantIDs     []string     `json:"participant_ids"`
	Objects            []Object     `json:"objects"`
	Scope              []Scope      `json:"scope"`
	Evidence           []Evidence   `json:"discovery_evidence"`
	Constraints        []Constraint `json:"constraints"`
	Approvals          []Approval   `json:"required_approvals"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Remediation struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	CreatedByID  string    `json:"created_by_id"`
	Input        Input     `json:"definition"`
	Blockers     []Blocker `json:"blockers"`
	Events       []Event   `json:"history"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Catalog struct {
	Items []Remediation `json:"items"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func oneOf(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func text(v string) bool {
	v = strings.ToLower(v)
	return !strings.Contains(v, "-----begin") && !strings.Contains(v, "password=") && !strings.Contains(v, "authorization: bearer") && len(v) <= 2000
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.ContentDescription) == "" || strings.TrimSpace(in.Reason) == "" || !text(in.ContentDescription) || !text(in.Reason) || !oneOf(in.Source.Kind, "security_finding", "privacy_incident", "support_case", "selected_object") || in.Source.ID == "" || !oneOf(in.Audience, "owners_only", "response_team", "named_participants") || len(in.ResponseOwnerIDs) == 0 || len(in.Objects) == 0 || len(in.Scope) == 0 || len(in.Evidence) == 0 || len(in.Approvals) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, o := range in.Objects {
		if o.ID == "" || seen[o.ID] || o.RepositoryID == "" || o.ObjectID == "" || o.AttributedTo == "" || !oneOf(o.Kind, "commit", "tree", "blob", "tag", "path") || !oneOf(o.Match, "confirmed", "false_match", "inaccessible") || !text(o.Reason) {
			return false
		}
		seen[o.ID] = true
	}
	for _, s := range in.Scope {
		if s.Reference == "" || !oneOf(s.Kind, "repository", "ref", "release", "package", "artifact", "environment") {
			return false
		}
	}
	seen = map[string]bool{}
	for _, e := range in.Evidence {
		if e.ID == "" || seen[e.ID] || e.Reference == "" || e.Digest == "" || e.Summary == "" || e.RecordedBy == "" || !text(e.Summary) || !text(e.Reason) || !oneOf(e.Status, "available", "inaccessible", "expired") {
			return false
		}
		seen[e.ID] = true
	}
	for _, c := range in.Constraints {
		if c.ID == "" || c.Reference == "" || c.OwnerID == "" || c.Rationale == "" || !text(c.Rationale) || !oneOf(c.Kind, "legal_hold", "retention", "continuity") || !oneOf(c.Status, "clear", "conflict", "inaccessible") {
			return false
		}
	}
	for _, a := range in.Approvals {
		if a.OwnerID == "" || !oneOf(a.Kind, "repository_owner", "security_owner", "privacy_owner", "legal_owner", "release_owner", "environment_owner") || !oneOf(a.Status, "pending", "approved", "denied") {
			return false
		}
	}
	return true
}
func derive(in Input) []Blocker {
	out := []Blocker{}
	for _, o := range in.Objects {
		if o.Match != "confirmed" {
			out = append(out, Blocker{o.Match, o.ID, o.Reason, o.AttributedTo})
		}
	}
	for _, e := range in.Evidence {
		if e.Status != "available" {
			out = append(out, Blocker{"evidence_" + e.Status, e.ID, e.Reason, e.RecordedBy})
		}
	}
	for _, c := range in.Constraints {
		if c.Status != "clear" {
			out = append(out, Blocker{c.Kind + "_" + c.Status, c.Reference, c.Rationale, c.OwnerID})
		}
	}
	for _, a := range in.Approvals {
		if a.Required && a.Status != "approved" {
			out = append(out, Blocker{"approval_" + a.Status, a.Kind, "required approval is " + a.Status, a.OwnerID})
		}
	}
	return out
}
func ident() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Remediation) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Remediation, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	x := Remediation{ident(), repo, actor, in, derive(in), []Event{{1, "remediation.opened", actor, now}}, now, now}
	return x, s.save(x)
}
func participant(x Remediation, actor string) bool {
	if actor == x.CreatedByID {
		return true
	}
	for _, id := range append(append([]string{}, x.Input.ResponseOwnerIDs...), x.Input.ParticipantIDs...) {
		if actor == id {
			return true
		}
	}
	for _, a := range x.Input.Approvals {
		if actor == a.OwnerID {
			return true
		}
	}
	return false
}
func (s *Store) Get(repo, id, actor string) (Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !participant(x, actor) {
		return Remediation{}, ErrNotFound
	}
	return x, nil
}
func (s *Store) Catalog(repo, actor string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Catalog{}, e
	}
	out := []Remediation{}
	for _, x := range xs {
		if participant(x, actor) {
			out = append(out, x)
		}
	}
	return Catalog{out}, nil
}
func (s *Store) read(repo, id string) (Remediation, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Remediation{}, ErrNotFound
	}
	var x Remediation
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Remediation{}, ErrNotFound
	}
	x.Blockers = derive(x.Input)
	return x, nil
}
func (s *Store) list(repo string) ([]Remediation, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Remediation{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Remediation{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
