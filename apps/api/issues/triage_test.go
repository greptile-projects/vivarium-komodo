package issues

import "testing"

func TestTriageAndInvestigationRetainAttributionDisputeAndStaleness(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.Create(CreateInput{RepositoryID: "repo", ReporterID: "reporter", Title: "broken clone", ExpectedBehavior: "works", ObservedBehavior: "fails", Severity: "high", Environment: "linux", ReproductionSteps: []string{"clone"}, Visibility: "repository"})
	if err != nil {
		t.Fatal(err)
	}
	item, err = s.SetTriage("repo", item.ID, "owner", item.Version, Triage{Classification: "regression", Priority: "urgent", AssigneeIDs: []string{"maintainer"}, Labels: []string{"git"}})
	if err != nil || item.Triage.UpdatedByID != "owner" || item.Version != 2 {
		t.Fatalf("triage = %#v err=%v", item.Triage, err)
	}
	if _, err = s.SetTriage("repo", item.ID, "owner", 1, Triage{Priority: "low"}); err != ErrConflict {
		t.Fatalf("stale triage err = %v", err)
	}
	item, err = s.AddRelationship("repo", item.ID, "owner", Relationship{Kind: "code", ResourceID: "repo", Revision: "aaaaaaaa", Path: "clone.go"})
	if err != nil || len(item.Relationships) != 1 {
		t.Fatalf("relationship = %#v err=%v", item.Relationships, err)
	}
	item, inv, err := s.CreateInvestigation("repo", item.ID, "attempt-1", "aaaaaaaa", "maintainer")
	if err != nil {
		t.Fatal(err)
	}
	item, hypothesis, err := s.AddInvestigationEntry("repo", item.ID, inv.ID, "human", "maintainer", InvestigationEntry{Kind: "hypothesis", Body: "the clone path rejects an empty ref", Citations: []Citation{{Kind: "reproduction_event", ResourceID: "attempt-1", EventSequence: 2}}, SuspectedRevisions: []string{"aaaaaaaa"}, SuspectedOwnerIDs: []string{"maintainer"}})
	if err != nil {
		t.Fatal(err)
	}
	item, challenge, err := s.AddInvestigationEntry("repo", item.ID, inv.ID, "human", "reporter", InvestigationEntry{Kind: "challenge", Body: "the trace shows the ref is present", TargetEntryID: hypothesis.ID, Citations: []Citation{{Kind: "relationship", ResourceID: item.Relationships[0].ID}}})
	if err != nil || challenge.AuthorID != "reporter" || !item.Investigations[0].Entries[0].Disputed {
		t.Fatalf("challenge=%#v investigation=%#v err=%v", challenge, item.Investigations[0], err)
	}
	item, token, err := s.StartAgentRun("repo", item.ID, inv.ID, "diagnostic-agent", "owner")
	if err != nil || token == "" {
		t.Fatal("agent run was not created")
	}
	_, context, run, err := s.AgentContext(token)
	if err != nil || context.ID != inv.ID || run.AgentID != "diagnostic-agent" {
		t.Fatalf("agent context = %#v %#v err=%v", context, run, err)
	}
	item, _, err = s.CreateInvestigation("repo", item.ID, "attempt-2", "bbbbbbbb", "maintainer")
	if err != nil || !item.Investigations[0].Entries[0].Stale || !item.Investigations[0].Entries[1].Stale {
		t.Fatalf("old evidence not stale: %#v err=%v", item.Investigations[0], err)
	}
}
