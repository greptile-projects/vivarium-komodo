// Package runtimeprobes retains bounded, privacy-safe runtime collection
// requests and evidence. It is a coordination record, not a provider client or
// an environment credential.
package runtimeprobes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("runtime probe not found")
var ErrInvalid = errors.New("invalid runtime probe")
var ErrStopped = errors.New("runtime probe stopped")

type Preview struct {
	DataCategories []string `json:"data_categories"`
	EstimatedCost  float64  `json:"estimated_cost"`
	EstimatedLoad  string   `json:"estimated_load"`
	Audience       string   `json:"audience"`
	SamplingRate   float64  `json:"sampling_rate"`
	RetentionHours int      `json:"retention_hours"`
	PrivacyPolicy  string   `json:"privacy_policy"`
	SecurityPolicy string   `json:"security_policy"`
}
type Diagnostic struct {
	Name     string `json:"name,omitempty"`
	Path     string `json:"path,omitempty"`
	Revision string `json:"revision,omitempty"`
}
type Transformation struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}
type Capture struct {
	ID              string           `json:"id"`
	StartedAt       time.Time        `json:"started_at"`
	EndedAt         time.Time        `json:"ended_at"`
	Status          string           `json:"status"`
	Completeness    string           `json:"completeness"`
	RecordsExpected int              `json:"records_expected"`
	RecordsCaptured int              `json:"records_captured"`
	SanitizedData   []string         `json:"sanitized_data,omitempty"`
	Gaps            []string         `json:"gaps,omitempty"`
	Transformations []Transformation `json:"transformations"`
	Provenance      string           `json:"provenance"`
	ActorID         string           `json:"actor_id"`
}
type Action struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Detail    string    `json:"detail"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Probe struct {
	ID              string     `json:"id"`
	RepositoryID    string     `json:"repository_id"`
	WorkspaceID     string     `json:"workspace_id"`
	Environment     string     `json:"environment"`
	Kind            string     `json:"kind"`
	Scope           []string   `json:"scope"`
	Diagnostic      Diagnostic `json:"diagnostic"`
	Preview         Preview    `json:"preview"`
	Purpose         string     `json:"purpose"`
	Status          string     `json:"status"`
	RequestedBy     string     `json:"requested_by"`
	ApprovedBy      string     `json:"approved_by,omitempty"`
	ConsentActorIDs []string   `json:"consent_actor_ids"`
	ExpiresAt       time.Time  `json:"expires_at"`
	Captures        []Capture  `json:"captures"`
	Actions         []Action   `json:"actions"`
	Authority       []string   `json:"authority"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
