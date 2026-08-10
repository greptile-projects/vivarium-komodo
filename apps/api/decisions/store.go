// Package decisions owns durable, attributable technical-decision scope and discussion.
package decisions

import (
	"crypto/rand"
	"crypto/sha256"
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
	ErrNotFound = errors.New("decision not found")
	ErrInvalid  = errors.New("invalid decision")
)

type Context struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type Resource struct {
	Kind         string `json:"kind"`
	RepositoryID string `json:"repository_id,omitempty"`
	ID           string `json:"id,omitempty"`
	Ref          string `json:"ref,omitempty"`
	Path         string `json:"path,omitempty"`
	Label        string `json:"label"`
}
type Scope struct {
	Version           int        `json:"version"`
	Question          string     `json:"question"`
	Constraints       []string   `json:"constraints"`
	SuccessMeasures   []string   `json:"success_measures"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	AffectedResources []Resource `json:"affected_resources"`
	ParticipantIDs    []string   `json:"participant_ids"`
	OwnerID           string     `json:"owner_id"`
	ChangedByID       string     `json:"changed_by_id"`
	ChangeSummary     string     `json:"change_summary"`
	CreatedAt         time.Time  `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Evidence struct {
	Kind         string    `json:"kind"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Revision     string    `json:"revision,omitempty"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Path         string    `json:"path,omitempty"`
	URL          string    `json:"url,omitempty"`
	Summary      string    `json:"summary"`
	ObservedAt   time.Time `json:"observed_at"`
}
type Claim struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	AuthorID     string    `json:"author_id"`
	SupersedesID string    `json:"supersedes_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
type Finding struct {
	ID          string     `json:"id"`
	Agent       string     `json:"agent"`
	Body        string     `json:"body"`
	Uncertainty string     `json:"uncertainty"`
	Evidence    []Evidence `json:"evidence"`
	CreatedAt   time.Time  `json:"created_at"`
}
type Alternative struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	State       string       `json:"state"`
	CreatedByID string       `json:"created_by_id"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Claims      []Claim      `json:"claims"`
	Evidence    []Evidence   `json:"evidence"`
	Findings    []Finding    `json:"agent_findings"`
	Experiments []Experiment `json:"experiments"`
}
type Measurement struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
type ExperimentCheckpoint struct {
	ID                    string           `json:"id"`
	ActorID               string           `json:"actor_id"`
	WorkspaceCheckpointID string           `json:"workspace_checkpoint_id"`
	Summary               string           `json:"summary"`
	Measurements          []Measurement    `json:"measurements,omitempty"`
	LogSequences          []int64          `json:"log_sequences,omitempty"`
	ArtifactPaths         []string         `json:"artifact_paths,omitempty"`
	ResourceUse           map[string]int64 `json:"resource_use,omitempty"`
	CreatedAt             time.Time        `json:"created_at"`
}
type Experiment struct {
	ID                     string                 `json:"id"`
	WorkspaceID            string                 `json:"workspace_id"`
	Revision               string                 `json:"revision"`
	CommandName            string                 `json:"command_name"`
	DefinitionDigest       string                 `json:"environment_digest"`
	DependencyDigest       string                 `json:"dependency_digest"`
	CreatedByID            string                 `json:"created_by_id"`
	ReproducesExperimentID string                 `json:"reproduces_experiment_id,omitempty"`
	State                  string                 `json:"state"`
	InvalidatedBy          []string               `json:"invalidated_by,omitempty"`
	Checkpoints            []ExperimentCheckpoint `json:"checkpoints"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
}
type Comparison struct {
	AlternativeID    string            `json:"alternative_id"`
	CurrentClaims    map[string]string `json:"current_claims"`
	MissingCriteria  []string          `json:"missing_criteria"`
	EvidenceCount    int               `json:"evidence_count"`
	EvidenceKinds    []string          `json:"evidence_kinds"`
	StaleEvidenceIDs []string          `json:"stale_evidence_ids"`
	DissentCount     int               `json:"dissent_count"`
}
type ApprovalRequirement struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	ActorID       string     `json:"actor_id"`
	Policy        string     `json:"policy,omitempty"`
	State         string     `json:"state"`
	RequestedByID string     `json:"requested_by_id"`
	RequestedAt   time.Time  `json:"requested_at"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	Note          string     `json:"note,omitempty"`
}
type Commitment struct {
	Version                int                   `json:"version"`
	ScopeVersion           int                   `json:"scope_version"`
	SelectedAlternativeID  string                `json:"selected_alternative_id"`
	RejectedAlternativeIDs []string              `json:"rejected_alternative_ids"`
	Rationale              string                `json:"rationale"`
	AcceptedTradeoffs      []string              `json:"accepted_tradeoffs"`
	Dissent                []string              `json:"dissent"`
	Conditions             []string              `json:"conditions"`
	ReviewDate             *time.Time            `json:"review_date,omitempty"`
	Evidence               []Evidence            `json:"evidence_considered"`
	Approvals              []ApprovalRequirement `json:"approvals"`
	PublishedByID          string                `json:"published_by_id"`
	PublishedAt            time.Time             `json:"published_at"`
}
type Exception struct {
	ID                string     `json:"id"`
	CommitmentVersion int        `json:"commitment_version"`
	Scope             string     `json:"scope"`
	Reason            string     `json:"reason"`
	Conditions        []string   `json:"conditions"`
	AuthorizedByID    string     `json:"authorized_by_id"`
	StartsAt          time.Time  `json:"starts_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}
type Decision struct {
	ID                   string                `json:"id"`
	RepositoryID         string                `json:"repository_id"`
	Title                string                `json:"title"`
	State                string                `json:"state"`
	Context              Context               `json:"context"`
	CreatedByID          string                `json:"created_by_id"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	Scope                Scope                 `json:"scope"`
	History              []Scope               `json:"history"`
	Comments             []Comment             `json:"comments"`
	Alternatives         []Alternative         `json:"alternatives"`
	Comparison           []Comparison          `json:"comparison"`
	ApprovalRequirements []ApprovalRequirement `json:"approval_requirements"`
	Commitments          []Commitment          `json:"commitments"`
	Exceptions           []Exception           `json:"exceptions"`
	PendingApprovalIDs   []string              `json:"pending_approval_ids"`
	Conflicts            []string              `json:"conflicts"`
}
type ScopeInput struct {
	Question          string     `json:"question"`
	Constraints       []string   `json:"constraints"`
	SuccessMeasures   []string   `json:"success_measures"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	AffectedResources []Resource `json:"affected_resources"`
	ParticipantIDs    []string   `json:"participant_ids"`
	OwnerID           string     `json:"owner_id"`
	ChangeSummary     string     `json:"change_summary"`
}
type Store struct {
	root   string
	mu     sync.Mutex
	now    func() time.Time
	tokens map[string]agentToken
}
type agentToken struct {
	DecisionID    string    `json:"decision_id"`
	AlternativeID string    `json:"alternative_id"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now, tokens: map[string]agentToken{}}, nil
}
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func cleanList(in []string) ([]string, bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || len(v) > 500 {
			return nil, false
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, true
}
func validContext(k string) bool {
	switch k {
	case "repository", "proposal", "investigation", "incident", "evolution_plan", "stewardship_opportunity":
		return true
	}
	return false
}
func scope(in ScopeInput, actor string, version int, now time.Time) (Scope, error) {
	q := strings.TrimSpace(in.Question)
	c, ok := cleanList(in.Constraints)
	if !ok {
		return Scope{}, ErrInvalid
	}
	m, ok := cleanList(in.SuccessMeasures)
	if !ok {
		return Scope{}, ErrInvalid
	}
	p, ok := cleanList(in.ParticipantIDs)
	if !ok || q == "" || len(q) > 4000 || len(c) == 0 || len(m) == 0 || in.OwnerID == "" || len(p) == 0 {
		return Scope{}, ErrInvalid
	}
	if !contains(p, in.OwnerID) || !contains(p, actor) {
		return Scope{}, ErrInvalid
	}
	for i := range in.AffectedResources {
		in.AffectedResources[i].Kind = strings.TrimSpace(in.AffectedResources[i].Kind)
		in.AffectedResources[i].Label = strings.TrimSpace(in.AffectedResources[i].Label)
		if in.AffectedResources[i].Kind == "" || in.AffectedResources[i].Label == "" || len(in.AffectedResources[i].Label) > 300 {
			return Scope{}, ErrInvalid
		}
	}
	if in.Deadline != nil {
		d := in.Deadline.UTC()
		in.Deadline = &d
	}
	return Scope{Version: version, Question: q, Constraints: c, SuccessMeasures: m, Deadline: in.Deadline, AffectedResources: in.AffectedResources, ParticipantIDs: p, OwnerID: in.OwnerID, ChangedByID: actor, ChangeSummary: strings.TrimSpace(in.ChangeSummary), CreatedAt: now}, nil
}
func contains(v []string, x string) bool {
	for _, a := range v {
		if a == x {
			return true
		}
	}
	return false
}
func (s *Store) Create(repo, actor, title string, context Context, in ScopeInput) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	title = strings.TrimSpace(title)
	context.Kind = strings.TrimSpace(context.Kind)
	context.ID = strings.TrimSpace(context.ID)
	if repo == "" || title == "" || len(title) > 200 || !validContext(context.Kind) || (context.Kind != "repository" && context.ID == "") {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	sc, e := scope(in, actor, 1, now)
	if e != nil {
		return Decision{}, e
	}
	v := Decision{ID: newID(), RepositoryID: repo, Title: title, State: "pending", Context: context, CreatedByID: actor, CreatedAt: now, UpdatedAt: now, Scope: sc, History: []Scope{sc}, Comments: []Comment{}, Alternatives: []Alternative{}, ApprovalRequirements: []ApprovalRequirement{}, Commitments: []Commitment{}, Exceptions: []Exception{}}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	s.compare(&v)
	return v, nil
}
func (s *Store) List(repo, kind, id string) ([]Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Decision{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Decision{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, z := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if z == nil && (kind == "" || (v.Context.Kind == kind && v.Context.ID == id)) {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}
func (s *Store) Revise(repo, id, actor, title string, in ScopeInput) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 200 || strings.TrimSpace(in.ChangeSummary) == "" {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	sc, e := scope(in, actor, len(v.History)+1, now)
	if e != nil {
		return Decision{}, e
	}
	v.Title = title
	v.Scope = sc
	v.History = append(v.History, sc)
	v.UpdatedAt = now
	reopen(&v)
	s.compare(&v)
	return v, s.write(v)
}
func (s *Store) Comment(repo, id, actor, body string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 65536 {
		return Decision{}, ErrInvalid
	}
	now := s.now().UTC()
	v.Comments = append(v.Comments, Comment{ID: newID(), AuthorID: actor, Body: body, CreatedAt: now})
	v.UpdatedAt = now
	return v, s.write(v)
}

var criteria = []string{"assumption", "tradeoff", "risk", "compatibility", "cost", "outcome"}

func validClaim(v string) bool {
	for _, x := range append(criteria, "dissent") {
		if v == x {
			return true
		}
	}
	return false
}
func validEvidence(v string) bool {
	switch v {
	case "code", "dependency", "release", "incident", "usage":
		return true
	}
	return false
}
func (s *Store) AddAlternative(repo, decision, actor, title string, claims []Claim, evidence []Evidence) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	title = strings.TrimSpace(title)
	now := s.now().UTC()
	if title == "" || len(title) > 200 || !prepareClaims(claims, actor, now, nil) || !prepareEvidence(evidence) {
		return Decision{}, ErrInvalid
	}
	a := Alternative{ID: newID(), Title: title, State: "active", CreatedByID: actor, CreatedAt: now, UpdatedAt: now, Claims: claims, Evidence: evidence, Findings: []Finding{}, Experiments: []Experiment{}}
	v.Alternatives = append(v.Alternatives, a)
	v.UpdatedAt = now
	reopen(&v)
	s.compare(&v)
	return v, s.write(v)
}
func (s *Store) StartExperiment(repo, decision, alternative, actor string, x Experiment) (Decision, Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, Experiment{}, ErrNotFound
	}
	if x.WorkspaceID == "" || len(x.Revision) != 40 || x.CommandName == "" || x.DefinitionDigest == "" || x.DependencyDigest == "" {
		return Decision{}, Experiment{}, ErrInvalid
	}
	for i := range v.Alternatives {
		if v.Alternatives[i].ID == alternative && v.Alternatives[i].State == "active" {
			if x.ReproducesExperimentID != "" {
				found := false
				for _, prior := range v.Alternatives[i].Experiments {
					if prior.ID == x.ReproducesExperimentID && prior.Revision == x.Revision && prior.CommandName == x.CommandName {
						found = true
					}
				}
				if !found {
					return Decision{}, Experiment{}, ErrInvalid
				}
			}
			now := s.now().UTC()
			x.ID = newID()
			x.CreatedByID = actor
			x.State = "running"
			x.Checkpoints = []ExperimentCheckpoint{}
			x.CreatedAt = now
			x.UpdatedAt = now
			v.Alternatives[i].Experiments = append(v.Alternatives[i].Experiments, x)
			v.Alternatives[i].UpdatedAt = now
			v.UpdatedAt = now
			return v, x, s.write(v)
		}
	}
	return Decision{}, Experiment{}, ErrNotFound
}
func (s *Store) AddExperimentCheckpoint(repo, decision, alternative, experiment, actor string, c ExperimentCheckpoint) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	if c.WorkspaceCheckpointID == "" || strings.TrimSpace(c.Summary) == "" || len(c.Summary) > 4000 {
		return Decision{}, ErrInvalid
	}
	for _, m := range c.Measurements {
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Unit) == "" {
			return Decision{}, ErrInvalid
		}
	}
	for i := range v.Alternatives {
		if v.Alternatives[i].ID == alternative {
			for j := range v.Alternatives[i].Experiments {
				x := &v.Alternatives[i].Experiments[j]
				if x.ID == experiment {
					now := s.now().UTC()
					c.ID = newID()
					c.ActorID = actor
					c.CreatedAt = now
					x.Checkpoints = append(x.Checkpoints, c)
					x.State = "completed"
					x.UpdatedAt = now
					v.UpdatedAt = now
					reopen(&v)
					return v, s.write(v)
				}
			}
		}
	}
	return Decision{}, ErrNotFound
}
func (s *Store) AssessExperiment(repo, decision, alternative, experiment, actor, revision, dependencyDigest, environmentDigest string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	for i := range v.Alternatives {
		if v.Alternatives[i].ID == alternative {
			for j := range v.Alternatives[i].Experiments {
				x := &v.Alternatives[i].Experiments[j]
				if x.ID == experiment {
					reasons := []string{}
					if revision != x.Revision {
						reasons = append(reasons, "code_changed")
					}
					if dependencyDigest != x.DependencyDigest {
						reasons = append(reasons, "dependencies_changed")
					}
					if environmentDigest != x.DefinitionDigest {
						reasons = append(reasons, "environment_changed")
					}
					x.InvalidatedBy = reasons
					if len(reasons) > 0 {
						x.State = "invalidated"
					}
					x.UpdatedAt = s.now().UTC()
					v.UpdatedAt = x.UpdatedAt
					reopen(&v)
					return v, s.write(v)
				}
			}
		}
	}
	return Decision{}, ErrNotFound
}
func prepareClaims(v []Claim, actor string, now time.Time, existing map[string]bool) bool {
	if len(v) == 0 {
		return false
	}
	for i := range v {
		v[i].Kind = strings.TrimSpace(v[i].Kind)
		v[i].Body = strings.TrimSpace(v[i].Body)
		if !validClaim(v[i].Kind) || v[i].Body == "" || len(v[i].Body) > 4000 {
			return false
		}
		if v[i].SupersedesID != "" && (existing == nil || !existing[v[i].SupersedesID]) {
			return false
		}
		v[i].ID = newID()
		v[i].AuthorID = actor
		v[i].CreatedAt = now
	}
	return true
}
func prepareEvidence(v []Evidence) bool {
	for i := range v {
		v[i].Kind = strings.TrimSpace(v[i].Kind)
		v[i].Summary = strings.TrimSpace(v[i].Summary)
		if !validEvidence(v[i].Kind) || v[i].Summary == "" || v[i].ObservedAt.IsZero() {
			return false
		}
		v[i].ObservedAt = v[i].ObservedAt.UTC()
		if v[i].Kind == "code" && (v[i].RepositoryID == "" || v[i].Revision == "" || v[i].Path == "") {
			return false
		}
		if v[i].Kind != "code" && v[i].ResourceID == "" {
			return false
		}
	}
	return true
}
func (s *Store) AddClaims(repo, decision, alternative, actor string, claims []Claim, evidence []Evidence) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, ErrNotFound
	}
	now := s.now().UTC()
	for i := range v.Alternatives {
		a := &v.Alternatives[i]
		if a.ID != alternative {
			continue
		}
		ids := map[string]bool{}
		for _, c := range a.Claims {
			ids[c.ID] = true
		}
		if !prepareClaims(claims, actor, now, ids) || !prepareEvidence(evidence) {
			return Decision{}, ErrInvalid
		}
		a.Claims = append(a.Claims, claims...)
		a.Evidence = append(a.Evidence, evidence...)
		a.UpdatedAt = now
		v.UpdatedAt = now
		reopen(&v)
		s.compare(&v)
		return v, s.write(v)
	}
	return Decision{}, ErrNotFound
}
func (s *Store) StartResearch(repo, decision, alternative, actor string) (Decision, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, decision)
	if e != nil || !contains(v.Scope.ParticipantIDs, actor) {
		return Decision{}, "", ErrNotFound
	}
	found := false
	for _, a := range v.Alternatives {
		if a.ID == alternative && a.State == "active" {
			found = true
		}
	}
	if !found {
		return Decision{}, "", ErrNotFound
	}
	raw := newID() + newID()
	digest := sha256.Sum256([]byte(raw))
	key := hex.EncodeToString(digest[:])
	record := agentToken{decision, alternative, s.now().UTC().Add(24 * time.Hour)}
	s.tokens[key] = record
	dir := filepath.Join(s.root, ".agent-tokens")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return Decision{}, "", e
	}
	b, _ := json.Marshal(record)
	if e = os.WriteFile(filepath.Join(dir, key+".json"), b, 0600); e != nil {
		return Decision{}, "", e
	}
	return v, raw, nil
}
func (s *Store) token(token string) (agentToken, bool) {
	sum := sha256.Sum256([]byte(token))
	key := hex.EncodeToString(sum[:])
	r, ok := s.tokens[key]
	if !ok {
		b, e := os.ReadFile(filepath.Join(s.root, ".agent-tokens", key+".json"))
		if e != nil || json.Unmarshal(b, &r) != nil {
			return r, false
		}
	}
	if !s.now().UTC().Before(r.ExpiresAt) {
		return r, false
	}
	s.tokens[key] = r
	return r, true
}
func (s *Store) ResearchContext(token string) (Decision, Alternative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.token(token)
	if !ok {
		return Decision{}, Alternative{}, ErrNotFound
	}
	v, e := s.find(r.DecisionID)
	if e != nil {
		return Decision{}, Alternative{}, ErrNotFound
	}
	for _, a := range v.Alternatives {
		if a.ID == r.AlternativeID {
			return v, a, nil
		}
	}
	return Decision{}, Alternative{}, ErrNotFound
}
func (s *Store) AddFinding(token, body, uncertainty string, evidence []Evidence) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.token(token)
	if !ok {
		return Decision{}, ErrNotFound
	}
	v, e := s.find(r.DecisionID)
	if e != nil {
		return Decision{}, ErrNotFound
	}
	body = strings.TrimSpace(body)
	uncertainty = strings.TrimSpace(uncertainty)
	for i := range v.Alternatives {
		a := &v.Alternatives[i]
		if a.ID != r.AlternativeID {
			continue
		}
		canonical, valid := citedEvidence(evidence, a.Evidence)
		if body == "" || uncertainty == "" || !valid {
			return Decision{}, ErrInvalid
		}
		now := s.now().UTC()
		a.Findings = append(a.Findings, Finding{newID(), "codex", body, uncertainty, canonical, now})
		a.UpdatedAt = now
		v.UpdatedAt = now
		reopen(&v)
		s.compare(&v)
		return v, s.write(v)
	}
	return Decision{}, ErrNotFound
}
func citedEvidence(cited, retained []Evidence) ([]Evidence, bool) {
	if len(cited) == 0 {
		return nil, false
	}
	out := make([]Evidence, 0, len(cited))
	for _, c := range cited {
		var exact *Evidence
		for _, r := range retained {
			if c.Kind == r.Kind && c.RepositoryID == r.RepositoryID && c.Revision == r.Revision && c.ResourceID == r.ResourceID && c.Path == r.Path {
				copy := r
				exact = &copy
			}
		}
		if exact == nil {
			return nil, false
		}
		out = append(out, *exact)
	}
	return out, true
}
func (s *Store) find(id string) (Decision, error) {
	dirs, _ := os.ReadDir(s.root)
	for _, d := range dirs {
		if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			continue
		}
		if v, e := s.read(d.Name(), id); e == nil {
			return v, nil
		}
	}
	return Decision{}, ErrNotFound
}
func (s *Store) compare(v *Decision) {
	v.Comparison = []Comparison{}
	v.PendingApprovalIDs = []string{}
	v.Conflicts = []string{}
	for _, r := range v.ApprovalRequirements {
		if r.State == "pending" {
			v.PendingApprovalIDs = append(v.PendingApprovalIDs, r.ID)
		}
		if r.State == "rejected" {
			v.Conflicts = append(v.Conflicts, "approval_rejected:"+r.ID)
		}
	}
	for _, a := range v.Alternatives {
		c := Comparison{AlternativeID: a.ID, CurrentClaims: map[string]string{}, EvidenceCount: len(a.Evidence), EvidenceKinds: []string{}, StaleEvidenceIDs: []string{}}
		sup := map[string]bool{}
		for _, x := range a.Claims {
			if x.SupersedesID != "" {
				sup[x.SupersedesID] = true
			}
		}
		kinds := map[string]bool{}
		for _, x := range a.Claims {
			if !sup[x.ID] {
				if x.Kind == "dissent" {
					c.DissentCount++
				} else {
					c.CurrentClaims[x.Kind] = x.Body
				}
			}
		}
		for _, x := range criteria {
			if c.CurrentClaims[x] == "" {
				c.MissingCriteria = append(c.MissingCriteria, x)
			}
		}
		for _, e := range a.Evidence {
			kinds[e.Kind] = true
			if e.ObservedAt.Before(v.Scope.CreatedAt) {
				c.StaleEvidenceIDs = append(c.StaleEvidenceIDs, e.Kind+":"+e.ResourceID+e.Path)
			}
		}
		for _, x := range []string{"code", "dependency", "release", "incident", "usage"} {
			if kinds[x] {
				c.EvidenceKinds = append(c.EvidenceKinds, x)
			}
		}
		v.Comparison = append(v.Comparison, c)
	}
}

