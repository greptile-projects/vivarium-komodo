package relationships

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type CompatibilityChange struct {
	ID             string    `json:"id"`
	Classification string    `json:"classification"`
	Area           string    `json:"area"`
	Summary        string    `json:"summary"`
	Rationale      string    `json:"rationale"`
	ActorID        string    `json:"actor_id"`
	CreatedAt      time.Time `json:"created_at"`
}
type MigrationStep struct {
	ID        string `json:"id"`
	Position  int    `json:"position"`
	OwnerID   string `json:"owner_id"`
	Summary   string `json:"summary"`
	DependsOn string `json:"depends_on,omitempty"`
}
type MigrationTask struct {
	ID                 string                   `json:"id"`
	Position           int                      `json:"position"`
	RepositoryID       string                   `json:"repository_id"`
	TargetRepositoryID string                   `json:"target_repository_id"`
	TargetVersion      string                   `json:"target_version"`
	Title              string                   `json:"title"`
	Outcome            string                   `json:"outcome"`
	CompletionCriteria []string                 `json:"completion_criteria"`
	DependsOn          []string                 `json:"depends_on"`
	Discussion         []MigrationTaskComment   `json:"discussion"`
	Assignment         *MigrationTaskAssignment `json:"assignment,omitempty"`
	Work               *MigrationTaskWork       `json:"work,omitempty"`
	Status             string                   `json:"status"`
	Ready              bool                     `json:"ready"`
	BlockedBy          []string                 `json:"blocked_by"`
	CreatedByID        string                   `json:"created_by_id"`
	UpdatedByID        string                   `json:"updated_by_id"`
	CreatedAt          time.Time                `json:"created_at"`
	UpdatedAt          time.Time                `json:"updated_at"`
}
type MigrationTaskComment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type MigrationTaskAssignment struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	AssigneeID   string    `json:"assignee_id"`
	Mandate      string    `json:"mandate"`
	BaseRevision string    `json:"base_revision"`
	AssignedByID string    `json:"assigned_by_id"`
	AssignedAt   time.Time `json:"assigned_at"`
}
type MigrationTaskWork struct {
	RepositoryID  string     `json:"repository_id"`
	Branch        string     `json:"branch"`
	BaseRevision  string     `json:"base_revision"`
	HeadRevision  string     `json:"head_revision"`
	SessionID     string     `json:"change_session_id,omitempty"`
	PullRequestID string     `json:"pull_request_id,omitempty"`
	StartedByID   string     `json:"started_by_id"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}
type MigrationTaskInput struct {
	RepositoryID       string   `json:"repository_id"`
	TargetRepositoryID string   `json:"target_repository_id"`
	TargetVersion      string   `json:"target_version"`
	Title              string   `json:"title"`
	Outcome            string   `json:"outcome"`
	CompletionCriteria []string `json:"completion_criteria"`
	DependsOn          []string `json:"depends_on"`
}
type EvolutionException struct {
	ID         string     `json:"id"`
	ConsumerID string     `json:"consumer_repository_id,omitempty"`
	Reason     string     `json:"reason"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	ActorID    string     `json:"actor_id"`
	CreatedAt  time.Time  `json:"created_at"`
}
type EvolutionAcknowledgement struct {
	ActorID     string    `json:"actor_id"`
	Decision    string    `json:"decision"`
	Note        string    `json:"note"`
	OwnerForIDs []string  `json:"owner_for_repository_ids"`
	CreatedAt   time.Time `json:"created_at"`
}
type EvolutionFinding struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Body          string    `json:"body"`
	Uncertainty   string    `json:"uncertainty,omitempty"`
	RepositoryIDs []string  `json:"repository_ids"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
}
type EvolutionAnalysis struct {
	ID                  string    `json:"id"`
	Agent               string    `json:"agent"`
	Mandate             string    `json:"mandate"`
	RepositoryIDs       []string  `json:"repository_ids"`
	State               string    `json:"state"`
	InitiatedByID       string    `json:"initiated_by_id"`
	CredentialExpiresAt time.Time `json:"credential_expires_at"`
	CredentialDigest    string    `json:"credential_digest,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}
