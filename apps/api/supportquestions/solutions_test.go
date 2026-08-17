package supportquestions

import "testing"

func TestVerifiedAnswerBecomesDiscoverableSolutionWithoutRewritingEvidence(t *testing.T) {
	s, _ := New(t.TempDir())
	q, _ := s.Create("repo", "asker", Input{Title: "Bound parser", Question: "How do I prevent hangs?", Subject: Subject{Kind: "repository"}, SoftwareVersion: "v2.1.0", Environment: "linux", Goal: "finish safely", AttemptedSteps: []string{"called Parse"}, Urgency: "normal", Audience: "public", Contact: Contact{Preference: "thread"}})
	q, _ = s.ReviseAnswer("repo", q.ID, "maintainer", answerInput())
	a, r := q.Answers[0], q.Answers[0].Revisions[0]
	verification := VerificationAttempt{ID: "verification-1", RepositoryID: "repo", QuestionID: q.ID, AnswerID: a.ID, AnswerRevisionID: r.ID, SoftwareVersion: "v2.1.0", CreatedByID: "tester", State: "passed"}
	in := ResolutionInput{AnswerID: a.ID, AnswerRevisionID: r.ID, VerificationID: verification.ID, Title: "Prevent parser hangs", Summary: "Use the cancellation-aware parser.", ApplicableVersions: []string{"v2.1.0"}, Limitations: []string{"Windows has not been tested"}, Audience: "public", Links: []SolutionLink{{Kind: "documentation", ResourceID: "docs"}, {Kind: "release", ResourceID: "release-2.1"}}}
	q, err := s.Resolve("repo", q.ID, "asker", in, verification)
	if err != nil {
		t.Fatal(err)
	}
	if q.Status != "resolved" || len(q.Solutions) != 1 || q.Answers[0].Revisions[0].Summary != "Use the bounded parser" {
		t.Fatalf("resolution lost history: %#v", q)
	}
	if len(q.Solutions[0].Credits) != 3 || len(q.Solutions[0].Notifications) != 3 {
		t.Fatalf("credit/notifications=%#v %#v", q.Solutions[0].Credits, q.Solutions[0].Notifications)
	}
	found, _ := s.Solutions("repo", "parser hangs", "", true)
	if len(found) != 1 || found[0].VerificationID != "verification-1" {
		t.Fatalf("search=%#v", found)
	}
	q, err = s.SolutionEvent("repo", q.ID, q.Solutions[0].ID, "maintainer", "request_revalidation", "Confirm the new runtime.", "", "", "v2.2.0")
	if err != nil || q.Solutions[0].Status != "revalidation_requested" || q.Solutions[0].Events[0].Type != "published" || len(q.Solutions[0].Events) != 2 {
		t.Fatalf("revalidation=%#v %v", q.Solutions[0], err)
	}
}

func TestSolutionPublicationRequiresPassedExactVerificationAndSafeAudience(t *testing.T) {
	s, _ := New(t.TempDir())
	q, _ := s.Create("repo", "asker", Input{Title: "Private question", Question: "How?", Subject: Subject{Kind: "repository"}, SoftwareVersion: "v2.1.0", Environment: "linux", Goal: "work", AttemptedSteps: []string{"try"}, Urgency: "low", Audience: "repository", Contact: Contact{Preference: "none"}})
	q, _ = s.ReviseAnswer("repo", q.ID, "owner", answerInput())
	a, r := q.Answers[0], q.Answers[0].Revisions[0]
	v := VerificationAttempt{ID: "v", AnswerID: a.ID, AnswerRevisionID: r.ID, State: "failed"}
	in := ResolutionInput{AnswerID: a.ID, AnswerRevisionID: r.ID, VerificationID: "v", Title: "Advice", Summary: "Summary", ApplicableVersions: []string{"v2.1.0"}, Audience: "public"}
	if _, err := s.Resolve("repo", q.ID, "asker", in, v); err != ErrInvalid {
		t.Fatalf("unsafe publication accepted: %v", err)
	}
}
