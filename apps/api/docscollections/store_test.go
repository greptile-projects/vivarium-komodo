package docscollections

import (
	"errors"
	"testing"
)

func input(revision string) Input {
	return Input{Name: "Guide", RootPath: "docs", EntryPaths: []string{"README.md"}, Versions: []VersionMapping{{Label: "v1", SourceRevision: revision}}, OwnerIDs: []string{"owner"}, Audiences: []string{"developers"}, Policy: Policy{Navigation: "path", Renderer: "markdown", Publication: "maintainer_reviewed"}, ChangeReason: "initial review"}
}
func TestCollectionVersionsAreImmutableAndConcurrencyChecked(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	in := input("0123456789012345678901234567890123456789")
	c, e := s.Create("repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	in.ExpectedVersion = 1
	in.Description = "Updated"
	in.ChangeReason = "clarified"
	updated, e := s.Update("repo", c.ID, "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(updated.History) != 2 || updated.History[0].Description != "" || updated.History[1].Description != "Updated" {
		t.Fatalf("unexpected history: %#v", updated.History)
	}
	if _, e = s.Update("repo", c.ID, "owner", in); !errors.Is(e, ErrConflict) {
		t.Fatalf("expected conflict, got %v", e)
	}
}
func TestCollectionRejectsTraversalAndUnreviewedRevision(t *testing.T) {
	s, _ := New(t.TempDir())
	in := input("branch-name")
	in.RootPath = "../private"
	if _, e := s.Create("repo", "owner", in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("expected invalid, got %v", e)
	}
}
