// Package securityexpectations owns versioned security intent and trust boundaries.
package securityexpectations

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

var ErrNotFound = errors.New("security expectation not found")
var ErrInvalid = errors.New("invalid security expectation")
var ErrConflict = errors.New("security expectation version conflict")

type Scope struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
}
type Asset struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Classification string   `json:"classification"`
	OwnerIDs       []string `json:"owner_ids"`
	Protection     string   `json:"protection"`
}
type Boundary struct {
	ID             string   `json:"id"`
	From           string   `json:"from"`
	To             string   `json:"to"`
	AssetIDs       []string `json:"asset_ids"`
	Allowed        bool     `json:"allowed"`
	Authentication string   `json:"authentication,omitempty"`
	Rationale      string   `json:"rationale"`
	OwnerIDs       []string `json:"owner_ids"`
}
type Actor struct {
	ID           string   `json:"id"`
	Description  string   `json:"description"`
	Trusted      bool     `json:"trusted"`
	Capabilities []string `json:"capabilities"`
}
type AbuseCase struct {
	ID          string   `json:"id"`
	ActorID     string   `json:"actor_id"`
	AssetIDs    []string `json:"asset_ids"`
	BoundaryIDs []string `json:"boundary_ids"`
	Description string   `json:"description"`
	Impact      string   `json:"impact"`
	Severity    string   `json:"severity"`
	ControlIDs  []string `json:"control_ids"`
	OwnerIDs    []string `json:"owner_ids"`
}
type Control struct {
	ID                string   `json:"id"`
	Description       string   `json:"description"`
	Guarantee         string   `json:"guarantee"`
	Supported         bool     `json:"supported"`
	UnsupportedReason string   `json:"unsupported_reason,omitempty"`
	OwnerIDs          []string `json:"owner_ids"`
}
type SeverityRule struct {
	Severity        string `json:"severity"`
	Response        string `json:"response"`
	ReleaseBlocking bool   `json:"release_blocking"`
}
type Link struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision,omitempty"`
	Commitment string `json:"commitment"`
}
type Exception struct {
	ID         string    `json:"id"`
	Subject    string    `json:"subject"`
	Rationale  string    `json:"rationale"`
	OwnerID    string    `json:"owner_id"`
	ApprovedBy string    `json:"approved_by"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Input struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Scopes         []Scope        `json:"scopes"`
	Assets         []Asset        `json:"protected_assets"`
	Boundaries     []Boundary     `json:"trust_boundaries"`
	Actors         []Actor        `json:"actors"`
	AbuseCases     []AbuseCase    `json:"abuse_cases"`
	Controls       []Control      `json:"required_controls"`
	OwnerIDs       []string       `json:"owner_ids"`
	SeverityPolicy []SeverityRule `json:"severity_policy"`
	Exceptions     []Exception    `json:"permitted_exceptions"`
	Links          []Link         `json:"commitment_links"`
	ChangeReason   string         `json:"change_reason"`
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
type Expectation struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
}
type Catalog struct {
	Items []Expectation `json:"items"`
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
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func unique(id string, seen map[string]bool) bool {
	id = strings.TrimSpace(id)
	if id == "" || seen[id] {
		return false
	}
	seen[id] = true
	return true
}
func refs(xs []string, known map[string]bool) bool {
	for _, x := range xs {
		if !known[x] {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || len(in.Scopes) == 0 || len(in.Assets) == 0 || len(in.Boundaries) == 0 || len(in.Actors) == 0 || len(in.Controls) == 0 || len(in.SeverityPolicy) == 0 {
		return false
	}
	for _, x := range in.Scopes {
		if !allowed(x.Kind, "repository", "service", "interface", "package", "extension", "environment", "user_journey") || x.Reference == "" {
			return false
		}
	}
	assets, actors, bounds, controls, abuses := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Assets {
		if !unique(x.ID, assets) || x.Name == "" || x.Protection == "" || !allowed(x.Classification, "public", "internal", "confidential", "restricted", "critical") {
			return false
		}
	}
	for _, x := range in.Actors {
		if !unique(x.ID, actors) || x.Description == "" || len(x.Capabilities) == 0 {
			return false
		}
	}
	for _, x := range in.Controls {
		if !unique(x.ID, controls) || x.Description == "" || x.Guarantee == "" || (x.Supported && x.UnsupportedReason != "") || (!x.Supported && x.UnsupportedReason == "") {
			return false
		}
	}
	for _, x := range in.Boundaries {
		if !unique(x.ID, bounds) || x.From == "" || x.To == "" || x.Rationale == "" || !refs(x.AssetIDs, assets) {
			return false
		}
	}
	for _, x := range in.AbuseCases {
		if !unique(x.ID, abuses) || !actors[x.ActorID] || x.Description == "" || x.Impact == "" || !allowed(x.Severity, "low", "medium", "high", "critical") || !refs(x.AssetIDs, assets) || !refs(x.BoundaryIDs, bounds) || !refs(x.ControlIDs, controls) {
			return false
		}
	}
	seenSeverity := map[string]bool{}
	for _, x := range in.SeverityPolicy {
		if !allowed(x.Severity, "low", "medium", "high", "critical") || x.Response == "" || seenSeverity[x.Severity] {
			return false
		}
		seenSeverity[x.Severity] = true
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || x.Subject == "" || x.Rationale == "" || x.OwnerID == "" || x.ApprovedBy == "" || x.ExpiresAt.IsZero() {
			return false
		}
	}
	for _, x := range in.Links {
		if !allowed(x.Kind, "design", "privacy", "infrastructure", "api", "quality", "release") || x.Reference == "" || x.Commitment == "" {
			return false
		}
	}
	return true
}
func derive(v Version, now time.Time) []Gap {
	out := []Gap{}
	add := func(k, s, d, a string) { out = append(out, Gap{k, s, d, a}) }
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "expectation has no accountable owner", v.AuthorID)
	}
	for _, a := range v.Assets {
		if len(a.OwnerIDs) == 0 {
			add("missing_owner", a.ID, "protected asset has no accountable owner", v.AuthorID)
		}
	}
	for _, c := range v.Controls {
		if len(c.OwnerIDs) == 0 {
			add("missing_owner", c.ID, "required control has no accountable owner", v.AuthorID)
		}
		if !c.Supported {
			add("unsupported_guarantee", c.ID, c.UnsupportedReason, v.AuthorID)
		}
	}
	for _, a := range v.AbuseCases {
		if len(a.OwnerIDs) == 0 {
			add("missing_owner", a.ID, "abuse case has no accountable owner", v.AuthorID)
		}
	}
	type claim struct {
		allowed bool
		id      string
	}
	claims := map[string]claim{}
	for _, b := range v.Boundaries {
		if len(b.OwnerIDs) == 0 {
			add("missing_owner", b.ID, "trust boundary has no accountable owner", v.AuthorID)
		}
		key := strings.ToLower(strings.TrimSpace(b.From)) + "\x00" + strings.ToLower(strings.TrimSpace(b.To))
		if p, ok := claims[key]; ok && p.allowed != b.Allowed {
			add("contradictory_boundary", b.ID, "conflicts with "+p.id+" for the same crossing", v.AuthorID)
		} else {
			claims[key] = claim{b.Allowed, b.ID}
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
func id() string                             { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Expectation) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Expectation, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Expectation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Expectation{}, e
	}
	for _, x := range xs {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Expectation{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	x := Expectation{id(), repo, 1, []Version{v}, derive(v, s.now().UTC())}
	return x, s.save(x)
}
func (s *Store) Revise(repo, idv, actor string, expected int64, in Input) (Expectation, error) {
	if actor == "" || !valid(in) {
		return Expectation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, idv)
	if e != nil {
		return Expectation{}, e
	}
	if x.CurrentVersion != expected {
		return Expectation{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Get(repo, idv string) (Expectation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, idv)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
}
func (s *Store) read(repo, idv string) (Expectation, error) {
	b, e := os.ReadFile(s.path(repo, idv))
	if errors.Is(e, fs.ErrNotExist) {
		return Expectation{}, ErrNotFound
	}
	var x Expectation
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != idv {
		return Expectation{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.Gaps = derive(x.Versions[len(x.Versions)-1], s.now().UTC())
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Expectation, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Expectation{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Expectation{}
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
