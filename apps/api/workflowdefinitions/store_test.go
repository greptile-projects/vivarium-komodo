package workflowdefinitions

import (
	"errors"
	"testing"
)

func definition() Input {
	return Input{Name: "Issue repair", Outcome: "A cited repair proposal is ready for human review", RepositoryRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", DefinitionPath: ".project/workflows/repair.json", Triggers: []Trigger{{ID: "accepted", Type: "repository_event", Event: "issue.accepted", Conditions: []string{"label=repair"}, InputMappings: map[string]string{"issue": "event.id"}}}, Inputs: []Field{{Name: "issue", Type: "string", Required: true}}, Steps: []Step{{ID: "draft", Name: "Draft repair", Conditions: []string{"issue is present"}, Inputs: map[string]string{"issue": "workflow.issue"}, Outputs: []Field{{Name: "proposal", Type: "string", Required: true}}, Invocation: Invocation{Kind: "approved_agent", Reference: "repair-agent", Revision: "v1", Accessible: true, OwnerIDs: []string{"owner"}, Capabilities: []string{"proposal:draft"}, Emits: []string{"proposal.drafted"}}, Retry: Retry{MaximumAttempts: 2, BackoffSeconds: 10}, TimeoutSeconds: 300, MaximumCost: 3, CompletionCriteria: []string{"proposal cites issue"}}}, Outputs: []Field{{Name: "proposal", Type: "string", Required: true}}, MaximumCost: 3, Currency: "USD", OwnerIDs: []string{"owner"}, CompletionCriteria: []string{"proposal exists"}, Policies: []Policy{{ID: "draft", Effect: "allow", Capability: "proposal:draft", OwnerIDs: []string{"owner"}}}, ChangeReason: "initial reviewed workflow"}
}

func TestVersionPreviewAndActivation(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	x, e := s.Create("repo", "owner", definition())
	if e != nil {
		t.Fatal(e)
	}
	if len(x.EventSubscriptions) != 1 || x.EventSubscriptions[0] != "issue.accepted" {
		t.Fatalf("subscriptions %#v", x.EventSubscriptions)
	}
	if x.EffectiveAuthority.GrantsAuthority || len(x.EffectiveAuthority.Capabilities) != 1 {
		t.Fatalf("authority %#v", x.EffectiveAuthority)
	}
	x, e = s.Activate("repo", x.ID, "owner", 1)
	if e != nil || x.State != "active" {
		t.Fatalf("activate: %#v %v", x, e)
	}
	in := definition()
	in.ChangeReason = "raise timeout"
	in.Steps[0].TimeoutSeconds = 600
	x, e = s.Revise("repo", x.ID, "owner", 1, in)
	if e != nil || x.State != "draft" || x.Activation != nil {
		t.Fatalf("revision must require activation: %#v %v", x, e)
	}
}

func TestDiagnosticsBlockActivation(t *testing.T) {
	s, _ := New(t.TempDir())
	in := definition()
	in.Steps[0].Needs = []string{"draft"}
	in.Steps[0].Invocation.Accessible = false
	in.Steps[0].Invocation.Emits = []string{"issue.accepted"}
	in.Policies = []Policy{{ID: "deny", Effect: "deny", Capability: "proposal:draft", OwnerIDs: []string{"security-owner"}}}
	x, e := s.Create("repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	kinds := map[string]bool{}
	for _, d := range x.Diagnostics {
		kinds[d.Kind] = true
	}
	for _, kind := range []string{"invalid_graph", "trigger_loop", "inaccessible_resource", "conflicting_policy"} {
		if !kinds[kind] {
			t.Fatalf("missing %s in %#v", kind, x.Diagnostics)
		}
	}
	_, e = s.Activate("repo", x.ID, "owner", 1)
	if !errors.Is(e, ErrBlocked) {
		t.Fatalf("expected blocker, got %v", e)
	}
}

func TestInvalidTypedDefinitionAndOwnerGate(t *testing.T) {
	s, _ := New(t.TempDir())
	in := definition()
	in.Inputs[0].Type = "secret"
	if _, e := s.Create("repo", "owner", in); !errors.Is(e, ErrInvalid) {
		t.Fatalf("expected invalid, got %v", e)
	}
	x, _ := s.Create("repo", "owner", definition())
	if _, e := s.Activate("repo", x.ID, "collaborator", 1); !errors.Is(e, ErrConflict) {
		t.Fatalf("expected owner gate, got %v", e)
	}
}
