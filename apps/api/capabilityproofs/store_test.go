package capabilityproofs

import (
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
	"testing"
	"time"
)

type plans struct{ p capabilityretirements.Plan }

func (p plans) Get(repo, id string) (capabilityretirements.Plan, error) { return p.p, nil }
func rev(s string) Revisions                                            { return Revisions{s, s, s, s, s} }
func TestProofReadinessAndTargetedInvalidation(t *testing.T) {
	p := capabilityretirements.Plan{Input: capabilityretirements.Input{Stages: []capabilityretirements.Stage{{ID: "dual"}}}}
	s, e := New(t.TempDir(), plans{p})
	if e != nil {
		t.Fatal(e)
	}
	checks := []Check{}
	for _, m := range []string{"old_only", "dual_support", "replacement", "rollback", "journey"} {
		checks = append(checks, Check{ID: m, Mode: m, Journey: "checkout", Expected: "supported", InputKeys: []string{"provider", "consumer"}})
	}
	c, e := s.Create("repo", "builder", Input{PlanID: "plan", StageID: "dual", Revisions: rev("r1"), Environment: Environment{Kind: "ephemeral", Reference: "preview-1", Networkless: true, Synthetic: true, CostLimit: 10}, Checks: checks, ConsumerIDs: []string{"client"}, RequiredOwnerIDs: []string{"owner"}})
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range checks {
		c, e = s.AddAttempt("repo", c.ID, "runner", AttemptInput{CheckID: x.ID, Status: "passed", Summary: "contained", Revisions: rev("r1"), Artifacts: []Artifact{{"log", "sha256:abc", "text/plain"}}, Cost: 1})
		if e != nil {
			t.Fatal(e)
		}
	}
	now := time.Now().UTC()
	c, e = s.AddUsage("repo", c.ID, "observer", UsageInput{ConsumerID: "client", Status: "zero", EvidenceReference: "usage:1", Revisions: rev("r1"), WindowStart: now.Add(-time.Hour), WindowEnd: now})
	if e != nil {
		t.Fatal(e)
	}
	c, e = s.Acknowledge("repo", c.ID, "owner", "acknowledged", "reviewed matrix and usage")
	if e != nil {
		t.Fatal(e)
	}
	if !c.RemovalReady {
		t.Fatalf("expected ready: %#v", c.Blockers)
	}
	changed := rev("r1")
	changed.Consumer = "r2"
	c, e = s.AddAttempt("repo", c.ID, "runner", AttemptInput{CheckID: "replacement", Status: "passed", Summary: "wrong consumer revision", Revisions: changed})
	if e != nil {
		t.Fatal(e)
	}
	if !c.RemovalReady {
		t.Fatalf("stale newer evidence must not supersede current proof: %#v", c.Blockers)
	}
	c, e = s.AddUsage("repo", c.ID, "observer", UsageInput{ConsumerID: "client", Status: "inaccessible", EvidenceReference: "private:denied", Revisions: rev("r1"), WindowStart: now, WindowEnd: now.Add(time.Hour), Inaccessible: true})
	if e != nil {
		t.Fatal(e)
	}
	if c.RemovalReady {
		t.Fatal("inaccessible current usage must block removal")
	}
}
