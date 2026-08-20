package assuranceprograms

import (
	"testing"
	"time"
)

func sample(now time.Time) Input {
	return Input{Name: "Service assurance", Description: "Concrete service obligations", Scope: "payments service production", OwnerIDs: []string{"compliance"}, ChangeReason: "initial mapping", Requirements: []Requirement{{ID: "privacy", SourceKind: "regulatory", SourceReference: "GDPR/32", SourceVersion: "2016", Title: "Security", Text: "protect data", Applicability: "EU customer data", Interpretation: "encrypt transfers", AuthorID: "legal"}, {ID: "privacy-alt", SourceKind: "regulatory", SourceReference: "GDPR/32", SourceVersion: "2016", Title: "Security alternate", Text: "protect data", Applicability: "EU customer data", Interpretation: "encryption optional", InheritedFrom: "organization:baseline-v2", AuthorID: "architect"}}, Controls: []Control{{ID: "transport", Objective: "Protect transfers", Claim: "TLS is required", ReviewPeriod: "quarterly", RequirementIDs: []string{"privacy"}, Targets: []Target{{Kind: "repository", Reference: "payments", Revision: "abc"}, {Kind: "policy", Reference: "tls-policy", Revision: "v3"}, {Kind: "data_flow", Reference: "card-flow", Revision: "v2"}, {Kind: "infrastructure", Reference: "edge", Revision: "infra-v4"}, {Kind: "environment", Reference: "production", Revision: "release-8"}, {Kind: "release", Reference: "release-8", Revision: "abc"}, {Kind: "procedure", Reference: "rotation-runbook", Revision: "v5"}}, EvidenceCriteria: []EvidenceCriterion{{ID: "tls-check", Kind: "automated_check", Description: "TLS policy passes", Frequency: "each release", SourceReference: "check:tls"}}}}, Exceptions: []Exception{{ID: "legacy", RequirementID: "privacy", ControlID: "transport", Rationale: "legacy client", OwnerID: "legal", ApprovalReference: "decision:1", ExpiresAt: now.Add(5 * 24 * time.Hour)}}}
}
func TestProgramDerivesAttributableGapsAndVersions(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	x, e := s.Create("repo", "writer", sample(now))
	if e != nil {
		t.Fatal(e)
	}
	k := map[string]bool{}
	for _, g := range x.Gaps {
		k[g.Kind] = true
		if g.AttributedTo == "" {
			t.Fatalf("gap lacks attribution: %#v", g)
		}
	}
	for _, want := range []string{"conflicting_interpretation", "inherited_obligation", "unmapped_requirement", "expiring_exception"} {
		if !k[want] {
			t.Errorf("missing %s", want)
		}
	}
	inWithoutOwner := sample(now)
	inWithoutOwner.OwnerIDs = nil
	inWithoutOwner.Name = "Owner gap"
	y, e := s.Create("repo", "writer", inWithoutOwner)
	if e != nil || y.Gaps[0].Kind != "missing_owner" {
		t.Fatalf("missing program ownership hidden: %#v %v", y, e)
	}
	in := sample(now)
	in.ChangeReason = "clarified"
	in.Requirements[1].Interpretation = in.Requirements[0].Interpretation
	in.Controls[0].RequirementIDs = append(in.Controls[0].RequirementIDs, "privacy-alt")
	in.Exceptions = nil
	x, e = s.Revise("repo", x.ID, "writer", 1, in)
	if e != nil || x.CurrentVersion != 2 {
		t.Fatalf("revise: %#v %v", x, e)
	}
	if _, e = s.Revise("repo", x.ID, "writer", 1, in); e != ErrConflict {
		t.Fatalf("expected conflict, got %v", e)
	}
}
func TestProgramRejectsUnrevisionedMappings(t *testing.T) {
	s, _ := New(t.TempDir())
	in := sample(time.Now())
	in.Controls[0].Targets[0].Revision = ""
	if _, e := s.Create("repo", "writer", in); e != ErrInvalid {
		t.Fatalf("got %v", e)
	}
}
