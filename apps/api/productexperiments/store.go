// Package productexperiments owns pre-exposure product experiment contracts.
package productexperiments

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
	ErrNotFound = errors.New("product experiment not found")
	ErrInvalid  = errors.New("invalid product experiment")
	ErrConflict = errors.New("product experiment version conflict")
)

type SignalVersion struct {
	Version            int64     `json:"version"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Unit               string    `json:"unit"`
	Event              string    `json:"event"`
	Properties         []string  `json:"properties"`
	PermittedAudiences []string  `json:"permitted_audiences"`
	Instrumented       bool      `json:"instrumented"`
	AuthorID           string    `json:"author_id"`
	ChangeReason       string    `json:"change_reason"`
	CreatedAt          time.Time `json:"created_at"`
}
type Signal struct {
	ID             string          `json:"id"`
	RepositoryID   string          `json:"repository_id"`
	CurrentVersion int64           `json:"current_version"`
	Versions       []SignalVersion `json:"versions"`
}
type Source struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Variant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Control     bool   `json:"control"`
}
type Audience struct {
	Description   string   `json:"description"`
	Eligibility   []string `json:"eligibility"`
	Exclusions    []string `json:"exclusions"`
	Consent       string   `json:"consent"`
	EstimatedSize int      `json:"estimated_size"`
}
type Measure struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	SignalID      string `json:"signal_id"`
	SignalVersion int64  `json:"signal_version"`
	Aggregation   string `json:"aggregation"`
	Threshold     string `json:"threshold"`
}
type PlanInput struct {
	Title           string    `json:"title"`
	Source          Source    `json:"source"`
	Hypothesis      string    `json:"hypothesis"`
	Variants        []Variant `json:"variants"`
	Audience        Audience  `json:"target_audience"`
	Measures        []Measure `json:"measures"`
	MinimumEvidence string    `json:"minimum_evidence"`
	DurationHours   int       `json:"duration_hours"`
	OwnerIDs        []string  `json:"owner_ids"`
	ParticipantIDs  []string  `json:"participant_ids"`
	StopConditions  []string  `json:"stop_conditions"`
	Assumptions     []string  `json:"assumptions"`
	OverlapKeys     []string  `json:"overlap_keys"`
	ChangeReason    string    `json:"change_reason"`
}
type PlanVersion struct {
	Number int64 `json:"number"`
	PlanInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	Body      string    `json:"body"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Approval struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
type AssumptionChange struct {
	ID         string    `json:"id"`
	Version    int64     `json:"version"`
	Assumption string    `json:"assumption"`
	Detail     string    `json:"detail"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Blocker struct {
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	ResourceID string `json:"resource_id,omitempty"`
}
type Experiment struct {
	ID                string             `json:"id"`
	RepositoryID      string             `json:"repository_id"`
	CurrentVersion    int64              `json:"current_version"`
	Versions          []PlanVersion      `json:"versions"`
	Comments          []Comment          `json:"comments"`
	Approvals         []Approval         `json:"approvals"`
	AssumptionChanges []AssumptionChange `json:"assumption_changes"`
	Blockers          []Blocker          `json:"blockers"`
	Ready             bool               `json:"ready"`
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
	return &Store{root: root, now: func() time.Time { return time.Now().UTC() }}, nil
}
func id(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
func (s *Store) path(repo, kind, item string) string {
	return filepath.Join(s.root, repo, kind, item+".json")
}
func write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}
func read[T any](path string) (T, error) {
	var v T
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func validSignal(in SignalVersion) bool {
	return strings.TrimSpace(in.Name) != "" && strings.TrimSpace(in.Description) != "" && strings.TrimSpace(in.Unit) != "" && strings.TrimSpace(in.Event) != "" && len(in.PermittedAudiences) > 0 && strings.TrimSpace(in.ChangeReason) != ""
}
func (s *Store) CreateSignal(repo, actor string, in SignalVersion) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validSignal(in) {
		return Signal{}, ErrInvalid
	}
	in.Version = 1
	in.AuthorID = actor
	in.CreatedAt = s.now()
	v := Signal{ID: id("sig_"), RepositoryID: repo, CurrentVersion: 1, Versions: []SignalVersion{in}}
	return v, write(s.path(repo, "signals", v.ID), v)
}
func (s *Store) ReviseSignal(repo, sid, actor string, expected int64, in SignalVersion) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Signal](s.path(repo, "signals", sid))
	if e != nil {
		return v, e
	}
	if v.CurrentVersion != expected {
		return v, ErrConflict
	}
	if !validSignal(in) {
		return v, ErrInvalid
	}
	in.Version = expected + 1
	in.AuthorID = actor
	in.CreatedAt = s.now()
	v.CurrentVersion++
	v.Versions = append(v.Versions, in)
	return v, write(s.path(repo, "signals", sid), v)
}
func (s *Store) Signals(repo string) ([]Signal, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "signals", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Signal{}
	for _, p := range paths {
		v, x := read[Signal](p)
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func validPlan(in PlanInput) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Hypothesis) == "" || !one(in.Source.Kind, "proposal", "issue", "decision", "pull_request", "preview", "release") || in.Source.ID == "" || len(in.Variants) < 2 || len(in.Measures) == 0 || in.Audience.Description == "" || len(in.Audience.Eligibility) == 0 || in.MinimumEvidence == "" || in.DurationHours < 1 || len(in.OwnerIDs) == 0 || len(in.ParticipantIDs) == 0 || len(in.StopConditions) == 0 || in.ChangeReason == "" {
		return false
	}
	control := 0
	for _, v := range in.Variants {
		if v.ID == "" || v.Name == "" {
			return false
		}
		if v.Control {
			control++
		}
	}
	if control != 1 {
		return false
	}
	for _, m := range in.Measures {
		if m.ID == "" || m.Name == "" || !one(m.Kind, "success", "guardrail") || m.SignalID == "" || m.SignalVersion < 1 || m.Aggregation == "" || m.Threshold == "" {
			return false
		}
	}
	return true
}
func one(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) Create(repo, actor string, in PlanInput) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validPlan(in) {
		return Experiment{}, ErrInvalid
	}
	p := PlanVersion{Number: 1, PlanInput: in, AuthorID: actor, CreatedAt: s.now()}
	v := Experiment{ID: id("exp_"), RepositoryID: repo, CurrentVersion: 1, Versions: []PlanVersion{p}}
	if e := write(s.path(repo, "experiments", v.ID), v); e != nil {
		return v, e
	}
	return s.resolve(repo, v), nil
}
func (s *Store) Revise(repo, eid, actor string, expected int64, in PlanInput) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	if e != nil {
		return v, e
	}
	if v.CurrentVersion != expected {
		return v, ErrConflict
	}
	if !validPlan(in) {
		return v, ErrInvalid
	}
	v.CurrentVersion++
	v.Versions = append(v.Versions, PlanVersion{Number: v.CurrentVersion, PlanInput: in, AuthorID: actor, CreatedAt: s.now()})
	e = write(s.path(repo, "experiments", eid), v)
	return s.resolve(repo, v), e
}
func (s *Store) mutate(repo, eid string, fn func(*Experiment) error) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	e = write(s.path(repo, "experiments", eid), v)
	return s.resolve(repo, v), e
}
func (s *Store) Comment(repo, eid, actor, body string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if strings.TrimSpace(body) == "" || len(body) > 10000 {
			return ErrInvalid
		}
		v.Comments = append(v.Comments, Comment{ID: id("com_"), Version: v.CurrentVersion, Body: body, AuthorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Approve(repo, eid, actor, decision, note string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		p := v.Versions[len(v.Versions)-1]
		found := false
		for _, x := range p.ParticipantIDs {
			if x == actor {
				found = true
			}
		}
		if !found || !one(decision, "approved", "changes_requested") {
			return ErrInvalid
		}
		v.Approvals = append(v.Approvals, Approval{ID: id("apr_"), Version: v.CurrentVersion, ActorID: actor, Decision: decision, Note: note, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) ChangeAssumption(repo, eid, actor, assumption, detail string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if assumption == "" || detail == "" {
			return ErrInvalid
		}
		v.AssumptionChanges = append(v.AssumptionChanges, AssumptionChange{ID: id("chg_"), Version: v.CurrentVersion, Assumption: assumption, Detail: detail, ActorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Get(repo, eid string) (Experiment, error) {
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	return s.resolve(repo, v), e
}
func (s *Store) List(repo string) ([]Experiment, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "experiments", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Experiment{}
	for _, p := range paths {
		v, x := read[Experiment](p)
		if x != nil {
			return nil, x
		}
		out = append(out, s.resolve(repo, v))
	}
	return out, nil
}
func (s *Store) resolve(repo string, v Experiment) Experiment {
	v.Blockers = nil
	if len(v.Versions) == 0 {
		return v
	}
	p := v.Versions[len(v.Versions)-1]
	signals, _ := s.Signals(repo)
	sm := map[string]Signal{}
	for _, x := range signals {
		sm[x.ID] = x
	}
	for _, m := range p.Measures {
		x, ok := sm[m.SignalID]
		if !ok || m.SignalVersion > x.CurrentVersion {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_instrumentation", Detail: m.Name + " references an unavailable signal version", ResourceID: m.SignalID})
			continue
		}
		sv := x.Versions[m.SignalVersion-1]
		if !sv.Instrumented {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_instrumentation", Detail: sv.Name + " is not instrumented", ResourceID: x.ID})
		}
		if !contains(sv.PermittedAudiences, p.Audience.Consent) {
			v.Blockers = append(v.Blockers, Blocker{Kind: "ineligible_audience", Detail: sv.Name + " does not permit audience consent class " + p.Audience.Consent, ResourceID: x.ID})
		}
	}
	all, _ := s.raw(repo)
	for _, x := range all {
		if x.ID == v.ID || len(x.Versions) == 0 {
			continue
		}
		q := x.Versions[len(x.Versions)-1]
		for _, a := range p.OverlapKeys {
			if contains(q.OverlapKeys, a) {
				v.Blockers = append(v.Blockers, Blocker{Kind: "overlapping_experiment", Detail: "audience or surface overlaps " + x.ID + " at " + a, ResourceID: x.ID})
			}
		}
	}
	for _, c := range v.AssumptionChanges {
		if c.Version == v.CurrentVersion {
			v.Blockers = append(v.Blockers, Blocker{Kind: "changed_assumption", Detail: c.Assumption + ": " + c.Detail, ResourceID: c.ID})
		}
	}
	for _, participant := range p.ParticipantIDs {
		decision := ""
		for i := len(v.Approvals) - 1; i >= 0; i-- {
			a := v.Approvals[i]
			if a.Version == v.CurrentVersion && a.ActorID == participant {
				decision = a.Decision
				break
			}
		}
		if decision != "approved" {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_approval", Detail: participant + " has not approved the current plan"})
		}
	}
	v.Ready = len(v.Blockers) == 0
	return v
}
func (s *Store) raw(repo string) ([]Experiment, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "experiments", "*.json"))
	out := []Experiment{}
	for _, p := range paths {
		v, x := read[Experiment](p)
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	return out, e
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
