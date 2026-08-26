// Package signalcontracts owns reviewable, versioned definitions of evidence a system may produce.
package signalcontracts

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
	ErrNotFound = errors.New("signal contract not found")
	ErrInvalid  = errors.New("invalid signal contract")
	ErrConflict = errors.New("signal contract version conflict")
)

type Field struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Description    string `json:"description"`
	Unit           string `json:"unit,omitempty"`
	Sensitive      bool   `json:"sensitive"`
	Classification string `json:"classification,omitempty"`
}
type Dimension struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Bounded       bool   `json:"bounded"`
	MaximumValues int    `json:"maximum_values,omitempty"`
	Sensitive     bool   `json:"sensitive"`
}
type Sampling struct {
	Strategy  string  `json:"strategy"`
	Rate      float64 `json:"rate"`
	Rationale string  `json:"rationale"`
}
type Aggregation struct {
	Method        string `json:"method"`
	WindowSeconds int    `json:"window_seconds"`
	Temporality   string `json:"temporality"`
}
type Correlation struct {
	Field       string `json:"field"`
	Target      string `json:"target"`
	Propagation string `json:"propagation"`
}
type Volume struct {
	EventsPerSecond float64 `json:"events_per_second"`
	BytesPerEvent   int64   `json:"bytes_per_event"`
	PeakMultiplier  float64 `json:"peak_multiplier"`
}
type Quality struct {
	MaximumDelaySeconds int     `json:"maximum_delay_seconds"`
	MinimumCompleteness float64 `json:"minimum_completeness"`
	MaximumErrorRate    float64 `json:"maximum_error_rate"`
}
type SourceReference struct {
	Path       string `json:"path"`
	Symbol     string `json:"symbol"`
	Revision   string `json:"revision"`
	Accessible bool   `json:"accessible"`
}
type ServiceBoundary struct {
	Service  string `json:"service"`
	Boundary string `json:"boundary"`
	Revision string `json:"revision"`
	OwnerID  string `json:"owner_id"`
}
type Dependency struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Revision  string `json:"revision"`
	Supported bool   `json:"supported"`
}
type Impact struct {
	Privacy             string   `json:"privacy"`
	Security            string   `json:"security"`
	Residency           string   `json:"residency"`
	Performance         string   `json:"performance"`
	Cardinality         string   `json:"cardinality"`
	Storage             string   `json:"storage"`
	Cost                string   `json:"cost"`
	RetentionDays       int      `json:"retention_days"`
	ResidencyRegions    []string `json:"residency_regions"`
	MonthlyStorageBytes int64    `json:"monthly_storage_bytes"`
	MonthlyCost         float64  `json:"monthly_cost"`
	Currency            string   `json:"currency"`
}
type Evidence struct {
	URI        string `json:"uri"`
	Revision   string `json:"revision"`
	Claim      string `json:"claim"`
	Accessible bool   `json:"accessible"`
}
type Alternative struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	Sampling       Sampling    `json:"sampling"`
	Aggregation    Aggregation `json:"aggregation"`
	RetentionDays  int         `json:"retention_days"`
	ExpectedVolume Volume      `json:"expected_volume"`
	Impact         Impact      `json:"impact"`
	Evidence       []Evidence  `json:"evidence"`
}
type Input struct {
	Name           string            `json:"name"`
	SignalType     string            `json:"signal_type"`
	Purpose        string            `json:"purpose"`
	Schema         []Field           `json:"schema"`
	Unit           string            `json:"unit"`
	Dimensions     []Dimension       `json:"dimensions"`
	Sampling       Sampling          `json:"sampling"`
	Aggregation    Aggregation       `json:"aggregation"`
	Correlation    []Correlation     `json:"correlation"`
	RetentionDays  int               `json:"retention_days"`
	ExpectedVolume Volume            `json:"expected_volume"`
	Quality        Quality           `json:"quality_thresholds"`
	OwnerIDs       []string          `json:"owner_ids"`
	Consumers      []string          `json:"consumers"`
	Sources        []SourceReference `json:"source_symbols"`
	Boundaries     []ServiceBoundary `json:"service_boundaries"`
	Collector      Dependency        `json:"collector"`
	Dependencies   []Dependency      `json:"dependencies"`
	Impact         Impact            `json:"impact"`
	Alternatives   []Alternative     `json:"alternatives"`
	Assumptions    []string          `json:"assumptions"`
	ChangeReason   string            `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Challenge struct {
	ID         string     `json:"id"`
	Version    int64      `json:"version"`
	AuthorID   string     `json:"author_id"`
	Agent      bool       `json:"agent"`
	Assumption string     `json:"assumption"`
	Position   string     `json:"position"`
	Evidence   []Evidence `json:"evidence"`
	CreatedAt  time.Time  `json:"created_at"`
}
type Finding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
	Subject  string `json:"subject"`
	OwnerID  string `json:"owner_id"`
}
type Comparison struct {
	AlternativeID       string  `json:"alternative_id"`
	Name                string  `json:"name"`
	EventsPerSecond     float64 `json:"events_per_second"`
	RetentionDays       int     `json:"retention_days"`
	MonthlyStorageBytes int64   `json:"monthly_storage_bytes"`
	MonthlyCost         float64 `json:"monthly_cost"`
	Currency            string  `json:"currency"`
}
type Contract struct {
	ID             string       `json:"id"`
	RepositoryID   string       `json:"repository_id"`
	CurrentVersion int64        `json:"current_version"`
	Versions       []Version    `json:"versions"`
	Challenges     []Challenge  `json:"challenges"`
	Findings       []Finding    `json:"findings"`
	Comparison     []Comparison `json:"comparison"`
	Complete       bool         `json:"complete"`
	Blocked        bool         `json:"blocked"`
	NonAuthority   []string     `json:"non_authority"`
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
func id() string         { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func text(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 4000 }
func list(xs []string) bool {
	if len(xs) == 0 || len(xs) > 100 {
		return false
	}
	for _, x := range xs {
		if !text(x) {
			return false
		}
	}
	return true
}
func validEvidence(xs []Evidence) bool {
	for _, e := range xs {
		if !text(e.URI) || !text(e.Revision) || !text(e.Claim) {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	if !text(in.Name) || !map[string]bool{"metric": true, "log": true, "trace": true, "profile": true, "event": true}[in.SignalType] || !text(in.Purpose) || len(in.Schema) == 0 || !text(in.Unit) || !list(in.OwnerIDs) || !list(in.Consumers) || len(in.Sources) == 0 || len(in.Boundaries) == 0 || !text(in.Collector.ID) || !text(in.Collector.Revision) || in.RetentionDays < 1 || in.ExpectedVolume.EventsPerSecond < 0 || in.ExpectedVolume.BytesPerEvent < 1 || in.ExpectedVolume.PeakMultiplier < 1 || !text(in.Sampling.Strategy) || in.Sampling.Rate <= 0 || in.Sampling.Rate > 1 || !text(in.Aggregation.Method) || in.Aggregation.WindowSeconds < 1 || !text(in.ChangeReason) {
		return false
	}
	seen := map[string]bool{}
	for _, f := range in.Schema {
		if !text(f.Name) || seen[f.Name] || !text(f.Type) || !text(f.Description) {
			return false
		}
		seen[f.Name] = true
	}
	for _, a := range in.Alternatives {
		if !text(a.ID) || seen["alt:"+a.ID] || !text(a.Name) || !validEvidence(a.Evidence) {
			return false
		}
		seen["alt:"+a.ID] = true
	}
	return true
}
func (s *Store) Create(repo, actor string, in Input) (Contract, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Contract{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publish(Contract{ID: id(), RepositoryID: repo}, actor, 0, in)
}
func (s *Store) Revise(repo, cid, actor string, expected int64, in Input) (Contract, error) {
	if actor == "" || !valid(in) {
		return Contract{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	return s.publish(c, actor, expected, in)
}
func (s *Store) publish(c Contract, actor string, expected int64, in Input) (Contract, error) {
	if c.CurrentVersion != expected {
		return c, ErrConflict
	}
	c.CurrentVersion++
	c.Versions = append(c.Versions, Version{Number: c.CurrentVersion, Input: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return c, s.write(c)
}
func (s *Store) Challenge(repo, cid, actor string, agent bool, version int64, assumption, position string, evidence []Evidence) (Contract, error) {
	if actor == "" || !text(assumption) || !text(position) || !validEvidence(evidence) || len(evidence) == 0 {
		return Contract{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	if version < 1 || version > c.CurrentVersion {
		return c, ErrInvalid
	}
	c.Challenges = append(c.Challenges, Challenge{ID: id(), Version: version, AuthorID: actor, Agent: agent, Assumption: assumption, Position: position, Evidence: evidence, CreatedAt: s.now().UTC()})
	return c, s.write(c)
}
func (s *Store) Get(repo, cid string) (Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, cid)
}
func (s *Store) List(repo string) ([]Contract, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Contract{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var c Contract
		if x == nil {
			x = json.Unmarshal(b, &c)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Store) read(repo, cid string) (Contract, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, cid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Contract{}, ErrNotFound
	}
	var c Contract
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func (s *Store) write(c Contract) error {
	d := filepath.Join(s.root, c.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "contract-*.tmp")
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
		e = os.Rename(n, filepath.Join(d, c.ID+".json"))
	}
	return e
}

func Resolve(c Contract) Contract {
	c.Findings = nil
	c.Comparison = nil
	c.Complete = false
	c.Blocked = false
	c.NonAuthority = []string{"Signal contracts and reviews grant no repository, telemetry, collector, secret, deployment, environment, spending, or operational authority."}
	if len(c.Versions) == 0 {
		return c
	}
	v := c.Versions[len(c.Versions)-1]
	owner := v.AuthorID
	for _, f := range v.Schema {
		if f.Sensitive && f.Classification == "" {
			c.Findings = append(c.Findings, Finding{"sensitive_field", "blocking", "Sensitive schema field lacks a classification.", f.Name, owner})
		}
	}
	for _, d := range v.Dimensions {
		if !d.Bounded || d.MaximumValues < 1 {
			c.Findings = append(c.Findings, Finding{"unbounded_dimension", "blocking", "Dimension has no enforceable cardinality bound.", d.Name, owner})
		}
		if d.Sensitive {
			c.Findings = append(c.Findings, Finding{"sensitive_dimension", "incomplete", "Sensitive dimension requires privacy and security review.", d.Name, owner})
		}
	}
	if !v.Collector.Supported {
		c.Findings = append(c.Findings, Finding{"unsupported_collector", "blocking", "The exact collector revision is not supported.", v.Collector.ID, owner})
	}
	for _, d := range v.Dependencies {
		if !d.Supported {
			c.Findings = append(c.Findings, Finding{"changed_dependency", "blocking", "A pinned dependency is unavailable or changed.", d.Kind + ":" + d.ID + "@" + d.Revision, owner})
		}
	}
	for _, s := range v.Sources {
		if !s.Accessible {
			c.Findings = append(c.Findings, Finding{"inaccessible_source", "incomplete", "A source symbol is inaccessible to reviewers.", s.Path + ":" + s.Symbol, owner})
		}
	}
	if v.Quality.MaximumDelaySeconds < 1 || v.Quality.MinimumCompleteness <= 0 || v.Quality.MaximumErrorRate < 0 {
		c.Findings = append(c.Findings, Finding{"missing_quality_threshold", "incomplete", "Latency, completeness, and error thresholds must be reviewable.", v.Name, owner})
	}
	imp := v.Impact
	if !text(imp.Privacy) || !text(imp.Security) || !text(imp.Residency) || !text(imp.Performance) || !text(imp.Cardinality) || !text(imp.Storage) || !text(imp.Cost) || imp.RetentionDays != v.RetentionDays || len(imp.ResidencyRegions) == 0 || imp.MonthlyStorageBytes < 1 || imp.MonthlyCost < 0 || !text(imp.Currency) {
		c.Findings = append(c.Findings, Finding{"incomplete_impact_preview", "incomplete", "Privacy, security, residency, performance, cardinality, storage, and cost effects require quantified preview.", v.Name, owner})
	}
	c.Comparison = append(c.Comparison, Comparison{"proposed", v.Name, v.ExpectedVolume.EventsPerSecond, v.RetentionDays, v.Impact.MonthlyStorageBytes, v.Impact.MonthlyCost, v.Impact.Currency})
	for _, a := range v.Alternatives {
		c.Comparison = append(c.Comparison, Comparison{a.ID, a.Name, a.ExpectedVolume.EventsPerSecond, a.RetentionDays, a.Impact.MonthlyStorageBytes, a.Impact.MonthlyCost, a.Impact.Currency})
	}
	for _, ch := range c.Challenges {
		if ch.Version == c.CurrentVersion {
			for _, e := range ch.Evidence {
				if !e.Accessible {
					c.Findings = append(c.Findings, Finding{"inaccessible_challenge_evidence", "incomplete", "Challenge evidence is not accessible to reviewers.", ch.ID, ch.AuthorID})
				}
			}
		}
	}
	c.Complete = len(c.Findings) == 0
	for _, f := range c.Findings {
		if f.Severity == "blocking" {
			c.Blocked = true
		}
	}
	return c
}
