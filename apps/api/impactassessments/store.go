// Package impactassessments owns durable, revision-exact prospective change analysis.
package impactassessments

import (
	"crypto/rand"
	"crypto/sha256"
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

var ErrNotFound = errors.New("impact assessment not found")
var ErrConflict = errors.New("impact assessment conflict")

type Source struct {
	Kind            string `json:"kind"`
	Path            string `json:"path,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	InvestigationID string `json:"investigation_id,omitempty"`
	ConclusionID    string `json:"conclusion_id,omitempty"`
	Diff            string `json:"diff,omitempty"`
}
type Evidence struct {
	RepositoryID string `json:"repository_id"`
	CommitID     string `json:"commit_id"`
	Kind         string `json:"kind"`
	Path         string `json:"path,omitempty"`
	Line         int    `json:"line,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Label        string `json:"label"`
}
type Impact struct {
	ID               string            `json:"id"`
	Category         string            `json:"category"`
	Summary          string            `json:"summary"`
	RepositoryID     string            `json:"repository_id"`
	OwnerIDs         []string          `json:"owner_ids,omitempty"`
	Verification     []string          `json:"verification,omitempty"`
	Evidence         []Evidence        `json:"evidence"`
	State            string            `json:"state"`
	Rationale        string            `json:"rationale,omitempty"`
	Acknowledgements []Acknowledgement `json:"acknowledgements,omitempty"`
	CreatedByID      string            `json:"created_by_id"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}
type Acknowledgement struct {
	OwnerID       string     `json:"owner_id"`
	State         string     `json:"state"`
	Note          string     `json:"note,omitempty"`
	RequestedByID string     `json:"requested_by_id"`
	DecidedByID   string     `json:"decided_by_id,omitempty"`
	RequestedAt   time.Time  `json:"requested_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
}
type Finding struct {
	ID          string     `json:"id"`
	Agent       string     `json:"agent"`
	Body        string     `json:"body"`
	Uncertainty string     `json:"uncertainty,omitempty"`
	Evidence    []Evidence `json:"evidence"`
	CreatedAt   time.Time  `json:"created_at"`
}
type Assessment struct {
	ID              string    `json:"id"`
	RepositoryID    string    `json:"repository_id"`
	Title           string    `json:"title"`
	Revision        string    `json:"revision"`
	CommitID        string    `json:"commit_id"`
	CreatorID       string    `json:"creator_id"`
	Participants    []string  `json:"participants"`
	Sources         []Source  `json:"sources"`
	Impacts         []Impact  `json:"impacts"`
	Findings        []Finding `json:"agent_findings"`
	Unknowns        []string  `json:"unknowns"`
	AnalysisStatus  string    `json:"analysis_status"`
	AnalysisReasons []string  `json:"analysis_reasons,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Store struct {
	root   string
	mu     sync.Mutex
	now    func() time.Time
	tokens map[string]string
}
type tokenRecord struct {
	AssessmentID string    `json:"assessment_id"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now, tokens: map[string]string{}}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(v Assessment) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v.Title = strings.TrimSpace(v.Title)
	if v.RepositoryID == "" || v.CommitID == "" || v.CreatorID == "" || v.Title == "" || len(v.Title) > 200 || len(v.Sources) == 0 {
		return Assessment{}, ErrConflict
	}
	now := s.now().UTC()
	v.ID = id()
	v.Participants = []string{v.CreatorID}
	v.CreatedAt = now
	v.UpdatedAt = now
	for i := range v.Impacts {
		v.Impacts[i].ID = id()
		v.Impacts[i].CreatedByID = v.CreatorID
		v.Impacts[i].CreatedAt = now
		v.Impacts[i].UpdatedAt = now
		if v.Impacts[i].State == "" {
			v.Impacts[i].State = "open"
		}
	}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Assessment{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, z := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if z == nil && v.RepositoryID == repo {
			v.Impacts = nil
			v.Findings = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) Update(repo, aid, actor, impactID, state, rationale string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(aid)
	if e != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Assessment{}, ErrNotFound
	}
	if state != "open" && state != "accepted_risk" && state != "unknown" && state != "mitigated" {
		return Assessment{}, ErrConflict
	}
	for i := range v.Impacts {
		if v.Impacts[i].ID == impactID {
			v.Impacts[i].State = state
			v.Impacts[i].Rationale = strings.TrimSpace(rationale)
			v.Impacts[i].UpdatedAt = s.now().UTC()
			v.UpdatedAt = v.Impacts[i].UpdatedAt
			return v, s.write(v)
		}
	}
	return Assessment{}, ErrNotFound
}
func (s *Store) AddImpact(repo, aid, actor string, x Impact) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(aid)
	if e != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Assessment{}, ErrNotFound
	}
	x.Summary = strings.TrimSpace(x.Summary)
	if x.Summary == "" || !category(x.Category) {
		return Assessment{}, ErrConflict
	}
	now := s.now().UTC()
	x.ID = id()
	x.State = "open"
	x.CreatedByID = actor
	x.CreatedAt = now
	x.UpdatedAt = now
	v.Impacts = append(v.Impacts, x)
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Request(repo, aid, actor, impactID, owner string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(aid)
	if e != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Assessment{}, ErrNotFound
	}
	for i := range v.Impacts {
		if v.Impacts[i].ID != impactID {
			continue
		}
		if !has(v.Impacts[i].OwnerIDs, owner) {
			return Assessment{}, ErrConflict
		}
		for _, a := range v.Impacts[i].Acknowledgements {
			if a.OwnerID == owner && a.State == "requested" {
				return Assessment{}, ErrConflict
			}
		}
		now := s.now().UTC()
		v.Impacts[i].Acknowledgements = append(v.Impacts[i].Acknowledgements, Acknowledgement{OwnerID: owner, State: "requested", RequestedByID: actor, RequestedAt: now})
		if !has(v.Participants, owner) {
			v.Participants = append(v.Participants, owner)
		}
		v.UpdatedAt = now
		return v, s.write(v)
	}
	return Assessment{}, ErrNotFound
}
func (s *Store) Decide(repo, aid, actor, impactID, state, note string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(aid)
	if e != nil || v.RepositoryID != repo {
		return Assessment{}, ErrNotFound
	}
	if state != "acknowledged" && state != "concern" {
		return Assessment{}, ErrConflict
	}
	for i := range v.Impacts {
		if v.Impacts[i].ID != impactID {
			continue
		}
		for j := range v.Impacts[i].Acknowledgements {
			a := &v.Impacts[i].Acknowledgements[j]
			if a.OwnerID == actor && a.State == "requested" {
				now := s.now().UTC()
				a.State = state
				a.Note = strings.TrimSpace(note)
				a.DecidedByID = actor
				a.DecidedAt = &now
				v.UpdatedAt = now
				return v, s.write(v)
			}
		}
	}
	return Assessment{}, ErrNotFound
}
func (s *Store) StartAgent(repo, aid, actor string) (Assessment, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(aid)
	if e != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Assessment{}, "", ErrNotFound
	}
	raw := id() + id()
	sum := sha256.Sum256([]byte(raw))
	digest := hex.EncodeToString(sum[:])
	s.tokens[digest] = aid
	dir := filepath.Join(s.root, ".agent-tokens")
	if e := os.MkdirAll(dir, 0700); e != nil {
		return Assessment{}, "", e
	}
	b, _ := json.Marshal(tokenRecord{AssessmentID: aid, ExpiresAt: s.now().UTC().Add(24 * time.Hour)})
	if e := os.WriteFile(filepath.Join(dir, digest+".json"), b, 0600); e != nil {
		return Assessment{}, "", e
	}
	return v, raw, nil
}
func (s *Store) tokenAssessment(token string) string {
	sum := sha256.Sum256([]byte(token))
	digest := hex.EncodeToString(sum[:])
	if aid := s.tokens[digest]; aid != "" {
		return aid
	}
	b, e := os.ReadFile(filepath.Join(s.root, ".agent-tokens", digest+".json"))
	var record tokenRecord
	if e != nil || json.Unmarshal(b, &record) != nil || !s.now().UTC().Before(record.ExpiresAt) {
		return ""
	}
	s.tokens[digest] = record.AssessmentID
	return record.AssessmentID
}
func (s *Store) AgentContext(token string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	aid := s.tokenAssessment(token)
	if aid == "" {
		return Assessment{}, ErrNotFound
	}
	return s.read(aid)
}
func (s *Store) AddFinding(token, body, uncertainty string, evidence []Evidence) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	aid := s.tokenAssessment(token)
	v, e := s.read(aid)
	body = strings.TrimSpace(body)
	if e != nil || body == "" {
		return Assessment{}, ErrNotFound
	}
	for _, cited := range evidence {
		valid := false
		for _, impact := range v.Impacts {
			for _, retained := range impact.Evidence {
				if cited.RepositoryID == retained.RepositoryID && cited.CommitID == retained.CommitID && cited.Kind == retained.Kind && cited.Path == retained.Path && cited.ResourceID == retained.ResourceID {
					valid = true
				}
			}
		}
		if !valid {
			return Assessment{}, ErrConflict
		}
	}
	now := s.now().UTC()
	v.Findings = append(v.Findings, Finding{ID: id(), Agent: "codex", Body: body, Uncertainty: strings.TrimSpace(uncertainty), Evidence: evidence, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}
func category(v string) bool {
	switch v {
	case "reference", "test", "owner", "package", "interface", "consumer", "release", "environment":
		return true
	}
	return false
}
func has(v []string, x string) bool {
	for _, y := range v {
		if y == x {
			return true
		}
	}
	return false
}
func (s *Store) read(id string) (Assessment, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(e) {
		return Assessment{}, ErrNotFound
	}
	var v Assessment
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) write(v Assessment) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
}
