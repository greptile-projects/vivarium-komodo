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
	Exceptions              []EvolutionException       `json:"exceptions"`
	Acknowledgements        []EvolutionAcknowledgement `json:"acknowledgements"`
	Findings                []EvolutionFinding         `json:"findings"`
	Analyses                []EvolutionAnalysis        `json:"analyses"`
	CreatedByID             string                     `json:"created_by_id"`
	CreatedAt               time.Time                  `json:"created_at"`
	UpdatedAt               time.Time                  `json:"updated_at"`
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
	v.Changes, v.Steps, v.Exceptions, v.Acknowledgements, v.Findings, v.Analyses = []CompatibilityChange{}, []MigrationStep{}, []EvolutionException{}, []EvolutionAcknowledgement{}, []EvolutionFinding{}, []EvolutionAnalysis{}
	return v, s.write("evolutions", v.ID, v)
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
