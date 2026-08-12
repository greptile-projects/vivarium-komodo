package federatedagents

import (
	"testing"
	"time"
)

func TestSessionKeepsLocalControlAndBoundedPublication(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.Create(CreateParams{RepositoryID: "fork", InitiatorID: "owner", Agent: "codex", Instructions: "Investigate", CredentialGrantID: "grant", CredentialExpiresAt: time.Now().Add(time.Hour), Context: Context{TargetPullReference: "pull-request:upstream", SourcePullReference: "pull-request:source", Revision: "base", Branch: "change", Paths: []string{"src/"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Event("fork", v.ID, "finding", map[string]string{"summary": "cause"}); err != nil {
		t.Fatal(err)
	}
	v, err = s.Publish("fork", v.ID, Publication{Summary: "Fixed it", Commands: []string{"go test ./..."}, Evidence: []string{"tests pass"}, Costs: map[string]string{"tokens": "1200"}, ResidualConcerns: []string{"platform matrix"}, CommitIDs: []string{"next"}, ChangedFiles: []string{"src/fix.go"}, SourceCommitID: "next"})
	if err != nil || v.Publication == nil || v.Publication.Costs["tokens"] != "1200" {
		t.Fatalf("publication = %#v, %v", v, err)
	}
	v, err = s.Revoke("fork", v.ID, time.Now())
	if err != nil || v.CredentialRevokedAt == nil || v.State != "published" {
		t.Fatalf("revocation = %#v, %v", v, err)
	}
}

func TestRevokedSessionCannotPublish(t *testing.T) {
	s, _ := New(t.TempDir())
	v, _ := s.Create(CreateParams{RepositoryID: "fork", InitiatorID: "owner", Agent: "codex", Instructions: "Repair", CredentialGrantID: "grant", CredentialExpiresAt: time.Now().Add(time.Hour), Context: Context{TargetPullReference: "target", SourcePullReference: "source", Revision: "base", Branch: "change"}})
	_, _ = s.Revoke("fork", v.ID, time.Now())
	if _, err := s.Publish("fork", v.ID, Publication{Summary: "x", CommitIDs: []string{"next"}, SourceCommitID: "next"}); err != ErrInvalid {
		t.Fatalf("publish err = %v", err)
	}
}
