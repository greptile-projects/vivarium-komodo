package reviewcompletion

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewwork"
)

func TestMatrixSelectiveStalenessAndOverride(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	areas := []reviewplans.Area{
		{ID: "code", Name: "Code", OwnerIDs: []string{"maintainer"}, Paths: []string{"api.go"}, CompletionRules: []string{"inspect diff"}},
		{ID: "security", Name: "Security", OwnerIDs: []string{"security"}, Paths: []string{"auth.go"}, CompletionRules: []string{"inspect boundary"}},
	}
	v := reviewplans.Version{Number: 1, Revision: "source-a", TargetRevision: "target-a", Input: reviewplans.Input{Risk: "high", Areas: areas}}
	r := reviewrouting.Routing{Revision: v.Revision, PlanVersion: 1, Assignments: []reviewrouting.Assignment{{ID: "code-review", AreaID: "code", ParticipantID: "maintainer", Kind: "human", State: "accepted", Revision: v.Revision}, {ID: "security-review", AreaID: "security", ParticipantID: "security", Kind: "human", State: "accepted", Revision: v.Revision}}}
	w := reviewwork.Workspace{Queue: []reviewwork.QueueItem{{ID: "code:diff", AreaID: "code", Accessible: true}, {ID: "security:diff", AreaID: "security", Accessible: true}}, Coverage: map[string][]string{"code:diff": {"code-review"}, "security:diff": {"security-review"}}}
	x, err := s.SetRequired("repo", "pull", "maintainer", []string{"code", "security"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Acknowledge("repo", "pull", "maintainer", "code", "code-review", "approve", "current code inspected", v, r, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	x, err = s.Acknowledge("repo", "pull", "security", "security", "security-review", "approve", "boundary inspected", v, r, x.Version)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.View("repo", "pull", v, r, w); !got.Ready {
		t.Fatalf("complete matrix not ready: %#v", got)
	}
	v.Areas[1].OwnerIDs = []string{"new-security-owner"}
	got := s.View("repo", "pull", v, r, w)
	if got.Ready || len(got.Areas[0].StaleApprovals) != 0 || len(got.Areas[1].StaleApprovals) != 1 {
		t.Fatalf("area-selective staleness lost: %#v", got)
	}
	x, err = s.Override("repo", "pull", "maintainer", "urgent containment", "task-123", []string{"security"}, now.Add(time.Hour), x.Version)
	if err != nil {
		t.Fatal(err)
	}
	got = s.View("repo", "pull", v, r, w)
	if !got.Ready || got.Areas[1].Override == nil || got.Areas[1].Override.FollowUp != "task-123" {
		t.Fatalf("bounded override not applied: %#v", got)
	}
	now = now.Add(2 * time.Hour)
	if s.View("repo", "pull", v, r, w).Ready {
		t.Fatal("expired override remained ready")
	}
}
