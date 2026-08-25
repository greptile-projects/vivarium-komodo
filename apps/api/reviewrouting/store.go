// Package reviewrouting owns revision-bound reviewer suggestions and assignments.
package reviewrouting

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrInvalid = errors.New("invalid review routing")
var ErrNotFound = errors.New("review routing not found")
var ErrConflict = errors.New("review routing changed")

type Evidence struct {
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Summary    string `json:"summary"`
	Accessible bool   `json:"accessible"`
}
type Candidate struct {
	ParticipantID        string     `json:"participant_id"`
	Kind                 string     `json:"kind"`
	Expertise            []string   `json:"expertise"`
	CodeOwnership        bool       `json:"code_ownership"`
	ProjectKnowledge     bool       `json:"project_knowledge"`
	TeamResponsibility   bool       `json:"team_responsibility"`
	Available            bool       `json:"available"`
	CurrentLoad          int        `json:"current_load"`
	Capacity             int        `json:"capacity"`
	Conflict             string     `json:"conflict,omitempty"`
	AgentApproval        string     `json:"agent_approval,omitempty"`
	ApprovedCapabilities []string   `json:"approved_capabilities,omitempty"`
	Evidence             []Evidence `json:"evidence"`
}
type Suggestion struct {
	AreaID        string     `json:"area_id"`
	ParticipantID string     `json:"participant_id"`
	Kind          string     `json:"kind"`
	Eligible      bool       `json:"eligible"`
	Reasons       []string   `json:"reasons"`
	Evidence      []Evidence `json:"evidence"`
	Blockers      []string   `json:"blockers"`
}
type Assignment struct {
	ID               string     `json:"id"`
	AreaID           string     `json:"area_id"`
	ParticipantID    string     `json:"participant_id"`
	Kind             string     `json:"kind"`
	State            string     `json:"state"`
	PlanVersion      int64      `json:"plan_version"`
	Revision         string     `json:"revision"`
	Scope            []string   `json:"scope"`
	Questions        []string   `json:"acceptance_questions"`
	Deadline         *time.Time `json:"deadline,omitempty"`
	Escalation       string     `json:"escalation,omitempty"`
	InvitationReason string     `json:"invitation_reason"`
	AuthorityNotice  string     `json:"authority_notice"`
	CreatedBy        string     `json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	OutcomeReason    string     `json:"outcome_reason,omitempty"`
	Replaces         string     `json:"replaces,omitempty"`
}
type Event struct {
	AssignmentID string    `json:"assignment_id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	Reason       string    `json:"reason,omitempty"`
	At           time.Time `json:"at"`
}
type Routing struct {
	RepositoryID      string       `json:"repository_id"`
	PullRequestID     string       `json:"pull_request_id"`
	PlanVersion       int64        `json:"plan_version"`
	Revision          string       `json:"revision"`
	Suggestions       []Suggestion `json:"suggestions"`
	Assignments       []Assignment `json:"assignments"`
	Events            []Event      `json:"events"`
	ReassignmentAreas []string     `json:"reassignment_areas"`
}
type Area struct {
	ID                          string
	Expertise, Paths, Questions []string
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
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) read(repo, pull string) (Routing, error) {
	b, e := os.ReadFile(s.path(repo, pull))
	if errors.Is(e, os.ErrNotExist) {
		return Routing{}, ErrNotFound
	}
	var x Routing
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) write(x Routing) error {
	b, _ := json.MarshalIndent(x, "", "  ")
	p := s.path(x.RepositoryID, x.PullRequestID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	return os.WriteFile(p, b, 0640)
}
func (s *Store) Get(repo, pull string) (Routing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pull)
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if strings.EqualFold(v, x) {
			return true
		}
	}
	return false
}
func (s *Store) Suggest(repo, pull, revision string, version int64, areas []Area, candidates []Candidate) (Routing, error) {
	if repo == "" || pull == "" || revision == "" || version < 1 || len(areas) == 0 {
		return Routing{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, pull)
	if errors.Is(e, ErrNotFound) {
		x = Routing{RepositoryID: repo, PullRequestID: pull}
	} else if e != nil {
		return Routing{}, e
	}
	x.PlanVersion = version
	x.Revision = revision
	x.Suggestions = nil
	for _, a := range areas {
		for _, c := range candidates {
			q := Suggestion{AreaID: a.ID, ParticipantID: c.ParticipantID, Kind: c.Kind, Eligible: true}
			if c.ParticipantID == "" || !map[string]bool{"human": true, "agent": true}[c.Kind] {
				continue
			}
			if c.CodeOwnership {
				q.Reasons = append(q.Reasons, "owns changed code")
			}
			if c.ProjectKnowledge {
				q.Reasons = append(q.Reasons, "demonstrated project knowledge")
			}
			if c.TeamResponsibility {
				q.Reasons = append(q.Reasons, "responsible team")
			}
			for _, tag := range a.Expertise {
				if contains(c.Expertise, tag) {
					q.Reasons = append(q.Reasons, "matches "+tag+" expertise")
				}
			}
			if !c.Available {
				q.Blockers = append(q.Blockers, "unavailable")
			}
			if c.Capacity > 0 && c.CurrentLoad >= c.Capacity {
				q.Blockers = append(q.Blockers, "overloaded")
			}
			if c.Conflict != "" {
				q.Blockers = append(q.Blockers, "conflict_of_interest: "+c.Conflict)
			}
			if c.Kind == "agent" && c.AgentApproval == "" {
				q.Blockers = append(q.Blockers, "agent_capabilities_not_approved")
			} else if c.Kind == "agent" && len(c.ApprovedCapabilities) == 0 {
				q.Blockers = append(q.Blockers, "no_approved_review_capability")
			} else if c.Kind == "agent" {
				q.Reasons = append(q.Reasons, "approved agent capabilities: "+strings.Join(c.ApprovedCapabilities, ", "))
			}
			if len(q.Reasons) == 0 {
				q.Blockers = append(q.Blockers, "no_permitted_qualification_evidence")
			}
			for _, ev := range c.Evidence {
				if ev.Accessible {
					q.Evidence = append(q.Evidence, ev)
				} else {
					q.Blockers = append(q.Blockers, "inaccessible_evidence: "+ev.Kind)
				}
			}
			q.Eligible = len(q.Blockers) == 0
			x.Suggestions = append(x.Suggestions, q)
		}
	}
	sort.SliceStable(x.Suggestions, func(i, j int) bool {
		if x.Suggestions[i].AreaID != x.Suggestions[j].AreaID {
			return x.Suggestions[i].AreaID < x.Suggestions[j].AreaID
		}
		if x.Suggestions[i].Eligible != x.Suggestions[j].Eligible {
			return x.Suggestions[i].Eligible
		}
		return x.Suggestions[i].ParticipantID < x.Suggestions[j].ParticipantID
	})
	return x, s.write(x)
}
func (s *Store) Invite(repo, pull, actor string, version int64, revision string, area Area, c Candidate, deadline *time.Time, escalation, reason, replaces string) (Routing, error) {
	if actor == "" || reason == "" || area.ID == "" || c.ParticipantID == "" {
		return Routing{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, pull)
	if e != nil {
		return Routing{}, e
	}
	if x.PlanVersion != version || x.Revision != revision {
		return Routing{}, ErrConflict
	}
	eligible := false
	for _, q := range x.Suggestions {
		if q.AreaID == area.ID && q.ParticipantID == c.ParticipantID && q.Eligible {
			eligible = true
		}
	}
	if !eligible {
		return Routing{}, ErrInvalid
	}
	now := s.now().UTC()
	a := Assignment{ID: area.ID + "-" + c.ParticipantID + "-" + now.Format("20060102T150405.000000000"), AreaID: area.ID, ParticipantID: c.ParticipantID, Kind: c.Kind, State: "invited", PlanVersion: version, Revision: revision, Scope: area.Paths, Questions: area.Questions, Deadline: deadline, Escalation: escalation, InvitationReason: reason, AuthorityNotice: "This assignment permits only the bounded review scope and creates no repository, merge, secret, governance, policy, or operational authority.", CreatedBy: actor, CreatedAt: now, UpdatedAt: now, Replaces: replaces}
	if replaces != "" {
		for i := range x.Assignments {
			if x.Assignments[i].ID == replaces && x.Assignments[i].State != "released" {
				x.Assignments[i].State = "replaced"
				x.Assignments[i].UpdatedAt = now
			}
		}
	}
	x.Assignments = append(x.Assignments, a)
	x.Events = append(x.Events, Event{a.ID, actor, "invited", reason, now})
	derive(&x)
	return x, s.write(x)
}
func (s *Store) Transition(repo, pull, id, actor, state, reason string, maintainer bool) (Routing, error) {
	allowed := map[string]bool{"accepted": true, "declined": true, "unavailable": true, "overloaded": true, "recused": true, "released": true, "revoked": true}
	if !allowed[state] || reason == "" {
		return Routing{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, pull)
	if e != nil {
		return Routing{}, e
	}
	now := s.now().UTC()
	found := false
	for i := range x.Assignments {
		a := &x.Assignments[i]
		if a.ID != id {
			continue
		}
		found = true
		if !maintainer && actor != a.ParticipantID {
			return Routing{}, ErrInvalid
		}
		if maintainer && state != "released" && state != "revoked" {
			return Routing{}, ErrInvalid
		}
		if !maintainer && state != "accepted" && state != "declined" && state != "unavailable" && state != "overloaded" && state != "recused" {
			return Routing{}, ErrInvalid
		}
		a.State = state
		a.OutcomeReason = reason
		a.UpdatedAt = now
		x.Events = append(x.Events, Event{id, actor, state, reason, now})
	}
	if !found {
		return Routing{}, ErrNotFound
	}
	derive(&x)
	return x, s.write(x)
}
func derive(x *Routing) {
	m := map[string]bool{}
	for _, a := range x.Assignments {
		if map[string]bool{"declined": true, "unavailable": true, "overloaded": true, "recused": true, "released": true, "revoked": true}[a.State] {
			m[a.AreaID] = true
		}
		if a.State == "accepted" {
			delete(m, a.AreaID)
		}
	}
	x.ReassignmentAreas = nil
	for a := range m {
		x.ReassignmentAreas = append(x.ReassignmentAreas, a)
	}
	sort.Strings(x.ReassignmentAreas)
}
