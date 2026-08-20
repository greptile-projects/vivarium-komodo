// Package capabilityinventories owns revision-exact maps of product capabilities and their consumers.
package capabilityinventories

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

var ErrNotFound = errors.New("capability inventory not found")
var ErrInvalid = errors.New("invalid capability inventory")
var ErrConflict = errors.New("capability inventory version conflict")

type Element struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Reference   string   `json:"reference"`
	Revision    string   `json:"revision"`
	OwnerIDs    []string `json:"owner_ids"`
	Description string   `json:"description"`
}
type Consumer struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Reference      string   `json:"reference"`
	Revision       string   `json:"revision,omitempty"`
	Status         string   `json:"status"`
	OwnerIDs       []string `json:"owner_ids"`
	EnvironmentIDs []string `json:"environment_ids"`
	ElementIDs     []string `json:"element_ids"`
	Discovery      string   `json:"discovery"`
	Audience       string   `json:"audience"`
	Detail         string   `json:"detail,omitempty"`
}
type Environment struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Revision string   `json:"revision"`
	OwnerIDs []string `json:"owner_ids"`
}
type UsageEvidence struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Reference      string     `json:"reference"`
	Revision       string     `json:"revision,omitempty"`
	ConsumerIDs    []string   `json:"consumer_ids"`
	ElementIDs     []string   `json:"element_ids"`
	EnvironmentIDs []string   `json:"environment_ids"`
	Status         string     `json:"status"`
	ObservedAt     time.Time  `json:"observed_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	AuthorID       string     `json:"author_id"`
	Detail         string     `json:"detail,omitempty"`
}
type CompatibilityPromise struct {
	ID          string     `json:"id"`
	Scope       string     `json:"scope"`
	Revision    string     `json:"revision"`
	ConsumerIDs []string   `json:"consumer_ids"`
	OwnerIDs    []string   `json:"owner_ids"`
	Guarantee   string     `json:"guarantee"`
	Until       *time.Time `json:"until,omitempty"`
}
type Input struct {
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	SourceRevision        string                 `json:"source_revision"`
	DefinitionPath        string                 `json:"definition_path"`
	Elements              []Element              `json:"elements"`
	Consumers             []Consumer             `json:"consumers"`
	Environments          []Environment          `json:"environments"`
	UsageEvidence         []UsageEvidence        `json:"usage_evidence"`
	CompatibilityPromises []CompatibilityPromise `json:"compatibility_promises"`
	OwnerIDs              []string               `json:"owner_ids"`
	DiscoveryNotes        []string               `json:"discovery_notes"`
	ChangeReason          string                 `json:"change_reason"`
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
	AttributedTo string `json:"attributed_to,omitempty"`
}
type Inventory struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
	RemovalReady   bool      `json:"removal_ready"`
}
type Catalog struct {
	Items []Inventory `json:"items"`
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
func identifier() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func unique(v string, seen map[string]bool) bool {
	v = strings.TrimSpace(v)
	if v == "" || seen[v] {
		return false
	}
	seen[v] = true
	return true
}
func refsOK(ids []string, known map[string]bool) bool {
	for _, id := range ids {
		if !known[id] {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || in.Description == "" || in.SourceRevision == "" || in.DefinitionPath == "" || in.ChangeReason == "" || len(in.Elements) == 0 || len(in.OwnerIDs) == 0 {
		return false
	}
	elements, consumers, envs, evidence, promises := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Elements {
		if !unique(x.ID, elements) || !allowed(x.Kind, "interface", "symbol", "flag", "package", "schema", "configuration", "documentation", "journey", "release") || x.Reference == "" || x.Revision == "" || x.Description == "" {
			return false
		}
	}
	for _, x := range in.Environments {
		if !unique(x.ID, envs) || x.Name == "" || x.Revision == "" {
			return false
		}
	}
	for _, x := range in.Consumers {
		if !unique(x.ID, consumers) || !allowed(x.Kind, "repository", "service", "package", "application", "extension", "user_journey", "external") || x.Reference == "" || !allowed(x.Status, "active", "inactive", "unknown", "inaccessible", "dynamic") || !allowed(x.Discovery, "declared", "static", "observed", "dynamic", "inaccessible", "unknown") || !allowed(x.Audience, "owners_only", "repository", "public", "embargoed") || !refsOK(x.ElementIDs, elements) || !refsOK(x.EnvironmentIDs, envs) {
			return false
		}
	}
	for _, x := range in.UsageEvidence {
		if !unique(x.ID, evidence) || !allowed(x.Kind, "code_search", "dependency_graph", "telemetry", "survey", "owner_attestation", "contract", "runtime_discovery") || x.Reference == "" || !allowed(x.Status, "current", "stale", "unknown", "inaccessible", "dynamic") || (x.Status == "current" && x.Revision == "") || x.AuthorID == "" || x.ObservedAt.IsZero() || !refsOK(x.ConsumerIDs, consumers) || !refsOK(x.ElementIDs, elements) || !refsOK(x.EnvironmentIDs, envs) {
			return false
		}
	}
	for _, x := range in.CompatibilityPromises {
		if !unique(x.ID, promises) || x.Scope == "" || x.Revision == "" || x.Guarantee == "" || len(x.OwnerIDs) == 0 || !refsOK(x.ConsumerIDs, consumers) {
			return false
		}
	}
	return true
}
func derive(v Version, now time.Time) []Gap {
	out := []Gap{}
	add := func(k, s, d, a string) { out = append(out, Gap{k, s, d, a}) }
	covered := map[string]bool{}
	consumerEvidence := map[string]bool{}
	environmentEvidence := map[string]bool{}
	for _, e := range v.UsageEvidence {
		current := e.Status == "current" && (e.ExpiresAt == nil || e.ExpiresAt.After(now))
		if current {
			for _, id := range e.ElementIDs {
				covered[id] = true
			}
			for _, id := range e.ConsumerIDs {
				consumerEvidence[id] = true
			}
			for _, id := range e.EnvironmentIDs {
				environmentEvidence[id] = true
			}
		} else {
			detail := e.Detail
			if detail == "" {
				detail = "usage evidence is " + e.Status
			}
			add(e.Status+"_usage_evidence", e.ID, detail, e.AuthorID)
		}
		if e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
			add("expired_usage_evidence", e.ID, "usage observation has expired", e.AuthorID)
		}
	}
	for _, x := range v.Elements {
		if len(x.OwnerIDs) == 0 {
			add("missing_owner", x.ID, "capability element has no accountable owner", v.AuthorID)
		}
		if !covered[x.ID] {
			add("missing_usage_evidence", x.ID, "no current usage evidence covers this element", v.AuthorID)
		}
	}
	for _, x := range v.Environments {
		if len(x.OwnerIDs) == 0 {
			add("missing_environment_owner", x.ID, "environment has no accountable owner", v.AuthorID)
		}
		if !environmentEvidence[x.ID] {
			add("missing_environment_usage_evidence", x.ID, "no current observation covers this environment", v.AuthorID)
		}
	}
	for _, x := range v.Consumers {
		if len(x.OwnerIDs) == 0 {
			add("missing_consumer_owner", x.ID, "consumer has no accountable owner", v.AuthorID)
		}
		if x.Status == "unknown" || x.Status == "inaccessible" || x.Status == "dynamic" {
			add(x.Status+"_consumer", x.ID, x.Detail, v.AuthorID)
		}
		if x.Discovery == "unknown" || x.Discovery == "inaccessible" || x.Discovery == "dynamic" {
			add(x.Discovery+"_discovery", x.ID, "consumer discovery remains "+x.Discovery, v.AuthorID)
		}
		if !consumerEvidence[x.ID] {
			add("unverified_consumer_status", x.ID, "no current evidence verifies this consumer status", v.AuthorID)
		}
	}
	if len(v.Consumers) == 0 {
		add("unknown_consumers", v.Name, "no consumer boundary has been declared", v.AuthorID)
	}
	for _, x := range v.CompatibilityPromises {
		if x.Until != nil && !x.Until.After(now) {
			add("expired_compatibility_promise", x.ID, x.Guarantee, x.OwnerIDs[0])
		}
	}
	return out
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Inventory) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Inventory, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Inventory{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.list(repo)
	if e != nil {
		return Inventory{}, e
	}
	for _, x := range items {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Inventory{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	g := derive(v, s.now().UTC())
	x := Inventory{identifier(), repo, 1, []Version{v}, g, len(g) == 0}
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (Inventory, error) {
	if actor == "" || !valid(in) {
		return Inventory{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return Inventory{}, e
	}
	if x.CurrentVersion != expected {
		return Inventory{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v, s.now().UTC())
	x.RemovalReady = len(x.Gaps) == 0
	return x, s.save(x)
}
func (s *Store) Get(repo, id string) (Inventory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
}
func (s *Store) read(repo, id string) (Inventory, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Inventory{}, ErrNotFound
	}
	var x Inventory
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Inventory{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.Gaps = derive(x.Versions[len(x.Versions)-1], s.now().UTC())
		x.RemovalReady = len(x.Gaps) == 0
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Inventory, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Inventory{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Inventory{}
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
		return out[i].Versions[len(out[i].Versions)-1].PublishedAt.After(out[j].Versions[len(out[j].Versions)-1].PublishedAt)
	})
	return out, nil
}
