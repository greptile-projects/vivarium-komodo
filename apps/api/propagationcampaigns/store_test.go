package propagationcampaigns

import (
	"testing"
	"time"
)

func TestCampaignRetainsExplicitTargetsAndBlockers(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	in := Input{Title: "Parser repair propagation", Intent: "Preserve strict parsing while accepting legacy headers", AcceptanceCriteria: []string{"legacy headers work", "strict syntax remains"}, Source: Source{Kind: "regression_correction", RepositoryID: "origin", ResourceID: "correction-1", Revision: "abc", CommitIDs: []string{"abc"}}, Targets: []Target{
		{ID: "stable", RepositoryID: "origin", ReleaseLine: "v2", Revision: "def", OwnerIDs: []string{"owner"}, Deadline: now.Add(time.Hour), Disposition: "pending", Authority: Authority{OwnerIDs: []string{"owner"}, Access: "write", Basis: "repository collaborator", ObservedAt: now}},
		{ID: "legacy", RepositoryReference: "https://peer.example/repos/lib", ReleaseLine: "v1", Deadline: now.Add(time.Hour), DependsOn: []string{"stable"}, Disposition: "inaccessible", DispositionReason: "peer access unavailable", Authority: Authority{OwnerIDs: []string{"peer-owner"}, Access: "unknown", Basis: "federated reference only", ObservedAt: now}},
	}, CompletionPolicy: CompletionPolicy{Mode: "all_supported", ExceptionRequiresOwner: true}}
	x, e := s.Create("origin", "author", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Blockers) != 1 || x.Blockers[0].Kind != "inaccessible" {
		t.Fatalf("missing blocker: %#v", x)
	}
	got, e := s.Get("origin", x.ID)
	if e != nil || got.Source.CommitIDs[0] != "abc" || got.Targets[1].Authority.Access != "unknown" {
		t.Fatalf("lost provenance: %#v %v", got, e)
	}
}

func TestCampaignRejectsCyclesAndImplicitUnknowns(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	base := Target{RepositoryID: "r", ReleaseLine: "v1", Deadline: now, Authority: Authority{Access: "read", Basis: "membership", ObservedAt: now}}
	a := base
	a.ID = "a"
	a.Disposition = "unknown" // reason is deliberately absent
	in := Input{Title: "x", Intent: "x", AcceptanceCriteria: []string{"x"}, Source: Source{Kind: "policy_change", RepositoryID: "r", ResourceID: "p", Revision: "c", CommitIDs: []string{"c"}}, Targets: []Target{a}, CompletionPolicy: CompletionPolicy{Mode: "all_supported"}}
	if _, e := s.Create("r", "u", in); e != ErrInvalid {
		t.Fatalf("expected invalid, got %v", e)
	}
	a.Disposition = "pending"
	a.DependsOn = []string{"b"}
	b := base
	b.ID = "b"
	b.Disposition = "pending"
	b.DependsOn = []string{"a"}
	in.Targets = []Target{a, b}
	if _, e := s.Create("r", "u", in); e != ErrInvalid {
		t.Fatalf("cycle accepted: %v", e)
	}
}

func assessment(revision string, proof bool) AssessmentInput {
	kinds := []string{"history", "symbols", "dependencies", "interfaces", "schemas", "prior_fixes", "release_commitments"}
	comparisons := make([]Comparison, 0, len(kinds))
	for _, kind := range kinds {
		comparisons = append(comparisons, Comparison{Kind: kind, SourceSummary: "source evidence", TargetSummary: "target evidence", Conclusion: "matched", BehavioralProof: proof && kind == "prior_fixes", Citations: []Citation{{Kind: kind, Reference: "evidence:" + kind, Revision: revision}}})
	}
	return AssessmentInput{TargetRevision: revision, SourceRevision: "abc", Classification: "already_satisfied", Rationale: "A prior target fix satisfies the source behavior.", Comparisons: comparisons, AssumptionsStillHold: true}
}

