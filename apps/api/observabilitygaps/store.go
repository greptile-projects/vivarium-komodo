// Package observabilitygaps owns collaborative, revisioned questions about running software.
package observabilitygaps

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

var (
	ErrNotFound = errors.New("observability gap not found")
	ErrInvalid  = errors.New("invalid observability gap")
	ErrConflict = errors.New("observability gap version conflict")
)

type Origin struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
}
type Timeliness struct {
	MaximumDelaySeconds int    `json:"maximum_delay_seconds"`
	DecisionWindow      string `json:"decision_window"`
}
type Evidence struct {
	ID                  string    `json:"id"`
	Kind                string    `json:"kind"`
	Source              string    `json:"source"`
	Semantics           string    `json:"semantics"`
	ReleaseID           string    `json:"release_id"`
	ReleaseRevision     string    `json:"release_revision"`
	Environment         string    `json:"environment"`
	EnvironmentRevision string    `json:"environment_revision"`
	ObservedAt          time.Time `json:"observed_at"`
	FreshUntil          time.Time `json:"fresh_until"`
	Accessible          bool      `json:"accessible"`
	OwnerID             string    `json:"owner_id"`
}
type Input struct {
	Origin          Origin     `json:"origin"`
	Question        string     `json:"question"`
	Behavior        string     `json:"behavior"`
	Audience        []string   `json:"audience"`
	Decision        string     `json:"decision"`
	Services        []string   `json:"affected_services"`
	Journeys        []string   `json:"affected_journeys"`
	Timeliness      Timeliness `json:"required_timeliness"`
	Evidence        []Evidence `json:"current_evidence"`
	MissingCoverage []string   `json:"missing_coverage"`
	OwnerIDs        []string   `json:"owner_ids"`
	SuccessCriteria []string   `json:"success_criteria"`
	ChangeReason    string     `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Finding struct {
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	EvidenceID string `json:"evidence_id,omitempty"`
	OwnerID    string `json:"owner_id"`
}
type Gap struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Findings       []Finding `json:"findings"`
	NonAuthority   []string  `json:"non_authority"`
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
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func listOK(xs []string) bool {
	if len(xs) == 0 || len(xs) > 50 {
		return false
	}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	if !map[string]bool{"service_objective": true, "incident": true, "debugging_workspace": true, "runbook": true, "support_thread": true, "deployment": true, "manual_question": true}[in.Origin.Kind] || in.Origin.ResourceID == "" || in.Origin.Revision == "" || strings.TrimSpace(in.Question) == "" || strings.TrimSpace(in.Behavior) == "" || !listOK(in.Audience) || strings.TrimSpace(in.Decision) == "" || !listOK(in.Services) || !listOK(in.Journeys) || in.Timeliness.MaximumDelaySeconds < 1 || in.Timeliness.DecisionWindow == "" || !listOK(in.OwnerIDs) || !listOK(in.SuccessCriteria) || in.ChangeReason == "" {
		return false
	}
	seen := map[string]bool{}
	for _, e := range in.Evidence {
		if e.ID == "" || seen[e.ID] || !map[string]bool{"metric": true, "log": true, "trace": true, "profile": true, "event": true}[e.Kind] || e.Source == "" || e.OwnerID == "" || e.ObservedAt.IsZero() {
			return false
		}
		seen[e.ID] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Gap, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Gap{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publish(Gap{ID: newID(), RepositoryID: repo}, actor, 0, in)
}
func (s *Store) Revise(repo, gid, actor string, expected int64, in Input) (Gap, error) {
	if actor == "" || !valid(in) {
		return Gap{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, e := s.read(repo, gid)
	if e != nil {
		return g, e
	}
	return s.publish(g, actor, expected, in)
}
func (s *Store) publish(g Gap, actor string, expected int64, in Input) (Gap, error) {
	if g.CurrentVersion != expected {
		return g, ErrConflict
	}
	g.CurrentVersion++
	g.Versions = append(g.Versions, Version{Number: g.CurrentVersion, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return g, s.write(g)
}
func (s *Store) Get(repo, gid string) (Gap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, gid)
}
func (s *Store) List(repo string) ([]Gap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Gap{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var g Gap
		if x == nil {
			x = json.Unmarshal(b, &g)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, g)
	}
	return out, nil
}
func (s *Store) read(repo, gid string) (Gap, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, gid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Gap{}, ErrNotFound
	}
	var g Gap
	if e == nil {
		e = json.Unmarshal(b, &g)
	}
	return g, e
}
func (s *Store) write(g Gap) error {
	d := filepath.Join(s.root, g.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(g, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "gap-*.tmp")
	if e != nil {
		return e
	}
	n := f.Name()
	defer os.Remove(n)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if x := f.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, g.ID+".json"))
	}
	return e
}

func Resolve(g Gap, now time.Time) Gap {
	g.Findings = nil
	g.NonAuthority = []string{"Observability gaps grant no repository, telemetry, secret, deployment, environment, incident, communication, or operational authority."}
	if len(g.Versions) == 0 {
		return g
	}
	v := g.Versions[len(g.Versions)-1]
	for _, m := range v.MissingCoverage {
		if strings.TrimSpace(m) != "" {
			g.Findings = append(g.Findings, Finding{Kind: "absent_coverage", Detail: m, OwnerID: v.AuthorID})
		}
	}
	for _, e := range v.Evidence {
		owner := e.OwnerID
		if !e.Accessible {
			g.Findings = append(g.Findings, Finding{Kind: "inaccessible_source", Detail: "Evidence source is not accessible to the declared audience.", EvidenceID: e.ID, OwnerID: owner})
		}
		if strings.TrimSpace(e.Semantics) == "" {
			g.Findings = append(g.Findings, Finding{Kind: "ambiguous_semantics", Detail: "Evidence meaning is not defined.", EvidenceID: e.ID, OwnerID: owner})
		}
		if e.ReleaseID == "" || e.ReleaseRevision == "" || e.Environment == "" || e.EnvironmentRevision == "" {
			g.Findings = append(g.Findings, Finding{Kind: "unbound_context", Detail: "Evidence is not bound to an exact release and environment.", EvidenceID: e.ID, OwnerID: owner})
		}
		if e.FreshUntil.IsZero() || !e.FreshUntil.After(now) {
			g.Findings = append(g.Findings, Finding{Kind: "stale_instrumentation", Detail: "Evidence freshness has expired or was not declared.", EvidenceID: e.ID, OwnerID: owner})
		}
	}
	if len(v.Evidence) == 0 && len(v.MissingCoverage) == 0 {
		g.Findings = append(g.Findings, Finding{Kind: "absent_coverage", Detail: "No current evidence or missing coverage was declared.", OwnerID: v.AuthorID})
	}
	return g
}
