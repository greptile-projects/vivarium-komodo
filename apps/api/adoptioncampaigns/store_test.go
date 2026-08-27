package adoptioncampaigns

import (
	"errors"
	"testing"
	"time"
)

func definition() VersionInput {
	return VersionInput{ReleaseID: "release-2", ReleaseVersion: "2.0.0", ReleaseRevision: "commit-2", BundleID: "bundle-2", BundleDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Title: "Adopt 2.0", Why: "Deliver the safer API", Audiences: []Audience{{ID: "api-users", Name: "API users", Kind: "api_consumers", DesiredCoverage: 90, OwnerID: "ecosystem"}}, StartingVersions: []StartingVersion{{Version: "1.x", Supported: true, UpgradePath: "docs:migrate", OwnerID: "maintainer"}, {Version: "0.x", Supported: false, OwnerID: "legacy-owner"}}, Deadline: time.Now().UTC().Add(30 * 24 * time.Hour), Measures: []Measure{{ID: "coverage", Description: "eligible clients verified", Target: 90, Unit: "percent", Evidence: "adoption records", OwnerID: "analyst"}}, SupportPolicy: "Support 1.x upgrades for 90 days", RollbackPolicy: "Restore 1.x when compatibility checks fail", OwnerIDs: []string{"maintainer"}, Links: []Link{{Kind: "change", ResourceID: "pull-2", Revision: "commit-2", Summary: "breaking API change", OwnerID: "maintainer"}, {Kind: "documentation", ResourceID: "migration", Revision: "docs-2", Summary: "migration guide", OwnerID: "docs"}}, Compatibility: []Compatibility{{Subject: "runtime", Requirement: "runtime 22 or newer", StartingVersions: []string{"1.x"}, OwnerID: "runtime-owner"}}, ChangeReason: "make adoption accountable"}
}

func TestVersionedCampaignRetainsReleaseAndFindings(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	in := definition()
	c, e := s.Create("repo", "maintainer", in)
	if e != nil {
		t.Fatal(e)
	}
	if c.AuthorityGranted || len(c.Findings) != 1 || c.Findings[0].Kind != "unsupported_upgrade_path" {
		t.Fatalf("unexpected campaign: %+v", c)
	}
	next := in
	next.SupportPolicy = "Support upgrades for 180 days"
	next.ChangeReason = "extend support after adopter review"
	c, e = s.Revise("repo", c.ID, "maintainer", 1, next)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, f := range c.Findings {
		found = found || f.Kind == "changed_commitment"
	}
	if !found {
		t.Fatalf("changed commitment was not retained: %+v", c.Findings)
	}
	changed := next
	changed.ReleaseID = "release-3"
	if _, e = s.Revise("repo", c.ID, "maintainer", 2, changed); !errors.Is(e, ErrInvalid) {
		t.Fatalf("release binding changed: %v", e)
	}
	if _, e = s.Revise("repo", c.ID, "maintainer", 1, next); !errors.Is(e, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", e)
	}
}
