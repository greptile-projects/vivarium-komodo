package decisions

import (
	"testing"
	"time"
)

func TestDecisionRetainsScopeHistoryAndDiscussion(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	in := ScopeInput{Question: "How should writes be coordinated?", Constraints: []string{"Preserve attribution"}, SuccessMeasures: []string{"No lost updates"}, Deadline: &deadline, AffectedResources: []Resource{{Kind: "code", Path: "store.go", Label: "storage boundary"}}, ParticipantIDs: []string{"author", "owner"}, OwnerID: "owner"}
	v, err := s.Create("repo", "author", "Coordinate writes", Context{Kind: "proposal", ID: "proposal-1"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "pending" || v.Scope.Version != 1 || len(v.History) != 1 {
		t.Fatalf("unexpected creation: %#v", v)
	}
	in.Question = "How should concurrent writes be coordinated?"
	in.ChangeSummary = "Included concurrent callers"
	v, err = s.Revise("repo", v.ID, "owner", "Coordinate concurrent writes", in)
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Comment("repo", v.ID, "author", "The API must expose conflicts.")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.History) != 2 || v.History[0].Question == v.Scope.Question || v.History[1].ChangedByID != "owner" || len(v.Comments) != 1 || v.Comments[0].AuthorID != "author" {
		t.Fatalf("history was not retained: %#v", v)
	}
	items, err := s.List("repo", "proposal", "proposal-1")
	if err != nil || len(items) != 1 {
		t.Fatalf("linked list: %v %#v", err, items)
	}
	reopened, err := New(s.root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get("repo", v.ID)
	if err != nil || len(got.History) != 2 || len(got.Comments) != 1 {
		t.Fatalf("persistence: %v %#v", err, got)
	}
}

func TestDecisionRejectsUnaccountableScope(t *testing.T) {
	s, _ := New(t.TempDir())
	_, err := s.Create("repo", "author", "Choice", Context{Kind: "repository"}, ScopeInput{Question: "Choose?", Constraints: []string{"safe"}, SuccessMeasures: []string{"works"}, ParticipantIDs: []string{"author"}, OwnerID: "missing"})
	if err != ErrInvalid {
		t.Fatalf("got %v", err)
	}
}
