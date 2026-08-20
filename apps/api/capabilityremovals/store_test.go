package capabilityremovals

import (
	"errors"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityproofs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
)

type fakeProofs struct{ value capabilityproofs.Candidate }

func (f *fakeProofs) Get(string, string) (capabilityproofs.Candidate, error) { return f.value, nil }

type fakePlans struct{ value capabilityretirements.Plan }

func (f *fakePlans) Get(string, string) (capabilityretirements.Plan, error) { return f.value, nil }

func removalStore(t *testing.T) (*Store, *fakeProofs, *fakePlans) {
	t.Helper()
	p := &fakePlans{capabilityretirements.Plan{ID: "plan", Ready: true, Input: capabilityretirements.Input{Stages: []capabilityretirements.Stage{{ID: "disable"}, {ID: "delete"}}}}}
	proof := &fakeProofs{capabilityproofs.Candidate{ID: "proof", RemovalReady: true, Input: capabilityproofs.Input{PlanID: "plan", StageID: "disable", Revisions: capabilityproofs.Revisions{Provider: "candidate"}, RequiredOwnerIDs: []string{"owner"}}}}
	s, e := New(t.TempDir(), proof, p)
	if e != nil {
		t.Fatal(e)
	}
	return s, proof, p
}
func removalInput() Input {
	return Input{PlanID: "plan", ProofID: "proof", CandidateRevision: "candidate", OwnerIDs: []string{"owner"}, Stages: []Stage{{ID: "disable", Name: "Disable new use", PlanStageID: "disable", RequiredEvidence: []string{"merge_queue", "release", "documentation"}, MaxRemainingUse: 0, RollbackBoundary: "reversible"}, {ID: "delete", Name: "Remove machinery", PlanStageID: "delete", RequiredEvidence: []string{"merge_queue", "release", "schema_migration", "infrastructure_migration", "documentation", "protected_environment"}, MaxRemainingUse: 0, RollbackBoundary: "irreversible"}}}
}
func satisfy(t *testing.T, s *Store, r Removal, kinds []string) Removal {
	t.Helper()
	var e error
	for _, k := range kinds {
		r, e = s.AddEvidence("repo", r.ID, "writer", r.Revision, DeliveryEvidence{StageID: r.ActiveStageID, Kind: k, ResourceID: k + "-1", Revision: "exact", Status: "passed", Reference: "repository-visible:" + k})
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e = s.AddSignal("repo", r.ID, "observer", r.Revision, SignalInput{StageID: r.ActiveStageID, Health: "healthy", Control: "passed", EvidenceReference: "signal:1", Environment: "protected-production", Release: "release-1", NextAction: "owner may advance"})
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestStagedRemovalCompletesOnlyWithFullCleanup(t *testing.T) {
	s, _, _ := removalStore(t)
	r, e := s.Create("repo", "owner", removalInput())
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "paused" {
		t.Fatalf("missing evidence should pause: %+v", r)
	}
	r = satisfy(t, s, r, []string{"merge_queue", "release", "documentation"})
	if len(r.Blockers) != 0 || r.State != "active" {
		t.Fatalf("stage should be ready: %+v", r.Blockers)
	}
	r, e = s.Control("repo", r.ID, "owner", r.Revision, "advance", "current zero-use and health evidence accepted")
	if e != nil {
		t.Fatal(e)
	}
	if r.ActiveStageID != "delete" || r.Compatibility != "limited" {
		t.Fatalf("did not advance safely: %+v", r)
	}
	r = satisfy(t, s, r, []string{"merge_queue", "release", "schema_migration", "infrastructure_migration", "documentation", "protected_environment"})
	items := []CleanupItem{}
	for _, c := range []string{"code", "flags", "data", "credentials", "telemetry", "documentation", "policy_exceptions"} {
		status := map[string]string{"data": "deleted", "credentials": "revoked"}[c]
		if status == "" {
			status = "removed"
		}
		items = append(items, CleanupItem{Category: c, Subject: "obsolete " + c, Status: status, EvidenceReference: "digest:" + c, Revision: "final"})
	}
	r, e = s.Complete("repo", r.ID, "owner", r.Revision, CompletionInput{Cleanup: items, OutcomeMeasures: []string{"supported journeys remain healthy"}, HistoricalEvidence: []string{"proof:proof", "plan:plan"}, CompletedRevision: "final"})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "completed" || r.Compatibility != "removed" || r.Completion == nil {
		t.Fatalf("not completed: %+v", r)
	}
}

func TestRegressionPausesAndReversibleStageCanRestore(t *testing.T) {
	s, _, _ := removalStore(t)
	r, e := s.Create("repo", "owner", removalInput())
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.AddSignal("repo", r.ID, "reader", r.Revision, SignalInput{StageID: "disable", RemainingUse: 3, Health: "regressed", Control: "failed", EvidenceReference: "health:regression", Environment: "production", Release: "release-bad", NextAction: "restore compatibility"})
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "paused" {
		t.Fatalf("regression did not pause: %+v", r)
	}
	r, e = s.Control("repo", r.ID, "owner", r.Revision, "restore", "unexpected consumer cannot migrate yet")
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "restored" || r.Compatibility != "restored" {
		t.Fatalf("not restored: %+v", r)
	}
	_, e = s.DiscoverConsumer("repo", r.ID, "reader", r.Revision, "late-client", "repo:evidence", "static scan found an unsupported client")
	if e != nil {
		t.Fatal(e)
	}
}

func TestOwnerPausePersistsUntilResume(t *testing.T) {
	s, _, _ := removalStore(t)
	r, e := s.Create("repo", "owner", removalInput())
	if e != nil {
		t.Fatal(e)
	}
	r = satisfy(t, s, r, []string{"merge_queue", "release", "documentation"})
	r, e = s.Control("repo", r.ID, "owner", r.Revision, "pause", "hold during support window")
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "paused" {
		t.Fatalf("owner pause was lost: %+v", r)
	}
	r, e = s.Control("repo", r.ID, "owner", r.Revision, "resume", "support window is healthy")
	if e != nil {
		t.Fatal(e)
	}
	if r.State != "active" {
		t.Fatalf("owner resume failed: %+v", r)
	}
}

func TestAuthorityAndCurrentInputsAreEnforced(t *testing.T) {
	s, proof, plans := removalStore(t)
	_, e := s.Create("repo", "reader", removalInput())
	if !errors.Is(e, ErrForbidden) {
		t.Fatalf("non-owner create: %v", e)
	}
	proof.value.RemovalReady = false
	_, e = s.Create("repo", "owner", removalInput())
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("stale proof: %v", e)
	}
	proof.value.RemovalReady = true
	plans.value.Ready = false
	_, e = s.Create("repo", "owner", removalInput())
	if !errors.Is(e, ErrConflict) {
		t.Fatalf("stale plan: %v", e)
	}
}
