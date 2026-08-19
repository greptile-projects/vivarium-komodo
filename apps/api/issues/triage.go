package issues

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Triage struct {
	Classification string     `json:"classification,omitempty"`
	Priority       string     `json:"priority,omitempty"`
	AssigneeIDs    []string   `json:"assignee_ids,omitempty"`
	Labels         []string   `json:"labels,omitempty"`
	DuplicateOf    string     `json:"duplicate_of,omitempty"`
	UpdatedByID    string     `json:"updated_by_id,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
}
type Relationship struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	ResourceID   string    `json:"resource_id"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Revision     string    `json:"revision,omitempty"`
	Path         string    `json:"path,omitempty"`
	Note         string    `json:"note,omitempty"`
	AddedByID    string    `json:"added_by_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Citation struct {
	Kind          string `json:"kind"`
	ResourceID    string `json:"resource_id"`
	Revision      string `json:"revision,omitempty"`
	Path          string `json:"path,omitempty"`
	EventSequence int64  `json:"event_sequence,omitempty"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
}
type InvestigationEntry struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	Body               string     `json:"body"`
	Citations          []Citation `json:"citations"`
	SuspectedRevisions []string   `json:"suspected_revisions,omitempty"`
	SuspectedOwnerIDs  []string   `json:"suspected_owner_ids,omitempty"`
	TargetEntryID      string     `json:"target_entry_id,omitempty"`
	AuthorKind         string     `json:"author_kind"`
	AuthorID           string     `json:"author_id"`
	Stale              bool       `json:"stale"`
	Disputed           bool       `json:"disputed"`
	CreatedAt          time.Time  `json:"created_at"`
}
type AgentRun struct {
	ID          string     `json:"id"`
	AgentID     string     `json:"agent_id"`
	CreatedByID string     `json:"created_by_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}
type Investigation struct {
	ID             string               `json:"id"`
	ReproductionID string               `json:"reproduction_id"`
	Revision       string               `json:"revision"`
	Status         string               `json:"status"`
	CreatedByID    string               `json:"created_by_id"`
	Entries        []InvestigationEntry `json:"entries"`
	AgentRuns      []AgentRun           `json:"agent_runs,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

func validTriage(t Triage) bool {
	classes := map[string]bool{"": true, "bug": true, "regression": true, "performance": true, "security": true, "documentation": true, "support": true}
	priorities := map[string]bool{"": true, "low": true, "medium": true, "high": true, "urgent": true}
	if !classes[t.Classification] || !priorities[t.Priority] || len(t.AssigneeIDs) > 20 || len(t.Labels) > 20 {
		return false
	}
	for _, value := range append(append([]string{}, t.AssigneeIDs...), t.Labels...) {
		if strings.TrimSpace(value) == "" || len(value) > 100 {
			return false
		}
	}
	return true
}
func (s *Store) SetTriage(repo, id, actor string, expected int64, triage Triage) (Issue, error) {
	if actor == "" || !validTriage(triage) {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, err
	}
	if expected != v.Version {
		return Issue{}, ErrConflict
	}
	now := s.now().UTC()
	triage.UpdatedByID = actor
	triage.UpdatedAt = &now
	v.Triage = triage
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "triage.updated", ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) AddRelationship(repo, id, actor string, link Relationship) (Issue, error) {
	allowed := map[string]bool{"code": true, "dependency": true, "release": true, "deployment": true, "incident": true, "proposal": true, "pull_request": true, "decision": true, "investigation": true, "exploratory_finding": true, "review": true, "quality_plan": true, "test_scenario": true, "check_run": true}
	link.Kind = strings.TrimSpace(link.Kind)
	link.ResourceID = strings.TrimSpace(link.ResourceID)
	if actor == "" || !allowed[link.Kind] || link.ResourceID == "" || len(link.Note) > 2000 {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, err
	}
	link.ID, _ = newID()
	link.AddedByID = actor
	link.CreatedAt = s.now().UTC()
	v.Relationships = append(v.Relationships, link)
	v.Version++
	v.UpdatedAt = link.CreatedAt
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "relationship.added", ActorID: actor, Detail: link.Kind + ":" + link.ResourceID, CreatedAt: link.CreatedAt})
	return v, s.write(v)
}
func (s *Store) CreateInvestigation(repo, id, reproduction, revision, actor string) (Issue, Investigation, error) {
	if reproduction == "" || revision == "" || actor == "" {
		return Issue{}, Investigation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, Investigation{}, err
	}
	iid, _ := newID()
	now := s.now().UTC()
	inv := Investigation{ID: iid, ReproductionID: reproduction, Revision: revision, Status: "open", CreatedByID: actor, Entries: []InvestigationEntry{}, AgentRuns: []AgentRun{}, CreatedAt: now, UpdatedAt: now}
	for i := range v.Investigations {
		if v.Investigations[i].Revision != revision {
			for j := range v.Investigations[i].Entries {
				v.Investigations[i].Entries[j].Stale = true
			}
		}
	}
	v.Investigations = append(v.Investigations, inv)
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "investigation.opened", ActorID: actor, Detail: iid, CreatedAt: now})
	return v, inv, s.write(v)
}
func validEntry(e InvestigationEntry) bool {
	kinds := map[string]bool{"hypothesis": true, "finding": true, "evidence_request": true, "conclusion": true, "challenge": true}
	return kinds[e.Kind] && strings.TrimSpace(e.Body) != "" && len(e.Body) <= 10000 && len(e.Citations) > 0 && len(e.Citations) <= 20
}
func (s *Store) AddInvestigationEntry(repo, id, investigation, actorKind, actor string, e InvestigationEntry) (Issue, InvestigationEntry, error) {
	if !validEntry(e) || actor == "" || (actorKind != "human" && actorKind != "agent") {
		return Issue{}, e, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, e, err
	}
	idx := -1
	for i := range v.Investigations {
		if v.Investigations[i].ID == investigation {
			idx = i
		}
	}
	if idx < 0 {
		return Issue{}, e, ErrNotFound
	}
	inv := &v.Investigations[idx]
	if inv.Status != "open" {
		return Issue{}, e, ErrConflict
	}
	for _, c := range e.Citations {
		if c.Kind != "reproduction_event" && c.Kind != "reproduction_artifact" && c.Kind != "code" && c.Kind != "relationship" {
			return Issue{}, e, ErrInvalid
		}
		if c.ResourceID == "" {
			return Issue{}, e, ErrInvalid
		}
	}
	if e.Kind == "challenge" && e.TargetEntryID == "" {
		return Issue{}, e, ErrInvalid
	}
	eid, _ := newID()
	now := s.now().UTC()
	e.ID = eid
	e.AuthorKind = actorKind
	e.AuthorID = actor
	e.CreatedAt = now
	e.Citations = append([]Citation{}, e.Citations...)
	if e.Kind == "challenge" {
		found := false
		for i := range inv.Entries {
			if inv.Entries[i].ID == e.TargetEntryID {
				inv.Entries[i].Disputed = true
				found = true
			}
		}
		if !found {
			return Issue{}, e, ErrInvalid
		}
	}
	inv.Entries = append(inv.Entries, e)
	inv.UpdatedAt = now
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "investigation." + e.Kind, ActorID: actor, Detail: eid, CreatedAt: now})
	return v, e, s.write(v)
}
func (s *Store) StartAgentRun(repo, id, investigation, agent, actor string) (Issue, string, error) {
	if agent == "" || actor == "" {
		return Issue{}, "", ErrInvalid
	}
	token, err := newID()
	if err != nil {
		return Issue{}, "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, "", err
	}
	idx := -1
	for i := range v.Investigations {
		if v.Investigations[i].ID == investigation {
			idx = i
		}
	}
	if idx < 0 {
		return Issue{}, "", ErrNotFound
	}
	now := s.now().UTC()
	sum := sha256.Sum256([]byte(token))
	runID, _ := newID()
	v.Investigations[idx].AgentRuns = append(v.Investigations[idx].AgentRuns, AgentRun{ID: runID, AgentID: agent, CreatedByID: actor, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour)})
	tokenDir := filepath.Join(s.root, ".agent-tokens")
	if err = os.MkdirAll(tokenDir, 0750); err != nil {
		return Issue{}, "", err
	}
	binding := map[string]string{"repository_id": repo, "issue_id": id, "investigation_id": investigation, "run_id": runID}
	data, _ := json.Marshal(binding)
	if err = os.WriteFile(filepath.Join(tokenDir, hex.EncodeToString(sum[:])+".json"), data, 0640); err != nil {
		return Issue{}, "", err
	}
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "investigation.agent_started", ActorID: actor, Detail: runID, CreatedAt: now})
	return v, token, s.write(v)
}

