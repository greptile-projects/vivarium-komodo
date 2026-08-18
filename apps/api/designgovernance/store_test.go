package designgovernance

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
)

func TestAcceptanceEvolutionAndRepairRemainPolicyBound(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	p, err := s.CreatePolicy("repository", "repo", "", "owner", PolicyInput{Name: "shared review language", TargetBranches: []string{"main"}, Selector: Selector{Components: []string{"review-card"}, Paths: []string{"web/**"}}, RequiredRoles: []string{"design_owner", "accessibility", "content", "localization", "invited_user"}})
	if err != nil {
		t.Fatal(err)
	}
	run := interfacechecks.Run{Revision: "candidate", Current: true, Cases: []interfacechecks.Case{{Current: true, Differences: []interfacechecks.Difference{{ID: "spacing", Current: true}}}}}
	a, err := s.Assess("repo", "pull", "candidate", "main", Selector{Components: []string{"review-card"}, Paths: []string{"web/review.tsx"}}, []Policy{p}, []interfacechecks.Run{run}, []Usage{{ComponentID: "legacy-button", Version: 1, Path: "web/review.tsx", Obsolete: true}})
	if err != nil || a.Ready || len(a.ObsoleteUses) != 1 {
		t.Fatalf("initial assessment = %#v, %v", a, err)
	}
	for _, role := range p.RequiredRoles {
		if _, err = s.Accept("repo", "pull", p.ID, "candidate", "preview", role, "accepted", "reviewed exact candidate", role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Except("repo", "pull", p.ID, "candidate", "bounded compatibility period", "owner", "task", "remove-legacy", "owner", now.Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	run.Cases[0].Differences[0].Classification = "intentional"
	a, _ = s.Assess("repo", "pull", "candidate", "main", Selector{Components: []string{"review-card"}}, []Policy{p}, []interfacechecks.Run{run}, nil)
	if !a.Ready || len(a.ExpiringException) != 1 {
		t.Fatalf("accepted assessment = %#v", a)
	}
	if _, err = s.CreateWork("design-owner", WorkInput{Kind: "migration", SystemID: "core", SystemVersion: 2, SourceKind: "design_system_change", SourceID: "core-v2", AffectedRepository: "repo", DocumentationIDs: []string{"docs"}, OwnerID: "maintainer", Summary: "adopt review card v2", AcceptanceCriteria: []string{"replace obsolete component", "update documentation"}}); err != nil {
		t.Fatal(err)
	}
	w, err := s.CreateWork("observer", WorkInput{Kind: "repair", ReleaseID: "release", SourceKind: "regression", SourceID: "observation", AffectedRepository: "repo", OwnerID: "maintainer", Summary: "restore shipped focus state", AcceptanceCriteria: []string{"current interface check passes"}})
	if err != nil || w.GrantsAuthority {
		t.Fatalf("repair work = %#v, %v", w, err)
	}
	stale, _ := s.Assess("repo", "pull", "changed", "main", Selector{Components: []string{"review-card"}}, []Policy{p}, []interfacechecks.Run{run}, nil)
	if stale.Ready {
		t.Fatalf("old acceptance or preview survived revision: %#v", stale)
	}
}
