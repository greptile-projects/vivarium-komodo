package accessibilitypolicies

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

func TestExactCandidateEvidenceDissentAndFollowUpOverride(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Create("repo", "owner", PolicyInput{Name: "Checkout access", CommitmentID: "commitment", CommitmentVersion: 2, TargetBranches: []string{"main"}, Paths: []string{"web/**"}, RequiredChecks: []string{"a11y-checkout"}, Scenarios: []ScenarioRequirement{{ScenarioID: "checkout", RequiredEvaluations: []string{"keyboard"}, RequiredRoles: []string{"screen-reader-participant"}}}})
	if err != nil {
		t.Fatal(err)
	}
	evidence := Evidence{Runs: []checkruns.Run{{CommitID: "old", Definition: checkruns.Definition{Name: "a11y-checkout"}, State: checkruns.Succeeded}}, Assessments: []accessibilityassessments.Assessment{{Input: accessibilityassessments.Input{Revision: "candidate", CommitmentID: "commitment", CommitmentVersion: 2, Scenarios: []accessibilityassessments.Scenario{{ID: "checkout", Evaluations: []string{"keyboard"}}}}, Automation: []accessibilityassessments.Automation{{ScenarioIDs: []string{"checkout"}, Evaluations: []string{"keyboard"}, Status: "succeeded"}}}}}
	a, err := s.Assess("repo", "pull", "candidate", "main", []string{"web/checkout.tsx"}, nil, nil, evidence)
	if err != nil || a.Ready {
		t.Fatalf("stale automation and missing participant must block: %#v %v", a, err)
	}
	evidence.Runs[0].CommitID = "candidate"
	rejected, err := s.Acknowledge("repo", "pull", p.ID, "preview", "candidate", "checkout", "screen-reader-participant", "rejected", "Focus escapes the dialog.", "participant")
	if err != nil {
		t.Fatal(err)
	}
	a, _ = s.Assess("repo", "pull", "candidate", "main", []string{"web/checkout.tsx"}, nil, nil, evidence)
	if a.Ready {
		t.Fatal("participant dissent must remain blocking")
	}
	o, err := s.Override("repo", "pull", rejected.ID, "owner", "The affected launch is time bounded and the barrier is documented.", FollowUp{Kind: "task", ResourceID: "task-repair", Summary: "Repair focus before general availability"})
	if err != nil || o.FollowUp.ResourceID != "task-repair" {
		t.Fatalf("override must retain follow-up work: %#v %v", o, err)
	}
	a, _ = s.Assess("repo", "pull", "candidate", "main", []string{"web/checkout.tsx"}, nil, nil, evidence)
	if !a.Ready {
		t.Fatalf("current evidence plus governed override should be ready: %#v", a.Requirements)
	}
	stale, _ := s.Assess("repo", "pull", "new-candidate", "main", []string{"web/checkout.tsx"}, nil, nil, evidence)
	if stale.Ready {
		t.Fatal("old audit and acknowledgement must not satisfy a new candidate")
	}
}
