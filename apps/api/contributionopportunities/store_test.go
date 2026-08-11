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
