package federation

import (
	"testing"
	"time"
)

func TestMergeReceiptIsDurableAndIdempotent(t *testing.T) {
	s, err := New(t.TempDir(), Config{Instance: "https://home.example", Operators: []Operator{{Name: "op", Contact: "mailto:op@example"}}, Capabilities: []string{"repository.contribution_receipts"}, Endpoints: Endpoints{Discovery: "https://home.example/.well-known/komodo-federation"}})
	if err != nil {
		t.Fatal(err)
	}
	r := MergeReceipt{SchemaVersion: 1, IdempotencyKey: "merge:repo:pull", UpstreamInstance: "https://upstream.example", ContributorInstance: "https://home.example", SourceCommitID: "abc", MergeCommitID: "def", MergedAt: time.Now().UTC(), Signature: "signed"}
	first, err := s.PutMergeReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.PutMergeReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate receipt IDs differ: %s %s", first.ID, second.ID)
	}
	r.MergeCommitID = "changed"
	if _, err := s.PutMergeReceipt(r); err != ErrConflict {
		t.Fatalf("changed retry error = %v", err)
	}
	reopened, err := New(s.root, Config{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.MergeReceipt("merge:repo:pull")
	if err != nil || got.MergeCommitID != "def" {
		t.Fatalf("retained receipt = %#v, %v", got, err)
	}
}
