package agentevaluations

import (
	"errors"
	"testing"
	"time"
)

func pilotFixture(t *testing.T) (*Store, Candidate, time.Time) {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	c, err := s.CreateCandidate("repo", "owner", CandidateInput{PullRequestID: "pr", Revision: "rev-1", AgentProjectID: "project", AgentProjectVersion: 1, Suites: []SuiteSelection{{SuiteID: "suite", SuiteVersion: 1, ScenarioIDs: []string{"case"}}}, Inputs: []BoundInput{{Key: "prompt:main", Revision: "p1"}}, ChangeReason: "pilot candidate"})
	if err != nil {
		t.Fatal(err)
	}
	return s, c, now
}
func createPilotFixture(t *testing.T, s *Store, c Candidate, now time.Time) Pilot {
	t.Helper()
	x, e := s.CreatePilot("repo", "owner", PilotInput{CandidateID: c.ID, Repositories: []string{"repo", "repo-2"}, Roles: []string{"reviewer"}, Participants: []string{"user"}, Tasks: []string{"triage issue"}, Actions: []string{"read", "draft"}, MaximumCost: 5, Currency: "USD", ExpiresAt: now.Add(time.Hour), ExpectedOutcomes: map[string]string{"triage issue": "useful draft"}, Purpose: "experience collaboration"})
	if e != nil {
		t.Fatal(e)
	}
	return x
}

func TestPilotRetainsBoundedCollaborationAndFeedback(t *testing.T) {
	s, c, now := pilotFixture(t)
	x := createPilotFixture(t, s, c, now)
	if x.Authority.Merge || x.Authority.Deploy || x.Authority.Disclose || x.Authority.AuthoritativeMutation || !x.Authority.Read || !x.Authority.Draft {
		t.Fatalf("unexpected authority: %#v", x.Authority)
	}
	if _, e := s.SetPilotConsent("repo", x.ID, "user", "accepted", ""); e != nil {
		t.Fatal(e)
	}
	x, e := s.StartPilotSession("repo", x.ID, "user", PilotSessionInput{RepositoryID: "repo-2", Role: "reviewer", Task: "triage issue"})
	if e != nil {
		t.Fatal(e)
	}
	session := x.Sessions[0]
	x, e = s.RecordPilotEvent("repo", x.ID, session.ID, "user", PilotEventInput{Kind: "draft", Summary: "prepared suggestion", Draft: "non-authoritative patch", Cost: 2, Currency: "USD"})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.RecordPilotEvent("repo", x.ID, session.ID, "owner", PilotEventInput{Kind: "guidance", Summary: "cover the failure path"})
	if e != nil {
		t.Fatal(e)
	}
	x, e = s.RecordPilotFeedback("repo", x.ID, "user", PilotFeedbackInput{SessionID: session.ID, CandidateRevision: "rev-1", Kind: "correction", Summary: "missed retry behavior", Correction: "include bounded retry", ExpectedOutcome: "useful draft"})
	if e != nil {
		t.Fatal(e)
	}
	if x.Spent != 2 || len(x.Sessions[0].Drafts) != 1 || len(x.Sessions[0].Events) != 2 || len(x.Feedback) != 1 {
		t.Fatalf("evidence not retained: %#v", x)
	}
	if _, e = s.RecordPilotFeedback("repo", x.ID, "user", PilotFeedbackInput{SessionID: session.ID, CandidateRevision: "rev-2", Kind: "feedback", Summary: "stale"}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("expected revision rejection, got %v", e)
	}
}

func TestPilotPauseConditionsPreserveEvidence(t *testing.T) {
	tests := []struct {
		name   string
		act    func(*Store, Pilot) (Pilot, error)
		reason string
	}{{"consent", func(s *Store, x Pilot) (Pilot, error) {
		return s.SetPilotConsent("repo", x.ID, "user", "revoked", "withdrawn")
	}, "consent_revoked:user"}, {"candidate", func(s *Store, x Pilot) (Pilot, error) { return s.ReconcilePilotCandidate("repo", x.ID, "new") }, "candidate_changed"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, c, now := pilotFixture(t)
			x := createPilotFixture(t, s, c, now)
			got, e := tt.act(s, x)
			if e != nil {
				t.Fatal(e)
			}
			if got.State != "paused" || !contains(got.PauseReasons, tt.reason) {
				t.Fatalf("expected %s: %#v", tt.reason, got)
			}
		})
	}
	s, c, now := pilotFixture(t)
	x := createPilotFixture(t, s, c, now)
	_, _ = s.SetPilotConsent("repo", x.ID, "user", "accepted", "")
	x, _ = s.StartPilotSession("repo", x.ID, "user", PilotSessionInput{RepositoryID: "repo", Role: "reviewer", Task: "triage issue"})
	x, e := s.RecordPilotEvent("repo", x.ID, x.Sessions[0].ID, "user", PilotEventInput{Kind: "unsafe_behavior", Summary: "attempted prohibited mutation", Cost: 1, Currency: "USD"})
	if e != nil {
		t.Fatal(e)
	}
	if x.State != "paused" || len(x.Sessions[0].Events) != 1 {
		t.Fatalf("unsafe evidence lost: %#v", x)
	}
}

func TestPilotRejectsAuthoritativeActionsAndExhaustsBudget(t *testing.T) {
	s, c, now := pilotFixture(t)
	in := PilotInput{CandidateID: c.ID, Repositories: []string{"repo"}, Roles: []string{"reviewer"}, Participants: []string{"user"}, Tasks: []string{"triage"}, Actions: []string{"merge"}, MaximumCost: 1, Currency: "USD", ExpiresAt: now.Add(time.Hour), ExpectedOutcomes: map[string]string{"triage": "draft"}, Purpose: "bad"}
	if _, e := s.CreatePilot("repo", "owner", in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("merge should be rejected: %v", e)
	}
	x := createPilotFixture(t, s, c, now)
	_, _ = s.SetPilotConsent("repo", x.ID, "user", "accepted", "")
	x, _ = s.StartPilotSession("repo", x.ID, "user", PilotSessionInput{RepositoryID: "repo", Role: "reviewer", Task: "triage issue"})
	x, e := s.RecordPilotEvent("repo", x.ID, x.Sessions[0].ID, "user", PilotEventInput{Kind: "escalation", Summary: "needs judgment", Cost: 5, Currency: "USD"})
	if e != nil {
		t.Fatal(e)
	}
	if x.State != "paused" || !contains(x.PauseReasons, "budget_exhausted") {
		t.Fatalf("budget did not pause: %#v", x)
	}
}
