// Package provenancepolicies owns versioned rules for acceptable material origins and distribution.
package provenancepolicies

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

var ErrNotFound = errors.New("provenance policy not found")
var ErrInvalid = errors.New("invalid provenance policy")
var ErrConflict = errors.New("provenance policy version conflict")

type MaterialRule struct {
	Kind           string   `json:"kind"`
	Origins        []string `json:"permitted_origins"`
	Licenses       []string `json:"permitted_licenses"`
	Uses           []string `json:"permitted_uses"`
	Attribution    []string `json:"required_attribution"`
	Attestations   []string `json:"contributor_attestations"`
	ReviewOwnerIDs []string `json:"review_owner_ids"`
}
type DistributionContext struct {
	ID             string   `json:"id"`
	Audience       string   `json:"audience"`
	Uses           []string `json:"uses"`
	Licenses       []string `json:"licenses"`
	NoticeRequired bool     `json:"notice_required"`
	OwnerIDs       []string `json:"owner_ids"`
}
type Link struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Boundary  string `json:"boundary,omitempty"`
}
type Exception struct {
	ID            string    `json:"id"`
	MaterialKinds []string  `json:"material_kinds"`
	License       string    `json:"license,omitempty"`
	ContextIDs    []string  `json:"distribution_context_ids"`
	Rationale     string    `json:"rationale"`
	OwnerID       string    `json:"owner_id"`
	ApprovedBy    string    `json:"approved_by"`
	ExpiresAt     time.Time `json:"expires_at"`
}
type Input struct {
	Name                 string                `json:"name"`
	Description          string                `json:"description"`
	Rules                []MaterialRule        `json:"material_rules"`
	DistributionContexts []DistributionContext `json:"distribution_contexts"`
	Links                []Link                `json:"links"`
	Exceptions           []Exception           `json:"exceptions"`
	OwnerIDs             []string              `json:"owner_ids"`
	ChangeReason         string                `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to,omitempty"`
}
type Policy struct {
	ID             string    `json:"id"`
	ScopeKind      string    `json:"scope_kind"`
	ScopeID        string    `json:"scope_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Blockers       []Blocker `json:"blockers"`
}
type Catalog struct {
	Items []Policy `json:"items"`
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
func allowed(x string, xs ...string) bool {
	for _, v := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func listOK(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || len(in.Rules) == 0 || len(in.DistributionContexts) == 0 {
		return false
	}
	kinds := map[string]bool{}
	for _, r := range in.Rules {
		if !allowed(r.Kind, "source", "generated_code", "asset", "model", "dataset", "package", "build_input") || kinds[r.Kind] || !listOK(r.Origins, true) || !listOK(r.Licenses, true) || !listOK(r.Uses, true) || !listOK(r.Attribution, false) || !listOK(r.Attestations, false) || !listOK(r.ReviewOwnerIDs, false) {
			return false
		}
		kinds[r.Kind] = true
	}
	contexts := map[string]bool{}
	for _, c := range in.DistributionContexts {
		if strings.TrimSpace(c.ID) == "" || contexts[c.ID] || !allowed(c.Audience, "private", "organization", "federated", "public", "customer") || !listOK(c.Uses, true) || !listOK(c.Licenses, true) || !listOK(c.OwnerIDs, false) {
			return false
		}
		contexts[c.ID] = true
	}
	for _, l := range in.Links {
		if !allowed(l.Kind, "contributor_pathway", "agent_contract", "package", "release", "contribution_boundary") || strings.TrimSpace(l.Reference) == "" || (l.Kind == "contribution_boundary" && !allowed(l.Boundary, "private", "federated")) {
			return false
		}
	}
	seen := map[string]bool{}
	for _, x := range in.Exceptions {
		if x.ID == "" || seen[x.ID] || !listOK(x.MaterialKinds, true) || !listOK(x.ContextIDs, false) || x.Rationale == "" || x.OwnerID == "" || x.ApprovedBy == "" || x.ExpiresAt.IsZero() {
			return false
		}
		seen[x.ID] = true
		for _, k := range x.MaterialKinds {
			if !kinds[k] {
				return false
			}
		}
		for _, c := range x.ContextIDs {
			if !contexts[c] {
				return false
			}
		}
	}
	return listOK(in.OwnerIDs, false)
}
func derive(v Version, now time.Time) []Blocker {
	out := []Blocker{}
	add := func(k, s, d, a string) { out = append(out, Blocker{k, s, d, a}) }
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "policy has no accountable owner", v.AuthorID)
	}
	licenses := map[string]string{}
	for _, r := range v.Rules {
		if len(r.ReviewOwnerIDs) == 0 {
			add("missing_owner", r.Kind, "material rule has no review owner", v.AuthorID)
		}
		for _, l := range r.Licenses {
			n := strings.ToLower(strings.TrimSpace(l))
			if n == "unknown" || n == "unidentified" || n == "other" {
				add("unknown_license", r.Kind, "license must be identified before acceptance", v.AuthorID)
			}
			if prior, ok := licenses[r.Kind+"\x00"+n]; ok && prior != strings.Join(r.Uses, "\x00") {
				add("conflicting_terms", r.Kind+":"+l, "same license has conflicting permitted uses", v.AuthorID)
			} else {
				licenses[r.Kind+"\x00"+n] = strings.Join(r.Uses, "\x00")
			}
		}
	}
	for _, c := range v.DistributionContexts {
		if len(c.OwnerIDs) == 0 {
			add("missing_owner", c.ID, "distribution context has no accountable owner", v.AuthorID)
		}
		for _, l := range c.Licenses {
			found := false
			for _, r := range v.Rules {
				for _, p := range r.Licenses {
					if strings.EqualFold(p, l) {
						found = true
					}
				}
			}
			if !found {
				add("conflicting_terms", c.ID+":"+l, "distribution license is not permitted by any material rule", v.AuthorID)
			}
		}
	}
	for _, x := range v.Exceptions {
		if !x.ExpiresAt.After(now) {
			add("expired_exception", x.ID, x.Rationale, x.OwnerID)
		} else if x.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			add("expiring_exception", x.ID, x.ExpiresAt.UTC().Format(time.RFC3339), x.OwnerID)
		}
	}
	return out
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(kind, scope, idv string) string {
	return filepath.Join(s.root, kind, scope, idv+".json")
}
func (s *Store) save(x Policy) error {
	p := s.path(x.ScopeKind, x.ScopeID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(p, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(kind, scope, actor string, in Input) (Policy, error) {
	if !allowed(kind, "repository", "organization") || scope == "" || actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(kind, scope)
	if e != nil {
		return Policy{}, e
	}
	for _, x := range xs {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Policy{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	x := Policy{id(), kind, scope, 1, []Version{v}, derive(v, s.now().UTC())}
	return x, s.save(x)
}
func (s *Store) Revise(kind, scope, idv, actor string, expected int64, in Input) (Policy, error) {
	if actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(kind, scope, idv)
	if e != nil {
		return Policy{}, e
	}
	if x.CurrentVersion != expected {
		return Policy{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Blockers = derive(v, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Get(kind, scope, idv string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(kind, scope, idv)
}
func (s *Store) Catalog(kind, scope string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(kind, scope)
	return Catalog{x}, e
}
func (s *Store) read(kind, scope, idv string) (Policy, error) {
	b, e := os.ReadFile(s.path(kind, scope, idv))
	if errors.Is(e, fs.ErrNotExist) {
		return Policy{}, ErrNotFound
	}
	var x Policy
	if e != nil || json.Unmarshal(b, &x) != nil || x.ScopeKind != kind || x.ScopeID != scope || x.ID != idv {
		return Policy{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.Blockers = derive(x.Versions[len(x.Versions)-1], s.now().UTC())
	}
	return x, nil
}
func (s *Store) list(kind, scope string) ([]Policy, error) {
	es, e := os.ReadDir(filepath.Join(s.root, kind, scope))
	if errors.Is(e, fs.ErrNotExist) {
		return []Policy{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Policy{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(kind, scope, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].PublishedAt.After(out[j].Versions[len(out[j].Versions)-1].PublishedAt)
	})
	return out, nil
}
