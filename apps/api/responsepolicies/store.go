// Package responsepolicies owns immutable, pre-alert response coverage contracts.
package responsepolicies

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

var ErrNotFound = errors.New("response policy not found")
var ErrInvalid = errors.New("invalid response policy")
var ErrConflict = errors.New("response policy version conflict")

type Resource struct {
	Kind         string   `json:"kind"`
	ID           string   `json:"id"`
	OwnerTeamIDs []string `json:"owner_team_ids"`
	Required     bool     `json:"required"`
}
type Team struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	MemberIDs []string `json:"member_ids"`
	Skills    []string `json:"skills"`
	Available bool     `json:"available"`
	Authority []string `json:"authority"`
}
type Target struct {
	AcknowledgeMinutes int `json:"acknowledge_minutes"`
	EngageMinutes      int `json:"engage_minutes"`
	UpdateMinutes      int `json:"update_minutes"`
}
type Escalation struct {
	AfterMinutes int      `json:"after_minutes"`
	TeamID       string   `json:"team_id"`
	AudienceIDs  []string `json:"audience_ids"`
	Action       string   `json:"action"`
}
type Coverage struct {
	ID                       string       `json:"id"`
	ResourceKind             string       `json:"resource_kind"`
	ResourceID               string       `json:"resource_id"`
	SignalClass              string       `json:"signal_class"`
	Severity                 string       `json:"severity"`
	TeamID                   string       `json:"team_id"`
	RequiredSkills           []string     `json:"required_skills"`
	Target                   Target       `json:"response_target"`
	Escalations              []Escalation `json:"escalation_path"`
	CommunicationAudienceIDs []string     `json:"communication_audience_ids"`
	ExpectedActions          []string     `json:"expected_actions"`
	IncidentCriteria         []string     `json:"incident_criteria"`
}
type RuleReference struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	Required   bool   `json:"required"`
	Accessible bool   `json:"accessible"`
	OwnerID    string `json:"owner_id"`
}
type Exception struct {
	ID         string    `json:"id"`
	CoverageID string    `json:"coverage_id"`
	Rationale  string    `json:"rationale"`
	OwnerID    string    `json:"owner_id"`
	ApprovedBy string    `json:"approved_by"`
	ExpiresAt  time.Time `json:"expires_at"`
}
type Input struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Resources      []Resource      `json:"resources"`
	Teams          []Team          `json:"teams"`
	Coverage       []Coverage      `json:"coverage"`
	RuleReferences []RuleReference `json:"rule_references"`
	Exceptions     []Exception     `json:"exceptions"`
	OwnerIDs       []string        `json:"owner_ids"`
	ChangeReason   string          `json:"change_reason"`
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
type Policy struct {
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func listOK(xs []string, required bool) bool {
	if required && len(xs) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func oneOf(x string, xs ...string) bool {
	for _, v := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Description) == "" || strings.TrimSpace(in.ChangeReason) == "" || !listOK(in.OwnerIDs, true) || len(in.Resources) == 0 || len(in.Teams) == 0 || len(in.Coverage) == 0 {
		return false
	}
	resources := map[string]bool{}
	for _, r := range in.Resources {
		if !oneOf(r.Kind, "repository", "service", "environment", "user_journey", "dependency") || r.ID == "" || !listOK(r.OwnerTeamIDs, false) {
			return false
		}
		resources[r.Kind+"\x00"+r.ID] = true
	}
	teams := map[string]bool{}
	for _, t := range in.Teams {
		if t.ID == "" || t.Name == "" || teams[t.ID] || !listOK(t.MemberIDs, false) || !listOK(t.Skills, false) || !listOK(t.Authority, false) {
			return false
		}
		teams[t.ID] = true
	}
	covers := map[string]bool{}
	for _, c := range in.Coverage {
		if c.ID == "" || covers[c.ID] || !resources[c.ResourceKind+"\x00"+c.ResourceID] || c.SignalClass == "" || !oneOf(c.Severity, "critical", "high", "medium", "low") || !teams[c.TeamID] || !listOK(c.RequiredSkills, true) || c.Target.AcknowledgeMinutes < 1 || c.Target.EngageMinutes < c.Target.AcknowledgeMinutes || c.Target.UpdateMinutes < 1 || !listOK(c.CommunicationAudienceIDs, true) || !listOK(c.ExpectedActions, true) || !listOK(c.IncidentCriteria, true) {
			return false
		}
		covers[c.ID] = true
		prior := 0
		for _, e := range c.Escalations {
			if e.AfterMinutes <= prior || !teams[e.TeamID] || e.Action == "" || !listOK(e.AudienceIDs, true) {
				return false
			}
			prior = e.AfterMinutes
		}
	}
	for _, r := range in.RuleReferences {
		if !oneOf(r.Kind, "organization_membership", "service_ownership", "access", "privacy", "security", "continuity") || r.ResourceID == "" || r.Revision == "" || r.OwnerID == "" {
			return false
		}
	}
	seen := map[string]bool{}
	for _, e := range in.Exceptions {
		if e.ID == "" || seen[e.ID] || !covers[e.CoverageID] || e.Rationale == "" || e.OwnerID == "" || e.ApprovedBy == "" || e.ExpiresAt.IsZero() {
			return false
		}
		seen[e.ID] = true
	}
	return true
}
func derive(v Version, now time.Time) []Gap {
	out := []Gap{}
	add := func(k, s, d, a string) { out = append(out, Gap{k, s, d, a}) }
	teams := map[string]Team{}
	for _, t := range v.Teams {
		teams[t.ID] = t
	}
	covered := map[string]bool{}
	for _, c := range v.Coverage {
		key := c.ResourceKind + "\x00" + c.ResourceID
		covered[key] = true
		t := teams[c.TeamID]
		if !t.Available || len(t.MemberIDs) == 0 {
			add("unavailable_team", c.ID, "accountable team has no available members", c.TeamID)
		}
		have := map[string]bool{}
		for _, s := range t.Skills {
			have[s] = true
		}
		for _, s := range c.RequiredSkills {
			if !have[s] {
				add("unavailable_skill", c.ID+":"+s, "accountable team does not declare the required skill", c.TeamID)
			}
		}
		if c.Target.EngageMinutes < c.Target.AcknowledgeMinutes || (!t.Available && c.Target.AcknowledgeMinutes < 30) {
			add("impossible_target", c.ID, "response target cannot be met by the declared coverage", c.TeamID)
		}
	}
	for _, r := range v.Resources {
		key := r.Kind + "\x00" + r.ID
		if r.Required && !covered[key] {
			add("uncovered_resource", r.Kind+":"+r.ID, "required resource has no signal coverage", v.AuthorID)
		}
		if len(r.OwnerTeamIDs) > 1 {
			add("conflicting_ownership", r.Kind+":"+r.ID, "resource declares multiple accountable owner teams", strings.Join(r.OwnerTeamIDs, ","))
		}
	}
	for _, r := range v.RuleReferences {
		if r.Required && !r.Accessible {
			add("inaccessible_rule", r.Kind+":"+r.ResourceID, "required governing rule cannot be inspected", r.OwnerID)
		}
	}
	for _, e := range v.Exceptions {
		if !e.ExpiresAt.After(now) {
			add("expired_exception", e.ID, e.Rationale, e.OwnerID)
		} else if e.ExpiresAt.Before(now.Add(30 * 24 * time.Hour)) {
			add("expiring_exception", e.ID, e.ExpiresAt.UTC().Format(time.RFC3339), e.OwnerID)
		}
	}
	return out
}
func newID() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(p Policy) error {
	path := s.path(p.RepositoryID, p.ID)
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(p, "", "  ")
	if e == nil {
		e = os.WriteFile(path, append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Policy, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := Version{1, in, actor, s.now().UTC()}
	p := Policy{newID(), repo, 1, []Version{v}, derive(v, s.now().UTC()), []string{"Response policies grant no repository, team, secret, communication, incident, deployment, environment, security, privacy, continuity, or operational authority."}}
	return p, s.save(p)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in Input) (Policy, error) {
	if actor == "" || !valid(in) {
		return Policy{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, e := s.read(repo, id)
	if e != nil {
		return p, e
	}
	if p.CurrentVersion != expected {
		return p, ErrConflict
	}
	p.CurrentVersion++
	v := Version{p.CurrentVersion, in, actor, s.now().UTC()}
	p.Versions = append(p.Versions, v)
	p.Gaps = derive(v, s.now().UTC())
	return p, s.save(p)
}
func (s *Store) read(repo, id string) (Policy, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Policy{}, ErrNotFound
	}
	var p Policy
	if e == nil {
		e = json.Unmarshal(b, &p)
	}
	return p, e
}
func (s *Store) Get(repo, id string) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	sort.Strings(files)
	out := []Policy{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var p Policy
		if x == nil {
			x = json.Unmarshal(b, &p)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, p)
	}
	return out, nil
}
