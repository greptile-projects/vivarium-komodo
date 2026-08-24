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
