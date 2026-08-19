// Package qualityplans owns immutable repository quality intent and its evidence links.
package qualityplans

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

var ErrNotFound = errors.New("quality plan not found")
var ErrInvalid = errors.New("invalid quality plan")
var ErrConflict = errors.New("quality plan version conflict")

type Scope struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
}
type Risk struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Mitigation  string `json:"mitigation,omitempty"`
}
type Requirement struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Rationale string `json:"rationale"`
}
type Behavior struct {
	ID               string   `json:"id"`
	Subject          string   `json:"subject"`
	Description      string   `json:"description"`
	Expected         string   `json:"expected"`
	RequirementIDs   []string `json:"requirement_ids"`
	RiskIDs          []string `json:"risk_ids"`
	TestLevels       []string `json:"test_levels"`
	EnvironmentIDs   []string `json:"environment_ids"`
	OwnerIDs         []string `json:"owner_ids"`
	JudgeIDs         []string `json:"judge_ids"`
	Testable         bool     `json:"testable"`
	UntestableReason string   `json:"untestable_reason,omitempty"`
}
type Environment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
}
type RepresentativeData struct {
	ID                    string `json:"id"`
	Description           string `json:"description"`
	Source                string `json:"source"`
	PrivacyClassification string `json:"privacy_classification"`
	Synthetic             bool   `json:"synthetic"`
}
type CoverageGoal struct {
	Subject   string  `json:"subject"`
	Metric    string  `json:"metric"`
	Target    float64 `json:"target"`
	TestLevel string  `json:"test_level"`
}
type Schedule struct {
	Cadence      string     `json:"cadence"`
	NextReviewAt *time.Time `json:"next_review_at,omitempty"`
	OwnerIDs     []string   `json:"owner_ids"`
}
type Threshold struct {
	ID       string  `json:"id"`
	Subject  string  `json:"subject"`
	Metric   string  `json:"metric"`
	Operator string  `json:"operator"`
	Value    float64 `json:"value"`
	Required bool    `json:"required"`
}
type Evidence struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Reference   string     `json:"reference"`
	Revision    string     `json:"revision,omitempty"`
	BehaviorIDs []string   `json:"behavior_ids"`
	Status      string     `json:"status"`
	Manual      bool       `json:"manual"`
	ObservedAt  time.Time  `json:"observed_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	AuthorID    string     `json:"author_id"`
}
type Exception struct {
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	Rationale string    `json:"rationale"`
	OwnerID   string    `json:"owner_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Input struct {
	Name               string               `json:"name"`
	Description        string               `json:"description"`
	Scopes             []Scope              `json:"scopes"`
	Risks              []Risk               `json:"risks"`
	Requirements       []Requirement        `json:"requirements"`
	Behaviors          []Behavior           `json:"expected_behaviors"`
	Environments       []Environment        `json:"supported_environments"`
	RepresentativeData []RepresentativeData `json:"representative_data"`
	CoverageGoals      []CoverageGoal       `json:"coverage_goals"`
	OwnerIDs           []string             `json:"owner_ids"`
	Schedules          []Schedule           `json:"schedules"`
	ReleaseThresholds  []Threshold          `json:"release_thresholds"`
	Evidence           []Evidence           `json:"evidence"`
	Exceptions         []Exception          `json:"exceptions"`
	ChangeReason       string               `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	Input
	AuthorID    string    `json:"author_id"`
	PublishedAt time.Time `json:"published_at"`
}
type Gap struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to,omitempty"`
}
type Plan struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	CurrentVersion int64     `json:"current_version"`
	Versions       []Version `json:"versions"`
	Gaps           []Gap     `json:"gaps"`
}
type Catalog struct {
	Items []Plan `json:"items"`
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
func identifier() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func unique(v string, seen map[string]bool) bool {
	v = strings.TrimSpace(v)
	if v == "" || seen[v] {
		return false
	}
	seen[v] = true
	return true
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || len(in.Scopes) == 0 || len(in.Behaviors) == 0 {
		return false
	}
	for _, x := range in.Scopes {
		if !allowed(x.Kind, "repository", "release", "journey", "interface", "environment") || x.Reference == "" {
			return false
		}
	}
	riskIDs, reqIDs, behaviorIDs, envIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, x := range in.Risks {
		if !unique(x.ID, riskIDs) || x.Description == "" || !allowed(x.Severity, "low", "medium", "high", "critical") {
			return false
		}
	}
	for _, x := range in.Requirements {
		if !unique(x.ID, reqIDs) || !allowed(x.Kind, "issue", "decision", "design", "accessibility", "privacy", "performance", "reliability") || x.Reference == "" || x.Rationale == "" {
			return false
		}
	}
	for _, x := range in.Environments {
		if !unique(x.ID, envIDs) || x.Name == "" || x.Description == "" {
			return false
		}
	}
	for _, x := range in.Behaviors {
		if !unique(x.ID, behaviorIDs) || x.Subject == "" || x.Description == "" || x.Expected == "" || len(x.TestLevels) == 0 || (x.Testable && x.UntestableReason != "") || (!x.Testable && x.UntestableReason == "") {
			return false
		}
		for _, l := range x.TestLevels {
			if !allowed(l, "unit", "component", "integration", "contract", "journey", "exploratory", "acceptance", "production") {
				return false
			}
		}
		for _, id := range x.RequirementIDs {
			if !reqIDs[id] {
				return false
			}
		}
		for _, id := range x.RiskIDs {
			if !riskIDs[id] {
				return false
			}
		}
		for _, id := range x.EnvironmentIDs {
			if !envIDs[id] {
				return false
			}
		}
	}
	for _, x := range in.RepresentativeData {
		if x.ID == "" || x.Description == "" || x.Source == "" || !allowed(x.PrivacyClassification, "public", "internal", "confidential", "restricted") {
			return false
		}
	}
	for _, x := range in.CoverageGoals {
		if x.Subject == "" || x.Metric == "" || x.Target < 0 {
			return false
		}
	}
	for _, x := range in.ReleaseThresholds {
		if x.ID == "" || x.Subject == "" || x.Metric == "" || !allowed(x.Operator, "gte", "lte", "eq") {
			return false
		}
	}
	for _, x := range in.Evidence {
		if x.ID == "" || !allowed(x.Kind, "check", "test_run", "review", "assessment", "observation", "artifact") || x.Reference == "" || !allowed(x.Status, "passing", "failing", "partial", "unknown") || x.AuthorID == "" {
			return false
		}
		for _, id := range x.BehaviorIDs {
			if !behaviorIDs[id] {
				return false
			}
		}
	}
	for _, x := range in.Exceptions {
		if x.ID == "" || x.Subject == "" || x.Rationale == "" || x.OwnerID == "" || x.ExpiresAt.IsZero() {
			return false
		}
	}
	return true
}
func derive(v Version, now time.Time) []Gap {
	out := []Gap{}
	add := func(k, s, d, a string) { out = append(out, Gap{k, s, d, a}) }
	if len(v.OwnerIDs) == 0 {
		add("missing_owner", v.Name, "plan has no accountable owner", v.AuthorID)
	}
	claims := map[string]Behavior{}
	for _, b := range v.Behaviors {
		if len(b.OwnerIDs) == 0 {
			add("missing_owner", b.ID, "behavior has no accountable owner", v.AuthorID)
		}
		if len(b.JudgeIDs) == 0 {
			add("missing_judge", b.ID, "no one is named to judge this behavior", v.AuthorID)
		}
		if !b.Testable {
			add("untestable_claim", b.ID, b.UntestableReason, v.AuthorID)
		}
		key := strings.ToLower(strings.TrimSpace(b.Subject))
		if prior, ok := claims[key]; ok && !strings.EqualFold(strings.TrimSpace(prior.Expected), strings.TrimSpace(b.Expected)) {
			add("contradictory_expectation", b.ID, "expects "+b.Expected+" but "+prior.ID+" expects "+prior.Expected, v.AuthorID)
		} else {
			claims[key] = b
		}
	}
	covered := map[string]bool{}
	for _, e := range v.Evidence {
		for _, id := range e.BehaviorIDs {
			covered[id] = true
		}
		if e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
			add("expired_evidence", e.ID, "linked evidence has expired", e.AuthorID)
		}
	}
	for _, b := range v.Behaviors {
		if !covered[b.ID] {
			add("missing_evidence", b.ID, "no automated or manual evidence is linked", v.AuthorID)
		}
	}
	for _, x := range v.Exceptions {
		if !x.ExpiresAt.After(now) {
			add("expired_exception", x.ID, x.Rationale, x.OwnerID)
		} else if x.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			add("expiring_exception", x.ID, x.ExpiresAt.UTC().Format(time.RFC3339), x.OwnerID)
		}
	}
	return out
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Plan) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Plan, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, e := s.list(repo)
	if e != nil {
		return Plan{}, e
	}
	for _, x := range items {
		if strings.EqualFold(x.Versions[len(x.Versions)-1].Name, in.Name) {
			return Plan{}, ErrConflict
		}
	}
	v := Version{1, in, actor, s.now().UTC()}
	x := Plan{identifier(), repo, 1, []Version{v}, derive(v, s.now().UTC())}
	return x, s.save(x)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (Plan, error) {
	if actor == "" || !valid(in) {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return Plan{}, e
	}
	if x.CurrentVersion != expected {
		return Plan{}, ErrConflict
	}
	v := Version{expected + 1, in, actor, s.now().UTC()}
	x.CurrentVersion = v.Number
	x.Versions = append(x.Versions, v)
	x.Gaps = derive(v, s.now().UTC())
	return x, s.save(x)
}
func (s *Store) Get(repo, id string) (Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.list(repo)
	return Catalog{x}, e
}
func (s *Store) read(repo, id string) (Plan, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Plan{}, ErrNotFound
	}
	var x Plan
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Plan{}, ErrNotFound
	}
	if len(x.Versions) > 0 {
		x.Gaps = derive(x.Versions[len(x.Versions)-1], s.now().UTC())
	}
	return x, nil
}
func (s *Store) list(repo string) ([]Plan, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Plan{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Plan{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Versions[len(out[i].Versions)-1].PublishedAt.After(out[j].Versions[len(out[j].Versions)-1].PublishedAt)
	})
	return out, nil
}
