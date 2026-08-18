package infrastructureplans

import (
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructurestate"
)

type fakePull struct{ revision string }

func (f *fakePull) CurrentRevision(string, string) (string, error) { return f.revision, nil }

type fakeDefinitions struct {
	value infrastructurestate.Definition
}

func (f *fakeDefinitions) Get(string, string) (infrastructurestate.Definition, error) {
	return f.value, nil
}

func TestPlanOrdersChangesAndInvalidatesCollaboration(t *testing.T) {
	rev := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	pulls := &fakePull{revision: rev}
	defs := &fakeDefinitions{value: infrastructurestate.Definition{ID: "inventory", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}, Observations: []infrastructurestate.Observation{{ID: "observation-1", ObservationInput: infrastructurestate.ObservationInput{EnvironmentID: "production", ObservedAt: time.Now()}}}}}
	s, err := New(t.TempDir(), pulls, defs)
	if err != nil {
		t.Fatal(err)
	}
	in := Input{Revision: rev, Definitions: []DefinitionRef{{ID: "inventory", Version: 1, ObservationIDs: []string{"observation-1"}}}, Changes: []Change{
		{ResourceID: "api", Action: "replace", EnvironmentIDs: []string{"production"}, DependsOn: []string{"database"}, OwnerIDs: []string{"service-owner"}, Summary: "replace runtime", RollbackLimit: "traffic rollback ends after contract", Risks: []Risk{{Kind: "availability", Level: "high", Detail: "restart"}, {Kind: "security", Level: "medium", Detail: "identity changes"}, {Kind: "privacy", Level: "medium", Detail: "processing boundary"}, {Kind: "continuity", Level: "high", Detail: "recovery window"}, {Kind: "cost", Level: "low", Detail: "overlap"}, {Kind: "data", Level: "critical", Detail: "schema compatibility"}}},
		{ResourceID: "database", Action: "change", EnvironmentIDs: []string{"production"}, OwnerIDs: []string{"data-owner"}, Summary: "expand storage", RollbackLimit: "allocated storage cannot shrink"},
	}, PolicyEffects: []PolicyEffect{{PolicyID: "security-policy", Revision: "v3", Effect: "exception_required", Detail: "replacement overlaps identities"}}, Assumptions: []string{"provider capacity remains available"}, RollbackLimits: []string{"no destructive action is authorized"}}
	p, err := s.Create("repo", "pull", "author", in)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DependencyOrder) != 2 || p.DependencyOrder[0] != "database" || p.Stale {
		t.Fatalf("unexpected plan: %+v", p)
	}
	p, err = s.Annotate("repo", "pull", p.ID, "reader", "investigation", "confirmed regional capacity", "evidence:capacity", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.Request("repo", "pull", p.ID, "reader", "service-owner", []string{"api"})
	if err != nil {
		t.Fatal(err)
	}
	ack := p.Acknowledgements[0].ID
	p, err = s.Decide("repo", "pull", p.ID, ack, "service-owner", "acknowledged", "impact understood")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Acknowledgements[0].Current || len(p.Annotations) != 1 {
		t.Fatalf("collaboration missing: %+v", p)
	}
	pulls.revision = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	p, err = s.Get("repo", "pull", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Stale || p.Acknowledgements[0].Current || p.StaleReasons[0] != "source_revision_changed" {
		t.Fatalf("source change silently reused plan: %+v", p)
	}
}

func TestPlanRejectsCyclesAndUnboundObservations(t *testing.T) {
	p := &fakePull{revision: "r"}
	d := &fakeDefinitions{value: infrastructurestate.Definition{ID: "d", CurrentVersion: 1, Versions: []infrastructurestate.Version{{Number: 1}}}}
	s, _ := New(t.TempDir(), p, d)
	_, err := s.Create("repo", "pull", "actor", Input{Revision: "r", Definitions: []DefinitionRef{{ID: "d", Version: 1, ObservationIDs: []string{"missing"}}}, Changes: []Change{{ResourceID: "a", Action: "create", EnvironmentIDs: []string{"test"}, DependsOn: []string{"b"}, Summary: "a", RollbackLimit: "a"}, {ResourceID: "b", Action: "create", EnvironmentIDs: []string{"test"}, DependsOn: []string{"a"}, Summary: "b", RollbackLimit: "b"}}})
	if err != ErrInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
}
