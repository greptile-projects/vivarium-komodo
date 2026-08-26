// Package runbookexecutions retains safe launches of exact operational procedures.
package runbookexecutions

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

var ErrNotFound = errors.New("runbook execution not found")
var ErrInvalid = errors.New("invalid runbook execution")
var ErrConflict = errors.New("duplicate runbook execution")
var ErrBlocked = errors.New("runbook execution blocked")

type Origin struct {
	Kind              string `json:"kind"`
	ResourceID        string `json:"resource_id"`
	Revision          string `json:"revision"`
	TimelineReference string `json:"timeline_reference"`
	Audience          string `json:"audience"`
}
type SignalWindow struct {
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
type Context struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	Permitted  bool   `json:"permitted"`
	Audience   string `json:"audience"`
	Accessible bool   `json:"accessible"`
}
type Check struct {
	ID                string `json:"id"`
	Satisfied         bool   `json:"satisfied"`
	EvidenceReference string `json:"evidence_reference,omitempty"`
	Detail            string `json:"detail,omitempty"`
}
type Access struct {
	Capability         string `json:"capability"`
	ResourceID         string `json:"resource_id"`
	Granted            bool   `json:"granted"`
	AuthorityReference string `json:"authority_reference,omitempty"`
}
type LaunchInput struct {
	IdempotencyKey    string       `json:"idempotency_key"`
	RunbookID         string       `json:"runbook_id"`
	RunbookVersion    int64        `json:"runbook_version"`
	Origin            Origin       `json:"origin"`
	AffectedResources []string     `json:"affected_resources"`
	SignalWindow      SignalWindow `json:"signal_window"`
	Context           []Context    `json:"context"`
	Preconditions     []Check      `json:"preconditions"`
	Access            []Access     `json:"access"`
	MatchExplanation  []string     `json:"match_explanation"`
	RehearsalID       string       `json:"rehearsal_id"`
	RehearsalRevision int64        `json:"rehearsal_revision"`
	RehearsalReady    bool         `json:"rehearsal_ready"`
	RunbookFindings   []string     `json:"runbook_findings,omitempty"`
}
type Blocker struct {
	Kind    string   `json:"kind"`
	Subject string   `json:"subject"`
	Detail  string   `json:"detail"`
	Choices []string `json:"choices,omitempty"`
}
type Execution struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	LaunchInput
	ControllerID string    `json:"controller_id"`
	CreatedAt    time.Time `json:"created_at"`
	State        string    `json:"state"`
	Blockers     []Blocker `json:"blockers"`
	NonAuthority []string  `json:"non_authority"`
}
type Candidate struct {
	RunbookID        string    `json:"runbook_id"`
	RunbookVersion   int64     `json:"runbook_version"`
	Name             string    `json:"name"`
	Eligible         bool      `json:"eligible"`
	Score            int       `json:"score"`
	MatchExplanation []string  `json:"match_explanation"`
	Blockers         []Blocker `json:"blockers"`
	Choices          []string  `json:"choices,omitempty"`
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
func uid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(s string, xs ...string) bool {
	for _, x := range xs {
		if s == x {
			return true
		}
	}
	return false
}
func unique(xs []string) bool {
	if len(xs) == 0 {
		return false
	}
	m := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func validate(in LaunchInput) bool {
	if in.IdempotencyKey == "" || in.RunbookID == "" || in.RunbookVersion < 1 || !allowed(in.Origin.Kind, "alert", "incident", "deployment", "failed_workflow", "service_objective", "support_thread", "manual_observation") || in.Origin.ResourceID == "" || in.Origin.Revision == "" || in.Origin.TimelineReference == "" || in.Origin.Audience == "" || !unique(in.AffectedResources) || in.SignalWindow.StartedAt.IsZero() || in.SignalWindow.EndedAt.Before(in.SignalWindow.StartedAt) || len(in.Context) == 0 || len(in.Preconditions) == 0 || len(in.Access) == 0 || len(in.MatchExplanation) == 0 {
		return false
	}
	for _, c := range in.Context {
		if c.Kind == "" || c.ResourceID == "" || c.Revision == "" || c.Audience == "" {
			return false
		}
	}
	for _, c := range in.Preconditions {
		if c.ID == "" {
			return false
		}
	}
	for _, a := range in.Access {
		if a.Capability == "" || a.ResourceID == "" {
			return false
		}
	}
	return true
}
func blockers(in LaunchInput) []Blocker {
	out := []Blocker{}
	for _, finding := range in.RunbookFindings {
		out = append(out, Blocker{"runbook_finding", in.RunbookID, finding, []string{"inspect current finding", "select another procedure"}})
	}
	if !in.RehearsalReady {
		out = append(out, Blocker{"stale_or_missing_rehearsal", in.RehearsalID, "selected revision lacks current rehearsal proof", []string{"select another eligible runbook", "rehearse this revision"}})
	}
	for _, c := range in.Context {
		if !c.Permitted {
			out = append(out, Blocker{"evidence_not_permitted", c.ResourceID, "origin audience does not permit this evidence", nil})
		}
		if !c.Accessible {
			out = append(out, Blocker{"dependency_unavailable", c.ResourceID, "bound context is currently unavailable", []string{"continue without this evidence", "wait for dependency"}})
		}
	}
	for _, c := range in.Preconditions {
		if !c.Satisfied {
			out = append(out, Blocker{"precondition_failed", c.ID, c.Detail, []string{"resolve precondition", "choose another procedure"}})
		}
	}
	for _, a := range in.Access {
		if !a.Granted || a.AuthorityReference == "" {
			out = append(out, Blocker{"access_unavailable", a.Capability + ":" + a.ResourceID, "current authority was not verified", []string{"request ordinary access", "choose a diagnostic-only procedure"}})
		}
	}
	return out
}
func (s *Store) Create(repo, actor string, in LaunchInput) (Execution, error) {
	if repo == "" || actor == "" || !validate(in) {
		return Execution{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Execution{}, e
	}
	for _, x := range xs {
		if x.IdempotencyKey == in.IdempotencyKey {
			return x, nil
		}
		if x.State == "ready" && x.RunbookID == in.RunbookID && x.RunbookVersion == in.RunbookVersion && x.Origin.Kind == in.Origin.Kind && x.Origin.ResourceID == in.Origin.ResourceID && x.Origin.Revision == in.Origin.Revision {
			return x, ErrConflict
		}
	}
	bs := blockers(in)
	state := "ready"
	if len(bs) > 0 {
		state = "blocked"
	}
	x := Execution{uid(), repo, 1, in, actor, s.now().UTC(), state, bs, []string{"Runbook executions coordinate permitted context and validated readiness; they grant no repository, secret, workflow, agent, communication, incident, deployment, environment, credential, or operational authority."}}
	return x, s.write(x)
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) write(x Execution) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, id string) (Execution, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Execution{}, ErrNotFound
	}
	var x Execution
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Get(repo, id string) (Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) list(repo string) ([]Execution, error) {
	ps, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	sort.Strings(ps)
	out := []Execution{}
	for _, p := range ps {
		b, x := os.ReadFile(p)
		var v Execution
		if x == nil {
			x = json.Unmarshal(b, &v)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	return out, e
}
func (s *Store) List(repo string) ([]Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(repo)
}
