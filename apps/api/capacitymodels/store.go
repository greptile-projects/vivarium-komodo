// Package capacitymodels owns immutable, revision-exact capacity forecasts and their discussion.
package capacitymodels

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
	ErrNotFound = errors.New("capacity model not found")
	ErrInvalid  = errors.New("invalid capacity model")
	ErrConflict = errors.New("capacity model changed")
)

type Window struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}
type Evidence struct {
	ID                     string `json:"id"`
	Kind                   string `json:"kind"`
	ResourceID             string `json:"resource_id"`
	Revision               string `json:"revision"`
	Window                 Window `json:"observation_window"`
	Visibility             string `json:"visibility"`
	Summary                string `json:"summary,omitempty"`
	Sanitized              bool   `json:"sanitized"`
	InstrumentationVersion string `json:"instrumentation_version,omitempty"`
	Anomalous              bool   `json:"anomalous"`
	AnomalyReason          string `json:"anomaly_reason,omitempty"`
}
type Assumption struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`
	EvidenceIDs []string `json:"evidence_ids"`
	OwnerID     string   `json:"owner_id"`
	Uncertainty string   `json:"uncertainty"`
}
type Segment struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Demand      float64  `json:"demand"`
	Unit        string   `json:"unit"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type Saturation struct {
	ID              string    `json:"id"`
	Resource        string    `json:"resource"`
	Metric          string    `json:"metric"`
	Capacity        float64   `json:"capacity"`
	Unit            string    `json:"unit"`
	SaturatesAt     time.Time `json:"saturates_at"`
	HeadroomPercent float64   `json:"headroom_percent"`
	EvidenceIDs     []string  `json:"evidence_ids"`
	Explanation     string    `json:"explanation"`
}
type CostPoint struct {
	Demand   float64 `json:"demand"`
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`
	Period   string  `json:"period"`
}
type Scenario struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	DemandMultiplier float64     `json:"demand_multiplier"`
	Probability      string      `json:"probability"`
	SaturationIDs    []string    `json:"saturation_ids"`
	CostCurve        []CostPoint `json:"cost_curve"`
}
type Input struct {
	ObjectiveID       string       `json:"objective_id"`
	ObjectiveVersion  int64        `json:"objective_version"`
	Title             string       `json:"title"`
	ReleaseID         string       `json:"release_id"`
	ReleaseRevision   string       `json:"release_revision"`
	ForecastWindow    Window       `json:"forecast_window"`
	Method            string       `json:"method"`
	AuthorKind        string       `json:"author_kind"`
	Evidence          []Evidence   `json:"evidence"`
	Assumptions       []Assumption `json:"assumptions"`
	WorkloadSegments  []Segment    `json:"workload_segments"`
	SaturationPoints  []Saturation `json:"saturation_points"`
	Scenarios         []Scenario   `json:"scenarios"`
	Uncertainty       string       `json:"uncertainty"`
	Provenance        []string     `json:"provenance"`
	SupersedesModelID string       `json:"supersedes_model_id,omitempty"`
}
type ChallengeInput struct {
	ExpectedRevision int64    `json:"expected_revision"`
	ConclusionID     string   `json:"conclusion_id"`
	Body             string   `json:"body"`
	EvidenceIDs      []string `json:"evidence_ids"`
	AuthorKind       string   `json:"author_kind"`
}
type Challenge struct {
	ID string `json:"id"`
	ChallengeInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Gap struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	Reference string `json:"reference,omitempty"`
}
type Model struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Revision     int64  `json:"revision"`
	Input
	AuthorID     string      `json:"author_id"`
	CreatedAt    time.Time   `json:"created_at"`
	Challenges   []Challenge `json:"challenges"`
	Gaps         []Gap       `json:"gaps"`
	NonAuthority []string    `json:"non_authority"`
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
func newID() string             { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func validWindow(w Window) bool { return !w.StartsAt.IsZero() && w.EndsAt.After(w.StartsAt) }
func valid(in Input) bool {
	if in.ObjectiveID == "" || in.ObjectiveVersion < 1 || strings.TrimSpace(in.Title) == "" || in.ReleaseID == "" || in.ReleaseRevision == "" || !validWindow(in.ForecastWindow) || strings.TrimSpace(in.Method) == "" || !map[string]bool{"human": true, "read_only_agent": true}[in.AuthorKind] || len(in.Evidence) == 0 || len(in.Assumptions) == 0 || len(in.WorkloadSegments) == 0 || len(in.SaturationPoints) == 0 || len(in.Scenarios) == 0 || in.Uncertainty == "" || len(in.Provenance) == 0 {
		return false
	}
	ids := map[string]bool{}
	for _, e := range in.Evidence {
		if e.ID == "" || ids[e.ID] || !map[string]bool{"usage": true, "performance": true, "reliability": true, "deployment": true, "infrastructure": true, "dependency": true, "experiment": true, "roadmap": true}[e.Kind] || e.ResourceID == "" || e.Revision == "" || !validWindow(e.Window) || !map[string]bool{"public": true, "repository": true, "inaccessible": true}[e.Visibility] || e.Visibility != "inaccessible" && !e.Sanitized {
			return false
		}
		ids[e.ID] = true
	}
	for _, a := range in.Assumptions {
		if a.ID == "" || a.Statement == "" || a.OwnerID == "" || a.Uncertainty == "" || len(a.EvidenceIDs) == 0 {
			return false
		}
	}
	for _, s := range in.WorkloadSegments {
		if s.ID == "" || s.Name == "" || s.Demand <= 0 || s.Unit == "" || len(s.EvidenceIDs) == 0 {
			return false
		}
	}
	for _, s := range in.SaturationPoints {
		if s.ID == "" || s.Resource == "" || s.Metric == "" || s.Capacity <= 0 || s.Unit == "" || s.SaturatesAt.IsZero() || s.Explanation == "" || len(s.EvidenceIDs) == 0 {
			return false
		}
	}
	for _, s := range in.Scenarios {
		if s.ID == "" || s.Name == "" || s.DemandMultiplier <= 0 || s.Probability == "" || len(s.SaturationIDs) == 0 || len(s.CostCurve) == 0 {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Model, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Model{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m := Model{ID: newID(), RepositoryID: repo, Revision: 1, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()}
	return m, s.write(m)
}
func (s *Store) Challenge(repo, id, actor string, in ChallengeInput) (Model, error) {
	if actor == "" || in.ConclusionID == "" || strings.TrimSpace(in.Body) == "" || !map[string]bool{"human": true, "read_only_agent": true}[in.AuthorKind] {
		return Model{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.read(repo, id)
	if e != nil {
		return m, e
	}
	if in.ExpectedRevision != m.Revision {
		return m, ErrConflict
	}
	m.Revision++
	m.Challenges = append(m.Challenges, Challenge{ID: newID(), ChallengeInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return m, s.write(m)
}
func (s *Store) Get(repo, id string) (Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Model, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Model{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var m Model
		if x == nil {
			x = json.Unmarshal(b, &m)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, m)
	}
	return out, nil
}
func (s *Store) read(repo, id string) (Model, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Model{}, ErrNotFound
	}
	var m Model
	if e == nil {
		e = json.Unmarshal(b, &m)
	}
	return m, e
}
func (s *Store) write(m Model) error {
	d := filepath.Join(s.root, m.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(m, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "model-*.tmp")
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
		e = os.Rename(n, filepath.Join(d, m.ID+".json"))
	}
	return e
}

// Resolve derives visible shortcomings without altering the immutable forecast.
func Resolve(m Model) Model {
	m.Gaps = nil
	m.NonAuthority = []string{"Capacity models grant no spending, provider, repository, release, deployment, environment, credential, or operational authority."}
	versions := map[string]bool{}
	anomalous := map[string]bool{}
	evidence := map[string]bool{}
	for i := range m.Evidence {
		e := m.Evidence[i]
		evidence[e.ID] = true
		if e.InstrumentationVersion != "" {
			versions[e.InstrumentationVersion] = true
		}
		if e.Anomalous {
			anomalous[e.ID] = true
			m.Gaps = append(m.Gaps, Gap{Kind: "anomalous_event", Detail: "Evidence " + e.ID + " is retained as anomalous: " + e.AnomalyReason, Reference: e.ID})
		}
		if e.Visibility == "inaccessible" {
			m.Gaps = append(m.Gaps, Gap{Kind: "inaccessible_evidence", Detail: "Evidence " + e.ID + " is unavailable to this audience; its body is not projected.", Reference: e.ID})
			m.Evidence[i].Summary = ""
			m.Evidence[i].AnomalyReason = ""
		}
	}
	if len(versions) > 1 {
		m.Gaps = append(m.Gaps, Gap{Kind: "instrumentation_change", Detail: "The observation window spans multiple instrumentation versions."})
	}
	for _, s := range m.SaturationPoints {
		for _, id := range s.EvidenceIDs {
			if !evidence[id] {
				m.Gaps = append(m.Gaps, Gap{Kind: "missing_evidence", Detail: "Saturation " + s.ID + " cites unknown evidence " + id + ".", Reference: s.ID})
			}
		}
	}
	if len(m.Challenges) > 0 {
		m.Gaps = append(m.Gaps, Gap{Kind: "forecast_disagreement", Detail: "Collaborators have challenged one or more conclusions; original predictions remain intact."})
	}
	_ = anomalous
	return m
}
