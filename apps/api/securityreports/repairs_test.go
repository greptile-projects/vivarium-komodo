package securityreports

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEmbargoedRepairsRetainCrossRepositoryWorkAndPrivateReview(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report, err := store.Create(CreateInput{
		ActorID: "reporter", Title: "private issue", Summary: "details",
		Contact: Contact{Channel: "email", Value: "safe@example.test"},
		Affected: []AffectedRepository{
			{RepositoryID: "repo-a", Versions: []string{"1.x"}},
			{RepositoryID: "repo-b", Versions: []string{"2.x"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	allow := func(string) bool { return true }
	firstReport, first, err := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo-a", Version: "1.x", Outcome: "remove unsafe parser path", BaseRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Branch: "refs/heads/embargo/opaque-a"}, allow)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo-b", Version: "2.x", Outcome: "update dependent binding", BaseRevision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Branch: "refs/heads/embargo/opaque-b", DependencyIDs: []string{first.ID}}, allow)
	if err != nil || second.DependencyIDs[0] != first.ID {
		t.Fatalf("dependency=%#v err=%v", second, err)
	}
	updated, session, err := store.StartRepairSession(report.ID, first.ID, "owner", "agent", "codex", "repair only the captured line", "credential-name", time.Now().Add(time.Hour), allow)
	if err != nil || session.State != "active" || len(updated.Repairs) != 2 {
		t.Fatalf("session=%#v err=%v", session, err)
	}
	updated, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "agent:codex", "branch_update", "bounded fix published", "cccccccccccccccccccccccccccccccccccccccc", "", allow)
	if err != nil {
		t.Fatal(err)
	}
	updated, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "reviewer", "review", "validated exact repair", "cccccccccccccccccccccccccccccccccccccccc", "approve", allow)
	if err != nil {
		t.Fatal(err)
	}
	if records := updated.Repairs[0].Sessions[0].Records; len(records) != 2 || records[1].Decision != "approve" || records[0].ActorID != "agent:codex" {
		t.Fatalf("records=%#v", records)
	}
	_, revoked, err := store.RevokeRepairSession(report.ID, first.ID, session.ID, "owner", allow)
	if err != nil || revoked.State != "revoked" {
		t.Fatalf("revoked=%#v err=%v", revoked, err)
	}
	if _, err = store.AddRepairRecord(report.ID, first.ID, session.ID, "agent:codex", "message", "late message", "", "", allow); !errors.Is(err, ErrTransition) {
		t.Fatalf("late record err=%v", err)
	}
	if len(firstReport.Audit) == 0 || first.Branch == "" {
		t.Fatal("repair did not retain private audit and isolated branch")
	}
}

func TestRepairVerificationRequiresExactCompleteEvidenceBeforeAttestation(t *testing.T) {
	store, _ := New(t.TempDir())
	report, _ := store.Create(CreateInput{ActorID: "reporter", Title: "issue", Summary: "details", Contact: Contact{Channel: "email", Value: "safe@example.test"}, Affected: []AffectedRepository{{RepositoryID: "repo", Versions: []string{"1.x"}}}})
	allow := func(string) bool { return true }
	_, repair, _ := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo", Version: "1.x", Outcome: "remove flaw", BaseRevision: strings.Repeat("a", 40), Branch: "refs/heads/embargo/opaque"}, allow)
	revision := strings.Repeat("b", 40)
	act := func(in VerificationAction) (Report, error) {
		in.ActorID, in.Revision = "owner", revision
		return store.UpdateRepairVerification(report.ID, repair.ID, in, allow)
	}
	if _, err := act(VerificationAction{Action: "begin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := act(VerificationAction{Action: "integrate", IntegrationEntryID: "queue-1", IntegrationCommitID: strings.Repeat("c", 40)}); !errors.Is(err, ErrTransition) {
		t.Fatalf("premature integration=%v", err)
	}
	if _, err := act(VerificationAction{Action: "gate", Kind: "required_check", Name: "line tests", AttemptID: "check-1", DefinitionDigest: strings.Repeat("1", 64), State: "passed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := act(VerificationAction{Action: "gate", Kind: "security_reproduction", Name: "CVE regression", AttemptID: "private-1", DefinitionDigest: strings.Repeat("2", 64), State: "passed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := act(VerificationAction{Action: "approve", Decision: "approve", Summary: "Reviewed the exact candidate and evidence."}); err != nil {
		t.Fatal(err)
	}
	updated, err := act(VerificationAction{Action: "integrate", IntegrationEntryID: "queue-1", IntegrationCommitID: strings.Repeat("c", 40)})
	if err != nil || updated.Repairs[0].Verification.State != "integrated" {
		t.Fatalf("integration=%#v err=%v", updated.Repairs[0].Verification, err)
	}
	updated, err = act(VerificationAction{Action: "attest", ReleaseID: "release-1", Version: "1.x", ArtifactID: "artifact-1", ArtifactSHA256: strings.Repeat("d", 64)})
	if err != nil || updated.Repairs[0].Verification.State != "attested" || len(updated.Repairs[0].Verification.ReleaseAttestations) != 1 {
		t.Fatalf("attestation=%#v err=%v", updated.Repairs[0].Verification, err)
	}
	data, _ := json.Marshal(updated)
	if strings.Contains(string(data), "command") || strings.Contains(string(data), "logs") {
		t.Fatalf("verification leaked sensitive execution material: %s", data)
	}
}

func TestDisclosurePublishesOnlyRedactedActionableAdvisory(t *testing.T) {
	store, _ := New(t.TempDir())
	report, _ := store.Create(CreateInput{ActorID: "reporter", Title: "unsafe parser", Summary: "secret reproduction details", Contact: Contact{Channel: "email", Value: "private@example.test"}, Affected: []AffectedRepository{{RepositoryID: "repo", Versions: []string{"1.x"}}}, Evidence: []Evidence{{Title: "exploit", Kind: "reproduction", Description: "secret payload"}}})
	allow := func(string) bool { return true }
	_, repair, _ := store.CreateRepair(report.ID, RepairInput{ActorID: "owner", RepositoryID: "repo", Version: "1.x", Outcome: "fix parser", BaseRevision: strings.Repeat("a", 40), Branch: "refs/heads/embargo/private"}, allow)
	revision := strings.Repeat("b", 40)
	act := func(action VerificationAction) {
		action.ActorID, action.Revision = "owner", revision
		if _, err := store.UpdateRepairVerification(report.ID, repair.ID, action, allow); err != nil {
			t.Fatal(err)
		}
	}
	act(VerificationAction{Action: "begin"})
	act(VerificationAction{Action: "gate", Kind: "required_check", Name: "tests", AttemptID: "check", DefinitionDigest: strings.Repeat("1", 64), State: "passed"})
	act(VerificationAction{Action: "gate", Kind: "security_reproduction", Name: "regression", AttemptID: "private", DefinitionDigest: strings.Repeat("2", 64), State: "passed"})
	act(VerificationAction{Action: "approve", Decision: "approve", Summary: "safe"})
	act(VerificationAction{Action: "integrate", IntegrationEntryID: "queue", IntegrationCommitID: strings.Repeat("c", 40)})
	act(VerificationAction{Action: "attest", ReleaseID: "release", Version: "1.x", ArtifactID: "artifact", ArtifactSHA256: strings.Repeat("d", 64)})
	prepared, err := store.PrepareDisclosure(report.ID, DisclosureInput{ActorID: "owner", AdvisoryID: "KSA-2026-001", Summary: "Parser validation vulnerability", UpgradeGuidance: "Upgrade to 1.2.3 immediately.", Credits: []string{"reporter"}, PublishedRefs: map[string]string{repair.ID: "refs/heads/security/ksa-2026-001-1.x"}}, allow)
	if err != nil || prepared.Disclosure.State != "ready" || len(prepared.Disclosure.Remaining) == 0 {
		t.Fatalf("prepared=%#v err=%v", prepared.Disclosure, err)
	}
	if _, err = store.GetPublicAdvisory("KSA-2026-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("advisory leaked before publication: %v", err)
	}
	published, err := store.CompleteDisclosure(report.ID, "owner", prepared.Disclosure.Branches, "", allow)
	if err != nil || published.EmbargoState != "lifted" {
		t.Fatal(err)
	}
	public, err := store.GetPublicAdvisory("KSA-2026-001")
	data, _ := json.Marshal(public)
	if err != nil || strings.Contains(string(data), "secret") || strings.Contains(string(data), "private@example") || !strings.Contains(string(data), "Upgrade to 1.2.3") {
		t.Fatalf("public=%s err=%v", data, err)
	}
}
