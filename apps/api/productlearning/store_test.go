package productlearning

import "testing"

func TestReciprocalLearningLifecycle(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Ensure("repo", "delivery", "roadmap", "outcome", "opp", 2)
	if err != nil {
		t.Fatal(err)
	}
	if v.OperationalAuthority {
		t.Fatal("learning must not grant authority")
	}
	v, err = s.Publish("repo", "delivery", "maintainer", UpdateInput{Kind: "measured_outcome", Summary: "The release shipped", Rationale: "Observed activation improved", Audience: "participants", FeedbackIDs: []string{"feedback"}, StakeholderIDs: []string{"stakeholder"}, Links: []Link{{Kind: "release", ResourceID: "rel", Label: "validate release", Public: true}}})
	if err != nil {
		t.Fatal(err)
	}
	u := v.Updates[0]
	v, err = s.Respond("repo", "delivery", u.ID, "feedback", "reporter", ResponseInput{Outcome: "mixed", Body: "Faster, but confusing", Evidence: []string{"follow-up"}, Dissent: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Responses) != 1 || !v.Responses[0].Dissent {
		t.Fatal("response and dissent not retained")
	}
	if _, err = s.Respond("repo", "delivery", u.ID, "feedback", "reporter", ResponseInput{Outcome: "improved", Body: "duplicate"}); err != ErrConflict {
		t.Fatalf("duplicate = %v", err)
	}
	if _, err = s.Leave("repo", "delivery", "reporter", "feedback", "no more updates"); err != nil {
		t.Fatal(err)
	}
	v, err = s.RecordLesson("repo", "delivery", "maintainer", LessonInput{ExpectedOutcomes: []string{"less friction"}, ObservedOutcomes: []string{"mixed"}, Lessons: []string{"simplify copy"}, Dissent: []string{"reporter found it confusing"}, ResultingWork: []Link{{Kind: "issue", ResourceID: "issue", Label: "follow-up", Public: true}}, RoadmapID: "roadmap", RoadmapVersion: 3, OpportunityDisposition: "fulfilled", ChangeReason: "measured after release", ExpectedRevision: 0})
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentRevision != 1 || v.Lessons[0].OpportunityDisposition != "fulfilled" {
		t.Fatal("lesson was not versioned")
	}
	if _, err = s.RecordLesson("repo", "delivery", "maintainer", v.Lessons[0].LessonInput); err != ErrConflict {
		t.Fatalf("stale lesson = %v", err)
	}
}

func TestRejectsUnboundedOrUnsupportedUpdates(t *testing.T) {
	s, _ := New(t.TempDir())
	_, _ = s.Ensure("repo", "delivery", "roadmap", "outcome", "opp", 1)
	if _, err := s.Publish("repo", "delivery", "actor", UpdateInput{Kind: "advertisement", Summary: "x", Rationale: "x", Audience: "public", StakeholderIDs: []string{"u"}}); err != ErrInvalid {
		t.Fatalf("invalid update = %v", err)
	}
	if _, err := s.RecordLesson("repo", "delivery", "actor", LessonInput{ExpectedOutcomes: []string{"x"}, ObservedOutcomes: []string{"x"}, Lessons: []string{"x"}, RoadmapID: "r", RoadmapVersion: 1, OpportunityDisposition: "deleted", ChangeReason: "x"}); err != ErrInvalid {
		t.Fatalf("invalid disposition = %v", err)
	}
}
