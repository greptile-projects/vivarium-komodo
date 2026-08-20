package threatmodels

import (
	"errors"
	"testing"
)

func remediationModel() Input {
	return Input{Title: "Callback isolation", Summary: "Bound callback destinations", Origin: Origin{Kind: "api_evolution", Reference: "callbacks", Revision: "base-a"}, Inputs: []InputBinding{{Kind: "code", Reference: "callback.go", Revision: "base-a"}}, EntryPoints: []EntryPoint{{ID: "url", Description: "user URL", Privileges: []string{"choose destination"}}}, AttackerGoals: []AttackerGoal{{ID: "ssrf", Actor: "writer", Goal: "reach metadata", Capability: "choose URL", Impact: "credential theft"}}, Mitigations: []Mitigation{{ID: "pin", Description: "pin validated IP", Status: "proposed"}}, AbusePaths: []AbusePath{{ID: "rebind", GoalID: "ssrf", EntryPointIDs: []string{"url"}, Steps: []string{"rebind DNS"}, MitigationIDs: []string{"pin"}, ResidualRisk: "redirects", Severity: "high", OwnerIDs: []string{"security-owner"}}}, OwnerIDs: []string{"security-owner"}, ResidualRisk: "redirect handling"}
}

func TestFindingRemediationPreservesAudienceAuthorityAndResolution(t *testing.T) {
	s, _ := New(t.TempDir())
	m, _ := s.Create("repo", "reader", remediationModel())
	m, _ = s.AddFinding("repo", m.ID, "finder", FindingInput{Kind: "finding", Body: "DNS rebinding reaches metadata", AbusePathIDs: []string{"rebind"}, Citations: []Citation{{Kind: "test", Reference: "public-repro", Revision: "base-a", Detail: "synthetic callback", Visibility: "public"}, {Kind: "trace", Reference: "private-trace", Revision: "base-a", Detail: "repository trace", Visibility: "repository"}}})
	fid := m.Findings[0].ID
	if _, err := s.ClassifyFinding("repo", m.ID, fid, "finder", "confirmed", "repository", "reproduced"); !errors.Is(err, ErrInvalid) {
		t.Fatal("finder gained owner classification authority")
	}
	m, _ = s.ClassifyFinding("repo", m.ID, fid, "security-owner", "confirmed", "public", "safe synthetic disclosure")
	bad := DeliveryInput{ProposalID: "proposal", TaskID: "task", ResourceKind: "task", OwnerKind: "agent", OwnerID: "agent-1", CandidateRevision: "base-a", AbusePathIDs: []string{"rebind"}, PermittedCitationReferences: []string{"private-trace"}, AcceptanceCriteria: []string{"metadata blocked"}}
	if _, err := s.LinkDelivery("repo", m.ID, fid, "security-owner", bad); !errors.Is(err, ErrInvalid) {
		t.Fatal("public task received repository-only evidence")
	}
	good := bad
	good.PermittedCitationReferences = []string{"public-repro"}
	m, _ = s.LinkDelivery("repo", m.ID, fid, "security-owner", good)
	if m.Findings[0].Delivery == nil || m.Findings[0].Delivery.CandidateRevision != "base-a" {
		t.Fatal("exact remediation context not retained")
	}
	m, _ = s.VerifyDelivery("repo", m.ID, fid, "security-owner", VerificationInput{PullRequestID: "pull", DesignChangeReferences: []string{"design-v2"}, CommitIDs: []string{"repair-b"}, ReviewID: "review", ScenarioID: "scenario", BaseAttemptID: "failed-base", RepairAttemptID: "passing-repair", MitigationCoverage: []string{"pin"}})
	if m.Findings[0].Delivery.VerifiedAt == nil {
		t.Fatal("durable mitigation coverage missing")
	}

	m, _ = s.AddFinding("repo", m.ID, "finder", FindingInput{Kind: "finding", Body: "same path", AbusePathIDs: []string{"rebind"}, Citations: []Citation{{Kind: "test", Reference: "second", Revision: "base-a", Detail: "same behavior", Visibility: "repository"}}})
	second := m.Findings[1].ID
	m, _ = s.ClassifyFinding("repo", m.ID, second, "security-owner", "suspected_duplicate", "embargoed", "likely same root cause")
	m, _ = s.ResolveFinding("repo", m.ID, second, "security-owner", ResolutionInput{Kind: "suspected_duplicate", Rationale: "compare after embargo", DuplicateFindingID: fid})
	if m.Findings[1].Resolution == nil || m.Findings[1].Resolution.OwnerID != "security-owner" {
		t.Fatal("attributable duplicate path missing")
	}
}
