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
