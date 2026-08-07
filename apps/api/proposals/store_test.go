package proposals

import (
	"testing"
)

func TestProposalAndConversationSurviveReopen(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := store.Create("repository", "author", "Explore agents", "Define the problem first.")
	if err != nil {
		t.Fatal(err)
	}
	comment, err := store.AddComment("repository", proposal.ID, "maintainer", "What outcome should we measure?")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := store.Close("repository", proposal.ID, "author")
	if err != nil {
		t.Fatal(err)
	}
	if closed.State != Closed || closed.ClosedByID != "author" || closed.ClosedAt == nil {
		t.Fatalf("closed proposal = %#v", closed)
	}

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get("repository", proposal.ID)
	if err != nil || got.AuthorID != "author" || got.Title != "Explore agents" {
		t.Fatalf("reopened proposal = %#v, %v", got, err)
	}
	comments, err := reopened.ListComments("repository", proposal.ID)
	if err != nil || len(comments) != 1 || comments[0].ID != comment.ID || comments[0].AuthorID != "maintainer" {
		t.Fatalf("reopened comments = %#v, %v", comments, err)
	}
}

func TestProposalValidation(t *testing.T) {
	store, _ := New(t.TempDir())
	if _, err := store.Create("repository", "author", "  ", ""); err != ErrInvalid {
		t.Fatalf("empty title error = %v", err)
	}
	proposal, _ := store.Create("repository", "author", "Valid", "")
	if _, err := store.AddComment("repository", proposal.ID, "author", "  "); err != ErrInvalidComment {
		t.Fatalf("empty comment error = %v", err)
	}
}
