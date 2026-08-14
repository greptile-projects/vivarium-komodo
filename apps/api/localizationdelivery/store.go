// Package localizationdelivery governs locale publication and retained regional feedback.
package localizationdelivery

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
)

var ErrInvalid = errors.New("invalid localization delivery record")
var ErrNotFound = errors.New("localization delivery record not found")
var ErrConflict = errors.New("localization delivery version conflict")

type LocaleRequirement struct {
	LocaleID            string   `json:"locale_id"`
	Audiences           []string `json:"audiences"`
	RiskClasses         []string `json:"risk_classes"`
	MinimumCoverage     int      `json:"minimum_coverage"`
	RequiredChecks      []string `json:"required_checks"`
	RequiredReviewerIDs []string `json:"required_reviewer_ids"`
}
type PolicyInput struct {
	Name           string              `json:"name"`
	TargetBranches []string            `json:"target_branches"`
	Paths          []string            `json:"paths,omitempty"`
	Locales        []LocaleRequirement `json:"locales"`
}
type Policy struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	PolicyInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type CandidateLocale struct {
	LocaleID       string `json:"locale_id"`
	State          string `json:"state"` // staged, deferred, withdrawn
	Audience       string `json:"audience"`
	RiskClass      string `json:"risk_class"`
	Coverage       int    `json:"coverage"`
	FallbackLocale string `json:"fallback_locale,omitempty"`
	Reason         string `json:"reason"`
}
type Candidate struct {
	RepositoryID  string            `json:"repository_id"`
	PullRequestID string            `json:"pull_request_id"`
	Revision      string            `json:"revision"`
	Version       int64             `json:"version"`
	Locales       []CandidateLocale `json:"locales"`
	ActorID       string            `json:"actor_id"`
	CreatedAt     time.Time         `json:"created_at"`
}
type Requirement struct {
	PolicyID string `json:"policy_id"`
	LocaleID string `json:"locale_id"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
	Blocking bool   `json:"blocking"`
}
type Assessment struct {
	Revision     string        `json:"revision"`
	Candidate    *Candidate    `json:"candidate,omitempty"`
	Requirements []Requirement `json:"requirements"`
	Ready        bool          `json:"ready"`
}
type PublicationInput struct {
	Kind                   string   `json:"kind"` // application or documentation
	ResourceID             string   `json:"resource_id"`
	Version                string   `json:"version"`
	Revision               string   `json:"revision"`
	LocaleID               string   `json:"locale_id"`
	State                  string   `json:"state"` // published, withdrawn
	FallbackLocale         string   `json:"fallback_locale,omitempty"`
	CandidatePullRequestID string   `json:"candidate_pull_request_id"`
	CandidateVersion       int64    `json:"candidate_version"`
	Provenance             []string `json:"provenance"`
	Reason                 string   `json:"reason"`
}
type Publication struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	PublicationInput
	PublishedByID string    `json:"published_by_id"`
	PublishedAt   time.Time `json:"published_at"`
}
type FindingInput struct {
	PublicationID string `json:"publication_id"`
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	Expected      string `json:"expected"`
	Observed      string `json:"observed"`
	Evidence      string `json:"evidence,omitempty"`
}
type Validation struct {
	State     string    `json:"state"`
	Rationale string    `json:"rationale"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Repair struct {
	OwnerKind          string    `json:"owner_kind"`
	OwnerID            string    `json:"owner_id"`
	ProposalID         string    `json:"proposal_id"`
	TaskID             string    `json:"task_id"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	ActorID            string    `json:"actor_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type Finding struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	FindingInput
	LocaleID   string      `json:"locale_id"`
	Revision   string      `json:"revision"`
	ReporterID string      `json:"reporter_id"`
	Validation *Validation `json:"validation,omitempty"`
	Repair     *Repair     `json:"repair,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
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
func ident() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validList(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 500 || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func validPolicy(in PolicyInput) bool {
	if strings.TrimSpace(in.Name) == "" || !validList(in.TargetBranches, true) || !validList(in.Paths, false) || len(in.Locales) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, l := range in.Locales {
		if l.LocaleID == "" || seen[l.LocaleID] || l.MinimumCoverage < 0 || l.MinimumCoverage > 100 || !validList(l.Audiences, true) || !validList(l.RiskClasses, true) || !validList(l.RequiredChecks, true) || !validList(l.RequiredReviewerIDs, true) {
			return false
		}
		seen[l.LocaleID] = true
	}
	return true
}
func write(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		e = os.WriteFile(path, b, 0640)
	}
	return e
}
func read[T any](path string) (T, error) {
	var v T
	b, e := os.ReadFile(path)
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) CreatePolicy(repo, actor string, in PolicyInput) (Policy, error) {
	if repo == "" || actor == "" || !validPolicy(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Policy{ID: ident(), RepositoryID: repo, PolicyInput: in, CreatedByID: actor, CreatedAt: s.now().UTC()}
	return v, write(filepath.Join(s.root, repo, "policies", v.ID+".json"), v)
}
func (s *Store) Policies(repo string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return listFiles[Policy](filepath.Join(s.root, repo, "policies"))
}
func listFiles[T any](dir string) ([]T, error) {
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []T{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []T{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := read[T](filepath.Join(dir, x.Name()))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	return out, nil
}
func validCandidate(ls []CandidateLocale) bool {
	if len(ls) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, l := range ls {
		if l.LocaleID == "" || seen[l.LocaleID] || !map[string]bool{"staged": true, "deferred": true, "withdrawn": true}[l.State] || l.Coverage < 0 || l.Coverage > 100 || l.Reason == "" || l.State == "staged" && (l.Audience == "" || l.RiskClass == "") {
			return false
		}
		seen[l.LocaleID] = true
	}
	return true
}
func (s *Store) SetCandidate(repo, pull, revision, actor string, expected int64, ls []CandidateLocale) (Candidate, error) {
	if repo == "" || pull == "" || revision == "" || actor == "" || !validCandidate(ls) {
		return Candidate{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, repo, "candidates", pull+".json")
	old, _ := read[Candidate](path)
	if old.Version != expected {
		return Candidate{}, ErrConflict
	}
	v := Candidate{RepositoryID: repo, PullRequestID: pull, Revision: revision, Version: expected + 1, Locales: ls, ActorID: actor, CreatedAt: s.now().UTC()}
	return v, write(path, v)
}
func (s *Store) Candidate(repo, pull string) (Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return read[Candidate](filepath.Join(s.root, repo, "candidates", pull+".json"))
}
func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y || strings.HasSuffix(x, "/**") && strings.HasPrefix(y, strings.TrimSuffix(x, "**")) {
				return true
			}
		}
	}
	return false
}
func (s *Store) Assess(repo, pull, revision, branch string, paths []string, v *localizationverification.Assessment) (Assessment, error) {
	ps, e := s.Policies(repo)
	if e != nil {
		return Assessment{}, e
	}
	c, e := s.Candidate(repo, pull)
	if errors.Is(e, ErrNotFound) {
		return Assessment{Revision: revision, Requirements: []Requirement{}, Ready: true}, nil
	}
	if e != nil {
		return Assessment{}, e
	}
	out := Assessment{Revision: revision, Candidate: &c, Requirements: []Requirement{}, Ready: true}
	if c.Revision != revision {
		out.Requirements = append(out.Requirements, Requirement{Kind: "stale_candidate", Detail: "Locale publication choices do not describe the exact candidate.", Blocking: true})
	}
	for _, cl := range c.Locales {
		if cl.State != "staged" {
			continue
		}
		for _, p := range ps {
			if !intersects(p.TargetBranches, []string{branch}) || len(p.Paths) > 0 && !intersects(p.Paths, paths) {
				continue
			}
			for _, lr := range p.Locales {
				if lr.LocaleID != cl.LocaleID || !intersects(lr.Audiences, []string{cl.Audience}) || !intersects(lr.RiskClasses, []string{cl.RiskClass}) {
					continue
				}
				if cl.Coverage < lr.MinimumCoverage {
					out.Requirements = append(out.Requirements, Requirement{p.ID, cl.LocaleID, "coverage", "Current coverage is below the locale publication threshold.", true})
				}
				if v == nil || v.Revision != revision {
					out.Requirements = append(out.Requirements, Requirement{p.ID, cl.LocaleID, "current_verification", "No verification exists for the exact candidate.", true})
					continue
				}
				for _, name := range lr.RequiredChecks {
					ok := false
					for _, ch := range v.Checks {
						ok = ok || ch.LocaleID == cl.LocaleID && ch.Name == name && ch.Status == "passed"
					}
					if !ok {
						out.Requirements = append(out.Requirements, Requirement{p.ID, cl.LocaleID, "check", "Required current locale check “" + name + "” has not passed.", true})
					}
				}
				for _, rid := range lr.RequiredReviewerIDs {
					ok := false
					for _, d := range v.Decisions {
						ok = ok || d.LocaleID == cl.LocaleID && d.ReviewerID == rid && d.Decision == "approve" && !d.Stale
					}
					if !ok {
						out.Requirements = append(out.Requirements, Requirement{p.ID, cl.LocaleID, "review", "Required regional reviewer “" + rid + "” has not approved current evidence.", true})
					}
				}
			}
		}
	}
	out.Ready = len(out.Requirements) == 0
	return out, nil
}
func (s *Store) Publish(repo, actor string, in PublicationInput) (Publication, error) {
	if repo == "" || actor == "" || !map[string]bool{"application": true, "documentation": true}[in.Kind] || in.ResourceID == "" || in.Version == "" || in.Revision == "" || in.LocaleID == "" || !map[string]bool{"published": true, "withdrawn": true}[in.State] || in.CandidatePullRequestID == "" || in.CandidateVersion < 1 || !validList(in.Provenance, true) || in.Reason == "" {
		return Publication{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Publication{ID: ident(), RepositoryID: repo, PublicationInput: in, PublishedByID: actor, PublishedAt: s.now().UTC()}
	return v, write(filepath.Join(s.root, repo, "publications", v.ID+".json"), v)
}
func (s *Store) Publications(repo string) ([]Publication, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := listFiles[Publication](filepath.Join(s.root, repo, "publications"))
	sort.Slice(v, func(i, j int) bool { return v[i].PublishedAt.After(v[j].PublishedAt) })
	return v, e
}
func (s *Store) publication(repo, id string) (Publication, error) {
	return read[Publication](filepath.Join(s.root, repo, "publications", id+".json"))
}
func (s *Store) Report(repo, actor string, in FindingInput) (Finding, error) {
	if actor == "" || !map[string]bool{"mistranslation": true, "cultural_mismatch": true, "broken_formatting": true, "missing_content": true}[in.Kind] || in.Path == "" || in.Expected == "" || in.Observed == "" {
		return Finding{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.publication(repo, in.PublicationID)
	if e != nil || p.State != "published" {
		return Finding{}, ErrInvalid
	}
	v := Finding{ID: ident(), RepositoryID: repo, FindingInput: in, LocaleID: p.LocaleID, Revision: p.Revision, ReporterID: actor, CreatedAt: s.now().UTC()}
	return v, write(filepath.Join(s.root, repo, "findings", v.ID+".json"), v)
}
func (s *Store) Findings(repo string) ([]Finding, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return listFiles[Finding](filepath.Join(s.root, repo, "findings"))
}
func (s *Store) Validate(repo, id, actor, state, rationale string) (Finding, error) {
	if actor == "" || !map[string]bool{"validated": true, "rejected": true, "duplicate": true}[state] || rationale == "" {
		return Finding{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, repo, "findings", id+".json")
	v, e := read[Finding](path)
	if e != nil {
		return v, e
	}
	if v.Validation != nil {
		return v, ErrConflict
	}
	v.Validation = &Validation{state, rationale, actor, s.now().UTC()}
	return v, write(path, v)
}
func (s *Store) LinkRepair(repo, id, actor string, r Repair) (Finding, error) {
	if actor == "" || !map[string]bool{"human": true, "agent": true}[r.OwnerKind] || r.OwnerID == "" || r.ProposalID == "" || r.TaskID == "" || !validList(r.AcceptanceCriteria, true) {
		return Finding{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, repo, "findings", id+".json")
	v, e := read[Finding](path)
	if e != nil {
		return v, e
	}
	if v.Validation == nil || v.Validation.State != "validated" || v.Repair != nil {
		return v, ErrConflict
	}
	r.ActorID = actor
	r.CreatedAt = s.now().UTC()
	v.Repair = &r
	return v, write(path, v)
}
