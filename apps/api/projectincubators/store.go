// Package projectincubators owns pre-repository project collaboration records.
package projectincubators

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

var ErrNotFound = errors.New("project incubator not found")
var ErrInvalid = errors.New("invalid project incubator")
var ErrForbidden = errors.New("project incubator action forbidden")

type Source struct {
	Kind           string `json:"kind"`
	RepositoryID   string `json:"repository_id,omitempty"`
	OrganizationID string `json:"organization_id,omitempty"`
	ResourceID     string `json:"resource_id,omitempty"`
	Label          string `json:"label,omitempty"`
	Status         string `json:"status"`
	Detail         string `json:"detail,omitempty"`
}
type Evidence struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Reference  string    `json:"reference"`
	Summary    string    `json:"summary"`
	Visibility string    `json:"visibility"`
	AddedByID  string    `json:"added_by_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Assumption struct {
	ID          string     `json:"id"`
	Statement   string     `json:"statement"`
	Status      string     `json:"status"`
	AddedByID   string     `json:"added_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedByID string     `json:"updated_by_id,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}
type CapabilityLink struct {
	Kind       string `json:"kind"`
	ScopeID    string `json:"scope_id,omitempty"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision,omitempty"`
	Visibility string `json:"visibility"`
}
type Alternative struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	ProductBoundary string           `json:"product_boundary"`
	Architecture    string           `json:"architecture"`
	Interfaces      []string         `json:"interfaces"`
	Dependencies    []string         `json:"dependencies"`
	Licenses        []string         `json:"licenses"`
	OperatingCosts  []string         `json:"operating_costs"`
	SecurityRisks   []string         `json:"security_risks"`
	DataRisks       []string         `json:"data_risks"`
	BuildOrAdopt    string           `json:"build_or_adopt"`
	Unknowns        []string         `json:"unknowns"`
	CapabilityLinks []CapabilityLink `json:"capability_links"`
	SupersedesID    string           `json:"supersedes_id,omitempty"`
	Status          string           `json:"status"`
	CreatedByID     string           `json:"created_by_id"`
	CreatedAt       time.Time        `json:"created_at"`
}
type Finding struct {
	ID            string     `json:"id"`
	AlternativeID string     `json:"alternative_id"`
	Kind          string     `json:"kind"`
	Claim         string     `json:"claim"`
	Evidence      []Evidence `json:"evidence"`
	AuthorID      string     `json:"author_id"`
	CreatedAt     time.Time  `json:"created_at"`
}
type Experiment struct {
	ID               string              `json:"id"`
	AlternativeID    string              `json:"alternative_id"`
	Question         string              `json:"question"`
	Method           []string            `json:"method"`
	Inputs           []string            `json:"inputs"`
	Commands         []string            `json:"commands"`
	SuccessCriteria  []string            `json:"success_criteria"`
	Budget           string              `json:"budget"`
	SafetyBoundary   string              `json:"safety_boundary"`
	CreatedByID      string              `json:"created_by_id"`
	CreatedAt        time.Time           `json:"created_at"`
	Attempts         []ExperimentAttempt `json:"attempts"`
	AuthorityGranted bool                `json:"authority_granted"`
}
type ExperimentAttempt struct {
	ID               string            `json:"id"`
	InputDigest      string            `json:"input_digest"`
	Measurements     map[string]string `json:"measurements"`
	Artifacts        []string          `json:"artifacts"`
	Outcome          string            `json:"outcome"`
	Notes            string            `json:"notes"`
	ReproductionOfID string            `json:"reproduction_of_id,omitempty"`
	ActorID          string            `json:"actor_id"`
	CreatedAt        time.Time         `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type ScopeChange struct {
	ID        string    `json:"id"`
	Rationale string    `json:"rationale"`
	Before    Input     `json:"before"`
	After     Input     `json:"after"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Participant struct {
	ID                  string     `json:"id"`
	Kind                string     `json:"kind"`
	UserID              string     `json:"user_id,omitempty"`
	AgentIdentity       string     `json:"agent_identity,omitempty"`
	OnboardingScopeKind string     `json:"onboarding_scope_kind,omitempty"`
	OnboardingScopeID   string     `json:"onboarding_scope_id,omitempty"`
	OnboardingID        string     `json:"onboarding_id,omitempty"`
	Role                string     `json:"role"`
	InvitedByID         string     `json:"invited_by_id"`
	InvitedAt           time.Time  `json:"invited_at"`
	Consent             string     `json:"consent"`
	RespondedAt         *time.Time `json:"responded_at,omitempty"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Input struct {
	Title           string   `json:"title"`
	Audience        string   `json:"audience"`
	Problem         string   `json:"problem"`
	DesiredOutcome  string   `json:"desired_outcome"`
	Constraints     []string `json:"constraints"`
	SuccessMeasures []string `json:"success_measures"`
	SponsorIDs      []string `json:"sponsor_ids"`
	DecisionRights  []string `json:"decision_rights"`
	Visibility      string   `json:"visibility"`
}
type Incubator struct {
	ID string `json:"id"`
	Input
	Source           Source        `json:"source"`
	CreatedByID      string        `json:"created_by_id"`
	Participants     []Participant `json:"participants"`
	Discussion       []Comment     `json:"discussion"`
	Evidence         []Evidence    `json:"evidence"`
	Assumptions      []Assumption  `json:"assumptions"`
	ScopeChanges     []ScopeChange `json:"scope_changes"`
	Alternatives     []Alternative `json:"alternatives"`
	Findings         []Finding     `json:"findings"`
	Experiments      []Experiment  `json:"experiments"`
	DuplicateIDs     []string      `json:"duplicate_incubator_ids"`
	History          []Event       `json:"history"`
	AuthorityGranted bool          `json:"authority_granted"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func cleanList(v []string, required bool) bool {
	if (required && len(v) == 0) || len(v) > 50 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" || len(x) > 2000 {
			return false
		}
	}
	return true
}
func valid(in Input) bool {
	return strings.TrimSpace(in.Title) != "" && len(in.Title) <= 200 && strings.TrimSpace(in.Audience) != "" && len(in.Audience) <= 4000 && strings.TrimSpace(in.Problem) != "" && len(in.Problem) <= 65536 && strings.TrimSpace(in.DesiredOutcome) != "" && len(in.DesiredOutcome) <= 65536 && cleanList(in.Constraints, false) && cleanList(in.SuccessMeasures, true) && cleanList(in.SponsorIDs, true) && cleanList(in.DecisionRights, true) && (in.Visibility == "public" || in.Visibility == "participants")
}
func validSource(v Source) bool {
	if !map[string]bool{"idea": true, "feedback": true, "support_gap": true, "governed_proposal": true}[v.Kind] {
		return false
	}
	if v.Kind == "idea" {
		return v.ResourceID == ""
	}
	return v.ResourceID != ""
}
func (s *Store) Create(actor string, in Input, source Source) (Incubator, error) {
	if actor == "" || !valid(in) || !validSource(source) {
		return Incubator{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	v := Incubator{ID: newID("inc_"), Input: in, Source: source, CreatedByID: actor, Participants: []Participant{}, Discussion: []Comment{}, Evidence: []Evidence{}, Assumptions: []Assumption{}, ScopeChanges: []ScopeChange{}, Alternatives: []Alternative{}, Findings: []Finding{}, Experiments: []Experiment{}, DuplicateIDs: []string{}, History: []Event{}, AuthorityGranted: false, CreatedAt: now, UpdatedAt: now}
	v.Participants = append(v.Participants, Participant{ID: newID("par_"), Kind: "human", UserID: actor, Role: "founder", InvitedByID: actor, InvitedAt: now, Consent: "accepted", RespondedAt: &now})
	v.event("incubator.opened", actor, "")
	all, _ := s.list()
	key := duplicateKey(in)
	for _, x := range all {
		if duplicateKey(x.Input) == key {
			v.DuplicateIDs = append(v.DuplicateIDs, x.ID)
			x.DuplicateIDs = appendUnique(x.DuplicateIDs, v.ID)
			_ = s.write(x)
		}
	}
	return v, s.write(v)
}
func validAlternative(a Alternative) bool {
	return strings.TrimSpace(a.Title) != "" && strings.TrimSpace(a.ProductBoundary) != "" && strings.TrimSpace(a.Architecture) != "" && strings.TrimSpace(a.BuildOrAdopt) != "" && cleanList(a.Interfaces, true) && cleanList(a.Dependencies, false) && cleanList(a.Licenses, true) && cleanList(a.OperatingCosts, true) && cleanList(a.SecurityRisks, true) && cleanList(a.DataRisks, true) && cleanList(a.Unknowns, false)
}
func (s *Store) AddAlternative(id, actor string, a Alternative) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if !validAlternative(a) {
			return ErrInvalid
		}
		if a.SupersedesID != "" {
			found := false
			for i := range v.Alternatives {
				if v.Alternatives[i].ID == a.SupersedesID {
					v.Alternatives[i].Status = "superseded"
					found = true
				}
			}
			if !found {
				return ErrInvalid
			}
		}
		for _, l := range a.CapabilityLinks {
			if !map[string]bool{"decision": true, "prototype": true, "package": true, "api": true, "code_intelligence": true}[l.Kind] || l.ResourceID == "" || !map[string]bool{"public": true, "organization": true}[l.Visibility] {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		a.ID = newID("alt_")
		a.Status = "active"
		a.CreatedByID = actor
		a.CreatedAt = now
		v.Alternatives = append(v.Alternatives, a)
		v.event("alternative.added", actor, a.ID)
		return nil
	})
}
func (s *Store) AddFinding(id, actor string, f Finding) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if !map[string]bool{"research": true, "measurement": true, "dissent": true, "unknown": true}[f.Kind] || strings.TrimSpace(f.Claim) == "" {
			return ErrInvalid
		}
		found := false
		for _, a := range v.Alternatives {
			found = found || a.ID == f.AlternativeID
		}
		if !found {
			return ErrInvalid
		}
		for i := range f.Evidence {
			e := &f.Evidence[i]
			if !map[string]bool{"public": true, "organization": true}[e.Visibility] || e.Reference == "" || e.Summary == "" {
				return ErrInvalid
			}
			e.ID = newID("evd_")
			e.AddedByID = actor
			e.CreatedAt = s.now().UTC()
		}
		f.ID = newID("fnd_")
		f.AuthorID = actor
		f.CreatedAt = s.now().UTC()
		v.Findings = append(v.Findings, f)
		v.event("finding.added", actor, f.ID)
		return nil
	})
}
func (s *Store) AddExperiment(id, actor string, x Experiment) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		found := false
		for _, a := range v.Alternatives {
			found = found || a.ID == x.AlternativeID
		}
		if !found || x.Question == "" || !cleanList(x.Method, true) || !cleanList(x.Inputs, true) || !cleanList(x.Commands, true) || !cleanList(x.SuccessCriteria, true) || x.Budget == "" || x.SafetyBoundary == "" {
			return ErrInvalid
		}
		x.ID = newID("exp_")
		x.CreatedByID = actor
		x.CreatedAt = s.now().UTC()
		x.Attempts = []ExperimentAttempt{}
		x.AuthorityGranted = false
		v.Experiments = append(v.Experiments, x)
		v.event("experiment.created", actor, x.ID)
		return nil
	})
}
func (s *Store) AddAttempt(id, experiment, actor string, a ExperimentAttempt) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if a.InputDigest == "" || len(a.Measurements) == 0 || !map[string]bool{"passed": true, "failed": true, "inconclusive": true, "blocked": true}[a.Outcome] {
			return ErrInvalid
		}
		for i := range v.Experiments {
			if v.Experiments[i].ID == experiment {
				if a.ReproductionOfID != "" {
					ok := false
					for _, old := range v.Experiments[i].Attempts {
						ok = ok || old.ID == a.ReproductionOfID
					}
					if !ok {
						return ErrInvalid
					}
				}
				a.ID = newID("att_")
				a.ActorID = actor
				a.CreatedAt = s.now().UTC()
				v.Experiments[i].Attempts = append(v.Experiments[i].Attempts, a)
				v.event("experiment.attempted", actor, a.ID)
				return nil
			}
		}
		return ErrNotFound
	})
}
func duplicateKey(in Input) string {
	return strings.ToLower(strings.Join(strings.Fields(in.Audience+" "+in.Problem), " "))
}
func appendUnique(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}
func (s *Store) List(actor string) ([]Incubator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return nil, e
	}
	out := []Incubator{}
	for _, v := range all {
		if canRead(v, actor) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *Store) Get(id, actor string) (Incubator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e == nil && !canRead(v, actor) {
		return Incubator{}, ErrNotFound
	}
	return v, e
}
func canRead(v Incubator, actor string) bool {
	if v.Visibility == "public" {
		return true
	}
	for _, p := range v.Participants {
		if p.Kind == "human" && p.UserID == actor && p.Consent != "declined" {
			return true
		}
		if p.Kind == "agent" && p.AgentIdentity == actor && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func canShape(v Incubator, actor string) bool {
	for _, p := range v.Participants {
		if p.Kind == "human" && p.UserID == actor && p.Consent == "accepted" {
			return true
		}
		if p.Kind == "agent" && p.AgentIdentity == actor && p.Consent == "accepted" {
			return true
		}
	}
	return false
}
func (s *Store) Invite(id, actor string, p Participant) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if p.Role == "" || (p.Kind == "human" && p.UserID == "") || (p.Kind == "agent" && (p.AgentIdentity == "" || p.OnboardingID == "" || p.OnboardingScopeKind == "" || p.OnboardingScopeID == "")) || (p.Kind != "human" && p.Kind != "agent") {
			return ErrInvalid
		}
		for _, x := range v.Participants {
			if (p.Kind == "human" && x.UserID == p.UserID) || (p.Kind == "agent" && x.AgentIdentity == p.AgentIdentity) {
				return ErrInvalid
			}
		}
		now := s.now().UTC()
		p.ID = newID("par_")
		p.InvitedByID = actor
		p.InvitedAt = now
		p.Consent = "pending"
		if p.Kind == "agent" {
			p.Consent = "accepted"
			p.RespondedAt = &now
		}
		v.Participants = append(v.Participants, p)
		v.event("participant.invited", actor, p.ID)
		return nil
	})
}
func (s *Store) Consent(id, participant, actor, decision string) (Incubator, error) {
	return s.mutate(id, actor, false, func(v *Incubator) error {
		if decision != "accepted" && decision != "declined" {
			return ErrInvalid
		}
		for i := range v.Participants {
			p := &v.Participants[i]
			if p.ID == participant && p.Kind == "human" && p.UserID == actor && p.Consent == "pending" {
				now := s.now().UTC()
				p.Consent = decision
				p.RespondedAt = &now
				v.event("participant."+decision, actor, p.ID)
				return nil
			}
		}
		return ErrForbidden
	})
}
func (s *Store) Comment(id, actor, body string) (Incubator, error) {
	body = strings.TrimSpace(body)
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if body == "" || len(body) > 65536 {
			return ErrInvalid
		}
		now := s.now().UTC()
		v.Discussion = append(v.Discussion, Comment{ID: newID("com_"), Body: body, AuthorID: actor, CreatedAt: now})
		v.event("comment.added", actor, "")
		return nil
	})
}
func (s *Store) AddEvidence(id, actor string, e Evidence) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if !map[string]bool{"public": true, "participants": true}[e.Visibility] || strings.TrimSpace(e.Kind) == "" || strings.TrimSpace(e.Reference) == "" || strings.TrimSpace(e.Summary) == "" {
			return ErrInvalid
		}
		now := s.now().UTC()
		e.ID = newID("evd_")
		e.AddedByID = actor
		e.CreatedAt = now
		v.Evidence = append(v.Evidence, e)
		v.event("evidence.added", actor, e.ID)
		return nil
	})
}
func (s *Store) AddAssumption(id, actor, statement string) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if strings.TrimSpace(statement) == "" {
			return ErrInvalid
		}
		now := s.now().UTC()
		a := Assumption{ID: newID("asm_"), Statement: statement, Status: "open", AddedByID: actor, CreatedAt: now}
		v.Assumptions = append(v.Assumptions, a)
		v.event("assumption.added", actor, a.ID)
		return nil
	})
}
func (s *Store) ResolveAssumption(id, assumption, actor, status string) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if status != "confirmed" && status != "disproved" && status != "superseded" {
			return ErrInvalid
		}
		for i := range v.Assumptions {
			if v.Assumptions[i].ID == assumption {
				now := s.now().UTC()
				v.Assumptions[i].Status = status
				v.Assumptions[i].UpdatedByID = actor
				v.Assumptions[i].UpdatedAt = &now
				v.event("assumption."+status, actor, assumption)
				return nil
			}
		}
		return ErrNotFound
	})
}
func (s *Store) ChangeScope(id, actor, rationale string, in Input) (Incubator, error) {
	return s.mutate(id, actor, true, func(v *Incubator) error {
		if !valid(in) || strings.TrimSpace(rationale) == "" {
			return ErrInvalid
		}
		now := s.now().UTC()
		v.ScopeChanges = append(v.ScopeChanges, ScopeChange{ID: newID("scp_"), Rationale: rationale, Before: v.Input, After: in, ActorID: actor, CreatedAt: now})
		v.Input = in
		v.event("scope.changed", actor, "")
		return nil
	})
}
func (s *Store) mutate(id, actor string, shape bool, fn func(*Incubator) error) (Incubator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil {
		return v, e
	}
	if !canRead(v, actor) || (shape && !canShape(v, actor)) {
		return Incubator{}, ErrForbidden
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (v *Incubator) event(t, a, s string) {
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: t, ActorID: a, SubjectID: s, CreatedAt: time.Now().UTC()})
}
func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }
func (s *Store) write(v Incubator) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e == nil {
		e = os.Rename(tmp, s.path(v.ID))
	}
	return e
}
func (s *Store) read(id string) (Incubator, error) {
	var v Incubator
	b, e := os.ReadFile(s.path(id))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) list() ([]Incubator, error) {
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Incubator{}
	for _, x := range es {
		if filepath.Ext(x.Name()) == ".json" {
			v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
			if er != nil {
				return nil, er
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
