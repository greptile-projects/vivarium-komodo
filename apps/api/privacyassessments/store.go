// Package privacyassessments retains revision-exact, collaborative privacy impact review.
package privacyassessments

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

var ErrNotFound = errors.New("privacy assessment not found")
var ErrInvalid = errors.New("invalid privacy assessment")

type Location struct {
	Path      string `json:"path"`
	BlobID    string `json:"blob_id"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}
type FlowComparison struct {
	Kind            string     `json:"kind"`
	Summary         string     `json:"summary"`
	BaselineFlowID  string     `json:"baseline_flow_id,omitempty"`
	CandidateFlowID string     `json:"candidate_flow_id,omitempty"`
	Categories      []string   `json:"categories,omitempty"`
	Before          string     `json:"before,omitempty"`
	After           string     `json:"after,omitempty"`
	Evidence        []Location `json:"evidence"`
}
type CommitmentRef struct {
	ID               string   `json:"id"`
	BaselineVersion  int64    `json:"baseline_version"`
	CandidateVersion int64    `json:"candidate_version"`
	DataUseIDs       []string `json:"data_use_ids"`
}
type Requirement struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	OwnerIDs  []string `json:"owner_ids"`
	Rationale string   `json:"rationale"`
}
type Input struct {
	Revision       string           `json:"revision"`
	TargetRevision string           `json:"target_revision"`
	Summary        string           `json:"summary"`
	Comparisons    []FlowComparison `json:"comparisons"`
	Commitments    []CommitmentRef  `json:"commitments"`
	Requirements   []Requirement    `json:"requirements"`
	ResidualRisk   string           `json:"residual_risk"`
}
type EntryInput struct {
	Kind           string     `json:"kind"`
	Body           string     `json:"body"`
	RequirementIDs []string   `json:"requirement_ids,omitempty"`
	Evidence       []Location `json:"evidence"`
}
type Entry struct {
	ID string `json:"id"`
	EntryInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Acknowledgement struct {
	RequirementID string    `json:"requirement_id"`
	OwnerID       string    `json:"owner_id"`
	Decision      string    `json:"decision"`
	Rationale     string    `json:"rationale"`
	Revision      string    `json:"revision"`
	CreatedAt     time.Time `json:"created_at"`
	Stale         bool      `json:"stale"`
}
type Blocker struct {
	Kind          string `json:"kind"`
	RequirementID string `json:"requirement_id,omitempty"`
	Detail        string `json:"detail"`
}
type Assessment struct {
	ID            string `json:"id"`
	RepositoryID  string `json:"repository_id"`
	PullRequestID string `json:"pull_request_id"`
	Input
	CreatedByID      string            `json:"created_by_id"`
	CreatedAt        time.Time         `json:"created_at"`
	Entries          []Entry           `json:"entries"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Stale            bool              `json:"stale"`
	Blockers         []Blocker         `json:"blockers"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	a, e := filepath.Abs(root)
	if root == "" {
		return nil, ErrInvalid
	}
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newID() string            { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func txt(v string, n int) bool { return strings.TrimSpace(v) != "" && len(v) <= n }
func list(v []string, optional bool) bool {
	if (!optional && len(v) == 0) || len(v) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range v {
		if !txt(x, 500) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}

var kinds = map[string]bool{"collection": true, "purpose": true, "recipient": true, "retention": true, "access": true, "user_control": true}
var actions = map[string]bool{"owner_acknowledgement": true, "notice": true, "consent_change": true, "migration": true, "test": true, "exception": true}

func loc(x Location) bool {
	return txt(x.Path, 500) && txt(x.BlobID, 100) && x.StartLine >= 0 && x.EndLine >= x.StartLine
}
func valid(in Input) bool {
	if !txt(in.Revision, 100) || !txt(in.TargetRevision, 100) || !txt(in.Summary, 65536) || len(in.Comparisons) == 0 || len(in.Comparisons) > 500 || len(in.Commitments) == 0 || len(in.Requirements) == 0 || len(in.ResidualRisk) > 65536 {
		return false
	}
	ids := map[string]bool{}
	for _, x := range in.Comparisons {
		if !kinds[x.Kind] || !txt(x.Summary, 4000) || len(x.Evidence) == 0 {
			return false
		}
		for _, e := range x.Evidence {
			if !loc(e) {
				return false
			}
		}
	}
	for _, c := range in.Commitments {
		if !txt(c.ID, 100) || c.BaselineVersion < 1 || c.CandidateVersion < 1 || !list(c.DataUseIDs, false) {
			return false
		}
	}
	for _, r := range in.Requirements {
		if !txt(r.ID, 100) || ids[r.ID] || !actions[r.Kind] || !list(r.OwnerIDs, false) || !txt(r.Rationale, 4000) {
			return false
		}
		ids[r.ID] = true
	}
	return true
}
func (s *Store) path(repo, pull, id string) string {
	return filepath.Join(s.root, repo, pull, id+".json")
}
func (s *Store) write(a Assessment) error {
	if e := os.MkdirAll(filepath.Dir(s.path(a.RepositoryID, a.PullRequestID, a.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(a.RepositoryID, a.PullRequestID, a.ID), b, 0640)
	}
	return e
}
func (s *Store) read(repo, pull, id string) (Assessment, error) {
	var a Assessment
	b, e := os.ReadFile(s.path(repo, pull, id))
	if errors.Is(e, fs.ErrNotExist) {
		return a, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	if e != nil || a.ID != id || a.RepositoryID != repo || a.PullRequestID != pull {
		return Assessment{}, ErrNotFound
	}
	return a, nil
}
func (s *Store) Create(repo, pull, actor string, in Input) (Assessment, error) {
	if repo == "" || pull == "" || actor == "" || !valid(in) {
		return Assessment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := Assessment{ID: newID(), RepositoryID: repo, PullRequestID: pull, Input: in, CreatedByID: actor, CreatedAt: s.now().UTC(), Entries: []Entry{}, Acknowledgements: []Acknowledgement{}}
	return a, s.write(a)
}
func (s *Store) mutate(repo, pull, id string, fn func(*Assessment) error) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pull, id)
	if e == nil {
		e = fn(&a)
	}
	if e == nil {
		e = s.write(a)
	}
	return a, e
}
func (s *Store) AddEntry(repo, pull, id, actor string, in EntryInput) (Assessment, error) {
	return s.mutate(repo, pull, id, func(a *Assessment) error {
		if actor == "" || !map[string]bool{"challenge": true, "mitigation": true, "residual_risk": true}[in.Kind] || !txt(in.Body, 65536) || len(in.Evidence) == 0 {
			return ErrInvalid
		}
		req := map[string]bool{}
		for _, r := range a.Requirements {
			req[r.ID] = true
		}
		for _, x := range in.RequirementIDs {
			if !req[x] {
				return ErrInvalid
			}
		}
		for _, x := range in.Evidence {
			if !loc(x) {
				return ErrInvalid
			}
		}
		a.Entries = append(a.Entries, Entry{ID: newID(), EntryInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Acknowledge(repo, pull, id, actor, requirement, decision, rationale, revision string) (Assessment, error) {
	return s.mutate(repo, pull, id, func(a *Assessment) error {
		ok := false
		for _, r := range a.Requirements {
			if r.ID == requirement {
				for _, o := range r.OwnerIDs {
					ok = ok || o == actor
				}
			}
		}
		if !ok || !map[string]bool{"accept": true, "request_changes": true}[decision] || !txt(rationale, 65536) || revision != a.Revision {
			return ErrInvalid
		}
		a.Acknowledgements = append(a.Acknowledgements, Acknowledgement{RequirementID: requirement, OwnerID: actor, Decision: decision, Rationale: rationale, Revision: revision, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) List(repo, pull string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo, pull))
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			a, e := s.read(repo, pull, strings.TrimSuffix(x.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Get(repo, pull, id string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pull, id)
}
func Derive(a *Assessment, currentRevision string, currentBlobs map[string]string) {
	a.Blockers = nil
	a.Stale = a.Revision != currentRevision
	for i := range a.Acknowledgements {
		a.Acknowledgements[i].Stale = a.Acknowledgements[i].Revision != currentRevision
	}
	for _, c := range a.Comparisons {
		for _, e := range c.Evidence {
			if b, ok := currentBlobs[e.Path]; !ok || b != e.BlobID {
				a.Stale = true
			}
		}
	}
	if a.Stale {
		a.Blockers = append(a.Blockers, Blocker{Kind: "stale_evidence", Detail: "the pull revision or relevant source evidence changed"})
	}
	for _, r := range a.Requirements {
		accepted := false
		rejected := false
		for _, x := range a.Acknowledgements {
			if x.RequirementID == r.ID && !x.Stale {
				accepted = accepted || x.Decision == "accept"
				rejected = rejected || x.Decision == "request_changes"
			}
		}
		if rejected {
			a.Blockers = append(a.Blockers, Blocker{Kind: "changes_requested", RequirementID: r.ID, Detail: "a required owner requested changes"})
		} else if !accepted {
			a.Blockers = append(a.Blockers, Blocker{Kind: "missing_acknowledgement", RequirementID: r.ID, Detail: "required owner acknowledgement is missing"})
		}
	}
}
