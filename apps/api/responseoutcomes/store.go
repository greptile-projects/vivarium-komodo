// Package responseoutcomes retains consent-aware response learning without rewriting alerts or authority.
package responseoutcomes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
)

var ErrNotFound = errors.New("response outcome not found")
var ErrInvalid = errors.New("invalid response outcome")
var ErrConflict = errors.New("response outcome changed")

type Metrics struct {
	AcknowledgementSeconds int64   `json:"acknowledgement_seconds"`
	ResolutionSeconds      int64   `json:"resolution_seconds,omitempty"`
	Handoffs               int     `json:"handoffs"`
	Escalations            int     `json:"escalations"`
	MissedTargets          int     `json:"missed_targets"`
	AlertVolume            int     `json:"alert_volume"`
	Deduplicated           int     `json:"deduplicated"`
	FalsePositive          bool    `json:"false_positive"`
	Interruptions          int     `json:"interruptions"`
	ResponderMinutes       int     `json:"responder_minutes"`
	AgentCost              float64 `json:"agent_cost"`
	IncidentCount          int     `json:"incident_count"`
	AffectedUsers          int     `json:"affected_users"`
	RecoveredUsers         int     `json:"recovered_users,omitempty"`
}
type Input struct {
	AlertID               string   `json:"alert_id"`
	ExpectedAlertRevision int64    `json:"expected_alert_revision"`
	Summary               string   `json:"summary"`
	Resolution            string   `json:"resolution"`
	Audience              string   `json:"audience"`
	UserOutcomeConsent    bool     `json:"user_outcome_consent"`
	UserOutcome           string   `json:"user_outcome,omitempty"`
	EvidenceReferences    []string `json:"evidence_references"`
	ResponderMinutes      int      `json:"responder_minutes"`
	AgentCost             float64  `json:"agent_cost"`
	RecoveredUsers        int      `json:"recovered_users"`
	FalsePositive         bool     `json:"false_positive"`
	Interruptions         int      `json:"interruptions"`
	UnsafeAutomation      bool     `json:"unsafe_automation"`
	Owners                []string `json:"owners"`
}
type Review struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}
type Correction struct {
	ID                      string    `json:"id"`
	Kind                    string    `json:"kind"`
	Summary                 string    `json:"summary"`
	ResourceID              string    `json:"resource_id"`
	MaterialAuthorityChange bool      `json:"material_authority_change"`
	Status                  string    `json:"status"`
	ProposedByID            string    `json:"proposed_by_id"`
	ApprovedByID            string    `json:"approved_by_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
}
type Work struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	OwnerKind   string    `json:"owner_kind"`
	OwnerID     string    `json:"owner_id"`
	ResourceID  string    `json:"resource_id"`
	Summary     string    `json:"summary"`
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Control struct {
	Kind        string    `json:"kind"`
	Scope       string    `json:"scope"`
	Reason      string    `json:"reason"`
	State       string    `json:"state"`
	ActivatedAt time.Time `json:"activated_at"`
}
type Outcome struct {
	ID                 string       `json:"id"`
	RepositoryID       string       `json:"repository_id"`
	Revision           int64        `json:"revision"`
	AlertID            string       `json:"alert_id"`
	AlertRevision      int64        `json:"alert_revision"`
	PolicyID           string       `json:"policy_id"`
	PolicyVersion      int64        `json:"policy_version"`
	RotationID         string       `json:"rotation_id,omitempty"`
	TeamID             string       `json:"team_id,omitempty"`
	Summary            string       `json:"summary"`
	Resolution         string       `json:"resolution"`
	Audience           string       `json:"audience"`
	UserOutcomeConsent bool         `json:"user_outcome_consent"`
	UserOutcome        string       `json:"user_outcome,omitempty"`
	EvidenceReferences []string     `json:"evidence_references"`
	Metrics            Metrics      `json:"metrics"`
	Owners             []string     `json:"owners"`
	Reviews            []Review     `json:"reviews"`
	Corrections        []Correction `json:"corrections"`
	Work               []Work       `json:"work"`
	Controls           []Control    `json:"controls"`
	CreatedByID        string       `json:"created_by_id"`
	CreatedAt          time.Time    `json:"created_at"`
	NonAuthority       []string     `json:"non_authority"`
}
type Summary struct {
	Outcomes         int     `json:"outcomes"`
	Metrics          Metrics `json:"metrics"`
	PausedRouting    int     `json:"paused_routing"`
	ActivatedBackups int     `json:"activated_backups"`
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
func id() string                            { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, x string) string { return filepath.Join(s.root, repo, x+".json") }
func (s *Store) save(x Outcome) error {
	p := s.path(x.RepositoryID, x.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(p, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, x string) (Outcome, error) {
	b, e := os.ReadFile(s.path(repo, x))
	if errors.Is(e, fs.ErrNotExist) {
		return Outcome{}, ErrNotFound
	}
	var o Outcome
	if e == nil {
		e = json.Unmarshal(b, &o)
	}
	return o, e
}
func (s *Store) list(repo string) ([]Outcome, error) {
	ps, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	sort.Strings(ps)
	out := []Outcome{}
	for _, p := range ps {
		b, x := os.ReadFile(p)
		var o Outcome
		if x == nil {
			x = json.Unmarshal(b, &o)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, o)
	}
	return out, e
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func metrics(a responsealerts.Alert, in Input) Metrics {
	m := Metrics{AlertVolume: 1, Deduplicated: a.DuplicateCount, FalsePositive: in.FalsePositive, Interruptions: in.Interruptions, ResponderMinutes: in.ResponderMinutes, AgentCost: in.AgentCost, AffectedUsers: a.Signal.AffectedUserCount, RecoveredUsers: in.RecoveredUsers}
	var ack, res *time.Time
	for _, e := range a.Events {
		switch e.Kind {
		case "acknowledged":
			t := e.CreatedAt
			ack = &t
		case "reassign":
			m.Handoffs++
		case "escalate":
			m.Escalations++
		case "resolve":
			t := e.CreatedAt
			res = &t
		}
	}
	if ack != nil {
		m.AcknowledgementSeconds = int64(ack.Sub(a.Signal.ObservedAt).Seconds())
		if a.ResponseDeadline != nil && ack.After(*a.ResponseDeadline) {
			m.MissedTargets++
		}
	} else if a.ResponseDeadline != nil && time.Now().UTC().After(*a.ResponseDeadline) {
		m.MissedTargets++
	}
	if res != nil {
		m.ResolutionSeconds = int64(res.Sub(a.Signal.ObservedAt).Seconds())
	}
	if a.Workspace != nil && a.Workspace.IncidentID != "" {
		m.IncidentCount = 1
	}
	return m
}
func (s *Store) Create(repo, actor string, in Input, a responsealerts.Alert) (Outcome, error) {
	if actor == "" || in.AlertID == "" || in.AlertID != a.ID || in.ExpectedAlertRevision != a.Revision || in.Summary == "" || in.Resolution == "" || !map[string]bool{"owners": true, "repository": true, "public": true}[in.Audience] || len(in.Owners) == 0 || in.ResponderMinutes < 0 || in.AgentCost < 0 || in.Interruptions < 0 || in.RecoveredUsers < 0 {
		return Outcome{}, ErrInvalid
	}
	if in.UserOutcome != "" && !in.UserOutcomeConsent {
		return Outcome{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Outcome{}, e
	}
	for _, x := range xs {
		if x.AlertID == a.ID {
			return Outcome{}, ErrConflict
		}
	}
	now := s.now().UTC()
	o := Outcome{ID: id(), RepositoryID: repo, Revision: 1, AlertID: a.ID, AlertRevision: a.Revision, PolicyID: a.PolicyID, PolicyVersion: a.PolicyVersion, RotationID: a.RotationID, TeamID: a.TeamID, Summary: in.Summary, Resolution: in.Resolution, Audience: in.Audience, UserOutcomeConsent: in.UserOutcomeConsent, UserOutcome: in.UserOutcome, EvidenceReferences: in.EvidenceReferences, Metrics: metrics(a, in), Owners: in.Owners, Reviews: []Review{}, Corrections: []Correction{}, Work: []Work{}, Controls: []Control{}, CreatedByID: actor, CreatedAt: now, NonAuthority: []string{"Outcome learning grants no repository, policy, routing, staffing, communication, incident, deployment, environment, agent, credential, or operational authority."}}
	repeats := 0
	for _, x := range xs {
		if x.TeamID == o.TeamID && (x.Metrics.Interruptions > 0 || x.Metrics.MissedTargets > 0) {
			repeats++
		}
	}
	if in.UnsafeAutomation || repeats >= 2 {
		o.Controls = append(o.Controls, Control{"routing_paused", o.TeamID, "repeated paging, missed coverage, or unsafe automation requires owner review", "active", now})
	}
	if a.Status == "delivery_failed" || a.Status == "unroutable" || repeats >= 2 {
		o.Controls = append(o.Controls, Control{"backup_activated", o.TeamID, "declared backup required; access is not broadened", "active", now})
	}
	return o, s.save(o)
}
func (s *Store) List(repo, viewer string, owner bool) ([]Outcome, Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return nil, Summary{}, e
	}
	out := []Outcome{}
	sum := Summary{}
	for _, o := range xs {
		if o.Audience == "owners" && !owner && !has(o.Owners, viewer) {
			continue
		}
		if o.Audience == "repository" && viewer == "" {
			continue
		}
		if !o.UserOutcomeConsent {
			o.UserOutcome = ""
		}
		out = append(out, o)
		sum.Outcomes++
		sum.Metrics.AlertVolume += o.Metrics.AlertVolume
		sum.Metrics.Deduplicated += o.Metrics.Deduplicated
		sum.Metrics.Handoffs += o.Metrics.Handoffs
		sum.Metrics.Escalations += o.Metrics.Escalations
		sum.Metrics.MissedTargets += o.Metrics.MissedTargets
		sum.Metrics.Interruptions += o.Metrics.Interruptions
		sum.Metrics.ResponderMinutes += o.Metrics.ResponderMinutes
		sum.Metrics.AgentCost += o.Metrics.AgentCost
		sum.Metrics.IncidentCount += o.Metrics.IncidentCount
		sum.Metrics.AffectedUsers += o.Metrics.AffectedUsers
		sum.Metrics.RecoveredUsers += o.Metrics.RecoveredUsers
		if o.Metrics.FalsePositive {
			sum.Metrics.FalsePositive = true
		}
		for _, c := range o.Controls {
			if c.Kind == "routing_paused" {
				sum.PausedRouting++
			}
			if c.Kind == "backup_activated" {
				sum.ActivatedBackups++
			}
		}
	}
	return out, sum, nil
}
func (s *Store) mutate(repo, x, actor string, expected int64, fn func(*Outcome) error) (Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, e := s.read(repo, x)
	if e != nil {
		return o, e
	}
	if o.Revision != expected {
		return o, ErrConflict
	}
	if !has(o.Owners, actor) {
		return o, ErrInvalid
	}
	if e = fn(&o); e != nil {
		return o, e
	}
	o.Revision++
	return o, s.save(o)
}
func (s *Store) Review(repo, x, actor string, expected int64, decision, rationale string) (Outcome, error) {
	if !map[string]bool{"confirmed": true, "corrected": true, "disputed": true}[decision] || rationale == "" {
		return Outcome{}, ErrInvalid
	}
	return s.mutate(repo, x, actor, expected, func(o *Outcome) error {
		o.Reviews = append(o.Reviews, Review{ID: id(), ActorID: actor, Decision: decision, Rationale: rationale, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Correct(repo, x, actor string, expected int64, c Correction) (Outcome, error) {
	if !map[string]bool{"signal_policy": true, "routing_policy": true}[c.Kind] || c.Summary == "" || c.ResourceID == "" {
		return Outcome{}, ErrInvalid
	}
	return s.mutate(repo, x, actor, expected, func(o *Outcome) error {
		c.ID = id()
		c.ProposedByID = actor
		c.CreatedAt = s.now().UTC()
		if c.MaterialAuthorityChange {
			c.Status = "pending_ordinary_approval"
		} else {
			c.Status = "proposed"
		}
		o.Corrections = append(o.Corrections, c)
		return nil
	})
}
func (s *Store) Approve(repo, x, correction, actor string, expected int64) (Outcome, error) {
	return s.mutate(repo, x, actor, expected, func(o *Outcome) error {
		for i := range o.Corrections {
			c := &o.Corrections[i]
			if c.ID == correction && c.MaterialAuthorityChange && c.ProposedByID != actor && c.Status == "pending_ordinary_approval" {
				c.Status = "approved"
				c.ApprovedByID = actor
				return nil
			}
		}
		return ErrInvalid
	})
}
func (s *Store) AddWork(repo, x, actor string, expected int64, w Work) (Outcome, error) {
	if !map[string]bool{"reliability": true, "documentation": true, "automation": true, "staffing": true}[w.Kind] || !map[string]bool{"human": true, "agent": true}[w.OwnerKind] || w.OwnerID == "" || w.ResourceID == "" || w.Summary == "" {
		return Outcome{}, ErrInvalid
	}
	return s.mutate(repo, x, actor, expected, func(o *Outcome) error {
		w.ID = id()
		w.CreatedByID = actor
		w.CreatedAt = s.now().UTC()
		o.Work = append(o.Work, w)
		return nil
	})
}
