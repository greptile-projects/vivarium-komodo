package assuranceassessments

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
)

func fixture(t *testing.T) (*Store, *assuranceprograms.Store, assuranceprograms.Program, Input) {
	t.Helper()
	ps, _ := assuranceprograms.New(t.TempDir())
	definition := assuranceprograms.Input{
		Name: "Delivery", Description: "regulated delivery", Scope: "production", ChangeReason: "initial",
		Requirements: []assuranceprograms.Requirement{{ID: "reg", SourceKind: "regulatory", SourceReference: "law", SourceVersion: "1", Title: "Retention", Text: "retain records", Applicability: "service", Interpretation: "release records", AuthorID: "legal"}},
		Controls:     []assuranceprograms.Control{{ID: "retention", Objective: "Retain release records", Claim: "Every release is retained", ReviewPeriod: "release", RequirementIDs: []string{"reg"}, OwnerIDs: []string{"control-owner"}, Targets: []assuranceprograms.Target{{Kind: "release", Reference: "service", Revision: "base"}}, EvidenceCriteria: []assuranceprograms.EvidenceCriterion{{ID: "release", Kind: "release", Description: "release proof", Frequency: "each release", SourceReference: "releases"}}}},
	}
	p, e := ps.Create("repo", "writer", definition)
	if e != nil {
		t.Fatal(e)
	}
	s, _ := New(t.TempDir(), ps)
	in := Input{
		CandidateKind: "pull_request", CandidateID: "pull:7", CandidateRevision: "candidate-1", ProgramID: p.ID, ProgramVersion: 1, Summary: "retention policy changes",
		Inputs:  []BoundInput{{Key: "candidate", Revision: "candidate-1"}, {Key: "program", Revision: "1"}, {Key: "policy:retention", Revision: "v3"}, {Key: "dependency:archive", Revision: "4"}},
		Impacts: []Impact{{ControlID: "retention", RequirementIDs: []string{"reg"}, Rationale: "archive behavior changes", ChangedEvidence: []string{"package:old"}, RequiredOwnerIDs: []string{"control-owner"}, InputKeys: []string{"candidate", "program", "policy:retention"}, RequiredForReadiness: true, Actions: []Action{{ID: "test", Kind: "test", Description: "exercise retention", Required: true}, {ID: "evidence", Kind: "evidence", Description: "collect release proof", EvidencePackageIDs: []string{"package:new"}, Required: true}, {ID: "notice", Kind: "notice", Description: "notify customers"}, {ID: "retain", Kind: "retention", Description: "retain prior manifests"}, {ID: "exception", Kind: "exception", Description: "review exception"}}}},
	}
	return s, ps, p, in
}

func TestSelectiveInvalidationAndReadiness(t *testing.T) {
	s, ps, p, in := fixture(t)
	a, e := s.Create("repo", "author", in)
	if e != nil || a.Ready {
		t.Fatalf("unacknowledged assessment ready: %#v %v", a, e)
	}
	if _, e = s.Decide("repo", a.ID, "reader", "retention", "acknowledge", "looks good"); e != ErrInvalid {
		t.Fatalf("non-owner decided: %v", e)
	}
	a, e = s.Decide("repo", a.ID, "control-owner", "retention", "acknowledge", "current evidence and actions satisfy control")
	if e != nil || !a.Ready || a.AuthorityGranted {
		t.Fatalf("current acknowledgement did not govern readiness: %#v %v", a, e)
	}
	// A dependency outside this decision's declared keys changes without staling it.
	a, e = s.Rebind("repo", a.ID, "writer", "candidate-1", "candidate-1", []BoundInput{{Key: "candidate", Revision: "candidate-1"}, {Key: "program", Revision: "1"}, {Key: "policy:retention", Revision: "v3"}, {Key: "dependency:archive", Revision: "5"}})
	if e != nil || !a.Ready || a.Decisions[0].Stale {
		t.Fatalf("unrelated input invalidated decision: %#v %v", a, e)
	}
	// A relevant policy revision invalidates only the affected decision.
	a, e = s.Rebind("repo", a.ID, "writer", "candidate-1", "candidate-2", []BoundInput{{Key: "candidate", Revision: "candidate-2"}, {Key: "program", Revision: "1"}, {Key: "policy:retention", Revision: "v4"}, {Key: "dependency:archive", Revision: "5"}})
	if e != nil || a.Ready || !a.Decisions[0].Stale {
		t.Fatalf("relevant input retained stale assurance: %#v %v", a, e)
	}
	def := p.Versions[0].Input
	def.ChangeReason = "new interpretation"
	if _, e = ps.Revise("repo", p.ID, "writer", 1, def); e != nil {
		t.Fatal(e)
	}
	a, _ = s.Get("repo", a.ID)
	if len(a.Blockers) == 0 {
		t.Fatal("program revision did not keep assessment blocked")
	}
}

func TestCitedReaderCollaborationRejectsRestrictedEvidence(t *testing.T) {
	s, _, _, in := fixture(t)
	a, _ := s.Create("repo", "author", in)
	a, e := s.Annotate("repo", a.ID, "readonly-agent", AnnotationInput{Kind: "challenge", Body: "the notice audience is incomplete", ControlIDs: []string{"retention"}, Citations: []Citation{{Reference: "policy:notice", Revision: "v2", Audience: "repository"}}})
	if e != nil || len(a.Annotations) != 1 {
		t.Fatalf("cited challenge rejected: %#v %v", a, e)
	}
	if _, e = s.Annotate("repo", a.ID, "agent", AnnotationInput{Kind: "analysis", Body: "copied private audit", ControlIDs: []string{"retention"}, Citations: []Citation{{Reference: "private:audit", Revision: "1", Audience: "owners"}}}); e != ErrInvalid {
		t.Fatalf("restricted citation accepted: %v", e)
	}
}

func TestAllCandidateKindsAreAccepted(t *testing.T) {
	for _, kind := range []string{"pull_request", "infrastructure_plan", "schema_migration", "extension_installation", "package_update", "release_candidate"} {
		s, _, _, in := fixture(t)
		in.CandidateKind = kind
		if _, e := s.Create("repo", "author", in); e != nil {
			t.Errorf("%s: %v", kind, e)
		}
	}
}
