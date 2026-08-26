package runbookexecutions

import (
	"testing"
	"time"
)

func launch() LaunchInput {
	now := time.Now().UTC()
	return LaunchInput{IdempotencyKey: "alert-1:rb-1", RunbookID: "rb-1", RunbookVersion: 2, Origin: Origin{"alert", "alert-1", "7", "/alerts/alert-1#timeline", "participants"}, AffectedResources: []string{"service:api"}, SignalWindow: SignalWindow{now.Add(-time.Minute), now}, Context: []Context{{"release", "release-7", "commit-7", true, "participants", true}}, Preconditions: []Check{{"impact", true, "metric:errors", "confirmed"}}, Access: []Access{{"telemetry:read", "api", true, "policy:on-call"}}, MatchExplanation: []string{"exact service match"}, RehearsalID: "proof", RehearsalRevision: 3, RehearsalReady: true}
}
func TestLaunchDeduplicationAndBlocking(t *testing.T) {
	s, _ := New(t.TempDir())
	in := launch()
	x, e := s.Create("repo", "responder", in)
	if e != nil || x.State != "ready" {
		t.Fatalf("launch: %#v %v", x, e)
	}
	again, e := s.Create("repo", "responder", in)
	if e != nil || again.ID != x.ID {
		t.Fatalf("idempotency failed: %#v %v", again, e)
	}
	in.IdempotencyKey = "other"
	_, e = s.Create("repo", "responder", in)
	if e != ErrConflict {
		t.Fatalf("duplicate=%v", e)
	}
	in.IdempotencyKey = "blocked"
	in.Origin.ResourceID = "alert-2"
	in.RehearsalReady = false
	in.Context[0].Accessible = false
	in.Access[0].Granted = false
	blocked, e := s.Create("repo", "responder", in)
	if e != nil || blocked.State != "blocked" || len(blocked.Blockers) != 3 {
		t.Fatalf("blockers: %#v %v", blocked, e)
	}
}
