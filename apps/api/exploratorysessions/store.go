// Package exploratorysessions owns bounded, revision-exact collaborative exploration.
package exploratorysessions

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

var ErrNotFound = errors.New("exploratory session not found")
var ErrInvalid = errors.New("invalid exploratory session")
var ErrConflict = errors.New("exploratory session changed")
var ErrScope = errors.New("exploratory session scope exceeded")

type Candidate struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
}
type Access struct {
	ExpiresAt       time.Time `json:"expires_at"`
	Environment     string    `json:"environment"`
	Network         string    `json:"network"`
	AllowedRoutes   []string  `json:"allowed_routes"`
	AllowedCommands []string  `json:"allowed_commands"`
}
type TestData struct {
	Description           string   `json:"description"`
	PrivacyClassification string   `json:"privacy_classification"`
	Synthetic             bool     `json:"synthetic"`
	SourceReference       string   `json:"source_reference,omitempty"`
	Transformations       []string `json:"transformations"`
}
type Budget struct {
	MaxMinutes      int     `json:"max_minutes"`
	MaxCost         float64 `json:"max_cost"`
	MaxAgentActions int     `json:"max_agent_actions"`
}
type Participant struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Approved bool   `json:"approved"`
	Role     string `json:"role"`
}
type Charter struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Risk        string   `json:"risk"`
	RiskLevel   string   `json:"risk_level"`
	Mission     string   `json:"mission"`
	OwnerID     string   `json:"owner_id"`
	Routes      []string `json:"routes"`
	BehaviorIDs []string `json:"behavior_ids"`
	Techniques  []string `json:"techniques"`
	Status      string   `json:"status"`
}
type Input struct {
	Title           string        `json:"title"`
	OriginKind      string        `json:"origin_kind"`
	OriginReference string        `json:"origin_reference"`
	Candidate       Candidate     `json:"candidate"`
	QualityPlanID   string        `json:"quality_plan_id,omitempty"`
	Access          Access        `json:"access"`
	TestData        TestData      `json:"test_data"`
	Budget          Budget        `json:"budget"`
	Participants    []Participant `json:"participants"`
	Charters        []Charter     `json:"charters"`
	Uncertainty     string        `json:"uncertainty"`
}
type Artifact struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"media_type"`
	Sanitized bool   `json:"sanitized"`
}
type EventInput struct {
	Kind        string     `json:"kind"`
	CharterID   string     `json:"charter_id,omitempty"`
	Route       string     `json:"route,omitempty"`
	BehaviorIDs []string   `json:"behavior_ids,omitempty"`
	Inputs      []string   `json:"inputs,omitempty"`
	Observation string     `json:"observation,omitempty"`
	Command     string     `json:"command,omitempty"`
	Coverage    string     `json:"coverage,omitempty"`
	Uncertainty string     `json:"uncertainty,omitempty"`
	Artifacts   []Artifact `json:"artifacts,omitempty"`
	Cost        float64    `json:"cost,omitempty"`
	AgentAction bool       `json:"agent_action,omitempty"`
}
type Event struct {
	ID string `json:"id"`
	EventInput
	ActorID           string    `json:"actor_id"`
	CandidateRevision string    `json:"candidate_revision"`
	Stale             bool      `json:"stale"`
	StaleReason       string    `json:"stale_reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