type AffectedConsumer struct {
	DependencyID string `json:"dependency_id"`
	RepositoryID string `json:"repository_id"`
	OwnerID      string `json:"owner_id"`
	CommitID     string `json:"commit_id"`
	Constraint   string `json:"constraint"`
}
type EvolutionPlan struct {
	ID                      string                     `json:"id"`
	RepositoryID            string                     `json:"repository_id"`
	InterfaceName           string                     `json:"interface_name"`
	SourceKind              string                     `json:"source_kind"`
	SourceID                string                     `json:"source_id"`
	CandidateCommitID       string                     `json:"candidate_commit_id"`
	CandidateSchemaPath     string                     `json:"candidate_schema_path"`
	CandidateSchemaSHA256   string                     `json:"candidate_schema_sha256"`
	Predecessor             Interface                  `json:"predecessor"`
	PredecessorSchemaSHA256 string                     `json:"predecessor_schema_sha256"`
	AffectedConsumers       []AffectedConsumer         `json:"affected_consumers"`
	Strategy                string                     `json:"strategy"`
	Changes                 []CompatibilityChange      `json:"changes"`
	Steps                   []MigrationStep            `json:"steps"`
	Tasks                   []MigrationTask            `json:"tasks"`
	Exceptions              []EvolutionException       `json:"exceptions"`
	Acknowledgements        []EvolutionAcknowledgement `json:"acknowledgements"`
	Findings                []EvolutionFinding         `json:"findings"`
	Analyses                []EvolutionAnalysis        `json:"analyses"`
	Verifications           []EvolutionVerification    `json:"verifications"`
	Rollout                 *EvolutionRollout          `json:"rollout,omitempty"`
	CreatedByID             string                     `json:"created_by_id"`
	CreatedAt               time.Time                  `json:"created_at"`
	UpdatedAt               time.Time                  `json:"updated_at"`
}

type EvolutionRevision struct {
	RepositoryID  string `json:"repository_id"`
	CommitID      string `json:"commit_id"`
	TaskID        string `json:"task_id"`
	PullRequestID string `json:"pull_request_id"`
	DependencyID  string `json:"dependency_id,omitempty"`
}
type EvolutionVerification struct {
	ID            string              `json:"id"`
	Revisions     []EvolutionRevision `json:"revisions"`
	RunIDs        []string            `json:"run_ids"`
	TriggeredByID string              `json:"triggered_by_id"`
	CreatedAt     time.Time           `json:"created_at"`
}
type EvolutionUpdate struct {
	Strategy   string                `json:"strategy"`
	Changes    []CompatibilityChange `json:"changes"`
	Steps      []MigrationStep       `json:"steps"`
	Exceptions []EvolutionException  `json:"exceptions"`
}

