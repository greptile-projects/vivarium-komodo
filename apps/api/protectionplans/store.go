// Package protectionplans retains metadata-only evidence for protected recovery inputs.
package protectionplans

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
	ErrNotFound = errors.New("protection plan not found")
	ErrInvalid  = errors.New("invalid protection plan")
	ErrConflict = errors.New("protection plan version conflict")
)

type Destination struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Region       string `json:"region"`
	Jurisdiction string `json:"jurisdiction"`
	Authorized   bool   `json:"authorized"`
}
type VersionInput struct {
	ObjectiveID        string        `json:"objective_id"`
	ObjectiveVersion   int64         `json:"objective_version"`
	ResourceIDs        []string      `json:"resource_ids"`
	EnvironmentID      string        `json:"environment_id,omitempty"`
	Mode               string        `json:"mode"`
	Schedule           string        `json:"schedule"`
	MaximumAgeSeconds  int64         `json:"maximum_age_seconds"`
	Encryption         string        `json:"encryption"`
	KeyReference       string        `json:"key_reference"`
	AccessScope        []string      `json:"access_scope"`
	Destinations       []Destination `json:"destinations"`
	Retention          string        `json:"retention"`
	ChecksumAlgorithm  string        `json:"checksum_algorithm"`
	ValidationCriteria []string      `json:"validation_criteria"`
	CostLimit          float64       `json:"cost_limit"`
	Currency           string        `json:"currency"`
	ChangeReason       string        `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ManifestResource struct {
	ResourceID         string            `json:"resource_id"`
	SourceVersion      string            `json:"source_version"`
	Provenance         string            `json:"provenance"`
	DependencyVersions map[string]string `json:"dependency_versions"`
	ObjectCount        int64             `json:"object_count"`
	ByteCount          int64             `json:"byte_count"`
	Checksum           string            `json:"checksum"`
	Complete           bool              `json:"complete"`
	SourceState        string            `json:"source_state"`
}
type Validation struct {
	CompletenessVerified  bool      `json:"completeness_verified"`
	ChecksumVerified      bool      `json:"checksum_verified"`
	DecryptionVerified    bool      `json:"decryption_verified"`
	KeyAvailable          bool      `json:"key_available"`
	DestinationAuthorized bool      `json:"destination_authorized"`
	ValidatedAt           time.Time `json:"validated_at"`
	EvidenceDigest        string    `json:"evidence_digest"`
}
type CaptureInput struct {
	IdempotencyKey string             `json:"idempotency_key"`
	PlanVersion    int64              `json:"plan_version"`
	StartedAt      time.Time          `json:"started_at"`
	CapturedAt     time.Time          `json:"captured_at"`
	Resources      []ManifestResource `json:"resources"`
	Validation     Validation         `json:"validation"`
	Cost           float64            `json:"cost"`
	Failure        string             `json:"failure,omitempty"`
}
type Capture struct {
	ID string `json:"id"`
	CaptureInput
	ActorID     string    `json:"actor_id"`
	RecordedAt  time.Time `json:"recorded_at"`
	Recoverable bool      `json:"recoverable"`
	Fresh       bool      `json:"fresh"`
	Failures    []string  `json:"failures"`
}
type Plan struct {
	ID                         string    `json:"id"`
	RepositoryID               string    `json:"repository_id"`
	CurrentVersion             int64     `json:"current_version"`
	Versions                   []Version `json:"versions"`
	Captures                   []Capture `json:"captures"`
	Coverage                   []string  `json:"coverage"`
	MissingResources           []string  `json:"missing_resources"`
	LatestRecoverableCaptureID string    `json:"latest_recoverable_capture_id,omitempty"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func stringsOK(v []string, required bool) bool {
	if (required && len(v) == 0) || len(v) > 100 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}
func valid(in VersionInput) bool {
	if in.ObjectiveID == "" || in.ObjectiveVersion < 1 || !stringsOK(in.ResourceIDs, true) || !stringsOK(in.AccessScope, true) || !stringsOK(in.ValidationCriteria, true) || in.Schedule == "" || in.MaximumAgeSeconds < 1 || in.Retention == "" || in.ChecksumAlgorithm == "" || in.ChangeReason == "" || in.CostLimit < 0 || in.Currency == "" {
		return false
	}
	if in.Mode != "snapshot" && in.Mode != "replica" {
		return false
	}
	if in.Encryption == "" || in.Encryption == "none" || in.KeyReference == "" || len(in.Destinations) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range in.ResourceIDs {
		if seen[x] {
			return false
		}
		seen[x] = true
	}
	seen = map[string]bool{}
	for _, d := range in.Destinations {
		if d.ID == "" || seen[d.ID] || d.Kind == "" || d.Region == "" || d.Jurisdiction == "" {
			return false
		}
		seen[d.ID] = true
	}
	return true
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) load(repo, id string) (Plan, error) {
	var x Plan
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(x Plan) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e != nil {
		return e
	}
	tmp := p + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, p)
}
func derive(x Plan, now time.Time) Plan {
	x.Coverage = nil
	x.MissingResources = nil
	x.LatestRecoverableCaptureID = ""
	if len(x.Versions) == 0 {
		return x
	}
	v := x.Versions[len(x.Versions)-1]
	covered := map[string]bool{}
	var latestRecoverableAt time.Time
	for i := range x.Captures {
		c := &x.Captures[i]
		c.Failures = nil
		c.Recoverable = false
		c.Fresh = false
		if c.PlanVersion != v.Number {
			c.Failures = append(c.Failures, "stale_plan_version")
		}
		if c.Failure != "" {
			c.Failures = append(c.Failures, "capture_failed")
		}
		if len(c.Resources) != len(v.ResourceIDs) {
			c.Failures = append(c.Failures, "incomplete_capture")
		}
		got := map[string]bool{}
		for _, r := range c.Resources {
			got[r.ResourceID] = true
			if !r.Complete {
				c.Failures = append(c.Failures, "incomplete_capture")
			}
			if r.SourceState != "committed" {
				c.Failures = append(c.Failures, "deleted_or_uncommitted_source")
			}
			if r.SourceVersion == "" || r.Provenance == "" || r.Checksum == "" || r.ObjectCount < 1 || r.ByteCount < 1 {
				c.Failures = append(c.Failures, "incomplete_manifest")
			}
		}
		for _, id := range v.ResourceIDs {
			if !got[id] {
				c.Failures = append(c.Failures, "missing_resource")
			}
		}
		if !c.Validation.CompletenessVerified {
			c.Failures = append(c.Failures, "completeness_unverified")
		}
		if !c.Validation.ChecksumVerified {
			c.Failures = append(c.Failures, "corruption_or_checksum_unverified")
		}
		if !c.Validation.DecryptionVerified {
			c.Failures = append(c.Failures, "decryption_unverified")
		}
		if !c.Validation.KeyAvailable {
			c.Failures = append(c.Failures, "key_unavailable")
		}
		if !c.Validation.DestinationAuthorized {
			c.Failures = append(c.Failures, "unauthorized_destination")
		}
		for _, destination := range v.Destinations {
			if !destination.Authorized {
				c.Failures = append(c.Failures, "unauthorized_destination")
			}
		}
		if c.Validation.ValidatedAt.IsZero() || c.Validation.EvidenceDigest == "" || c.Validation.ValidatedAt.Before(c.CapturedAt) || c.Validation.ValidatedAt.After(now) {
			c.Failures = append(c.Failures, "missing_validation_evidence")
		}
		if c.Cost > v.CostLimit {
			c.Failures = append(c.Failures, "cost_limit_exceeded")
		}
		c.Fresh = !c.CapturedAt.IsZero() && !c.CapturedAt.After(now) && now.Sub(c.CapturedAt) <= time.Duration(v.MaximumAgeSeconds)*time.Second
		if !c.Fresh {
			c.Failures = append(c.Failures, "stale_capture")
		}
		c.Failures = unique(c.Failures)
		c.Recoverable = len(c.Failures) == 0
		if c.Recoverable {
			if x.LatestRecoverableCaptureID == "" || c.CapturedAt.After(latestRecoverableAt) {
				x.LatestRecoverableCaptureID = c.ID
				latestRecoverableAt = c.CapturedAt
			}
			for id := range got {
				covered[id] = true
			}
		}
	}
	for _, id := range v.ResourceIDs {
		if covered[id] {
			x.Coverage = append(x.Coverage, id)
		} else {
			x.MissingResources = append(x.MissingResources, id)
		}
	}
	sort.Strings(x.Coverage)
	sort.Strings(x.MissingResources)
	return x
}
func unique(v []string) []string {
	m := map[string]bool{}
	o := []string{}
	for _, x := range v {
		if !m[x] {
			m[x] = true
			o = append(o, x)
		}
	}
	return o
}
func (s *Store) Create(repo, actor string, in VersionInput) (Plan, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	x := Plan{ID: newid(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: now}}, Captures: []Capture{}}
	if e := s.save(x); e != nil {
		return Plan{}, e
	}
	return derive(x, now), nil
}
func (s *Store) Revise(repo, id, actor string, expected int64, in VersionInput) (Plan, error) {
	if actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return Plan{}, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	if e = s.save(x); e != nil {
		return Plan{}, e
	}
	return derive(x, s.now().UTC()), nil
}
func (s *Store) Capture(repo, id, actor string, in CaptureInput) (Plan, error) {
	if actor == "" || in.IdempotencyKey == "" || in.PlanVersion < 1 || in.StartedAt.IsZero() || in.CapturedAt.IsZero() || in.CapturedAt.Before(in.StartedAt) || in.Cost < 0 {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	if e != nil {
		return x, e
	}
	for _, c := range x.Captures {
		if c.IdempotencyKey == in.IdempotencyKey {
			b, _ := json.Marshal(c.CaptureInput)
			q, _ := json.Marshal(in)
			if string(b) != string(q) {
				return Plan{}, ErrConflict
			}
			return derive(x, s.now().UTC()), nil
		}
	}
	x.Captures = append(x.Captures, Capture{ID: newid(), CaptureInput: in, ActorID: actor, RecordedAt: s.now().UTC()})
	if e = s.save(x); e != nil {
		return Plan{}, e
	}
	return derive(x, s.now().UTC()), nil
}
func (s *Store) Get(repo, id string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, id)
	return derive(x, s.now().UTC()), e
}
func (s *Store) List(repo string) ([]Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Plan{}, nil
	}
	if e != nil {
		return nil, e
	}
	o := []Plan{}
	for _, f := range es {
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, q := s.load(repo, strings.TrimSuffix(f.Name(), ".json"))
		if q != nil {
			return nil, q
		}
		o = append(o, derive(x, s.now().UTC()))
	}
	sort.Slice(o, func(i, j int) bool { return o[i].Versions[0].CreatedAt.Before(o[j].Versions[0].CreatedAt) })
	return o, nil
}
