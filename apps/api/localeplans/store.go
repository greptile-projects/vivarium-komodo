// Package localeplans owns versioned repository localization contracts.
package localeplans

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
	ErrNotFound = errors.New("locale plan not found")
	ErrInvalid  = errors.New("invalid locale plan")
	ErrConflict = errors.New("locale plan version conflict")
)

type Scope struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Name       string `json:"name"`
}
type Locale struct {
	ID                string   `json:"id"`
	Language          string   `json:"language"`
	Region            string   `json:"region,omitempty"`
	FallbackLocaleIDs []string `json:"fallback_locale_ids,omitempty"`
}
type Term struct {
	Concept   string   `json:"concept"`
	LocaleID  string   `json:"locale_id"`
	Preferred string   `json:"preferred"`
	Avoid     []string `json:"avoid,omitempty"`
	Context   string   `json:"context,omitempty"`
}
type Format struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
}
type Journey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	LocaleIDs   []string `json:"locale_ids"`
	OwnerIDs    []string `json:"owner_ids"`
	ReviewerIDs []string `json:"reviewer_ids"`
}
type Threshold struct {
	LocaleID           string   `json:"locale_id"`
	MinimumPercent     int      `json:"minimum_percent"`
	RequiredJourneyIDs []string `json:"required_journey_ids"`
	RequiredFormatIDs  []string `json:"required_format_ids"`
}
type Resource struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Path           string   `json:"path"`
	SourceRevision string   `json:"source_revision"`
	FormatID       string   `json:"format_id"`
	JourneyIDs     []string `json:"journey_ids"`
	OwnerIDs       []string `json:"owner_ids"`
}
type VersionInput struct {
	Title        string      `json:"title"`
	Scopes       []Scope     `json:"scopes"`
	Locales      []Locale    `json:"locales"`
	Terms        []Term      `json:"terminology"`
	Formats      []Format    `json:"formatting_requirements"`
	Journeys     []Journey   `json:"journeys"`
	OwnerIDs     []string    `json:"owner_ids"`
	ReviewerIDs  []string    `json:"reviewer_ids"`
	Thresholds   []Threshold `json:"release_thresholds"`
	Resources    []Resource  `json:"resources"`
	ChangeReason string      `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type CoverageInput struct {
	Version        int64  `json:"version"`
	ResourceID     string `json:"resource_id"`
	LocaleID       string `json:"locale_id"`
	JourneyID      string `json:"journey_id"`
	SourceRevision string `json:"source_revision"`
	Percent        int    `json:"percent"`
	Status         string `json:"status"`
	Evidence       string `json:"evidence"`
}
type Coverage struct {
	ID string `json:"id"`
	CoverageInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind       string `json:"kind"`
	LocaleID   string `json:"locale_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	JourneyID  string `json:"journey_id,omitempty"`
	Detail     string `json:"detail"`
}
type Plan struct {
	ID             string     `json:"id"`
	RepositoryID   string     `json:"repository_id"`
	CurrentVersion int64      `json:"current_version"`
	Versions       []Version  `json:"versions"`
	Coverage       []Coverage `json:"coverage"`
	Blockers       []Blocker  `json:"blockers"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func stringsOK(x []string, required bool) bool {
	if required && len(x) == 0 {
		return false
	}
	for _, v := range x {
		if strings.TrimSpace(v) == "" {
			return false
		}
	}
	return len(x) <= 100
}
func valid(in VersionInput) bool {
	if in.Title == "" || in.ChangeReason == "" || len(in.Scopes) == 0 || len(in.Locales) == 0 || len(in.Journeys) == 0 || len(in.Resources) == 0 || !stringsOK(in.OwnerIDs, false) || !stringsOK(in.ReviewerIDs, false) {
		return false
	}
	locales := map[string]bool{}
	formats := map[string]bool{}
	journeys := map[string]bool{}
	resources := map[string]bool{}
	for _, x := range in.Scopes {
		if !map[string]bool{"repository": true, "product": true, "documentation": true, "release": true}[x.Kind] || x.Name == "" {
			return false
		}
	}
	for _, x := range in.Locales {
		if x.ID == "" || x.Language == "" || locales[x.ID] {
			return false
		}
		locales[x.ID] = true
	}
	for _, x := range in.Locales {
		for _, f := range x.FallbackLocaleIDs {
			if !locales[f] || f == x.ID {
				return false
			}
		}
	}
	for _, x := range in.Formats {
		if x.ID == "" || x.Description == "" || formats[x.ID] {
			return false
		}
		formats[x.ID] = true
	}
	for _, x := range in.Journeys {
		if x.ID == "" || x.Name == "" || journeys[x.ID] || !stringsOK(x.LocaleIDs, true) {
			return false
		}
		journeys[x.ID] = true
		for _, l := range x.LocaleIDs {
			if !locales[l] {
				return false
			}
		}
	}
	for _, x := range in.Resources {
		if x.ID == "" || x.Kind == "" || x.Path == "" || x.SourceRevision == "" || resources[x.ID] || !formats[x.FormatID] || !stringsOK(x.JourneyIDs, true) || !stringsOK(x.OwnerIDs, false) {
			return false
		}
		resources[x.ID] = true
		for _, j := range x.JourneyIDs {
			if !journeys[j] {
				return false
			}
		}
	}
	for _, x := range in.Terms {
		if x.Concept == "" || x.Preferred == "" || !locales[x.LocaleID] {
			return false
		}
	}
	for _, x := range in.Thresholds {
		if !locales[x.LocaleID] || x.MinimumPercent < 0 || x.MinimumPercent > 100 {
			return false
		}
		for _, j := range x.RequiredJourneyIDs {
			if !journeys[j] {
				return false
			}
		}
		for _, f := range x.RequiredFormatIDs {
			if !formats[f] {
				return false
			}
		}
	}
	return true
}
func (s *Store) path(repo, p string) string { return filepath.Join(s.root, repo, p+".json") }
func (s *Store) save(p Plan) error {
	if e := os.MkdirAll(filepath.Dir(s.path(p.RepositoryID, p.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path(p.RepositoryID, p.ID), b, 0640)
}
func (s *Store) raw(repo, p string) (Plan, error) {
	var x Plan
	b, e := os.ReadFile(s.path(repo, p))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Create(repo, actor string, in VersionInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	p := Plan{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}}}
	derive(&p)
	return p, s.save(p)
}
func (s *Store) Revise(repo, p, actor string, expected int64, in VersionInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	x, e := s.raw(repo, p)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return x, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	derive(&x)
	return x, s.save(x)
}
func (s *Store) RecordCoverage(repo, p, actor string, in CoverageInput) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.raw(repo, p)
	if e != nil {
		return x, e
	}
	if actor == "" || in.Version < 1 || in.Version > x.CurrentVersion || in.SourceRevision == "" || strings.TrimSpace(in.Evidence) == "" || in.Percent < 0 || in.Percent > 100 || !map[string]bool{"complete": true, "partial": true, "missing": true, "unsupported": true}[in.Status] {
		return x, ErrInvalid
	}
	v := x.Versions[in.Version-1]
	okR, okL, okJ := false, false, false
	for _, r := range v.Resources {
		okR = okR || r.ID == in.ResourceID
	}
	for _, l := range v.Locales {
		okL = okL || l.ID == in.LocaleID
	}
	for _, j := range v.Journeys {
		okJ = okJ || j.ID == in.JourneyID
	}
	if !okR || !okL || !okJ {
		return x, ErrInvalid
	}
	x.Coverage = append(x.Coverage, Coverage{ID: id(), CoverageInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	derive(&x)
	return x, s.save(x)
}
func (s *Store) Get(repo, p string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.raw(repo, p)
	if e == nil {
		derive(&x)
	}
	return x, e
}
func (s *Store) List(repo string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Plan{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Plan{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		p, e := s.raw(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		derive(&p)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Versions[0].CreatedAt.After(out[j].Versions[0].CreatedAt) })
	return out, nil
}
func derive(p *Plan) {
	p.Blockers = nil
	v := p.Versions[len(p.Versions)-1]
	if len(v.OwnerIDs) == 0 {
		p.Blockers = append(p.Blockers, Blocker{Kind: "missing_ownership", Detail: "plan has no accountable owner"})
	}
	for _, j := range v.Journeys {
		if len(j.OwnerIDs) == 0 {
			p.Blockers = append(p.Blockers, Blocker{Kind: "missing_ownership", JourneyID: j.ID, Detail: "journey has no owner"})
		}
	}
	for _, r := range v.Resources {
		if len(r.OwnerIDs) == 0 {
			p.Blockers = append(p.Blockers, Blocker{Kind: "missing_ownership", ResourceID: r.ID, Detail: "resource has no owner"})
		}
		for _, f := range v.Formats {
			if f.ID == r.FormatID && !f.Supported {
				p.Blockers = append(p.Blockers, Blocker{Kind: "unsupported_format", ResourceID: r.ID, Detail: r.FormatID + " is unsupported"})
			}
		}
	}
	seen := map[string]string{}
	for _, t := range v.Terms {
		k := t.LocaleID + ":" + strings.ToLower(t.Concept)
		if old, ok := seen[k]; ok && old != t.Preferred {
			p.Blockers = append(p.Blockers, Blocker{Kind: "conflicting_terminology", LocaleID: t.LocaleID, Detail: t.Concept + " has conflicting preferred terms"})
		}
		seen[k] = t.Preferred
	}
	latest := map[string]Coverage{}
	for _, c := range p.Coverage {
		if c.Version == v.Number {
			latest[c.ResourceID+":"+c.LocaleID+":"+c.JourneyID] = c
		}
	}
	for _, r := range v.Resources {
		for _, j := range r.JourneyIDs {
			var journey Journey
			for _, x := range v.Journeys {
				if x.ID == j {
					journey = x
				}
			}
			for _, l := range journey.LocaleIDs {
				k := r.ID + ":" + l + ":" + j
				c, ok := latest[k]
				if !ok {
					p.Blockers = append(p.Blockers, Blocker{Kind: "missing_coverage", ResourceID: r.ID, LocaleID: l, JourneyID: j, Detail: "required locale journey has no coverage"})
					continue
				}
				if c.SourceRevision != r.SourceRevision {
					p.Blockers = append(p.Blockers, Blocker{Kind: "stale_coverage", ResourceID: r.ID, LocaleID: l, JourneyID: j, Detail: "coverage does not match the exact source revision"})
				}
			}
		}
		for _, t := range v.Thresholds {
			for k, c := range latest {
				_ = k
				if c.LocaleID == t.LocaleID && c.Percent < t.MinimumPercent {
					p.Blockers = append(p.Blockers, Blocker{Kind: "release_threshold_unmet", ResourceID: c.ResourceID, LocaleID: c.LocaleID, JourneyID: c.JourneyID, Detail: "coverage is below the declared release threshold"})
				}
			}
		}
	}
}
