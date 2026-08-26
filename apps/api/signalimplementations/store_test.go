package signalimplementations

import "testing"

func TestGovernedWorkAndBoundedCandidateProof(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := []Work{
		{Kind: "task", OwnerKind: "human", OwnerID: "maintainer", RepositoryID: "app", ResourceID: "task-1", Revision: "1", Permitted: true},
		{Kind: "session", OwnerKind: "agent", OwnerID: "agent:otel", RepositoryID: "library", ResourceID: "session-1", Revision: "2", Permitted: true},
		{Kind: "workspace", OwnerKind: "agent", OwnerID: "agent:otel", RepositoryID: "library", ResourceID: "workspace-1", Revision: "commit-a", Permitted: true},
		{Kind: "pull_request", OwnerKind: "human", OwnerID: "infra-owner", RepositoryID: "infra", ResourceID: "pull-9", Revision: "commit-i", Permitted: true},
	}
	p, err := s.CreatePlan("app", "contract-1", 3, "base-a", "maintainer", work)
	if err != nil || len(p.Work) != 4 || len(p.NonAuthority) == 0 {
		t.Fatalf("plan: %+v %v", p, err)
	}
	work[0].Permitted = false
	if _, err = s.CreatePlan("app", "contract-1", 3, "base-a", "maintainer", work); err != ErrInvalid {
		t.Fatalf("unpermitted work accepted: %v", err)
	}

	results := []Result{}
	for _, name := range required {
		results = append(results, Result{Check: name, Status: "passed", Summary: name + " matches the contract", Coverage: []string{"checkout.success", "checkout.timeout"}, Evidence: []Evidence{{Kind: "trace", Summary: "sanitized synthetic evidence", Digest: "sha256:1234567890abcdef", Sanitized: true, Accessible: true}}})
	}
	run := Run{RepositoryID: "app", PullRequestID: "pull-1", CandidateRevision: "candidate-a", PlanID: p.ID, ContractID: "contract-1", ContractVersion: 3, ConfigPath: ".komodo/telemetry-checks.json", ConfigRevision: "blob-1", Journey: "synthetic checkout", Failure: "synthetic provider timeout", Results: results, Differences: []Difference{{Kind: "sampling", Summary: "candidate uses the reviewed rate", Expected: "0.1", Actual: "0.1"}}, DurationMS: 1200, Cost: 0.04, Currency: "USD", Authorship: []string{"maintainer", "agent:otel"}, PolicyChecks: []string{"review", "privacy", "security", "provenance", "merge"}, CreatedByID: "maintainer"}
	created, err := s.CreateRun(run)
	if err != nil || !created.Passed || len(created.Findings) != 0 || len(created.NonAuthority) == 0 {
		t.Fatalf("run: %+v %v", created, err)
	}
	run.Results[0].Evidence[0].Sanitized = false
	if _, err = s.CreateRun(run); err != ErrInvalid {
		t.Fatalf("unsanitized evidence accepted: %v", err)
	}
	run.Results = run.Results[:len(run.Results)-1]
	run.Results[0].Evidence[0].Sanitized = true
	missing, err := s.CreateRun(run)
	if err != nil || missing.Passed || len(missing.Findings) != 1 {
		t.Fatalf("missing coverage not retained: %+v %v", missing, err)
	}
}
