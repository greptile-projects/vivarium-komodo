// Package independentassessments retains bounded, attributable third-party assurance reviews.
package independentassessments

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

var ErrNotFound = errors.New("independent assessment not found")
var ErrInvalid = errors.New("invalid independent assessment")
var ErrConflict = errors.New("independent assessment conflict")
var ErrForbidden = errors.New("independent assessment forbidden")
var ErrExpired = errors.New("independent assessment access expired")

type Scope struct {
	ProgramID          string    `json:"program_id"`
	ProgramVersion     int64     `json:"program_version"`
	ControlIDs         []string  `json:"control_ids"`
	Systems            []string  `json:"systems"`
	Releases           []string  `json:"releases"`
	PeriodStart        time.Time `json:"period_start"`
	PeriodEnd          time.Time `json:"period_end"`
	EvidencePackageIDs []string  `json:"evidence_package_ids"`
}
type OpenInput struct {
	Title     string    `json:"title"`
	Purpose   string    `json:"purpose"`
	Scope     Scope     `json:"scope"`
	StartsAt  time.Time `json:"starts_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
type InvitationInput struct {
	AssessorID         string    `json:"assessor_id"`
	AssessorName       string    `json:"assessor_name"`
	Organization       string    `json:"organization"`
	Kind               string    `json:"kind"`
	ConflictDisclosure string    `json:"conflict_disclosure"`
	ExpiresAt          time.Time `json:"expires_at"`
}
type Invitation struct {
	ID                 string    `json:"id"`
	AssessorID         string    `json:"assessor_id"`
	AssessorName       string    `json:"assessor_name"`
	Organization       string    `json:"organization"`
	Kind               string    `json:"kind"`
	ConflictDisclosure string    `json:"conflict_disclosure"`
	ConflictStatus     string    `json:"conflict_status"`
	Status             string    `json:"status"`
	ExpiresAt          time.Time `json:"expires_at"`
	InvitedBy          string    `json:"invited_by"`
	InvitedAt          time.Time `json:"invited_at"`
	CredentialHash     string    `json:"credential_hash,omitempty"`
}

// Redact removes credential verifiers before a record crosses an API boundary.
func Redact(a Assessment) Assessment {
	for i := range a.Invitations {
		a.Invitations[i].CredentialHash = ""
	}
	return a
}
func RedactInvitation(i Invitation) Invitation { i.CredentialHash = ""; return i }

type EventInput struct {
	Kind               string   `json:"kind"`
	Subject            string   `json:"subject"`
	Body               string   `json:"body"`
	ControlID          string   `json:"control_id,omitempty"`
	EvidencePackageIDs []string `json:"evidence_package_ids,omitempty"`
	ParentID           string   `json:"parent_id,omitempty"`
	Disposition        string   `json:"disposition,omitempty"`
}
type Event struct {
	ID string `json:"id"`
	EventInput
	ActorID   string    `json:"actor_id"`
	ActorRole string    `json:"actor_role"`
	CreatedAt time.Time `json:"created_at"`
}
type ScopeChange struct {
	ID        string    `json:"id"`
	Prior     Scope     `json:"prior"`
	Current   Scope     `json:"current"`
	Reason    string    `json:"reason"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Assessment struct {
	ID           string        `json:"id"`
	RepositoryID string        `json:"repository_id"`
	Title        string        `json:"title"`
	Purpose      string        `json:"purpose"`
	Scope        Scope         `json:"scope"`
	StartsAt     time.Time     `json:"starts_at"`
	ExpiresAt    time.Time     `json:"expires_at"`
	Status       string        `json:"status"`
	OwnerID      string        `json:"owner_id"`
	Invitations  []Invitation  `json:"invitations"`
	Events       []Event       `json:"events"`
	ScopeChanges []ScopeChange `json:"scope_changes"`
	Revision     int64         `json:"revision"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	NonAuthority []string      `json:"non_authority"`
}
type Credential struct {
	Token        string    `json:"token"`
	AssessmentID string    `json:"assessment_id"`
	InvitationID string    `json:"invitation_id"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        Scope     `json:"scope"`
	Authority    []string  `json:"authority"`
}
type Evidence struct {
	ID          string            `json:"id"`
	ControlID   string            `json:"control_id"`
	PeriodStart time.Time         `json:"period_start"`
	PeriodEnd   time.Time         `json:"period_end"`
	PackageHash string            `json:"package_hash"`
	Attestation string            `json:"attestation"`
	Fresh       bool              `json:"fresh"`
	Coverage    map[string]string `json:"coverage"`
	Gaps        any               `json:"gaps"`
	Records     any               `json:"records"`
}
type Context struct {
	Assessment             Assessment `json:"assessment"`
	Assessor               Invitation `json:"assessor"`
	Evidence               []Evidence `json:"evidence"`
	UnavailableEvidenceIDs []string   `json:"unavailable_evidence_ids"`
}
type evidenceStore interface {
	Package(string, string, string, string) (interface{}, error)
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func token() string                          { var b [32]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func hash(v string) string                   { x := sha256.Sum256([]byte(v)); return hex.EncodeToString(x[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func validScope(x Scope) bool {
	return x.ProgramID != "" && x.ProgramVersion > 0 && len(x.ControlIDs) > 0 && !x.PeriodStart.IsZero() && x.PeriodEnd.After(x.PeriodStart)
}
func unique(xs []string) bool {
	m := map[string]bool{}
	for _, x := range xs {
		if strings.TrimSpace(x) == "" || m[x] {
			return false
		}
		m[x] = true
	}
	return true
}
func (s *Store) save(a Assessment) error {
	if e := os.MkdirAll(filepath.Dir(s.path(a.RepositoryID, a.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(a.RepositoryID, a.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) read(repo, id string) (Assessment, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, os.ErrNotExist) {
		return Assessment{}, ErrNotFound
	}
	var a Assessment
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	return a, e
}
func (s *Store) Open(repo, actor string, in OpenInput) (Assessment, error) {
	if repo == "" || actor == "" || strings.TrimSpace(in.Title) == "" || !validScope(in.Scope) || !unique(in.Scope.ControlIDs) || in.StartsAt.IsZero() || !in.ExpiresAt.After(in.StartsAt) || in.ExpiresAt.Sub(in.StartsAt) > 366*24*time.Hour {
		return Assessment{}, ErrInvalid
	}
	now := s.now().UTC()
	a := Assessment{ID: id("assessment_"), RepositoryID: repo, Title: in.Title, Purpose: in.Purpose, Scope: in.Scope, StartsAt: in.StartsAt.UTC(), ExpiresAt: in.ExpiresAt.UTC(), Status: "open", OwnerID: actor, Invitations: []Invitation{}, Events: []Event{}, ScopeChanges: []ScopeChange{}, Revision: 1, CreatedAt: now, UpdatedAt: now, NonAuthority: []string{"repository_write", "git", "secret", "credential", "production", "environment", "approval", "merge", "release", "deployment", "operational"}}
	return a, s.save(a)
}
func (s *Store) Get(repo, id string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, f := range files {
		b, e := os.ReadFile(f)
		var a Assessment
		if e == nil {
			e = json.Unmarshal(b, &a)
		}
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Invite(repo, aid, actor string, in InvitationInput) (Assessment, Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e != nil {
		return a, Credential{}, e
	}
	now := s.now().UTC()
	if a.OwnerID != actor {
		return a, Credential{}, ErrForbidden
	}
	if a.Status != "open" || !now.Before(a.ExpiresAt) || in.AssessorID == "" || in.AssessorName == "" || (in.Kind != "internal" && in.Kind != "external") || in.ExpiresAt.After(a.ExpiresAt) || !in.ExpiresAt.After(now) {
		return a, Credential{}, ErrInvalid
	}
	t := token()
	conflict := "clear"
	if strings.TrimSpace(in.ConflictDisclosure) != "" {
		conflict = "disclosed"
	}
	v := Invitation{ID: id("invite_"), AssessorID: in.AssessorID, AssessorName: in.AssessorName, Organization: in.Organization, Kind: in.Kind, ConflictDisclosure: in.ConflictDisclosure, ConflictStatus: conflict, Status: "active", ExpiresAt: in.ExpiresAt.UTC(), InvitedBy: actor, InvitedAt: now, CredentialHash: hash(t)}
	a.Invitations = append(a.Invitations, v)
	a.Revision++
	a.UpdatedAt = now
	e = s.save(a)
	return a, Credential{Token: t, AssessmentID: a.ID, InvitationID: v.ID, ExpiresAt: v.ExpiresAt, Scope: a.Scope, Authority: []string{"read_scoped_assessment", "read_selected_evidence", "verify_attestation", "request_evidence", "ask_question", "record_sample", "request_walkthrough", "record_finding", "disagree", "appeal"}}, e
}
func assessorEvent(k string) bool {
	switch k {
	case "question", "sample", "walkthrough_request", "attestation_verification", "evidence_request", "finding", "disagreement", "appeal":
		return true
	}
	return false
}
func ownerEvent(k string) bool {
	switch k {
	case "response", "walkthrough", "evidence_response", "finding_response", "resolution", "appeal_decision", "conflict_resolution", "unavailable_evidence":
		return true
	}
	return false
}
func (s *Store) Add(repo, aid, actor, role string, in EventInput) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e != nil {
		return a, e
	}
	now := s.now().UTC()
	if a.Status != "open" || now.Before(a.StartsAt) || !now.Before(a.ExpiresAt) || strings.TrimSpace(in.Subject) == "" || strings.TrimSpace(in.Body) == "" || ((role == "assessor" && !assessorEvent(in.Kind)) || (role == "owner" && !ownerEvent(in.Kind))) {
		return a, ErrInvalid
	}
	if in.ControlID != "" && !contains(a.Scope.ControlIDs, in.ControlID) {
		return a, ErrForbidden
	}
	for _, pid := range in.EvidencePackageIDs {
		if !contains(a.Scope.EvidencePackageIDs, pid) {
			return a, ErrForbidden
		}
	}
	if role == "owner" && a.OwnerID != actor {
		return a, ErrForbidden
	}
	a.Events = append(a.Events, Event{ID: id("event_"), EventInput: in, ActorID: actor, ActorRole: role, CreatedAt: now})
	a.Revision++
	a.UpdatedAt = now
	return a, s.save(a)
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) Authenticate(tokenValue string) (Assessment, Invitation, error) {
	h := hash(strings.TrimSpace(tokenValue))
	s.mu.Lock()
	defer s.mu.Unlock()
	repos, e := os.ReadDir(s.root)
	if e != nil {
		return Assessment{}, Invitation{}, e
	}
	now := s.now().UTC()
	for _, rd := range repos {
		if !rd.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(s.root, rd.Name(), "*.json"))
		for _, f := range files {
			b, _ := os.ReadFile(f)
			var a Assessment
			if json.Unmarshal(b, &a) != nil {
				continue
			}
			for _, v := range a.Invitations {
				if v.CredentialHash == h {
					if v.Status != "active" || now.Before(a.StartsAt) || !now.Before(v.ExpiresAt) || !now.Before(a.ExpiresAt) || a.Status != "open" {
						return a, v, ErrExpired
					}
					return a, v, nil
				}
			}
		}
	}
	return Assessment{}, Invitation{}, ErrNotFound
}
func (s *Store) Revoke(repo, aid, actor, invite string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e != nil {
		return a, e
	}
	if a.OwnerID != actor {
		return a, ErrForbidden
	}
	found := false
	for i := range a.Invitations {
		if a.Invitations[i].ID == invite {
			a.Invitations[i].Status = "revoked"
			found = true
		}
	}
	if !found {
		return a, ErrNotFound
	}
	a.Revision++
	a.UpdatedAt = s.now().UTC()
	return a, s.save(a)
}
func (s *Store) ChangeScope(repo, aid, actor string, expected int64, next Scope, reason string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, aid)
	if e != nil {
		return a, e
	}
	if a.OwnerID != actor {
		return a, ErrForbidden
	}
	if a.Revision != expected {
		return a, ErrConflict
	}
	if !validScope(next) || reason == "" || !unique(next.ControlIDs) {
		return a, ErrInvalid
	}
	now := s.now().UTC()
	a.ScopeChanges = append(a.ScopeChanges, ScopeChange{ID: id("scope_"), Prior: a.Scope, Current: next, Reason: reason, ActorID: actor, CreatedAt: now})
	a.Scope = next
	for i := range a.Invitations {
		a.Invitations[i].Status = "scope_changed"
	}
	a.Revision++
	a.UpdatedAt = now
	return a, s.save(a)
}
