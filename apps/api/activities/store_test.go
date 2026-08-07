package activities

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type resolver map[string]users.User

func (r resolver) FindByHandle(handle string) (users.User, error) { return r[handle], nil }

func TestActivityAndStableMentionsSurviveReopen(t *testing.T) {
	people := resolver{"reviewer": {ID: "reviewer-id", Handle: "reviewer"}}
	store, err := New(t.TempDir(), people)
	if err != nil {
		t.Fatal(err)
	}
	root := store.root
	store.now = func() time.Time { return time.Unix(100, 0) }
	created, err := store.Record(Input{RepositoryID: "repository", ActorID: "author", Type: "pull_request.commented", Resource: Resource{Type: "pull_request", ID: "pr"}, MentionText: "Please check this, @Reviewer. @reviewer"})
	if err != nil {
		t.Fatal(err)
	}
	reopened, _ := New(root, people)
	events, err := reopened.List("repository")
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %#v, err = %v", events, err)
	}
	var mention Event
	for _, event := range events {
		if event.Type == "mention.created" {
			mention = event
		}
	}
	if created.Type != "pull_request.commented" || mention.TargetUserID != "reviewer-id" || mention.Resource.ID != "pr" || mention.Metadata["source_event_id"] != created.ID {
		t.Fatalf("created = %#v mention = %#v", created, mention)
	}
}
