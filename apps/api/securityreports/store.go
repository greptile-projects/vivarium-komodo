// Package securityreports owns private vulnerability reports and their access ledger.
package securityreports

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
	ErrNotFound   = errors.New("security report not found")
	ErrInvalid    = errors.New("invalid security report")
	ErrConflict   = errors.New("security report conflict")
	ErrTransition = errors.New("invalid security investigation transition")
)

type AffectedRepository struct {
	RepositoryID string   `json:"repository_id"`
	Versions     []string `json:"versions"`
}
type Evidence struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}
type ResourceLink struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	RepositoryID string    `json:"repository_id"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Revision     string    `json:"revision,omitempty"`
	Label        string    `json:"label"`
	Details      string    `json:"details,omitempty"`
	ActorID      string    `json:"actor_id"`
	CreatedAt    time.Time `json:"created_at"`
}
type Finding struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Body        string    `json:"body"`
	ActorID     string    `json:"actor_id"`
	EvidenceIDs []string  `json:"evidence_ids"`
	CreatedAt   time.Time `json:"created_at"`
}
type Impact struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Version      string    `json:"version"`
	Environment  string    `json:"environment"`
	State        string    `json:"state"`
	Rationale    string    `json:"rationale"`
	ActorID      string    `json:"actor_id"`
	EvidenceIDs  []string  `json:"evidence_ids"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type InvestigationRecord struct {
	Sequence    int64     `json:"sequence"`
	Type        string    `json:"type"`
	ActorID     string    `json:"actor_id"`
	Body        string    `json:"body"`
	Uncertainty string    `json:"uncertainty,omitempty"`
	EvidenceIDs []string  `json:"evidence_ids"`
	CreatedAt   time.Time `json:"created_at"`
}
type Investigation struct {
	ID                  string                `json:"id"`
	Agent               string                `json:"agent"`
	Mandate             string                `json:"mandate"`
	State               string                `json:"state"`
	InitiatedByID       string                `json:"initiated_by_id"`
	EvidenceIDs         []string              `json:"evidence_ids"`
	Authority           []string              `json:"authority"`
	Records             []InvestigationRecord `json:"records"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
	CredentialExpiresAt time.Time             `json:"credential_expires_at"`
	CredentialDigest    string                `json:"credential_digest,omitempty"`
}
type RepairRecord struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Body      string    `json:"body"`
	Revision  string    `json:"revision,omitempty"`
	Decision  string    `json:"decision,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type RepairSession struct {
	ID                  string         `json:"id"`
	Kind                string         `json:"kind"`
	AssigneeID          string         `json:"assignee_id"`
	Mandate             string         `json:"mandate"`
	State               string         `json:"state"`
	Authority           []string       `json:"authority"`
	InitiatedByID       string         `json:"initiated_by_id"`
	CredentialName      string         `json:"credential_name,omitempty"`
	CredentialExpiresAt time.Time      `json:"credential_expires_at"`
	Records             []RepairRecord `json:"records"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// Verification is the response-team-safe evidence summary for one exact repair
// candidate. Commands, reproduction inputs, and execution logs deliberately do
// not belong in this record; those stay in the private runner identified by an
// opaque attempt ID and definition digest.
type VerificationGate struct {
	Kind             string    `json:"kind"`
	Name             string    `json:"name"`
	AttemptID        string    `json:"attempt_id"`
	DefinitionDigest string    `json:"definition_digest"`
	State            string    `json:"state"`
	RecordedByID     string    `json:"recorded_by_id"`
	RecordedAt       time.Time `json:"recorded_at"`
}
type VerificationApproval struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}
type ReleaseAttestation struct {
	ReleaseID      string    `json:"release_id"`
	Version        string    `json:"version"`
	ArtifactID     string    `json:"artifact_id"`
	ArtifactSHA256 string    `json:"artifact_sha256"`
	RecordedByID   string    `json:"recorded_by_id"`
	RecordedAt     time.Time `json:"recorded_at"`
}
type RepairVerification struct {
	Revision            string                 `json:"revision"`
	State               string                 `json:"state"`
	Gates               []VerificationGate     `json:"gates"`
	Approvals           []VerificationApproval `json:"approvals"`
	IntegrationEntryID  string                 `json:"integration_entry_id,omitempty"`
	IntegrationCommitID string                 `json:"integration_commit_id,omitempty"`
	ReleaseAttestations []ReleaseAttestation   `json:"release_attestations"`
	RemainingGaps       []string               `json:"remaining_gaps"`
	UpdatedAt           time.Time              `json:"updated_at"`
}
type RepairTask struct {
	ID            string              `json:"id"`
	RepositoryID  string              `json:"repository_id"`
	Version       string              `json:"version"`
	Outcome       string              `json:"outcome"`
	BaseRevision  string              `json:"base_revision"`
	Branch        string              `json:"branch"`
	DependencyIDs []string            `json:"dependency_ids"`
	State         string              `json:"state"`
	CreatedByID   string              `json:"created_by_id"`
	Sessions      []RepairSession     `json:"sessions"`
	Verification  *RepairVerification `json:"verification,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}
type DisclosureBranch struct {
	RepairID       string `json:"repair_id"`
	RepositoryID   string `json:"repository_id"`
	Version        string `json:"version"`
	SourceRef      string `json:"source_ref,omitempty"`
	PublishedRef   string `json:"published_ref"`
	Revision       string `json:"revision"`
	ReleaseID      string `json:"release_id"`
	ArtifactID     string `json:"artifact_id"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	State          string `json:"state"`
	Error          string `json:"error,omitempty"`
}
type Disclosure struct {
	State           string             `json:"state"`
	AdvisoryID      string             `json:"advisory_id"`
	Summary         string             `json:"summary"`
	UpgradeGuidance string             `json:"upgrade_guidance"`
	Credits         []string           `json:"credits"`
	ScheduledAt     *time.Time         `json:"scheduled_at,omitempty"`
	PublishedAt     *time.Time         `json:"published_at,omitempty"`
	PublishedByID   string             `json:"published_by_id,omitempty"`
	Remaining       []string           `json:"remaining"`
	Branches        []DisclosureBranch `json:"branches"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
type Contact struct {
	Channel string `json:"channel"`
	Value   string `json:"value"`
}
type TeamMember struct {
	UserID      string    `json:"user_id"`
	InvitedByID string    `json:"invited_by_id"`
	InvitedAt   time.Time `json:"invited_at"`
}
type Message struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type AuditEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	SubjectID string    `json:"subject_id,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Report struct {
	ID             string               `json:"id"`
	Title          string               `json:"title"`
	Summary        string               `json:"summary"`
	ReporterID     string               `json:"reporter_id"`
	Contact        Contact              `json:"contact"`
	Affected       []AffectedRepository `json:"affected_repositories"`
	Evidence       []Evidence           `json:"evidence"`
	Severity       string               `json:"severity"`
	EmbargoState   string               `json:"embargo_state"`
	Team           []TeamMember         `json:"response_team"`
	Messages       []Message            `json:"messages"`
	Audit          []AuditEvent         `json:"audit_log"`
	ResourceLinks  []ResourceLink       `json:"resource_links"`
	Findings       []Finding            `json:"findings"`
	ImpactMatrix   []Impact             `json:"impact_matrix"`
	Investigations []Investigation      `json:"investigations"`
	Repairs        []RepairTask         `json:"repairs"`
	Disclosure     *Disclosure          `json:"disclosure,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
type CreateInput struct {
	ActorID, Title, Summary string
	Contact                 Contact
	Affected                []AffectedRepository
	Evidence                []Evidence
}
type TriageInput struct{ ActorID, Severity, EmbargoState string }
type VerificationAction struct {
	ActorID             string `json:"-"`
	Action              string `json:"action"`
	Revision            string `json:"revision"`
	Kind                string `json:"kind"`
	Name                string `json:"name"`
	AttemptID           string `json:"attempt_id"`
	DefinitionDigest    string `json:"definition_digest"`
	State               string `json:"state"`
	Decision            string `json:"decision"`
	Summary             string `json:"summary"`
	IntegrationEntryID  string `json:"integration_entry_id"`
	IntegrationCommitID string `json:"integration_commit_id"`
	ReleaseID           string `json:"release_id"`
	Version             string `json:"version"`
	ArtifactID          string `json:"artifact_id"`
	ArtifactSHA256      string `json:"artifact_sha256"`
	Gap                 string `json:"gap"`
}
type DisclosureInput struct {
	ActorID, AdvisoryID, Summary, UpgradeGuidance string
	Credits                                       []string
	ScheduledAt                                   *time.Time
	PublishedRefs                                 map[string]string
}

// PrepareDisclosure freezes the public payload and proves every supported line
// has an integrated, attested repair before any ordinary ref can be exposed.
func (s *Store) PrepareDisclosure(id string, in DisclosureInput, maintainer func(string) bool) (Report, error) {
	in.AdvisoryID, in.Summary, in.UpgradeGuidance = strings.TrimSpace(in.AdvisoryID), strings.TrimSpace(in.Summary), strings.TrimSpace(in.UpgradeGuidance)
	if in.ActorID == "" || in.AdvisoryID == "" || in.Summary == "" || in.UpgradeGuidance == "" || len(in.Summary) > 20000 || len(in.UpgradeGuidance) > 20000 || len(in.Credits) > 50 {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, in.ActorID, maintainer) {
		return Report{}, ErrNotFound
	}
	if r.EmbargoState == "lifted" || r.Disclosure != nil || len(r.Repairs) == 0 {
		return r, ErrTransition
	}
	branches := make([]DisclosureBranch, 0, len(r.Repairs))
	seen := map[string]bool{}
	for _, repair := range r.Repairs {
		v := repair.Verification
		published := strings.TrimSpace(in.PublishedRefs[repair.ID])
		if v == nil || v.State != "attested" || len(v.ReleaseAttestations) == 0 || !strings.HasPrefix(published, "refs/heads/") || strings.HasPrefix(published, "refs/heads/embargo/") || seen[repair.RepositoryID+"\x00"+published] {
			return r, ErrInvalid
		}
		seen[repair.RepositoryID+"\x00"+published] = true
		a := v.ReleaseAttestations[len(v.ReleaseAttestations)-1]
		branches = append(branches, DisclosureBranch{RepairID: repair.ID, RepositoryID: repair.RepositoryID, Version: repair.Version, SourceRef: repair.Branch, PublishedRef: published, Revision: v.IntegrationCommitID, ReleaseID: a.ReleaseID, ArtifactID: a.ArtifactID, ArtifactSHA256: a.ArtifactSHA256, State: "pending"})
	}
	now := s.now().UTC()
	state := "ready"
	if in.ScheduledAt != nil && in.ScheduledAt.After(now) {
		state = "scheduled"
	}
	r.Disclosure = &Disclosure{State: state, AdvisoryID: in.AdvisoryID, Summary: in.Summary, UpgradeGuidance: in.UpgradeGuidance, Credits: append([]string{}, in.Credits...), ScheduledAt: in.ScheduledAt, Remaining: []string{"publish_repaired_branches", "publish_advisory", "notify_affected_owners"}, Branches: branches, UpdatedAt: now}
	r.UpdatedAt = now
	r.append("disclosure.prepared", in.ActorID, in.AdvisoryID, state, now)
	return r, s.write(r)
}

func (s *Store) CompleteDisclosure(id, actor string, branches []DisclosureBranch, failure string, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if r.Disclosure == nil || r.Disclosure.State == "published" {
		return r, ErrTransition
	}
	now := s.now().UTC()
	r.Disclosure.Branches, r.Disclosure.UpdatedAt = branches, now
	if failure != "" {
		r.Disclosure.State = "paused"
		r.Disclosure.Remaining = []string{"retry_branch_publication", "publish_advisory", "notify_affected_owners"}
		r.append("disclosure.paused", actor, r.Disclosure.AdvisoryID, failure, now)
	} else {
		r.Disclosure.State = "published"
		r.Disclosure.Remaining = []string{}
		r.Disclosure.PublishedAt = &now
		r.Disclosure.PublishedByID = actor
		r.EmbargoState = "lifted"
		r.append("disclosure.published", actor, r.Disclosure.AdvisoryID, "fixes and guidance public", now)
	}
	r.UpdatedAt = now
	return r, s.write(r)
}

func (s *Store) GetPublicAdvisory(advisoryID string) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, _ := os.ReadDir(s.root)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		r, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err == nil && r.Disclosure != nil && r.Disclosure.AdvisoryID == advisoryID && r.Disclosure.State == "published" {
			r.Summary = ""
			r.ReporterID = ""
			r.Contact = Contact{}
			r.Evidence = nil
			r.Team = nil
			r.Messages = nil
			r.Audit = nil
			r.ResourceLinks = nil
			r.Findings = nil
			r.Investigations = nil
			r.Repairs = nil
			for i := range r.Disclosure.Branches {
				r.Disclosure.Branches[i].SourceRef = ""
				r.Disclosure.Branches[i].Error = ""
			}
			return r, nil
		}
	}
	return Report{}, ErrNotFound
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

func (s *Store) Create(in CreateInput) (Report, error) {
	in.Title, in.Summary = strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary)
	in.Contact.Channel, in.Contact.Value = strings.TrimSpace(in.Contact.Channel), strings.TrimSpace(in.Contact.Value)
	if in.ActorID == "" || in.Title == "" || len(in.Title) > 200 || in.Summary == "" || len(in.Summary) > 20000 || in.Contact.Channel == "" || len(in.Contact.Channel) > 80 || in.Contact.Value == "" || len(in.Contact.Value) > 500 || len(in.Affected) == 0 || len(in.Affected) > 20 || len(in.Evidence) > 50 {
		return Report{}, ErrInvalid
	}
	seen := map[string]bool{}
	for x := range in.Affected {
		a := &in.Affected[x]
		a.RepositoryID = strings.TrimSpace(a.RepositoryID)
		if a.RepositoryID == "" || seen[a.RepositoryID] || len(a.Versions) == 0 || len(a.Versions) > 50 {
			return Report{}, ErrInvalid
		}
		seen[a.RepositoryID] = true
		for y := range a.Versions {
			a.Versions[y] = strings.TrimSpace(a.Versions[y])
			if a.Versions[y] == "" || len(a.Versions[y]) > 200 {
				return Report{}, ErrInvalid
			}
		}
	}
	for x := range in.Evidence {
		e := &in.Evidence[x]
		e.Title = strings.TrimSpace(e.Title)
		e.Kind = strings.ToLower(strings.TrimSpace(e.Kind))
		e.Description = strings.TrimSpace(e.Description)
		if e.Title == "" || len(e.Title) > 300 || !validEvidenceKind(e.Kind) || e.Description == "" || len(e.Description) > 20000 {
			return Report{}, ErrInvalid
		}
		if e.ID == "" {
			e.ID, _ = newID()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Report{}, err
	}
	now := s.now().UTC()
	r := Report{ID: id, Title: in.Title, Summary: in.Summary, ReporterID: in.ActorID, Contact: in.Contact, Affected: in.Affected, Evidence: in.Evidence, Severity: "unknown", EmbargoState: "requested", Team: []TeamMember{}, Messages: []Message{}, Audit: []AuditEvent{}, ResourceLinks: []ResourceLink{}, Findings: []Finding{}, ImpactMatrix: []Impact{}, Investigations: []Investigation{}, Repairs: []RepairTask{}, CreatedAt: now, UpdatedAt: now}
	r.append("report.created", in.ActorID, "", "private report submitted", now)
	return r, s.write(r)
}
func (s *Store) ListVisible(actor string, maintainer func(string) bool) ([]Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	out := []Report{}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		r, er := s.read(strings.TrimSuffix(e.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		if canAccess(r, actor, maintainer) {
			r.Summary = ""
			r.Contact = Contact{}
			r.Evidence = nil
			r.Messages = nil
			r.Audit = nil
			r.ResourceLinks = nil
			r.Findings = nil
			r.ImpactMatrix = nil
			r.Investigations = nil
			r.Repairs = nil
			for x := range r.Affected {
				r.Affected[x].Versions = nil
			}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func evidenceSet(r Report) map[string]bool {
	out := map[string]bool{}
	for _, e := range r.Evidence {
		out[e.ID] = true
	}
	for _, e := range r.ResourceLinks {
		out[e.ID] = true
	}
	return out
}
func validCitations(r Report, ids []string) bool {
	set := evidenceSet(r)
	for _, id := range ids {
		if !set[id] {
			return false
		}
	}
	return true
}
func affectedRepository(r Report, id string) bool {
	for _, affected := range r.Affected {
		if affected.RepositoryID == id {
			return true
		}
	}
	return false
}

func (s *Store) AddResource(id, actor string, in ResourceLink, maintainer func(string) bool) (Report, error) {
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	in.RepositoryID = strings.TrimSpace(in.RepositoryID)
	in.ResourceID = strings.TrimSpace(in.ResourceID)
	in.Revision = strings.TrimSpace(in.Revision)
	in.Label = strings.TrimSpace(in.Label)
	in.Details = strings.TrimSpace(in.Details)
	if !oneOf(in.Kind, "commit", "dependency", "build", "release_artifact", "deployment", "version_line") || in.RepositoryID == "" || in.Label == "" || len(in.Label) > 300 || len(in.Details) > 10000 || (in.ResourceID == "" && in.Revision == "") {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if !affectedRepository(r, in.RepositoryID) {
		return r, ErrInvalid
	}
	in.ID, err = newID()
	if err != nil {
		return r, err
	}
	now := s.now().UTC()
	in.ActorID = actor
	in.CreatedAt = now
	r.ResourceLinks = append(r.ResourceLinks, in)
	r.UpdatedAt = now
	r.append("evidence.linked", actor, in.ID, in.Kind, now)
	return r, s.write(r)
}
func (s *Store) AddFinding(id, actor string, in Finding, maintainer func(string) bool) (Report, error) {
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	in.Body = strings.TrimSpace(in.Body)
	if !oneOf(in.Type, "hypothesis", "conclusion") || in.Body == "" || len(in.Body) > 20000 || len(in.EvidenceIDs) == 0 {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if !validCitations(r, in.EvidenceIDs) {
		return r, ErrInvalid
	}
	in.ID, err = newID()
	if err != nil {
		return r, err
	}
	now := s.now().UTC()
	in.ActorID = actor
	in.CreatedAt = now
	r.Findings = append(r.Findings, in)
	r.UpdatedAt = now
	r.append("finding."+in.Type, actor, in.ID, "attributed assessment", now)
	return r, s.write(r)
}
func (s *Store) SetImpact(id, actor string, in Impact, maintainer func(string) bool) (Report, error) {
	in.RepositoryID = strings.TrimSpace(in.RepositoryID)
	in.Version = strings.TrimSpace(in.Version)
	in.Environment = strings.TrimSpace(in.Environment)
	in.State = strings.ToLower(strings.TrimSpace(in.State))
	in.Rationale = strings.TrimSpace(in.Rationale)
	if in.RepositoryID == "" || in.Version == "" || in.Environment == "" || !oneOf(in.State, "confirmed", "suspected", "unaffected", "fixed") || in.Rationale == "" || len(in.Rationale) > 10000 || len(in.EvidenceIDs) == 0 {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if !affectedRepository(r, in.RepositoryID) {
		return r, ErrInvalid
	}
	if !validCitations(r, in.EvidenceIDs) {
		return r, ErrInvalid
	}
	now := s.now().UTC()
	in.ActorID = actor
	in.UpdatedAt = now
	for x := range r.ImpactMatrix {
		if r.ImpactMatrix[x].RepositoryID == in.RepositoryID && r.ImpactMatrix[x].Version == in.Version && r.ImpactMatrix[x].Environment == in.Environment {
			in.ID = r.ImpactMatrix[x].ID
			r.ImpactMatrix[x] = in
			r.UpdatedAt = now
			r.append("impact.updated", actor, in.ID, in.State, now)
			return r, s.write(r)
		}
	}
	in.ID, err = newID()
	if err != nil {
		return r, err
	}
	r.ImpactMatrix = append(r.ImpactMatrix, in)
	r.UpdatedAt = now
	r.append("impact.created", actor, in.ID, in.State, now)
	return r, s.write(r)
}
func (s *Store) StartInvestigation(id, actor, agent, mandate string, evidenceIDs []string, maintainer func(string) bool) (Report, string, error) {
	agent = strings.TrimSpace(agent)
	mandate = strings.TrimSpace(mandate)
	if agent == "" || mandate == "" || len(mandate) > 10000 || len(evidenceIDs) == 0 {
		return Report{}, "", ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, "", ErrNotFound
	}
	if !validCitations(r, evidenceIDs) {
		return r, "", ErrInvalid
	}
	iid, _ := newID()
	token, _ := newToken()
	now := s.now().UTC()
	inv := Investigation{ID: iid, Agent: agent, Mandate: mandate, State: "running", InitiatedByID: actor, EvidenceIDs: append([]string{}, evidenceIDs...), Authority: []string{"advisory:selected_evidence:read", "advisory:investigation_records:write"}, Records: []InvestigationRecord{}, CreatedAt: now, UpdatedAt: now, CredentialExpiresAt: now.Add(24 * time.Hour), CredentialDigest: digestToken(token)}
	r.Investigations = append(r.Investigations, inv)
	r.UpdatedAt = now
	r.append("investigation.delegated", actor, iid, "read-only selected evidence", now)
	return r, token, s.write(r)
}
func (s *Store) InvestigationContext(token string) (Report, Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findInvestigation(token)
}
func (s *Store) AddInvestigationRecord(token, typ, body, uncertainty string, evidenceIDs []string) (Report, Investigation, error) {
	typ = strings.ToLower(strings.TrimSpace(typ))
	body = strings.TrimSpace(body)
	uncertainty = strings.TrimSpace(uncertainty)
	if !oneOf(typ, "finding", "tool", "question", "uncertainty") || body == "" || len(body) > 10000 || len(uncertainty) > 2000 {
		return Report{}, Investigation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, inv, err := s.findInvestigation(token)
	if err != nil {
		return r, inv, err
	}
	if inv.State != "running" {
		return r, inv, ErrTransition
	}
	allowed := map[string]bool{}
	for _, id := range inv.EvidenceIDs {
		allowed[id] = true
	}
	for _, id := range evidenceIDs {
		if !allowed[id] {
			return r, inv, ErrInvalid
		}
	}
	now := s.now().UTC()
	inv.Records = append(inv.Records, InvestigationRecord{Sequence: int64(len(inv.Records) + 1), Type: typ, ActorID: "agent:" + inv.Agent, Body: body, Uncertainty: uncertainty, EvidenceIDs: append([]string{}, evidenceIDs...), CreatedAt: now})
	inv.UpdatedAt = now
	for x := range r.Investigations {
		if r.Investigations[x].ID == inv.ID {
			r.Investigations[x] = inv
		}
	}
	r.UpdatedAt = now
	r.append("agent."+typ, "agent:"+inv.Agent, inv.ID, "embargoed investigation", now)
	err = s.write(r)
	return r, inv, err
}
func (s *Store) ControlInvestigation(id, session, actor, action, message string, maintainer func(string) bool) (Report, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if !oneOf(action, "pause", "resume", "cancel", "guide") || (action == "guide" && strings.TrimSpace(message) == "") {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	for x := range r.Investigations {
		inv := &r.Investigations[x]
		if inv.ID != session {
			continue
		}
		if action == "pause" && inv.State == "running" {
			inv.State = "paused"
		} else if action == "resume" && inv.State == "paused" {
			inv.State = "running"
		} else if action == "cancel" && inv.State != "cancelled" {
			inv.State = "cancelled"
			inv.CredentialDigest = ""
		} else if action != "guide" {
			return r, ErrTransition
		}
		now := s.now().UTC()
		inv.Records = append(inv.Records, InvestigationRecord{Sequence: int64(len(inv.Records) + 1), Type: action, ActorID: actor, Body: message, CreatedAt: now})
		inv.UpdatedAt = now
		r.UpdatedAt = now
		r.append("investigation."+action, actor, session, message, now)
		return r, s.write(r)
	}
	return r, ErrNotFound
}

type RepairInput struct {
	ActorID, RepositoryID, Version, Outcome, BaseRevision, Branch string
	DependencyIDs                                                 []string
}

func (s *Store) CreateRepair(id string, in RepairInput, maintainer func(string) bool) (Report, RepairTask, error) {
	in.RepositoryID, in.Version, in.Outcome = strings.TrimSpace(in.RepositoryID), strings.TrimSpace(in.Version), strings.TrimSpace(in.Outcome)
	if in.ActorID == "" || in.RepositoryID == "" || in.Version == "" || in.Outcome == "" || len(in.Outcome) > 10000 || in.BaseRevision == "" || !strings.HasPrefix(in.Branch, "refs/heads/embargo/") || len(in.DependencyIDs) > 20 {
		return Report{}, RepairTask{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, in.ActorID, maintainer) {
		return Report{}, RepairTask{}, ErrNotFound
	}
	if r.EmbargoState == "lifted" || !affectedRepository(r, in.RepositoryID) || !affectedVersion(r, in.RepositoryID, in.Version) {
		return r, RepairTask{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, dependency := range in.DependencyIDs {
		if seen[dependency] {
			return r, RepairTask{}, ErrInvalid
		}
		seen[dependency] = true
		found := false
		for _, task := range r.Repairs {
			found = found || task.ID == dependency
		}
		if !found {
			return r, RepairTask{}, ErrInvalid
		}
	}
	tid, err := newID()
	if err != nil {
		return r, RepairTask{}, err
	}
	now := s.now().UTC()
	task := RepairTask{ID: tid, RepositoryID: in.RepositoryID, Version: in.Version, Outcome: in.Outcome, BaseRevision: in.BaseRevision, Branch: in.Branch, DependencyIDs: append([]string{}, in.DependencyIDs...), State: "open", CreatedByID: in.ActorID, Sessions: []RepairSession{}, CreatedAt: now, UpdatedAt: now}
	r.Repairs = append(r.Repairs, task)
	r.UpdatedAt = now
	r.append("repair.created", in.ActorID, tid, "embargoed version line", now)
	return r, task, s.write(r)
}

func affectedVersion(r Report, repositoryID, version string) bool {
	for _, affected := range r.Affected {
		if affected.RepositoryID == repositoryID {
			for _, candidate := range affected.Versions {
				if candidate == version {
					return true
				}
			}
		}
	}
	return false
}

func (s *Store) StartRepairSession(id, taskID, actor, kind, assignee, mandate, credentialName string, expires time.Time, maintainer func(string) bool) (Report, RepairSession, error) {
	kind, assignee, mandate = strings.ToLower(strings.TrimSpace(kind)), strings.TrimSpace(assignee), strings.TrimSpace(mandate)
	if !oneOf(kind, "human", "agent") || assignee == "" || mandate == "" || len(mandate) > 10000 || credentialName == "" {
		return Report{}, RepairSession{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, RepairSession{}, ErrNotFound
	}
	if r.EmbargoState == "lifted" {
		return r, RepairSession{}, ErrTransition
	}
	for x := range r.Repairs {
		task := &r.Repairs[x]
		if task.ID != taskID {
			continue
		}
		sid, er := newID()
		if er != nil {
			return r, RepairSession{}, er
		}
		now := s.now().UTC()
		session := RepairSession{ID: sid, Kind: kind, AssigneeID: assignee, Mandate: mandate, State: "active", Authority: []string{"embargoed_branch:read", "embargoed_branch:write", "security_repair:records:write"}, InitiatedByID: actor, CredentialName: credentialName, CredentialExpiresAt: expires, Records: []RepairRecord{}, CreatedAt: now, UpdatedAt: now}
		task.Sessions = append(task.Sessions, session)
		task.UpdatedAt = now
		r.UpdatedAt = now
		r.append("repair.session.started", actor, sid, kind+" scoped branch access", now)
		return r, session, s.write(r)
	}
	return r, RepairSession{}, ErrNotFound
}

func (s *Store) AddRepairRecord(id, taskID, sessionID, actor, typ, body, revision, decision string, maintainer func(string) bool) (Report, error) {
	typ, body, decision = strings.ToLower(strings.TrimSpace(typ)), strings.TrimSpace(body), strings.ToLower(strings.TrimSpace(decision))
	if !oneOf(typ, "message", "status", "branch_update", "review") || body == "" || len(body) > 20000 || (typ == "branch_update" && revision == "") || (typ == "review" && !oneOf(decision, "approve", "request_changes")) {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	for x := range r.Repairs {
		if r.Repairs[x].ID == taskID {
			for y := range r.Repairs[x].Sessions {
				session := &r.Repairs[x].Sessions[y]
				if session.ID == sessionID {
					if session.State != "active" {
						return r, ErrTransition
					}
					now := s.now().UTC()
					session.Records = append(session.Records, RepairRecord{Sequence: int64(len(session.Records) + 1), Type: typ, ActorID: actor, Body: body, Revision: revision, Decision: decision, CreatedAt: now})
					session.UpdatedAt = now
					r.Repairs[x].UpdatedAt = now
					r.UpdatedAt = now
					r.append("repair."+typ, actor, sessionID, "embargoed collaboration", now)
					return r, s.write(r)
				}
			}
		}
	}
	return r, ErrNotFound
}

func (s *Store) RevokeRepairSession(id, taskID, sessionID, actor string, maintainer func(string) bool) (Report, RepairSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, RepairSession{}, ErrNotFound
	}
	for x := range r.Repairs {
		if r.Repairs[x].ID == taskID {
			for y := range r.Repairs[x].Sessions {
				session := &r.Repairs[x].Sessions[y]
				if session.ID == sessionID {
					if session.State == "revoked" {
						return r, *session, ErrConflict
					}
					now := s.now().UTC()
					session.State = "revoked"
					session.UpdatedAt = now
					r.UpdatedAt = now
					r.append("repair.session.revoked", actor, sessionID, "branch access revoked", now)
					return r, *session, s.write(r)
				}
			}
		}
	}
	return r, RepairSession{}, ErrNotFound
}

// UpdateRepairVerification advances the evidence ledger only through explicit,
// monotonic actions. The HTTP boundary separately proves Revision is still the
// embargo branch tip before calling this method.
func (s *Store) UpdateRepairVerification(id, taskID string, in VerificationAction, maintainer func(string) bool) (Report, error) {
	in.Action, in.Kind, in.State, in.Decision = strings.ToLower(strings.TrimSpace(in.Action)), strings.ToLower(strings.TrimSpace(in.Kind)), strings.ToLower(strings.TrimSpace(in.State)), strings.ToLower(strings.TrimSpace(in.Decision))
	in.Name, in.AttemptID, in.DefinitionDigest, in.Summary, in.Gap = strings.TrimSpace(in.Name), strings.TrimSpace(in.AttemptID), strings.TrimSpace(in.DefinitionDigest), strings.TrimSpace(in.Summary), strings.TrimSpace(in.Gap)
	if in.ActorID == "" || in.Revision == "" {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, in.ActorID, maintainer) {
		return Report{}, ErrNotFound
	}
	if r.EmbargoState == "lifted" {
		return r, ErrTransition
	}
	for x := range r.Repairs {
		task := &r.Repairs[x]
		if task.ID != taskID {
			continue
		}
		now := s.now().UTC()
		if task.Verification == nil || task.Verification.Revision != in.Revision {
			if in.Action != "begin" {
				return r, ErrConflict
			}
			task.Verification = &RepairVerification{Revision: in.Revision, State: "verifying", Gates: []VerificationGate{}, Approvals: []VerificationApproval{}, ReleaseAttestations: []ReleaseAttestation{}, RemainingGaps: []string{}, UpdatedAt: now}
		} else if in.Action == "begin" {
			return r, ErrConflict
		}
		v := task.Verification
		if v.IntegrationCommitID != "" && in.Action != "attest" {
			return r, ErrTransition
		}
		switch in.Action {
		case "begin":
		case "gate":
			if !oneOf(in.Kind, "required_check", "security_reproduction") || in.Name == "" || len(in.Name) > 200 || in.AttemptID == "" || !validSHA256(in.DefinitionDigest) || !oneOf(in.State, "queued", "running", "passed", "failed", "cancelled") {
				return r, ErrInvalid
			}
			for _, prior := range v.Gates {
				if prior.AttemptID == in.AttemptID {
					return r, ErrConflict
				}
			}
			gate := VerificationGate{Kind: in.Kind, Name: in.Name, AttemptID: in.AttemptID, DefinitionDigest: in.DefinitionDigest, State: in.State, RecordedByID: in.ActorID, RecordedAt: now}
			v.Gates = append(v.Gates, gate)
		case "approve":
			if !oneOf(in.Decision, "approve", "request_changes") || in.Summary == "" || len(in.Summary) > 2000 {
				return r, ErrInvalid
			}
			approval := VerificationApproval{ActorID: in.ActorID, Decision: in.Decision, Summary: in.Summary, CreatedAt: now}
			replaced := false
			for i := range v.Approvals {
				if v.Approvals[i].ActorID == in.ActorID {
					v.Approvals[i], replaced = approval, true
				}
			}
			if !replaced {
				v.Approvals = append(v.Approvals, approval)
			}
		case "gap":
			if in.Gap == "" || len(in.Gap) > 2000 {
				return r, ErrInvalid
			}
			if len(v.RemainingGaps) >= 50 {
				return r, ErrInvalid
			}
			v.RemainingGaps = append(v.RemainingGaps, in.Gap)
		case "resolve_gap":
			found := false
			for i := range v.RemainingGaps {
				if v.RemainingGaps[i] == in.Gap {
					v.RemainingGaps = append(v.RemainingGaps[:i], v.RemainingGaps[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				return r, ErrConflict
			}
		case "integrate":
			if !maintainer(task.RepositoryID) || in.IntegrationEntryID == "" || in.IntegrationCommitID == "" || !verificationPassed(v) {
				return r, ErrTransition
			}
			v.IntegrationEntryID, v.IntegrationCommitID = in.IntegrationEntryID, in.IntegrationCommitID
		case "attest":
			if !maintainer(task.RepositoryID) || v.IntegrationCommitID == "" || in.ReleaseID == "" || in.Version != task.Version || in.ArtifactID == "" || !validSHA256(in.ArtifactSHA256) {
				return r, ErrTransition
			}
			for _, prior := range v.ReleaseAttestations {
				if prior.ReleaseID == in.ReleaseID && prior.ArtifactID == in.ArtifactID {
					return r, ErrConflict
				}
			}
			v.ReleaseAttestations = append(v.ReleaseAttestations, ReleaseAttestation{ReleaseID: in.ReleaseID, Version: in.Version, ArtifactID: in.ArtifactID, ArtifactSHA256: in.ArtifactSHA256, RecordedByID: in.ActorID, RecordedAt: now})
		default:
			return r, ErrInvalid
		}
		v.State = verificationState(v)
		v.UpdatedAt, task.UpdatedAt, r.UpdatedAt = now, now, now
		r.append("repair.verification."+in.Action, in.ActorID, taskID, "exact candidate evidence updated", now)
		return r, s.write(r)
	}
	return r, ErrNotFound
}

func verificationPassed(v *RepairVerification) bool {
	check, reproduction, approval := false, false, false
	latest := map[string]VerificationGate{}
	for _, gate := range v.Gates {
		latest[gate.Kind+"\x00"+gate.Name] = gate
	}
	for _, gate := range latest {
		if gate.State != "passed" {
			return false
		}
		check = check || gate.Kind == "required_check"
		reproduction = reproduction || gate.Kind == "security_reproduction"
	}
	for _, review := range v.Approvals {
		if review.Decision == "request_changes" {
			return false
		}
		approval = approval || review.Decision == "approve"
	}
	return check && reproduction && approval && len(v.RemainingGaps) == 0
}

func verificationState(v *RepairVerification) string {
	if len(v.ReleaseAttestations) > 0 {
		return "attested"
	}
	if v.IntegrationCommitID != "" {
		return "integrated"
	}
	if verificationPassed(v) {
		return "ready"
	}
	latest := map[string]VerificationGate{}
	for _, gate := range v.Gates {
		latest[gate.Kind+"\x00"+gate.Name] = gate
	}
	for _, gate := range latest {
		if gate.State == "failed" || gate.State == "cancelled" {
			return "failed"
		}
	}
	return "verifying"
}
func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
func (s *Store) findInvestigation(token string) (Report, Investigation, error) {
	d := digestToken(token)
	entries, _ := os.ReadDir(s.root)
	for _, f := range entries {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		r, err := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if err != nil {
			continue
		}
		for _, inv := range r.Investigations {
			if d != "" && inv.CredentialDigest == d {
				if s.now().UTC().After(inv.CredentialExpiresAt) || inv.State == "cancelled" {
					return r, inv, ErrConflict
				}
				return r, inv, nil
			}
		}
	}
	return Report{}, Investigation{}, ErrNotFound
}
func newToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func digestToken(v string) string { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func (s *Store) Get(id, actor string, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	now := s.now().UTC()
	r.append("access.viewed", actor, "", "report opened", now)
	r.UpdatedAt = now
	if err = s.write(r); err != nil {
		return Report{}, err
	}
	return r, nil
}
func (s *Store) Triage(id string, in TriageInput, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, in.ActorID, maintainer) {
		return Report{}, ErrNotFound
	}
	sev := strings.ToLower(strings.TrimSpace(in.Severity))
	emb := strings.ToLower(strings.TrimSpace(in.EmbargoState))
	if sev == "" {
		sev = r.Severity
	}
	if emb == "" {
		emb = r.EmbargoState
	}
	if !oneOf(sev, "unknown", "low", "medium", "high", "critical") || !oneOf(emb, "requested", "active", "lifted") {
		return r, ErrInvalid
	}
	if sev == r.Severity && emb == r.EmbargoState {
		return r, ErrConflict
	}
	r.Severity, r.EmbargoState = sev, emb
	now := s.now().UTC()
	r.UpdatedAt = now
	r.append("triage.updated", in.ActorID, "", sev+" severity; "+emb+" embargo", now)
	return r, s.write(r)
}
func (s *Store) SetMember(id, actor, subject string, add bool, maintainer func(string) bool) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !isMaintainer(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	if subject == "" || subject == r.ReporterID {
		return r, ErrInvalid
	}
	at := -1
	for i, m := range r.Team {
		if m.UserID == subject {
			at = i
		}
	}
	now := s.now().UTC()
	if add {
		if at >= 0 {
			return r, ErrConflict
		}
		if len(r.Team) >= 20 {
			return r, ErrInvalid
		}
		r.Team = append(r.Team, TeamMember{UserID: subject, InvitedByID: actor, InvitedAt: now})
		r.append("team.invited", actor, subject, "response access granted", now)
	} else {
		if at < 0 {
			return r, ErrConflict
		}
		r.Team = append(r.Team[:at], r.Team[at+1:]...)
		r.append("team.removed", actor, subject, "response access revoked", now)
	}
	r.UpdatedAt = now
	return r, s.write(r)
}
func (s *Store) AddMessage(id, actor, body string, maintainer func(string) bool) (Report, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 20000 {
		return Report{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.read(id)
	if err != nil || !canAccess(r, actor, maintainer) {
		return Report{}, ErrNotFound
	}
	mid, err := newID()
	if err != nil {
		return r, err
	}
	now := s.now().UTC()
	r.Messages = append(r.Messages, Message{ID: mid, AuthorID: actor, Body: body, CreatedAt: now})
	r.UpdatedAt = now
	r.append("message.created", actor, "", "private message added", now)
	return r, s.write(r)
}
func canAccess(r Report, actor string, maintainer func(string) bool) bool {
	if actor == r.ReporterID || isMaintainer(r, actor, maintainer) {
		return true
	}
	for _, m := range r.Team {
		if m.UserID == actor {
			return true
		}
	}
	return false
}
func isMaintainer(r Report, actor string, maintainer func(string) bool) bool {
	for _, a := range r.Affected {
		if maintainer(a.RepositoryID) {
			return true
		}
	}
	return false
}
func (r *Report) append(kind, actor, subject, detail string, at time.Time) {
	r.Audit = append(r.Audit, AuditEvent{Sequence: int64(len(r.Audit) + 1), Type: kind, ActorID: actor, SubjectID: subject, Detail: detail, CreatedAt: at})
}
func (s *Store) read(id string) (Report, error) {
	b, err := os.ReadFile(filepath.Join(s.root, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, err
	}
	var r Report
	if json.Unmarshal(b, &r) != nil {
		return Report{}, ErrInvalid
	}
	for x := range r.Evidence {
		if r.Evidence[x].ID == "" {
			sum := sha256.Sum256([]byte(r.ID + ":submitted:" + r.Evidence[x].Title))
			r.Evidence[x].ID = hex.EncodeToString(sum[:16])
		}
	}
	return r, nil
}
func (s *Store) write(r Report) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, r.ID+".json"), append(b, '\n'), 0640)
}
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func validEvidenceKind(v string) bool {
	return oneOf(v, "description", "reproduction", "log", "artifact", "reference")
}
