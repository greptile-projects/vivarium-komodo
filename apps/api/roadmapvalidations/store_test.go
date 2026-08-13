package roadmapvalidations

import (
	"errors"
	"testing"
	"time"
)

func TestValidationRetainsConsentBoundLearningAndPriorPlan(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 13, 21, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	in := Input{OutcomeID: "outcome-a", Kind: "prototype", Title: "Review prototype", Hypothesis: "The direction resolves the cited need", ChangeReason: "Test before commitment", Measures: []Measure{{Name: "task completion", Kind: "success", FeedbackIDs: []string{"feedback-a"}, Threshold: "4 of 5 complete"}, {Name: "keyboard access", Kind: "guardrail", FeedbackIDs: []string{"feedback-a"}, Threshold: "no blocking issue"}}, Activity: Activity{Kind: "preview", Revision: "commit-a", Scope: "submit flow only", StartsAt: now, EndsAt: now.Add(time.Hour)}}
	v, err := s.Create("repo", "roadmap", 3, "opportunity", 2, "owner", in)
	if err != nil || v.RoadmapVersion != 3 || v.OpportunityVersion != 2 || v.OperationalAuthority {
		t.Fatalf("create = %#v, %v", v, err)
	}
	v, token, err := s.Invite("repo", v.ID, "owner", "participant", "feedback-a", "screen reader")
	if err != nil || token == "" || v.Invitations[0].TokenDigest == "" {
		t.Fatalf("invite = %#v, %v", v, err)
	}
	context, invitation, err := s.Participant(token)
	if err != nil || context.ID != v.ID || invitation.ActivityRevision != "commit-a" {
		t.Fatalf("context = %#v %#v %v", context, invitation, err)
	}
	v, err = s.Find(token, Finding{Finding: "The primary task worked but focus order failed", AccessibilityNeeds: "preserve visible focus", Dissent: "Do not accept this direction yet", Acceptance: "dissent", EvidenceValidity: "invalid"})
	if err != nil || len(v.Findings) != 1 || v.Findings[0].ParticipantID != "participant" {
		t.Fatalf("finding = %#v, %v", v, err)
	}
	v, err = s.Assess("repo", v.ID, "owner", Assessment{FindingIDs: []string{v.Findings[0].ID}, EvidenceStatus: "invalid", Decision: "revise", Rationale: "Repair focus order and repeat the bounded preview"})
	if err != nil || v.Assessments[0].Decision != "revise" || v.RoadmapVersion != 3 || len(v.Versions) != 1 {
		t.Fatalf("assessment changed prior plan = %#v, %v", v, err)
	}
	in.ChangeReason = "broaden prototype"
	in.Activity.Revision = "commit-b"
	v, err = s.Revise("repo", v.ID, "owner", 1, in)
	if err != nil || v.CurrentVersion != 2 || v.Invitations[0].ActivityRevision != "commit-a" {
		t.Fatalf("revision = %#v, %v", v, err)
	}
	if _, err = s.Revise("repo", v.ID, "owner", 1, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict = %v", err)
	}
}

func TestValidationRequiresRepresentativeMeasures(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now()
	in := Input{OutcomeID: "o", Kind: "prototype", Title: "x", Hypothesis: "x", ChangeReason: "x", Measures: []Measure{{Name: "only success", Kind: "success", FeedbackIDs: []string{"f"}, Threshold: "yes"}}, Activity: Activity{Kind: "research", Revision: "r", Scope: "x", StartsAt: now, EndsAt: now.Add(time.Hour)}}
	if _, e := s.Create("repo", "roadmap", 1, "opp", 1, "owner", in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("error = %v", e)
	}
}
