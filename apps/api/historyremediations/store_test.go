package historyremediations

import (
	"errors"
	"testing"
	"time"
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

func TestImmutableRewriteCandidateAndBoundedRehearsal(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Title: "Repair", Source: Source{Kind: "selected_object", ID: "object:1"}, ContentDescription: "Unsafe bytes omitted.", Reason: "Contain copies.", Audience: "owners_only", ResponseOwnerIDs: []string{"responder"}, Objects: []Object{{ID: "object-1", RepositoryID: "repo", Kind: "blob", ObjectID: "badblob", Match: "confirmed", AttributedTo: "owner"}}, Scope: []Scope{{Kind: "repository", Reference: "repo"}, {Kind: "ref", Reference: "refs/heads/main", Revision: "oldtip"}}, Evidence: []Evidence{{ID: "e", Kind: "scan", Reference: "scan:1", Digest: "sha256:scan", Summary: "Object ID matched; bytes omitted.", Status: "available", RecordedBy: "owner"}}, Approvals: []Approval{{Kind: "repository_owner", OwnerID: "owner", Required: true, Status: "approved"}}}
	x, _ := s.Create("repo", "owner", in)
	x, e := s.AddRewriteRule("repo", x.ID, "responder", RewriteRuleInput{Kind: "replace_object", ObjectIDs: []string{"badblob"}, ReplacementDigest: "sha256:sanitized", PreserveAuthorship: true, PreserveTimestamps: true, SignaturePolicy: "preserve_if_unchanged", Rationale: "Replace the unsafe blob with a reviewed placeholder."})
	if e != nil || len(x.RewriteRules) != 1 {
		t.Fatalf("rule=%+v %v", x, e)
	}
	c := RewriteCandidateInput{RuleIDs: []string{x.RewriteRules[0].ID}, Refs: []RefReplacement{{Reference: "refs/heads/main", OldRevision: "oldtip", NewRevision: "newtip"}}, CommitMap: []CommitMapping{{OldCommit: "oldtip", NewCommit: "newtip", AuthorshipPreserved: true, SignatureStatus: "broken"}}, UnaffectedDigest: "sha256:unchanged-tree-set", CandidateDigest: "sha256:candidate", ChangedObjectIDs: []string{"badblob", "oldtip", "newtip"}, StorageBeforeBytes: 1000, StorageAfterBytes: 800, RollbackUntil: time.Now().Add(time.Hour), RollbackLimits: []string{"Independent clones cannot be rolled back centrally."}, CollaboratorActions: []string{"Fetch replacement refs and rebase unpublished work."}, LinkImpacts: []LinkImpact{{Kind: "commit_link", Reference: "issue:1", Status: "broken", Action: "Use the restricted commit map."}}, UnrewritableResources: []string{"fork:independent"}}
	x, e = s.AddCandidate("repo", x.ID, "responder", c)
	if e != nil || len(x.Candidates) != 1 || x.Candidates[0].Published {
		t.Fatalf("candidate=%+v %v", x, e)
	}
	checks := []RehearsalCheck{}
	for _, d := range []string{"integrity", "build", "check", "release", "dependency", "clone", "fetch"} {
		checks = append(checks, RehearsalCheck{Domain: d, Status: "passed", Reference: "run:" + d, Digest: "sha256:" + d, Summary: "Scenario completed against the isolated candidate."})
	}
	checks[3].Status = "failed"
	checks[3].Summary = "Signed release references the replaced commit and must be rebuilt."
	x, e = s.AddRehearsal("repo", x.ID, "responder", RehearsalInput{CandidateID: x.Candidates[0].ID, Environment: "networkless-workspace:1", BudgetMinutes: 30, BudgetCost: 500, Checks: checks, ObservedMinutes: 12, ObservedCost: 100})
	if e != nil || len(x.Rehearsals) != 1 || x.Rehearsals[0].Status != "blocked" || len(x.Rehearsals[0].Blockers) != 1 {
		t.Fatalf("rehearsal=%+v %v", x, e)
	}
	if _, e = s.AddCandidate("repo", x.ID, "responder", RewriteCandidateInput{}); !errors.Is(e, ErrInvalid) {
		t.Fatalf("invalid candidate=%v", e)
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