type FindingInput struct {
	CharterID         string   `json:"charter_id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	EventIDs          []string `json:"event_ids"`
	ReproductionSteps []string `json:"reproduction_steps"`
	Uncertainty       string   `json:"uncertainty"`
}
type Finding struct {
	ID string `json:"id"`
	FindingInput
	AuthorID          string             `json:"author_id"`
	Status            string             `json:"status"`
	Classification    string             `json:"classification"`
	Reproduction      string             `json:"reproduction"`
	Rationale         string             `json:"rationale,omitempty"`
	CandidateRevision string             `json:"candidate_revision"`
	Stale             bool               `json:"stale"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	Delivery          *Delivery          `json:"delivery,omitempty"`
	Resolution        *FindingResolution `json:"resolution,omitempty"`
}
type Delivery struct {
	IssueID               string     `json:"issue_id"`
	ProposalID            string     `json:"proposal_id"`
	TaskID                string     `json:"task_id"`
	OwnerKind             string     `json:"owner_kind"`
	OwnerID               string     `json:"owner_id"`
	AcceptanceCriteria    []string   `json:"acceptance_criteria"`
	PermittedEventIDs     []string   `json:"permitted_event_ids"`
	MinimizedReproduction []string   `json:"minimized_reproduction"`
	PullRequestID         string     `json:"pull_request_id,omitempty"`
	BaseRevision          string     `json:"base_revision"`
	RepairRevision        string     `json:"repair_revision,omitempty"`
	FailingEvidenceID     string     `json:"failing_evidence_id,omitempty"`
	PassingEvidenceID     string     `json:"passing_evidence_id,omitempty"`
	ReviewID              string     `json:"review_id,omitempty"`
	QualityPlanID         string     `json:"quality_plan_id,omitempty"`
	QualityPlanVersion    int64      `json:"quality_plan_version,omitempty"`
	ScenarioID            string     `json:"regression_scenario_id,omitempty"`
	ScenarioVersion       int64      `json:"regression_scenario_version,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	VerifiedAt            *time.Time `json:"verified_at,omitempty"`
}
type FindingResolution struct {
	Kind               string    `json:"kind"`
	Rationale          string    `json:"rationale"`
	DuplicateFindingID string    `json:"duplicate_finding_id,omitempty"`
	Environment        string    `json:"environment,omitempty"`
	FollowUp           string    `json:"follow_up,omitempty"`
	ActorID            string    `json:"actor_id"`
	CreatedAt          time.Time `json:"created_at"`
}
type Control struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	Guidance  string    `json:"guidance,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Session struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Input
	CurrentRevision string    `json:"current_revision"`
	Status          string    `json:"status"`
	Revision        int64     `json:"revision"`
	Events          []Event   `json:"timeline"`
	Findings        []Finding `json:"findings"`
	Controls        []Control `json:"controls"`
	Cost            float64   `json:"cost"`
	AgentActions    int       `json:"agent_actions"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type Catalog struct {
	Items []Session `json:"items"`
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func allowed(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func validInput(x Input, now time.Time) bool {
	if strings.TrimSpace(x.Title) == "" || !allowed(x.OriginKind, "pull_request_preview", "release_candidate", "issue", "quality_plan") || x.OriginReference == "" || !allowed(x.Candidate.Kind, "pull_request", "release", "issue", "quality_plan") || x.Candidate.Reference == "" || x.Candidate.Revision == "" || x.Access.ExpiresAt.Before(now) || !allowed(x.Access.Network, "none", "loopback", "preview") || x.Access.Environment == "" || x.Budget.MaxMinutes < 1 || x.Budget.MaxCost < 0 || x.Budget.MaxAgentActions < 0 || len(x.Participants) == 0 || len(x.Charters) == 0 {
		return false
	}
	if x.Access.ExpiresAt.After(now.Add(time.Duration(x.Budget.MaxMinutes) * time.Minute)) {
		return false
	}
	people := map[string]Participant{}
	for _, p := range x.Participants {
		if p.ID == "" || !allowed(p.Kind, "human", "agent") || !allowed(p.Role, "lead", "tester", "observer") || (p.Kind == "agent" && !p.Approved) {
			return false
		}
		people[p.ID] = p
	}
	seen := map[string]bool{}
	for _, c := range x.Charters {
		if c.ID == "" || seen[c.ID] || c.Title == "" || c.Risk == "" || !allowed(c.RiskLevel, "low", "medium", "high", "critical") || c.Mission == "" || people[c.OwnerID].ID == "" || len(c.BehaviorIDs) == 0 {
			return false
		}
		seen[c.ID] = true
	}
	if x.TestData.Description == "" || !allowed(x.TestData.PrivacyClassification, "public", "internal", "confidential", "restricted") || (!x.TestData.Synthetic && len(x.TestData.Transformations) == 0) {
		return false
	}
	return true
}
func (s *Store) path(repo, sid string) string { return filepath.Join(s.root, repo, sid+".json") }
func (s *Store) save(x Session) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, sid string) (Session, error) {
	var x Session
	b, e := os.ReadFile(s.path(repo, sid))
	if errors.Is(e, fs.ErrNotExist) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) Create(repo, actor string, in Input) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.now().UTC()
	if repo == "" || actor == "" || !validInput(in, n) {
		return Session{}, ErrInvalid
	}
	participant := false
	for _, p := range in.Participants {
		participant = participant || p.ID == actor
	}
	if !participant {
		return Session{}, ErrScope
	}
	for i := range in.Charters {
		in.Charters[i].Status = "active"
	}
	x := Session{ID: id(), RepositoryID: repo, Input: in, CurrentRevision: in.Candidate.Revision, Status: "active", Revision: 1, Events: []Event{}, Findings: []Finding{}, Controls: []Control{}, CreatedBy: actor, CreatedAt: n, UpdatedAt: n}
	return x, s.save(x)
}
func (s *Store) Get(repo, sid string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, sid)
}
func (s *Store) Catalog(repo string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Catalog{Items: []Session{}}
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	for _, f := range entries {
		if filepath.Ext(f.Name()) == ".json" {
			x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
			if e != nil {
				return out, e
			}
			out.Items = append(out.Items, x)
		}
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].UpdatedAt.After(out.Items[j].UpdatedAt) })
	return out, nil
}
func participant(x Session, actor string) (Participant, bool) {
	for _, p := range x.Participants {
		if p.ID == actor {
			return p, true
		}
	}
	return Participant{}, false
}
func charter(x Session, cid, actor string) (Charter, bool) {
	for _, c := range x.Charters {
		if c.ID == cid && (c.OwnerID == actor || cid == "") {
			return c, true
		}
	}
	return Charter{}, false
}
func routeAllowed(prefixes []string, route string) bool {
	if route == "" {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(route, p) {
			return true
		}
	}
	return false
}
func (s *Store) Append(repo, sid, actor string, expected int64, in EventInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	p, ok := participant(x, actor)
	if !ok || x.Status != "active" || x.Access.ExpiresAt.Before(s.now()) || expected != x.Revision {
		return x, ErrConflict
	}
	if !allowed(in.Kind, "route", "input", "observation", "screenshot", "trace", "command", "coverage", "uncertainty", "note") {
		return x, ErrInvalid
	}
	c, ok := charter(x, in.CharterID, actor)
	if !ok {
		return x, ErrScope
	}
	if !routeAllowed(x.Access.AllowedRoutes, in.Route) || !routeAllowed(c.Routes, in.Route) {
		return x, ErrScope
	}
	if in.Command != "" && !routeAllowed(x.Access.AllowedCommands, in.Command) {
		return x, ErrScope
	}
	if p.Kind == "agent" && !in.AgentAction {
		return x, ErrInvalid
	}
	if in.AgentAction {
		x.AgentActions++
		if x.AgentActions > x.Budget.MaxAgentActions {
			return x, ErrScope
		}
	}
	x.Cost += in.Cost
	if x.Cost > x.Budget.MaxCost {
		return x, ErrScope
	}
	for _, a := range in.Artifacts {
		if !allowed(a.Kind, "screenshot", "trace", "recording", "log") || a.Reference == "" || a.SHA256 == "" || !a.Sanitized {
			return x, ErrInvalid
		}
	}
	n := s.now().UTC()
	x.Events = append(x.Events, Event{ID: id(), EventInput: in, ActorID: actor, CandidateRevision: x.CurrentRevision, CreatedAt: n})
	x.Revision++
	x.UpdatedAt = n
	e = s.save(x)
	return x, e
}
func (s *Store) AddFinding(repo, sid, actor string, expected int64, in FindingInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	if _, ok := participant(x, actor); !ok || expected != x.Revision || x.Status != "active" {
		return x, ErrConflict
	}
	if in.Title == "" || in.Description == "" || in.CharterID == "" || len(in.EventIDs) == 0 || len(in.ReproductionSteps) == 0 {
		return x, ErrInvalid
	}
	if _, ok := charter(x, in.CharterID, actor); !ok {
		return x, ErrScope
	}
	events := map[string]bool{}
	for _, v := range x.Events {
		events[v.ID] = true
	}
	for _, v := range in.EventIDs {
		if !events[v] {
			return x, ErrInvalid
		}
	}
	n := s.now().UTC()
	x.Findings = append(x.Findings, Finding{ID: id(), FindingInput: in, AuthorID: actor, Status: "open", Classification: "unclassified", Reproduction: "unattempted", CandidateRevision: x.CurrentRevision, CreatedAt: n, UpdatedAt: n})
	x.Revision++
	x.UpdatedAt = n
	e = s.save(x)
	return x, e
}

type FindingUpdate struct {
	Status         string `json:"status,omitempty"`
	Classification string `json:"classification,omitempty"`
	Reproduction   string `json:"reproduction,omitempty"`
	Rationale      string `json:"rationale"`
}

func (s *Store) UpdateFinding(repo, sid, fid, actor string, expected int64, in FindingUpdate) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	if _, ok := participant(x, actor); !ok || expected != x.Revision {
		return x, ErrConflict
	}
	found := false
	for i := range x.Findings {
		f := &x.Findings[i]
		if f.ID != fid {
			continue
		}
		found = true
		if in.Status != "" {
			if !allowed(in.Status, "open", "classified", "discarded", "resolved") {
				return x, ErrInvalid
			}
			f.Status = in.Status
		}
		if in.Classification != "" {
			if !allowed(in.Classification, "defect", "usability", "accessibility", "performance", "security", "data", "question", "duplicate", "false_positive") {
				return x, ErrInvalid
			}
			f.Classification = in.Classification
		}
		if in.Reproduction != "" {
			if !allowed(in.Reproduction, "unattempted", "reproduced", "intermittent", "environment_specific", "not_reproduced") {
				return x, ErrInvalid
			}
			f.Reproduction = in.Reproduction
		}
		f.Rationale = in.Rationale
		f.UpdatedAt = s.now().UTC()
	}
	if !found {
		return x, ErrNotFound
	}
	x.Revision++
	x.UpdatedAt = s.now().UTC()
	e = s.save(x)
	return x, e
}

type ControlInput struct {
	Action   string `json:"action"`
	Guidance string `json:"guidance"`
}

func (s *Store) Control(repo, sid, actor string, expected int64, in ControlInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	p, ok := participant(x, actor)
	if !ok || p.Role != "lead" || expected != x.Revision {
		return x, ErrConflict
	}
	if !allowed(in.Action, "guide", "pause", "resume", "close") {
		return x, ErrInvalid
	}
	if in.Action == "pause" {
		x.Status = "paused"
	}
	if in.Action == "resume" {
		x.Status = "active"
	}
	if in.Action == "close" {
		x.Status = "closed"
	}
	n := s.now().UTC()
	x.Controls = append(x.Controls, Control{ID: id(), Action: in.Action, ActorID: actor, Guidance: in.Guidance, CreatedAt: n})
	x.Revision++
	x.UpdatedAt = n
	e = s.save(x)
	return x, e
}

type CandidateUpdate struct {
	Revision            string   `json:"revision"`
	AffectedRoutes      []string `json:"affected_routes"`
	AffectedBehaviorIDs []string `json:"affected_behavior_ids"`
}

func (s *Store) UpdateCandidate(repo, sid, actor string, expected int64, in CandidateUpdate) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	p, ok := participant(x, actor)
	if !ok || p.Role != "lead" || expected != x.Revision {
		return x, ErrConflict
	}
	if in.Revision == "" || in.Revision == x.CurrentRevision {
		return x, ErrInvalid
	}
	affected := func(route string, behaviors []string) bool {
		for _, r := range in.AffectedRoutes {
			if strings.HasPrefix(route, r) {
				return true
			}
		}
		for _, a := range in.AffectedBehaviorIDs {
			for _, b := range behaviors {
				if a == b {
					return true
				}
			}
		}
		return false
	}
	for i := range x.Events {
		if affected(x.Events[i].Route, x.Events[i].BehaviorIDs) {
			x.Events[i].Stale = true
			x.Events[i].StaleReason = "candidate changed from " + x.Events[i].CandidateRevision + " to " + in.Revision
		}
	}
	for i := range x.Findings {
		for _, eid := range x.Findings[i].EventIDs {
			for _, ev := range x.Events {
				if ev.ID == eid && ev.Stale {
					x.Findings[i].Stale = true
				}
			}
		}
	}
	x.CurrentRevision = in.Revision
	x.Candidate.Revision = in.Revision
	x.Revision++
	x.UpdatedAt = s.now().UTC()
	e = s.save(x)
	return x, e
}

type DeliveryLinkInput struct {
	IssueID, ProposalID, TaskID, OwnerKind, OwnerID              string
	AcceptanceCriteria, PermittedEventIDs, MinimizedReproduction []string
}

func (s *Store) LinkDelivery(repo, sid, fid, actor string, expected int64, in DeliveryLinkInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	if _, ok := participant(x, actor); !ok || expected != x.Revision {
		return x, ErrConflict
	}
	if in.IssueID == "" || in.ProposalID == "" || in.TaskID == "" || !allowed(in.OwnerKind, "human", "agent") || in.OwnerID == "" || len(in.AcceptanceCriteria) == 0 || len(in.PermittedEventIDs) == 0 || len(in.MinimizedReproduction) == 0 {
		return x, ErrInvalid
	}
	events := map[string]bool{}
	for _, ev := range x.Events {
		events[ev.ID] = !ev.Stale
	}
	for _, eid := range in.PermittedEventIDs {
		if !events[eid] {
			return x, ErrInvalid
		}
	}
	n := s.now().UTC()
	for i := range x.Findings {
		if x.Findings[i].ID == fid {
			f := &x.Findings[i]
			if f.Stale || f.Classification == "unclassified" || f.Classification == "duplicate" || f.Classification == "false_positive" || f.Reproduction != "reproduced" || f.Delivery != nil || f.Resolution != nil {
				return x, ErrConflict
			}
			f.Delivery = &Delivery{IssueID: in.IssueID, ProposalID: in.ProposalID, TaskID: in.TaskID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, AcceptanceCriteria: in.AcceptanceCriteria, PermittedEventIDs: in.PermittedEventIDs, MinimizedReproduction: in.MinimizedReproduction, BaseRevision: f.CandidateRevision, CreatedAt: n}
			f.Status = "classified"
			f.UpdatedAt = n
			x.Revision++
			x.UpdatedAt = n
			return x, s.save(x)
		}
	}
	return x, ErrNotFound
}

type VerificationInput struct {
	PullRequestID      string `json:"pull_request_id"`
	BaseRevision       string `json:"base_revision"`
	RepairRevision     string `json:"repair_revision"`
	FailingEvidenceID  string `json:"failing_evidence_id"`
	PassingEvidenceID  string `json:"passing_evidence_id"`
	ReviewID           string `json:"review_id"`
	QualityPlanID      string `json:"quality_plan_id"`
	QualityPlanVersion int64  `json:"quality_plan_version"`
	ScenarioID         string `json:"regression_scenario_id"`
	ScenarioVersion    int64  `json:"regression_scenario_version"`
}

func (s *Store) VerifyDelivery(repo, sid, fid, actor string, expected int64, in VerificationInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	if _, ok := participant(x, actor); !ok || expected != x.Revision {
		return x, ErrConflict
	}
	if in.PullRequestID == "" || in.BaseRevision == "" || in.RepairRevision == "" || in.BaseRevision == in.RepairRevision || in.FailingEvidenceID == "" || in.PassingEvidenceID == "" || in.FailingEvidenceID == in.PassingEvidenceID || in.ReviewID == "" || in.QualityPlanID == "" || in.QualityPlanVersion < 1 || in.ScenarioID == "" || in.ScenarioVersion < 1 {
		return x, ErrInvalid
	}
	n := s.now().UTC()
	for i := range x.Findings {
		if x.Findings[i].ID == fid {
			d := x.Findings[i].Delivery
			if d == nil || d.BaseRevision != in.BaseRevision || d.VerifiedAt != nil {
				return x, ErrConflict
			}
			d.PullRequestID = in.PullRequestID
			d.RepairRevision = in.RepairRevision
			d.FailingEvidenceID = in.FailingEvidenceID
			d.PassingEvidenceID = in.PassingEvidenceID
			d.ReviewID = in.ReviewID
			d.QualityPlanID = in.QualityPlanID
			d.QualityPlanVersion = in.QualityPlanVersion
			d.ScenarioID = in.ScenarioID
			d.ScenarioVersion = in.ScenarioVersion
			d.VerifiedAt = &n
			x.Findings[i].Status = "resolved"
			x.Findings[i].UpdatedAt = n
			x.Revision++
			x.UpdatedAt = n
			return x, s.save(x)
		}
	}
	return x, ErrNotFound
}

type ResolutionInput struct {
	Kind               string `json:"kind"`
	Rationale          string `json:"rationale"`
	DuplicateFindingID string `json:"duplicate_finding_id"`
	Environment        string `json:"environment"`
	FollowUp           string `json:"follow_up"`
}

func (s *Store) ResolveWithoutDelivery(repo, sid, fid, actor string, expected int64, in ResolutionInput) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, sid)
	if e != nil {
		return x, e
	}
	if _, ok := participant(x, actor); !ok || expected != x.Revision {
		return x, ErrConflict
	}
	if !allowed(in.Kind, "flaky", "duplicate", "environment_specific", "not_reproducible") || strings.TrimSpace(in.Rationale) == "" || (in.Kind == "duplicate" && in.DuplicateFindingID == "") || (in.Kind == "environment_specific" && in.Environment == "") || (in.Kind == "flaky" && in.FollowUp == "") {
		return x, ErrInvalid
	}
	n := s.now().UTC()
	for i := range x.Findings {
		if x.Findings[i].ID == fid {
			f := &x.Findings[i]
			if f.Delivery != nil || f.Resolution != nil {
				return x, ErrConflict
			}
			f.Resolution = &FindingResolution{Kind: in.Kind, Rationale: in.Rationale, DuplicateFindingID: in.DuplicateFindingID, Environment: in.Environment, FollowUp: in.FollowUp, ActorID: actor, CreatedAt: n}
			f.Status = "resolved"
			f.UpdatedAt = n
			x.Revision++
			x.UpdatedAt = n
			return x, s.save(x)
		}
	}
	return x, ErrNotFound
}
