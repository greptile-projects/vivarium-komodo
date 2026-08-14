// Package serviceobjectives owns versioned, repository-scoped reliability contracts.
package serviceobjectives

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
	ErrNotFound = errors.New("service objective not found")
	ErrInvalid  = errors.New("invalid service objective")
	ErrConflict = errors.New("service objective version conflict")
)

type Scope struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Name       string `json:"name"`
}
type Indicator struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Signal       string `json:"signal"`
	SignalStatus string `json:"signal_status"`
	Calculation  string `json:"calculation"`
	Unit         string `json:"unit"`
	GoodEvent    string `json:"good_event,omitempty"`
	TotalEvent   string `json:"total_event,omitempty"`
}
type Window struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Duration  string `json:"duration"`
	Alignment string `json:"alignment,omitempty"`
}
type Target struct {
	IndicatorID        string  `json:"indicator_id"`
	WindowID           string  `json:"window_id"`
	Comparator         string  `json:"comparator"`
	Value              float64 `json:"value"`
	ErrorBudgetPercent float64 `json:"error_budget_percent"`
}
type Journey struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Behavior string   `json:"behavior"`
	OwnerIDs []string `json:"owner_ids"`
}
type Dependency struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	OwnerIDs []string `json:"owner_ids"`
	Required bool     `json:"required"`
}
type Severity struct {
	Level                 string   `json:"level"`
	BudgetConsumedPercent float64  `json:"budget_consumed_percent"`
	Response              string   `json:"response"`
	OwnerIDs              []string `json:"owner_ids"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
}
type Exception struct {
	ID         string    `json:"id"`
	Reason     string    `json:"reason"`
	ApprovedBy string    `json:"approved_by"`
	OwnerID    string    `json:"owner_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type VersionInput struct {
	Title           string       `json:"title"`
	Description     string       `json:"description"`
	Scopes          []Scope      `json:"scopes"`
	Indicators      []Indicator  `json:"indicators"`
	Windows         []Window     `json:"measurement_windows"`
	Targets         []Target     `json:"targets"`
	Journeys        []Journey    `json:"journeys"`
	Dependencies    []Dependency `json:"dependencies"`
	Severities      []Severity   `json:"severity_thresholds"`
	OwnerIDs        []string     `json:"owner_ids"`
	Links           []Link       `json:"commitment_links"`
	Exceptions      []Exception  `json:"exceptions"`
	ExceptionPolicy string       `json:"exception_policy"`
	ChangeReason    string       `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	IndicatorID  string `json:"indicator_id,omitempty"`
	DependencyID string `json:"dependency_id,omitempty"`
	ExceptionID  string `json:"exception_id,omitempty"`
	Detail       string `json:"detail"`
}
type Objective struct {
	ID             string          `json:"id"`
	RepositoryID   string          `json:"repository_id"`
	CurrentVersion int64           `json:"current_version"`
	Versions       []Version       `json:"versions"`
	Blockers       []Blocker       `json:"blockers"`
	Mappings       []SignalMapping `json:"signal_mappings"`
	Attainment     []Attainment    `json:"attainment_history"`
}
type SourceMapping struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	SanitizedFields []string `json:"sanitized_fields"`
	ResourceID      string   `json:"resource_id,omitempty"`
}
type MappingInput struct {
	ExpectedVersion         int64           `json:"expected_version"`
	ObjectiveVersion        int64           `json:"objective_version"`
	IndicatorID             string          `json:"indicator_id"`
	WindowID                string          `json:"window_id"`
	InstrumentationRevision string          `json:"instrumentation_revision"`
	Sources                 []SourceMapping `json:"sources"`
	ChangeReason            string          `json:"change_reason"`
}
type MappingVersion struct {
	Number                  int64           `json:"number"`
	ObjectiveVersion        int64           `json:"objective_version"`
	IndicatorID             string          `json:"indicator_id"`
	WindowID                string          `json:"window_id"`
	InstrumentationRevision string          `json:"instrumentation_revision"`
	Sources                 []SourceMapping `json:"sources"`
	ChangeReason            string          `json:"change_reason"`
	AuthorID                string          `json:"author_id"`
	CreatedAt               time.Time       `json:"created_at"`
}
type SignalMapping struct {
	ID             string           `json:"id"`
	CurrentVersion int64            `json:"current_version"`
	Versions       []MappingVersion `json:"versions"`
}
type EvidenceRef struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	Label      string `json:"label"`
}
type ObservationInput struct {
	MappingID                  string        `json:"mapping_id"`
	MappingVersion             int64         `json:"mapping_version"`
	WindowStart                time.Time     `json:"window_start"`
	WindowEnd                  time.Time     `json:"window_end"`
	Value                      float64       `json:"value"`
	ErrorBudgetConsumedPercent float64       `json:"error_budget_consumed_percent"`
	Uncertainty                string        `json:"uncertainty"`
	ComparableToPrevious       bool          `json:"comparable_to_previous"`
	Sanitized                  bool          `json:"sanitized"`
	ContainsRestrictedData     bool          `json:"contains_restricted_data"`
	Audience                   string        `json:"audience"`
	Evidence                   []EvidenceRef `json:"evidence"`
}
type Attainment struct {
	ID string `json:"id"`
	ObservationInput
	ObjectiveVersion        int64     `json:"objective_version"`
	IndicatorID             string    `json:"indicator_id"`
	WindowID                string    `json:"window_id"`
	InstrumentationRevision string    `json:"instrumentation_revision"`
	Status                  string    `json:"status"`
	GapKinds                []string  `json:"gap_kinds"`
	AuthorID                string    `json:"author_id"`
	CreatedAt               time.Time `json:"created_at"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

// Project removes repository-reader observations from anonymous public views.
func Project(x Objective, authenticated bool) Objective {
	if authenticated {
		return x
	}
	out := x.Attainment[:0]
	for _, a := range x.Attainment {
		if a.Audience == "public" {
			out = append(out, a)
		}
	}
	x.Attainment = out
	if len(out) == 0 {
		x.Blockers = append(x.Blockers, Blocker{Kind: "missing_visible_attainment", Detail: "no attainment evidence is visible to this reader"})
	}
	return x
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
func newid() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func listOK(v []string, required bool) bool {
	if required && len(v) == 0 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return len(v) <= 100
}
func valid(in VersionInput) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || strings.TrimSpace(in.ExceptionPolicy) == "" || len(in.Scopes) == 0 || len(in.Indicators) == 0 || len(in.Windows) == 0 || len(in.Targets) == 0 || len(in.Journeys) == 0 || !listOK(in.OwnerIDs, false) {
		return false
	}
	ids := map[string]bool{}
	windows := map[string]bool{}
	journeys := map[string]bool{}
	deps := map[string]bool{}
	exceptions := map[string]bool{}
	for _, x := range in.Scopes {
		if !map[string]bool{"repository": true, "release": true, "environment": true}[x.Kind] || x.Name == "" || (x.Kind != "repository" && x.ResourceID == "") {
			return false
		}
	}
	for _, x := range in.Indicators {
		if x.ID == "" || x.Name == "" || x.Description == "" || x.Signal == "" || ids[x.ID] || !map[string]bool{"available": true, "missing": true}[x.SignalStatus] || x.Calculation == "" || x.Unit == "" {
			return false
		}
		ids[x.ID] = true
	}
	for _, x := range in.Windows {
		if x.ID == "" || windows[x.ID] || !map[string]bool{"rolling": true, "calendar": true}[x.Kind] || x.Duration == "" {
			return false
		}
		windows[x.ID] = true
	}
	for _, x := range in.Targets {
		if !ids[x.IndicatorID] || !windows[x.WindowID] || !map[string]bool{"gte": true, "lte": true}[x.Comparator] || x.ErrorBudgetPercent < 0 || x.ErrorBudgetPercent > 100 {
			return false
		}
	}
	for _, x := range in.Journeys {
		if x.ID == "" || x.Name == "" || x.Behavior == "" || journeys[x.ID] || !listOK(x.OwnerIDs, false) {
			return false
		}
		journeys[x.ID] = true
	}
	for _, x := range in.Dependencies {
		if x.ID == "" || x.Name == "" || x.Kind == "" || deps[x.ID] || !listOK(x.OwnerIDs, false) {
			return false
		}
		deps[x.ID] = true
	}
	for _, x := range in.Severities {
		if !map[string]bool{"warning": true, "critical": true, "exhausted": true}[x.Level] || x.BudgetConsumedPercent < 0 || x.BudgetConsumedPercent > 100 || x.Response == "" || !listOK(x.OwnerIDs, false) {
			return false
		}
	}
	for _, x := range in.Links {
		if !map[string]bool{"product": true, "performance": true, "accessibility": true, "privacy": true, "release": true}[x.Kind] || x.ResourceID == "" || x.Label == "" || !map[string]bool{"linked": true, "missing": true}[x.Status] {
			return false
		}
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || exceptions[x.ID] || x.Reason == "" || x.ApprovedBy == "" || x.OwnerID == "" || x.ExpiresAt.IsZero() {
			return false
		}
		exceptions[x.ID] = true
	}
	return true
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Objective) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), b, 0640)
	}
	return e
}
func (s *Store) raw(repo, id string) (Objective, error) {
	var x Objective
	b, e := os.ReadFile(s.path(repo, id))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Create(repo, actor string, in VersionInput) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	x := Objective{ID: newid(), RepositoryID: repo, CurrentVersion: 1, Versions: []Version{{Number: 1, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()}}}
	derive(&x, nil, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in VersionInput) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if actor == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	x, e := s.raw(repo, id)
	if e != nil {
		return x, e
	}
	if x.CurrentVersion != expected {
		return x, ErrConflict
	}
	x.CurrentVersion++
	x.Versions = append(x.Versions, Version{Number: x.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	derive(&x, nil, s.now().UTC())
	return x, s.save(x)
}

var sourceKinds = map[string]bool{"metric": true, "log": true, "trace": true, "health_check": true, "support_report": true, "deployment": true, "release": true, "commit": true, "pull_request": true, "package": true, "dependent_service": true}

func mappingValid(x Objective, in MappingInput) bool {
	if in.ObjectiveVersion < 1 || in.ObjectiveVersion > x.CurrentVersion || in.IndicatorID == "" || in.WindowID == "" || in.InstrumentationRevision == "" || in.ChangeReason == "" || len(in.Sources) == 0 || len(in.Sources) > 100 {
		return false
	}
	v := x.Versions[in.ObjectiveVersion-1]
	indicator, window := false, false
	for _, i := range v.Indicators {
		indicator = indicator || i.ID == in.IndicatorID
	}
	for _, w := range v.Windows {
		window = window || w.ID == in.WindowID
	}
	seen := map[string]bool{}
	for _, q := range in.Sources {
		if q.ID == "" || q.Name == "" || !sourceKinds[q.Kind] || seen[q.ID] || len(q.SanitizedFields) == 0 || !listOK(q.SanitizedFields, true) {
			return false
		}
		seen[q.ID] = true
	}
	return indicator && window
}
func (s *Store) PutMapping(repo, id, mapping, actor string, in MappingInput) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.raw(repo, id)
	if e != nil {
		return x, e
	}
	if actor == "" || !mappingValid(x, in) {
		return x, ErrInvalid
	}
	var m *SignalMapping
	if mapping == "" {
		x.Mappings = append(x.Mappings, SignalMapping{ID: newid()})
		m = &x.Mappings[len(x.Mappings)-1]
	} else {
		for i := range x.Mappings {
			if x.Mappings[i].ID == mapping {
				m = &x.Mappings[i]
				break
			}
		}
		if m == nil {
			return x, ErrNotFound
		}
	}
	if m.CurrentVersion != in.ExpectedVersion {
		return x, ErrConflict
	}
	m.CurrentVersion++
	m.Versions = append(m.Versions, MappingVersion{Number: m.CurrentVersion, ObjectiveVersion: in.ObjectiveVersion, IndicatorID: in.IndicatorID, WindowID: in.WindowID, InstrumentationRevision: in.InstrumentationRevision, Sources: in.Sources, ChangeReason: in.ChangeReason, AuthorID: actor, CreatedAt: s.now().UTC()})
	derive(&x, nil, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Observe(repo, id, actor string, in ObservationInput) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.raw(repo, id)
	if e != nil {
		return x, e
	}
	var mv *MappingVersion
	for i := range x.Mappings {
		m := &x.Mappings[i]
		if m.ID == in.MappingID && in.MappingVersion > 0 && in.MappingVersion <= int64(len(m.Versions)) {
			mv = &m.Versions[in.MappingVersion-1]
		}
	}
	if actor == "" || mv == nil || !in.Sanitized || in.ContainsRestrictedData || !map[string]bool{"public": true, "repository": true}[in.Audience] || in.WindowStart.IsZero() || !in.WindowEnd.After(in.WindowStart) || in.ErrorBudgetConsumedPercent < 0 || len(in.Evidence) == 0 || len(in.Evidence) > 100 {
		return x, ErrInvalid
	}
	for _, q := range in.Evidence {
		if !sourceKinds[q.Kind] || q.ResourceID == "" || q.Revision == "" || q.Label == "" {
			return x, ErrInvalid
		}
	}
	target := Target{}
	ov := x.Versions[mv.ObjectiveVersion-1]
	for _, t := range ov.Targets {
		if t.IndicatorID == mv.IndicatorID && t.WindowID == mv.WindowID {
			target = t
		}
	}
	status := "met"
	if target.Comparator == "gte" && in.Value < target.Value || target.Comparator == "lte" && in.Value > target.Value {
		status = "missed"
	}
	gaps := []string{}
	if !in.ComparableToPrevious && len(x.Attainment) > 0 {
		gaps = append(gaps, "incomparable_window")
	}
	x.Attainment = append(x.Attainment, Attainment{ID: newid(), ObservationInput: in, ObjectiveVersion: mv.ObjectiveVersion, IndicatorID: mv.IndicatorID, WindowID: mv.WindowID, InstrumentationRevision: mv.InstrumentationRevision, Status: status, GapKinds: gaps, AuthorID: actor, CreatedAt: s.now().UTC()})
	derive(&x, nil, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Get(repo, id string) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.raw(repo, id)
	if e == nil {
		all, _ := s.listRaw(repo)
		derive(&x, all, s.now().UTC())
	}
	return x, e
}
func (s *Store) listRaw(repo string) ([]Objective, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Objective{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Objective{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.raw(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, nil
}
func (s *Store) List(repo string) ([]Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out, e := s.listRaw(repo)
	if e != nil {
		return nil, e
	}
	now := s.now().UTC()
	for i := range out {
		derive(&out[i], out, now)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Versions[0].CreatedAt.After(out[j].Versions[0].CreatedAt) })
	return out, nil
}
func derive(x *Objective, all []Objective, now time.Time) {
	x.Blockers = nil
	v := x.Versions[len(x.Versions)-1]
	for _, i := range v.Indicators {
		found := false
		for _, m := range x.Mappings {
			if len(m.Versions) > 0 {
				q := m.Versions[len(m.Versions)-1]
				found = found || q.ObjectiveVersion == v.Number && q.IndicatorID == i.ID
			}
		}
		if !found {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_signal_mapping", IndicatorID: i.ID, Detail: "current objective indicator has no revision-exact signal mapping"})
		}
	}
	if len(x.Attainment) == 0 {
		x.Blockers = append(x.Blockers, Blocker{Kind: "missing_attainment", Detail: "objective has no sanitized attainment observation"})
	}
	for i := 1; i < len(x.Attainment); i++ {
		a, b := x.Attainment[i-1], x.Attainment[i]
		if a.MappingID == b.MappingID && a.InstrumentationRevision != b.InstrumentationRevision {
			x.Blockers = append(x.Blockers, Blocker{Kind: "changed_instrumentation", IndicatorID: b.IndicatorID, Detail: "instrumentation changed between retained observations"})
		}
		if !b.ComparableToPrevious {
			x.Blockers = append(x.Blockers, Blocker{Kind: "incomparable_window", IndicatorID: b.IndicatorID, Detail: "observation is not comparable with its predecessor"})
		}
	}
	if len(v.OwnerIDs) == 0 {
		x.Blockers = append(x.Blockers, Blocker{Kind: "missing_ownership", Detail: "objective has no accountable owner"})
	}
	allowed := map[string]bool{"ratio": true, "rate": true, "count": true, "latency": true, "availability": true}
	for _, i := range v.Indicators {
		if i.SignalStatus == "missing" {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_signal", IndicatorID: i.ID, Detail: "indicator signal is not available"})
		}
		if !allowed[i.Calculation] {
			x.Blockers = append(x.Blockers, Blocker{Kind: "unsupported_calculation", IndicatorID: i.ID, Detail: i.Calculation + " is not supported"})
		}
	}
	for _, j := range v.Journeys {
		if len(j.OwnerIDs) == 0 {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_ownership", Detail: "journey " + j.ID + " has no owner"})
		}
	}
	for _, d := range v.Dependencies {
		if d.Required && len(d.OwnerIDs) == 0 {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_dependency_owner", DependencyID: d.ID, Detail: "required dependency has no accountable owner"})
		}
	}
	for _, l := range v.Links {
		if l.Status == "missing" {
			x.Blockers = append(x.Blockers, Blocker{Kind: "missing_commitment", Detail: l.Kind + " commitment is unresolved"})
		}
	}
	for _, e := range v.Exceptions {
		if !e.ExpiresAt.After(now) {
			x.Blockers = append(x.Blockers, Blocker{Kind: "expired_exception", ExceptionID: e.ID, Detail: "exception has expired"})
		} else if e.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			x.Blockers = append(x.Blockers, Blocker{Kind: "expiring_exception", ExceptionID: e.ID, Detail: "exception expires within 30 days"})
		}
	}
	for _, other := range all {
		if other.ID == x.ID || len(other.Versions) == 0 {
			continue
		}
		ov := other.Versions[len(other.Versions)-1]
		for _, s := range v.Scopes {
			for _, os := range ov.Scopes {
				if s.Kind != os.Kind || s.ResourceID != os.ResourceID {
					continue
				}
				for _, t := range v.Targets {
					for _, ot := range ov.Targets {
						if t.IndicatorID == ot.IndicatorID && t.WindowID == ot.WindowID && (t.Comparator != ot.Comparator || t.Value != ot.Value) {
							x.Blockers = append(x.Blockers, Blocker{Kind: "conflicting_target", IndicatorID: t.IndicatorID, Detail: "overlapping objective " + other.ID + " declares a different target"})
						}
					}
				}
			}
		}
	}
}
