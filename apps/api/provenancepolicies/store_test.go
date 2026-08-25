package provenancepolicies

import (
	"testing"
	"time"
)

func fixture(now time.Time) Input {
	return Input{Name: "Project materials", Description: "Acceptable origins and uses", Rules: []MaterialRule{{Kind: "source", Origins: []string{"original", "federated_contribution"}, Licenses: []string{"Apache-2.0", "unknown"}, Uses: []string{"modify", "public_distribution"}, Attribution: []string{"copyright notice"}, Attestations: []string{"developer-certificate"}, ReviewOwnerIDs: []string{"legal"}}, {Kind: "package", Origins: []string{"verified_registry"}, Licenses: []string{"Apache-2.0"}, Uses: []string{"build", "public_distribution"}}}, DistributionContexts: []DistributionContext{{ID: "public", Audience: "public", Uses: []string{"public_distribution"}, Licenses: []string{"Apache-2.0", "GPL-3.0"}, NoticeRequired: true}}, Links: []Link{{Kind: "contributor_pathway", Reference: "pathway:external"}, {Kind: "agent_contract", Reference: "agent:repair", Revision: "v2"}, {Kind: "contribution_boundary", Reference: "peer:upstream", Boundary: "federated"}}, Exceptions: []Exception{{ID: "legacy", MaterialKinds: []string{"package"}, ContextIDs: []string{"public"}, Rationale: "replace legacy package", OwnerID: "legal", ApprovedBy: "owner", ExpiresAt: now.Add(48 * time.Hour)}}, ChangeReason: "initial"}
}
func TestVersionedPolicyRetainsExplicitBlockers(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p, e := s.Create("repository", "repo", "owner", fixture(now))
	if e != nil {
		t.Fatal(e)
	}
	k := map[string]bool{}
	for _, b := range p.Blockers {
		k[b.Kind] = true
	}
	if !k["unknown_license"] || !k["missing_owner"] || !k["conflicting_terms"] || !k["expiring_exception"] {
		t.Fatalf("missing blockers: %#v", p.Blockers)
	}
	in := fixture(now)
	in.Name = "Revised materials"
	p, e = s.Revise("repository", "repo", p.ID, "owner", 1, in)
	if e != nil || p.CurrentVersion != 2 || len(p.Versions) != 2 {
		t.Fatalf("revision failed: %#v %v", p, e)
	}
	if _, e = s.Revise("repository", "repo", p.ID, "owner", 1, in); e != ErrConflict {
		t.Fatalf("expected conflict, got %v", e)
	}
}
