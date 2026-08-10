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

func TestAlternativesCompareCurrentEvidenceAndBoundResearch(t *testing.T) {
	s, _ := New(t.TempDir())
	base := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }
	in := ScopeInput{Question: "Which queue?", Constraints: []string{"Safe"}, SuccessMeasures: []string{"Fast"}, ParticipantIDs: []string{"author", "owner"}, OwnerID: "owner"}
	v, err := s.Create("repo", "author", "Queue choice", Context{Kind: "repository"}, in)
	if err != nil {
		t.Fatal(err)
	}
	claims := []Claim{
		{Kind: "assumption", Body: "Traffic stays bounded"}, {Kind: "tradeoff", Body: "More storage"},
		{Kind: "risk", Body: "Recovery lag"}, {Kind: "compatibility", Body: "Protocol unchanged"},
		{Kind: "cost", Body: "Two engineer-weeks"}, {Kind: "outcome", Body: "P95 under 100ms"},
	}
	evidence := []Evidence{{Kind: "code", RepositoryID: "repo", Revision: "abc123", Path: "queue.go", Summary: "current implementation", ObservedAt: base}}
	v, err = s.AddAlternative("repo", v.ID, "author", "Durable queue", claims, evidence)
	if err != nil || len(v.Comparison) != 1 || len(v.Comparison[0].MissingCriteria) != 0 {
		t.Fatalf("comparison: %v %#v", err, v.Comparison)
	}
	alt := v.Alternatives[0]
	oldRisk := alt.Claims[2].ID
	v, err = s.AddClaims("repo", v.ID, alt.ID, "owner", []Claim{{Kind: "risk", Body: "Recovery is bounded", SupersedesID: oldRisk}, {Kind: "dissent", Body: "Cost estimate excludes operations"}}, nil)
	if err != nil || v.Comparison[0].CurrentClaims["risk"] != "Recovery is bounded" || v.Comparison[0].DissentCount != 1 {
		t.Fatalf("claims: %v %#v", err, v.Comparison)
	}
	_, token, err := s.StartResearch("repo", v.ID, alt.ID, "owner")
	if err != nil || token == "" {
		t.Fatal("missing scoped research credential")
	}
	context, selected, err := s.ResearchContext(token)
	if err != nil || context.ID != v.ID || selected.ID != alt.ID {
		t.Fatalf("context: %v %#v", err, selected)
	}
	_, err = s.AddFinding(token, "The recovery path is exercised.", "Production volume remains unknown.", evidence)
	if err != nil {
		t.Fatal(err)
	}
	bad := []Evidence{{Kind: "usage", ResourceID: "unknown", Summary: "not retained", ObservedAt: base}}
	if _, err = s.AddFinding(token, "Unsupported", "Unknown", bad); err != ErrInvalid {
		t.Fatalf("uncited finding: %v", err)
	}
	s.now = func() time.Time { return base.Add(time.Hour) }
	in.ChangeSummary = "Traffic increased"
	v, err = s.Revise("repo", v.ID, "owner", v.Title, in)
	if err != nil || len(v.Comparison[0].StaleEvidenceIDs) != 1 || len(v.Alternatives[0].Findings) != 1 {
		t.Fatalf("staleness/history: %v %#v", err, v)
	}
	reopened, _ := New(s.root)
	got, err := reopened.Get("repo", v.ID)
	if err != nil || len(got.Alternatives) != 1 || len(got.Alternatives[0].Claims) != 8 {
		t.Fatalf("persistence: %v %#v", err, got)
	}
}

func TestExperimentRetainsWorkspaceEvidenceAndReportsInvalidation(t *testing.T) {
	s, _ := New(t.TempDir())
	in := ScopeInput{Question: "Which cache?", Constraints: []string{"bounded"}, SuccessMeasures: []string{"latency"}, ParticipantIDs: []string{"author", "owner"}, OwnerID: "owner"}
	v, _ := s.Create("repo", "author", "Cache", Context{Kind: "repository"}, in)
	v, _ = s.AddAlternative("repo", v.ID, "author", "Local cache", []Claim{{Kind: "assumption", Body: "fits"}}, nil)
	alt := v.Alternatives[0]
	revision := "0123456789012345678901234567890123456789"
	v, x, err := s.StartExperiment("repo", v.ID, alt.ID, "owner", Experiment{WorkspaceID: "workspace-1", Revision: revision, CommandName: "benchmark", DefinitionDigest: "env-1", DependencyDigest: "deps-1"})
	if err != nil || x.CreatedByID != "owner" || x.State != "running" {
		t.Fatalf("start: %v %#v", err, x)
	}
	v, err = s.AddExperimentCheckpoint("repo", v.ID, alt.ID, x.ID, "author", ExperimentCheckpoint{WorkspaceCheckpointID: "checkpoint-1", Summary: "p95 improved", Measurements: []Measurement{{Name: "p95", Value: 42, Unit: "ms"}}, LogSequences: []int64{3}, ArtifactPaths: []string{"bench.json"}, ResourceUse: map[string]int64{"cpu_seconds": 4}})
	if err != nil || v.Alternatives[0].Experiments[0].Checkpoints[0].ActorID != "author" || v.Alternatives[0].Experiments[0].State != "completed" {
		t.Fatalf("checkpoint: %v %#v", err, v)
	}
	v, err = s.AssessExperiment("repo", v.ID, alt.ID, x.ID, "owner", revision, "deps-2", "env-2")
	got := v.Alternatives[0].Experiments[0]
	if err != nil || got.State != "invalidated" || len(got.InvalidatedBy) != 2 {
		t.Fatalf("validity: %v %#v", err, got)
	}
}

