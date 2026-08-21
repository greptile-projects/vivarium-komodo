package projectincubators

import (
	"errors"
	"testing"
)

func base() Input {
	return Input{Title: "Shared compiler", Audience: "Small language teams", Problem: "Build tooling is fragmented", DesiredOutcome: "One approachable toolchain", Constraints: []string{"offline"}, SuccessMeasures: []string{"first build under five minutes"}, SponsorIDs: []string{"u_sponsor"}, DecisionRights: []string{"sponsors decide scope; participants decide alternatives"}, Visibility: "participants"}
}

func TestIncubatorRetainsAttributionConsentAndScope(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Create("u_founder", base(), Source{Kind: "feedback", RepositoryID: "repo", ResourceID: "fb", Status: "inaccessible", Detail: "not readable"})
	if e != nil {
		t.Fatal(e)
	}
	if v.AuthorityGranted || v.Source.Status != "inaccessible" || len(v.Participants) != 1 || v.Participants[0].Consent != "accepted" {
		t.Fatalf("bad creation projection: %#v", v)
	}
	v, e = s.Invite(v.ID, "u_founder", Participant{Kind: "human", UserID: "u_guest", Role: "affected developer"})
	if e != nil {
		t.Fatal(e)
	}
	pid := v.Participants[1].ID
	if _, e = s.Comment(v.ID, "u_guest", "before consent"); !errors.Is(e, ErrForbidden) {
		t.Fatalf("pending participant shaped: %v", e)
	}
	v, e = s.Consent(v.ID, pid, "u_guest", "accepted")
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.Comment(v.ID, "u_guest", "The offline constraint matters.")
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.AddEvidence(v.ID, "u_guest", Evidence{Kind: "interview", Reference: "public:study", Summary: "Three teams reported this", Visibility: "participants"})
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.AddAssumption(v.ID, "u_guest", "Teams can standardize on one runtime")
	if e != nil {
		t.Fatal(e)
	}
	aid := v.Assumptions[0].ID
	v, e = s.ResolveAssumption(v.ID, aid, "u_founder", "disproved")
	if e != nil {
		t.Fatal(e)
	}
	next := base()
	next.DesiredOutcome = "A common interface over several runtimes"
	v, e = s.ChangeScope(v.ID, "u_founder", "Evidence disproved a single-runtime direction", next)
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Discussion) != 1 || len(v.Evidence) != 1 || v.Assumptions[0].Status != "disproved" || len(v.ScopeChanges) != 1 || v.History[len(v.History)-1].ActorID != "u_founder" {
		t.Fatalf("attribution missing: %#v", v)
	}
}

func TestDuplicateAndPrivateVisibility(t *testing.T) {
	s, _ := New(t.TempDir())
	a, _ := s.Create("u_one", base(), Source{Kind: "idea", Status: "accessible"})
	b, _ := s.Create("u_two", base(), Source{Kind: "idea", Status: "accessible"})
	if len(b.DuplicateIDs) != 1 || b.DuplicateIDs[0] != a.ID {
		t.Fatalf("duplicate not reported: %#v", b.DuplicateIDs)
	}
	a, _ = s.Get(a.ID, "u_one")
	if len(a.DuplicateIDs) != 1 || a.DuplicateIDs[0] != b.ID {
		t.Fatalf("duplicate not symmetric: %#v", a.DuplicateIDs)
	}
	if _, e := s.Get(a.ID, "stranger"); !errors.Is(e, ErrNotFound) {
		t.Fatalf("private incubator leaked: %v", e)
	}
}

func TestAlternativesResearchAndReproducibleExperiments(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create("u_founder", base(), Source{Kind: "idea", Status: "accessible"})
	a := Alternative{Title: "Adopt a parser core", ProductBoundary: "CLI and stable library API", Architecture: "thin adapters around an adopted parser", Interfaces: []string{"CLI", "Go API"}, Dependencies: []string{"parser.example v2"}, Licenses: []string{"Apache-2.0"}, OperatingCosts: []string{"one shared runner"}, SecurityRisks: []string{"untrusted grammar input"}, DataRisks: []string{"source remains local"}, BuildOrAdopt: "adopt parser; build product workflow", Unknowns: []string{"incremental parse latency"}, CapabilityLinks: []CapabilityLink{{Kind: "package", ResourceID: "parser.example", Revision: "v2", Visibility: "public"}, {Kind: "api", ScopeID: "org_tools", ResourceID: "parser-contract", Revision: "3", Visibility: "organization"}}}
	v, e := s.AddAlternative(v.ID, "u_founder", a)
	if e != nil {
		t.Fatal(e)
	}
	alt := v.Alternatives[0]
	v, e = s.AddFinding(v.ID, "u_founder", Finding{AlternativeID: alt.ID, Kind: "dissent", Claim: "The dependency may constrain error recovery", Evidence: []Evidence{{Kind: "package", Reference: "https://example.test/issues/12", Summary: "Open recovery limitation", Visibility: "public"}}})
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.AddExperiment(v.ID, "u_founder", Experiment{AlternativeID: alt.ID, Question: "Can it meet interactive latency?", Method: []string{"run fixed synthetic corpus"}, Inputs: []string{"corpus digest sha256:abc"}, Commands: []string{"bench --corpus fixture"}, SuccessCriteria: []string{"p95 under 50ms"}, Budget: "10 minutes, USD 1", SafetyBoundary: "networkless synthetic workspace"})
	if e != nil {
		t.Fatal(e)
	}
	exp := v.Experiments[0]
	v, e = s.AddAttempt(v.ID, exp.ID, "u_founder", ExperimentAttempt{InputDigest: "sha256:abc", Measurements: map[string]string{"p95": "42ms"}, Artifacts: []string{"sha256:result"}, Outcome: "passed", Notes: "isolated run"})
	if e != nil {
		t.Fatal(e)
	}
	first := v.Experiments[0].Attempts[0]
	v, e = s.AddAttempt(v.ID, exp.ID, "u_founder", ExperimentAttempt{InputDigest: "sha256:abc", Measurements: map[string]string{"p95": "44ms"}, Artifacts: []string{"sha256:reproduction"}, Outcome: "passed", ReproductionOfID: first.ID})
	if e != nil {
		t.Fatal(e)
	}
	if v.AuthorityGranted || v.Experiments[0].AuthorityGranted || len(v.Findings) != 1 || len(v.Experiments[0].Attempts) != 2 {
		t.Fatalf("learning record lost containment: %#v", v)
	}
}
