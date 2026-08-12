package docscollections

import (
	"errors"
	"strings"
	"testing"
)

func input(revision string) Input {
	return Input{Name: "Guide", RootPath: "docs", EntryPaths: []string{"README.md"}, Versions: []VersionMapping{{Label: "v1", SourceRevision: revision}}, OwnerIDs: []string{"owner"}, Audiences: []string{"developers"}, Policy: Policy{Navigation: "path", Renderer: "markdown", Publication: "maintainer_reviewed"}, ChangeReason: "initial review"}
}

func TestPublishedEditionsArchiveAndFeedbackRetainsSafeAccountability(t *testing.T) {
	s, _ := New(t.TempDir())
	base := Publication{RepositoryID: "repo", CollectionID: "guide", CollectionVersion: 1, PullRequestID: "pull-1", PreviewID: "preview-1", SourceRevision: strings.Repeat("a", 40), MergeRevision: strings.Repeat("b", 40), Pages: []ReviewPage{{Path: "docs/start.md", BlobID: "blob-1", Rendered: "Install v1"}}, Versions: []VersionMapping{{Label: "v1", SourceRevision: strings.Repeat("a", 40), ReleaseID: "release-1"}}, PublishedByID: "owner"}
	first, err := s.Publish(base)
	if err != nil {
		t.Fatal(err)
	}
	base.PullRequestID = "pull-2"
	base.PreviewID = "preview-2"
	base.SourceRevision = strings.Repeat("c", 40)
	base.MergeRevision = strings.Repeat("d", 40)
	base.Pages[0].Rendered = "Install v2"
	base.Versions[0].Label = "v2"
	second, err := s.Publish(base)
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.ListPublications("repo", "guide")
	if err != nil || len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID || items[0].Pages[0].Rendered != "Install v1" {
		t.Fatalf("editions %#v %v", items, err)
	}
	f, err := s.CreateFeedback(Feedback{RepositoryID: "repo", PublicationID: first.ID, CollectionID: "guide", PagePath: "docs/start.md", Kind: "failed_example", Body: "The command fails", ReporterID: "reader", Evidence: []FeedbackEvidence{{Kind: "log", Name: "output.txt", Content: "token=secret failure"}}})
	if err != nil || strings.Contains(f.Evidence[0].Content, "secret") {
		t.Fatalf("feedback %#v %v", f, err)
	}
	f, err = s.TriageFeedback("repo", f.ID, "owner", "documentation_task", "task-1")
	if err != nil || f.Triage.ResourceID != "task-1" {
		t.Fatalf("triage %#v %v", f, err)
	}
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
