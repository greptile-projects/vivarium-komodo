// Package assurancedelivery connects independent findings to governed work and signed, bounded claims.
package assurancedelivery

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

var ErrNotFound = errors.New("assurance delivery not found")
var ErrInvalid = errors.New("invalid assurance delivery")
var ErrForbidden = errors.New("assurance delivery forbidden")
var ErrConflict = errors.New("assurance delivery changed")

type AssessmentScope struct {
	ProgramID                                string
	ProgramVersion                           int64
	ControlIDs, Releases, EvidencePackageIDs []string
	PeriodStart, PeriodEnd                   time.Time
}
type FindingSource struct {
	AssessmentID, FindingID, ControlID, FindingBody, ActorID string
	Scope                                                    AssessmentScope
	OwnerID                                                  string
}
type Source interface {
	Finding(string, string, string) (FindingSource, error)
}
type Signer interface {
	Sign([]byte) (string, string, error)
}

type WorkInput struct {
	Kind               string   `json:"kind"`
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	ResourceID         string   `json:"resource_id,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}
type RemediationInput struct {
	AffectedRevision   string      `json:"affected_revision"`
	Deadline           time.Time   `json:"deadline"`
	EvidencePackageIDs []string    `json:"evidence_package_ids"`
	Work               []WorkInput `json:"work"`
}
type WorkItem struct {
	ID       string `json:"id"`
	Position int    `json:"position"`
	WorkInput
	Status   string     `json:"status"`
	Progress []Progress `json:"progress"`
}
type ProgressInput struct {
	Status             string   `json:"status"`
	Summary            string   `json:"summary"`
	ResourceID         string   `json:"resource_id,omitempty"`
	Revision           string   `json:"revision,omitempty"`
	EvidencePackageIDs []string `json:"evidence_package_ids,omitempty"`
}
type Progress struct {
	ID string `json:"id"`
	ProgressInput
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type VerificationInput struct {
	AffectedRevision   string          `json:"affected_revision"`
	EvidenceDigest     string          `json:"evidence_digest"`
	EvidencePackageIDs []string        `json:"evidence_package_ids"`
	Criteria           map[string]bool `json:"criteria"`
	Summary            string          `json:"summary"`
}
type Verification struct {
	ID string `json:"id"`
	VerificationInput
	ActorID   string    `json:"actor_id"`
	Current   bool      `json:"current"`
	Passed    bool      `json:"passed"`
	CreatedAt time.Time `json:"created_at"`
}
type Disposition struct {
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	ActorID   string    `json:"actor_id"`
	ActorRole string    `json:"actor_role"`
	CreatedAt time.Time `json:"created_at"`
}
type Remediation struct {
	ID                 string         `json:"id"`
	RepositoryID       string         `json:"repository_id"`
	AssessmentID       string         `json:"assessment_id"`
	FindingID          string         `json:"finding_id"`
	ControlID          string         `json:"control_id"`
	FindingBody        string         `json:"finding_body"`
	AffectedRevision   string         `json:"affected_revision"`
	Deadline           time.Time      `json:"deadline"`
	EvidencePackageIDs []string       `json:"evidence_package_ids"`
	Work               []WorkItem     `json:"work"`
	Verifications      []Verification `json:"verifications"`
	Dispositions       []Disposition  `json:"dispositions"`
	Status             string         `json:"status"`
	Revision           int64          `json:"revision"`
	CreatedByID        string         `json:"created_by_id"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	AuthorityGranted   bool           `json:"authority_granted"`
}
type StatementInput struct {
	ProgramID           string    `json:"program_id"`
	ProgramVersion      int64     `json:"program_version"`
	ReleaseID           string    `json:"release_id"`
	ReleaseRevision     string    `json:"release_revision"`
	Scope               string    `json:"scope"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	ExpiresAt           time.Time `json:"expires_at"`
	ControlIDs          []string  `json:"control_ids"`
	ExceptionReferences []string  `json:"exception_references"`
	EvidencePackageIDs  []string  `json:"evidence_package_ids"`
	RemediationIDs      []string  `json:"remediation_ids"`
	Audience            string    `json:"audience"`
	EvidenceDigest      string    `json:"evidence_digest"`
}
type Statement struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	StatementInput
	KeyID            string     `json:"key_id"`
	Signature        string     `json:"signature"`
	SignedPayload    string     `json:"signed_payload"`
	PayloadDigest    string     `json:"payload_digest"`
	Status           string     `json:"status"`
	StatusReasons    []string   `json:"status_reasons"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevokedBy        string     `json:"revoked_by,omitempty"`
	RevocationReason string     `json:"revocation_reason,omitempty"`
	CreatedByID      string     `json:"created_by_id"`
	CreatedAt        time.Time  `json:"created_at"`
	AuthorityGranted bool       `json:"authority_granted"`
}
type Store struct {
	root   string
	source Source
	signer Signer
	mu     sync.Mutex
	now    func() time.Time
}

