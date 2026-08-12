package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// DecisionReceipt is the immutable bridge between a tally and implementation.
// It records a mandate, not operational authority.
type DecisionReceipt struct {
	ID                    string    `json:"id"`
	ProposalID            string    `json:"proposal_id"`
	CharterVersion        int64     `json:"charter_version"`
	TallyDigest           string    `json:"tally_digest"`
	WinningAlternative    string    `json:"winning_alternative"`
	Scope                 string    `json:"scope"`
	AffectedResources     []string  `json:"affected_resources"`
	ImplementationEffects []string  `json:"implementation_effects"`
	Digest                string    `json:"digest"`
	CreatedAt             time.Time `json:"created_at"`
	AuthorityGranted      bool      `json:"authority_granted"`
}

type ImplementationStep struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Status        string    `json:"status"`
	ResourceRef   string    `json:"resource_ref,omitempty"`
	Detail        string    `json:"detail"`
	ActorID       string    `json:"actor_id"`
	OwnerApproval bool      `json:"resource_owner_approval"`
	CreatedAt     time.Time `json:"created_at"`
}

type Implementation struct {
	ReceiptID             string               `json:"decision_receipt_id"`
	ArtifactKind          string               `json:"artifact_kind"`
	State                 string               `json:"state"`
	Mandate               string               `json:"community_mandate"`
	OwnerApprovalRequired bool                 `json:"resource_owner_approval_required"`
	OperationalAuthority  []string             `json:"operational_authority"`
	Steps                 []ImplementationStep `json:"steps"`
	Blockers              []string             `json:"blockers"`
	AmendmentRequired     bool                 `json:"amendment_required"`
	CreatedAt             time.Time            `json:"created_at"`
	UpdatedAt             time.Time            `json:"updated_at"`
}

type ImplementationInput struct {
	ExpectedReceiptDigest string   `json:"expected_receipt_digest"`
	ArtifactKind          string   `json:"artifact_kind"`
	ResourceRef           string   `json:"resource_ref"`
	Detail                string   `json:"detail"`
	Scope                 string   `json:"scope"`
	AffectedResources     []string `json:"affected_resources"`
	ImplementationEffects []string `json:"implementation_effects"`
	MaterialChange        bool     `json:"material_change"`
}

var implementationKinds = map[string]bool{
	"policy_revision": true, "initiative": true, "task_plan": true,
	"role_transition": true, "access_request": true,
}

func selectedEffects(p GovernedProposal) []string {
	for _, a := range p.Alternatives {
		if p.Tally != nil && a.ID == p.Tally.Winner {
			return append(append([]string{}, p.ImplementationEffects...), a.Effects...)
		}
	}
	return append([]string{}, p.ImplementationEffects...)
}

func newDecisionReceipt(p GovernedProposal, now time.Time) *DecisionReceipt {
	r := &DecisionReceipt{ID: id(), ProposalID: p.ID, CharterVersion: p.CharterVersion, TallyDigest: p.Tally.Digest, WinningAlternative: p.Tally.Winner, Scope: p.Scope, AffectedResources: append([]string{}, p.AffectedResources...), ImplementationEffects: selectedEffects(p), CreatedAt: now, AuthorityGranted: false}
	raw, _ := json.Marshal([]any{r.ProposalID, r.CharterVersion, r.TallyDigest, r.WinningAlternative, r.Scope, r.AffectedResources, r.ImplementationEffects})
	sum := sha256.Sum256(raw)
	r.Digest = hex.EncodeToString(sum[:])
	return r
}

func artifactKind(p GovernedProposal) string {
	switch p.Kind {
	case "initiative", "leadership_nomination":
		if p.Kind == "leadership_nomination" {
			return "role_transition"
		}
		return "initiative"
	case "resource_request", "funding_request", "policy_exception":
		return "access_request"
	case "charter_amendment":
		return "policy_revision"
	default:
		return "task_plan"
	}
}

func newImplementation(p GovernedProposal, now time.Time) *Implementation {
	return &Implementation{ReceiptID: p.DecisionReceipt.ID, ArtifactKind: artifactKind(p), State: "awaiting_owner_approval", Mandate: "Approved by the governed electorate; existing resource controls remain authoritative.", OwnerApprovalRequired: true, OperationalAuthority: []string{}, Steps: []ImplementationStep{}, Blockers: []string{"resource_owner_approval_required"}, CreatedAt: now, UpdatedAt: now}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// RecordImplementation lets the resource owner link an ordinary operational
// artifact or blocker. It never creates credentials or bypasses that resource's
// own review, integration, release, environment, extension, or agent policy.
func (s *Store) RecordImplementation(t, scope, proposal, actor string, in ImplementationInput) (GovernedProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.readProposal(t, scope, proposal)
	if e != nil {
		return v, e
	}
	if v.State != "approved" || v.DecisionReceipt == nil || v.Implementation == nil || in.ExpectedReceiptDigest != v.DecisionReceipt.Digest {
		return v, ErrConflict
	}
	if !implementationKinds[in.ArtifactKind] || in.ArtifactKind != v.Implementation.ArtifactKind || !clean(in.Detail) {
		return v, ErrInvalid
	}
	now := s.now().UTC()
	step := ImplementationStep{ID: id(), Kind: in.ArtifactKind, Detail: strings.TrimSpace(in.Detail), ActorID: actor, CreatedAt: now}
	changed := in.MaterialChange || in.Scope != v.DecisionReceipt.Scope || !sameStrings(in.AffectedResources, v.DecisionReceipt.AffectedResources) || !sameStrings(in.ImplementationEffects, v.DecisionReceipt.ImplementationEffects)
	if changed {
		step.Status = "blocked_amendment_required"
		v.Implementation.State = "blocked"
		v.Implementation.AmendmentRequired = true
		v.Implementation.Blockers = []string{"new_or_amended_decision_required"}
	} else if strings.TrimSpace(in.ResourceRef) == "" {
		step.Status = "blocked"
		v.Implementation.State = "blocked"
		v.Implementation.Blockers = []string{"operational_resource_not_created"}
	} else {
		step.Status, step.ResourceRef, step.OwnerApproval = "routed", strings.TrimSpace(in.ResourceRef), true
		v.Implementation.State = "routed"
		v.Implementation.Blockers = []string{}
	}
	v.Implementation.Steps = append(v.Implementation.Steps, step)
	v.Implementation.UpdatedAt = now
	e = s.writeProposal(t, scope, v)
	return v, e
}
