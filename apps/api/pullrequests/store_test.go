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