func TestAssessmentsAreEvidenceBoundAndStaleOnlyTheirTarget(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	targets := []Target{{ID: "a", RepositoryID: "r", ReleaseLine: "v1", OwnerIDs: []string{"owner"}, Deadline: now, Disposition: "pending", Authority: Authority{OwnerIDs: []string{"owner"}, Access: "read", Basis: "membership", ObservedAt: now}}, {ID: "b", RepositoryID: "r", ReleaseLine: "v2", OwnerIDs: []string{"other"}, Deadline: now, Disposition: "pending", Authority: Authority{Access: "read", Basis: "membership", ObservedAt: now}}}
	c, err := s.Create("r", "author", Input{Title: "x", Intent: "x", AcceptanceCriteria: []string{"x"}, Source: Source{Kind: "policy_change", RepositoryID: "r", ResourceID: "p", Revision: "abc", CommitIDs: []string{"abc"}}, Targets: targets, CompletionPolicy: CompletionPolicy{Mode: "all_supported"}})
	if err != nil {
		t.Fatal(err)
	}
	weak := assessment("one", false)
	if _, err = s.Assess("r", c.ID, "a", "analyst", weak); err != ErrInvalid {
		t.Fatalf("similarity became equivalence: %v", err)
	}
	c, err = s.Assess("r", c.ID, "a", "analyst", assessment("one", true))
	if err != nil {
		t.Fatal(err)
	}
	first := c.Assessments[0].ID
	c, err = s.Assess("r", c.ID, "b", "analyst", assessment("b-one", true))
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.Assess("r", c.ID, "a", "analyst", assessment("two", true))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Assessments[0].Stale || c.Assessments[1].Stale || c.Assessments[2].Stale {
		t.Fatalf("incorrect selective staleness: %#v", c.Assessments)
	}
	c, err = s.AddFinding("r", c.ID, "a", first, "reader", FindingInput{ActorKind: "read_only_agent", Summary: "Release promise may differ", Uncertainty: "Runtime unavailable", Citations: []Citation{{Kind: "release_commitments", Reference: "release:v1", Revision: "one"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Acknowledge("r", c.ID, "a", first, "reader", "acknowledged", "looks right"); err != ErrForbidden {
		t.Fatalf("non-owner acknowledgement: %v", err)
	}
	c, err = s.Acknowledge("r", c.ID, "a", first, "owner", "changes_requested", "Recheck the supported schema.")
	if err != nil || len(c.Assessments[0].Acknowledgements) != 1 || len(c.Assessments[0].Findings) != 1 {
		t.Fatalf("collaboration lost: %#v %v", c, err)
	}
}

func TestContributionPreservesIntentAuthorshipAdaptationAndBoundaries(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	targets := []Target{
		{ID: "local", RepositoryID: "origin", ReleaseLine: "stable", Revision: "stable-1", OwnerIDs: []string{"maintainer"}, Deadline: now.Add(time.Hour), Disposition: "pending", Authority: Authority{Access: "write", Basis: "collaborator", ObservedAt: now}},
		{ID: "peer", RepositoryReference: "https://peer.example/lib", ReleaseLine: "legacy", Revision: "peer-1", Deadline: now.Add(time.Hour), Disposition: "pending", Authority: Authority{Access: "contribute", Basis: "ordinary federation", ObservedAt: now}},
	}
	c, err := s.Create("origin", "campaign-owner", Input{Title: "propagate", Intent: "accept legacy headers without weakening strict parsing", AcceptanceCriteria: []string{"legacy headers pass", "strict inputs stay strict"}, Source: Source{Kind: "regression_correction", RepositoryID: "origin", ResourceID: "correction-1", Revision: "source-1", CommitIDs: []string{"source-1", "test-1"}}, Targets: targets, CompletionPolicy: CompletionPolicy{Mode: "all_supported"}})
	if err != nil {
		t.Fatal(err)
	}
	a := assessment("stable-1", true)
	a.SourceRevision = "source-1"
	a.Classification = "adaptation_required"
	c, err = s.Assess("origin", c.ID, "local", "analyst", a)
	if err != nil {
		t.Fatal(err)
	}
	in := ContributionInput{AssessmentID: c.Assessments[0].ID, Mode: "adapted", Rationale: "Use the stable parser adapter.", SourceAuthorIDs: []string{"original-author"}, RelevantCommitIDs: []string{"source-1", "test-1"}, Constraints: []string{"no schema change"}, AcceptanceCriteria: []string{"legacy headers pass", "strict inputs stay strict"}, Deviations: []string{"replace the new parser hook with the stable adapter"}, ContextReferences: []string{"assessment:" + c.Assessments[0].ID, "embargo:digest-only"}, Tasks: []ContributionTask{
		{ID: "implement", Title: "Adapt parser repair", OwnerKind: "agent", OwnerID: "repair-agent", Scope: []string{"parser", "tests"}, AcceptanceCriteria: []string{"both parser behaviors pass"}, TaskID: "task:1", SessionID: "session:1", WorkspaceID: "workspace:1"},
		{ID: "review", Title: "Review stable behavior", OwnerKind: "human", OwnerID: "maintainer", DependsOn: []string{"implement"}, Scope: []string{"review only"}, AcceptanceCriteria: []string{"intent is preserved"}, PullRequestID: "pull:1"},
	}}
	c, err = s.CreateContribution("origin", c.ID, "local", "campaign-owner", in)
	if err != nil || len(c.Contributions) != 1 {
		t.Fatalf("contribution: %#v %v", c, err)
	}
	got := c.Contributions[0]
	if got.SourceIntent != c.Intent || got.SourceAuthorIDs[0] != "original-author" || len(got.Deviations) != 1 || len(got.AuthorityGranted) != 0 || got.Tasks[1].DependsOn[0] != "implement" {
		t.Fatalf("lost contribution provenance: %#v", got)
	}

	peer := assessment("peer-1", true)
	peer.SourceRevision = "source-1"
	peer.Classification = "directly_applicable"
	c, err = s.Assess("origin", c.ID, "peer", "analyst", peer)
	if err != nil {
		t.Fatal(err)
	}
	direct := ContributionInput{AssessmentID: c.Assessments[1].ID, Mode: "direct", Rationale: "Apply the proven change unchanged.", SourceAuthorIDs: []string{"original-author"}, RelevantCommitIDs: []string{"source-1"}, Constraints: []string{"peer review required"}, AcceptanceCriteria: []string{"legacy headers pass"}, Tasks: []ContributionTask{{ID: "send", Title: "Send upstream", OwnerKind: "human", OwnerID: "contributor", Scope: []string{"contribution"}, AcceptanceCriteria: []string{"peer can review"}, PullRequestID: "pull:peer"}}}
	if _, err = s.CreateContribution("origin", c.ID, "peer", "campaign-owner", direct); err != ErrInvalid {
		t.Fatalf("implicit peer write accepted: %v", err)
	}
	direct.Tasks[0].ForkRepositoryID = "fork:peer"
	if _, err = s.CreateContribution("origin", c.ID, "peer", "campaign-owner", direct); err != nil {
		t.Fatalf("ordinary fork rejected: %v", err)
	}
}

func TestEquivalenceMatrixRequiresExactBoundedProofAndSelectiveInvalidation(t *testing.T) {
	s, _ := New(t.TempDir())
	now := time.Now().UTC()
	targets := []Target{
		{ID: "stable", RepositoryID: "origin", ReleaseLine: "v2", Revision: "stable-1", OwnerIDs: []string{"owner"}, Deadline: now, Disposition: "pending", Authority: Authority{Access: "write", Basis: "membership", ObservedAt: now}},
		{ID: "peer", RepositoryID: "peer", ReleaseLine: "v1", Revision: "peer-1", Deadline: now, Disposition: "pending", Authority: Authority{Access: "read", Basis: "federation", ObservedAt: now}},
	}
	c, err := s.Create("origin", "author", Input{Title: "equivalence", Intent: "same parser behavior", AcceptanceCriteria: []string{"legacy accepted", "strict remains"}, Source: Source{Kind: "regression_correction", RepositoryID: "origin", ResourceID: "repair", Revision: "source-1", CommitIDs: []string{"source-1"}}, Targets: targets, CompletionPolicy: CompletionPolicy{Mode: "all_supported"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		a := assessment(target.Revision, true)
		a.SourceRevision = "source-1"
		a.Classification = "adaptation_required"
		c, err = s.Assess("origin", c.ID, target.ID, "analyst", a)
		if err != nil {
			t.Fatal(err)
		}
	}
	spec := EquivalenceSpecificationInput{SourceRevision: "source-1", Environment: "networkless-go-1.25", MaximumCost: 5, Currency: "USD", TimeoutSeconds: 300, Scenarios: []EquivalenceScenario{
		{ID: "legacy", Behavior: "legacy header is accepted", SourceEvidence: []string{"test:source-1"}, Commands: []string{"go test ./parser -run Legacy"}, RequiredCoverage: []string{"legacy-header"}, OrdinaryCheckNames: []string{"unit"}},
		{ID: "strict", Behavior: "invalid syntax remains rejected", SourceEvidence: []string{"requirement:strict"}, Commands: []string{"go test ./parser -run Strict"}, RequiredCoverage: []string{"strict-rejection"}, OrdinaryCheckNames: []string{"unit"}, SubstituteAllowed: true, SubstituteRequirement: "owner-reviewed conformance trace"},
	}}
	c, err = s.DefineEquivalence("origin", c.ID, "author", spec)
	if err != nil {
		t.Fatal(err)
	}
	attempt := func(targetID, targetRevision, dependency string, unsupported bool) EquivalenceAttemptInput {
		status, substitute := "passed", []string(nil)
		if unsupported {
			status = "unsupported"
			substitute = []string{"owner-reviewed trace artifact sha256:def"}
		}
		return EquivalenceAttemptInput{SpecificationID: c.EquivalenceSpecifications[0].ID, AssessmentID: func() string {
			for _, a := range c.Assessments {
				if a.TargetID == targetID {
					return a.ID
				}
			}
			return ""
		}(), SourceRevision: "source-1", TargetRevision: targetRevision, AdaptationRevision: targetRevision + "-adapt", Environment: spec.Environment, BoundInputs: []BoundInput{{Key: "source", Revision: "source-1"}, {Key: "target", Revision: targetRevision}, {Key: "dependency:parser", Revision: dependency}, {Key: "assumption:header-format", Revision: "v1"}}, Evidence: []ScenarioEvidence{{ScenarioID: "legacy", Status: "passed", Commands: []string{"go test ./..."}, OrdinaryChecks: []string{"unit"}, Logs: []string{"PASS legacy"}, Artifacts: []Artifact{{Name: "junit", Digest: "sha256:abc", MediaType: "application/xml", Size: 12}}, Coverage: []string{"legacy-header"}}, {ScenarioID: "strict", Status: status, Commands: []string{"go test ./..."}, OrdinaryChecks: []string{"unit"}, Logs: []string{"conformance retained"}, Coverage: []string{"strict-rejection"}, SubstituteEvidence: substitute, ResidualDifference: "different internal parser"}}, Cost: 1.25, Currency: "USD", DurationSeconds: 20}
	}
	c, err = s.RecordEquivalenceAttempt("origin", c.ID, "stable", "runner", attempt("stable", "stable-1", "dep-1", false))
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.RecordEquivalenceAttempt("origin", c.ID, "peer", "runner", attempt("peer", "peer-1", "dep-old", true))
	if err != nil {
		t.Fatal(err)
	}
	c, err = s.RecordEquivalenceAttempt("origin", c.ID, "stable", "runner", attempt("stable", "stable-1", "dep-2", false))
	if err != nil {
		t.Fatal(err)
	}
	if !c.EquivalenceAttempts[0].Stale || c.EquivalenceAttempts[1].Stale || c.EquivalenceAttempts[2].Stale || !c.EquivalenceAttempts[1].Passing {
		t.Fatalf("selective matrix staleness/substitute failed: %#v", c.EquivalenceAttempts)
	}
	if _, err = s.DecideEquivalence("origin", c.ID, "stable", c.EquivalenceAttempts[2].ID, "not-owner", "accepted", "proof reviewed"); err != ErrForbidden {
		t.Fatalf("non-owner decided: %v", err)
	}
	c, err = s.DecideEquivalence("origin", c.ID, "stable", c.EquivalenceAttempts[2].ID, "owner", "accepted", "ordinary checks and coverage prove the behavior")
	if err != nil || len(c.EquivalenceAttempts[2].OwnerDecisions) != 1 {
		t.Fatalf("owner decision lost: %#v %v", c, err)
	}
	bad := attempt("peer", "peer-1", "dep-old", true)
	bad.Evidence[1].SubstituteEvidence = nil
	if _, err = s.RecordEquivalenceAttempt("origin", c.ID, "peer", "runner", bad); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get("origin", c.ID)
	last := got.EquivalenceAttempts[len(got.EquivalenceAttempts)-1]
	if last.Passing || len(last.Blockers) == 0 || last.Blockers[0].Kind != "missing_substitute_evidence" {
		t.Fatalf("unsupported test became proof: %#v", last)
	}
}