func TestDecisionCommitmentApprovalsReopeningAndExceptions(t *testing.T) {
	s, _ := New(t.TempDir())
	base := time.Date(2026, 8, 10, 15, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }
	in := ScopeInput{Question: "Which queue?", Constraints: []string{"No loss"}, SuccessMeasures: []string{"Recovery under a minute"}, ParticipantIDs: []string{"author", "owner", "affected"}, OwnerID: "owner"}
	v, _ := s.Create("repo", "author", "Queue commitment", Context{Kind: "repository"}, in)
	evidence := []Evidence{{Kind: "code", RepositoryID: "repo", Revision: "0123456789012345678901234567890123456789", Path: "queue.go", Summary: "bounded recovery implementation", ObservedAt: base}}
	v, _ = s.AddAlternative("repo", v.ID, "author", "Durable queue", []Claim{{Kind: "tradeoff", Body: "Additional storage"}, {Kind: "dissent", Body: "Operations cost remains uncertain"}}, evidence)
	selected := v.Alternatives[0].ID
	v, _ = s.AddAlternative("repo", v.ID, "affected", "In-memory queue", []Claim{{Kind: "risk", Body: "Restart loses work"}}, nil)
	rejected := v.Alternatives[1].ID

	v, err := s.RequestApproval("repo", v.ID, "owner", "acknowledgement", "affected", "")
	if err != nil || len(v.PendingApprovalIDs) != 1 {
		t.Fatalf("request = %#v, %v", v, err)
	}
	requirement := v.ApprovalRequirements[0].ID
	if _, err = s.Publish("repo", v.ID, "owner", selected, []string{rejected}, "Best recovery evidence.", []string{"Additional storage"}, []string{"Operations cost remains uncertain"}, []string{"Monitor recovery"}, nil, evidence); err != ErrInvalid {
		t.Fatalf("published without acknowledgement: %v", err)
	}
	v, err = s.RespondApproval("repo", v.ID, requirement, "affected", "acknowledged", "Impact understood.")
	if err != nil || len(v.PendingApprovalIDs) != 0 {
		t.Fatalf("response = %#v, %v", v, err)
	}
	review := base.Add(30 * 24 * time.Hour)
	v, err = s.Publish("repo", v.ID, "owner", selected, []string{rejected}, "Best recovery evidence.", []string{"Additional storage"}, []string{"Operations cost remains uncertain"}, []string{"Monitor recovery"}, &review, evidence)
	if err != nil || v.State != "published" || len(v.Commitments) != 1 || v.Commitments[0].Evidence[0].Path != "queue.go" {
		t.Fatalf("commitment = %#v, %v", v, err)
	}

	s.now = func() time.Time { return base.Add(time.Hour) }
	in.ChangeSummary = "Recovery target tightened"
	in.SuccessMeasures = []string{"Recovery under thirty seconds"}
	v, err = s.Revise("repo", v.ID, "owner", v.Title, in)
	if err != nil || v.State != "reopened" || len(v.Commitments) != 1 {
		t.Fatalf("reopen = %#v, %v", v, err)
	}

	expires := base.Add(48 * time.Hour)
	v, err = s.AuthorizeException("repo", v.ID, "owner", Exception{Scope: "legacy worker", Reason: "Migration requires the old retry interval", Conditions: []string{"read-only traffic"}, ExpiresAt: expires})
	if err != nil || len(v.Exceptions) != 1 || v.Exceptions[0].CommitmentVersion != 1 {
		t.Fatalf("exception = %#v, %v", v, err)
	}
	v, err = s.RevokeException("repo", v.ID, v.Exceptions[0].ID, "owner")
	if err != nil || v.Exceptions[0].RevokedAt == nil {
		t.Fatalf("revoke = %#v, %v", v, err)
	}
}

func TestRejectedPolicyApprovalIsPublicConflict(t *testing.T) {
	s, _ := New(t.TempDir())
	in := ScopeInput{Question: "Ship?", Constraints: []string{"Policy"}, SuccessMeasures: []string{"Approved"}, ParticipantIDs: []string{"owner", "reviewer"}, OwnerID: "owner"}
	v, _ := s.Create("repo", "owner", "Policy decision", Context{Kind: "repository"}, in)
	v, _ = s.RequestApproval("repo", v.ID, "owner", "approval", "reviewer", "architecture/high-risk")
	v, err := s.RespondApproval("repo", v.ID, v.ApprovalRequirements[0].ID, "reviewer", "rejected", "Violates compatibility policy.")
	if err != nil || len(v.Conflicts) != 1 || v.ApprovalRequirements[0].Note == "" {
		t.Fatalf("conflict = %#v, %v", v, err)
	}
}
