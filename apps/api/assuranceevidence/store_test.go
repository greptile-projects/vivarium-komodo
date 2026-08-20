package assuranceevidence

import (
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
)

func TestEvidencePackagesAreExactHashedAndPermissionProjected(t *testing.T) {
	programs, _ := assuranceprograms.New(t.TempDir())
	program, _ := programs.Create("repo", "owner", assuranceprograms.Input{Name: "SOC", Description: "delivery", Scope: "production", ChangeReason: "initial", Requirements: []assuranceprograms.Requirement{{ID: "req", SourceKind: "contractual", SourceReference: "contract", SourceVersion: "1", Title: "review", Text: "review changes", Applicability: "service", Interpretation: "pull reviews", AuthorID: "legal"}}, Controls: []assuranceprograms.Control{{ID: "reviewed", Objective: "review changes", Claim: "all changes reviewed", ReviewPeriod: "quarterly", RequirementIDs: []string{"req"}, OwnerIDs: []string{"owner"}, Targets: []assuranceprograms.Target{{Kind: "repository", Reference: "repo", Revision: "abc"}}, EvidenceCriteria: []assuranceprograms.EvidenceCriterion{{ID: "reviews", Kind: "review", Description: "merged reviews", Frequency: "daily", SourceReference: "pulls"}}}}})
	s, _ := New(t.TempDir(), programs)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	pub, _ := s.CreateQuery("repo", program.ID, "owner", QueryInput{ControlVersion: 1, ControlID: "reviewed", Name: "merged reviews", Kind: "review", Source: "/repositories/repo/pulls", Selector: map[string]string{"state": "merged"}, Schedule: "0 2 * * *", FreshnessHours: 48, Audience: "public", Transformations: []string{"count by reviewer"}})
	private, _ := s.CreateQuery("repo", program.ID, "owner", QueryInput{ControlVersion: 1, ControlID: "reviewed", Name: "access decisions", Kind: "access", Source: "/repositories/repo/access", Schedule: "0 3 * * *", FreshnessHours: 48, Audience: "repository"})
	in := CollectInput{ControlVersion: 1, ControlID: "reviewed", PeriodStart: now.Add(-24 * time.Hour), PeriodEnd: now, Records: []Record{{QueryID: pub.ID, SourceRecordID: "pull:7", SourceRevision: "merge-abc", ObservedAt: now.Add(-time.Hour), SourceDigest: strings.Repeat("a", 64), SourceAttestation: "server:pulls", Audience: "public", Accessible: true, Result: "approved"}, {QueryID: private.ID, SourceRecordID: "grant:2", SourceRevision: "event-2", ObservedAt: now.Add(-time.Hour), SourceDigest: strings.Repeat("b", 64), SourceAttestation: "server:access", Audience: "repository", Accessible: true, Result: "least_privilege"}}}
	pkg, e := s.Collect("repo", program.ID, "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(pkg.PackageHash) != 64 || !strings.Contains(pkg.Attestation, pkg.PackageHash) || !pkg.Immutable || !pkg.Fresh {
		t.Fatalf("package is not immutable attested current evidence: %#v", pkg)
	}
	public, _ := s.Catalog("repo", program.ID, "public")
	if len(public.Queries) != 1 || len(public.Packages[0].Records) != 1 || public.Packages[0].Records[0].SourceRecordID != "pull:7" {
		t.Fatalf("least privilege projection leaked or hid records: %#v", public)
	}
	if _, e = s.Collect("repo", program.ID, "owner", CollectInput{ControlVersion: 1, ControlID: "reviewed", PeriodStart: now.Add(-time.Hour), PeriodEnd: now, Records: []Record{{QueryID: pub.ID, SourceRecordID: "bad", SourceRevision: "x", ObservedAt: now, SourceDigest: strings.Repeat("c", 64), SourceAttestation: "source", Audience: "public", Accessible: true, ContainsCredentials: true, Result: "pass"}}}); e != ErrInvalid {
		t.Fatalf("credential-bearing evidence accepted: %v", e)
	}
}
