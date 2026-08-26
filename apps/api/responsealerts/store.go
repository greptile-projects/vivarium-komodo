// Package responsealerts turns revision-bound signals into durable, policy-evaluated attention.
package responsealerts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
)

var ErrNotFound = errors.New("response alert not found")
var ErrInvalid = errors.New("invalid response alert")
var ErrConflict = errors.New("response alert changed")

type Evidence struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision"`
	Accessible bool   `json:"accessible"`
	Summary    string `json:"summary,omitempty"`
}
type Window struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Reason   string    `json:"reason"`
}
type Signal struct {
	SignalClass       string     `json:"signal_class"`
	Severity          string     `json:"severity"`
	ResourceKind      string     `json:"resource_kind"`
	ResourceID        string     `json:"resource_id"`
	Revision          string     `json:"revision"`
	ObservedAt        time.Time  `json:"observed_at"`
	CorrelationKey    string     `json:"correlation_key"`
	Summary           string     `json:"summary"`
	AffectedResources []string   `json:"affected_resources"`
	AffectedUserCount int        `json:"affected_user_count"`
	Evidence          []Evidence `json:"evidence"`
	Uncertainty       string     `json:"uncertainty"`
}
type Input struct {
	Signal             Signal   `json:"signal"`
	SuppressionKeys    []string `json:"suppression_keys"`
	MaintenanceWindows []Window `json:"maintenance_windows"`
	RateLimitPerHour   int      `json:"rate_limit_per_hour"`
}
type Event struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
type RoutingAttempt struct {
	ID               string    `json:"id"`
	RecipientID      string    `json:"recipient_id,omitempty"`
	Channel          string    `json:"channel"`
	Status           string    `json:"status"`
	Reason           string    `json:"reason,omitempty"`
	PolicyVersion    int64     `json:"policy_version"`
	RotationRevision int64     `json:"rotation_revision,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}
type Alert struct {
	ID               string           `json:"id"`
	RepositoryID     string           `json:"repository_id"`
	Revision         int64            `json:"revision"`
	Signal           Signal           `json:"signal"`
	PolicyID         string           `json:"policy_id"`
	PolicyVersion    int64            `json:"policy_version"`
	CoverageID       string           `json:"coverage_id,omitempty"`
	TeamID           string           `json:"team_id,omitempty"`
	RotationID       string           `json:"rotation_id,omitempty"`
	ResponseDeadline *time.Time       `json:"response_deadline,omitempty"`
	Status           string           `json:"status"`
	DuplicateCount   int              `json:"duplicate_count"`
	Events           []Event          `json:"events"`
	RoutingAttempts  []RoutingAttempt `json:"routing_attempts"`
	Gaps             []string         `json:"gaps"`
	CreatedBy        string           `json:"created_by"`
	CreatedAt        time.Time        `json:"created_at"`
	NonAuthority     []string         `json:"non_authority"`
}
type AttemptInput struct {
	ExpectedRevision int64  `json:"expected_revision"`
	RecipientID      string `json:"recipient_id"`
	Channel          string `json:"channel"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	PolicyVersion    int64  `json:"policy_version"`
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
func newID() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func validSignal(x Signal) bool {
	if x.SignalClass == "" || x.ResourceKind == "" || x.ResourceID == "" || x.Revision == "" || x.ObservedAt.IsZero() || x.CorrelationKey == "" || x.Summary == "" || x.AffectedUserCount < 0 {
		return false
	}
	switch x.SignalClass {
	case "reliability", "deployment", "security", "privacy", "dependency", "workflow", "user_impact":
	default:
		return false
	}
	switch x.Severity {
	case "critical", "high", "medium", "low":
	default:
		return false
	}
	for _, e := range x.Evidence {
		if e.Kind == "" || e.Reference == "" || e.Revision == "" {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(a Alert) error {
	p := s.path(a.RepositoryID, a.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e == nil {
		e = os.WriteFile(p, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, id string) (Alert, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Alert{}, ErrNotFound
	}
	var a Alert
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	return a, e
}
func (s *Store) list(repo string) ([]Alert, error) {
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	sort.Strings(files)
	out := []Alert{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var a Alert
		if x == nil {
			x = json.Unmarshal(b, &a)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, a)
	}
	return out, e
}
func policyVersion(p responsepolicies.Policy) responsepolicies.Version {
	for _, v := range p.Versions {
		if v.Number == p.CurrentVersion {
			return v
		}
	}
	return responsepolicies.Version{}
}
func activeRotation(rs []responserotations.Rotation, p responsepolicies.Policy, team string) (responserotations.Rotation, bool) {
	for _, r := range rs {
		if r.PolicyID == p.ID && r.PolicyVersion == p.CurrentVersion && r.TeamID == team && r.CurrentShift != nil {
			return r, true
		}
	}
	return responserotations.Rotation{}, false
}
func (s *Store) Create(repo, actor string, in Input, p responsepolicies.Policy, rs []responserotations.Rotation) (Alert, error) {
	if repo == "" || actor == "" || !validSignal(in.Signal) || in.RateLimitPerHour < 0 {
		return Alert{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	items, e := s.list(repo)
	if e != nil {
		return Alert{}, e
	}
	for _, a := range items {
		if a.Signal.CorrelationKey == in.Signal.CorrelationKey && a.PolicyID == p.ID && a.PolicyVersion == p.CurrentVersion && a.Status != "closed" {
			a.DuplicateCount++
			a.Revision++
			a.Events = append(a.Events, Event{newID(), "deduplicated", actor, "related signal " + in.Signal.Revision + " correlated without another page", now})
			return a, s.save(a)
		}
	}
	a := Alert{ID: newID(), RepositoryID: repo, Revision: 1, Signal: in.Signal, PolicyID: p.ID, PolicyVersion: p.CurrentVersion, Status: "pending", Events: []Event{}, RoutingAttempts: []RoutingAttempt{}, Gaps: []string{}, CreatedBy: actor, CreatedAt: now, NonAuthority: []string{"Alerts grant no repository, secret, communication, incident, deployment, environment, security, privacy, continuity, governance, or operational authority."}}
	v := policyVersion(p)
	var cov *responsepolicies.Coverage
	for i := range v.Coverage {
		c := &v.Coverage[i]
		if c.ResourceKind == in.Signal.ResourceKind && c.ResourceID == in.Signal.ResourceID && c.SignalClass == in.Signal.SignalClass && c.Severity == in.Signal.Severity {
			cov = c
			break
		}
	}
	if cov == nil {
		a.Status = "unroutable"
		a.Gaps = append(a.Gaps, "active policy has no matching coverage")
		return a, s.save(a)
	}
	a.CoverageID = cov.ID
	a.TeamID = cov.TeamID
	d := in.Signal.ObservedAt.Add(time.Duration(cov.Target.AcknowledgeMinutes) * time.Minute)
	a.ResponseDeadline = &d
	if in.Signal.ObservedAt.After(now.Add(5*time.Minute)) || now.Sub(in.Signal.ObservedAt) > 24*time.Hour {
		a.Status = "stale"
		a.Gaps = append(a.Gaps, "signal observation is stale or future-dated")
	}
	for _, k := range in.SuppressionKeys {
		if k == in.Signal.CorrelationKey {
			a.Status = "suppressed"
			a.Events = append(a.Events, Event{newID(), "suppressed", actor, "repository suppression key matched", now})
		}
	}
	for _, w := range in.MaintenanceWindows {
		if !now.Before(w.StartsAt) && now.Before(w.EndsAt) {
			a.Status = "maintenance"
			a.Events = append(a.Events, Event{newID(), "maintenance_window", actor, w.Reason, now})
		}
	}
	for _, ev := range in.Signal.Evidence {
		if !ev.Accessible {
			a.Gaps = append(a.Gaps, "inaccessible evidence: "+ev.Reference)
		}
	}
	if in.RateLimitPerHour > 0 {
		n := 0
		for _, x := range items {
			if x.TeamID == a.TeamID && x.CreatedAt.After(now.Add(-time.Hour)) {
				n++
			}
		}
		if n >= in.RateLimitPerHour {
			a.Status = "rate_limited"
			a.Gaps = append(a.Gaps, "repository alert rate limit reached")
		}
	}
	if a.Status == "pending" {
		if r, ok := activeRotation(rs, p, a.TeamID); ok {
			a.RotationID = r.ID
			u := r.CurrentShift.ResponderID
			a.Status = "delivering"
			a.RoutingAttempts = append(a.RoutingAttempts, RoutingAttempt{newID(), u, "inbox", "pending", "", p.CurrentVersion, r.Revision, now})
		} else {
			a.Status = "delivery_failed"
			a.Gaps = append(a.Gaps, "no current policy-pinned responder")
			a.RoutingAttempts = append(a.RoutingAttempts, RoutingAttempt{ID: newID(), Channel: "inbox", Status: "failed", Reason: "no eligible current responder", PolicyVersion: p.CurrentVersion, CreatedAt: now})
		}
	}
	return a, s.save(a)
}
func (s *Store) RecordAttempt(repo, id, actor string, in AttemptInput) (Alert, error) {
	if actor == "" || in.RecipientID == "" || in.Channel == "" || (in.Status != "delivered" && in.Status != "failed") || in.PolicyVersion < 1 {
		return Alert{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, id)
	if e != nil {
		return a, e
	}
	if a.Revision != in.ExpectedRevision {
		return a, ErrConflict
	}
	a.Revision++
	a.RoutingAttempts = append(a.RoutingAttempts, RoutingAttempt{newID(), in.RecipientID, in.Channel, in.Status, in.Reason, in.PolicyVersion, 0, s.now().UTC()})
	if in.PolicyVersion != a.PolicyVersion {
		a.Status = "policy_changed"
		a.Gaps = append(a.Gaps, "delivery used a policy version different from the evaluated alert")
	} else if in.Status == "delivered" {
		a.Status = "delivered"
	} else {
		a.Status = "delivery_failed"
		a.Gaps = append(a.Gaps, "delivery failed: "+in.Reason)
	}
	return a, s.save(a)
}
func (s *Store) Get(repo, id string) (Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo, recipient string) ([]Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if recipient == "" {
		return xs, e
	}
	out := []Alert{}
	for _, a := range xs {
		for _, r := range a.RoutingAttempts {
			if r.RecipientID == recipient {
				out = append(out, a)
				break
			}
		}
	}
	return out, e
}
