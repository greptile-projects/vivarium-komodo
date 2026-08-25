// Package reviewplans owns immutable, revision-exact pull-request review plans.
package reviewplans

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalid = errors.New("invalid review plan")
var ErrNotFound = errors.New("review plan not found")
var ErrConflict = errors.New("review plan version conflict")

type Context struct {
	Kind       string   `json:"kind"`
	Reference  string   `json:"reference"`
	Revision   string   `json:"revision,omitempty"`
	Accessible bool     `json:"accessible"`
	OwnerIDs   []string `json:"owner_ids,omitempty"`
}
type Evidence struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Reference   string `json:"reference,omitempty"`
	Required    bool   `json:"required"`
}
type Area struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Expertise       []string   `json:"expertise"`
	Paths           []string   `json:"paths"`
	OwnerIDs        []string   `json:"owner_ids"`
	Questions       []string   `json:"acceptance_questions"`
	Evidence        []Evidence `json:"required_evidence"`
	DependsOn       []string   `json:"depends_on"`
	CompletionRules []string   `json:"completion_rules"`
}
type Input struct {
	Intent           string    `json:"intent"`
	Risk             string    `json:"risk"`
	PolicyReferences []string  `json:"policy_references"`
	Commitments      []Context `json:"commitments"`
	Context          []Context `json:"context"`
	Areas            []Area    `json:"review_areas"`
	ChangeReason     string    `json:"change_reason"`
}
type Version struct {
	Number         int64    `json:"number"`
	Revision       string   `json:"revision"`
	TargetRevision string   `json:"target_revision"`
	ChangedPaths   []string `json:"changed_paths"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to,omitempty"`
}
type Plan struct {
	RepositoryID   string    `json:"repository_id"`
	PullRequestID  string    `json:"pull_request_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Blockers       []Blocker `json:"blockers"`
	Stale          bool      `json:"stale"`
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
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) Get(repo, pull string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pull)
}
func (s *Store) read(repo, pull string) (Plan, error) {
	b, e := os.ReadFile(s.path(repo, pull))
	if errors.Is(e, os.ErrNotExist) {
		return Plan{}, ErrNotFound
	}
	var p Plan
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func clean(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Intent) == "" || strings.TrimSpace(in.ChangeReason) == "" || !map[string]bool{"low": true, "medium": true, "high": true, "critical": true}[in.Risk] || len(in.Areas) == 0 {
		return false
	}
	ids := map[string]bool{}
	for _, a := range in.Areas {
		if a.ID == "" || a.Name == "" || ids[a.ID] || len(a.Paths) == 0 || len(a.Questions) == 0 || len(a.Evidence) == 0 || len(a.CompletionRules) == 0 {
			return false
		}
		ids[a.ID] = true
		for _, e := range a.Evidence {
			if e.Kind == "" || e.Description == "" {
				return false
			}
		}
	}
	for _, a := range in.Areas {
		for _, d := range a.DependsOn {
			if !ids[d] || d == a.ID {
				return false
			}
		}
	}
	deps := map[string][]string{}
	for _, a := range in.Areas {
		deps[a.ID] = a.DependsOn
	}
	state := map[string]int{}
	var visit func(string) bool
	visit = func(id string) bool {
		if state[id] == 1 {
			return false
		}
		if state[id] == 2 {
			return true
		}
		state[id] = 1
		for _, d := range deps[id] {
			if !visit(d) {
				return false
			}
		}
		state[id] = 2
		return true
	}
	for id := range deps {
		if !visit(id) {
			return false
		}
	}
	return true
}
func (s *Store) Publish(repo, pull, revision, target, actor string, paths []string, expected int64, in Input) (Plan, error) {
	if repo == "" || pull == "" || revision == "" || target == "" || actor == "" || len(paths) == 0 || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, pull)
	if errors.Is(e, ErrNotFound) {
		p = Plan{RepositoryID: repo, PullRequestID: pull}
	} else if e != nil {
		return Plan{}, e
	}
	if p.CurrentVersion != expected {
		return Plan{}, ErrConflict
	}
	for i := range in.Areas {
		in.Areas[i].Paths = clean(in.Areas[i].Paths)
		in.Areas[i].OwnerIDs = clean(in.Areas[i].OwnerIDs)
		in.Areas[i].Expertise = clean(in.Areas[i].Expertise)
		in.Areas[i].DependsOn = clean(in.Areas[i].DependsOn)
	}
	v := Version{Number: p.CurrentVersion + 1, Revision: revision, TargetRevision: target, ChangedPaths: clean(paths), Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	p.CurrentVersion = v.Number
	p.Versions = append(p.Versions, v)
	p = Derive(p, revision, target)
	b, _ := json.MarshalIndent(p, "", "  ")
	if e = os.MkdirAll(filepath.Dir(s.path(repo, pull)), 0750); e == nil {
		e = os.WriteFile(s.path(repo, pull), b, 0640)
	}
	return p, e
}
func Derive(p Plan, revision, target string) Plan {
	p.Blockers = nil
	p.Stale = false
	if len(p.Versions) == 0 {
		return p
	}
	v := p.Versions[len(p.Versions)-1]
	if v.Revision != revision || v.TargetRevision != target {
		p.Stale = true
		p.Blockers = append(p.Blockers, Blocker{"stale_analysis", "revision", "The plan analyzes " + v.Revision + " against " + v.TargetRevision + ", not the current pull request.", v.AuthorID})
	}
	covered := map[string][]string{}
	changed := map[string]bool{}
	for _, path := range v.ChangedPaths {
		changed[path] = true
	}
	for _, a := range v.Areas {
		if len(a.OwnerIDs) == 0 {
			p.Blockers = append(p.Blockers, Blocker{"missing_ownership", a.ID, "No accountable owner is identified for this review area.", v.AuthorID})
		}
		for _, path := range a.Paths {
			covered[path] = append(covered[path], a.ID)
			if !changed[path] {
				p.Blockers = append(p.Blockers, Blocker{"scope_not_in_change", path, "The area cites a path outside the exact changed code.", v.AuthorID})
			}
		}
	}
	for path := range changed {
		if len(covered[path]) == 0 {
			p.Blockers = append(p.Blockers, Blocker{"unplanned_scope", path, "Changed code is not covered by any review area.", v.AuthorID})
		}
	}
	for path, areas := range covered {
		if len(areas) > 1 {
			p.Blockers = append(p.Blockers, Blocker{"overlapping_scope", path, "Review areas overlap: " + strings.Join(areas, ", ") + ".", v.AuthorID})
		}
	}
	for _, c := range append(append([]Context{}, v.Context...), v.Commitments...) {
		if !c.Accessible {
			p.Blockers = append(p.Blockers, Blocker{"inaccessible_context", c.Kind + ":" + c.Reference, "Required context is not accessible; it is not treated as reviewed.", v.AuthorID})
		}
		if len(c.OwnerIDs) == 0 {
			p.Blockers = append(p.Blockers, Blocker{"missing_ownership", c.Kind + ":" + c.Reference, "The affected context has no declared owner.", v.AuthorID})
		}
	}
	return p
}
