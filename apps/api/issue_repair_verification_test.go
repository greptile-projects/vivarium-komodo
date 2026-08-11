package main

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
)

func TestRepairVerificationRequiresExactReproductionAndChecks(t *testing.T) {
	input := issues.ReproductionInput{Name: "case.txt", SHA256: "fixture"}
	v := issues.RepairVerification{Revision: "candidate", CandidateDefinitionDigest: "definition", InputDigest: inputDigest([]issues.ReproductionInput{input}), RequiredChecks: []string{"unit"}, AcceptanceCriteria: []string{"reported request succeeds"}}
	attempt := issues.ReproductionAttempt{ID: "attempt", Revision: "candidate", DefinitionDigest: "definition", Inputs: []issues.ReproductionInput{input}, State: "failed", Reproduced: false, FailureReason: "reproduction command failed"}
	runs := []checkruns.Run{{ID: "run", CommitID: "candidate", Definition: checkruns.Definition{Name: "unit"}, State: checkruns.Succeeded}}

	evidence := verificationEvidence(v, attempt, runs, "candidate")
	if evidence["state"] != "ready_for_reporter" || evidence["reproduction_fixed"] != true || evidence["required_checks_passed"] != true || evidence["evidence_digest"] == "" {
		t.Fatalf("ready evidence = %#v", evidence)
	}

	stale := verificationEvidence(v, attempt, runs, "new-candidate")
	if stale["state"] != "invalid" || stale["stale"] != true {
		t.Fatalf("moved revision evidence = %#v", stale)
	}

	attempt.Inputs[0].SHA256 = "changed"
	changed := verificationEvidence(v, attempt, runs, "candidate")
	if changed["state"] != "invalid" || changed["stale"] != true {
		t.Fatalf("changed input evidence = %#v", changed)
	}
}
