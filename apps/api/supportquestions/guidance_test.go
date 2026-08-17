package supportquestions

import "testing"

func answerInput() AnswerInput {
	return AnswerInput{AuthorKind: "human", Summary: "Use the bounded parser", Instructions: []string{"Call Parse with a context"}, ApplicableVersions: []string{"v2.1.0"}, Claims: []Claim{{Text: "Parse rejects an expired context", Mode: "verified", Citations: []Citation{{Kind: "source", Revision: "abc", Path: "parse.go", LineStart: 12, LineEnd: 20, Visibility: "public"}}}}}
}

func TestGuidanceRevisionsAndFeedbackRemainAttributable(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	q, err := s.Create("repo", "asker", Input{Title: "Parser timeout", Question: "How do I bound parsing?", Subject: Subject{Kind: "repository"}, Goal: "Avoid hangs", AttemptedSteps: []string{"called Parse"}, SoftwareVersion: "v2.1.0", Environment: "linux", Urgency: "normal", Audience: "public", Contact: Contact{Preference: "thread"}})
	if err != nil {
		t.Fatal(err)
	}
	q, err = s.ReviseAnswer("repo", q.ID, "maintainer", answerInput())
	if err != nil {
		t.Fatal(err)
	}
	a, r := q.Answers[0], q.Answers[0].Revisions[0]
	in := answerInput()
	in.AnswerID = a.ID
	in.SupersedesID = r.ID
	in.Summary = "Use the cancellation-aware parser"
	in.AuthorKind = "agent"
	in.Uncertainty = "Not tested on Windows"
	q, err = s.ReviseAnswer("repo", q.ID, "agent:local", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Answers[0].Revisions) != 2 || q.Answers[0].Revisions[0].Summary != "Use the bounded parser" {
		t.Fatalf("immutable revisions lost: %#v", q.Answers[0])
	}
	latest := q.Answers[0].Revisions[1]
	q, err = s.Feedback("repo", q.ID, a.ID, latest.ID, latest.Claims[0].ID, "community", "challenge", "Windows behavior is unknown")
	if err != nil {
		t.Fatal(err)
	}
	if q.Answers[0].Feedback[0].Kind != "challenge" || q.Answers[0].Feedback[0].ActorID != "community" {
		t.Fatalf("feedback=%#v", q.Answers[0].Feedback)
	}
}

func TestAgentGuidanceRequiresUncertaintyAndCurrentSupersession(t *testing.T) {
	s, _ := New(t.TempDir())
	q, _ := s.Create("repo", "asker", Input{Title: "Question", Question: "What now?", Subject: Subject{Kind: "repository"}, Goal: "Ship", AttemptedSteps: []string{"read docs"}, SoftwareVersion: "v1", Environment: "linux", Urgency: "low", Audience: "repository", Contact: Contact{Preference: "none"}})
	in := answerInput()
	in.AuthorKind = "agent"
	if _, err := s.ReviseAnswer("repo", q.ID, "agent", in); err != ErrInvalid {
		t.Fatalf("agent uncertainty accepted: %v", err)
	}
	in.AuthorKind = "human"
	q, _ = s.ReviseAnswer("repo", q.ID, "owner", in)
	in.AnswerID = q.Answers[0].ID
	in.SupersedesID = "stale"
	if _, err := s.ReviseAnswer("repo", q.ID, "owner", in); err != ErrInvalid {
		t.Fatalf("stale supersession accepted: %v", err)
	}
}
