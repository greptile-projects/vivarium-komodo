// Package reliabilityinvestigations retains revision-bound, collaborative reliability diagnosis.
package reliabilityinvestigations

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

var ErrNotFound = errors.New("reliability investigation not found")
var ErrInvalid = errors.New("invalid reliability investigation")

type Trigger struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
}
type Evidence struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	ResourceID  string   `json:"resource_id"`
	Revision    string   `json:"revision"`
	JourneyIDs  []string `json:"journey_ids,omitempty"`
	Window      string   `json:"window,omitempty"`
	Summary     string   `json:"summary"`
	Audience    string   `json:"audience"`
	Baseline    bool     `json:"baseline"`
	Uncertainty string   `json:"uncertainty,omitempty"`
}
type Citation struct {
	EvidenceID string `json:"evidence_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Path       string `json:"path,omitempty"`
	Label      string `json:"label,omitempty"`
}
type Entry struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Body         string     `json:"body"`
	Citations    []Citation `json:"citations"`
	Uncertainty  string     `json:"uncertainty,omitempty"`
	Challenges   string     `json:"challenges,omitempty"`
	Verdict      string     `json:"verdict,omitempty"`
	ActorID      string     `json:"actor_id"`
	Stale        bool       `json:"stale"`
	StaleReasons []string   `json:"stale_reasons,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
type InputRequest struct {
	ID              string    `json:"id"`
	OwnerID         string    `json:"owner_id"`
	OwnerKind       string    `json:"owner_kind"`
	Question        string    `json:"question"`
	EvidenceNeeded  []string  `json:"evidence_needed"`
	Status          string    `json:"status"`
	ResponseEntryID string    `json:"response_entry_id,omitempty"`
	RequestedBy     string    `json:"requested_by"`
	CreatedAt       time.Time `json:"created_at"`
}
type Outcome struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	ResourceID        string    `json:"resource_id"`
	Rationale         string    `json:"rationale"`
	ConclusionEntryID string    `json:"conclusion_entry_id"`
	ActorID           string    `json:"actor_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Investigation struct {
	ID               string         `json:"id"`
	RepositoryID     string         `json:"repository_id"`
	ObjectiveID      string         `json:"objective_id"`
	ObjectiveVersion int64          `json:"objective_version"`
	Revision         string         `json:"revision"`
	Trigger          Trigger        `json:"trigger"`
	Title            string         `json:"title"`
	Question         string         `json:"question"`
	JourneyIDs       []string       `json:"journey_ids"`
	Participants     []string       `json:"participants"`
	Evidence         []Evidence     `json:"evidence"`
	Entries          []Entry        `json:"entries"`
	InputRequests    []InputRequest `json:"input_requests"`
	Outcomes         []Outcome      `json:"outcomes"`
	CreatorID        string         `json:"creator_id"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Blockers         []string       `json:"blockers"`
}
type CreateInput struct {
	ObjectiveID      string     `json:"objective_id"`
	ObjectiveVersion int64      `json:"objective_version"`
	Revision         string     `json:"revision"`
	Trigger          Trigger    `json:"trigger"`
	Title            string     `json:"title"`
	Question         string     `json:"question"`
	JourneyIDs       []string   `json:"journey_ids"`
	Evidence         []Evidence `json:"evidence"`
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
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}
func id() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func unique(xs []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) Create(repo, actor string, in CreateInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || in.ObjectiveID == "" || in.ObjectiveVersion < 1 || in.Revision == "" || !map[string]bool{"objective": true, "pull_request": true, "deployment": true, "budget_consumption": true}[in.Trigger.Kind] || in.Trigger.ResourceID == "" || in.Trigger.Revision == "" || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Question) == "" || len(in.JourneyIDs) == 0 || len(in.Evidence) < 2 {
		return Investigation{}, ErrInvalid
	}
	baseline, affected := false, false
	seen := map[string]bool{}
	for i := range in.Evidence {
		e := &in.Evidence[i]
		if e.Kind == "" || e.ResourceID == "" || e.Revision == "" || e.Summary == "" || !map[string]bool{"repository": true, "participants": true}[e.Audience] {
			return Investigation{}, ErrInvalid
		}
		e.ID = id()
		if seen[e.Kind+":"+e.ResourceID+":"+e.Revision] {
			return Investigation{}, ErrInvalid
		}
		seen[e.Kind+":"+e.ResourceID+":"+e.Revision] = true
		baseline = baseline || e.Baseline
		affected = affected || !e.Baseline
	}
	if !baseline || !affected {
		return Investigation{}, ErrInvalid
	}
	now := s.now().UTC()
	v := Investigation{ID: id(), RepositoryID: repo, ObjectiveID: in.ObjectiveID, ObjectiveVersion: in.ObjectiveVersion, Revision: in.Revision, Trigger: in.Trigger, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), JourneyIDs: unique(in.JourneyIDs), Participants: []string{actor}, Evidence: in.Evidence, CreatorID: actor, CreatedAt: now, UpdatedAt: now}
	v = derive(v)
	return v, s.write(v)
}
func (s *Store) Get(repo, x string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Investigation{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, e := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if e == nil && v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Invite(repo, x, actor, participant string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || participant == "" {
		return Investigation{}, ErrInvalid
	}
	v.Participants = unique(append(v.Participants, participant))
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (s *Store) Add(repo, x, actor string, in Entry) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || !map[string]bool{"observation": true, "hypothesis": true, "comparison": true, "challenge": true, "conclusion": true, "response": true}[in.Kind] || strings.TrimSpace(in.Body) == "" || len(in.Citations) == 0 {
		return Investigation{}, ErrInvalid
	}
	for _, c := range in.Citations {
		if c.EvidenceID == "" && c.ResourceID == "" {
			return Investigation{}, ErrInvalid
		}
		if c.EvidenceID != "" && !evidence(v, c.EvidenceID) {
			return Investigation{}, ErrInvalid
		}
	}
	if in.Challenges != "" && !entry(v, in.Challenges) {
		return Investigation{}, ErrInvalid
	}
	if in.Kind == "conclusion" && !map[string]bool{"supported": true, "disputed": true, "inconclusive": true}[in.Verdict] {
		return Investigation{}, ErrInvalid
	}
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	in.Stale = false
	in.StaleReasons = nil
	v.Entries = append(v.Entries, in)
	v.UpdatedAt = in.CreatedAt
	v = derive(v)
	return v, s.write(v)
}
func (s *Store) Request(repo, x, actor string, in InputRequest) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || in.OwnerID == "" || !map[string]bool{"service": true, "dependency": true}[in.OwnerKind] || in.Question == "" || len(in.EvidenceNeeded) == 0 {
		return Investigation{}, ErrInvalid
	}
	in.ID = id()
	in.Status = "open"
	in.RequestedBy = actor
	in.CreatedAt = s.now().UTC()
	v.InputRequests = append(v.InputRequests, in)
	v.UpdatedAt = in.CreatedAt
	v = derive(v)
	return v, s.write(v)
}
func (s *Store) AddOutcome(repo, x, actor string, in Outcome) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(x)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || !map[string]bool{"issue": true, "incident": true, "decision": true, "planned_improvement": true}[in.Kind] || in.ResourceID == "" || in.Rationale == "" || !entry(v, in.ConclusionEntryID) {
		return Investigation{}, ErrInvalid
	}
	in.ID = id()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	v.Outcomes = append(v.Outcomes, in)
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}
func Resolve(v Investigation, currentObjective int64, currentRevision string) Investigation {
	for i := range v.Evidence {
		stale := currentObjective != v.ObjectiveVersion || (v.Evidence[i].Kind == "code" && v.Evidence[i].Revision != currentRevision)
		if stale {
			for j := range v.Entries {
				for _, c := range v.Entries[j].Citations {
					if c.EvidenceID == v.Evidence[i].ID {
						v.Entries[j].Stale = true
						v.Entries[j].StaleReasons = unique(append(v.Entries[j].StaleReasons, "source_changed"))
					}
				}
			}
		}
	}
	if currentObjective != v.ObjectiveVersion {
		v.Blockers = unique(append(v.Blockers, "objective_version_changed"))
	}
	return derive(v)
}
func derive(v Investigation) Investigation {
	b := []string{}
	for _, e := range v.Evidence {
		if e.Uncertainty != "" {
			b = append(b, "uncertain_evidence")
		}
	}
	for _, e := range v.Entries {
		if e.Kind == "challenge" {
			b = append(b, "disputed_conclusion")
		}
		if e.Kind == "conclusion" && e.Verdict == "inconclusive" {
			b = append(b, "inconclusive_signals")
		}
		if e.Stale {
			b = append(b, "stale_evidence")
		}
	}
	for _, r := range v.InputRequests {
		if r.Status == "open" && r.OwnerKind == "dependency" {
			b = append(b, "dependency_input_pending")
		}
	}
	v.Blockers = unique(append(v.Blockers, b...))
	return v
}
func evidence(v Investigation, x string) bool {
	for _, e := range v.Evidence {
		if e.ID == x {
			return true
		}
	}
	return false
}
func entry(v Investigation, x string) bool {
	for _, e := range v.Entries {
		if e.ID == x {
			return true
		}
	}
	return false
}
func (s *Store) read(x string) (Investigation, error) {
	var v Investigation
	b, e := os.ReadFile(filepath.Join(s.root, x+".json"))
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func (s *Store) write(v Investigation) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := filepath.Join(s.root, "."+v.ID+".tmp")
	if e = os.WriteFile(tmp, b, 0600); e == nil {
		e = os.Rename(tmp, filepath.Join(s.root, v.ID+".json"))
	}
	return e
}