func (s *Store) CreateEvolution(v EvolutionPlan) (EvolutionPlan, error) {
	v.InterfaceName, v.SourceKind, v.SourceID = strings.TrimSpace(v.InterfaceName), strings.ToLower(strings.TrimSpace(v.SourceKind)), strings.TrimSpace(v.SourceID)
	if v.RepositoryID == "" || v.InterfaceName == "" || !oneOfEvolution(v.SourceKind, "proposal", "pull_request") || v.SourceID == "" || v.CandidateCommitID == "" || v.CandidateSchemaPath == "" || v.Predecessor.ID == "" || v.CreatedByID == "" {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v.ID, _ = newID()
	if v.ID == "" {
		return EvolutionPlan{}, errors.New("generate id")
	}
	now := s.now().UTC()
	v.CreatedAt, v.UpdatedAt = now, now
	v.Changes, v.Steps, v.Tasks, v.Exceptions, v.Acknowledgements, v.Findings, v.Analyses, v.Verifications = []CompatibilityChange{}, []MigrationStep{}, []MigrationTask{}, []EvolutionException{}, []EvolutionAcknowledgement{}, []EvolutionFinding{}, []EvolutionAnalysis{}, []EvolutionVerification{}
	return v, s.write("evolutions", v.ID, v)
}

func (s *Store) CreateEvolutionVerification(planID, actor string, revisions []EvolutionRevision) (EvolutionPlan, EvolutionVerification, error) {
	if actor == "" || len(revisions) < 2 || len(revisions) > 25 {
		return EvolutionPlan{}, EvolutionVerification{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, EvolutionVerification{}, err
	}
	seen := map[string]bool{}
	for _, revision := range revisions {
		if revision.RepositoryID == "" || revision.CommitID == "" || revision.TaskID == "" || revision.PullRequestID == "" || seen[revision.RepositoryID] {
			return v, EvolutionVerification{}, ErrInvalid
		}
		seen[revision.RepositoryID] = true
	}
	id, _ := newID()
	attempt := EvolutionVerification{ID: id, Revisions: append([]EvolutionRevision{}, revisions...), RunIDs: []string{}, TriggeredByID: actor, CreatedAt: s.now().UTC()}
	v.Verifications = append(v.Verifications, attempt)
	v.UpdatedAt = attempt.CreatedAt
	return v, attempt, s.write("evolutions", v.ID, v)
}

func (s *Store) AttachEvolutionVerificationRuns(planID, attemptID string, runIDs []string) (EvolutionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	for i := range v.Verifications {
		if v.Verifications[i].ID == attemptID {
			if len(v.Verifications[i].RunIDs) != 0 {
				return v, ErrConflict
			}
			v.Verifications[i].RunIDs = append([]string{}, runIDs...)
			return v, s.write("evolutions", v.ID, v)
		}
	}
	return v, ErrNotFound
}

func (s *Store) CreateMigrationTask(planID, actor string, in MigrationTaskInput) (EvolutionPlan, error) {
	if actor == "" {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	in.RepositoryID, in.TargetRepositoryID, in.TargetVersion, in.Title, in.Outcome = strings.TrimSpace(in.RepositoryID), strings.TrimSpace(in.TargetRepositoryID), strings.TrimSpace(in.TargetVersion), strings.TrimSpace(in.Title), strings.TrimSpace(in.Outcome)
	if in.RepositoryID == "" || in.TargetRepositoryID == "" || in.TargetVersion == "" || in.Title == "" || in.Outcome == "" || len(in.CompletionCriteria) == 0 || len(in.DependsOn) > 100 {
		return v, ErrInvalid
	}
	allowed := map[string]bool{v.RepositoryID: true}
	for _, c := range v.AffectedConsumers {
		allowed[c.RepositoryID] = true
	}
	if !allowed[in.TargetRepositoryID] {
		return v, ErrInvalid
	}
	existing := map[string]bool{}
	for _, task := range v.Tasks {
		existing[task.ID] = true
	}
	for _, id := range in.DependsOn {
		if !existing[id] {
			return v, ErrInvalid
		}
	}
	criteria := make([]string, len(in.CompletionCriteria))
	for i, c := range in.CompletionCriteria {
		criteria[i] = strings.TrimSpace(c)
		if criteria[i] == "" {
			return v, ErrInvalid
		}
	}
	id, _ := newID()
	now := s.now().UTC()
	t := MigrationTask{ID: id, Position: len(v.Tasks) + 1, RepositoryID: in.RepositoryID, TargetRepositoryID: in.TargetRepositoryID, TargetVersion: in.TargetVersion, Title: in.Title, Outcome: in.Outcome, CompletionCriteria: criteria, DependsOn: append([]string{}, in.DependsOn...), Discussion: []MigrationTaskComment{}, Status: "planned", CreatedByID: actor, UpdatedByID: actor, CreatedAt: now, UpdatedAt: now}
	v.Tasks = append(v.Tasks, t)
	deriveMigrationReadiness(&v)
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) AssignMigrationTask(planID, taskID, actor, expectedID, kind, assignee, mandate, base string) (EvolutionPlan, error) {
	kind, assignee, mandate, base = strings.ToLower(strings.TrimSpace(kind)), strings.TrimSpace(assignee), strings.TrimSpace(mandate), strings.TrimSpace(base)
	if actor == "" || !oneOfEvolution(kind, "human", "agent") || assignee == "" || mandate == "" || base == "" {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	deriveMigrationReadiness(&v)
	i := migrationTaskIndex(v.Tasks, taskID)
	if i < 0 {
		return v, ErrNotFound
	}
	t := &v.Tasks[i]
	if !t.Ready || t.Work != nil || (t.Assignment != nil && t.Assignment.ID != expectedID) || (t.Assignment == nil && expectedID != "") {
		return v, ErrConflict
	}
	id, _ := newID()
	now := s.now().UTC()
	t.Assignment = &MigrationTaskAssignment{ID: id, Kind: kind, AssigneeID: assignee, Mandate: mandate, BaseRevision: base, AssignedByID: actor, AssignedAt: now}
	t.UpdatedByID, t.UpdatedAt = actor, now
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) StartMigrationTask(planID, taskID, actor, expectedID, repositoryID, branch, head, session string) (EvolutionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	i := migrationTaskIndex(v.Tasks, taskID)
	if i < 0 {
		return v, ErrNotFound
	}
	t := &v.Tasks[i]
	if t.Assignment == nil || t.Assignment.ID != expectedID || t.Work != nil || repositoryID != t.RepositoryID || branch == "" || head != t.Assignment.BaseRevision {
		return v, ErrConflict
	}
	now := s.now().UTC()
	t.Work = &MigrationTaskWork{RepositoryID: repositoryID, Branch: branch, BaseRevision: head, HeadRevision: head, SessionID: session, StartedByID: actor, StartedAt: now}
	t.Status = "in_progress"
	t.Ready = false
	t.UpdatedByID, t.UpdatedAt = actor, now
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) SynchronizeMigrationTask(planID, taskID, actor, head, pullID string, completed bool) (EvolutionPlan, error) {
	if actor == "" || head == "" {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	i := migrationTaskIndex(v.Tasks, taskID)
	if i < 0 {
		return v, ErrNotFound
	}
	t := &v.Tasks[i]
	if t.Work == nil {
		return v, ErrConflict
	}
	now := s.now().UTC()
	t.Work.HeadRevision = head
	if pullID != "" {
		t.Work.PullRequestID = pullID
		t.Status = "review"
	}
	if completed {
		t.Status = "completed"
		t.Work.CompletedAt = &now
	}
	t.UpdatedByID, t.UpdatedAt = actor, now
	deriveMigrationReadiness(&v)
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) CommentMigrationTask(planID, taskID, actor, body string) (EvolutionPlan, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 10000 {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(planID)
	if err != nil {
		return v, err
	}
	i := migrationTaskIndex(v.Tasks, taskID)
	if i < 0 {
		return v, ErrNotFound
	}
	id, _ := newID()
	now := s.now().UTC()
	v.Tasks[i].Discussion = append(v.Tasks[i].Discussion, MigrationTaskComment{ID: id, AuthorID: actor, Body: body, CreatedAt: now})
	v.Tasks[i].UpdatedAt = now
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func migrationTaskIndex(tasks []MigrationTask, id string) int {
	for i := range tasks {
		if tasks[i].ID == id {
			return i
		}
	}
	return -1
}
func deriveMigrationReadiness(v *EvolutionPlan) {
	done := map[string]bool{}
	for _, t := range v.Tasks {
		done[t.ID] = t.Status == "completed"
	}
	for i := range v.Tasks {
		t := &v.Tasks[i]
		t.BlockedBy = []string{}
		for _, d := range t.DependsOn {
			if !done[d] {
				t.BlockedBy = append(t.BlockedBy, d)
			}
		}
		t.Ready = t.Status == "planned" && len(t.BlockedBy) == 0
	}
}
func (s *Store) Evolution(id string) (EvolutionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readEvolution(id)
}
func (s *Store) Evolutions(repositoryID string) ([]EvolutionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []EvolutionPlan{}
	err := s.list("evolutions", func(b []byte) error {
		var v EvolutionPlan
		if jsonUnmarshal(b, &v) != nil {
			return ErrNotFound
		}
		if v.RepositoryID == repositoryID {
			scrubEvolution(&v)
			out = append(out, v)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, err
}
func (s *Store) UpdateEvolution(id, actor string, in EvolutionUpdate) (EvolutionPlan, error) {
	in.Strategy = strings.TrimSpace(in.Strategy)
	if actor == "" || in.Strategy == "" || len(in.Strategy) > 20000 || len(in.Changes) > 100 || len(in.Steps) > 100 || len(in.Exceptions) > 100 {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(id)
	if err != nil {
		return v, err
	}
	now := s.now().UTC()
	stepIDs := map[string]bool{}
	for i := range in.Changes {
		c := &in.Changes[i]
		c.Classification = strings.ToLower(strings.TrimSpace(c.Classification))
		c.Area = strings.TrimSpace(c.Area)
		c.Summary = strings.TrimSpace(c.Summary)
		c.Rationale = strings.TrimSpace(c.Rationale)
		if !oneOfEvolution(c.Classification, "breaking", "compatible", "behavioral", "unknown") || c.Area == "" || c.Summary == "" {
			return v, ErrInvalid
		}
		c.ID, _ = newID()
		c.ActorID = actor
		c.CreatedAt = now
	}
	for i := range in.Steps {
		p := &in.Steps[i]
		p.Summary = strings.TrimSpace(p.Summary)
		if p.Summary == "" || p.OwnerID == "" {
			return v, ErrInvalid
		}
		if p.ID == "" {
			p.ID, _ = newID()
		}
		p.Position = i + 1
		stepIDs[p.ID] = true
	}
	for i := range in.Steps {
		if in.Steps[i].DependsOn != "" && !stepIDs[in.Steps[i].DependsOn] {
			return v, ErrInvalid
		}
	}
	for i := range in.Exceptions {
		e := &in.Exceptions[i]
		e.Reason = strings.TrimSpace(e.Reason)
		if e.Reason == "" {
			return v, ErrInvalid
		}
		e.ID, _ = newID()
		e.ActorID = actor
		e.CreatedAt = now
	}
	v.Strategy, v.Changes, v.Steps, v.Exceptions, v.UpdatedAt = in.Strategy, in.Changes, in.Steps, in.Exceptions, now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) AcknowledgeEvolution(id, actor, decision, note string, ownerFor []string) (EvolutionPlan, error) {
	decision, note = strings.ToLower(strings.TrimSpace(decision)), strings.TrimSpace(note)
	if actor == "" || !oneOfEvolution(decision, "acknowledge", "request_changes") || len(note) > 5000 || len(ownerFor) == 0 {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(id)
	if err != nil {
		return v, err
	}
	a := EvolutionAcknowledgement{ActorID: actor, Decision: decision, Note: note, OwnerForIDs: ownerFor, CreatedAt: s.now().UTC()}
	for i := range v.Acknowledgements {
		if v.Acknowledgements[i].ActorID == actor {
			v.Acknowledgements[i] = a
			v.UpdatedAt = a.CreatedAt
			return v, s.write("evolutions", v.ID, v)
		}
	}
	v.Acknowledgements = append(v.Acknowledgements, a)
	v.UpdatedAt = a.CreatedAt
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) StartEvolutionAnalysis(id, actor, agent, mandate string, repositories []string) (EvolutionPlan, string, error) {
	agent, mandate = strings.TrimSpace(agent), strings.TrimSpace(mandate)
	if actor == "" || agent == "" || mandate == "" || len(repositories) == 0 {
		return EvolutionPlan{}, "", ErrInvalid
	}
	tokenID, _ := newID()
	secret, _ := newID()
	token := "evo_" + tokenID + secret
	digest := sha256.Sum256([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.readEvolution(id)
	if err != nil {
		return v, "", err
	}
	allowed := map[string]bool{v.RepositoryID: true}
	for _, c := range v.AffectedConsumers {
		allowed[c.RepositoryID] = true
	}
	for _, r := range repositories {
		if !allowed[r] {
			return v, "", ErrInvalid
		}
	}
	now := s.now().UTC()
	v.Analyses = append(v.Analyses, EvolutionAnalysis{ID: tokenID, Agent: agent, Mandate: mandate, RepositoryIDs: repositories, State: "active", InitiatedByID: actor, CredentialExpiresAt: now.Add(24 * time.Hour), CredentialDigest: hex.EncodeToString(digest[:]), CreatedAt: now})
	v.UpdatedAt = now
	return v, token, s.write("evolutions", v.ID, v)
}
func (s *Store) EvolutionAnalysisContext(token string) (EvolutionPlan, EvolutionAnalysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, a, err := s.analysisByToken(token)
	scrubEvolution(&v)
	a.CredentialDigest = ""
	return v, a, err
}
func (s *Store) AddEvolutionFinding(token, kind, body, uncertainty string, repositories []string) (EvolutionPlan, error) {
	kind, body, uncertainty = strings.ToLower(strings.TrimSpace(kind)), strings.TrimSpace(body), strings.TrimSpace(uncertainty)
	if !oneOfEvolution(kind, "finding", "question", "risk") || body == "" || len(body) > 20000 {
		return EvolutionPlan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, a, err := s.analysisByToken(token)
	if err != nil {
		return v, err
	}
	allowed := map[string]bool{}
	for _, id := range a.RepositoryIDs {
		allowed[id] = true
	}
	for _, id := range repositories {
		if !allowed[id] {
			return v, ErrInvalid
		}
	}
	now := s.now().UTC()
	id, _ := newID()
	v.Findings = append(v.Findings, EvolutionFinding{ID: id, Kind: kind, Body: body, Uncertainty: uncertainty, RepositoryIDs: repositories, ActorID: "agent:" + a.Agent, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write("evolutions", v.ID, v)
}
func (s *Store) analysisByToken(token string) (EvolutionPlan, EvolutionAnalysis, error) {
	d := sha256.Sum256([]byte(token))
	digest := hex.EncodeToString(d[:])
	var found EvolutionPlan
	var analysis EvolutionAnalysis
	err := s.list("evolutions", func(b []byte) error {
		var v EvolutionPlan
		if jsonUnmarshal(b, &v) != nil {
			return nil
		}
		for _, a := range v.Analyses {
			if a.CredentialDigest == digest {
				found, analysis = v, a
				return nil
			}
		}
		return nil
	})
	if err != nil || found.ID == "" {
		return found, analysis, ErrNotFound
	}
	if analysis.State != "active" || !s.now().Before(analysis.CredentialExpiresAt) {
		return found, analysis, ErrConflict
	}
	return found, analysis, nil
}
func (s *Store) readEvolution(id string) (EvolutionPlan, error) {
	var found EvolutionPlan
	err := s.list("evolutions", func(b []byte) error {
		var v EvolutionPlan
		if jsonUnmarshal(b, &v) != nil {
			return ErrNotFound
		}
		if v.ID == id {
			found = v
		}
		return nil
	})
	if err != nil || found.ID == "" {
		return found, ErrNotFound
	}
	return found, nil
}
func scrubEvolution(v *EvolutionPlan) {
	for i := range v.Analyses {
		v.Analyses[i].CredentialDigest = ""
	}
}
func oneOfEvolution(v string, values ...string) bool {
	for _, candidate := range values {
		if v == candidate {
			return true
		}
	}
	return false
}
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