func (s *Store) RevokeAgentRun(repo, id, investigation, runID, actor string) (Issue, error) {
	if actor == "" {
		return Issue{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, id)
	if err != nil {
		return v, err
	}
	now := s.now().UTC()
	for i := range v.Investigations {
		if v.Investigations[i].ID == investigation {
			for j := range v.Investigations[i].AgentRuns {
				run := &v.Investigations[i].AgentRuns[j]
				if run.ID == runID {
					if run.RevokedAt == nil {
						run.RevokedAt = &now
						v.Version++
						v.UpdatedAt = now
						v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "investigation.agent_revoked", ActorID: actor, Detail: runID, CreatedAt: now})
					}
					return v, s.write(v)
				}
			}
		}
	}
	return Issue{}, ErrNotFound
}
func (s *Store) AgentContext(token string) (Issue, Investigation, AgentRun, error) {
	sum := sha256.Sum256([]byte(token))
	wanted := hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(filepath.Join(s.root, ".agent-tokens", wanted+".json"))
	if err != nil {
		return Issue{}, Investigation{}, AgentRun{}, ErrNotFound
	}
	var binding map[string]string
	if json.Unmarshal(data, &binding) != nil {
		return Issue{}, Investigation{}, AgentRun{}, ErrNotFound
	}
	v, err := s.read(binding["repository_id"], binding["issue_id"])
	if err != nil {
		return Issue{}, Investigation{}, AgentRun{}, err
	}
	now := s.now().UTC()
	for _, inv := range v.Investigations {
		if inv.ID != binding["investigation_id"] {
			continue
		}
		for _, run := range inv.AgentRuns {
			if run.ID == binding["run_id"] && run.RevokedAt == nil && now.Before(run.ExpiresAt) {
				return v, inv, run, nil
			}
		}
	}
	return Issue{}, Investigation{}, AgentRun{}, ErrNotFound
}
