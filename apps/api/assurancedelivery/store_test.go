package assurancedelivery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"
)

type sourceStub struct{ finding FindingSource }

func (s sourceStub) Finding(repo, assessment, finding string) (FindingSource, error) {
	if repo != "repo" || assessment != s.finding.AssessmentID || finding != s.finding.FindingID {
		return FindingSource{}, ErrNotFound
	}
	return s.finding, nil
}

type signerStub struct{}

func (signerStub) Sign(b []byte) (string, string, error) {
	x := sha256.Sum256(b)
	return "key-1", hex.EncodeToString(x[:]), nil
}

func TestFindingDeliveryAndStatementLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s, err := New(t.TempDir(), sourceStub{FindingSource{AssessmentID: "assessment", FindingID: "finding", ControlID: "access-control", FindingBody: "review access", OwnerID: "owner", Scope: AssessmentScope{ProgramID: "program", ProgramVersion: 3, ControlIDs: []string{"access-control"}, Releases: []string{"release-7"}, EvidencePackageIDs: []string{"evidence-1"}, PeriodStart: now.Add(-24 * time.Hour), PeriodEnd: now}}}, signerStub{})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return now }
	r, err := s.CreateRemediation("repo", "assessment", "finding", "owner", RemediationInput{AffectedRevision: "revision-a", Deadline: now.Add(7 * 24 * time.Hour), EvidencePackageIDs: []string{"evidence-1"}, Work: []WorkInput{{Kind: "task", Title: "change access policy", OwnerKind: "agent", OwnerID: "agent", AcceptanceCriteria: []string{"test denies stale access"}}, {Kind: "policy_change", Title: "approve narrow policy", OwnerKind: "human", OwnerID: "owner", AcceptanceCriteria: []string{"policy is revision bound"}}}})
	if err != nil || r.AuthorityGranted || len(r.Work) != 2 {
		t.Fatalf("create: %#v %v", r, err)
	}
	if _, err = s.Progress("repo", r.ID, r.Work[1].ID, "owner", ProgressInput{Status: "completed", Summary: "premature"}); err != ErrConflict {
		t.Fatalf("ordered work accepted: %v", err)
	}
	r, err = s.Progress("repo", r.ID, r.Work[0].ID, "agent", ProgressInput{Status: "completed", Summary: "pull reviewed", ResourceID: "pull-1", Revision: "revision-a", EvidencePackageIDs: []string{"evidence-1"}})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Progress("repo", r.ID, r.Work[1].ID, "owner", ProgressInput{Status: "completed", Summary: "policy published", ResourceID: "policy-v2", Revision: "revision-a"})
	if err != nil {
		t.Fatal(err)
	}
	digest := hex.EncodeToString(make([]byte, 32))
	r, err = s.Verify("repo", r.ID, "owner", VerificationInput{AffectedRevision: "revision-a", EvidenceDigest: digest, EvidencePackageIDs: []string{"evidence-1"}, Criteria: map[string]bool{"test denies stale access": true, "policy is revision bound": true}, Summary: "current evidence passes"})
	if err != nil || r.Status != "verified" {
		t.Fatalf("verify: %s %v", r.Status, err)
	}
	r, err = s.Disposition("repo", r.ID, "assessor", "assessor", "accept", "repair addresses the sampled finding")
	if err != nil || r.Status != "closed" {
		t.Fatalf("close: %s %v", r.Status, err)
	}
	statement, err := s.Publish("repo", "owner", StatementInput{ProgramID: "program", ProgramVersion: 3, ReleaseID: "release-7", ReleaseRevision: "revision-a", Scope: "billing service", PeriodStart: now.Add(-24 * time.Hour), PeriodEnd: now, ExpiresAt: now.Add(30 * 24 * time.Hour), ControlIDs: []string{"access-control"}, ExceptionReferences: []string{"exception-2"}, EvidencePackageIDs: []string{"evidence-1"}, RemediationIDs: []string{r.ID}, Audience: "public", EvidenceDigest: digest})
	if err != nil || statement.Signature == "" || statement.PayloadDigest == "" || statement.Status != "current" || statement.AuthorityGranted {
		t.Fatalf("statement: %#v %v", statement, err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(statement.SignedPayload)
	if err != nil {
		t.Fatal(err)
	}
	signed := sha256.Sum256(payload)
	if statement.Signature != hex.EncodeToString(signed[:]) || statement.PayloadDigest != statement.Signature {
		t.Fatal("published payload is not independently verifiable")
	}
	r, err = s.Drift("repo", r.ID, "owner", "revision-b", "later policy change")
	if err != nil || r.Status != "reopened" {
		t.Fatalf("drift: %s %v", r.Status, err)
	}
	statement, err = s.Statement("repo", statement.ID, "public")
	if err != nil || statement.Status != "changed" || len(statement.StatusReasons) == 0 {
		t.Fatalf("derived status: %#v %v", statement, err)
	}
}

func TestRepositoryStatementIsNotPublic(t *testing.T) {
	now := time.Now().UTC()
	s, _ := New(t.TempDir(), sourceStub{FindingSource{AssessmentID: "a", FindingID: "f", ControlID: "c", OwnerID: "owner", Scope: AssessmentScope{ProgramID: "p", ProgramVersion: 1, EvidencePackageIDs: []string{"e"}}}}, signerStub{})
	s.now = func() time.Time { return now }
	v, err := s.Publish("repo", "owner", StatementInput{ProgramID: "p", ProgramVersion: 1, ReleaseID: "r", ReleaseRevision: "x", Scope: "service", PeriodStart: now.Add(-time.Hour), PeriodEnd: now, ExpiresAt: now.Add(time.Hour), ControlIDs: []string{"c"}, EvidencePackageIDs: []string{"e"}, Audience: "repository", EvidenceDigest: hex.EncodeToString(make([]byte, 32))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Statement("repo", v.ID, "public"); err != ErrForbidden {
		t.Fatalf("private statement exposed: %v", err)
	}
}
