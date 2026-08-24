// Package historyremediations owns restricted, pre-inspection history repair scopes.
package historyremediations

import (
	"crypto/rand"
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

var ErrNotFound = errors.New("history remediation not found")
var ErrInvalid = errors.New("invalid history remediation")

type Source struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision string `json:"revision,omitempty"`
}
type Object struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	Kind         string `json:"kind"`
	ObjectID     string `json:"object_id"`
	Path         string `json:"path,omitempty"`
	Digest       string `json:"digest,omitempty"`
	Match        string `json:"match"`
	Reason       string `json:"reason,omitempty"`
	AttributedTo string `json:"attributed_to"`
}
type Scope struct {
	Kind         string `json:"kind"`
	RepositoryID string `json:"repository_id,omitempty"`
	Reference    string `json:"reference"`
	Revision     string `json:"revision,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Reference  string `json:"reference"`
	Revision   string `json:"revision,omitempty"`
	Digest     string `json:"digest"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	RecordedBy string `json:"recorded_by"`
}
type Constraint struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	OwnerID   string `json:"owner_id"`
	Rationale string `json:"rationale"`
}
type Approval struct {
	Kind     string `json:"kind"`
	OwnerID  string `json:"owner_id"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
}
type Input struct {
	Title              string       `json:"title"`
	Source             Source       `json:"source"`
	ContentDescription string       `json:"content_description"`
	Reason             string       `json:"reason"`
	Audience           string       `json:"audience"`
	ResponseOwnerIDs   []string     `json:"response_owner_ids"`
	ParticipantIDs     []string     `json:"participant_ids"`
	Objects            []Object     `json:"objects"`
	Scope              []Scope      `json:"scope"`
	Evidence           []Evidence   `json:"discovery_evidence"`
	Constraints        []Constraint `json:"constraints"`
	Approvals          []Approval   `json:"required_approvals"`
}
type Blocker struct {
	Kind         string `json:"kind"`
	Subject      string `json:"subject"`
	Detail       string `json:"detail"`
	AttributedTo string `json:"attributed_to"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Citation struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision,omitempty"`
	Digest    string `json:"digest"`
	Access    string `json:"access"`
}
type DerivedExposure struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	State     string `json:"state"`
}
type ReachabilityFinding struct {
	ID               string            `json:"id"`
	CopyKind         string            `json:"copy_kind"`
	Reference        string            `json:"reference"`
	RepositoryID     string            `json:"repository_id,omitempty"`
	Revision         string            `json:"revision,omitempty"`
	ObjectIDs        []string          `json:"object_ids"`
	DerivedExposures []DerivedExposure `json:"derived_exposures"`
	Status           string            `json:"status"`
	ControlledBy     string            `json:"controlled_by,omitempty"`
	Summary          string            `json:"summary"`
	Uncertainty      string            `json:"uncertainty,omitempty"`
	Citations        []Citation        `json:"citations"`
	RecordedBy       string            `json:"recorded_by"`
	RecordedAt       time.Time         `json:"recorded_at"`
}
type ReachabilityInput struct {
	CopyKind         string            `json:"copy_kind"`
	Reference        string            `json:"reference"`
	RepositoryID     string            `json:"repository_id,omitempty"`
	Revision         string            `json:"revision,omitempty"`
	ObjectIDs        []string          `json:"object_ids"`
	DerivedExposures []DerivedExposure `json:"derived_exposures"`
	Status           string            `json:"status"`
	ControlledBy     string            `json:"controlled_by,omitempty"`
	Summary          string            `json:"summary"`
	Uncertainty      string            `json:"uncertainty,omitempty"`
	Citations        []Citation        `json:"citations"`
}
type ReachabilitySummary struct {
	ByStatus             map[string]int `json:"by_status"`
	AffectedObjectIDs    []string       `json:"affected_object_ids"`
	DerivedExposureCount int            `json:"derived_exposure_count"`
}

