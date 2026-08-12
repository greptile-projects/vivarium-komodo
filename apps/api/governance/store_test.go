package governance

import (
	"errors"
	"testing"
	"time"
)

func validInput() Input {
	return Input{Title: "Project charter", Purpose: "Make decision rights public", Roles: []Role{{Name: "maintainer", Purpose: "Steward the project", Eligibility: []string{"accepted contributor"}, Responsibilities: []string{"review policy"}, MinimumMembers: 1, TermDays: 365}}, DecisionClasses: []DecisionClass{{Name: "policy", Description: "Change project policy", EligibleRoles: []string{"maintainer"}, Participation: "open_deliberation", Quorum: 1, Threshold: 67, ProtectedResources: []string{"branches:main"}}}, ParticipationRules: []string{"publish rationale"}, ProtectedResources: []string{"branches:main"}, Procedures: Procedures{Removal: "Reasoned recall", Succession: "Election before expiry", Vacancy: "Owner appoints an interim"}, AmendmentPolicy: AmendmentPolicy{EligibleRoles: []string{"maintainer"}, NoticeDays: 7, Quorum: 1, Threshold: 67}, ChangeReason: "Initial charter"}
}

func TestCharterVersionsActivationAndHistoricalExceptions(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	blocked := Preview{GeneratedAt: now, Blockers: []string{"quorum impossible"}}
	v, e := s.Publish("repository", "repo", "owner", 0, validInput(), blocked)
	if e != nil || v.Current.Version != 1 || v.Current.State != "draft" {
		t.Fatalf("publish %#v %v", v, e)
	}
	if _, e = s.Activate("repository", "repo", "owner", 1, blocked); !errors.Is(e, ErrConflict) {
		t.Fatalf("blocked activation = %v", e)
	}
	v, e = s.Approve("repository", "repo", "owner", "reviewed", 1)
	if e != nil || len(v.Approvals) != 1 {
		t.Fatal(e)
	}
	clear := Preview{GeneratedAt: now}
	v, e = s.Activate("repository", "repo", "owner", 1, clear)
	if e != nil || v.ActiveVersion != 1 || v.Current.State != "active" {
		t.Fatalf("activate %#v %v", v, e)
	}
	x := Exception{Scope: "release freeze", Reason: "critical repair", ExpiresAt: now.Add(time.Hour)}
	v, e = s.Except("repository", "repo", "owner", 1, x)
	if e != nil || len(v.Exceptions) != 1 || v.Exceptions[0].Version != 1 {
		t.Fatalf("exception %#v %v", v, e)
	}
	in := validInput()
	in.ChangeReason = "Clarify succession"
	in.Procedures.Succession = "Election thirty days before expiry"
	v, e = s.Publish("repository", "repo", "owner", 1, in, clear)
	if e != nil || v.Current.Version != 2 || len(v.History) != 1 || v.History[0].State != "active" || v.Exceptions[0].Version != 1 {
		t.Fatalf("history %#v %v", v, e)
	}
}

func TestStandingIsEvidenceBoundedTermedAndCarriesNoAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	v, _ := s.Publish("repository", "repo", "owner", 0, validInput(), Preview{})
	v, _ = s.Approve("repository", "repo", "owner", "", 1)
	v, _ = s.Activate("repository", "repo", "owner", 1, Preview{})
	v, e := s.Invite("repository", "repo", "owner", 1, StandingInput{PrincipalID: "contributor", Role: "maintainer", Evidence: []Evidence{{Kind: "review", Reference: "pull:42", Summary: "reviewed three releases"}}, Nominations: []string{"steward"}, Appeals: []string{"owner review"}, ConflictDisclosure: "employed by a vendor"})
	if e != nil || len(v.Standings) != 1 || v.Standings[0].State != "invited" || len(v.Standings[0].OperationalAuthority) != 0 {
		t.Fatalf("invite %#v %v", v, e)
	}
	v, e = s.Transition("repository", "repo", v.Standings[0].ID, "contributor", "accept", "")
	if e != nil || v.Standings[0].State != "active" || v.Standings[0].TermEndsAt == nil {
		t.Fatalf("accept %#v %v", v, e)
	}
	v, e = s.Transition("repository", "repo", v.Standings[0].ID, "contributor", "recuse", "")
	if e != nil || v.Standings[0].State != "recused" {
		t.Fatalf("recuse %#v %v", v, e)
	}
	v, e = s.Transition("repository", "repo", v.Standings[0].ID, "owner", "suspend", "undisclosed conflict")
	if e != nil || v.Standings[0].State != "suspended" || len(v.Standings[0].Events) != 4 {
		t.Fatalf("suspend %#v %v", v, e)
	}
}

func TestStewardshipRecoverySeparatesGovernanceFromResourceAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	v, _ := s.Publish("repository", "repo", "owner", 0, validInput(), Preview{})
	v, _ = s.Approve("repository", "repo", "owner", "", 1)
	v, _ = s.Activate("repository", "repo", "owner", 1, Preview{})
	v, _ = s.Invite("repository", "repo", "owner", 1, StandingInput{PrincipalID: "old", Role: "maintainer", Evidence: []Evidence{{Kind: "ownership", Reference: "repository:repo", Summary: "founding steward"}}})
	old := v.Standings[0].ID
	v, _ = s.Transition("repository", "repo", old, "old", "accept", "")
	v, _ = s.Invite("repository", "repo", "owner", 1, StandingInput{PrincipalID: "next", Role: "maintainer", Evidence: []Evidence{{Kind: "review", Reference: "pull:7", Summary: "release reviewer"}}})
	next := v.Standings[1].ID
	v, _ = s.Transition("repository", "repo", next, "next", "accept", "")
	v, e := s.OpenStewardship("repository", "repo", "owner", StewardshipInput{Kind: "succession", Role: "maintainer", FormerStandingID: old, NomineeStandingID: next, DecisionReceiptID: "receipt-1", Reason: "term turnover", ResourceHandoffs: []ResourceHandoff{{Resource: "repository:repo", FromID: "old", ToID: "next"}}})
	if e != nil {
		t.Fatal(e)
	}
	c := v.Stewardship[0]
	v, e = s.TransitionStewardship("repository", "repo", c.ID, "owner", "complete", "election certified", "")
	if e != nil || v.Standings[0].State != "recalled" || len(v.Standings[0].Evidence) == 0 || v.Stewardship[0].Handoffs[0].State != "pending_owner_approval" {
		t.Fatalf("succession %#v %v", v, e)
	}
	v, e = s.TransitionStewardship("repository", "repo", c.ID, "owner", "approve_handoff", "resource owner approved separately", "repository:repo")
	if e != nil || v.Stewardship[0].Handoffs[0].State != "approved_external_action_required" {
		t.Fatalf("handoff %#v %v", v, e)
	}
	expires := now.Add(2 * time.Hour)
	review := now.Add(time.Hour)
	v, e = s.OpenStewardship("repository", "repo", "owner", StewardshipInput{Kind: "emergency", Role: "maintainer", Reason: "quorum unavailable", EmergencyScope: []string{"triage:security"}, ExpiresAt: &expires, ReviewDueAt: &review})
	if e != nil || v.Stewardship[1].State != "active" {
		t.Fatalf("emergency %#v %v", v, e)
	}
	h, e := s.Health("repository", "repo")
	if e != nil || len(h.ActiveEmergencyPowers) != 1 || len(h.UnresolvedHandoffs) != 1 {
		t.Fatalf("health %#v %v", h, e)
	}
}