func reopen(v *Decision) {
	if v.State == "published" {
		v.State = "reopened"
	}
}

func (s *Store) RequestApproval(repo, id, actor, kind, target, policy string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if actor != v.Scope.OwnerID || !contains(v.Scope.ParticipantIDs, target) {
		return Decision{}, ErrNotFound
	}
	kind, policy = strings.TrimSpace(kind), strings.TrimSpace(policy)
	if (kind != "acknowledgement" && kind != "approval") || (kind == "approval" && policy == "") || len(policy) > 500 {
		return Decision{}, ErrInvalid
	}
	for _, r := range v.ApprovalRequirements {
		if r.ActorID == target && r.Kind == kind && r.Policy == policy && r.State == "pending" {
			return Decision{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	v.ApprovalRequirements = append(v.ApprovalRequirements, ApprovalRequirement{ID: newID(), Kind: kind, ActorID: target, Policy: policy, State: "pending", RequestedByID: actor, RequestedAt: now})
	v.UpdatedAt = now
	s.compare(&v)
	return v, s.write(v)
}

func (s *Store) RespondApproval(repo, id, requirement, actor, response, note string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	response, note = strings.TrimSpace(response), strings.TrimSpace(note)
	for i := range v.ApprovalRequirements {
		r := &v.ApprovalRequirements[i]
		if r.ID == requirement {
			if r.ActorID != actor || r.State != "pending" {
				return Decision{}, ErrNotFound
			}
			if (r.Kind == "acknowledgement" && response != "acknowledged") || (r.Kind == "approval" && response != "approved" && response != "rejected") || len(note) > 4000 {
				return Decision{}, ErrInvalid
			}
			now := s.now().UTC()
			r.State, r.Note, r.RespondedAt = response, note, &now
			v.UpdatedAt = now
			s.compare(&v)
			return v, s.write(v)
		}
	}
	return Decision{}, ErrNotFound
}

func (s *Store) Publish(repo, id, actor, selected string, rejected []string, rationale string, tradeoffs, dissent, conditions []string, reviewDate *time.Time, evidence []Evidence) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if actor != v.Scope.OwnerID || (v.State != "pending" && v.State != "reopened") {
		return Decision{}, ErrNotFound
	}
	rationale = strings.TrimSpace(rationale)
	tradeoffs, ok1 := cleanList(tradeoffs)
	dissent, ok2 := cleanList(dissent)
	conditions, ok3 := cleanList(conditions)
	if rationale == "" || len(rationale) > 10000 || !ok1 || !ok2 || !ok3 {
		return Decision{}, ErrInvalid
	}
	found := false
	rejectedSet := map[string]bool{}
	for _, x := range rejected {
		rejectedSet[x] = true
	}
	for _, a := range v.Alternatives {
		if a.ID == selected {
			found = true
		}
		if a.ID != selected && !rejectedSet[a.ID] {
			return Decision{}, ErrInvalid
		}
	}
	if !found || rejectedSet[selected] || len(rejectedSet) != len(v.Alternatives)-1 {
		return Decision{}, ErrInvalid
	}
	for _, r := range v.ApprovalRequirements {
		if r.State == "pending" || r.State == "rejected" {
			return Decision{}, ErrInvalid
		}
	}
	retained := []Evidence{}
	for _, a := range v.Alternatives {
		retained = append(retained, a.Evidence...)
		for _, f := range a.Findings {
			retained = append(retained, f.Evidence...)
		}
	}
	canonical, valid := citedEvidence(evidence, retained)
	if !valid {
		return Decision{}, ErrInvalid
	}
	if reviewDate != nil {
		d := reviewDate.UTC()
		if !d.After(s.now().UTC()) {
			return Decision{}, ErrInvalid
		}
		reviewDate = &d
	}
	now := s.now().UTC()
	c := Commitment{Version: len(v.Commitments) + 1, ScopeVersion: v.Scope.Version, SelectedAlternativeID: selected, RejectedAlternativeIDs: append([]string{}, rejected...), Rationale: rationale, AcceptedTradeoffs: tradeoffs, Dissent: dissent, Conditions: conditions, ReviewDate: reviewDate, Evidence: canonical, Approvals: append([]ApprovalRequirement{}, v.ApprovalRequirements...), PublishedByID: actor, PublishedAt: now}
	v.Commitments = append(v.Commitments, c)
	v.State = "published"
	for i := range v.Alternatives {
		if v.Alternatives[i].ID == selected {
			v.Alternatives[i].State = "selected"
		} else {
			v.Alternatives[i].State = "rejected"
		}
	}
	v.UpdatedAt = now
	s.compare(&v)
	return v, s.write(v)
}

func (s *Store) AuthorizeException(repo, id, actor string, in Exception) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if actor != v.Scope.OwnerID || len(v.Commitments) == 0 {
		return Decision{}, ErrNotFound
	}
	in.Scope, in.Reason = strings.TrimSpace(in.Scope), strings.TrimSpace(in.Reason)
	cs, ok := cleanList(in.Conditions)
	now := s.now().UTC()
	if in.Scope == "" || in.Reason == "" || len(in.Reason) > 4000 || !ok || !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(365*24*time.Hour)) {
		return Decision{}, ErrInvalid
	}
	in.ID = newID()
	in.CommitmentVersion = len(v.Commitments)
	in.Conditions = cs
	in.AuthorizedByID = actor
	in.StartsAt = now
	in.ExpiresAt = in.ExpiresAt.UTC()
	in.RevokedAt = nil
	v.Exceptions = append(v.Exceptions, in)
	v.UpdatedAt = now
	return v, s.write(v)
}

func (s *Store) RevokeException(repo, id, exception, actor string) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return Decision{}, e
	}
	if actor != v.Scope.OwnerID {
		return Decision{}, ErrNotFound
	}
	for i := range v.Exceptions {
		if v.Exceptions[i].ID == exception && v.Exceptions[i].RevokedAt == nil {
			now := s.now().UTC()
			v.Exceptions[i].RevokedAt = &now
			v.UpdatedAt = now
			return v, s.write(v)
		}
	}
	return Decision{}, ErrNotFound
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) read(repo, id string) (Decision, error) {
	var v Decision
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo {
		return Decision{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Decision) error {
	if err := os.MkdirAll(filepath.Join(s.root, v.RepositoryID), 0750); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := s.path(v.RepositoryID, v.ID) + ".tmp"
	if e = os.WriteFile(tmp, b, 0640); e != nil {
		return e
	}
	return os.Rename(tmp, s.path(v.RepositoryID, v.ID))
}