// RewriteRule is immutable once appended. It describes transformation intent
// without retaining the affected payload or a replacement value.
type RewriteRule struct {
	ID                 string    `json:"id"`
	Kind               string    `json:"kind"`
	ObjectIDs          []string  `json:"object_ids"`
	Path               string    `json:"path,omitempty"`
	ReplacementDigest  string    `json:"replacement_digest,omitempty"`
	PreserveAuthorship bool      `json:"preserve_authorship"`
	PreserveTimestamps bool      `json:"preserve_timestamps"`
	SignaturePolicy    string    `json:"signature_policy"`
	Rationale          string    `json:"rationale"`
	CreatedBy          string    `json:"created_by"`
	CreatedAt          time.Time `json:"created_at"`
}
type RewriteRuleInput struct {
	Kind               string   `json:"kind"`
	ObjectIDs          []string `json:"object_ids"`
	Path               string   `json:"path,omitempty"`
	ReplacementDigest  string   `json:"replacement_digest,omitempty"`
	PreserveAuthorship bool     `json:"preserve_authorship"`
	PreserveTimestamps bool     `json:"preserve_timestamps"`
	SignaturePolicy    string   `json:"signature_policy"`
	Rationale          string   `json:"rationale"`
}
type RefReplacement struct {
	Reference   string `json:"reference"`
	OldRevision string `json:"old_revision"`
	NewRevision string `json:"new_revision"`
}
type CommitMapping struct {
	OldCommit           string `json:"old_commit"`
	NewCommit           string `json:"new_commit"`
	AuthorshipPreserved bool   `json:"authorship_preserved"`
	SignatureStatus     string `json:"signature_status"`
}
type LinkImpact struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Action    string `json:"action,omitempty"`
}
type RewriteCandidateInput struct {
	RuleIDs               []string         `json:"rule_ids"`
	Refs                  []RefReplacement `json:"refs"`
	CommitMap             []CommitMapping  `json:"commit_map"`
	UnaffectedDigest      string           `json:"unaffected_content_digest"`
	CandidateDigest       string           `json:"candidate_digest"`
	ChangedObjectIDs      []string         `json:"changed_object_ids"`
	StorageBeforeBytes    int64            `json:"storage_before_bytes"`
	StorageAfterBytes     int64            `json:"storage_after_bytes"`
	RollbackUntil         time.Time        `json:"rollback_until"`
	RollbackLimits        []string         `json:"rollback_limits"`
	CollaboratorActions   []string         `json:"collaborator_actions"`
	LinkImpacts           []LinkImpact     `json:"link_impacts"`
	UnrewritableResources []string         `json:"unrewritable_resources"`
}
type RewriteCandidate struct {
	ID string `json:"id"`
	RewriteCandidateInput
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	Published bool      `json:"published"`
}
type RehearsalCheck struct {
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	Reference string `json:"reference"`
	Digest    string `json:"digest,omitempty"`
	Summary   string `json:"summary"`
}
type RehearsalInput struct {
	CandidateID     string           `json:"candidate_id"`
	Environment     string           `json:"environment"`
	BudgetMinutes   int              `json:"budget_minutes"`
	BudgetCost      int64            `json:"budget_cost"`
	Checks          []RehearsalCheck `json:"checks"`
	ObservedMinutes int              `json:"observed_minutes"`
	ObservedCost    int64            `json:"observed_cost"`
}
type Rehearsal struct {
	ID string `json:"id"`
	RehearsalInput
	Status     string    `json:"status"`
	Blockers   []Blocker `json:"blockers"`
	RecordedBy string    `json:"recorded_by"`
	RecordedAt time.Time `json:"recorded_at"`
}
type Attestation struct {
	Digest    string `json:"digest"`
	SignerID  string `json:"signer_id"`
	Signature string `json:"signature"`
}
type CredentialAction struct {
	Reference string `json:"reference"`
	Action    string `json:"action"`
	Receipt   string `json:"receipt"`
}
type Pause struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Guidance  string `json:"guidance"`
}
type MigrationTarget struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Reference    string `json:"reference"`
	OwnerID      string `json:"owner_id"`
	Audience     string `json:"audience"`
	Authority    string `json:"authority"`
	Instructions string `json:"instructions"`
	Mapping      string `json:"mapping"`
	Status       string `json:"status"`
	Receipt      string `json:"receipt,omitempty"`
}
type MigrationDecisionInput struct {
	Status  string `json:"status"`
	Receipt string `json:"receipt"`
}
type PublicationInput struct {
	CandidateID          string             `json:"candidate_id"`
	ExpectedUpdatedAt    time.Time          `json:"expected_updated_at"`
	Attestation          Attestation        `json:"attestation"`
	QuarantinedObjectIDs []string           `json:"quarantined_object_ids"`
	CredentialActions    []CredentialAction `json:"credential_actions"`
	Pauses               []Pause            `json:"pauses"`
	MigrationTargets     []MigrationTarget  `json:"migration_targets"`
}
type Publication struct {
	ID                   string             `json:"id"`
	CandidateID          string             `json:"candidate_id"`
	Refs                 []RefReplacement   `json:"refs"`
	Attestation          Attestation        `json:"attestation"`
	QuarantinedObjectIDs []string           `json:"quarantined_object_ids"`
	CredentialActions    []CredentialAction `json:"credential_actions"`
	Pauses               []Pause            `json:"pauses"`
	MigrationTargets     []MigrationTarget  `json:"migration_targets"`
	PublishedBy          string             `json:"published_by"`
	PublishedAt          time.Time          `json:"published_at"`
}
type RefPublisher func([]RefReplacement) error
type Remediation struct {
	ID                  string                `json:"id"`
	RepositoryID        string                `json:"repository_id"`
	CreatedByID         string                `json:"created_by_id"`
	Input               Input                 `json:"definition"`
	Blockers            []Blocker             `json:"blockers"`
	Events              []Event               `json:"history"`
	Reachability        []ReachabilityFinding `json:"reachability_map"`
	ReachabilitySummary ReachabilitySummary   `json:"reachability_summary"`
	RewriteRules        []RewriteRule         `json:"rewrite_rules"`
	Candidates          []RewriteCandidate    `json:"rewrite_candidates"`
	Rehearsals          []Rehearsal           `json:"rewrite_rehearsals"`
	Publications        []Publication         `json:"publications"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
}
type Catalog struct {
	Items []Remediation `json:"items"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func oneOf(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func text(v string) bool {
	v = strings.ToLower(v)
	return !strings.Contains(v, "-----begin") && !strings.Contains(v, "password=") && !strings.Contains(v, "authorization: bearer") && len(v) <= 2000
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.ContentDescription) == "" || strings.TrimSpace(in.Reason) == "" || !text(in.ContentDescription) || !text(in.Reason) || !oneOf(in.Source.Kind, "security_finding", "privacy_incident", "support_case", "selected_object") || in.Source.ID == "" || !oneOf(in.Audience, "owners_only", "response_team", "named_participants") || len(in.ResponseOwnerIDs) == 0 || len(in.Objects) == 0 || len(in.Scope) == 0 || len(in.Evidence) == 0 || len(in.Approvals) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, o := range in.Objects {
		if o.ID == "" || seen[o.ID] || o.RepositoryID == "" || o.ObjectID == "" || o.AttributedTo == "" || !oneOf(o.Kind, "commit", "tree", "blob", "tag", "path") || !oneOf(o.Match, "confirmed", "false_match", "inaccessible") || !text(o.Reason) {
			return false
		}
		seen[o.ID] = true
	}
	for _, s := range in.Scope {
		if s.Reference == "" || !oneOf(s.Kind, "repository", "ref", "release", "package", "artifact", "environment") {
			return false
		}
	}
	seen = map[string]bool{}
	for _, e := range in.Evidence {
		if e.ID == "" || seen[e.ID] || e.Reference == "" || e.Digest == "" || e.Summary == "" || e.RecordedBy == "" || !text(e.Summary) || !text(e.Reason) || !oneOf(e.Status, "available", "inaccessible", "expired") {
			return false
		}
		seen[e.ID] = true
	}
	for _, c := range in.Constraints {
		if c.ID == "" || c.Reference == "" || c.OwnerID == "" || c.Rationale == "" || !text(c.Rationale) || !oneOf(c.Kind, "legal_hold", "retention", "continuity") || !oneOf(c.Status, "clear", "conflict", "inaccessible") {
			return false
		}
	}
	for _, a := range in.Approvals {
		if a.OwnerID == "" || !oneOf(a.Kind, "repository_owner", "security_owner", "privacy_owner", "legal_owner", "release_owner", "environment_owner") || !oneOf(a.Status, "pending", "approved", "denied") {
			return false
		}
	}
	return true
}
func derive(in Input) []Blocker {
	out := []Blocker{}
	for _, o := range in.Objects {
		if o.Match != "confirmed" {
			out = append(out, Blocker{o.Match, o.ID, o.Reason, o.AttributedTo})
		}
	}
	for _, e := range in.Evidence {
		if e.Status != "available" {
			out = append(out, Blocker{"evidence_" + e.Status, e.ID, e.Reason, e.RecordedBy})
		}
	}
	for _, c := range in.Constraints {
		if c.Status != "clear" {
			out = append(out, Blocker{c.Kind + "_" + c.Status, c.Reference, c.Rationale, c.OwnerID})
		}
	}
	for _, a := range in.Approvals {
		if a.Required && a.Status != "approved" {
			out = append(out, Blocker{"approval_" + a.Status, a.Kind, "required approval is " + a.Status, a.OwnerID})
		}
	}
	return out
}
func validReachability(in ReachabilityInput) bool {
	if !oneOf(in.CopyKind, "branch", "tag", "pull_request", "fork", "federated_contribution", "workspace", "checkpoint", "cache", "package", "release_artifact", "documentation", "deployment", "backup", "active_clone") || in.Reference == "" || !text(in.Reference) || !text(in.Revision) || len(in.ObjectIDs) == 0 || !oneOf(in.Status, "confirmed", "suspected", "unreachable", "independently_controlled", "unverifiable") || in.Summary == "" || !text(in.Summary) || !text(in.Uncertainty) || len(in.Citations) == 0 {
		return false
	}
	if in.Status == "independently_controlled" && in.ControlledBy == "" {
		return false
	}
	seen := map[string]bool{}
	for _, id := range in.ObjectIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	for _, d := range in.DerivedExposures {
		if !oneOf(d.Kind, "credential", "personal_data", "restricted_data", "derived_data") || d.Reference == "" || !text(d.Reference) || !oneOf(d.State, "active", "rotated", "revoked", "deleted", "unknown") {
			return false
		}
	}
	for _, c := range in.Citations {
		if c.Kind == "" || c.Reference == "" || c.Digest == "" || !text(c.Kind) || !text(c.Reference) || !text(c.Revision) || !text(c.Digest) || !oneOf(c.Access, "available", "restricted", "inaccessible", "expired") {
			return false
		}
	}
	return true
}
func validRule(in RewriteRuleInput) bool {
	if !oneOf(in.Kind, "remove_object", "replace_object", "remove_path", "replace_path", "rewrite_metadata") || len(in.ObjectIDs) == 0 || !oneOf(in.SignaturePolicy, "preserve_if_unchanged", "resign", "accept_breakage") || in.Rationale == "" || !text(in.Path) || !text(in.ReplacementDigest) || !text(in.Rationale) {
		return false
	}
	if strings.HasPrefix(in.Kind, "replace_") && in.ReplacementDigest == "" {
		return false
	}
	seen := map[string]bool{}
	for _, id := range in.ObjectIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func validCandidate(in RewriteCandidateInput) bool {
	if len(in.RuleIDs) == 0 || len(in.Refs) == 0 || len(in.CommitMap) == 0 || in.UnaffectedDigest == "" || in.CandidateDigest == "" || len(in.ChangedObjectIDs) == 0 || in.StorageBeforeBytes < 0 || in.StorageAfterBytes < 0 || in.RollbackUntil.IsZero() || !text(in.UnaffectedDigest) || !text(in.CandidateDigest) {
		return false
	}
	seen := map[string]bool{}
	for _, r := range in.Refs {
		if r.Reference == "" || r.OldRevision == "" || r.NewRevision == "" || r.OldRevision == r.NewRevision || seen[r.Reference] {
			return false
		}
		seen[r.Reference] = true
	}
	seen = map[string]bool{}
	for _, m := range in.CommitMap {
		if m.OldCommit == "" || m.NewCommit == "" || m.OldCommit == m.NewCommit || seen[m.OldCommit] || !oneOf(m.SignatureStatus, "preserved", "broken", "resigned", "unsigned") {
			return false
		}
		seen[m.OldCommit] = true
	}
	for _, x := range append(append([]string{}, in.RollbackLimits...), in.CollaboratorActions...) {
		if x == "" || !text(x) {
			return false
		}
	}
	for _, x := range in.LinkImpacts {
		if x.Kind == "" || x.Reference == "" || !oneOf(x.Status, "preserved", "broken", "redirectable", "unknown") || !text(x.Action) {
			return false
		}
	}
	return true
}
func rehearsalBlockers(in RehearsalInput) ([]Blocker, bool) {
	required := map[string]bool{"integrity": false, "build": false, "check": false, "release": false, "dependency": false, "clone": false, "fetch": false}
	out := []Blocker{}
	for _, c := range in.Checks {
		if _, ok := required[c.Domain]; !ok || !oneOf(c.Status, "passed", "failed", "blocked", "not_applicable") || c.Reference == "" || c.Summary == "" || !text(c.Summary) {
			return nil, false
		}
		required[c.Domain] = true
		if c.Status != "passed" {
			out = append(out, Blocker{"rehearsal_" + c.Status, c.Domain, c.Summary, c.Reference})
		}
	}
	for d, ok := range required {
		if !ok {
			out = append(out, Blocker{"missing_rehearsal_check", d, "required rehearsal domain was not run", "system"})
		}
	}
	if in.BudgetMinutes <= 0 || in.BudgetCost < 0 || in.ObservedMinutes < 0 || in.ObservedCost < 0 || in.Environment == "" || in.CandidateID == "" {
		return nil, false
	}
	if in.ObservedMinutes > in.BudgetMinutes || in.ObservedCost > in.BudgetCost {
		out = append(out, Blocker{"budget_exhausted", in.CandidateID, "rehearsal exceeded its declared bound", "system"})
	}
	return out, true
}
func validPublication(in PublicationInput) bool {
	if in.CandidateID == "" || in.ExpectedUpdatedAt.IsZero() || in.Attestation.Digest == "" || in.Attestation.SignerID == "" || in.Attestation.Signature == "" || len(in.QuarantinedObjectIDs) == 0 || len(in.Pauses) == 0 || len(in.MigrationTargets) == 0 || !text(in.Attestation.Digest) || !text(in.Attestation.Signature) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range in.QuarantinedObjectIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	for _, a := range in.CredentialActions {
		if a.Reference == "" || !oneOf(a.Action, "revoke", "rotate") || a.Receipt == "" || !text(a.Receipt) {
			return false
		}
	}
	kinds := map[string]bool{}
	for _, p := range in.Pauses {
		if !oneOf(p.Kind, "push", "queue", "session", "workflow", "release") || p.Reference == "" || p.Status != "paused" || p.Guidance == "" || !text(p.Guidance) {
			return false
		}
		kinds[p.Kind] = true
	}
	for _, kind := range []string{"push", "queue", "session", "workflow", "release"} {
		if !kinds[kind] {
			return false
		}
	}
	seen = map[string]bool{}
	for _, m := range in.MigrationTargets {
		if m.ID == "" || seen[m.ID] || !oneOf(m.Kind, "local_branch", "fork", "federated_copy", "open_pull_request", "integration") || m.Reference == "" || m.OwnerID == "" || !oneOf(m.Audience, "owner", "participants", "public") || !oneOf(m.Authority, "coordinator", "independent_owner") || m.Instructions == "" || !oneOf(m.Mapping, "full", "redacted", "unavailable") || !oneOf(m.Status, "pending", "acknowledged", "migrated") || !text(m.Instructions) {
			return false
		}
		seen[m.ID] = true
	}
	return true
}

// Publish atomically swaps every selected ref through the repository storage
// callback before recording containment and migration state. Independent targets
// remain instructions and acknowledgements, never delegated authority.
func (s *Store) Publish(repo, id, actor string, in PublicationInput, publish RefPublisher) (Remediation, error) {
	if actor == "" || publish == nil || !validPublication(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !responder(x, actor) {
		return Remediation{}, ErrNotFound
	}
	if !x.UpdatedAt.Equal(in.ExpectedUpdatedAt) || len(derive(x.Input)) != 0 || len(x.Publications) != 0 {
		return Remediation{}, ErrInvalid
	}
	var candidate *RewriteCandidate
	for i := range x.Candidates {
		if x.Candidates[i].ID == in.CandidateID && !x.Candidates[i].Published {
			candidate = &x.Candidates[i]
		}
	}
	if candidate == nil || in.Attestation.Digest != candidate.CandidateDigest {
		return Remediation{}, ErrInvalid
	}
	passed := false
	for _, rehearsal := range x.Rehearsals {
		if rehearsal.CandidateID == candidate.ID {
			passed = rehearsal.Status == "passed"
		}
	}
	if !passed {
		return Remediation{}, ErrInvalid
	}
	changed := map[string]bool{}
	for _, oid := range candidate.ChangedObjectIDs {
		changed[oid] = true
	}
	for _, oid := range in.QuarantinedObjectIDs {
		if !changed[oid] {
			return Remediation{}, ErrInvalid
		}
	}
	if e = publish(candidate.Refs); e != nil {
		return Remediation{}, e
	}
	now := s.now().UTC()
	candidate.Published = true
	p := Publication{ID: ident(), CandidateID: candidate.ID, Refs: append([]RefReplacement{}, candidate.Refs...), Attestation: in.Attestation, QuarantinedObjectIDs: append([]string{}, in.QuarantinedObjectIDs...), CredentialActions: append([]CredentialAction{}, in.CredentialActions...), Pauses: append([]Pause{}, in.Pauses...), MigrationTargets: append([]MigrationTarget{}, in.MigrationTargets...), PublishedBy: actor, PublishedAt: now}
	x.Publications = append(x.Publications, p)
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "rewrite.published", actor, now})
	return x, s.save(x)
}

func (s *Store) PushPause(repo string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return false, ""
	}
	for _, x := range xs {
		for _, p := range x.Publications {
			for _, pause := range p.Pauses {
				if pause.Kind == "push" && pause.Status == "paused" {
					return true, pause.Guidance
				}
			}
		}
	}
	return false, ""
}

