// Package incidents owns durable, attributable incident coordination records.
package incidents

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
	ErrNotFound   = errors.New("incident not found")
	ErrInvalid    = errors.New("invalid incident")
	ErrTransition = errors.New("invalid incident transition")
)

type AffectedEnvironment struct {
	RepositoryID  string `json:"repository_id"`
	EnvironmentID string `json:"environment_id"`
}
type SourceSignal struct {
	RepositoryID  string `json:"repository_id"`
	DeploymentID  string `json:"deployment_id"`
	EventSequence int64  `json:"event_sequence"`
	Stage         string `json:"stage,omitempty"`
	Signal        string `json:"signal,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
}
type Acknowledgement struct {
	ActorID        string    `json:"actor_id"`
	UpdateSequence int64     `json:"update_sequence"`
	CreatedAt      time.Time `json:"created_at"`
}

// Evidence is a durable pointer to operational or collaboration data. The
// source remains authoritative; CapturedAt and the optional time window make
// the responder's exact investigative context reproducible.
type Evidence struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	RepositoryID  string     `json:"repository_id"`
	ResourceID    string     `json:"resource_id"`
	EventSequence int64      `json:"event_sequence,omitempty"`
	StartAt       *time.Time `json:"start_at,omitempty"`
	EndAt         *time.Time `json:"end_at,omitempty"`
	Title         string     `json:"title"`
	Audience      string     `json:"audience"`
	AttachedByID  string     `json:"attached_by_id"`
	CapturedAt    time.Time  `json:"captured_at"`
}

// Finding records the team's reasoning separately from raw evidence. Sources
// are evidence IDs, so later readers can verify each claim without guessing.
type Finding struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Body        string    `json:"body"`
	Query       string    `json:"query,omitempty"`
	EvidenceIDs []string  `json:"evidence_ids"`
	Audience    string    `json:"audience"`
	AuthorID    string    `json:"author_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Revision struct {
	RepositoryID string `json:"repository_id"`
	CommitID     string `json:"commit_id"`
}
type OperationalAccess struct {
	RepositoryID string     `json:"repository_id"`
	Kind         string     `json:"kind"`
	ResourceID   string     `json:"resource_id"`
	StartAt      *time.Time `json:"start_at,omitempty"`
	EndAt        *time.Time `json:"end_at,omitempty"`
}
type InvestigationRecord struct {
	Sequence    int64     `json:"sequence"`
	Type        string    `json:"type"`
	ActorID     string    `json:"actor_id"`
	Message     string    `json:"message"`
	EvidenceIDs []string  `json:"evidence_ids,omitempty"`
	Uncertainty string    `json:"uncertainty,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
type Investigation struct {
	ID                  string                `json:"id"`
	Agent               string                `json:"agent"`
	Mandate             string                `json:"mandate"`
	EvidenceIDs         []string              `json:"evidence_ids"`
	Revisions           []Revision            `json:"revisions"`
	OperationalAccess   []OperationalAccess   `json:"operational_access"`
	Authority           []string              `json:"authority"`
	State               string                `json:"state"`
	InitiatedByID       string                `json:"initiated_by_id"`
	Records             []InvestigationRecord `json:"records"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	CredentialExpiresAt time.Time             `json:"credential_expires_at"`
	CredentialDigest    string                `json:"credential_digest,omitempty"`
}
type InvestigationInput struct {
	ActorID           string
	Agent, Mandate    string
	EvidenceIDs       []string
	Revisions         []Revision
	OperationalAccess []OperationalAccess
}
type Event struct {
	Sequence       int64                 `json:"sequence"`
	Type           string                `json:"type"`
	ActorID        string                `json:"actor_id"`
	Status         string                `json:"status,omitempty"`
	Severity       string                `json:"severity,omitempty"`
	Audience       string                `json:"audience,omitempty"`
	Message        string                `json:"message,omitempty"`
	Roles          map[string]string     `json:"roles,omitempty"`
	Affected       []AffectedEnvironment `json:"affected,omitempty"`
	UpdateSequence int64                 `json:"update_sequence,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
}
type Incident struct {
	ID               string                `json:"id"`
	RepositoryID     string                `json:"repository_id"`
	Title            string                `json:"title"`
	Summary          string                `json:"summary"`
	Severity         string                `json:"severity"`
	Status           string                `json:"status"`
	DeclaredByID     string                `json:"declared_by_id"`
	Roles            map[string]string     `json:"roles"`
	Affected         []AffectedEnvironment `json:"affected"`
	SourceSignal     *SourceSignal         `json:"source_signal,omitempty"`
	Followers        []string              `json:"followers"`
	Acknowledgements []Acknowledgement     `json:"acknowledgements"`
	Evidence         []Evidence            `json:"evidence"`
	Findings         []Finding             `json:"findings"`
	Investigations   []Investigation       `json:"investigations"`
	Timeline         []Event               `json:"timeline"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}
