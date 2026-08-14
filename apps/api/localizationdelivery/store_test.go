package localizationdelivery

import (
	"errors"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/localizationverification"
)

func TestLocaleReadinessDefersIndependentlyAndRequiresCurrentEvidence(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	policy, err := s.CreatePolicy("repo", "owner", PolicyInput{Name: "regional launch", TargetBranches: []string{"main"}, Paths: []string{"ui/**"}, Locales: []LocaleRequirement{{LocaleID: "fr-CA", Audiences: []string{"canada"}, RiskClasses: []string{"high"}, MinimumCoverage: 95, RequiredChecks: []string{"journey"}, RequiredReviewerIDs: []string{"regional"}}}})
	if err != nil || policy.ID == "" {
		t.Fatal(err)
	}
	c, err := s.SetCandidate("repo", "pull", "candidate", "owner", 0, []CandidateLocale{{LocaleID: "fr-CA", State: "staged", Audience: "canada", RiskClass: "high", Coverage: 94, Reason: "launch"}, {LocaleID: "ar", State: "deferred", FallbackLocale: "en", Reason: "RTL repair pending"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Assess("repo", "pull", "candidate", "main", []string{"ui/cart.tsx"}, nil)
	if err != nil || a.Ready || len(a.Requirements) != 2 {
		t.Fatalf("unexpected assessment: %#v %v", a, err)
	}
	// The deferred Arabic locale contributes no blocker; only the staged locale is governed.
	v := localizationverification.Assessment{Revision: "candidate", Checks: []localizationverification.Check{{Name: "journey", LocaleID: "fr-CA", Status: "passed"}}, Decisions: []localizationverification.Decision{{LocaleID: "fr-CA", ReviewerID: "regional", Decision: "approve"}}}
	_, err = s.SetCandidate("repo", "pull", "candidate", "owner", c.Version, []CandidateLocale{{LocaleID: "fr-CA", State: "staged", Audience: "canada", RiskClass: "high", Coverage: 100, Reason: "current evidence"}, {LocaleID: "ar", State: "deferred", FallbackLocale: "en", Reason: "RTL repair pending"}})
	if err != nil {
		t.Fatal(err)
	}
	a, err = s.Assess("repo", "pull", "candidate", "main", []string{"ui/cart.tsx"}, &v)
	if err != nil || !a.Ready {
		t.Fatalf("expected ready: %#v %v", a, err)
	}
}

func TestPublicationFindingValidationAndRepair(t *testing.T) {
	s, _ := New(t.TempDir())
	c, _ := s.SetCandidate("repo", "pull", "release", "owner", 0, []CandidateLocale{{LocaleID: "es-MX", State: "staged", Audience: "mexico", RiskClass: "standard", Coverage: 100, Reason: "approved"}})
	p, err := s.Publish("repo", "owner", PublicationInput{Kind: "application", ResourceID: "release-1", Version: "1.2.0", Revision: "release", LocaleID: "es-MX", State: "published", FallbackLocale: "es", CandidatePullRequestID: "pull", CandidateVersion: c.Version, Provenance: []string{"locale-plan:1", "verification:pull"}, Reason: "regional launch"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Report("repo", "reader", FindingInput{PublicationID: p.ID, Kind: "cultural_mismatch", Path: "/checkout", Expected: "regional address form", Observed: "US-only state field"})
	if err != nil || f.Revision != "release" {
		t.Fatal(err)
	}
	if _, err = s.LinkRepair("repo", f.ID, "owner", Repair{OwnerKind: "agent", OwnerID: "agent-1", ProposalID: "proposal", TaskID: "task", AcceptanceCriteria: []string{"Mexico address accepted"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("unvalidated repair should fail: %v", err)
	}
	f, err = s.Validate("repo", f.ID, "maintainer", "validated", "reproduced on the published locale")
	if err != nil || f.Validation == nil {
		t.Fatal(err)
	}
	f, err = s.LinkRepair("repo", f.ID, "owner", Repair{OwnerKind: "agent", OwnerID: "agent-1", ProposalID: "proposal", TaskID: "task", AcceptanceCriteria: []string{"Mexico address accepted"}})
	if err != nil || f.Repair == nil || f.Repair.OwnerKind != "agent" {
		t.Fatalf("repair not retained: %#v %v", f, err)
	}
}
