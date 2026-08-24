package historyremediations

import (
	"errors"
	"testing"
)

func TestRestrictedHistoryRemediationScope(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Title: "Remove exposed credential from history", Source: Source{Kind: "security_finding", ID: "finding-1", Revision: "v3"}, ContentDescription: "A credential-shaped value in the named blob; payload intentionally omitted.", Reason: "Prevent continued retrieval from repository history.", Audience: "response_team", ResponseOwnerIDs: []string{"responder"}, ParticipantIDs: []string{"legal"}, Objects: []Object{{ID: "object-1", RepositoryID: "repo", Kind: "blob", ObjectID: "abc123", Digest: "sha256:safe", Match: "confirmed", AttributedTo: "scanner"}, {ID: "object-2", RepositoryID: "repo", Kind: "blob", ObjectID: "def456", Match: "false_match", Reason: "synthetic fixture", AttributedTo: "privacy"}}, Scope: []Scope{{Kind: "repository", RepositoryID: "repo", Reference: "repo"}, {Kind: "ref", RepositoryID: "repo", Reference: "refs/heads/main", Revision: "deadbeef"}, {Kind: "release", RepositoryID: "repo", Reference: "release-1"}, {Kind: "package", Reference: "package-1"}, {Kind: "artifact", Reference: "artifact-1"}, {Kind: "environment", Reference: "production"}}, Evidence: []Evidence{{ID: "evidence-1", Kind: "object_scan", Reference: "scan-1", Revision: "rules:v2", Digest: "sha256:evidence", Summary: "Scanner matched the exact blob object ID; no matched bytes retained.", Status: "available", RecordedBy: "scanner"}, {ID: "evidence-2", Kind: "release_inventory", Reference: "inventory-1", Digest: "sha256:inventory", Summary: "Restricted release inventory cannot be projected.", Status: "inaccessible", Reason: "release owner access required", RecordedBy: "release-owner"}}, Constraints: []Constraint{{ID: "hold-1", Kind: "legal_hold", Reference: "case-9", Status: "conflict", OwnerID: "legal", Rationale: "Preservation direction may cover this object."}, {ID: "continuity-1", Kind: "continuity", Reference: "objective-2", Status: "conflict", OwnerID: "sre", Rationale: "Recovery copy retains the affected release."}}, Approvals: []Approval{{Kind: "repository_owner", OwnerID: "owner", Required: true, Status: "pending"}, {Kind: "legal_owner", OwnerID: "legal", Required: true, Status: "pending"}}}
	x, e := s.Create("repo", "owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Blockers) != 6 {
		t.Fatalf("blockers=%#v", x.Blockers)
	}
	if _, e = s.Get("repo", x.ID, "stranger"); e != ErrNotFound {
		t.Fatalf("private read=%v", e)
	}
	if got, e := s.Get("repo", x.ID, "responder"); e != nil || got.Input.Objects[0].ObjectID != "abc123" {
		t.Fatalf("response read=%#v %v", got, e)
	}
	if c, e := s.Catalog("repo", "legal"); e != nil || len(c.Items) != 1 {
		t.Fatalf("approval owner catalog=%#v %v", c, e)
	}
}

func TestReachabilityMapRetainsStatusesObjectsAndDerivedExposure(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Title: "Repair", Source: Source{Kind: "selected_object", ID: "object:1"}, ContentDescription: "Unsafe bytes omitted.", Reason: "Contain copies.", Audience: "owners_only", ResponseOwnerIDs: []string{"owner"}, Objects: []Object{{ID: "object-1", RepositoryID: "repo", Kind: "blob", ObjectID: "deadbeef", Match: "confirmed", AttributedTo: "owner"}}, Scope: []Scope{{Kind: "repository", Reference: "repo"}}, Evidence: []Evidence{{ID: "e", Kind: "scan", Reference: "scan:1", Digest: "sha256:scan", Summary: "Object ID matched; bytes omitted.", Status: "available", RecordedBy: "owner"}}, Approvals: []Approval{{Kind: "repository_owner", OwnerID: "owner", Required: true, Status: "pending"}}}
	x, _ := s.Create("repo", "owner", in)
	add := ReachabilityInput{CopyKind: "active_clone", Reference: "clone:developer-7", ObjectIDs: []string{"deadbeef"}, DerivedExposures: []DerivedExposure{{Kind: "credential", Reference: "credential:deploy-key", State: "rotated"}}, Status: "unverifiable", Summary: "Clone was active after exposure discovery; contents were not inspected.", Uncertainty: "Owner has not acknowledged migration.", Citations: []Citation{{Kind: "clone_activity", Reference: "event:7", Digest: "sha256:activity", Access: "restricted"}}}
	x, err := s.AddReachability("repo", x.ID, "owner", add)
	if err != nil || len(x.Reachability) != 1 || x.ReachabilitySummary.ByStatus["unverifiable"] != 1 || x.ReachabilitySummary.DerivedExposureCount != 1 {
		t.Fatalf("map=%+v err=%v", x, err)
	}
	if _, err = s.AddReachability("repo", x.ID, "stranger", add); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger err=%v", err)
	}
}

func TestRejectsSensitivePayloadShapedText(t *testing.T) {
	s, _ := New(t.TempDir())
	_, e := s.Create("repo", "owner", Input{Title: "bad", Source: Source{Kind: "selected_object", ID: "x"}, ContentDescription: "-----BEGIN PRIVATE KEY-----", Reason: "remove", Audience: "owners_only", ResponseOwnerIDs: []string{"owner"}, Objects: []Object{{ID: "o", RepositoryID: "repo", Kind: "blob", ObjectID: "x", Match: "confirmed", AttributedTo: "owner"}}, Scope: []Scope{{Kind: "repository", Reference: "repo"}}, Evidence: []Evidence{{ID: "e", Kind: "scan", Reference: "x", Digest: "sha256:x", Summary: "safe", Status: "available", RecordedBy: "owner"}}, Approvals: []Approval{{Kind: "repository_owner", OwnerID: "owner", Required: true, Status: "pending"}}})
	if e != ErrInvalid {
		t.Fatalf("error=%v", e)
	}
}
