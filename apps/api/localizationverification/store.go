// Package localizationverification retains exact-input localized experience evidence.
package localizationverification

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

var ErrNotFound = errors.New("localization verification not found")
var ErrInvalid = errors.New("invalid localization verification")
var ErrForbidden = errors.New("localization verification forbidden")

type Check struct {
	Name              string    `json:"name"`
	Kind              string    `json:"kind"`
	LocaleID          string    `json:"locale_id"`
	JourneyID         string    `json:"journey_id,omitempty"`
	Route             string    `json:"route,omitempty"`
	Status            string    `json:"status"`
	Summary           string    `json:"summary"`
	SourceDigest      string    `json:"source_digest"`
	TranslationDigest string    `json:"translation_digest"`
	InterfaceDigest   string    `json:"interface_digest"`
	Command           string    `json:"command,omitempty"`
	UnitIDs           []string  `json:"unit_ids,omitempty"`
	InterfacePaths    []string  `json:"interface_paths,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
type Preview struct {
	ID             string    `json:"id"`
	PreviewID      string    `json:"preview_id"`
	LocaleID       string    `json:"locale_id"`
	RouteAllowlist []string  `json:"route_allowlist"`
	Revision       string    `json:"revision"`
	URL            string    `json:"url"`
	ReviewerIDs    []string  `json:"reviewer_ids"`
	CreatedByID    string    `json:"created_by_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type Finding struct {
	ID                string    `json:"id"`
	PreviewID         string    `json:"locale_preview_id"`
	LocaleID          string    `json:"locale_id"`
	Route             string    `json:"route"`
	Revision          string    `json:"revision"`
	UnitIDs           []string  `json:"unit_ids,omitempty"`
	InterfacePaths    []string  `json:"interface_paths,omitempty"`
	Kind              string    `json:"kind"`
	Severity          string    `json:"severity"`
	Body              string    `json:"body"`
	ActorID           string    `json:"actor_id"`
	SourceDigest      string    `json:"source_digest"`
	TranslationDigest string    `json:"translation_digest"`
	InterfaceDigest   string    `json:"interface_digest"`
	CreatedAt         time.Time `json:"created_at"`
	Stale             bool      `json:"stale"`
}
type Decision struct {
	ID                string    `json:"id"`
	PreviewID         string    `json:"locale_preview_id"`
	LocaleID          string    `json:"locale_id"`
	Route             string    `json:"route"`
	Revision          string    `json:"revision"`
	Decision          string    `json:"decision"`
	Rationale         string    `json:"rationale"`
	ReviewerID        string    `json:"reviewer_id"`
	SourceDigest      string    `json:"source_digest"`
	TranslationDigest string    `json:"translation_digest"`
	InterfaceDigest   string    `json:"interface_digest"`
	CreatedAt         time.Time `json:"created_at"`
	Stale             bool      `json:"stale"`
}
type Assessment struct {
	ID            string     `json:"id"`
	RepositoryID  string     `json:"repository_id"`
	PullRequestID string     `json:"pull_request_id"`
	Revision      string     `json:"revision"`
	ConfigPath    string     `json:"config_path"`
	ConfigBlobID  string     `json:"config_blob_id"`
	Checks        []Check    `json:"checks"`
	Previews      []Preview  `json:"previews,omitempty"`
	Findings      []Finding  `json:"findings,omitempty"`
	Decisions     []Decision `json:"decisions,omitempty"`
	CreatedByID   string     `json:"created_by_id"`
	CreatedAt     time.Time  `json:"created_at"`
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
func id() string                               { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) load(repo, pull string) (Assessment, error) {
	var a Assessment
	b, e := os.ReadFile(s.path(repo, pull))
	if os.IsNotExist(e) {
		return a, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	return a, e
}
func (s *Store) save(a Assessment) error {
	if e := os.MkdirAll(filepath.Dir(s.path(a.RepositoryID, a.PullRequestID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(a.RepositoryID, a.PullRequestID), b, 0640)
	}
	return e
}
func (s *Store) Put(repo, pull, revision, configPath, configBlob, actor string, checks []Check) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || pull == "" || revision == "" || configBlob == "" || actor == "" || len(checks) == 0 {
		return Assessment{}, ErrInvalid
	}
	old, _ := s.load(repo, pull)
	now := s.now().UTC()
	for i := range checks {
		if !validKind(checks[i].Kind) || checks[i].Name == "" || checks[i].LocaleID == "" || !map[string]bool{"passed": true, "failed": true}[checks[i].Status] || checks[i].SourceDigest == "" || checks[i].TranslationDigest == "" || checks[i].InterfaceDigest == "" {
			return Assessment{}, ErrInvalid
		}
		checks[i].CreatedAt = now
	}
	a := Assessment{ID: id(), RepositoryID: repo, PullRequestID: pull, Revision: revision, ConfigPath: configPath, ConfigBlobID: configBlob, Checks: checks, Previews: old.Previews, Findings: old.Findings, Decisions: old.Decisions, CreatedByID: actor, CreatedAt: now}
	derive(&a)
	return a, s.save(a)
}
func validKind(k string) bool {
	return map[string]bool{"variables": true, "pluralization": true, "formatting": true, "terminology": true, "links": true, "layout_expansion": true, "bidirectional_text": true, "fallback": true, "journey": true}[k]
}
func (s *Store) Get(repo, pull string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(repo, pull)
}
func (s *Store) List(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fs, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, f := range fs {
		var a Assessment
		b, er := os.ReadFile(f)
		if er == nil {
			er = json.Unmarshal(b, &a)
		}
		if er != nil {
			return nil, er
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) AddPreview(repo, pull, actor, preview, locale, revision, url string, routes, reviewers []string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.load(repo, pull)
	if e != nil {
		return a, e
	}
	validRoutes := len(routes) > 0
	for _, route := range routes {
		validRoutes = validRoutes && strings.HasPrefix(route, "/") && !strings.Contains(route, "..")
	}
	if actor == "" || preview == "" || locale == "" || revision != a.Revision || url == "" || !validRoutes {
		return a, ErrInvalid
	}
	p := Preview{ID: id(), PreviewID: preview, LocaleID: locale, RouteAllowlist: routes, Revision: revision, URL: url, ReviewerIDs: reviewers, CreatedByID: actor, CreatedAt: s.now().UTC()}
	a.Previews = append(a.Previews, p)
	return a, s.save(a)
}
func (s *Store) AddFinding(repo, pull, actor, preview, route, kind, severity, body string, units, paths []string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.load(repo, pull)
	if e != nil {
		return a, e
	}
	p, ok := findPreview(a, preview, route)
	if !ok || actor == "" || body == "" || !map[string]bool{"linguistic": true, "cultural": true, "formatting": true, "layout": true, "directionality": true, "fallback": true, "journey": true}[kind] || !map[string]bool{"low": true, "medium": true, "high": true, "blocking": true}[severity] {
		return a, ErrInvalid
	}
	sd, td, ui := digests(a.Checks, p.LocaleID, route, units, paths)
	a.Findings = append(a.Findings, Finding{ID: id(), PreviewID: p.ID, LocaleID: p.LocaleID, Route: route, Revision: a.Revision, UnitIDs: units, InterfacePaths: paths, Kind: kind, Severity: severity, Body: body, ActorID: actor, SourceDigest: sd, TranslationDigest: td, InterfaceDigest: ui, CreatedAt: s.now().UTC()})
	return a, s.save(a)
}
func (s *Store) Decide(repo, pull, actor, preview, route, decision, rationale string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.load(repo, pull)
	if e != nil {
		return a, e
	}
	p, ok := findPreview(a, preview, route)
	if !ok || rationale == "" || !map[string]bool{"approve": true, "reject": true}[decision] {
		return a, ErrInvalid
	}
	allowed := false
	for _, x := range p.ReviewerIDs {
		allowed = allowed || x == actor
	}
	if !allowed {
		return a, ErrForbidden
	}
	sd, td, ui := digests(a.Checks, p.LocaleID, route, nil, nil)
	a.Decisions = append(a.Decisions, Decision{ID: id(), PreviewID: p.ID, LocaleID: p.LocaleID, Route: route, Revision: a.Revision, Decision: decision, Rationale: rationale, ReviewerID: actor, SourceDigest: sd, TranslationDigest: td, InterfaceDigest: ui, CreatedAt: s.now().UTC()})
	return a, s.save(a)
}
func findPreview(a Assessment, idv, route string) (Preview, bool) {
	for _, p := range a.Previews {
		if p.ID == idv {
			for _, r := range p.RouteAllowlist {
				if r == route {
					return p, true
				}
			}
		}
	}
	return Preview{}, false
}
func digests(cs []Check, locale, route string, units, paths []string) (string, string, string) {
	var s, t, u string
	for _, c := range cs {
		if c.LocaleID == locale && (c.Route == "" || c.Route == route) && intersectsOrUnscoped(c.UnitIDs, units) && intersectsOrUnscoped(c.InterfacePaths, paths) {
			s += c.SourceDigest
			t += c.TranslationDigest
			u += c.InterfaceDigest
		}
	}
	return s, t, u
}
func intersectsOrUnscoped(check, evidence []string) bool {
	if len(evidence) == 0 || len(check) == 0 {
		return true
	}
	for _, a := range check {
		for _, b := range evidence {
			if a == b {
				return true
			}
		}
	}
	return false
}
func derive(a *Assessment) {
	for i := range a.Findings {
		sd, td, ui := digests(a.Checks, a.Findings[i].LocaleID, a.Findings[i].Route, a.Findings[i].UnitIDs, a.Findings[i].InterfacePaths)
		a.Findings[i].Stale = sd != a.Findings[i].SourceDigest || td != a.Findings[i].TranslationDigest || ui != a.Findings[i].InterfaceDigest
	}
	for i := range a.Decisions {
		sd, td, ui := digests(a.Checks, a.Decisions[i].LocaleID, a.Decisions[i].Route, nil, nil)
		a.Decisions[i].Stale = sd != a.Decisions[i].SourceDigest || td != a.Decisions[i].TranslationDigest || ui != a.Decisions[i].InterfaceDigest
	}
}
func Clean(xs []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