type RequestInput struct {
	WorkspaceID     string     `json:"workspace_id"`
	Environment     string     `json:"environment"`
	Kind            string     `json:"kind"`
	Scope           []string   `json:"scope"`
	Diagnostic      Diagnostic `json:"diagnostic"`
	Preview         Preview    `json:"preview"`
	Purpose         string     `json:"purpose"`
	ConsentActorIDs []string   `json:"consent_actor_ids"`
	ExpiresAt       time.Time  `json:"expires_at"`
}
type CaptureInput struct {
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
	RecordsExpected int       `json:"records_expected"`
	Records         []string  `json:"records"`
	Gaps            []string  `json:"gaps"`
	Provenance      string    `json:"provenance"`
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
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func uniq(xs []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (s *Store) Request(repo, actor string, in RequestInput) (Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	validKind := map[string]bool{"logs": true, "traces": true, "profile": true, "state_snapshot": true, "dynamic_diagnostic": true}
	p := in.Preview
	if repo == "" || actor == "" || in.WorkspaceID == "" || in.Environment == "" || !validKind[in.Kind] || len(in.Scope) == 0 || in.Purpose == "" || len(p.DataCategories) == 0 || p.EstimatedCost < 0 || !map[string]bool{"low": true, "moderate": true, "high": true}[p.EstimatedLoad] || !map[string]bool{"repository": true, "participants": true}[p.Audience] || p.SamplingRate <= 0 || p.SamplingRate > 1 || p.RetentionHours < 1 || p.RetentionHours > 720 || p.PrivacyPolicy == "" || p.SecurityPolicy == "" || !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(24*time.Hour)) {
		return Probe{}, ErrInvalid
	}
	if in.Kind == "dynamic_diagnostic" && (in.Diagnostic.Name == "" || !strings.HasPrefix(in.Diagnostic.Path, ".komodo/") || strings.Contains(in.Diagnostic.Path, "..") || in.Diagnostic.Revision == "") {
		return Probe{}, ErrInvalid
	}
	v := Probe{ID: id(), RepositoryID: repo, WorkspaceID: in.WorkspaceID, Environment: in.Environment, Kind: in.Kind, Scope: uniq(in.Scope), Diagnostic: in.Diagnostic, Preview: p, Purpose: in.Purpose, Status: "pending_approval", RequestedBy: actor, ConsentActorIDs: uniq(in.ConsentActorIDs), ExpiresAt: in.ExpiresAt.UTC(), Authority: []string{}, CreatedAt: now, UpdatedAt: now}
	v.Actions = []Action{{ID: id(), Kind: "requested", Detail: "preview accepted; no environment or credential authority granted", ActorID: actor, CreatedAt: now}}
	return v, s.write(v)
}
func (s *Store) List(repo, workspace string) ([]Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Probe{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if e == nil && v.RepositoryID == repo && (workspace == "" || v.WorkspaceID == workspace) {
			s.derive(&v)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Get(repo, pid string) (Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e != nil || v.RepositoryID != repo {
		return Probe{}, ErrNotFound
	}
	s.derive(&v)
	return v, nil
}
func (s *Store) Decide(repo, pid, actor, decision, reason string) (Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e != nil || v.RepositoryID != repo {
		return Probe{}, ErrNotFound
	}
	if v.Status != "pending_approval" || !map[string]bool{"approved": true, "denied": true}[decision] || reason == "" {
		return Probe{}, ErrInvalid
	}
	now := s.now().UTC()
	v.Status = decision
	v.ApprovedBy = actor
	v.UpdatedAt = now
	v.Actions = append(v.Actions, Action{ID: id(), Kind: decision, Detail: reason, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) Control(repo, pid, actor, kind, detail string) (Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e != nil || v.RepositoryID != repo {
		return Probe{}, ErrNotFound
	}
	if !map[string]bool{"revoke": true, "overload": true, "narrow": true, "consent_revoked": true}[kind] || detail == "" {
		return Probe{}, ErrInvalid
	}
	now := s.now().UTC()
	switch kind {
	case "revoke":
		v.Status = "revoked"
	case "overload":
		v.Status = "overload_stopped"
	case "consent_revoked":
		v.Status = "consent_revoked"
	case "narrow":
		if v.Status != "approved" {
			return Probe{}, ErrStopped
		}
		v.Status = "narrowed"
	}
	v.UpdatedAt = now
	v.Actions = append(v.Actions, Action{ID: id(), Kind: kind, Detail: detail, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}

var secretPattern = regexp.MustCompile(`(?i)(authorization|cookie|password|passwd|secret|token|api[_-]?key)\s*[:=]\s*[^\s,;]+`)
var userPattern = regexp.MustCompile(`(?i)(email|user[_-]?id|ip|phone)\s*[:=]\s*[^\s,;]+`)

func sanitize(records []string) ([]string, []Transformation) {
	out := make([]string, 0, len(records))
	secrets, users := 0, 0
	for _, r := range records {
		n := secretPattern.ReplaceAllStringFunc(r, func(x string) string {
			secrets++
			return strings.SplitN(strings.NewReplacer(":", "=", "=", "=").Replace(x), "=", 2)[0] + "=[REDACTED]"
		})
		n = userPattern.ReplaceAllStringFunc(n, func(x string) string {
			users++
			return strings.SplitN(strings.NewReplacer(":", "=", "=", "=").Replace(x), "=", 2)[0] + "=[REDACTED_USER_DATA]"
		})
		if len(n) > 4096 {
			n = n[:4096]
		}
		out = append(out, n)
	}
	return out, []Transformation{{Kind: "secret_redaction", Count: secrets}, {Kind: "user_data_redaction", Count: users}, {Kind: "policy_sampling", Count: 0}}
}
func (s *Store) Capture(repo, pid, actor string, in CaptureInput) (Probe, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(pid)
	if e != nil || v.RepositoryID != repo {
		return Probe{}, ErrNotFound
	}
	now := s.now().UTC()
	s.derive(&v)
	if v.Status != "approved" && v.Status != "narrowed" {
		return Probe{}, ErrStopped
	}
	if in.StartedAt.IsZero() || in.EndedAt.Before(in.StartedAt) || in.EndedAt.After(now.Add(time.Minute)) || in.RecordsExpected < len(in.Records) || in.Provenance == "" {
		return Probe{}, ErrInvalid
	}
	data, tx := sanitize(in.Records)
	complete := len(data) == in.RecordsExpected && len(in.Gaps) == 0
	status, completeness := "captured", "complete"
	if !complete {
		status = "partial"
		completeness = "incomplete"
		if len(in.Gaps) == 0 {
			in.Gaps = []string{"expected records were not captured"}
		}
	}
	c := Capture{ID: id(), StartedAt: in.StartedAt.UTC(), EndedAt: in.EndedAt.UTC(), Status: status, Completeness: completeness, RecordsExpected: in.RecordsExpected, RecordsCaptured: len(data), SanitizedData: data, Gaps: uniq(in.Gaps), Transformations: tx, Provenance: in.Provenance, ActorID: actor}
	v.Captures = append(v.Captures, c)
	v.UpdatedAt = now
	v.Actions = append(v.Actions, Action{ID: id(), Kind: "capture_" + status, Detail: c.ID, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) derive(v *Probe) {
	if v.Status == "approved" || v.Status == "narrowed" {
		if !v.ExpiresAt.After(s.now().UTC()) {
			v.Status = "expired"
		}
	}
}
func (s *Store) read(pid string) (Probe, error) {
	var v Probe
	b, e := os.ReadFile(filepath.Join(s.root, pid+".json"))
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Probe) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
