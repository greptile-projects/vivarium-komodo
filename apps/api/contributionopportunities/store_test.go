package contributionopportunities

import (
	"errors"
	"testing"
	"time"
)

func TestMatchingClaimsExpireWithoutAuthority(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	in := Input{Source: Source{Kind: "issue", ResourceID: "issue-1"}, RequiredSkills: []string{"Go", "testing"}, Interests: []string{"developer experience"}, ExpectedOutcome: "A regression test and focused fix", Scope: []string{"apps/api/widget"}, Dependencies: []string{}, Risk: "low", MentorIDs: []string{"maintainer"}, Assistance: "human_or_agent"}
	o, err := s.Publish("repo", "owner", "Fix confusing setup", "abc123", "triaged", true, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Profile("repo", "alice", Profile{Interests: []string{"developer experience"}, Skills: []string{"Go"}, MaxRisk: "medium", AvailableHours: 8, Assistance: "agent"}); err != nil {
		t.Fatal(err)
	}
	m, err := s.Matches("repo", "alice")
	if err != nil || len(m) != 1 || m[0].Score != 100 || m[0].GrantsWriteAccess {
		t.Fatalf("unexpected match %#v %v", m, err)
	}
	c, err := s.Claim("repo", o.ID, "alice", "starting", 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Claim("repo", o.ID, "bob", "duplicate", 24); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected exclusive claim, got %v", err)
	}
	now = now.Add(25 * time.Hour)
	if _, err = s.Claim("repo", o.ID, "bob", "claim after expiry", 24); err != nil {
		t.Fatalf("expired claim blocked work: %v", err)
	}
	if _, err = s.Release("repo", c.ID, "bob"); !errors.Is(err, ErrConflict) {
		t.Fatalf("non-owner released claim: %v", err)
	}
}

func TestSourceCannotBePublishedTwice(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Source: Source{Kind: "proposal", ResourceID: "p"}, RequiredSkills: []string{"writing"}, Interests: []string{"docs"}, ExpectedOutcome: "Document it", Scope: []string{"docs"}, Risk: "low", Assistance: "human"}
	if _, e := s.Publish("r", "owner", "Docs", "proposal:p", "open", true, in); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Publish("r", "owner", "Docs", "proposal:p", "open", true, in); !errors.Is(e, ErrConflict) {
		t.Fatalf("duplicate source: %v", e)
	}
}

func TestOpportunityRetainsBootstrapContractAndReportsFriction(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Source: Source{Kind: "issue", ResourceID: "i"}, RequiredSkills: []string{"Go"}, Interests: []string{"API"}, ExpectedOutcome: "Repair setup", AcceptanceCriteria: []string{"setup check passes"}, SampleData: []string{"testdata/public.json"}, Scope: []string{"apps/api"}, Risk: "low", Assistance: "agent"}
	o, err := s.Publish("r", "owner", "Repair setup", "0123456789012345678901234567890123456789", "triaged", true, in)
	if err != nil || len(o.AcceptanceCriteria) != 1 || len(o.SampleData) != 1 {
		t.Fatalf("bootstrap contract: %#v %v", o, err)
	}
	report, err := s.Report("r", o.ID, "newcomer", "workspace-1", "obsolete_instructions", "setup names a removed tool")
	if err != nil || report.ActorID != "newcomer" {
		t.Fatalf("friction report: %#v %v", report, err)
	}
	data, _ := s.List("r")
	if len(data.Reports) != 1 || data.Reports[0].OpportunityID != o.ID {
		t.Fatalf("report was not retained: %#v", data.Reports)
	}
	if _, err = s.Report("r", o.ID, "newcomer", "workspace-1", "secret_dump", "no"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsafe report kind accepted: %v", err)
	}
	if _, err = s.Report("r", o.ID, "newcomer", "workspace-1", "missing_access", "Authorization: Bearer secret"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("credential-like report accepted: %v", err)
	}
}

func TestCompletedContributionRetainsTrustOutcome(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Source: Source{Kind: "proposal", ResourceID: "p"}, RequiredSkills: []string{"Go"}, Interests: []string{"onboarding"}, ExpectedOutcome: "Improve setup", Scope: []string{"README.md"}, Risk: "low", Assistance: "human_or_agent"}
	o, err := s.Publish("repo", "owner", "Improve setup", "abc123", "open", true, in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Complete("repo", o.ID, "newcomer", "pull-1", "release-1", "Authored the setup repair", "Used feedback well and explained the tradeoff.", 1.5, "ready_with_support", "opportunity-2", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if out.ContributorID != "newcomer" || out.SupportHours != 1.5 || out.RecordedByID != "owner" {
		t.Fatalf("outcome = %#v", out)
	}
	data, _ := s.List("repo")
	if len(data.Outcomes) != 1 || data.Outcomes[0].ReleaseID != "release-1" {
		t.Fatalf("persisted outcomes = %#v", data.Outcomes)
	}
	if _, err = s.Complete("repo", o.ID, "newcomer", "pull-1", "release-1", "credit", "feedback", 1, "ready", "", "owner"); !errors.Is(err, ErrConflict) {
		t.Fatalf("mutable outcome accepted: %v", err)
	}
}
