package responseoutcomes

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
)

func alert(id, team string, created time.Time, missed bool) responsealerts.Alert {
	d := created.Add(time.Minute)
	ack := created.Add(30 * time.Second)
	if missed {
		ack = created.Add(2 * time.Minute)
	}
	return responsealerts.Alert{ID: id, Revision: 4, PolicyID: "policy", PolicyVersion: 2, RotationID: "rotation", TeamID: team, Signal: responsealerts.Signal{ObservedAt: created, AffectedUserCount: 12}, ResponseDeadline: &d, Status: "resolved", Events: []responsealerts.Event{{Kind: "acknowledged", CreatedAt: ack}, {Kind: "reassign", CreatedAt: ack}, {Kind: "escalate", CreatedAt: ack}, {Kind: "resolve", CreatedAt: created.Add(5 * time.Minute)}}, Workspace: &responsealerts.Workspace{IncidentID: "incident"}}
}
func TestOutcomeLearningApprovalAndContainment(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC().Add(-time.Hour)
	create := func(n int, unsafe bool) Outcome {
		a := alert(string(rune('a'+n)), "ops", now.Add(time.Duration(n)*time.Minute), true)
		o, e := s.Create("repo", "owner", Input{AlertID: a.ID, ExpectedAlertRevision: 4, Summary: "review response", Resolution: "recovered", Audience: "owners", Owners: []string{"owner", "approver"}, EvidenceReferences: []string{"incident:1"}, ResponderMinutes: 20, AgentCost: 1.25, Interruptions: 1, RecoveredUsers: 12, UnsafeAutomation: unsafe}, a)
		if e != nil {
			t.Fatal(e)
		}
		return o
	}
	_ = create(0, false)
	_ = create(1, false)
	o := create(2, true)
	if o.Metrics.AcknowledgementSeconds != 120 || o.Metrics.ResolutionSeconds != 300 || o.Metrics.Handoffs != 1 || o.Metrics.Escalations != 1 || o.Metrics.MissedTargets != 1 || o.Metrics.IncidentCount != 1 || len(o.Controls) != 2 {
		t.Fatalf("metrics or containment missing: %+v", o)
	}
	o, e := s.Correct("repo", o.ID, "owner", o.Revision, Correction{Kind: "routing_policy", Summary: "reduce automation scope", ResourceID: "policy", MaterialAuthorityChange: true})
	if e != nil {
		t.Fatal(e)
	}
	if o.Corrections[0].Status != "pending_ordinary_approval" {
		t.Fatal("material change bypassed approval")
	}
	if _, e = s.Approve("repo", o.ID, o.Corrections[0].ID, "owner", o.Revision); e != ErrInvalid {
		t.Fatalf("proposer self-approved: %v", e)
	}
	o, e = s.Approve("repo", o.ID, o.Corrections[0].ID, "approver", o.Revision)
	if e != nil || o.Corrections[0].Status != "approved" {
		t.Fatalf("independent approval absent: %+v %v", o, e)
	}
	o, e = s.AddWork("repo", o.ID, "owner", o.Revision, Work{Kind: "staffing", OwnerKind: "human", OwnerID: "lead", ResourceID: "task:7", Summary: "restore coverage"})
	if e != nil || len(o.Work) != 1 {
		t.Fatalf("linked work missing: %+v %v", o, e)
	}
	_, summary, e := s.List("repo", "owner", true)
	if e != nil || summary.Outcomes != 3 || summary.Metrics.MissedTargets != 3 || summary.PausedRouting != 1 || summary.ActivatedBackups != 1 {
		t.Fatalf("summary wrong: %+v %v", summary, e)
	}
}

func TestUserOutcomeRequiresConsentAndVisibility(t *testing.T) {
	s, _ := New(t.TempDir())
	a := alert("a", "ops", time.Now().UTC().Add(-time.Hour), false)
	_, e := s.Create("repo", "owner", Input{AlertID: "a", ExpectedAlertRevision: 4, Summary: "x", Resolution: "x", Audience: "public", Owners: []string{"owner"}, UserOutcome: "customer recovered"}, a)
	if e != ErrInvalid {
		t.Fatalf("unconsented outcome accepted: %v", e)
	}
	o, e := s.Create("repo", "owner", Input{AlertID: "a", ExpectedAlertRevision: 4, Summary: "x", Resolution: "x", Audience: "owners", Owners: []string{"owner"}}, a)
	if e != nil {
		t.Fatal(e)
	}
	xs, _, _ := s.List("repo", "stranger", false)
	if len(xs) != 0 {
		t.Fatalf("owners-only outcome leaked: %+v", o)
	}
}
