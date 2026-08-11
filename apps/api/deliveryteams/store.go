package deliveryteams

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

var ErrNotFound = errors.New("delivery team not found")
var ErrInvalid = errors.New("invalid delivery team")
var ErrConflict = errors.New("delivery team changed")
var ErrForbidden = errors.New("delivery team action forbidden")

type Outcome struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}
type Budget struct {
	Hours     int `json:"hours"`
	CostUnits int `json:"cost_units"`
	AgentRuns int `json:"agent_runs"`
}
type AccessPreview struct {
	Actions         []string   `json:"actions"`
	Sources         []string   `json:"sources"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	Missing         []string   `json:"missing,omitempty"`
	GrantsAuthority bool       `json:"grants_authority"`
}
type Participant struct {
	ID               string        `json:"id"`
	Kind             string        `json:"kind"`
	PrincipalID      string        `json:"principal_id"`
	Role             string        `json:"role"`
	Why              string        `json:"why"`
	Responsibilities []string      `json:"responsibilities"`
	Budget           Budget        `json:"budget"`
	Deadline         *time.Time    `json:"deadline,omitempty"`
	EscalatesToID    string        `json:"escalates_to_id,omitempty"`
	RequestedActions []string      `json:"requested_actions"`
	Access           AccessPreview `json:"effective_access"`
	State            string        `json:"state"`
	InvitedByID      string        `json:"invited_by_id"`
	InvitedAt        time.Time     `json:"invited_at"`
	RespondedByID    string        `json:"responded_by_id,omitempty"`
	RespondedAt      *time.Time    `json:"responded_at,omitempty"`
	RemovedByID      string        `json:"removed_by_id,omitempty"`
	RemovedAt        *time.Time    `json:"removed_at,omitempty"`
}
type Charter struct {
	Version             int64      `json:"version"`
	Outcome             string     `json:"outcome"`
	SuccessMeasures     []string   `json:"success_measures"`
	OperatingPrinciples []string   `json:"operating_principles"`
	TotalBudget         Budget     `json:"total_budget"`
	Deadline            *time.Time `json:"deadline,omitempty"`
	DefaultEscalation   string     `json:"default_escalation"`
	ChangedByID         string     `json:"changed_by_id"`
	ChangeReason        string     `json:"change_reason,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}
type Event struct {
	Sequence      int64     `json:"sequence"`
	Type          string    `json:"type"`
	ActorID       string    `json:"actor_id"`
	ParticipantID string    `json:"participant_id,omitempty"`
	Detail        string    `json:"detail,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
type Team struct {
	ID             string          `json:"id"`
	RepositoryID   string          `json:"repository_id"`
	Name           string          `json:"name"`
	Source         Outcome         `json:"source"`
	OrganizerID    string          `json:"organizer_id"`
	State          string          `json:"state"`
	Version        int64           `json:"version"`
	Charter        Charter         `json:"charter"`
	CharterHistory []Charter       `json:"charter_history"`
	Participants   []Participant   `json:"participants"`
	Events         []Event         `json:"events"`
	Plan           *Plan           `json:"plan,omitempty"`
	Executions     []Execution     `json:"executions"`
	Timeline       []TimelineEntry `json:"timeline"`
	Handoffs       []Handoff       `json:"handoffs"`
	StreamRuns     []StreamRun     `json:"stream_runs"`
	Controls       []Control       `json:"controls"`
	Runtime        RuntimeView     `json:"runtime"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
type CharterInput struct {
	Outcome             string     `json:"outcome"`
	SuccessMeasures     []string   `json:"success_measures"`
	OperatingPrinciples []string   `json:"operating_principles"`
	TotalBudget         Budget     `json:"total_budget"`
	Deadline            *time.Time `json:"deadline"`
	DefaultEscalation   string     `json:"default_escalation"`
	ChangeReason        string     `json:"change_reason"`
}
type ParticipantInput struct {
	Kind             string        `json:"kind"`
	PrincipalID      string        `json:"principal_id"`
	Role             string        `json:"role"`
	Why              string        `json:"why"`
	Responsibilities []string      `json:"responsibilities"`
	Budget           Budget        `json:"budget"`
	Deadline         *time.Time    `json:"deadline"`
	EscalatesToID    string        `json:"escalates_to_id"`
	RequestedActions []string      `json:"requested_actions"`
	Access           AccessPreview `json:"-"`
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
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(p, 0750); e != nil {
		return nil, e
	}
	return &Store{root: p, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func clean(xs []string, max int) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || len(x) > max {
			return nil, false
		}
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out, true
}
func validBudget(b Budget) bool {
	return b.Hours >= 0 && b.Hours <= 100000 && b.CostUnits >= 0 && b.CostUnits <= 100000000 && b.AgentRuns >= 0 && b.AgentRuns <= 100000
}
func charter(in CharterInput, actor string, v int64, now time.Time) (Charter, error) {
	in.Outcome = strings.TrimSpace(in.Outcome)
	in.DefaultEscalation = strings.TrimSpace(in.DefaultEscalation)
	measures, ok := clean(in.SuccessMeasures, 1000)
	if !ok || len(measures) == 0 {
		return Charter{}, ErrInvalid
	}
	principles, ok := clean(in.OperatingPrinciples, 1000)
	if !ok || len(principles) == 0 || in.Outcome == "" || len(in.Outcome) > 4000 || in.DefaultEscalation == "" || !validBudget(in.TotalBudget) {
		return Charter{}, ErrInvalid
	}
	if in.Deadline != nil && !in.Deadline.After(now) {
		return Charter{}, ErrInvalid
	}
	return Charter{Version: v, Outcome: in.Outcome, SuccessMeasures: measures, OperatingPrinciples: principles, TotalBudget: in.TotalBudget, Deadline: in.Deadline, DefaultEscalation: in.DefaultEscalation, ChangedByID: actor, ChangeReason: strings.TrimSpace(in.ChangeReason), CreatedAt: now}, nil
}
func (s *Store) path(repo, team string) string { return filepath.Join(s.root, repo, team+".json") }
func (s *Store) read(repo, team string) (Team, error) {
	b, e := os.ReadFile(s.path(repo, team))
	if e != nil {
		return Team{}, ErrNotFound
	}
	var v Team
	if json.Unmarshal(b, &v) != nil {
		return Team{}, ErrInvalid
	}
	if v.Plan != nil {
		v.Plan.Current.Blockers = deriveBlockers(v, v.Plan.Current.Streams)
		if len(v.Plan.Current.Blockers) > 0 && v.Plan.Current.Status == "accepted" {
			v.Plan.Current.Status = "blocked"
		}
	}
	v.Runtime = deriveRuntime(v, s.now().UTC())
	return v, nil
}
func (s *Store) write(v Team) error {
	p := s.path(v.RepositoryID, v.ID)
	if e := os.MkdirAll(filepath.Dir(p), 0750); e != nil {
		return e
	}
	// Runtime is a clock-sensitive projection, not accepted source data.
	v.Runtime = RuntimeView{}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(p, b, 0640)
}
func addEvent(v *Team, typ, actor, participant, detail string, now time.Time) {
	v.Events = append(v.Events, Event{Sequence: int64(len(v.Events) + 1), Type: typ, ActorID: actor, ParticipantID: participant, Detail: detail, CreatedAt: now})
	v.Version++
	v.UpdatedAt = now
}
func (s *Store) Create(repo, name, actor string, source Outcome, in CharterInput) (Team, error) {
	name = strings.TrimSpace(name)
	source.Kind = strings.TrimSpace(source.Kind)
	source.ID = strings.TrimSpace(source.ID)
	source.Title = strings.TrimSpace(source.Title)
	validSource := source.Kind == "proposal" || source.Kind == "initiative" || source.Kind == "decision" || source.Kind == "incident_follow_up" || source.Kind == "planned_outcome"
	if name == "" || len(name) > 200 || !validSource || source.Title == "" || (source.Kind != "planned_outcome" && source.ID == "") {
		return Team{}, ErrInvalid
	}
	now := s.now().UTC()
	c, e := charter(in, actor, 1, now)
	if e != nil {
		return Team{}, e
	}
	v := Team{ID: id(), RepositoryID: repo, Name: name, Source: source, OrganizerID: actor, State: "forming", Version: 1, Charter: c, CharterHistory: []Charter{c}, Participants: []Participant{}, Events: []Event{{Sequence: 1, Type: "team.created", ActorID: actor, Detail: source.Kind + ":" + source.ID, CreatedAt: now}}, Executions: []Execution{}, Timeline: []TimelineEntry{}, Handoffs: []Handoff{}, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	defer s.mu.Unlock()
	return v, s.write(v)
}
func (s *Store) Get(repo, team string) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, team)
}
func (s *Store) List(repo string) ([]Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Team{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Team{}
	for _, x := range entries {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func active(v Team, actor string) bool {
	if actor == v.OrganizerID {
		return true
	}
	for _, p := range v.Participants {
		if p.Kind == "human" && p.PrincipalID == actor && p.State == "accepted" {
			return true
		}
	}
	return false
}
func check(v Team, actor string, version int64) error {
	if version != v.Version {
		return ErrConflict
	}
	if !active(v, actor) {
		return ErrForbidden
	}
	return nil
}
func (s *Store) Revise(repo, team, actor string, version int64, in CharterInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if e = check(v, actor, version); e != nil {
		return v, e
	}
	c, e := charter(in, actor, v.Charter.Version+1, s.now().UTC())
	if e != nil {
		return v, e
	}
	v.Charter = c
	v.CharterHistory = append(v.CharterHistory, c)
	addEvent(&v, "charter.revised", actor, "", c.ChangeReason, c.CreatedAt)
	return v, s.write(v)
}
func (s *Store) Invite(repo, team, actor string, version int64, in ParticipantInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if e = check(v, actor, version); e != nil {
		return v, e
	}
	in.Role = strings.TrimSpace(in.Role)
	in.Why = strings.TrimSpace(in.Why)
	resp, ok := clean(in.Responsibilities, 1000)
	if !ok || len(resp) == 0 || in.PrincipalID == "" || in.Role == "" || in.Why == "" || !validBudget(in.Budget) || (in.Kind != "human" && in.Kind != "agent") {
		return v, ErrInvalid
	}
	actions, ok := clean(in.RequestedActions, 100)
	if !ok {
		return v, ErrInvalid
	}
	if in.Budget.Hours > v.Charter.TotalBudget.Hours || in.Budget.CostUnits > v.Charter.TotalBudget.CostUnits || in.Budget.AgentRuns > v.Charter.TotalBudget.AgentRuns || (in.Deadline != nil && v.Charter.Deadline != nil && in.Deadline.After(*v.Charter.Deadline)) {
		return v, ErrInvalid
	}
	for _, p := range v.Participants {
		if p.PrincipalID == in.PrincipalID && (p.State == "invited" || p.State == "accepted") {
			return v, ErrConflict
		}
		if p.Role == in.Role && (p.State == "invited" || p.State == "accepted") {
			return v, ErrConflict
		}
	}
	now := s.now().UTC()
	p := Participant{ID: id(), Kind: in.Kind, PrincipalID: in.PrincipalID, Role: in.Role, Why: in.Why, Responsibilities: resp, Budget: in.Budget, Deadline: in.Deadline, EscalatesToID: in.EscalatesToID, RequestedActions: actions, Access: in.Access, State: "invited", InvitedByID: actor, InvitedAt: now}
	v.Participants = append(v.Participants, p)
	addEvent(&v, "participant.invited", actor, p.ID, in.Role, now)
	return v, s.write(v)
}
func (s *Store) Respond(repo, team, participant, actor, response string, version int64) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if version != v.Version {
		return v, ErrConflict
	}
	for i := range v.Participants {
		p := &v.Participants[i]
		if p.ID == participant {
			if p.State != "invited" || (response != "accepted" && response != "declined") {
				return v, ErrForbidden
			}
			now := s.now().UTC()
			p.State = response
			p.RespondedByID = actor
			p.RespondedAt = &now
			addEvent(&v, "participant."+response, actor, p.ID, "", now)
			return v, s.write(v)
		}
	}
	return v, ErrNotFound
}
func (s *Store) Remove(repo, team, participant, actor, reason string, version int64) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, team)
	if e != nil {
		return v, e
	}
	if e = check(v, actor, version); e != nil {
		return v, e
	}
	for i := range v.Participants {
		p := &v.Participants[i]
		if p.ID == participant && p.State != "removed" {
			now := s.now().UTC()
			p.State = "removed"
			p.RemovedByID = actor
			p.RemovedAt = &now
			addEvent(&v, "participant.removed", actor, p.ID, strings.TrimSpace(reason), now)
			return v, s.write(v)
		}
	}
	return v, ErrNotFound
}
func (s *Store) Replace(repo, team, participant, actor string, version int64, in ParticipantInput) (Team, error) {
	v, e := s.Remove(repo, team, participant, actor, "replaced", version)
	if e != nil {
		return v, e
	}
	return s.Invite(repo, team, actor, v.Version, in)
}
