// Package assuranceprograms owns versioned obligation-to-control maps.
package assuranceprograms

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

var ErrNotFound = errors.New("assurance program not found")
var ErrInvalid = errors.New("invalid assurance program")
var ErrConflict = errors.New("assurance program version conflict")

type Requirement struct {
	ID              string `json:"id"`
	SourceKind      string `json:"source_kind"`
	SourceReference string `json:"source_reference"`
	SourceVersion   string `json:"source_version"`
	Title           string `json:"title"`
	Text            string `json:"text"`
	Applicability   string `json:"applicability"`
	Interpretation  string `json:"interpretation"`
	InheritedFrom   string `json:"inherited_from,omitempty"`
	AuthorID        string `json:"author_id"`
}
type Target struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
	Detail    string `json:"detail"`
}
type EvidenceCriterion struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Description     string `json:"description"`
	Frequency       string `json:"frequency"`
	SourceReference string `json:"source_reference"`
}
type Exception struct {
	ID                string    `json:"id"`
	RequirementID     string    `json:"requirement_id"`
	ControlID         string    `json:"control_id,omitempty"`
	Rationale         string    `json:"rationale"`
	OwnerID           string    `json:"owner_id"`
	ApprovalReference string    `json:"approval_reference"`
	ExpiresAt         time.Time `json:"expires_at"`
}
type Control struct {
	ID               string              `json:"id"`
	Objective        string              `json:"objective"`
	Claim            string              `json:"claim"`
	ReviewPeriod     string              `json:"review_period"`
	RequirementIDs   []string            `json:"requirement_ids"`
	OwnerIDs         []string            `json:"owner_ids"`
	Targets          []Target            `json:"targets"`
	EvidenceCriteria []EvidenceCriterion `json:"evidence_criteria"`
}
type Input struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Scope        string        `json:"scope"`
	ChangeReason string        `json:"change_reason"`
	OwnerIDs     []string      `json:"owner_ids"`
	Requirements []Requirement `json:"requirements"`
	Controls     []Control     `json:"controls"`
	Exceptions   []Exception   `json:"exceptions"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Gap struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to"`
}
type Program struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
	ClaimStatus    string    `json:"claim_status"`
}
type Catalog struct {
	Items []Program `json:"items"`
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
func id() string          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func clean(s string) bool { return strings.TrimSpace(s) != "" }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func valid(in Input) bool {
	if !clean(in.Name) || !clean(in.Description) || !clean(in.Scope) || !clean(in.ChangeReason) || len(in.Requirements) == 0 {
		return false
	}
	rq, ct, ex := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Requirements {
		if !clean(x.ID) || rq[x.ID] || !allowed(x.SourceKind, "regulatory", "contractual", "organization") || !clean(x.SourceReference) || !clean(x.SourceVersion) || !clean(x.Title) || !clean(x.Text) || !clean(x.Applicability) || !clean(x.Interpretation) || !clean(x.AuthorID) {
			return false
		}
		rq[x.ID] = true
	}
	for _, x := range in.Controls {
		if !clean(x.ID) || ct[x.ID] || !clean(x.Objective) || !clean(x.Claim) || !clean(x.ReviewPeriod) || len(x.RequirementIDs) == 0 {
			return false
		}
		ct[x.ID] = true
		for _, r := range x.RequirementIDs {
			if !rq[r] {
				return false
			}
		}
		for _, t := range x.Targets {
			if !allowed(t.Kind, "repository", "policy", "data_flow", "infrastructure", "environment", "release", "procedure") || !clean(t.Reference) || !clean(t.Revision) {
				return false
			}
		}
		seen := map[string]bool{}
		for _, e := range x.EvidenceCriteria {
			if !clean(e.ID) || seen[e.ID] || !clean(e.Kind) || !clean(e.Description) || !clean(e.Frequency) || !clean(e.SourceReference) {
				return false
			}
			seen[e.ID] = true
		}
	}
	for _, x := range in.Exceptions {
		if !clean(x.ID) || ex[x.ID] || !rq[x.RequirementID] || (!clean(x.ControlID) && x.ControlID != "") || !clean(x.Rationale) || !clean(x.OwnerID) || !clean(x.ApprovalReference) || x.ExpiresAt.IsZero() {
			return false
		}
		if x.ControlID != "" && !ct[x.ControlID] {
			return false
		}
		ex[x.ID] = true
	}
	return true
}
func derive(v Version, now time.Time) []Gap {
	out := []Gap{}
	add := func(k, s, d, a string) { out = append(out, Gap{k, s, d, a}) }
	covered := map[string]bool{}
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "assurance program has no accountable owner", v.AuthorID)
	}
	for _, c := range v.Controls {
		for _, r := range c.RequirementIDs {
			covered[r] = true
		}
		if len(c.OwnerIDs) == 0 {
			add("missing_owner", c.ID, "control has no accountable owner", v.AuthorID)
		}
		if len(c.Targets) == 0 {
			add("unsupported_claim", c.ID, "control claim has no exact implementation mapping", v.AuthorID)
		}
		if len(c.EvidenceCriteria) == 0 {
			add("unsupported_claim", c.ID, "control claim has no evidence criteria", v.AuthorID)
		}
	}
	for _, r := range v.Requirements {
		if !covered[r.ID] {
			add("unmapped_requirement", r.ID, "applicable obligation has no project control", r.AuthorID)
		}
		if r.InheritedFrom != "" {
			add("inherited_obligation", r.ID, "inherited from "+r.InheritedFrom, r.AuthorID)
		}
		for _, q := range v.Requirements {
			if r.ID < q.ID && r.SourceReference == q.SourceReference && r.SourceVersion == q.SourceVersion && r.Interpretation != q.Interpretation {
				add("conflicting_interpretation", r.ID+":"+q.ID, "same source has conflicting interpretations", r.AuthorID)
			}
		}
	}
	for _, x := range v.Exceptions {
		if !x.ExpiresAt.After(now) {
			add("expired_exception", x.ID, x.Rationale, x.OwnerID)
		} else if x.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			add("expiring_exception", x.ID, x.Rationale, x.OwnerID)
		}
	}
	return out
}
func (s *Store) path(repo, p string) string { return filepath.Join(s.root, repo, p+".json") }
func (s *Store) save(x Program) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Program, error) {
	if !clean(repo) || !clean(actor) || !valid(in) {
		return Program{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Program{}, e
	}
	for _, x := range xs {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Program{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	g := derive(v, s.now().UTC())
	x := Program{id(), repo, 1, []Version{v}, g, status(g)}
	return x, s.save(x)
}
func (s *Store) Revise(repo, p, actor string, expected int64, in Input) (Program, error) {
	if !clean(actor) || !valid(in) {
		return Program{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, p)
	if e != nil {
		return Program{}, e
	}
	if x.CurrentVersion != expected {
		return Program{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v, s.now().UTC())
	x.ClaimStatus = status(x.Gaps)
	return x, s.save(x)
}
func status(g []Gap) string {
	if len(g) == 0 {
		return "supported"
	}
	return "gaps_explicit"
}
func (s *Store) Get(repo, p string) (Program, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, p)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
}
func (s *Store) read(repo, p string) (Program, error) {
	b, e := os.ReadFile(s.path(repo, p))
	if errors.Is(e, fs.ErrNotExist) {
		return Program{}, ErrNotFound
	}
	var x Program
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != p {
		return Program{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.Gaps = derive(x.Versions[len(x.Versions)-1], s.now().UTC())
		x.ClaimStatus = status(x.Gaps)
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Program, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Program{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Program{}
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].Name < out[j].Versions[len(out[j].Versions)-1].Name
	})
	return out, nil
}
