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

func TestDocumentationReviewRetainsExactContentAndRequiredAreas(t *testing.T) {
	s, _ := New(t.TempDir())
	p, err := s.CreateReviewPreview(ReviewPreview{RepositoryID: "repo", PullRequestID: "pull", CollectionID: "guide", CollectionVersion: 2, Revision: "0123456789012345678901234567890123456789", Pages: []ReviewPage{{Path: "docs/start.md", BlobID: "blob-one", Rendered: "<p>Start</p>"}}, AffectedVersions: []string{"v2"}, Gaps: []ReviewGap{{Area: "migration", Detail: "No downgrade example."}}, CreatedByID: "author"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.InviteReview("repo", "pull", p.ID, "author", "reader", "audience")
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.AddReviewComment("repo", "pull", p.ID, "reader", ReviewComment{Path: "docs/start.md", BlobID: "blob-one", Start: 3, End: 8, Body: "This step is ambiguous."})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.PutAreaDecision("repo", "pull", p.ID, "reader", AreaDecision{Area: "audience", Decision: "request_changes", Body: "Clarify the first-run path."})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Invitations) != 1 || len(p.Comments) != 1 || len(p.Decisions) != 1 || p.Decisions[0].BlobIDs[0] != "blob-one" {
		t.Fatalf("review = %#v", p)
	}
	reopened, err := s.GetReviewPreview("repo", "pull", p.ID)
	if err != nil || reopened.Comments[0].Body != "This step is ambiguous." {
		t.Fatalf("reopened = %#v, %v", reopened, err)
	}
	if _, err = s.AddReviewComment("repo", "pull", p.ID, "reader", ReviewComment{Path: "docs/start.md", BlobID: "new-blob", Body: "wrong subject"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong blob = %v", err)
	}
}
