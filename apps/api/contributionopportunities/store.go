// Package contributionopportunities owns repository-scoped newcomer work
// descriptions, matching preferences, and exclusive time-bounded claims.
package contributionopportunities

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
	ErrNotFound = errors.New("opportunity not found")
	ErrInvalid  = errors.New("invalid opportunity")
	ErrConflict = errors.New("opportunity changed")
)

type Source struct {
	Kind           string `json:"kind"`
	ResourceID     string `json:"resource_id"`
	ParentID       string `json:"parent_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
}
type Opportunity struct {
	ID                 string    `json:"id"`
	RepositoryID       string    `json:"repository_id"`
	Source             Source    `json:"source"`
	Title              string    `json:"title"`
	RequiredSkills     []string  `json:"required_skills"`
	Interests          []string  `json:"interests"`
	ExpectedOutcome    string    `json:"expected_outcome"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	SampleData         []string  `json:"sample_data,omitempty"`
	Scope              []string  `json:"scope"`
	Dependencies       []string  `json:"dependencies"`
	Risk               string    `json:"risk"`
	MentorIDs          []string  `json:"mentor_ids"`
	Assistance         string    `json:"assistance"`
	Revision           string    `json:"revision"`
	Ready              bool      `json:"ready"`
	SourceState        string    `json:"source_state"`
	PublishedByID      string    `json:"published_by_id"`
	Version            int64     `json:"version"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
type Input struct {
	Source             Source   `json:"source"`
	RequiredSkills     []string `json:"required_skills"`
	Interests          []string `json:"interests"`
	ExpectedOutcome    string   `json:"expected_outcome"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	SampleData         []string `json:"sample_data,omitempty"`
	Scope              []string `json:"scope"`
	Dependencies       []string `json:"dependencies"`
	Risk               string   `json:"risk"`
	MentorIDs          []string `json:"mentor_ids"`
	Assistance         string   `json:"assistance"`
}
type Profile struct {
	ActorID        string    `json:"actor_id"`
	Interests      []string  `json:"interests"`
	Skills         []string  `json:"skills"`
	MaxRisk        string    `json:"max_risk"`
	AvailableHours int       `json:"available_hours"`
	Assistance     string    `json:"assistance"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type Claim struct {
	ID            string     `json:"id"`
	OpportunityID string     `json:"opportunity_id"`
	ActorID       string     `json:"actor_id"`
	Note          string     `json:"note,omitempty"`
	ClaimedAt     time.Time  `json:"claimed_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ReleasedByID  string     `json:"released_by_id,omitempty"`
}
type Data struct {
	Opportunities  []Opportunity   `json:"opportunities"`
	Profiles       []Profile       `json:"profiles"`
	Claims         []Claim         `json:"claims"`
	Reports        []Report        `json:"reports"`
	Collaborations []Collaboration `json:"collaborations"`
	Outcomes       []Outcome       `json:"outcomes"`
}