func (s *Store) RecordMigration(repo, id, targetID, actor string, in MigrationDecisionInput) (Remediation, error) {
	if actor == "" || targetID == "" || !oneOf(in.Status, "acknowledged", "migrated") || in.Receipt == "" || !text(in.Receipt) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil {
		return Remediation{}, ErrNotFound
	}
	found := false
	for pi := range x.Publications {
		for mi := range x.Publications[pi].MigrationTargets {
			m := &x.Publications[pi].MigrationTargets[mi]
			if m.ID == targetID && m.OwnerID == actor && m.Status == "pending" {
				m.Status = in.Status
				m.Receipt = in.Receipt
				found = true
			}
		}
	}
	if !found {
		return Remediation{}, ErrNotFound
	}
	now := s.now().UTC()
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "migration." + in.Status, actor, now})
	return x, s.save(x)
}
func summarize(xs []ReachabilityFinding) ReachabilitySummary {
	s := ReachabilitySummary{ByStatus: map[string]int{}}
	objects := map[string]bool{}
	for _, x := range xs {
		s.ByStatus[x.Status]++
		s.DerivedExposureCount += len(x.DerivedExposures)
		for _, id := range x.ObjectIDs {
			objects[id] = true
		}
	}
	for id := range objects {
		s.AffectedObjectIDs = append(s.AffectedObjectIDs, id)
	}
	sort.Strings(s.AffectedObjectIDs)
	return s
}
func ident() string                          { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, id string) string { return filepath.Join(s.root, repo, id+".json") }
func (s *Store) save(x Remediation) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.ID), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) Create(repo, actor string, in Input) (Remediation, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	x := Remediation{ID: ident(), RepositoryID: repo, CreatedByID: actor, Input: in, Blockers: derive(in), Events: []Event{{1, "remediation.opened", actor, now}}, Reachability: []ReachabilityFinding{}, ReachabilitySummary: summarize(nil), RewriteRules: []RewriteRule{}, Candidates: []RewriteCandidate{}, Rehearsals: []Rehearsal{}, Publications: []Publication{}, CreatedAt: now, UpdatedAt: now}
	return x, s.save(x)
}
func (s *Store) AddRewriteRule(repo, id, actor string, in RewriteRuleInput) (Remediation, error) {
	if actor == "" || !validRule(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !responder(x, actor) {
		return Remediation{}, ErrNotFound
	}
	affected := map[string]bool{}
	for _, o := range x.Input.Objects {
		if o.Match == "confirmed" {
			affected[o.ObjectID] = true
		}
	}
	for _, oid := range in.ObjectIDs {
		if !affected[oid] {
			return Remediation{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	x.RewriteRules = append(x.RewriteRules, RewriteRule{ID: ident(), Kind: in.Kind, ObjectIDs: append([]string{}, in.ObjectIDs...), Path: in.Path, ReplacementDigest: in.ReplacementDigest, PreserveAuthorship: in.PreserveAuthorship, PreserveTimestamps: in.PreserveTimestamps, SignaturePolicy: in.SignaturePolicy, Rationale: in.Rationale, CreatedBy: actor, CreatedAt: now})
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "rewrite_rule.created", actor, now})
	return x, s.save(x)
}
func (s *Store) AddCandidate(repo, id, actor string, in RewriteCandidateInput) (Remediation, error) {
	if actor == "" || !validCandidate(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !responder(x, actor) {
		return Remediation{}, ErrNotFound
	}
	rules := map[string]bool{}
	for _, r := range x.RewriteRules {
		rules[r.ID] = true
	}
	for _, rid := range in.RuleIDs {
		if !rules[rid] {
			return Remediation{}, ErrInvalid
		}
	}
	scoped := map[string]string{}
	for _, v := range x.Input.Scope {
		if v.Kind == "ref" {
			scoped[v.Reference] = v.Revision
		}
	}
	for _, r := range in.Refs {
		if scoped[r.Reference] != r.OldRevision {
			return Remediation{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	x.Candidates = append(x.Candidates, RewriteCandidate{ID: ident(), RewriteCandidateInput: in, CreatedBy: actor, CreatedAt: now, Published: false})
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "rewrite_candidate.assembled", actor, now})
	return x, s.save(x)
}
func (s *Store) AddRehearsal(repo, id, actor string, in RehearsalInput) (Remediation, error) {
	blockers, ok := rehearsalBlockers(in)
	if actor == "" || !ok {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !responder(x, actor) {
		return Remediation{}, ErrNotFound
	}
	found := false
	for _, c := range x.Candidates {
		if c.ID == in.CandidateID && !c.Published {
			found = true
		}
	}
	if !found {
		return Remediation{}, ErrInvalid
	}
	now := s.now().UTC()
	status := "passed"
	if len(blockers) > 0 {
		status = "blocked"
	}
	x.Rehearsals = append(x.Rehearsals, Rehearsal{ID: ident(), RehearsalInput: in, Status: status, Blockers: blockers, RecordedBy: actor, RecordedAt: now})
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{int64(len(x.Events) + 1), "rewrite_rehearsal." + status, actor, now})
	return x, s.save(x)
}
func (s *Store) AddReachability(repo, id, actor string, in ReachabilityInput) (Remediation, error) {
	if actor == "" || !validReachability(in) {
		return Remediation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !participant(x, actor) {
		return Remediation{}, ErrNotFound
	}
	affected := map[string]bool{}
	for _, object := range x.Input.Objects {
		affected[object.ObjectID] = true
	}
	for _, objectID := range in.ObjectIDs {
		if !affected[objectID] {
			return Remediation{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	x.Reachability = append(x.Reachability, ReachabilityFinding{ID: ident(), CopyKind: in.CopyKind, Reference: in.Reference, RepositoryID: in.RepositoryID, Revision: in.Revision, ObjectIDs: append([]string{}, in.ObjectIDs...), DerivedExposures: append([]DerivedExposure{}, in.DerivedExposures...), Status: in.Status, ControlledBy: in.ControlledBy, Summary: in.Summary, Uncertainty: in.Uncertainty, Citations: append([]Citation{}, in.Citations...), RecordedBy: actor, RecordedAt: now})
	x.ReachabilitySummary = summarize(x.Reachability)
	x.UpdatedAt = now
	x.Events = append(x.Events, Event{Sequence: int64(len(x.Events) + 1), Type: "reachability.recorded", ActorID: actor, CreatedAt: now})
	return x, s.save(x)
}
func participant(x Remediation, actor string) bool {
	if actor == x.CreatedByID {
		return true
	}
	for _, id := range append(append([]string{}, x.Input.ResponseOwnerIDs...), x.Input.ParticipantIDs...) {
		if actor == id {
			return true
		}
	}
	for _, a := range x.Input.Approvals {
		if actor == a.OwnerID {
			return true
		}
	}
	for _, p := range x.Publications {
		for _, m := range p.MigrationTargets {
			if actor == m.OwnerID {
				return true
			}
		}
	}
	return false
}
func responder(x Remediation, actor string) bool {
	if actor == x.CreatedByID {
		return true
	}
	for _, id := range x.Input.ResponseOwnerIDs {
		if actor == id {
			return true
		}
	}
	return false
}
func (s *Store) Get(repo, id, actor string) (Remediation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, id)
	if e != nil || !participant(x, actor) {
		return Remediation{}, ErrNotFound
	}
	return x, nil
}
func (s *Store) Catalog(repo, actor string) (Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	xs, e := s.list(repo)
	if e != nil {
		return Catalog{}, e
	}
	out := []Remediation{}
	for _, x := range xs {
		if participant(x, actor) {
			out = append(out, x)
		}
	}
	return Catalog{out}, nil
}
func (s *Store) read(repo, id string) (Remediation, error) {
	b, e := os.ReadFile(s.path(repo, id))
	if errors.Is(e, fs.ErrNotExist) {
		return Remediation{}, ErrNotFound
	}
	var x Remediation
	if e != nil || json.Unmarshal(b, &x) != nil || x.RepositoryID != repo || x.ID != id {
		return Remediation{}, ErrNotFound
	}
	x.Blockers = derive(x.Input)
	if x.Reachability == nil {
		x.Reachability = []ReachabilityFinding{}
	}
	if x.RewriteRules == nil {
		x.RewriteRules = []RewriteRule{}
	}
	if x.Candidates == nil {
		x.Candidates = []RewriteCandidate{}
	}
	if x.Rehearsals == nil {
		x.Rehearsals = []Rehearsal{}
	}
	if x.Publications == nil {
		x.Publications = []Publication{}
	}
	x.ReachabilitySummary = summarize(x.Reachability)
	return x, nil
}
func (s *Store) list(repo string) ([]Remediation, error) {
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Remediation{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Remediation{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.read(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