type CreateInput struct {
	RepositoryID, ActorID, Title, Summary, Severity string
	Roles                                           map[string]string
	Affected                                        []AffectedEnvironment
	SourceSignal                                    *SourceSignal
}
type UpdateInput struct {
	ActorID, Summary, Severity, Status string
	Roles                              map[string]string
	Affected                           []AffectedEnvironment
}

type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0750); err != nil {
		return nil, err
	}
	return &Store{root: abs, now: time.Now}, nil
}
func (s *Store) Create(in CreateInput) (Incident, error) {
	in.Title, in.Summary = strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary)
	in.Severity = strings.ToLower(strings.TrimSpace(in.Severity))
	if in.RepositoryID == "" || in.ActorID == "" || in.Title == "" || len(in.Title) > 200 || in.Summary == "" || len(in.Summary) > 10000 || !validSeverity(in.Severity) || !validRoles(in.Roles) || len(in.Affected) == 0 || !validAffected(in.Affected) {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Incident{}, err
	}
	now := s.now().UTC()
	i := Incident{ID: id, RepositoryID: in.RepositoryID, Title: in.Title, Summary: in.Summary, Severity: in.Severity, Status: "declared", DeclaredByID: in.ActorID, Roles: copyRoles(in.Roles), Affected: copyAffected(in.Affected), SourceSignal: in.SourceSignal, Followers: []string{in.ActorID}, Acknowledgements: []Acknowledgement{}, Evidence: []Evidence{}, Findings: []Finding{}, Investigations: []Investigation{}, Timeline: []Event{}, CreatedAt: now, UpdatedAt: now}
	i.append(Event{Type: "declared", ActorID: in.ActorID, Status: i.Status, Severity: i.Severity, Message: i.Summary, Roles: copyRoles(i.Roles), Affected: copyAffected(i.Affected), CreatedAt: now})
	return i, s.write(i)
}
func (s *Store) Get(repositoryID, id string) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repositoryID, id)
}
func (s *Store) List(repositoryID string) ([]Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(filepath.Join(s.root, repositoryID))
	if errors.Is(err, fs.ErrNotExist) {
		return []Incident{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Incident{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		v, er := s.read(repositoryID, strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, v)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].UpdatedAt.After(out[b].UpdatedAt) })
	return out, nil
}
func (s *Store) Update(repositoryID, id string, in UpdateInput) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	if i.Status == "resolved" {
		return i, ErrTransition
	}
	now := s.now().UTC()
	changed := false
	e := Event{Type: "coordination.updated", ActorID: in.ActorID, CreatedAt: now}
	if in.Summary != "" {
		in.Summary = strings.TrimSpace(in.Summary)
		if len(in.Summary) > 10000 {
			return i, ErrInvalid
		}
		i.Summary = in.Summary
		e.Message = in.Summary
		changed = true
	}
	if in.Severity != "" {
		in.Severity = strings.ToLower(in.Severity)
		if !validSeverity(in.Severity) {
			return i, ErrInvalid
		}
		i.Severity = in.Severity
		e.Severity = in.Severity
		changed = true
	}
	if in.Status != "" {
		if !validStatus(in.Status) || !statusTransition(i.Status, in.Status) {
			return i, ErrTransition
		}
		i.Status = in.Status
		e.Status = in.Status
		changed = true
	}
	if in.Roles != nil {
		if !validRoles(in.Roles) {
			return i, ErrInvalid
		}
		i.Roles = copyRoles(in.Roles)
		e.Roles = copyRoles(in.Roles)
		changed = true
	}
	if in.Affected != nil {
		if len(in.Affected) == 0 || !validAffected(in.Affected) {
			return i, ErrInvalid
		}
		i.Affected = copyAffected(in.Affected)
		e.Affected = copyAffected(in.Affected)
		changed = true
	}
	if !changed {
		return i, ErrInvalid
	}
	if e.Status == "resolved" {
		e.Type = "resolved"
	}
	i.UpdatedAt = now
	i.append(e)
	return i, s.write(i)
}
func (s *Store) AddUpdate(repositoryID, id, actor, audience, message string) (Incident, error) {
	audience = strings.ToLower(strings.TrimSpace(audience))
	message = strings.TrimSpace(message)
	if actor == "" || (audience != "participants" && audience != "public") || message == "" || len(message) > 10000 {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	if i.Status == "resolved" {
		return i, ErrTransition
	}
	now := s.now().UTC()
	i.UpdatedAt = now
	i.append(Event{Type: "update", ActorID: actor, Audience: audience, Message: message, CreatedAt: now})
	return i, s.write(i)
}
func (s *Store) Follow(repositoryID, id, actor string, following bool) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	found := -1
	for x, v := range i.Followers {
		if v == actor {
			found = x
		}
	}
	if following && found < 0 {
		i.Followers = append(i.Followers, actor)
		sort.Strings(i.Followers)
	} else if !following && found >= 0 {
		i.Followers = append(i.Followers[:found], i.Followers[found+1:]...)
	} else {
		return i, ErrTransition
	}
	now := s.now().UTC()
	i.UpdatedAt = now
	typ := "followed"
	if !following {
		typ = "unfollowed"
	}
	i.append(Event{Type: typ, ActorID: actor, CreatedAt: now})
	return i, s.write(i)
}
func (s *Store) Acknowledge(repositoryID, id, actor string, sequence int64) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	valid := false
	for _, e := range i.Timeline {
		if e.Sequence == sequence && e.Type == "update" {
			valid = true
		}
	}
	if !valid {
		return i, ErrInvalid
	}
	for _, a := range i.Acknowledgements {
		if a.ActorID == actor && a.UpdateSequence == sequence {
			return i, ErrTransition
		}
	}
	now := s.now().UTC()
	i.Acknowledgements = append(i.Acknowledgements, Acknowledgement{actor, sequence, now})
	i.UpdatedAt = now
	i.append(Event{Type: "acknowledged", ActorID: actor, UpdateSequence: sequence, CreatedAt: now})
	return i, s.write(i)
}
func (s *Store) AddEvidence(repositoryID, id, actor string, evidence Evidence) (Incident, error) {
	evidence.Kind, evidence.Title, evidence.Audience = strings.ToLower(strings.TrimSpace(evidence.Kind)), strings.TrimSpace(evidence.Title), strings.ToLower(strings.TrimSpace(evidence.Audience))
	if actor == "" || evidence.RepositoryID == "" || evidence.ResourceID == "" || evidence.Title == "" || len(evidence.Title) > 300 || !validEvidenceKind(evidence.Kind) || !validAudience(evidence.Audience) || (evidence.StartAt != nil && evidence.EndAt != nil && evidence.EndAt.Before(*evidence.StartAt)) {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	if i.Status == "resolved" {
		return i, ErrTransition
	}
	evidence.ID, err = newID()
	if err != nil {
		return i, err
	}
	evidence.AttachedByID, evidence.CapturedAt = actor, s.now().UTC()
	i.Evidence = append(i.Evidence, evidence)
	i.UpdatedAt = evidence.CapturedAt
	i.append(Event{Type: "evidence.attached", ActorID: actor, Audience: evidence.Audience, Message: evidence.Title, CreatedAt: evidence.CapturedAt})
	return i, s.write(i)
}
func (s *Store) AddFinding(repositoryID, id, actor string, finding Finding) (Incident, error) {
	finding.Kind, finding.Body, finding.Query, finding.Audience = strings.ToLower(strings.TrimSpace(finding.Kind)), strings.TrimSpace(finding.Body), strings.TrimSpace(finding.Query), strings.ToLower(strings.TrimSpace(finding.Audience))
	if actor == "" || !validFindingKind(finding.Kind) || finding.Body == "" || len(finding.Body) > 10000 || len(finding.Query) > 4000 || !validAudience(finding.Audience) {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	if i.Status == "resolved" {
		return i, ErrTransition
	}
	seen := map[string]bool{}
	for _, e := range i.Evidence {
		seen[e.ID] = true
	}
	for _, source := range finding.EvidenceIDs {
		if !seen[source] {
			return i, ErrInvalid
		}
	}
	finding.ID, err = newID()
	if err != nil {
		return i, err
	}
	finding.AuthorID, finding.CreatedAt = actor, s.now().UTC()
	finding.EvidenceIDs = append([]string{}, finding.EvidenceIDs...)
	i.Findings = append(i.Findings, finding)
	i.UpdatedAt = finding.CreatedAt
	i.append(Event{Type: "investigation." + finding.Kind, ActorID: actor, Audience: finding.Audience, Message: finding.Body, CreatedAt: finding.CreatedAt})
	return i, s.write(i)
}
func (s *Store) StartInvestigation(repositoryID, id string, in InvestigationInput) (Incident, string, error) {
	in.Agent, in.Mandate = strings.TrimSpace(in.Agent), strings.TrimSpace(in.Mandate)
	if in.ActorID == "" || in.Agent == "" || len(in.Agent) > 100 || in.Mandate == "" || len(in.Mandate) > 10000 || len(in.EvidenceIDs) == 0 || len(in.Revisions) == 0 || len(in.OperationalAccess) == 0 {
		return Incident{}, "", ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, "", err
	}
	if i.Status == "resolved" {
		return i, "", ErrTransition
	}
	seen := map[string]bool{}
	for _, e := range i.Evidence {
		seen[e.ID] = true
	}
	for _, eid := range in.EvidenceIDs {
		if !seen[eid] {
			return i, "", ErrInvalid
		}
	}
	for _, a := range in.OperationalAccess {
		if a.RepositoryID == "" || a.ResourceID == "" || (a.Kind != "deployment_logs" && a.Kind != "health_signals") || (a.StartAt != nil && a.EndAt != nil && a.EndAt.Before(*a.StartAt)) {
			return i, "", ErrInvalid
		}
	}
	for _, rev := range in.Revisions {
		if rev.RepositoryID == "" || rev.CommitID == "" {
			return i, "", ErrInvalid
		}
	}
	sid, err := newID()
	if err != nil {
		return i, "", err
	}
	token, err := newToken()
	if err != nil {
		return i, "", err
	}
	now := s.now().UTC()
	investigation := Investigation{ID: sid, Agent: in.Agent, Mandate: in.Mandate, EvidenceIDs: append([]string{}, in.EvidenceIDs...), Revisions: append([]Revision{}, in.Revisions...), OperationalAccess: append([]OperationalAccess{}, in.OperationalAccess...), Authority: []string{"incident:read", "evidence:read", "deployment_logs:read", "health_signals:read"}, State: "running", InitiatedByID: in.ActorID, Records: []InvestigationRecord{}, CreatedAt: now, UpdatedAt: now, CredentialExpiresAt: now.Add(24 * time.Hour), CredentialDigest: digestToken(token)}
	i.Investigations = append(i.Investigations, investigation)
	i.UpdatedAt = now
	i.append(Event{Type: "investigation.delegated", ActorID: in.ActorID, Audience: "participants", Message: in.Mandate, CreatedAt: now})
	return i, token, s.write(i)
}
func (s *Store) InvestigationContext(token string) (Incident, Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findInvestigation(token)
}
func (s *Store) AddInvestigationRecord(token, typ, message, uncertainty string, evidenceIDs []string) (Incident, Investigation, error) {
	typ, message, uncertainty = strings.ToLower(strings.TrimSpace(typ)), strings.TrimSpace(message), strings.TrimSpace(uncertainty)
	if (typ != "finding" && typ != "tool" && typ != "question" && typ != "uncertainty") || message == "" || len(message) > 10000 || len(uncertainty) > 2000 {
		return Incident{}, Investigation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, inv, err := s.findInvestigation(token)
	if err != nil {
		return i, inv, err
	}
	if inv.State != "running" {
		return i, inv, ErrTransition
	}
	allowed := map[string]bool{}
	for _, id := range inv.EvidenceIDs {
		allowed[id] = true
	}
	for _, id := range evidenceIDs {
		if !allowed[id] {
			return i, inv, ErrInvalid
		}
	}
	now := s.now().UTC()
	rec := InvestigationRecord{Sequence: int64(len(inv.Records) + 1), Type: typ, ActorID: "agent:" + inv.Agent, Message: message, EvidenceIDs: append([]string{}, evidenceIDs...), Uncertainty: uncertainty, CreatedAt: now}
	inv.Records = append(inv.Records, rec)
	inv.UpdatedAt = now
	for x := range i.Investigations {
		if i.Investigations[x].ID == inv.ID {
			i.Investigations[x] = inv
		}
	}
	i.UpdatedAt = now
	i.append(Event{Type: "agent." + typ, ActorID: rec.ActorID, Audience: "participants", Message: message, CreatedAt: now})
	err = s.write(i)
	return i, inv, err
}
func (s *Store) ControlInvestigation(repositoryID, id, session, actor, action, message string) (Incident, error) {
	action, message = strings.ToLower(strings.TrimSpace(action)), strings.TrimSpace(message)
	if actor == "" || (action != "guide" && action != "pause" && action != "resume" && action != "cancel") || (action == "guide" && message == "") {
		return Incident{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	i, err := s.read(repositoryID, id)
	if err != nil {
		return i, err
	}
	now := s.now().UTC()
	for x := range i.Investigations {
		inv := &i.Investigations[x]
		if inv.ID != session {
			continue
		}
		if inv.State == "cancelled" {
			return i, ErrTransition
		}
		switch action {
		case "pause":
			if inv.State != "running" {
				return i, ErrTransition
			}
			inv.State = "paused"
		case "resume":
			if inv.State != "paused" {
				return i, ErrTransition
			}
			inv.State = "running"
		case "cancel":
			inv.State = "cancelled"
			inv.CredentialDigest = ""
		case "guide":
		}
		inv.Records = append(inv.Records, InvestigationRecord{Sequence: int64(len(inv.Records) + 1), Type: action, ActorID: actor, Message: message, CreatedAt: now})
		inv.UpdatedAt = now
		i.UpdatedAt = now
		i.append(Event{Type: "investigation." + action, ActorID: actor, Audience: "participants", Message: message, CreatedAt: now})
		return i, s.write(i)
	}
	return i, ErrNotFound
}
func (s *Store) findInvestigation(token string) (Incident, Investigation, error) {
	d := digestToken(token)
	entries, _ := os.ReadDir(s.root)
	for _, repo := range entries {
		if !repo.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(s.root, repo.Name()))
		for _, f := range files {
			if filepath.Ext(f.Name()) != ".json" {
				continue
			}
			i, err := s.read(repo.Name(), strings.TrimSuffix(f.Name(), ".json"))
			if err != nil {
				continue
			}
			for _, inv := range i.Investigations {
				if inv.CredentialDigest == d && d != "" {
					if s.now().UTC().After(inv.CredentialExpiresAt) || inv.State == "cancelled" {
						return i, inv, ErrTransition
					}
					return i, inv, nil
				}
			}
		}
	}
	return Incident{}, Investigation{}, ErrNotFound
}
func newToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func digestToken(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func (i *Incident) append(e Event) {
	e.Sequence = int64(len(i.Timeline) + 1)
	i.Timeline = append(i.Timeline, e)
}
func (s *Store) read(repo, id string) (Incident, error) {
	b, err := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Incident{}, ErrNotFound
	}
	if err != nil {
		return Incident{}, err
	}
	var i Incident
	if json.Unmarshal(b, &i) != nil || i.ID != id || i.RepositoryID != repo {
		return Incident{}, ErrInvalid
	}
	return i, nil
}
func (s *Store) write(i Incident) error {
	dir := filepath.Join(s.root, i.RepositoryID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".incident-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0640); err == nil {
		_, err = tmp.Write(b)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(dir, i.ID+".json"))
	}
	return err
}
func newID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func validSeverity(v string) bool {
	return v == "critical" || v == "high" || v == "medium" || v == "low"
}
func validAudience(v string) bool { return v == "participants" || v == "public" }
func validEvidenceKind(v string) bool {
	return v == "logs" || v == "health_signal" || v == "deployment" || v == "release" || v == "commit" || v == "pull_request" || v == "incident"
}
func validFindingKind(v string) bool {
	return v == "observation" || v == "hypothesis" || v == "query" || v == "conclusion"
}
func validStatus(v string) bool {
	return v == "declared" || v == "investigating" || v == "mitigating" || v == "monitoring" || v == "resolved"
}
func statusTransition(a, b string) bool {
	if a == b {
		return false
	}
	order := map[string]int{"declared": 0, "investigating": 1, "mitigating": 2, "monitoring": 3, "resolved": 4}
	return order[b] >= order[a] || b == "investigating" || b == "mitigating"
}
func validRoles(r map[string]string) bool {
	if r == nil || strings.TrimSpace(r["commander"]) == "" {
		return false
	}
	for k, v := range r {
		if k != "commander" && k != "operations" && k != "communications" {
			return false
		}
		if strings.TrimSpace(v) == "" {
			return false
		}
	}
	return true
}
func validAffected(a []AffectedEnvironment) bool {
	seen := map[string]bool{}
	for _, v := range a {
		if strings.TrimSpace(v.RepositoryID) == "" {
			return false
		}
		k := v.RepositoryID + "\x00" + v.EnvironmentID
		if seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}
func copyRoles(v map[string]string) map[string]string {
	o := map[string]string{}
	for k, x := range v {
		o[k] = x
	}
	return o
}
func copyAffected(v []AffectedEnvironment) []AffectedEnvironment {
	return append([]AffectedEnvironment{}, v...)
}
