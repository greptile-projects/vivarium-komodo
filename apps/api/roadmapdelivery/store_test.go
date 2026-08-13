package roadmapdelivery

import "testing"

func TestDeliveryDistinguishesShippingFromAchievedValue(t *testing.T) {
	in := Input{OutcomeID: "outcome-1", BaseRevision: "abc", Tasks: []Task{
		{Title: "instrument", OwnerKind: "agent", OwnerID: "agent-1", AcceptanceCriteria: []string{"signal is trustworthy"}, EvidenceIDs: []string{"feedback-1"}, SuccessMeasures: []string{"activation"}},
		{Title: "ship", OwnerKind: "human", OwnerID: "user-1", AcceptanceCriteria: []string{"need is satisfied"}, EvidenceIDs: []string{"validation-1"}, SuccessMeasures: []string{"retention"}, DependsOn: []int{1}},
	}}
	if !Validate(in, []string{"activation", "retention"}) {
		t.Fatal("expected evidence-covered ordered plan")
	}
	if Validate(in, []string{"activation", "retention", "guardrail"}) {
		t.Fatal("uncovered measure must reject promotion")
	}
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create("repo", "roadmap", 2, "outcome-1", "opp", 3, "proposal", "owner", "abc", []string{"task-1", "task-2"}, []string{"activation", "retention"}, []string{"feedback-1", "validation-1"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = s.Report("repo", v.ID, "owner", Link{Kind: "release", ResourceID: "release-1", Revision: "def", State: "published"})
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "delivered_not_achieved" {
		t.Fatalf("shipped work is not value: %#v", v)
	}
	v, err = s.Report("repo", v.ID, "researcher", Link{Kind: "experiment", ResourceID: "experiment-1", Revision: "run-1", State: "completed", MeasureResults: map[string]string{"activation": "passed", "retention": "failed"}, UnresolvedNeeds: []string{"keyboard users still abandon"}})
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "delivered_not_achieved" || len(v.Blockers) < 2 {
		t.Fatalf("failed evidence must block: %#v", v)
	}
	v, err = s.Revisit("repo", v.ID, "owner", "retention failed for an unresolved audience", []string{"experiment-1"})
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "revisit_required" {
		t.Fatalf("expected explicit revisit: %#v", v)
	}
}
