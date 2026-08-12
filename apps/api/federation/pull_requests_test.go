package federation

import (
	"testing"
	"time"
)

func TestPullRequestEventsAreIdempotentAndPreserveRemoteIdentity(t *testing.T) {
	s, err := New(t.TempDir(), config("https://home.example"))
	if err != nil {
		t.Fatal(err)
	}
	e := PullRequestEvent{SchemaVersion: 1, IdempotencyKey: "review-1", PullReference: "pull-request:source@https://peer.example#repository=fork", TargetReference: "pull-request:target@https://home.example#repository=repo", SourceInstance: "https://peer.example", ActorSubject: "user:reviewer@https://peer.example", Kind: "review", Revision: "abc", State: "request_changes", Audience: "participants", OccurredAt: time.Now().UTC(), Verification: "verified_peer_signature"}
	first, err := s.PutPullRequestEvent(e)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.PutPullRequestEvent(e)
	if err != nil || again.ID != first.ID || again.ActorSubject != e.ActorSubject {
		t.Fatalf("replay = %#v, %v", again, err)
	}
	e.Body = "different signed claim"
	if _, err = s.PutPullRequestEvent(e); err != ErrConflict {
		t.Fatalf("conflict err = %v", err)
	}
	items, err := s.PullRequestEvents(e.PullReference)
	if err != nil || len(items) != 1 || items[0].Verification != "verified_peer_signature" {
		t.Fatalf("items = %#v, %v", items, err)
	}
}

func TestPullRequestEventSignatureExcludesOnlyLocalObservation(t *testing.T) {
	e := PullRequestEvent{SchemaVersion: 1, IdempotencyKey: "check", PullReference: "source", TargetReference: "target", SourceInstance: "https://peer.example", ActorSubject: "agent:ci@https://peer.example", Kind: "check", Revision: "abc", Audience: "public", OccurredAt: time.Now().UTC()}
	a := string(PullRequestEventBytes(e))
	e.Verification = "verified"
	e.Current = true
	e.ImportedAt = time.Now()
	b := string(PullRequestEventBytes(e))
	if a != b {
		t.Fatal("local observation fields changed signed bytes")
	}
	e.State = "passed"
	if a == string(PullRequestEventBytes(e)) {
		t.Fatal("authoritative state missing from signed bytes")
	}
}
