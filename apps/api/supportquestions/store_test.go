package supportquestions

import (
	"encoding/base64"
	"testing"
)

func TestQuestionRetainsContextAndHistory(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Create("repo", "user", Input{Title: "Client auth fails", Question: "How do I authenticate?", Subject: Subject{Kind: "api", ResourceID: "POST /v1/jobs"}, Goal: "create a job", Urgency: "high", Audience: "repository", Contact: Contact{Preference: "email", Value: "dev@example.test"}, Evidence: []Evidence{{Kind: "log", Name: "error.log", MediaType: "text/plain", Content: base64.StdEncoding.EncodeToString([]byte("redacted")), Visibility: "maintainers"}}})
	if e != nil {
		t.Fatal(e)
	}
	if v.Status != "needs_context" || len(v.MissingContext) != 3 {
		t.Fatalf("missing context not derived: %#v", v)
	}
	v, e = s.Comment("repo", v.ID, "maintainer", "Which SDK version?")
	if e != nil || len(v.History) != 2 {
		t.Fatalf("comment: %#v %v", v, e)
	}
	v, e = s.Status("repo", v.ID, "user", "closed")
	if e != nil || v.Status != "closed" || len(v.History) != 3 {
		t.Fatalf("status: %#v %v", v, e)
	}
}

func TestQuestionImprovementFreezesPermittedContextAndReportsProgress(t *testing.T) {
	s, _ := New(t.TempDir())
	q, err := s.Create("repo", "asker", Input{Title: "Current answer is incomplete", Question: "The command fails on v2", Subject: Subject{Kind: "repository"}, SoftwareVersion: "v2", Environment: "linux", Goal: "Complete setup", AttemptedSteps: []string{"run setup"}, Urgency: "normal", Audience: "repository", Contact: Contact{Preference: "thread"}})
	if err != nil {
		t.Fatal(err)
	}
	q, err = s.Comment("repo", q.ID, "asker", "Only this redacted reproduction may be carried forward")
	if err != nil {
		t.Fatal(err)
	}
	q, improvement, err := s.CreateImprovement("repo", q.ID, "maintainer", "compatibility_problem", "issue", "issue-1", []string{"setup passes on v2"}, []string{q.Discussion[0].ID})
	if err != nil || improvement.Context.SoftwareVersion != "v2" || len(improvement.Context.Discussion) != 1 {
		t.Fatalf("improvement %#v %v", improvement, err)
	}
	q, err = s.AddImprovementLink("repo", q.ID, improvement.ID, "maintainer", ImprovementLink{Kind: "pull_request", ResourceID: "pull-1", State: "in_progress", Revision: "abc"})
	if err != nil || len(q.Improvements[0].Links) != 1 || q.History[len(q.History)-1].Type != "improvement.progress" {
		t.Fatalf("progress %#v %v", q, err)
	}
}
