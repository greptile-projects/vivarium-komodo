// Package capacityobjectives owns immutable shared demand and capacity contracts.
package capacityobjectives

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
	ErrNotFound = errors.New("capacity objective not found")
	ErrInvalid  = errors.New("invalid capacity objective")
	ErrConflict = errors.New("capacity objective version conflict")
)

type Forecast struct {
	ID         string    `json:"id"`
	Segment    string    `json:"segment"`
	Demand     float64   `json:"demand"`
	Unit       string    `json:"unit"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Confidence string    `json:"confidence"`
	Evidence   []string  `json:"evidence"`
}
type TrafficShape struct {
	Name            string  `json:"name"`
	Pattern         string  `json:"pattern"`
	PeakMultiplier  float64 `json:"peak_multiplier"`
	DurationMinutes int     `json:"duration_minutes"`
}
type Seasonality struct {
	Name       string  `json:"name"`
	Schedule   string  `json:"schedule"`
	Multiplier float64 `json:"multiplier"`
}
type Commitment struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Scope    string  `json:"scope"`
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Source   string  `json:"source,omitempty"`
}
type Limit struct {
	Dependency string  `json:"dependency"`
	Metric     string  `json:"metric"`
	Maximum    float64 `json:"maximum"`
	Unit       string  `json:"unit"`
	OwnerID    string  `json:"owner_id,omitempty"`
}
type Signal struct {
	Name     string `json:"name"`
	Source   string `json:"source,omitempty"`
	Required bool   `json:"required"`
	OwnerID  string `json:"owner_id,omitempty"`
}
type Assumption struct {
	ID        string    `json:"id"`
	Statement string    `json:"statement"`
	Evidence  string    `json:"evidence,omitempty"`
	OwnerID   string    `json:"owner_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Label      string `json:"label,omitempty"`
}
type VersionInput struct {
	SubjectKind          string         `json:"subject_kind"`
	SubjectID            string         `json:"subject_id"`
	Title                string         `json:"title"`
	Description          string         `json:"description"`
	Forecasts            []Forecast     `json:"demand_forecasts"`
	TrafficShapes        []TrafficShape `json:"traffic_shapes"`
	Seasonality          []Seasonality  `json:"seasonality"`
	ServiceLevels        []Commitment   `json:"service_levels"`
	BottleneckThresholds []Commitment   `json:"bottleneck_thresholds"`
	DependencyLimits     []Limit        `json:"dependency_limits"`
	Regions              []string       `json:"regions"`
	OwnerIDs             []string       `json:"owner_ids"`
	BudgetAmount         float64        `json:"budget_amount"`
	BudgetCurrency       string         `json:"budget_currency"`
	LeadTimeDays         int            `json:"lead_time_days"`
	Signals              []Signal       `json:"signals"`
	Assumptions          []Assumption   `json:"assumptions"`
	SuccessCriteria      []string       `json:"success_criteria"`
	RollbackCriteria     []string       `json:"rollback_criteria"`
	Links                []Link         `json:"links"`
	ChangeReason         string         `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Gap struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	OwnerID   string `json:"owner_id,omitempty"`
	Reference string `json:"reference,omitempty"`
}
type Objective struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
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
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func textList(xs []string) bool {
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
	if !map[string]bool{"service": true, "api": true, "job": true, "workspace": true, "package_delivery": true, "user_journey": true}[in.SubjectKind] || in.SubjectID == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Description) == "" || len(in.Forecasts) == 0 || len(in.TrafficShapes) == 0 || len(in.ServiceLevels) == 0 || len(in.BottleneckThresholds) == 0 || len(in.DependencyLimits) == 0 || !textList(in.Regions) || !textList(in.OwnerIDs) || in.BudgetAmount <= 0 || in.BudgetCurrency == "" || in.LeadTimeDays < 0 || !textList(in.SuccessCriteria) || !textList(in.RollbackCriteria) || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	seen := map[string]bool{}
	for _, f := range in.Forecasts {
		if f.ID == "" || seen[f.ID] || f.Segment == "" || f.Demand <= 0 || f.Unit == "" || f.StartsAt.IsZero() || !f.EndsAt.After(f.StartsAt) || !map[string]bool{"supported": true, "estimated": true, "unknown": true}[f.Confidence] {
			return false
		}
		seen[f.ID] = true
	}
	for _, x := range in.TrafficShapes {
		if x.Name == "" || x.Pattern == "" || x.PeakMultiplier < 1 || x.DurationMinutes < 1 {
			return false
		}
	}
	for _, x := range append(append([]Commitment{}, in.ServiceLevels...), in.BottleneckThresholds...) {
		if x.ID == "" || x.Kind == "" || x.Scope == "" || !map[string]bool{"at_least": true, "at_most": true}[x.Operator] || x.Value < 0 || x.Unit == "" {
			return false
		}
	}
	for _, x := range in.DependencyLimits {
		if x.Dependency == "" || x.Metric == "" || x.Maximum <= 0 || x.Unit == "" {
			return false
		}
	}
	for _, x := range in.Assumptions {
		if x.ID == "" || x.Statement == "" || x.OwnerID == "" || x.ExpiresAt.IsZero() {
			return false
		}
	}
	for _, x := range in.Links {
		if !map[string]bool{"product_roadmap": true, "experiment": true, "performance_goal": true, "service_objective": true, "infrastructure": true, "release": true, "funding": true}[x.Kind] || x.ResourceID == "" {
			return false
		}
	}
	return true
}
func (s *Store) Create(repo, actor string, in VersionInput) (Objective, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publish(Objective{ID: id(), RepositoryID: repo}, actor, 0, in)
}
func (s *Store) Revise(repo, oid, actor string, expected int64, in VersionInput) (Objective, error) {
	if actor == "" || !valid(in) {
		return Objective{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(repo, oid)
	if e != nil {
		return o, e
	}
	return s.publish(o, actor, expected, in)
}
func (s *Store) publish(o Objective, actor string, expected int64, in VersionInput) (Objective, error) {
	if o.CurrentVersion != expected {
		return o, ErrConflict
	}
	o.CurrentVersion++
	o.Versions = append(o.Versions, Version{Number: o.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	return o, s.write(o)
}
func (s *Store) Get(repo, oid string) (Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, oid)
}
func (s *Store) List(repo string) ([]Objective, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Objective{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var o Objective
		if x == nil {
			x = json.Unmarshal(b, &o)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, o)
	}
	return out, nil
}
func (s *Store) read(repo, oid string) (Objective, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, oid+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Objective{}, ErrNotFound
	}
	var o Objective
	if e == nil {
		e = json.Unmarshal(b, &o)
	}
	return o, e
}
func (s *Store) write(o Objective) error {
	d := filepath.Join(s.root, o.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "objective-*.tmp")
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
		e = os.Rename(n, filepath.Join(d, o.ID+".json"))
	}
	return e
}

func Resolve(o Objective, now time.Time) Objective {
	o.Gaps = nil
	o.NonAuthority = []string{"Capacity objectives grant no spending, provider, repository, release, deployment, environment, credential, or operational authority."}
	if len(o.Versions) == 0 {
		return o
	}
	v := o.Versions[len(o.Versions)-1]
	for _, f := range v.Forecasts {
		if f.Confidence != "supported" || len(f.Evidence) == 0 {
			o.Gaps = append(o.Gaps, Gap{Kind: "unsupported_forecast", Detail: "Forecast " + f.ID + " lacks supporting evidence or is explicitly uncertain.", Reference: f.ID, OwnerID: v.AuthorID})
		}
	}
	for _, s := range v.Signals {
		if s.Required && strings.TrimSpace(s.Source) == "" {
			o.Gaps = append(o.Gaps, Gap{Kind: "missing_signal", Detail: "Required signal " + s.Name + " has no source.", Reference: s.Name, OwnerID: s.OwnerID})
		}
	}
	all := append(append([]Commitment{}, v.ServiceLevels...), v.BottleneckThresholds...)
	for i, a := range all {
		for _, b := range all[i+1:] {
			if a.Kind == b.Kind && a.Scope == b.Scope && a.Unit == b.Unit && a.Operator != b.Operator && ((a.Operator == "at_least" && a.Value > b.Value) || (a.Operator == "at_most" && b.Value > a.Value)) {
				o.Gaps = append(o.Gaps, Gap{Kind: "conflicting_commitment", Detail: "Commitments " + a.ID + " and " + b.ID + " cannot both be met.", Reference: a.ID + ":" + b.ID, OwnerID: v.AuthorID})
			}
		}
	}
	for _, a := range v.Assumptions {
		kind := ""
		if !a.ExpiresAt.After(now) {
			kind = "expired_assumption"
		} else if a.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			kind = "expiring_assumption"
		}
		if kind != "" {
			o.Gaps = append(o.Gaps, Gap{Kind: kind, Detail: "Assumption " + a.ID + " expires at " + a.ExpiresAt.UTC().Format(time.RFC3339) + ".", Reference: a.ID, OwnerID: a.OwnerID})
		}
	}
	return o
}
