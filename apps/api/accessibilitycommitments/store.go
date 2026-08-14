// Package accessibilitycommitments owns versioned, testable accessibility contracts.
package accessibilitycommitments

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
	ErrNotFound = errors.New("accessibility commitment not found")
	ErrInvalid  = errors.New("invalid accessibility commitment")
	ErrConflict = errors.New("accessibility commitment version conflict")
)

type Scope struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Name       string `json:"name"`
}
type Standard struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Level   string `json:"level,omitempty"`
}
type AssistiveTechnology struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Platform string `json:"platform"`
}
type Scenario struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ScopeIDs               []string `json:"scope_ids"`
	StandardIDs            []string `json:"standard_ids"`
	AssistiveTechnologyIDs []string `json:"assistive_technology_ids"`
}
type SeverityRule struct {
	Severity             string `json:"severity"`
	Definition           string `json:"definition"`
	ReviewEffect         string `json:"review_effect"`
	ResolutionTargetDays int    `json:"resolution_target_days,omitempty"`
}
type Exception struct {
	ID          string    `json:"id"`
	ScenarioIDs []string  `json:"scenario_ids"`
	Reason      string    `json:"reason"`
	ApprovedBy  string    `json:"approved_by"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label,omitempty"`
}
type VersionInput struct {
	Title                 string                `json:"title"`
	Scopes                []Scope               `json:"scopes"`
	Standards             []Standard            `json:"standards"`
	AssistiveTechnologies []AssistiveTechnology `json:"assistive_technologies"`
	TargetAudiences       []string              `json:"target_audiences"`
	Scenarios             []Scenario            `json:"required_scenarios"`
	SeverityPolicy        []SeverityRule        `json:"severity_policy"`
	OwnerIDs              []string              `json:"owner_ids"`
	Exceptions            []Exception           `json:"exceptions"`
	Links                 []Link                `json:"links"`
	ChangeReason          string                `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type CoverageInput struct {
	Version               int64  `json:"version"`
	ScenarioID            string `json:"scenario_id"`
	AssistiveTechnologyID string `json:"assistive_technology_id"`
	Status                string `json:"status"`
	Revision              string `json:"revision,omitempty"`
	Evidence              string `json:"evidence"`
	Notes                 string `json:"notes,omitempty"`
}
type Coverage struct {
	ID string `json:"id"`
	CoverageInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind          string `json:"kind"`
	ScenarioID    string `json:"scenario_id,omitempty"`
	EnvironmentID string `json:"environment_id,omitempty"`
	ExceptionID   string `json:"exception_id,omitempty"`
	CommitmentID  string `json:"commitment_id,omitempty"`
	Detail        string `json:"detail"`
}
type Commitment struct {
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
func textList(v []string) bool {
	if len(v) == 0 || len(v) > 100 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if strings.TrimSpace(in.Title) == "" || !textList(in.TargetAudiences) || !textList(in.OwnerIDs) || len(in.Scopes) == 0 || len(in.Standards) == 0 || len(in.AssistiveTechnologies) == 0 || len(in.Scenarios) == 0 || len(in.SeverityPolicy) == 0 || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	scopes, standards, ats, scenarios := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Scopes {
		if !map[string]bool{"repository": true, "journey": true, "component": true, "release": true}[x.Kind] || x.Name == "" {
			return false
		}
		id := x.Kind + ":" + x.ResourceID
		if scopes[id] {
			return false
		}
		scopes[id] = true
	}
	for _, x := range in.Standards {
		if x.ID == "" || x.Name == "" || x.Version == "" || standards[x.ID] {
			return false
		}
		standards[x.ID] = true
	}
	for _, x := range in.AssistiveTechnologies {
		if x.ID == "" || x.Name == "" || x.Platform == "" || ats[x.ID] {
			return false
		}
		ats[x.ID] = true
	}
	for _, x := range in.Scenarios {
		if x.ID == "" || x.Name == "" || scenarios[x.ID] || !textList(x.ScopeIDs) || !textList(x.StandardIDs) || !textList(x.AssistiveTechnologyIDs) {
			return false
		}
		scenarios[x.ID] = true
		for _, id := range x.ScopeIDs {
			if !scopes[id] {
				return false
			}
		}
		for _, id := range x.StandardIDs {
			if !standards[id] {
				return false
			}
		}
		for _, id := range x.AssistiveTechnologyIDs {
			if !ats[id] {
				return false
			}
		}
	}
	for _, x := range in.SeverityPolicy {
		if !map[string]bool{"critical": true, "high": true, "medium": true, "low": true}[x.Severity] || x.Definition == "" || !map[string]bool{"block_review": true, "block_merge": true, "track": true}[x.ReviewEffect] {
			return false
		}
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || x.Reason == "" || x.ApprovedBy == "" || x.ExpiresAt.IsZero() || !textList(x.ScenarioIDs) {
			return false
		}
		for _, id := range x.ScenarioIDs {
			if !scenarios[id] {
				return false
			}
		}
	}
	for _, x := range in.Links {
		if !map[string]bool{"roadmap_outcome": true, "documentation": true, "preview": true, "release_policy": true}[x.Kind] || x.ResourceID == "" {
			return false
		}
	}
	return true
}
func id() string                              { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, cid string) string { return filepath.Join(s.root, repo, cid+".json") }
func (s *Store) save(c Commitment) error {
	d := filepath.Dir(s.path(c.RepositoryID, c.ID))
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path(c.RepositoryID, c.ID), b, 0640)
}
func (s *Store) raw(repo, cid string) (Commitment, error) {
	var c Commitment
	b, e := os.ReadFile(s.path(repo, cid))
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
	now := s.now().UTC()
	c := Commitment{ID: id(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: now}}, Coverage: []Coverage{}}
	return c, s.save(c)
}
func (s *Store) Revise(repo, cid, actor string, expected int64, in VersionInput) (Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || !valid(in) {
		return Commitment{}, ErrInvalid
	}
	c, e := s.raw(repo, cid)
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
func (s *Store) RecordCoverage(repo, cid, actor string, in CoverageInput) (Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.raw(repo, cid)
	if e != nil {
		return c, e
	}
	v := current(c)
	if actor == "" || in.Version != c.CurrentVersion || in.Evidence == "" || !map[string]bool{"passed": true, "failed": true, "unsupported": true, "not_tested": true}[in.Status] {
		return c, ErrInvalid
	}
	foundS, foundA := false, false
	for _, x := range v.Scenarios {
		if x.ID == in.ScenarioID {
			foundS = true
			for _, a := range x.AssistiveTechnologyIDs {
				if a == in.AssistiveTechnologyID {
					foundA = true
				}
			}
		}
	}
	if !foundS || !foundA {
		return c, ErrInvalid
	}
	c.Coverage = append(c.Coverage, Coverage{ID: id(), CoverageInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	return c, s.save(c)
}
func (s *Store) List(repo string) ([]Commitment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Commitment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Commitment{}
	for _, x := range entries {
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
func (s *Store) Get(repo, cid string) (Commitment, error) {
	all, e := s.List(repo)
	if e != nil {
		return Commitment{}, e
	}
	for _, c := range all {
		if c.ID == cid {
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
		latest := map[string]Coverage{}
		for _, x := range c.Coverage {
			if x.Version == c.CurrentVersion {
				latest[x.ScenarioID+"\x00"+x.AssistiveTechnologyID] = x
			}
		}
		activeException := func(sid string) bool {
			for _, x := range v.Exceptions {
				if x.ExpiresAt.After(now) {
					for _, y := range x.ScenarioIDs {
						if y == sid {
							return true
						}
					}
				}
			}
			return false
		}
		for _, sc := range v.Scenarios {
			for _, at := range sc.AssistiveTechnologyIDs {
				x, ok := latest[sc.ID+"\x00"+at]
				if !ok && !activeException(sc.ID) {
					c.Blockers = append(c.Blockers, Blocker{Kind: "missing_coverage", ScenarioID: sc.ID, EnvironmentID: at, Detail: "required scenario has no current evidence"})
				} else if ok && x.Status == "failed" {
					c.Blockers = append(c.Blockers, Blocker{Kind: "failed_coverage", ScenarioID: sc.ID, EnvironmentID: at, Detail: "required scenario currently fails"})
				} else if ok && (x.Status == "unsupported" || x.Status == "not_tested") {
					c.Blockers = append(c.Blockers, Blocker{Kind: "unsupported_environment", ScenarioID: sc.ID, EnvironmentID: at, Detail: "current evidence is " + x.Status})
				}
			}
		}
		for _, x := range v.Exceptions {
			days := x.ExpiresAt.Sub(now)
			kind := ""
			if days <= 0 {
				kind = "expired_exception"
			} else if days <= 30*24*time.Hour {
				kind = "expiring_exception"
			}
			if kind != "" {
				c.Blockers = append(c.Blockers, Blocker{Kind: kind, ExceptionID: x.ID, Detail: "exception expires " + x.ExpiresAt.Format(time.RFC3339)})
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
			for _, x := range a.Standards {
				for _, y := range b.Standards {
					if strings.EqualFold(x.Name, y.Name) && (x.Version != y.Version || x.Level != y.Level) {
						all[i].Blockers = append(all[i].Blockers, Blocker{Kind: "conflicting_requirement", CommitmentID: all[j].ID, Detail: x.Name + " requirements differ"})
						all[j].Blockers = append(all[j].Blockers, Blocker{Kind: "conflicting_requirement", CommitmentID: all[i].ID, Detail: x.Name + " requirements differ"})
					}
				}
			}
		}
	}
}
