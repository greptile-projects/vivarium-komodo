package signalevaluations

import (
	"testing"
	"time"
)

func TestEvidenceDecisionAndRetirementLifecycle(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	in := Input{GapVersion: 2, Title: "checkout retry diagnosis", SignalIDs: []string{"retry"}, Signals: []Signal{{ID: "retry", ContractID: "contract", ContractVersion: 3, RolloutID: "rollout", Revision: "collector-7", Kind: "metric"}}, Queries: []Query{{ID: "q1", Expression: "retry ratio by release and provider", WindowStart: now.Add(-time.Hour), WindowEnd: now, SignalIDs: []string{"retry"}, ReleaseIDs: []string{"release-9"}, DeploymentIDs: []string{"deploy-9"}, CodeRevisions: []string{"abc"}, DependencyRevisions: []string{"payments@4"}, JourneyIDs: []string{"purchase"}, ResultDigest: "sha256:result"}}, Citations: []Citation{{ID: "c1", QueryID: "q1", Source: "telemetry://retry", Revision: "collector-7", Digest: "sha256:evidence", Accessible: true}}}
	e, err := s.Create("repo", "gap", "owner", in)
	if err != nil {
		t.Fatal(err)
	}
	e, err = s.AddFinding("repo", "gap", e.ID, "agent", "read_only_agent", FindingInput{ExpectedRevision: 1, Kind: "misleading", Statement: "aggregate retries hide provider failures", CitationIDs: []string{"c1"}, Uncertainty: "small regional sample", Reproduction: "run q1 with the pinned window", Criteria: map[string]string{"distinguish_provider": "failed"}})
	if err != nil || e.CriteriaStatus["distinguish_provider"] != "failed" {
		t.Fatalf("finding: %#v %v", e, err)
	}
	e, err = s.Resolve("repo", "gap", e.ID, "owner", ResolutionInput{ExpectedRevision: 2, FindingID: e.Findings[0].ID, Disposition: "repair_required", RepairKind: "signal_contract", RepairID: "repair-2", Rationale: "add provider correlation"})
	if err != nil || len(e.Blockers) != 1 {
		t.Fatalf("repair: %#v %v", e, err)
	}
	e, err = s.Lifecycle("repo", "gap", e.ID, "owner", LifecycleInput{ExpectedRevision: 3, Action: "remove", SignalIDs: []string{"retry"}, Rationale: "diagnosis complete", PolicyID: "retention", PolicyRevision: "4", ApprovedByID: "privacy-owner", Consumers: []Consumer{{Kind: "alert", ID: "alert-definition", Revision: "8", OwnerID: "response", Impact: "migrate to stable retry rate", Acknowledged: true}}, HistoricalMeaning: "collector-7 values retain their original schema", ProvenanceRefs: []string{"contract@3"}})
	if err != nil || e.Lifecycles[0].Applied || len(e.Lifecycles[0].Blockers) == 0 {
		t.Fatalf("unverified removal applied: %#v %v", e, err)
	}
	stopped := now
	e, err = s.Lifecycle("repo", "gap", e.ID, "owner", LifecycleInput{ExpectedRevision: 4, Action: "remove", SignalIDs: []string{"retry"}, Rationale: "diagnosis complete", PolicyID: "retention", PolicyRevision: "4", ApprovedByID: "privacy-owner", Consumers: []Consumer{{Kind: "alert", ID: "alert-definition", Revision: "8", OwnerID: "response", Impact: "migrated", Acknowledged: true}}, HistoricalMeaning: "collector-7 values retain their original schema", ProvenanceRefs: []string{"contract@3", "rollout@collector-7"}, StopEvidenceIDs: []string{"collector-zero-window"}, CollectionStoppedAt: &stopped})
	if err != nil || !e.Lifecycles[1].Applied || e.CurrentSignalState["retry"] != "remove" {
		t.Fatalf("verified removal not applied: %#v %v", e, err)
	}
}
