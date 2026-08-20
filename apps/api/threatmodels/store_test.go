package threatmodels

import "testing"

func sample() Input {
	return Input{
		Title: "Safer upload design", Summary: "Inspect upload parsing before implementation", Origin: Origin{Kind: "design_proposal", Reference: "design-7", Revision: "v3"},
		Inputs:        []InputBinding{{Kind: "architecture", Reference: "upload-flow", Revision: "a1"}, {Kind: "dependency", Reference: "image-parser", Revision: "2.0"}, {Kind: "trust_boundary", Reference: "public-to-worker", Revision: "b1"}},
		EntryPoints:   []EntryPoint{{ID: "upload", Description: "Public upload endpoint", Privileges: []string{"submit untrusted bytes"}, OwnerIDs: []string{"api-owner"}}},
		Dependencies:  []Dependency{{ID: "parser", Name: "image parser", Revision: "2.0", Trust: "Processes attacker-controlled bytes", OwnerIDs: []string{"media-owner"}}},
		DataFlows:     []DataFlow{{ID: "bytes", From: "browser", To: "worker", Data: []string{"image bytes"}, Boundary: "public-to-worker", DependencyIDs: []string{"parser"}}},
		AttackerGoals: []AttackerGoal{{ID: "exec", Actor: "anonymous user", Goal: "execute code in worker", Capability: "craft image bytes", Impact: "service compromise"}},
		Mitigations:   []Mitigation{{ID: "sandbox", Description: "Parse without service credentials", Status: "proposed", OwnerIDs: []string{"media-owner"}}},
		AbusePaths:    []AbusePath{{ID: "parser-rce", GoalID: "exec", EntryPointIDs: []string{"upload"}, DataFlowIDs: []string{"bytes"}, DependencyIDs: []string{"parser"}, Steps: []string{"upload malformed image", "trigger parser flaw"}, MitigationIDs: []string{"sandbox"}, ResidualRisk: "kernel escape remains possible", Severity: "critical", OwnerIDs: []string{"media-owner"}}},
		Alternatives:  []Alternative{{ID: "client-parse", Description: "Parse in the client", SecurityEffect: "Removes server parser attack surface", AbusePathIDs: []string{"parser-rce"}, Evidence: []Citation{{Kind: "design", Reference: "design-7", Revision: "v3", Detail: "alternative frame", Visibility: "repository"}}}},
		OwnerIDs:      []string{"security-owner"}, ResidualRisk: "Novel parser and sandbox escapes remain possible",
	}
}

func TestCollaborationStalenessAndRestrictedEvidence(t *testing.T) {
	s, _ := New(t.TempDir())
	m, e := s.Create("repo", "author", sample())
	if e != nil {
		t.Fatal(e)
	}
	f := FindingInput{Kind: "challenge", Body: "Sandbox still shares a kernel", AbusePathIDs: []string{"parser-rce"}, Citations: []Citation{{Kind: "code", Reference: "worker", Revision: "abc", Path: "worker.go", Detail: "namespace setup", Visibility: "repository"}}}
	if m, e = s.AddFinding("repo", m.ID, "read-only-agent", f); e != nil || len(m.Findings) != 1 || m.Findings[0].AuthorID != "read-only-agent" {
		t.Fatalf("finding lost: %#v %v", m, e)
	}
	f.Citations[0].Visibility = "restricted"
	if _, e = s.AddFinding("repo", m.ID, "agent", f); e != ErrInvalid {
		t.Fatalf("restricted evidence accepted: %v", e)
	}
	if _, e = s.Acknowledge("repo", m.ID, "not-owner", "acknowledge", "looks safe", "v3"); e != ErrInvalid {
		t.Fatalf("non-owner acknowledged: %v", e)
	}
	m, e = s.Acknowledge("repo", m.ID, "security-owner", "request_changes", "choose client parsing", "v3")
	if e != nil {
		t.Fatal(e)
	}
	Derive(&m, map[string]string{"dependency:image-parser": "2.1"})
	if !m.Stale || len(m.StaleInputs) != 1 || !m.Acknowledgements[0].Stale {
		t.Fatalf("change was hidden: %#v", m)
	}
}
