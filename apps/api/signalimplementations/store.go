// Package signalimplementations retains governed instrumentation work and exact-candidate proof.
package signalimplementations

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

var ErrNotFound = errors.New("signal implementation not found")
var ErrInvalid = errors.New("invalid signal implementation")

type Work struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	OwnerKind    string `json:"owner_kind"`
	OwnerID      string `json:"owner_id"`
	RepositoryID string `json:"repository_id"`
	ResourceID   string `json:"resource_id"`
	Revision     string `json:"revision"`
	Permitted    bool   `json:"permitted"`
}
type Plan struct {
	ID              string    `json:"id"`
	RepositoryID    string    `json:"repository_id"`
	ContractID      string    `json:"contract_id"`
	ContractVersion int64     `json:"contract_version"`
	BaseRevision    string    `json:"base_revision"`
	Work            []Work    `json:"work"`
	CreatedByID     string    `json:"created_by_id"`
	CreatedAt       time.Time `json:"created_at"`
	NonAuthority    []string  `json:"non_authority"`
}
type Evidence struct {
	Kind       string `json:"kind"`
	Summary    string `json:"summary"`
	Digest     string `json:"digest"`
	Sanitized  bool   `json:"sanitized"`
	Accessible bool   `json:"accessible"`
}
type Result struct {
	Check    string     `json:"check"`
	Status   string     `json:"status"`
	Summary  string     `json:"summary"`
	Coverage []string   `json:"coverage"`
	Evidence []Evidence `json:"evidence"`
}
type Difference struct {
	Kind     string `json:"kind"`
	Summary  string `json:"summary"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}
type Run struct {
	ID                string       `json:"id"`
	RepositoryID      string       `json:"repository_id"`
	PullRequestID     string       `json:"pull_request_id"`
	CandidateRevision string       `json:"candidate_revision"`
	PlanID            string       `json:"plan_id"`
	ContractID        string       `json:"contract_id"`
	ContractVersion   int64        `json:"contract_version"`
	ConfigPath        string       `json:"config_path"`
	ConfigRevision    string       `json:"config_revision"`
	Journey           string       `json:"synthetic_journey"`
	Failure           string       `json:"synthetic_failure"`
	Results           []Result     `json:"results"`
	Differences       []Difference `json:"contract_differences"`
	DurationMS        int64        `json:"duration_ms"`
	Cost              float64      `json:"cost"`
	Currency          string       `json:"currency"`
	Authorship        []string     `json:"authorship"`
	PolicyChecks      []string     `json:"ordinary_policy_checks"`
	CreatedByID       string       `json:"created_by_id"`
	CreatedAt         time.Time    `json:"created_at"`
	Passed            bool         `json:"passed"`
	Findings          []string     `json:"findings"`
	NonAuthority      []string     `json:"non_authority"`
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
func id() string         { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(v string) bool { return strings.TrimSpace(v) != "" && len(v) <= 4000 }

func (s *Store) CreatePlan(repo, contract string, version int64, base, actor string, work []Work) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !text(repo) || !text(contract) || version < 1 || !text(base) || !text(actor) || len(work) == 0 {
		return Plan{}, ErrInvalid
	}
	seen := map[string]bool{}
	kinds := map[string]bool{"task": true, "session": true, "workspace": true, "pull_request": true}
	for i := range work {
		w := &work[i]
		if !kinds[w.Kind] || !map[string]bool{"human": true, "agent": true}[w.OwnerKind] || !text(w.OwnerID) || !text(w.RepositoryID) || !text(w.ResourceID) || !text(w.Revision) || !w.Permitted || seen[w.Kind+":"+w.ResourceID] {
			return Plan{}, ErrInvalid
		}
		seen[w.Kind+":"+w.ResourceID] = true
		w.ID = id()
	}
	p := Plan{ID: id(), RepositoryID: repo, ContractID: contract, ContractVersion: version, BaseRevision: base, Work: work, CreatedByID: actor, CreatedAt: s.now().UTC(), NonAuthority: []string{"Implementation records grant no repository, agent, telemetry, preview, review, merge, release, environment, secret, or operational authority."}}
	return p, s.write(filepath.Join(repo, "plans", p.ID+".json"), p)
}
func (s *Store) GetPlan(repo, pid string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p Plan
	e := s.read(filepath.Join(repo, "plans", pid+".json"), &p)
	return p, e
}

var required = []string{"emission", "schema", "units", "correlation", "sampling", "redaction", "access_boundaries", "performance_overhead", "failure_behavior"}

func (s *Store) CreateRun(run Run) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !text(run.RepositoryID) || !text(run.PullRequestID) || !text(run.CandidateRevision) || !text(run.PlanID) || !text(run.ContractID) || run.ContractVersion < 1 || !text(run.ConfigPath) || !text(run.ConfigRevision) || !text(run.Journey) || !text(run.Failure) || run.DurationMS < 0 || run.Cost < 0 || !text(run.Currency) || len(run.Authorship) == 0 || len(run.PolicyChecks) == 0 {
		return Run{}, ErrInvalid
	}
	seen := map[string]bool{}
	passed := true
	for _, r := range run.Results {
		if !contains(required, r.Check) || seen[r.Check] || !map[string]bool{"passed": true, "failed": true, "inconclusive": true}[r.Status] || !text(r.Summary) || len(r.Coverage) == 0 || len(r.Evidence) == 0 {
			return Run{}, ErrInvalid
		}
		seen[r.Check] = true
		passed = passed && r.Status == "passed"
		for _, e := range r.Evidence {
			if !map[string]bool{"signal": true, "log": true, "trace": true, "coverage": true, "performance": true, "cost": true}[e.Kind] || !text(e.Summary) || len(e.Digest) < 16 || !e.Sanitized {
				return Run{}, ErrInvalid
			}
			if !e.Accessible {
				passed = false
			}
		}
	}
	for _, name := range required {
		if !seen[name] {
			run.Findings = append(run.Findings, "missing_check:"+name)
			passed = false
		}
	}
	if len(run.Differences) > 0 {
		for _, d := range run.Differences {
			if !text(d.Kind) || !text(d.Summary) || !text(d.Expected) || !text(d.Actual) {
				return Run{}, ErrInvalid
			}
		}
	}
	run.ID = id()
	run.CreatedAt = s.now().UTC()
	run.Passed = passed
	run.NonAuthority = []string{"Candidate proof does not replace ordinary review, privacy, security, provenance, merge, release, telemetry, or environment authority."}
	return run, s.write(filepath.Join(run.RepositoryID, "pulls", run.PullRequestID, run.ID+".json"), run)
}
func (s *Store) ListRuns(repo, pull string) ([]Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, repo, "pulls", pull)
	es, e := os.ReadDir(dir)
	if errors.Is(e, fs.ErrNotExist) {
		return []Run{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Run{}
	for _, x := range es {
		var r Run
		if filepath.Ext(x.Name()) == ".json" && s.read(filepath.Join(repo, "pulls", pull, x.Name()), &r) == nil {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func (s *Store) read(rel string, v any) error {
	b, e := os.ReadFile(filepath.Join(s.root, rel))
	if errors.Is(e, fs.ErrNotExist) {
		return ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, v)
	}
	return e
}
func (s *Store) write(rel string, v any) error {
	p := filepath.Join(s.root, rel)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, b, 0640)
}
