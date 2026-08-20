package independentassessments

import (
	"errors"
	"testing"
	"time"
)

func TestBoundedAssessmentLifecycle(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	scope := Scope{ProgramID: "soc", ProgramVersion: 2, ControlIDs: []string{"access", "release"}, Systems: []string{"api"}, Releases: []string{"v4"}, PeriodStart: now.Add(-30 * 24 * time.Hour), PeriodEnd: now, EvidencePackageIDs: []string{"pkg-access"}}
	a, e := s.Open("repo", "owner", OpenInput{Title: "Independent review", Purpose: "annual controls", Scope: scope, StartsAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour)})
	if e != nil {
		t.Fatal(e)
	}
	a, c, e := s.Invite("repo", a.ID, "owner", InvitationInput{AssessorID: "auditor@example.test", AssessorName: "Ada Assessor", Organization: "Independent LLP", Kind: "external", ConflictDisclosure: "Former vendor", ExpiresAt: now.Add(24 * time.Hour)})
	if e != nil || c.Token == "" || a.Invitations[0].ConflictStatus != "disclosed" {
		t.Fatalf("invite %#v %#v %v", a, c, e)
	}
	got, invite, e := s.Authenticate(c.Token)
	if e != nil || got.ID != a.ID || invite.AssessorID != "auditor@example.test" {
		t.Fatalf("auth %#v %#v %v", got, invite, e)
	}
	a, e = s.Add("repo", a.ID, invite.AssessorID, "assessor", EventInput{Kind: "attestation_verification", Subject: "Package digest", Body: "Hash and collector attestation verify", ControlID: "access", EvidencePackageIDs: []string{"pkg-access"}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Add("repo", a.ID, invite.AssessorID, "assessor", EventInput{Kind: "response", Subject: "mutate", Body: "not allowed"}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("assessor mutation = %v", e)
	}
	if _, e = s.Add("repo", a.ID, invite.AssessorID, "assessor", EventInput{Kind: "finding", Subject: "out of scope", Body: "hidden", ControlID: "secret-control"}); !errors.Is(e, ErrForbidden) {
		t.Fatalf("scope escape = %v", e)
	}
	a, e = s.Add("repo", a.ID, "owner", "owner", EventInput{Kind: "response", Subject: "Package digest", Body: "Acknowledged", ParentID: a.Events[0].ID})
	if e != nil || len(a.Events) != 2 {
		t.Fatalf("response %#v %v", a, e)
	}
	prior := a.Revision
	next := scope
	next.ControlIDs = []string{"access"}
	a, e = s.ChangeScope("repo", a.ID, "owner", prior, next, "release control moved to a later review")
	if e != nil || a.Invitations[0].Status != "scope_changed" {
		t.Fatalf("scope %#v %v", a, e)
	}
	if _, _, e = s.Authenticate(c.Token); !errors.Is(e, ErrExpired) {
		t.Fatalf("old scope credential = %v", e)
	}
}

func TestExpiryAndOwnerBoundaries(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	scope := Scope{ProgramID: "p", ProgramVersion: 1, ControlIDs: []string{"c"}, PeriodStart: now.Add(-time.Hour), PeriodEnd: now}
	a, _ := s.Open("r", "owner", OpenInput{Title: "Review", Scope: scope, StartsAt: now, ExpiresAt: now.Add(2 * time.Hour)})
	if _, _, e := s.Invite("r", a.ID, "writer", InvitationInput{AssessorID: "x", AssessorName: "X", Kind: "internal", ExpiresAt: now.Add(time.Hour)}); !errors.Is(e, ErrForbidden) {
		t.Fatalf("non-owner invite = %v", e)
	}
	_, c, _ := s.Invite("r", a.ID, "owner", InvitationInput{AssessorID: "x", AssessorName: "X", Kind: "internal", ExpiresAt: now.Add(time.Hour)})
	now = now.Add(90 * time.Minute)
	if _, _, e := s.Authenticate(c.Token); !errors.Is(e, ErrExpired) {
		t.Fatalf("expired = %v", e)
	}
}
