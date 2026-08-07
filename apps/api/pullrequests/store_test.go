package pullrequests

import "testing"

func TestPullRequestSurvivesReopen(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(CreateParams{RepositoryID: "repository", ProposalID: "proposal", AuthorID: "author", Title: "Ship the change", Body: "This makes setup reproducible.", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "source", TargetCommitID: "target"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get("repository", created.ID)
	if err != nil || got != created || got.Status != Open {
		t.Fatalf("reopened pull request = %#v, %v", got, err)
	}
	items, err := reopened.List("repository")
	if err != nil || len(items) != 1 || items[0] != created {
		t.Fatalf("listed pull requests = %#v, %v", items, err)
	}
}

func TestPullRequestValidation(t *testing.T) {
	store, _ := New(t.TempDir())
	valid := CreateParams{RepositoryID: "repository", AuthorID: "author", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "source", TargetCommitID: "target"}
	valid.TargetBranch = valid.SourceBranch
	if _, err := store.Create(valid); err != ErrInvalid {
		t.Fatalf("same branch error = %v", err)
	}
}

func TestPullRequestDiscussionSurvivesReopen(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root)
	pullRequest, _ := store.Create(CreateParams{RepositoryID: "repository", AuthorID: "author", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "source", TargetCommitID: "target"})
	created, err := store.AddComment("repository", pullRequest.ID, "collaborator", "  This explains the tradeoff.  ")
	if err != nil || created.Body != "This explains the tradeoff." || created.AuthorID != "collaborator" {
		t.Fatalf("created comment = %#v, %v", created, err)
	}
	reopened, _ := New(root)
	comments, err := reopened.ListComments("repository", pullRequest.ID)
	if err != nil || len(comments) != 1 || comments[0] != created {
		t.Fatalf("reopened comments = %#v, %v", comments, err)
	}
	if _, err := reopened.AddComment("repository", pullRequest.ID, "collaborator", " "); err != ErrInvalidComment {
		t.Fatalf("empty comment error = %v", err)
	}
}

func TestReviewCanBeReplacedWithdrawnAndReopened(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root)
	pullRequest, _ := store.Create(CreateParams{RepositoryID: "repository", AuthorID: "author", Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "source", TargetCommitID: "target"})
	approved, err := store.PutReview("repository", pullRequest.ID, "reviewer", Approve, "commit-one")
	if err != nil || approved.Decision != Approve || approved.CommitID != "commit-one" {
		t.Fatalf("approved review = %#v, %v", approved, err)
	}
	replaced, err := store.PutReview("repository", pullRequest.ID, "reviewer", RequestChanges, "commit-two")
	if err != nil || replaced.Decision != RequestChanges || replaced.CommitID != "commit-two" || replaced.SubmittedAt != approved.SubmittedAt {
		t.Fatalf("replaced review = %#v, %v", replaced, err)
	}
	reopened, _ := New(root)
	reviews, err := reopened.ListReviews("repository", pullRequest.ID)
	if err != nil || len(reviews) != 1 || reviews[0] != replaced {
		t.Fatalf("reopened reviews = %#v, %v", reviews, err)
	}
	if err := reopened.DeleteReview("repository", pullRequest.ID, "reviewer"); err != nil {
		t.Fatal(err)
	}
	reviews, _ = reopened.ListReviews("repository", pullRequest.ID)
	if len(reviews) != 0 {
		t.Fatalf("reviews after withdrawal = %#v", reviews)
	}
	if _, err := reopened.PutReview("repository", pullRequest.ID, "reviewer", "comment", "commit"); err != ErrInvalidReview {
		t.Fatalf("invalid decision error = %v", err)
	}
}