func New(root string, source Source, signer Signer) (*Store, error) {
	if root == "" || source == nil || signer == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, source: source, signer: signer, now: time.Now}, e
}
func newID(prefix string) string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}
func uniq(x []string, required bool) bool {
	if required && len(x) == 0 {
		return false
	}
	m := map[string]bool{}
	for _, v := range x {
		if strings.TrimSpace(v) == "" || m[v] {
			return false
		}
		m[v] = true
	}
	return true
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(repo, id string, v any) error {
	if e := os.MkdirAll(filepath.Dir(s.path(repo, id)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo, id), append(b, '\n'), 0640)
	}
	return e
}
func read[T any](p string) (T, error) {
	var v T
	b, e := os.ReadFile(p)
	if errors.Is(e, fs.ErrNotExist) {
		return v, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &v)
	}
	return v, e
}

func (s *Store) CreateRemediation(repo, assessment, finding, actor string, in RemediationInput) (Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, e := s.source.Finding(repo, assessment, finding)
	if e != nil {
		return Remediation{}, ErrNotFound
	}
	if actor != src.OwnerID || in.AffectedRevision == "" || in.Deadline.Before(s.now()) || !uniq(in.EvidencePackageIDs, true) || len(in.Work) == 0 {
		return Remediation{}, ErrInvalid
	}
	for _, x := range in.EvidencePackageIDs {
		if !contains(src.Scope.EvidencePackageIDs, x) {
			return Remediation{}, ErrForbidden
		}
	}
	items := make([]WorkItem, len(in.Work))
	for i, x := range in.Work {
		if !contains([]string{"task", "session", "workspace", "pull_request", "policy_change", "operational_work"}, x.Kind) || !contains([]string{"human", "agent"}, x.OwnerKind) || x.OwnerID == "" || x.Title == "" || !uniq(x.AcceptanceCriteria, true) {
			return Remediation{}, ErrInvalid
		}
		items[i] = WorkItem{ID: newID("work_"), Position: i + 1, WorkInput: x, Status: "planned", Progress: []Progress{}}
	}
	now := s.now().UTC()
	v := Remediation{ID: newID("remediation_"), RepositoryID: repo, AssessmentID: assessment, FindingID: finding, ControlID: src.ControlID, FindingBody: src.FindingBody, AffectedRevision: in.AffectedRevision, Deadline: in.Deadline.UTC(), EvidencePackageIDs: in.EvidencePackageIDs, Work: items, Verifications: []Verification{}, Dispositions: []Disposition{}, Status: "open", Revision: 1, CreatedByID: actor, CreatedAt: now, UpdatedAt: now, AuthorityGranted: false}
	return v, s.save(repo, v.ID, v)
}
func (s *Store) GetRemediation(repo, id string) (Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Remediation](s.path(repo, id))
	if e == nil {
		s.derive(&v)
	}
	return v, e
}
func (s *Store) ListRemediations(repo string) ([]Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "remediation_*.json"))
	if e != nil {
		return nil, e
	}
	out := []Remediation{}
	for _, f := range files {
		v, e := read[Remediation](f)
		if e != nil {
			return nil, e
		}
		s.derive(&v)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Progress(repo, id, work, actor string, in ProgressInput) (Remediation, error) {
	return s.change(repo, id, func(v *Remediation) error {
		if !contains([]string{"in_progress", "blocked", "completed"}, in.Status) || in.Summary == "" || !uniq(in.EvidencePackageIDs, false) {
			return ErrInvalid
		}
		for _, x := range in.EvidencePackageIDs {
			if !contains(v.EvidencePackageIDs, x) {
				return ErrForbidden
			}
		}
		idx := -1
		for i := range v.Work {
			if v.Work[i].ID == work {
				idx = i
			}
		}
		if idx < 0 {
			return ErrNotFound
		}
		if v.Work[idx].OwnerID != actor && v.CreatedByID != actor {
			return ErrForbidden
		}
		if in.Status == "completed" && idx > 0 && v.Work[idx-1].Status != "completed" {
			return ErrConflict
		}
		p := Progress{ID: newID("progress_"), ProgressInput: in, ActorID: actor, CreatedAt: s.now().UTC()}
		v.Work[idx].Progress = append(v.Work[idx].Progress, p)
		v.Work[idx].Status = in.Status
		return nil
	})
}
func (s *Store) Verify(repo, id, actor string, in VerificationInput) (Remediation, error) {
	return s.change(repo, id, func(v *Remediation) error {
		if actor != v.CreatedByID || in.AffectedRevision == "" || len(in.EvidenceDigest) != 64 || !uniq(in.EvidencePackageIDs, true) || len(in.Criteria) == 0 || in.Summary == "" {
			return ErrInvalid
		}
		if _, e := hex.DecodeString(in.EvidenceDigest); e != nil {
			return ErrInvalid
		}
		pass := true
		for _, ok := range in.Criteria {
			pass = pass && ok
		}
		for _, work := range v.Work {
			for _, criterion := range work.AcceptanceCriteria {
				pass = pass && in.Criteria[criterion]
			}
		}
		for _, x := range in.EvidencePackageIDs {
			if !contains(v.EvidencePackageIDs, x) {
				return ErrForbidden
			}
		}
		for i := range v.Verifications {
			v.Verifications[i].Current = false
		}
		v.Verifications = append(v.Verifications, Verification{ID: newID("verification_"), VerificationInput: in, ActorID: actor, Current: in.AffectedRevision == v.AffectedRevision, Passed: pass, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Disposition(repo, id, actor, role, decision, rationale string) (Remediation, error) {
	return s.change(repo, id, func(v *Remediation) error {
		if role == "owner" && actor != v.CreatedByID {
			return ErrForbidden
		}
		if !contains([]string{"accept", "reject", "reopen"}, decision) || rationale == "" || (role != "owner" && role != "assessor") {
			return ErrInvalid
		}
		if decision == "accept" && v.Status != "verified" {
			return ErrConflict
		}
		v.Dispositions = append(v.Dispositions, Disposition{Decision: decision, Rationale: rationale, ActorID: actor, ActorRole: role, CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) Drift(repo, id, actor, revision, reason string) (Remediation, error) {
	return s.change(repo, id, func(v *Remediation) error {
		if actor != v.CreatedByID {
			return ErrForbidden
		}
		if revision == "" || reason == "" || revision == v.AffectedRevision {
			return ErrInvalid
		}
		v.AffectedRevision = revision
		v.Dispositions = append(v.Dispositions, Disposition{Decision: "reopen", Rationale: "affected revision changed: " + reason, ActorID: actor, ActorRole: "owner", CreatedAt: s.now().UTC()})
		return nil
	})
}
func (s *Store) change(repo, id string, fn func(*Remediation) error) (Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Remediation](s.path(repo, id))
	if e == nil {
		e = fn(&v)
	}
	if e == nil {
		v.Revision++
		v.UpdatedAt = s.now().UTC()
		s.derive(&v)
		e = s.save(repo, id, v)
	}
	return v, e
}
func (s *Store) derive(v *Remediation) {
	v.Status = "open"
	complete := true
	for _, x := range v.Work {
		complete = complete && x.Status == "completed"
	}
	if complete && len(v.Verifications) > 0 {
		z := v.Verifications[len(v.Verifications)-1]
		if z.Current && z.Passed {
			v.Status = "verified"
		}
	}
	for _, d := range v.Dispositions {
		if d.Decision == "accept" && v.Status == "verified" {
			v.Status = "closed"
		}
		if d.Decision == "reject" || d.Decision == "reopen" {
			v.Status = "reopened"
		}
	}
	if s.now().After(v.Deadline) && v.Status != "closed" {
		v.Status = "overdue"
	}
}

func statementBytes(x Statement) []byte {
	x.Signature = ""
	x.SignedPayload = ""
	x.PayloadDigest = ""
	x.Status = ""
	x.StatusReasons = nil
	x.RevokedAt = nil
	x.RevokedBy = ""
	x.RevocationReason = ""
	b, _ := json.Marshal(x)
	return b
}
func (s *Store) Publish(repo, actor string, in StatementInput) (Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.ProgramID == "" || in.ProgramVersion < 1 || in.ReleaseID == "" || in.ReleaseRevision == "" || in.Scope == "" || !in.PeriodEnd.After(in.PeriodStart) || !in.ExpiresAt.After(s.now()) || !uniq(in.ControlIDs, true) || !uniq(in.EvidencePackageIDs, true) || len(in.EvidenceDigest) != 64 || !contains([]string{"public", "repository"}, in.Audience) {
		return Statement{}, ErrInvalid
	}
	if _, e := hex.DecodeString(in.EvidenceDigest); e != nil {
		return Statement{}, ErrInvalid
	}
	program, version := in.ProgramID, in.ProgramVersion
	for _, rid := range in.RemediationIDs {
		r, e := read[Remediation](s.path(repo, rid))
		if e != nil || r.Status != "closed" || !contains(in.ControlIDs, r.ControlID) {
			return Statement{}, ErrConflict
		}
		src, e := s.source.Finding(repo, r.AssessmentID, r.FindingID)
		if e != nil {
			return Statement{}, ErrConflict
		}
		if program != src.Scope.ProgramID || version != src.Scope.ProgramVersion {
			return Statement{}, ErrConflict
		}
	}
	now := s.now().UTC()
	v := Statement{ID: newID("statement_"), RepositoryID: repo, StatementInput: in, CreatedByID: actor, CreatedAt: now, AuthorityGranted: false}
	payload := statementBytes(v)
	sum := sha256.Sum256(payload)
	v.PayloadDigest = hex.EncodeToString(sum[:])
	v.SignedPayload = base64.RawURLEncoding.EncodeToString(payload)
	var e error
	v.KeyID, v.Signature, e = s.signer.Sign(payload)
	if e != nil {
		return Statement{}, e
	}
	v.Status = "current"
	return v, s.save(repo, v.ID, v)
}
func (s *Store) Statement(repo, id, audience string) (Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Statement](s.path(repo, id))
	if e != nil {
		return v, e
	}
	if v.Audience == "repository" && audience != "repository" {
		return Statement{}, ErrForbidden
	}
	s.deriveStatement(&v)
	return v, nil
}
func (s *Store) RevokeStatement(repo, id, actor, reason string) (Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Statement](s.path(repo, id))
	if e != nil {
		return v, e
	}
	if v.CreatedByID != actor {
		return v, ErrForbidden
	}
	if reason == "" || v.RevokedAt != nil {
		return v, ErrInvalid
	}
	now := s.now().UTC()
	v.RevokedAt = &now
	v.RevokedBy = actor
	v.RevocationReason = reason
	s.deriveStatement(&v)
	return v, s.save(repo, id, v)
}
func (s *Store) deriveStatement(v *Statement) {
	v.Status = "current"
	v.StatusReasons = nil
	changed := false
	for _, rid := range v.RemediationIDs {
		r, e := read[Remediation](s.path(v.RepositoryID, rid))
		if e != nil {
			changed = true
			v.StatusReasons = append(v.StatusReasons, "remediation_unavailable")
			continue
		}
		s.derive(&r)
		if r.Status != "closed" {
			changed = true
			v.StatusReasons = append(v.StatusReasons, "finding_reopened_or_drifted")
		}
	}
	if changed {
		v.Status = "changed"
	}
	if !s.now().Before(v.ExpiresAt) {
		v.Status = "expired"
		v.StatusReasons = append(v.StatusReasons, "statement_expired")
	}
	if v.RevokedAt != nil {
		v.Status = "revoked"
		v.StatusReasons = append(v.StatusReasons, "publisher_revoked")
	}
}
func (s *Store) ListStatements(repo, audience string) ([]Statement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "statement_*.json"))
	if e != nil {
		return nil, e
	}
	out := []Statement{}
	for _, f := range files {
		v, e := read[Statement](f)
		if e != nil {
			return nil, e
		}
		if v.Audience == "repository" && audience != "repository" {
			continue
		}
		s.deriveStatement(&v)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