// Outcome is the immutable maintainer assessment of one delivered guided
// contribution. Delivery evidence is resolved by the HTTP boundary before it
// reaches the store; the record itself grants no future authority.
type Outcome struct {
	OpportunityID   string    `json:"opportunity_id"`
	ContributorID   string    `json:"contributor_id"`
	PullRequestID   string    `json:"pull_request_id"`
	ReleaseID       string    `json:"release_id"`
	Credit          string    `json:"credit"`
	Feedback        string    `json:"feedback"`
	SupportHours    float64   `json:"support_hours"`
	Readiness       string    `json:"readiness"`
	NextOpportunity string    `json:"next_opportunity,omitempty"`
	RecordedByID    string    `json:"recorded_by_id"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type Collaboration struct {
	OpportunityID         string               `json:"opportunity_id"`
	ClaimID               string               `json:"claim_id"`
	WorkspaceID           string               `json:"workspace_id"`
	WorkspaceRepositoryID string               `json:"workspace_repository_id"`
	ContributorID         string               `json:"contributor_id"`
	Revision              string               `json:"revision"`
	State                 string               `json:"state"`
	ResponseExpectedBy    time.Time            `json:"response_expected_by"`
	LastResponseAt        *time.Time           `json:"last_response_at,omitempty"`
	Presence              []HelpPresence       `json:"presence"`
	Mentors               []MentorAvailability `json:"mentors"`
	Controls              []AgentControl       `json:"agent_controls"`
	Events                []HelpEvent          `json:"events"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
}
type HelpPresence struct {
	ActorID    string    `json:"actor_id"`
	Role       string    `json:"role"`
	Surface    string    `json:"surface"`
	ObservedAt time.Time `json:"observed_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type MentorAvailability struct {
	MentorID       string     `json:"mentor_id"`
	State          string     `json:"state"`
	Note           string     `json:"note,omitempty"`
	AvailableUntil *time.Time `json:"available_until,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
type AgentControl struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	Mode        string    `json:"mode"`
	Paths       []string  `json:"paths,omitempty"`
	State       string    `json:"state"`
	GrantedByID string    `json:"granted_by_id"`
	Version     int64     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type HelpEvent struct {
	Sequence        int64     `json:"sequence"`
	Kind            string    `json:"kind"`
	ActorID         string    `json:"actor_id"`
	Role            string    `json:"role"`
	Message         string    `json:"message"`
	DecisionOwnerID string    `json:"decision_owner_id,omitempty"`
	TargetID        string    `json:"target_id,omitempty"`
	Paths           []string  `json:"paths,omitempty"`
	Resolved        bool      `json:"resolved"`
	CreatedAt       time.Time `json:"created_at"`
}
type Report struct {
	ID            string    `json:"id"`
	OpportunityID string    `json:"opportunity_id"`
	ActorID       string    `json:"actor_id"`
	WorkspaceID   string    `json:"workspace_id"`
	Kind          string    `json:"kind"`
	Detail        string    `json:"detail"`
	CreatedAt     time.Time `json:"created_at"`
}
type Match struct {
	Opportunity       Opportunity `json:"opportunity"`
	Score             int         `json:"score"`
	Reasons           []string    `json:"reasons"`
	Gaps              []string    `json:"gaps"`
	Claim             *Claim      `json:"claim,omitempty"`
	Available         bool        `json:"available"`
	GrantsWriteAccess bool        `json:"grants_write_access"`
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
	root, _ = filepath.Abs(root)
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func clean(xs []string, required bool) bool {
	if required && len(xs) == 0 || len(xs) > 30 {
		return false
	}
	for i := range xs {
		xs[i] = strings.TrimSpace(xs[i])
		if xs[i] == "" || len(xs[i]) > 500 {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	kinds := map[string]bool{"issue": true, "proposal": true, "proposal_task": true, "stewardship": true}
	risks := map[string]bool{"low": true, "medium": true, "high": true}
	assist := map[string]bool{"human": true, "agent": true, "human_or_agent": true, "none": true}
	return kinds[in.Source.Kind] && in.Source.ResourceID != "" && clean(in.RequiredSkills, true) && clean(in.Interests, true) && clean(in.Scope, true) && clean(in.Dependencies, false) && clean(in.MentorIDs, false) && clean(in.AcceptanceCriteria, false) && clean(in.SampleData, false) && strings.TrimSpace(in.ExpectedOutcome) != "" && risks[in.Risk] && assist[in.Assistance]
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Publish(repo, actor, title, revision, state string, ready bool, in Input) (Opportunity, error) {
	if repo == "" || actor == "" || title == "" || revision == "" || !valid(in) {
		return Opportunity{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Opportunity{}, err
	}
	for _, o := range d.Opportunities {
		if o.Source == in.Source {
			return Opportunity{}, ErrConflict
		}
	}
	now := s.now().UTC()
	criteria := in.AcceptanceCriteria
	if len(criteria) == 0 {
		criteria = []string{strings.TrimSpace(in.ExpectedOutcome)}
	}
	o := Opportunity{ID: id(), RepositoryID: repo, Source: in.Source, Title: title, RequiredSkills: in.RequiredSkills, Interests: in.Interests, ExpectedOutcome: strings.TrimSpace(in.ExpectedOutcome), AcceptanceCriteria: criteria, SampleData: in.SampleData, Scope: in.Scope, Dependencies: in.Dependencies, Risk: in.Risk, MentorIDs: in.MentorIDs, Assistance: in.Assistance, Revision: revision, Ready: ready, SourceState: state, PublishedByID: actor, Version: 1, CreatedAt: now, UpdatedAt: now}
	d.Opportunities = append(d.Opportunities, o)
	return o, s.write(repo, d)
}
func (s *Store) Get(repo, opportunity string) (Opportunity, error) {
	d, err := s.List(repo)
	if err != nil {
		return Opportunity{}, err
	}
	for _, o := range d.Opportunities {
		if o.ID == opportunity {
			return o, nil
		}
	}
	return Opportunity{}, ErrNotFound
}

func (s *Store) StartCollaboration(repo, opportunity, claimID, workspaceRepo, workspace, contributor, revision string, responseHours int) (Collaboration, error) {
	if workspaceRepo == "" || workspace == "" || contributor == "" || revision == "" || responseHours < 1 || responseHours > 168 {
		return Collaboration{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Collaboration{}, err
	}
	var claimOK bool
	for _, c := range d.Claims {
		if c.ID == claimID && c.OpportunityID == opportunity && c.ActorID == contributor && c.ReleasedAt == nil && c.ExpiresAt.After(s.now().UTC()) {
			claimOK = true
		}
	}
	if !claimOK {
		return Collaboration{}, ErrConflict
	}
	for _, c := range d.Collaborations {
		if c.OpportunityID == opportunity && c.ClaimID == claimID {
			return Collaboration{}, ErrConflict
		}
	}
	now := s.now().UTC()
	c := Collaboration{OpportunityID: opportunity, ClaimID: claimID, WorkspaceID: workspace, WorkspaceRepositoryID: workspaceRepo, ContributorID: contributor, Revision: revision, State: "active", ResponseExpectedBy: now.Add(time.Duration(responseHours) * time.Hour), Presence: []HelpPresence{}, Mentors: []MentorAvailability{}, Controls: []AgentControl{}, Events: []HelpEvent{{Sequence: 1, Kind: "started", ActorID: contributor, Role: "contributor", Message: "opened the shared help thread", DecisionOwnerID: contributor, CreatedAt: now}}, CreatedAt: now, UpdatedAt: now}
	d.Collaborations = append(d.Collaborations, c)
	return c, s.write(repo, d)
}

func (s *Store) Collaboration(repo, opportunity string) (Collaboration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Collaboration{}, err
	}
	for _, c := range d.Collaborations {
		if c.OpportunityID == opportunity {
			return activeHelpPresence(c, s.now().UTC()), nil
		}
	}
	return Collaboration{}, ErrNotFound
}

func (s *Store) mutateCollaboration(repo, opportunity string, fn func(*Collaboration, time.Time) error) (Collaboration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Collaboration{}, err
	}
	for i := range d.Collaborations {
		if d.Collaborations[i].OpportunityID == opportunity {
			now := s.now().UTC()
			if err = fn(&d.Collaborations[i], now); err != nil {
				return Collaboration{}, err
			}
			d.Collaborations[i].UpdatedAt = now
			return activeHelpPresence(d.Collaborations[i], now), s.write(repo, d)
		}
	}
	return Collaboration{}, ErrNotFound
}
func activeHelpPresence(c Collaboration, now time.Time) Collaboration {
	kept := c.Presence[:0]
	for _, p := range c.Presence {
		if p.ExpiresAt.After(now) {
			kept = append(kept, p)
		}
	}
	c.Presence = kept
	return c
}
func appendHelpEvent(c *Collaboration, now time.Time, e HelpEvent) {
	e.Sequence = int64(len(c.Events) + 1)
	e.CreatedAt = now
	c.Events = append(c.Events, e)
}

func (s *Store) ObserveHelp(repo, opportunity, actor, role, surface string) (Collaboration, error) {
	return s.mutateCollaboration(repo, opportunity, func(c *Collaboration, now time.Time) error {
		if c.State != "active" || actor == "" || !map[string]bool{"thread": true, "files": true, "setup": true, "checkpoint": true}[surface] {
			return ErrInvalid
		}
		kept := c.Presence[:0]
		for _, p := range c.Presence {
			if p.ExpiresAt.After(now) && p.ActorID != actor {
				kept = append(kept, p)
			}
		}
		c.Presence = append(kept, HelpPresence{ActorID: actor, Role: role, Surface: surface, ObservedAt: now, ExpiresAt: now.Add(45 * time.Second)})
		return nil
	})
}
func (s *Store) AddHelpEvent(repo, opportunity string, event HelpEvent) (Collaboration, error) {
	event.Message = strings.TrimSpace(event.Message)
	if event.ActorID == "" || event.Message == "" || len(event.Message) > 4000 || !map[string]bool{"question": true, "checkpoint_request": true, "advice": true, "answer": true, "handoff": true, "intervention": true, "resolved": true, "scope_changed": true, "agent_explanation": true, "agent_diagnosis": true, "agent_edit": true}[event.Kind] {
		return Collaboration{}, ErrInvalid
	}
	return s.mutateCollaboration(repo, opportunity, func(c *Collaboration, now time.Time) error {
		if c.State != "active" {
			return ErrConflict
		}
		appendHelpEvent(c, now, event)
		if event.Role == "mentor" && (event.Kind == "advice" || event.Kind == "answer") {
			c.LastResponseAt = &now
		}
		return nil
	})
}
func (s *Store) SetMentorAvailability(repo, opportunity, mentor, state, note string, until *time.Time) (Collaboration, error) {
	if !map[string]bool{"available": true, "limited": true, "unavailable": true}[state] || len(note) > 1000 {
		return Collaboration{}, ErrInvalid
	}
	return s.mutateCollaboration(repo, opportunity, func(c *Collaboration, now time.Time) error {
		for i := range c.Mentors {
			if c.Mentors[i].MentorID == mentor {
				c.Mentors[i] = MentorAvailability{MentorID: mentor, State: state, Note: strings.TrimSpace(note), AvailableUntil: until, UpdatedAt: now}
				appendHelpEvent(c, now, HelpEvent{Kind: "mentor_availability", ActorID: mentor, Role: "mentor", Message: state})
				return nil
			}
		}
		c.Mentors = append(c.Mentors, MentorAvailability{MentorID: mentor, State: state, Note: strings.TrimSpace(note), AvailableUntil: until, UpdatedAt: now})
		appendHelpEvent(c, now, HelpEvent{Kind: "mentor_availability", ActorID: mentor, Role: "mentor", Message: state})
		return nil
	})
}
func (s *Store) ControlAgent(repo, opportunity, actor, agent, mode string, paths []string, controlID, action string, version int64) (Collaboration, error) {
	return s.mutateCollaboration(repo, opportunity, func(c *Collaboration, now time.Time) error {
		if action == "grant" {
			if !map[string]bool{"explain": true, "diagnose_setup": true, "edit": true}[mode] || agent == "" || (mode == "edit" && len(paths) == 0) {
				return ErrInvalid
			}
			for _, p := range paths {
				if strings.TrimSpace(p) == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
					return ErrInvalid
				}
			}
			g := AgentControl{ID: id(), AgentID: agent, Mode: mode, Paths: paths, State: "active", GrantedByID: actor, Version: 1, CreatedAt: now, UpdatedAt: now}
			c.Controls = append(c.Controls, g)
			appendHelpEvent(c, now, HelpEvent{Kind: "agent_control", ActorID: actor, Role: "contributor", TargetID: agent, Message: "granted " + mode, Paths: paths, DecisionOwnerID: c.ContributorID})
			return nil
		}
		for i := range c.Controls {
			g := &c.Controls[i]
			if g.ID == controlID {
				if g.Version != version || g.State == "revoked" {
					return ErrConflict
				}
				if action != "pause" && action != "resume" && action != "revoke" {
					return ErrInvalid
				}
				g.State = map[string]string{"pause": "paused", "resume": "active", "revoke": "revoked"}[action]
				g.Version++
				g.UpdatedAt = now
				appendHelpEvent(c, now, HelpEvent{Kind: "intervention", ActorID: actor, Role: "contributor", TargetID: g.AgentID, Message: action + " agent assistance", DecisionOwnerID: c.ContributorID})
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) TransitionCollaboration(repo, opportunity, actor, role, state, reason string) (Collaboration, error) {
	if !map[string]bool{"reassignment_required": true, "exited": true}[state] || strings.TrimSpace(reason) == "" {
		return Collaboration{}, ErrInvalid
	}
	return s.mutateCollaboration(repo, opportunity, func(c *Collaboration, now time.Time) error {
		c.State = state
		for i := range c.Controls {
			if c.Controls[i].State != "revoked" {
				c.Controls[i].State = "revoked"
				c.Controls[i].Version++
				c.Controls[i].UpdatedAt = now
			}
		}
		c.Presence = nil
		appendHelpEvent(c, now, HelpEvent{Kind: state, ActorID: actor, Role: role, Message: strings.TrimSpace(reason), DecisionOwnerID: c.ContributorID})
		return nil
	})
}
func (s *Store) Report(repo, opportunity, actor, workspace, kind, detail string) (Report, error) {
	allowed := map[string]bool{"missing_access": true, "obsolete_instructions": true, "non_reproducible_prerequisite": true}
	lower := strings.ToLower(detail)
	credentialLike := []string{"-----begin private key", "authorization: bearer", "github_pat_", "ghp_", "aws_secret_access_key", "sk-"}
	unsafe := false
	for _, marker := range credentialLike {
		if strings.Contains(lower, marker) {
			unsafe = true
		}
	}
	if actor == "" || workspace == "" || !allowed[kind] || strings.TrimSpace(detail) == "" || unsafe {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Report{}, err
	}
	found := false
	for _, o := range d.Opportunities {
		if o.ID == opportunity {
			found = true
		}
	}
	if !found {
		return Report{}, ErrNotFound
	}
	r := Report{ID: id(), OpportunityID: opportunity, ActorID: actor, WorkspaceID: workspace, Kind: kind, Detail: strings.TrimSpace(detail), CreatedAt: s.now().UTC()}
	d.Reports = append(d.Reports, r)
	return r, s.write(repo, d)
}
func (s *Store) Complete(repo, opportunity, contributor, pull, release, credit, feedback string, supportHours float64, readiness, next, actor string) (Outcome, error) {
	credit, feedback, next = strings.TrimSpace(credit), strings.TrimSpace(feedback), strings.TrimSpace(next)
	if contributor == "" || pull == "" || release == "" || actor == "" || credit == "" || feedback == "" || len(credit) > 2000 || len(feedback) > 4000 || supportHours < 0 || supportHours > 1000 || !map[string]bool{"ready": true, "ready_with_support": true, "needs_guidance": true}[readiness] {
		return Outcome{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.read(repo)
	if err != nil {
		return Outcome{}, err
	}
	found := false
	for _, o := range d.Opportunities {
		found = found || o.ID == opportunity
	}
	if !found {
		return Outcome{}, ErrNotFound
	}
	for _, o := range d.Outcomes {
		if o.OpportunityID == opportunity {
			return Outcome{}, ErrConflict
		}
	}
	out := Outcome{OpportunityID: opportunity, ContributorID: contributor, PullRequestID: pull, ReleaseID: release, Credit: credit, Feedback: feedback, SupportHours: supportHours, Readiness: readiness, NextOpportunity: next, RecordedByID: actor, RecordedAt: s.now().UTC()}
	d.Outcomes = append(d.Outcomes, out)
	return out, s.write(repo, d)
}
func (s *Store) List(repo string) (Data, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo)
}
func (s *Store) Profile(repo, actor string, p Profile) (Profile, error) {
	if actor == "" || !clean(p.Interests, false) || !clean(p.Skills, false) || p.AvailableHours < 1 || p.AvailableHours > 100 || !map[string]bool{"low": true, "medium": true, "high": true}[p.MaxRisk] || !map[string]bool{"human": true, "agent": true, "human_or_agent": true, "none": true}[p.Assistance] {
		return p, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo)
	if e != nil {
		return p, e
	}
	p.ActorID = actor
	p.UpdatedAt = s.now().UTC()
	found := false
	for i := range d.Profiles {
		if d.Profiles[i].ActorID == actor {
			d.Profiles[i] = p
			found = true
		}
	}
	if !found {
		d.Profiles = append(d.Profiles, p)
	}
	return p, s.write(repo, d)
}
func (s *Store) Claim(repo, op, actor, note string, hours int) (Claim, error) {
	if hours < 1 || hours > 336 {
		return Claim{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo)
	if e != nil {
		return Claim{}, e
	}
	found := false
	for _, o := range d.Opportunities {
		if o.ID == op && o.Ready {
			found = true
		}
	}
	if !found {
		return Claim{}, ErrNotFound
	}
	now := s.now().UTC()
	for _, c := range d.Claims {
		if c.OpportunityID == op && c.ReleasedAt == nil && c.ExpiresAt.After(now) {
			return Claim{}, ErrConflict
		}
	}
	c := Claim{ID: id(), OpportunityID: op, ActorID: actor, Note: strings.TrimSpace(note), ClaimedAt: now, ExpiresAt: now.Add(time.Duration(hours) * time.Hour)}
	d.Claims = append(d.Claims, c)
	return c, s.write(repo, d)
}
func (s *Store) Release(repo, claim, actor string) (Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo)
	if e != nil {
		return Claim{}, e
	}
	for i := range d.Claims {
		c := &d.Claims[i]
		if c.ID == claim {
			if c.ActorID != actor || c.ReleasedAt != nil {
				return Claim{}, ErrConflict
			}
			now := s.now().UTC()
			c.ReleasedAt = &now
			c.ReleasedByID = actor
			return *c, s.write(repo, d)
		}
	}
	return Claim{}, ErrNotFound
}
func (s *Store) Matches(repo, actor string) ([]Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.read(repo)
	if e != nil {
		return nil, e
	}
	var p Profile
	for _, x := range d.Profiles {
		if x.ActorID == actor {
			p = x
		}
	}
	now := s.now().UTC()
	out := []Match{}
	for _, o := range d.Opportunities {
		m := Match{Opportunity: o, Available: o.Ready, GrantsWriteAccess: false}
		for i := range d.Claims {
			c := d.Claims[i]
			if c.OpportunityID == o.ID && c.ReleasedAt == nil && c.ExpiresAt.After(now) {
				m.Claim = &c
				if c.ActorID != actor {
					m.Available = false
					m.Gaps = append(m.Gaps, "reserved by another contributor until "+c.ExpiresAt.Format(time.RFC3339))
				} else {
					m.Reasons = append(m.Reasons, "you reserved this opportunity")
				}
			}
		}
		for _, x := range o.Interests {
			if contains(p.Interests, x) {
				m.Score += 30
				m.Reasons = append(m.Reasons, "matches interest: "+x)
			}
		}
		for _, x := range o.RequiredSkills {
			if contains(p.Skills, x) {
				m.Score += 20
				m.Reasons = append(m.Reasons, "skill fit: "+x)
			} else {
				m.Gaps = append(m.Gaps, "skill to learn: "+x)
			}
		}
		if riskRank(o.Risk) <= riskRank(p.MaxRisk) {
			m.Score += 15
			m.Reasons = append(m.Reasons, "risk fits your constraint")
		} else {
			m.Gaps = append(m.Gaps, "risk exceeds your preference")
		}
		if o.Assistance == p.Assistance || o.Assistance == "human_or_agent" {
			m.Score += 10
			m.Reasons = append(m.Reasons, "requested assistance is available")
		}
		if o.Ready {
			m.Score += 25
		} else {
			m.Gaps = append(m.Gaps, "source is not currently ready")
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if strings.EqualFold(v, x) {
			return true
		}
	}
	return false
}
func riskRank(x string) int { return map[string]int{"low": 1, "medium": 2, "high": 3}[x] }
func (s *Store) read(repo string) (Data, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Data{Opportunities: []Opportunity{}, Profiles: []Profile{}, Claims: []Claim{}, Reports: []Report{}}, nil
	}
	var d Data
	if e == nil {
		e = json.Unmarshal(b, &d)
	}
	return d, e
}
func (s *Store) write(repo string, d Data) error {
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(s.root, "opportunities-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(s.root, repo+".json"))
}
