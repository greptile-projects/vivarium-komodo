// Package performancegoals owns versioned, evidence-backed performance contracts.
package performancegoals

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("performance goal not found")
	ErrInvalid  = errors.New("invalid performance goal")
	ErrConflict = errors.New("performance goal version conflict")
)

type Range struct {
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
}
type Environment struct {
	Name         string `json:"name"`
	OS           string `json:"os,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Runtime      string `json:"runtime,omitempty"`
	Hardware     string `json:"hardware,omitempty"`
	Dataset      string `json:"dataset,omitempty"`
	Digest       string `json:"digest"`
}
type Metric struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Unit               string     `json:"unit"`
	Direction          string     `json:"direction"`
	Baseline           *float64   `json:"baseline,omitempty"`
	Target             Range      `json:"target"`
	Budget             *float64   `json:"budget,omitempty"`
	EnvironmentDigest  string     `json:"environment_digest"`
	BaselineMeasuredAt *time.Time `json:"baseline_measured_at,omitempty"`
	BaselineSource     string     `json:"baseline_source,omitempty"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label,omitempty"`
	Status     string `json:"status,omitempty"`
}
type Version struct {
	Number                 int64         `json:"number"`
	SubjectKind            string        `json:"subject_kind"`
	SubjectID              string        `json:"subject_id,omitempty"`
	Title                  string        `json:"title"`
	Workloads              []string      `json:"workloads"`
	Metrics                []Metric      `json:"metrics"`
	CorrectnessConstraints []string      `json:"correctness_constraints"`
	Environments           []Environment `json:"supported_environments"`
	OwnerIDs               []string      `json:"owner_ids"`
	Links                  []Link        `json:"links"`
	BaselineMaxAgeDays     int           `json:"baseline_max_age_days"`
	ChangeReason           string        `json:"change_reason"`
	AuthorID               string        `json:"author_id"`
	CreatedAt              time.Time     `json:"created_at"`
}
type Measurement struct {
	ID                string    `json:"id"`
	Version           int64     `json:"version"`
	MetricID          string    `json:"metric_id"`
	Value             float64   `json:"value"`
	EnvironmentDigest string    `json:"environment_digest"`
	Revision          string    `json:"revision,omitempty"`
	Source            string    `json:"source"`
	ActorID           string    `json:"actor_id"`
	MeasuredAt        time.Time `json:"measured_at"`
	CreatedAt         time.Time `json:"created_at"`
}
type Status struct {
	MetricID      string `json:"metric_id"`
	State         string `json:"state"`
	Detail        string `json:"detail"`
	MeasurementID string `json:"measurement_id,omitempty"`
}
type Goal struct {
	ID             string        `json:"id"`
	RepositoryID   string        `json:"repository_id"`
	CurrentVersion int64         `json:"current_version"`
	Versions       []Version     `json:"versions"`
	Measurements   []Measurement `json:"measurements"`
	Statuses       []Status      `json:"statuses"`
	Conflicts      []string      `json:"conflicts"`
}
type VersionInput struct {
	SubjectKind            string        `json:"subject_kind"`
	SubjectID              string        `json:"subject_id"`
	Title                  string        `json:"title"`
	Workloads              []string      `json:"workloads"`
	Metrics                []Metric      `json:"metrics"`
	CorrectnessConstraints []string      `json:"correctness_constraints"`
	Environments           []Environment `json:"supported_environments"`
	OwnerIDs               []string      `json:"owner_ids"`
	Links                  []Link        `json:"links"`
	BaselineMaxAgeDays     int           `json:"baseline_max_age_days"`
	ChangeReason           string        `json:"change_reason"`
}
type MeasurementInput struct {
	Version           int64     `json:"version"`
	MetricID          string    `json:"metric_id"`
	Value             float64   `json:"value"`
	EnvironmentDigest string    `json:"environment_digest"`
	Revision          string    `json:"revision"`
	Source            string    `json:"source"`
	MeasuredAt        time.Time `json:"measured_at"`
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
func clean(xs []string) bool {
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
func valid(in VersionInput) bool {
	allowed := map[string]bool{"repository": true, "release": true, "user_journey": true, "api": true, "command": true, "service": true}
	if !allowed[in.SubjectKind] || strings.TrimSpace(in.Title) == "" || !clean(in.Workloads) || !clean(in.CorrectnessConstraints) || !clean(in.OwnerIDs) || len(in.Metrics) == 0 || len(in.Environments) == 0 || in.BaselineMaxAgeDays < 1 {
		return false
	}
	env := map[string]bool{}
	for _, e := range in.Environments {
		if e.Name == "" || e.Digest == "" {
			return false
		}
		env[e.Digest] = true
	}
	ids := map[string]bool{}
	for _, m := range in.Metrics {
		if m.ID == "" || m.Name == "" || m.Unit == "" || !env[m.EnvironmentDigest] || (m.Direction != "lower" && m.Direction != "higher" && m.Direction != "range") || ids[m.ID] || m.Target.Minimum == nil && m.Target.Maximum == nil {
			return false
		}
		ids[m.ID] = true
	}
	for _, l := range in.Links {
		if !map[string]bool{"issue": true, "incident": true, "preview": true, "release": true, "decision": true}[l.Kind] || l.ResourceID == "" {
			return false
		}
	}
	return true
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in VersionInput) (Goal, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Goal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g := Goal{ID: id(), RepositoryID: repo}
	return s.publish(g, actor, 0, in)
}
func (s *Store) Revise(repo, gid, actor string, expected int64, in VersionInput) (Goal, error) {
	if !valid(in) {
		return Goal{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	g, e := s.read(repo, gid)
	if e != nil {
		return g, e
	}
	return s.publish(g, actor, expected, in)
}
func (s *Store) publish(g Goal, actor string, expected int64, in VersionInput) (Goal, error) {
	if g.CurrentVersion != expected {
		return g, ErrConflict
	}
	v := Version{Number: expected + 1, SubjectKind: in.SubjectKind, SubjectID: strings.TrimSpace(in.SubjectID), Title: strings.TrimSpace(in.Title), Workloads: in.Workloads, Metrics: in.Metrics, CorrectnessConstraints: in.CorrectnessConstraints, Environments: in.Environments, OwnerIDs: in.OwnerIDs, Links: in.Links, BaselineMaxAgeDays: in.BaselineMaxAgeDays, ChangeReason: strings.TrimSpace(in.ChangeReason), AuthorID: actor, CreatedAt: s.now().UTC()}
	g.CurrentVersion = v.Number
	g.Versions = append(g.Versions, v)
	return g, s.write(g)
}
func (s *Store) Measure(repo, gid, actor string, in MeasurementInput) (Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, e := s.read(repo, gid)
	if e != nil {
		return g, e
	}
	if in.Version != g.CurrentVersion || in.Source == "" || in.EnvironmentDigest == "" {
		return g, ErrInvalid
	}
	v := g.Versions[len(g.Versions)-1]
	ok := false
	for _, m := range v.Metrics {
		if m.ID == in.MetricID {
			ok = true
		}
	}
	if !ok {
		return g, ErrInvalid
	}
	if in.MeasuredAt.IsZero() {
		in.MeasuredAt = s.now().UTC()
	}
	g.Measurements = append(g.Measurements, Measurement{ID: id(), Version: in.Version, MetricID: in.MetricID, Value: in.Value, EnvironmentDigest: in.EnvironmentDigest, Revision: in.Revision, Source: in.Source, ActorID: actor, MeasuredAt: in.MeasuredAt.UTC(), CreatedAt: s.now().UTC()})
	return g, s.write(g)
}
func (s *Store) Get(repo, gid string) (Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, gid)
}
func (s *Store) List(repo string) ([]Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Goal{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var g Goal
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
func (s *Store) read(repo, gid string) (Goal, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, gid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Goal{}, ErrNotFound
	}
	var g Goal
	if e == nil {
		e = json.Unmarshal(b, &g)
	}
	return g, e
}
func (s *Store) write(g Goal) error {
	d := filepath.Join(s.root, g.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(g, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, "goal-*.tmp")
	if e != nil {
		return e
	}
	n := tmp.Name()
	defer os.Remove(n)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if x := tmp.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(d, g.ID+".json"))
	}
	return e
}

func Resolve(g Goal, now time.Time) Goal {
	g.Statuses = nil
	g.Conflicts = nil
	if len(g.Versions) == 0 {
		return g
	}
	v := g.Versions[len(g.Versions)-1]
	for _, m := range v.Metrics {
		st := Status{MetricID: m.ID, State: "missing_measurement", Detail: "No measurement is recorded for this goal version."}
		var latest *Measurement
		for i := range g.Measurements {
			x := &g.Measurements[i]
			if x.Version == v.Number && x.MetricID == m.ID && (latest == nil || x.MeasuredAt.After(latest.MeasuredAt)) {
				latest = x
			}
		}
		if latest != nil {
			st.MeasurementID = latest.ID
			if latest.EnvironmentDigest != m.EnvironmentDigest {
				st.State = "incomparable_environment"
				st.Detail = "The latest measurement uses a different environment than the baseline."
			} else if m.BaselineMeasuredAt != nil && now.Sub(*m.BaselineMeasuredAt) > time.Duration(v.BaselineMaxAgeDays)*24*time.Hour {
				st.State = "stale_baseline"
				st.Detail = "The declared baseline is older than the allowed baseline age."
			} else {
				st.State = "measured"
				st.Detail = "The latest measurement is comparable to the declared baseline."
			}
		}
		g.Statuses = append(g.Statuses, st)
	}
	for i, a := range v.Metrics {
		for _, b := range v.Metrics[i+1:] {
			if a.Name == b.Name && a.Unit == b.Unit && a.EnvironmentDigest == b.EnvironmentDigest && a.Target.Minimum != nil && b.Target.Maximum != nil && *a.Target.Minimum > *b.Target.Maximum {
				g.Conflicts = append(g.Conflicts, "Conflicting target ranges for "+a.Name)
			}
		}
	}
	return g
}
