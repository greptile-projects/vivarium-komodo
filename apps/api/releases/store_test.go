package releases

import (
	"errors"
	"testing"
)

func TestReleaseDefinitionIsDurableAndVersionIsUnique(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(CreateParams{RepositoryID: "repo", Version: "v1.0.0", Notes: "First release", CommitID: "abc", CreatedByID: "maintainer", PullRequests: []PullRequestLink{{ID: "pr", Title: "Feature", AuthorID: "author", MergeCommitID: "abc"}}, ProposalIDs: []string{"proposal", "proposal"}, TaskIDs: []string{"task"}, ContributorIDs: []string{"author", "author"}})
	if err != nil || created.Status != Candidate || len(created.ProposalIDs) != 1 || len(created.ContributorIDs) != 1 {
		t.Fatalf("created = %#v, %v", created, err)
	}
	reopened, _ := New(store.root)
	item, err := reopened.Get("repo", created.ID)
	if err != nil || item.CommitID != "abc" || len(item.PullRequests) != 1 || item.PullRequests[0].ID != "pr" {
		t.Fatalf("reopened = %#v, %v", item, err)
	}
	_, err = reopened.Create(CreateParams{RepositoryID: "repo", Version: "V1.0.0", CommitID: "def", CreatedByID: "maintainer"})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("duplicate version error = %v", err)
	}
}
