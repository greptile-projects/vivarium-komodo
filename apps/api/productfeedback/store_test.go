package productfeedback

import "testing"

func TestFeedbackRetainsConsentDiscussionLinksAndWithdrawal(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create("repo", "reporter", Input{Context: Context{Kind: "project"}, Need: "I need a quicker review loop", DesiredOutcome: "Know what blocks a review", Frequency: "weekly", Impact: "Work waits for days", Audience: "public", IdentityVisibility: "maintainers", ContactPreference: "discussion", Consent: Consent{Research: true}, Evidence: []Evidence{{Kind: "quote", Name: "redacted.txt", MediaType: "text/plain", Content: "[redacted] waited three days", Visibility: "maintainers", Redacted: true}}})
	if err != nil || v.ReporterID != "reporter" || !v.Consent.Research {
		t.Fatalf("create: %#v %v", v, err)
	}
	v, err = s.Comment("repo", v.ID, "maintainer", "Which stage is slow?")
	if err != nil || len(v.Discussion) != 1 {
		t.Fatalf("comment: %#v %v", v, err)
	}
	v, err = s.Link("repo", v.ID, "maintainer", "issue", "issue-1")
	if err != nil || len(v.Links) != 1 {
		t.Fatalf("link: %#v %v", v, err)
	}
	v, err = s.Withdraw("repo", v.ID, "reporter")
	if err != nil || v.Consent.WithdrawnAt == nil || v.ContactPreference != "none" || len(v.History) != 4 {
		t.Fatalf("withdraw: %#v %v", v, err)
	}
}

func TestFeedbackRejectsUnredactedOrMismatchedPrivacy(t *testing.T) {
	s, _ := New(t.TempDir())
	_, err := s.Create("repo", "reporter", Input{Context: Context{Kind: "project"}, Need: "Need", DesiredOutcome: "Outcome", Frequency: "daily", Impact: "Impact", Audience: "organization", IdentityVisibility: "audience", ContactPreference: "none", Evidence: []Evidence{{Kind: "quote", Name: "raw", MediaType: "text/plain", Content: "secret", Visibility: "audience"}}})
	if err != ErrInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
}
