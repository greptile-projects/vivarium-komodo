// Package capabilityproofs retains revision-exact evidence that compatibility stages work and old use is understood.
package capabilityproofs

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

	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
)

var ErrNotFound = errors.New("capability proof candidate not found")
var ErrInvalid = errors.New("invalid capability proof candidate")
var ErrForbidden = errors.New("capability proof owner required")

type Retirements interface {
	Get(repository, plan string) (capabilityretirements.Plan, error)
}
type Revisions struct {
	Provider      string `json:"provider"`
	Consumer      string `json:"consumer"`
	Schema        string `json:"schema"`
	Configuration string `json:"configuration"`
	Release       string `json:"release"`
}
type Environment struct {
	Kind        string  `json:"kind"`
	Reference   string  `json:"reference"`
	Networkless bool    `json:"networkless"`
	Synthetic   bool    `json:"synthetic"`
	CostLimit   float64 `json:"cost_limit"`
}
type Check struct {
	ID        string   `json:"id"`
	Mode      string   `json:"mode"`
	Journey   string   `json:"journey"`
	Expected  string   `json:"expected"`
	InputKeys []string `json:"input_keys"`
}
type Input struct {
	PlanID           string      `json:"plan_id"`
	StageID          string      `json:"stage_id"`
	Revisions        Revisions   `json:"revisions"`
	Environment      Environment `json:"environment"`
	Checks           []Check     `json:"checks"`
	ConsumerIDs      []string    `json:"consumer_ids"`
	RequiredOwnerIDs []string    `json:"required_owner_ids"`
}
type Artifact struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
}
type AttemptInput struct {
	CheckID    string     `json:"check_id"`
	Status     string     `json:"status"`
	Summary    string     `json:"summary"`
	Revisions  Revisions  `json:"revisions"`
	Artifacts  []Artifact `json:"artifacts"`
	Cost       float64    `json:"cost"`
	DurationMS int64      `json:"duration_ms"`
}
type Attempt struct {
	ID string `json:"id"`
	AttemptInput
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	Current        bool      `json:"current"`
	Superseded     bool      `json:"superseded"`
	StaleInputKeys []string  `json:"stale_input_keys"`
}
type UsageInput struct {
	ConsumerID        string    `json:"consumer_id"`
	Status            string    `json:"status"`
	EvidenceReference string    `json:"evidence_reference"`
	Revisions         Revisions `json:"revisions"`
	Count             int64     `json:"count"`
	WindowStart       time.Time `json:"window_start"`
	WindowEnd         time.Time `json:"window_end"`
	Inaccessible      bool      `json:"inaccessible"`
}
type Usage struct {
	ID string `json:"id"`
	UsageInput
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	Current        bool      `json:"current"`
	Superseded     bool      `json:"superseded"`
	StaleInputKeys []string  `json:"stale_input_keys"`
}
type Acknowledgement struct {
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	DecidedAt time.Time `json:"decided_at"`
}
type Blocker struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}
type Candidate struct {
	ID               string            `json:"id"`
	RepositoryID     string            `json:"repository_id"`
	Input            Input             `json:"input"`
	CreatedByID      string            `json:"created_by_id"`
	CreatedAt        time.Time         `json:"created_at"`
	Attempts         []Attempt         `json:"attempts"`
	Usage            []Usage           `json:"usage"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	Blockers         []Blocker         `json:"blockers"`
	RemovalReady     bool              `json:"removal_ready"`
	NonAuthority     []string          `json:"non_authority"`
}
type Store struct {
	root  string
	plans Retirements
	mu    sync.Mutex
	now   func() time.Time
}

func New(root string, plans Retirements) (*Store, error) {
	if strings.TrimSpace(root) == "" || plans == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, plans: plans, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func revision(r Revisions, key string) string {
	switch key {
	case "provider":
		return r.Provider
	case "consumer":
		return r.Consumer
	case "schema":
		return r.Schema
	case "configuration":
		return r.Configuration
	case "release":
		return r.Release
	}
	return ""
}
func validInput(in Input) bool {
	if in.PlanID == "" || in.StageID == "" || in.Revisions.Provider == "" || in.Revisions.Consumer == "" || in.Revisions.Schema == "" || in.Revisions.Configuration == "" || in.Revisions.Release == "" || in.Environment.Kind == "" || in.Environment.Reference == "" || (!in.Environment.Networkless && !in.Environment.Synthetic) || in.Environment.CostLimit <= 0 || len(in.Checks) == 0 || len(in.ConsumerIDs) == 0 || len(in.RequiredOwnerIDs) == 0 {
		return false
	}
	modes := map[string]bool{}
	ids := map[string]bool{}
	for _, c := range in.Checks {
		if c.ID == "" || ids[c.ID] || !allowed(c.Mode, "old_only", "dual_support", "replacement", "rollback", "journey") || c.Expected == "" || len(c.InputKeys) == 0 {
			return false
		}
		ids[c.ID] = true
		modes[c.Mode] = true
		for _, k := range c.InputKeys {
			if revision(in.Revisions, k) == "" {
				return false
			}
		}
	}
	for _, m := range []string{"old_only", "dual_support", "replacement", "rollback", "journey"} {
		if !modes[m] {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, c string) string { return filepath.Join(s.root, repo, c+".json") }
func (s *Store) Create(repo, actor string, in Input) (Candidate, error) {
	if repo == "" || actor == "" || !validInput(in) {
		return Candidate{}, ErrInvalid
	}
	p, e := s.plans.Get(repo, in.PlanID)
	if e != nil {
		return Candidate{}, ErrInvalid
	}
	stage := false
	for _, x := range p.Input.Stages {
		stage = stage || x.ID == in.StageID
	}
	if !stage {
		return Candidate{}, ErrInvalid
	}
	now := s.now().UTC()
	c := Candidate{ID: id(), RepositoryID: repo, Input: in, CreatedByID: actor, CreatedAt: now, Attempts: []Attempt{}, Usage: []Usage{}, Acknowledgements: []Acknowledgement{}, NonAuthority: []string{"repository write", "consumer access", "release", "deployment", "credential", "environment", "operational authority"}}
	c = s.derive(c)
	return c, s.save(c)
}
func stale(base, got Revisions, keys []string) []string {
	out := []string{}
	for _, k := range keys {
		if revision(base, k) != revision(got, k) {
			out = append(out, k)
		}
	}
	return out
}
func (s *Store) derive(c Candidate) Candidate {
	latest := map[string]int{}
	for i := range c.Attempts {
		c.Attempts[i].StaleInputKeys = stale(c.Input.Revisions, c.Attempts[i].Revisions, checkKeys(c.Input.Checks, c.Attempts[i].CheckID))
		c.Attempts[i].Current = len(c.Attempts[i].StaleInputKeys) == 0
		c.Attempts[i].Superseded = false
		if c.Attempts[i].Current {
			if old, ok := latest[c.Attempts[i].CheckID]; ok {
				c.Attempts[old].Superseded = true
			}
			latest[c.Attempts[i].CheckID] = i
		}
	}
	latestUsage := map[string]int{}
	for i := range c.Usage {
		c.Usage[i].StaleInputKeys = stale(c.Input.Revisions, c.Usage[i].Revisions, []string{"consumer", "configuration", "release"})
		c.Usage[i].Current = len(c.Usage[i].StaleInputKeys) == 0
		c.Usage[i].Superseded = false
		if c.Usage[i].Current {
			if old, ok := latestUsage[c.Usage[i].ConsumerID]; ok {
				c.Usage[old].Superseded = true
			}
			latestUsage[c.Usage[i].ConsumerID] = i
		}
	}
	b := []Blocker{}
	add := func(k, s, d string) { b = append(b, Blocker{k, s, d}) }
	totalCost := float64(0)
	for _, a := range c.Attempts {
		totalCost += a.Cost
	}
	if totalCost > c.Input.Environment.CostLimit {
		add("cost_exceeded", c.Input.Environment.Reference, "retained attempt cost exceeds the candidate ceiling")
	}
	for _, x := range c.Input.Checks {
		i, ok := latest[x.ID]
		if !ok {
			add("missing_check", x.ID, "no current attempt")
		} else if c.Attempts[i].Status != "passed" {
			add("failed_check", x.ID, c.Attempts[i].Summary)
		}
	}
	for _, consumer := range c.Input.ConsumerIDs {
		i, found := latestUsage[consumer]
		if found {
			u := c.Usage[i]
			if u.Inaccessible || u.Status == "inaccessible" {
				add("inaccessible_usage", consumer, "usage cannot be measured")
			} else if u.Status == "unmeasured" {
				add("unmeasured_usage", consumer, "usage has not been measured")
			} else if u.Count > 0 {
				add("residual_dependent", consumer, "old behavior still has observed use")
			}
		} else {
			add("missing_usage", consumer, "no current usage observation")
		}
	}
	for _, owner := range c.Input.RequiredOwnerIDs {
		found := false
		for _, a := range c.Acknowledgements {
			if a.OwnerID == owner {
				found = true
				if a.Decision != "acknowledged" {
					add("owner_not_ready", owner, a.Rationale)
				}
			}
		}
		if !found {
			add("owner_pending", owner, "owner has not acknowledged current proof")
		}
	}
	c.Blockers = b
	c.RemovalReady = len(b) == 0
	return c
}
func checkKeys(xs []Check, id string) []string {
	for _, x := range xs {
		if x.ID == id {
			return x.InputKeys
		}
	}
	return nil
}
func (s *Store) AddAttempt(repo, cid, actor string, in AttemptInput) (Candidate, error) {
	if actor == "" || !allowed(in.Status, "passed", "failed", "blocked") || in.Summary == "" || in.Cost < 0 || in.DurationMS < 0 {
		return Candidate{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	if checkKeys(c.Input.Checks, in.CheckID) == nil {
		return Candidate{}, ErrInvalid
	}
	for _, a := range in.Artifacts {
		if a.Name == "" || a.Digest == "" || a.MediaType == "" {
			return Candidate{}, ErrInvalid
		}
	}
	c.Attempts = append(c.Attempts, Attempt{ID: id(), AttemptInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	c = s.derive(c)
	return c, s.save(c)
}
func (s *Store) AddUsage(repo, cid, actor string, in UsageInput) (Candidate, error) {
	if actor == "" || in.ConsumerID == "" || !allowed(in.Status, "observed", "zero", "unmeasured", "inaccessible") || in.EvidenceReference == "" || in.Count < 0 || in.WindowStart.IsZero() || !in.WindowEnd.After(in.WindowStart) {
		return Candidate{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	known := false
	for _, x := range c.Input.ConsumerIDs {
		known = known || x == in.ConsumerID
	}
	if !known {
		return Candidate{}, ErrInvalid
	}
	c.Usage = append(c.Usage, Usage{ID: id(), UsageInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	c = s.derive(c)
	return c, s.save(c)
}
func (s *Store) Acknowledge(repo, cid, actor, decision, rationale string) (Candidate, error) {
	if actor == "" || !allowed(decision, "acknowledged", "changes_requested") || rationale == "" {
		return Candidate{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, cid)
	if e != nil {
		return c, e
	}
	required := false
	for _, x := range c.Input.RequiredOwnerIDs {
		required = required || x == actor
	}
	if !required {
		return Candidate{}, ErrForbidden
	}
	now := s.now().UTC()
	replaced := false
	for i := range c.Acknowledgements {
		if c.Acknowledgements[i].OwnerID == actor {
			c.Acknowledgements[i] = Acknowledgement{actor, decision, rationale, now}
			replaced = true
		}
	}
	if !replaced {
		c.Acknowledgements = append(c.Acknowledgements, Acknowledgement{actor, decision, rationale, now})
	}
	c = s.derive(c)
	return c, s.save(c)
}
func (s *Store) save(c Candidate) error {
	if e := os.MkdirAll(filepath.Dir(s.path(c.RepositoryID, c.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(c.RepositoryID, c.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, cid string) (Candidate, error) {
	b, e := os.ReadFile(s.path(repo, cid))
	if errors.Is(e, fs.ErrNotExist) {
		return Candidate{}, ErrNotFound
	}
	var c Candidate
	if e != nil || json.Unmarshal(b, &c) != nil || c.RepositoryID != repo || c.ID != cid {
		return Candidate{}, ErrNotFound
	}
	return s.derive(c), nil
}
func (s *Store) Get(repo, cid string) (Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, cid)
}
func (s *Store) List(repo string) ([]Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Candidate{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Candidate{}
	for _, f := range es {
		if filepath.Ext(f.Name()) == ".json" {
			c, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
			if e != nil {
				return nil, e
			}
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
