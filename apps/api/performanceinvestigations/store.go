// Package performanceinvestigations retains collaborative, evidence-bound bottleneck diagnoses.
package performanceinvestigations

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

var ErrNotFound = errors.New("performance investigation not found")
var ErrInvalid = errors.New("invalid performance investigation")

type EvidenceRef struct {
	TrialID           string `json:"trial_id"`
	Revision          string `json:"revision"`
	WorkloadSource    string `json:"workload_source"`
	EnvironmentDigest string `json:"environment_digest"`
	Visibility        string `json:"visibility"`
}
type Citation struct {
	Kind        string `json:"kind"`
	TrialID     string `json:"trial_id,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Path        string `json:"path,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Dependency  string `json:"dependency,omitempty"`
	CommitID    string `json:"commit_id,omitempty"`
	ReleaseID   string `json:"release_id,omitempty"`
	RuntimePath string `json:"runtime_path,omitempty"`
	Label       string `json:"label,omitempty"`
}
type Entry struct {
	ID           string     `json:"id"`
	Kind         string     `json:"kind"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	Audience     string     `json:"audience"`
	Citations    []Citation `json:"citations"`
	Flamegraph   string     `json:"flamegraph,omitempty"`
	Comparison   string     `json:"comparison,omitempty"`
	Uncertainty  string     `json:"uncertainty,omitempty"`
	Challenges   string     `json:"challenges,omitempty"`
	Verdict      string     `json:"verdict,omitempty"`
	ActorID      string     `json:"actor_id"`
	Stale        bool       `json:"stale"`
	StaleReasons []string   `json:"stale_reasons,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
type Investigation struct {
	ID           string        `json:"id"`
	RepositoryID string        `json:"repository_id"`
	GoalID       string        `json:"goal_id"`
	GoalVersion  int64         `json:"goal_version"`
	Title        string        `json:"title"`
	Question     string        `json:"question"`
	Participants []string      `json:"participants"`
	OwnerIDs     []string      `json:"owner_ids"`
	Evidence     []EvidenceRef `json:"evidence"`
	Entries      []Entry       `json:"entries"`
	Changes      []Change      `json:"changes,omitempty"`
	CreatorID    string        `json:"creator_id"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}
type Change struct {
	ID               string    `json:"id"`
	Kind             string    `json:"kind"`
	OwnerKind        string    `json:"owner_kind"`
	OwnerID          string    `json:"owner_id"`
	GoalID           string    `json:"goal_id"`
	GoalVersion      int64     `json:"goal_version"`
	DiagnosisEntryID string    `json:"diagnosis_entry_id"`
	BaselineTrialID  string    `json:"baseline_trial_id"`
	Title            string    `json:"title"`
	Constraints      []string  `json:"constraints"`
	ProposalID       string    `json:"proposal_id,omitempty"`
	TaskID           string    `json:"task_id,omitempty"`
	PullRequestID    string    `json:"pull_request_id,omitempty"`
	CreatorID        string    `json:"creator_id"`
	CreatedAt        time.Time `json:"created_at"`
}
type CreateInput struct {
	GoalID      string        `json:"goal_id"`
	GoalVersion int64         `json:"goal_version"`
	Title       string        `json:"title"`
	Question    string        `json:"question"`
	OwnerIDs    []string      `json:"owner_ids"`
	Evidence    []EvidenceRef `json:"evidence"`
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
func newid() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) Create(repo, actor string, in CreateInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || in.GoalID == "" || in.GoalVersion < 1 || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Question) == "" || len(in.Evidence) == 0 || len(in.Evidence) > 20 {
		return Investigation{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, e := range in.Evidence {
		if e.TrialID == "" || e.Revision == "" || e.WorkloadSource == "" || e.EnvironmentDigest == "" || (e.Visibility != "repository" && e.Visibility != "participants") || seen[e.TrialID] {
			return Investigation{}, ErrInvalid
		}
		seen[e.TrialID] = true
	}
	now := s.now().UTC()
	v := Investigation{ID: newid(), RepositoryID: repo, GoalID: in.GoalID, GoalVersion: in.GoalVersion, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), Participants: []string{actor}, OwnerIDs: unique(in.OwnerIDs), Evidence: in.Evidence, CreatorID: actor, CreatedAt: now, UpdatedAt: now}
	if !has(v.OwnerIDs, actor) {
		v.OwnerIDs = append(v.OwnerIDs, actor)
	}
	sort.Strings(v.OwnerIDs)
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) AddChange(repo, investigation, actor string, change Change) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(investigation)
	if err != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !has(v.Participants, actor) || change.Title == "" || change.DiagnosisEntryID == "" || change.BaselineTrialID == "" {
		return Investigation{}, ErrInvalid
	}
	found := false
	for _, entry := range v.Entries {
		if entry.ID == change.DiagnosisEntryID && entry.Kind == "conclusion" && !entry.Stale {
			found = true
		}
	}
	if !found {
		return Investigation{}, ErrInvalid
	}
	change.ID, change.GoalID, change.GoalVersion = newid(), v.GoalID, v.GoalVersion
	change.CreatorID, change.CreatedAt = actor, s.now().UTC()
	v.Changes = append(v.Changes, change)
	v.UpdatedAt = change.CreatedAt
	return v, s.write(v)
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
		v, z := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if z == nil && v.RepositoryID == repo {
			v.Entries = nil
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) Invite(repo, id, actor, participant string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Investigation{}, ErrNotFound
	}
	if participant == "" {
		return Investigation{}, ErrInvalid
	}
	if !has(v.Participants, participant) {
		v.Participants = append(v.Participants, participant)
		sort.Strings(v.Participants)
	}
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (s *Store) Add(repo, id, actor string, e Entry) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, x := s.read(id)
	if x != nil || v.RepositoryID != repo || !has(v.Participants, actor) {
		return Investigation{}, ErrNotFound
	}
	if !map[string]bool{"hypothesis": true, "flame_graph": true, "comparison": true, "runtime_path": true, "challenge": true, "confirmation": true, "conclusion": true}[e.Kind] || strings.TrimSpace(e.Title) == "" || strings.TrimSpace(e.Body) == "" || len(e.Citations) == 0 || len(e.Citations) > 30 || (e.Audience != "participants" && e.Audience != "repository") {
		return Investigation{}, ErrInvalid
	}
	if e.Kind == "flame_graph" && e.Flamegraph == "" || e.Kind == "comparison" && e.Comparison == "" || e.Kind == "challenge" && e.Challenges == "" || e.Kind == "confirmation" && e.Verdict != "confirmed" && e.Verdict != "rejected" {
		return Investigation{}, ErrInvalid
	}
	if e.Challenges != "" && !entry(v.Entries, e.Challenges) {
		return Investigation{}, ErrInvalid
	}
	e.ID = newid()
	e.ActorID = actor
	e.Title = strings.TrimSpace(e.Title)
	e.Body = strings.TrimSpace(e.Body)
	e.CreatedAt = s.now().UTC()
	e.Stale = false
	e.StaleReasons = nil
	v.Entries = append(v.Entries, e)
	v.UpdatedAt = e.CreatedAt
	return v, s.write(v)
}
func Resolve(v Investigation, currentVersion int64, trials map[string]EvidenceRef) Investigation {
	for i := range v.Entries {
		reasons := []string{}
		if currentVersion != v.GoalVersion {
			reasons = append(reasons, "goal_version_changed")
		}
		for _, c := range v.Entries[i].Citations {
			if c.TrialID == "" {
				continue
			}
			captured, ok := evidence(v.Evidence, c.TrialID)
			live, liveok := trials[c.TrialID]
			if !ok || !liveok {
				reasons = append(reasons, "evidence_unavailable")
				continue
			}
			if captured.Revision != live.Revision {
				reasons = append(reasons, "revision_changed")
			}
			if captured.WorkloadSource != live.WorkloadSource {
				reasons = append(reasons, "workload_changed")
			}
			if captured.EnvironmentDigest != live.EnvironmentDigest {
				reasons = append(reasons, "environment_changed")
			}
		}
		v.Entries[i].Stale = len(reasons) > 0
		v.Entries[i].StaleReasons = unique(reasons)
	}
	return v
}
func evidence(xs []EvidenceRef, id string) (EvidenceRef, bool) {
	for _, x := range xs {
		if x.TrialID == id {
			return x, true
		}
	}
	return EvidenceRef{}, false
}
func entry(xs []Entry, id string) bool {
	for _, x := range xs {
		if x.ID == id {
			return true
		}
	}
	return false
}
func has(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
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
	return out
}
func (s *Store) read(id string) (Investigation, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if os.IsNotExist(e) {
		return Investigation{}, ErrNotFound
	}
	var v Investigation
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}
func (s *Store) write(v Investigation) error {
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
