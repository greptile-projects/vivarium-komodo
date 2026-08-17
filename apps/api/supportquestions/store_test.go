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
