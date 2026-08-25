package reviewplans

import (
	"errors"
	"testing"
)

func TestPublishRetainsExactVersionsAndVisibleGaps(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Intent: "Change authentication and UI", Risk: "high", ChangeReason: "initial analysis", Commitments: []Context{{Kind: "security", Reference: "session-policy", Revision: "v2", Accessible: false}}, Areas: []Area{
		{ID: "security", Name: "Authentication", Expertise: []string{"security"}, Paths: []string{"auth.go"}, Questions: []string{"Are sessions revoked?"}, Evidence: []Evidence{{Kind: "scenario", Description: "Revocation passes", Required: true}}, CompletionRules: []string{"question answered with current evidence"}},
		{ID: "ui", Name: "Interface", Paths: []string{"auth.go"}, OwnerIDs: []string{"designer"}, Questions: []string{"Can keyboard users sign in?"}, Evidence: []Evidence{{Kind: "interface-check", Description: "Keyboard journey", Required: true}}, DependsOn: []string{"security"}, CompletionRules: []string{"current check passes"}},
	}}
	p, err := s.Publish("repo", "pull", "candidate", "base", "author", []string{"auth.go", "login.tsx"}, 0, in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"missing_ownership": false, "inaccessible_context": false, "overlapping_scope": false, "unplanned_scope": false}
	for _, b := range p.Blockers {
		if _, ok := want[b.Kind]; ok {
			want[b.Kind] = true
		}
	}
	for kind, found := range want {
		if !found {
			t.Errorf("missing %s blocker: %#v", kind, p.Blockers)
		}
	}
	if _, err = s.Publish("repo", "pull", "next", "base", "author", []string{"auth.go"}, 0, in); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	stale := Derive(p, "next", "base")
	if !stale.Stale || stale.Blockers[0].AttributedTo != "author" {
		t.Fatalf("staleness not attributable: %#v", stale)
	}
}

func TestPublishRejectsIncompleteReviewArea(t *testing.T) {
	s, _ := New(t.TempDir())
	_, err := s.Publish("repo", "pull", "candidate", "base", "author", []string{"a.go"}, 0, Input{Intent: "x", Risk: "medium", ChangeReason: "x", Areas: []Area{{ID: "code", Name: "Code", Paths: []string{"a.go"}}}})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid, got %v", err)
	}
}
