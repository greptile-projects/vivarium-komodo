package main

import (
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/supportquestions"
)

func TestSupportVerificationProjectionExplainsAffectedStaleness(t *testing.T) {
	revision := supportquestions.AnswerRevision{ID: "answer-v2", Instructions: []string{"run fixed command"}}
	q := supportquestions.Question{SoftwareVersion: "2.0", Environment: "linux", Answers: []supportquestions.Answer{{ID: "answer", CurrentID: revision.ID, Revisions: []supportquestions.AnswerRevision{revision}}}}
	old := supportquestions.VerificationAttempt{ID: "old", AnswerID: "answer", AnswerRevisionID: revision.ID, Instructions: revision.Instructions, InstructionsDigest: supportInstructionsDigest(revision.Instructions), SourceRevision: "commit-a", SoftwareVersion: "1.0", Environment: supportquestions.VerificationEnvironment{Name: "linux", ImageDigest: "sha256:old"}, Dependencies: map[string]string{"sdk": "1"}, Inputs: []supportquestions.VerificationInput{{Name: "case.json", SHA256: "old"}}}
	latest := old
	latest.ID, latest.SourceRevision, latest.SoftwareVersion, latest.Environment.ImageDigest = "latest", "commit-b", "2.0", "sha256:new"
	latest.Dependencies = map[string]string{"sdk": "2"}
	latest.Inputs = []supportquestions.VerificationInput{{Name: "case.json", SHA256: "new"}}
	p := supportVerificationProjection(old, q, &latest)
	reasons := map[string]bool{}
	for _, reason := range p["stale_reasons"].([]string) {
		reasons[reason] = true
	}
	for _, want := range []string{"software_version_changed", "source_revision_changed", "environment_dependency_changed", "dependencies_changed", "inputs_changed"} {
		if !reasons[want] {
			t.Fatalf("missing %s in %#v", want, reasons)
		}
	}
}
