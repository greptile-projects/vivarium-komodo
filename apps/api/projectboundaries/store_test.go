package projectboundaries

import (
	"errors"
	"testing"
)

func manifest() Input {
	rs := []Resource{}
	for _, k := range requiredKinds {
		rs = append(rs, Resource{Kind: k, Mode: "create", Name: k, OwnerIDs: []string{"owner"}, Access: []Access{{"contributors", "read", "project default"}}, MonthlyCost: 1, Generated: []GeneratedContent{{Path: k + ".md", Template: "safe-v1", Source: "accepted alternative alt", ApprovedByIDs: []string{"owner"}}}, Policies: []Policy{{Kind: k, Source: "baseline-v1", Summary: "least privilege"}}})
	}
	return Input{IncubatorID: "inc", AlternativeID: "alt", Title: "Compiler", Visibility: "public", OwnerIDs: []string{"owner"}, Resources: rs, RecurringCostLimit: 20}
}
func TestAtomicActivationRollbackAndRetry(t *testing.T) {
	s, _ := New(t.TempDir())
	v, e := s.Create("owner", manifest())
	if e != nil || len(v.MissingKinds) > 0 || v.TotalMonthlyCost != 13 {
		t.Fatalf("preview: %#v %v", v, e)
	}
	if _, e = s.Activate(v.ID, "owner", 1); !errors.Is(e, ErrConflict) {
		t.Fatalf("unapproved activation: %v", e)
	}
	v, e = s.Decide(v.ID, "owner", "approved", "direction and cost accepted", 1)
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.Activate(v.ID, "owner", 1)
	if e != nil || v.State != "active" || len(v.Attempts) != 1 {
		t.Fatalf("activation: %#v %v", v, e)
	}
	for _, r := range v.Resources {
		if r.State != "active" || r.Handle == "" {
			t.Fatalf("orphan: %#v", r)
		}
	}
	v, e = s.Rollback(v.ID, "owner", "return names for a corrected manifest", 1)
	if e != nil || v.State != "rolled_back" {
		t.Fatalf("rollback: %#v %v", v, e)
	}
	for _, r := range v.Resources {
		if r.Handle != "" {
			t.Fatalf("name retained: %#v", r)
		}
	}
	v, e = s.Activate(v.ID, "owner", 1)
	if e != nil || v.State != "active" || len(v.Attempts) != 3 {
		t.Fatalf("retry: %#v %v", v, e)
	}
}
func TestPreviewRequiresEveryBoundaryAndAllOwners(t *testing.T) {
	in := manifest()
	in.OwnerIDs = []string{"owner", "privacy-owner"}
	in.Resources = in.Resources[:len(in.Resources)-1]
	s, _ := New(t.TempDir())
	v, _ := s.Create("owner", in)
	if len(v.MissingKinds) != 1 || len(v.ActivationBlockers) < 2 {
		t.Fatalf("missing blockers: %#v", v)
	}
	if _, e := s.Get(v.ID, "stranger", true); e != nil {
		t.Fatalf("public preview unavailable: %v", e)
	}
}
