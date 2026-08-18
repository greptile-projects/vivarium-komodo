// Package interfacechecks retains revision-exact evidence that an implementation
// satisfies an accepted product-design contract across real usage contexts.
package interfacechecks

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

var ErrNotFound = errors.New("interface check not found")
var ErrInvalid = errors.New("invalid interface check")

type Context struct {
	Viewport            string `json:"viewport"`
	Theme               string `json:"theme"`
	ContentLength       string `json:"content_length"`
	Locale              string `json:"locale"`
	InteractionState    string `json:"interaction_state"`
	AssistiveTechnology string `json:"assistive_technology"`
}
type Input struct {
	Path   string `json:"path"`
	BlobID string `json:"blob_id"`
}
type Artifact struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	URL       string `json:"url,omitempty"`
}
type Difference struct {
	ID                string     `json:"id"`
	Kind              string     `json:"kind"`
	Summary           string     `json:"summary"`
	RequirementIDs    []string   `json:"requirement_ids"`
	BaselineArtifact  string     `json:"baseline_artifact,omitempty"`
	CandidateArtifact string     `json:"candidate_artifact,omitempty"`
	Classification    string     `json:"classification,omitempty"`
	Rationale         string     `json:"rationale,omitempty"`
	ClassifiedByID    string     `json:"classified_by_id,omitempty"`
	ClassifiedAt      *time.Time `json:"classified_at,omitempty"`
	Current           bool       `json:"current"`
}
type Case struct {
	Name           string             `json:"name"`
	Journey        string             `json:"journey"`
	Surface        string             `json:"surface"`
	Context        Context            `json:"context"`
	RequirementIDs []string           `json:"requirement_ids"`
	Inputs         []Input            `json:"inputs"`
	Status         string             `json:"status"`
	Summary        string             `json:"summary"`
	Coverage       []string           `json:"coverage"`
	DurationMS     int64              `json:"duration_ms"`
	Performance    map[string]float64 `json:"performance,omitempty"`
	Artifacts      []Artifact         `json:"artifacts"`
	Differences    []Difference       `json:"differences"`
	Current        bool               `json:"current"`
}
type Approval struct {
	ID            string    `json:"id"`
	CaseName      string    `json:"case_name"`
	DifferenceIDs []string  `json:"difference_ids"`
	Decision      string    `json:"decision"`
	Note          string    `json:"note"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
	Current       bool      `json:"current"`
}
type Run struct {
	ID                   string     `json:"id"`
	RepositoryID         string     `json:"repository_id"`
	PullRequestID        string     `json:"pull_request_id"`
	Revision             string     `json:"revision"`
	SpecificationKind    string     `json:"specification_kind"`
	SpecificationID      string     `json:"specification_id"`
	SpecificationVersion int64      `json:"specification_version"`
	ConfigPath           string     `json:"config_path"`
	ConfigBlobID         string     `json:"config_blob_id"`
	CreatedByID          string     `json:"created_by_id"`
	CreatedAt            time.Time  `json:"created_at"`
	Cases                []Case     `json:"cases"`
	Approvals            []Approval `json:"approvals"`
	Current              bool       `json:"current"`
	Passed               bool       `json:"passed"`
	MissingContexts      []string   `json:"missing_contexts,omitempty"`
	AffectedRequirements []string   `json:"affected_requirements,omitempty"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
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
func validArtifact(a Artifact) bool {
	return map[string]bool{"screenshot": true, "recording": true, "trace": true, "accessibility_tree": true, "behavior_log": true, "performance": true, "diff": true}[a.Kind] && a.Name != "" && len(a.Digest) >= 16 && a.Size >= 0 && a.Size <= 50<<20
}
func validCase(c Case) bool {
	if c.Name == "" || c.Journey == "" || c.Surface == "" || len(c.RequirementIDs) == 0 || len(c.Inputs) == 0 || !map[string]bool{"passed": true, "failed": true, "inconclusive": true}[c.Status] {
		return false
	}
	x := c.Context
	if x.Viewport == "" || x.Theme == "" || x.ContentLength == "" || x.Locale == "" || x.InteractionState == "" || x.AssistiveTechnology == "" {
		return false
	}
	for _, a := range c.Artifacts {
		if !validArtifact(a) {
			return false
		}
	}
	for _, d := range c.Differences {
		if d.ID == "" || d.Summary == "" || len(d.RequirementIDs) == 0 || !map[string]bool{"visual": true, "behavioral": true, "content": true, "accessibility": true, "performance": true}[d.Kind] {
			return false
		}
	}
	return true
}
func (s *Store) Create(run Run) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run.RepositoryID == "" || run.PullRequestID == "" || run.Revision == "" || run.SpecificationID == "" || run.SpecificationVersion < 1 || run.ConfigPath == "" || run.ConfigBlobID == "" || run.CreatedByID == "" || len(run.Cases) == 0 || !map[string]bool{"design_proposal": true, "implementation_contract": true}[run.SpecificationKind] {
		return Run{}, ErrInvalid
	}
	names := map[string]bool{}
	for i := range run.Cases {
		c := &run.Cases[i]
		if !validCase(*c) || names[c.Name] {
			return Run{}, ErrInvalid
		}
		names[c.Name] = true
		c.RequirementIDs = clean(c.RequirementIDs)
		c.Coverage = clean(c.Coverage)
		c.Current = true
		for j := range c.Differences {
			c.Differences[j].Current = true
		}
	}
	run.ID = id()
	run.CreatedAt = s.now().UTC()
	run.Current = true
	derive(&run)
	if e := s.write(run); e != nil {
		return Run{}, e
	}
	return run, nil
}
func (s *Store) Get(repo, pull, run string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pull, run)
}
func (s *Store) List(repo, pull string) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, pull)
	es, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return []Run{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Run{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, e := s.read(repo, pull, strings.TrimSuffix(x.Name(), ".json"))
		if e == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Classify(repo, pull, run, caseName, diff, actor, class, rationale string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || rationale == "" || !map[string]bool{"intentional": true, "regression": true, "false_positive": true}[class] {
		return Run{}, ErrInvalid
	}
	v, e := s.read(repo, pull, run)
	if e != nil {
		return Run{}, e
	}
	found := false
	now := s.now().UTC()
	for i := range v.Cases {
		if v.Cases[i].Name != caseName {
			continue
		}
		for j := range v.Cases[i].Differences {
			d := &v.Cases[i].Differences[j]
			if d.ID == diff && d.Current {
				d.Classification = class
				d.Rationale = rationale
				d.ClassifiedByID = actor
				d.ClassifiedAt = &now
				found = true
			}
		}
	}
	if !found {
		return Run{}, ErrNotFound
	}
	derive(&v)
	e = s.write(v)
	return v, e
}
func (s *Store) Approve(repo, pull, run, caseName, actor, decision, note string, diffs []string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || note == "" || !map[string]bool{"approved": true, "rejected": true}[decision] {
		return Run{}, ErrInvalid
	}
	v, e := s.read(repo, pull, run)
	if e != nil {
		return Run{}, e
	}
	valid := map[string]bool{}
	foundCase := false
	for _, c := range v.Cases {
		if c.Name == caseName && c.Current {
			foundCase = true
			for _, d := range c.Differences {
				if d.Current && d.Classification != "" {
					valid[d.ID] = true
				}
			}
		}
	}
	diffs = clean(diffs)
	if !foundCase {
		return Run{}, ErrNotFound
	}
	for _, d := range diffs {
		if !valid[d] {
			return Run{}, ErrInvalid
		}
	}
	v.Approvals = append(v.Approvals, Approval{ID: id(), CaseName: caseName, DifferenceIDs: diffs, Decision: decision, Note: note, ActorID: actor, CreatedAt: s.now().UTC(), Current: true})
	derive(&v)
	e = s.write(v)
	return v, e
}

// DeriveCurrent invalidates only cases whose declared input blobs changed. The
// config blob represents repository-owned scenario/specification changes.
func DeriveCurrent(v *Run, _ string, currentConfigBlob string, blobs map[string]string) {
	// A newer pull revision may reuse a case only when its definition and every
	// declared input blob are identical. The original candidate revision remains
	// visible as provenance; unrelated commits do not discard useful evidence.
	v.Current = currentConfigBlob == v.ConfigBlobID
	affected := map[string]bool{}
	for i := range v.Cases {
		c := &v.Cases[i]
		c.Current = v.Current
		for _, in := range c.Inputs {
			if blobs[in.Path] != in.BlobID {
				c.Current = false
			}
		}
		for j := range c.Differences {
			c.Differences[j].Current = c.Current
			if !c.Current {
				for _, r := range c.Differences[j].RequirementIDs {
					affected[r] = true
				}
			}
		}
	}
	for i := range v.Approvals {
		a := &v.Approvals[i]
		a.Current = false
		for _, c := range v.Cases {
			if c.Name == a.CaseName && c.Current {
				a.Current = true
				for _, id := range a.DifferenceIDs {
					ok := false
					for _, d := range c.Differences {
						ok = ok || (d.ID == id && d.Current && d.Classification != "")
					}
					a.Current = a.Current && ok
				}
			}
		}
	}
	v.AffectedRequirements = nil
	for x := range affected {
		v.AffectedRequirements = append(v.AffectedRequirements, x)
	}
	sort.Strings(v.AffectedRequirements)
	derive(v)
}
func derive(v *Run) {
	v.Passed = v.Current
	for _, c := range v.Cases {
		if !c.Current || c.Status != "passed" {
			v.Passed = false
		}
		for _, d := range c.Differences {
			if d.Current && (d.Classification == "" || d.Classification == "regression") {
				v.Passed = false
			}
		}
	}
	for _, a := range v.Approvals {
		if a.Current && a.Decision == "rejected" {
			v.Passed = false
		}
	}
}
func (s *Store) path(repo, pull, id string) string {
	return filepath.Join(s.root, repo, pull, id+".json")
}
func (s *Store) write(v Run) error {
	p := s.path(v.RepositoryID, v.PullRequestID, v.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, p)
	}
	return e
}
func (s *Store) read(repo, pull, id string) (Run, error) {
	b, e := os.ReadFile(s.path(repo, pull, id))
	if os.IsNotExist(e) {
		return Run{}, ErrNotFound
	}
	var v Run
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
