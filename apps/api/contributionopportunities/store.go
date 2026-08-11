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
	Opportunities []Opportunity `json:"opportunities"`
	Profiles      []Profile     `json:"profiles"`
	Claims        []Claim       `json:"claims"`
	Reports       []Report      `json:"reports"`
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
