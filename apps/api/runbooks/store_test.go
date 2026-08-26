package runbooks

import "testing"

func sample() Input {
	return Input{Name: "Restore checkout", Purpose: "Diagnose and safely restore checkout", Scope: Scope{Kind: "service", ResourceID: "checkout", Revision: "service-v7", OwnerID: "service-owner"}, Preconditions: []Precondition{{ID: "impact", Description: "Confirm current user impact", Evidence: "dashboard window", OwnerID: "responder", Safe: true}, {ID: "backup", Description: "Confirm rollback target", Evidence: "attested release", OwnerID: "release-owner", Assumption: "backup is healthy", Safe: false}}, Steps: []Step{{ID: "inspect", Kind: "diagnostic", Title: "Inspect saturation", Purpose: "Separate dependency failure from release regression", Preconditions: []string{"impact"}, References: []Reference{{Kind: "command", ResourceID: "cmd:metrics", Revision: "sha256:1", Detail: "read sanitized saturation", Accessible: false, Reviewed: true, OwnerID: "operators"}}, ExpectedEvidence: []string{"sanitized saturation"}, RequiredSkills: []string{"diagnosis"}}, {ID: "choose", Kind: "decision", Title: "Choose containment", Purpose: "Require human judgment before change", Preconditions: []string{"impact", "backup"}, Decision: &Decision{Question: "Rollback?", Options: []string{"rollback", "escalate"}, HumanRequired: true, OwnerID: "commander"}, References: []Reference{{Kind: "agent", ResourceID: "triage-agent", Revision: "v2", Detail: "read-only comparison", Accessible: true, Reviewed: true, SecretBearing: true, OwnerID: "agent-owner"}}, ExpectedEvidence: []string{"attributed decision"}, RequiredAuthority: []string{"deployment:rollback"}, OwnerIDs: []string{"commander"}, RequiredSkills: []string{"release"}, DependsOn: []string{"inspect"}, RollbackCriteria: []string{"health worsens"}}}, RollbackCriteria: []string{"error rate rises"}, OwnerIDs: []string{"service-owner"}, RequiredSkills: []string{"diagnosis", "release"}, EscalationPaths: []Escalation{{Condition: "no safe option", OwnerID: "commander", RequiredSkills: []string{"incident-command"}, AudienceIDs: []string{"service-owners"}, Action: "open incident"}}, PolicyReferences: []PolicyReference{{Kind: "environment", ResourceID: "prod", Revision: "v4", Accessible: true, Conflicting: true, OwnerID: "platform"}}, ChangeReason: "make recovery reviewable"}
}
func TestVersionedRunbookDerivesSafetyAndAuthority(t *testing.T) {
	s, _ := New(t.TempDir())
	x, e := s.Create("repo", "alice", sample())
	if e != nil {
		t.Fatal(e)
	}
	if len(x.Findings) != 5 || len(x.AuthorityPreview) != 2 || x.AuthorityPreview[1].Granted || !x.AuthorityPreview[1].RequiresHumanJudgment {
		t.Fatalf("lost review findings or authority boundary: %#v", x)
	}
	in := sample()
	in.ChangeReason = "clarify evidence"
	if _, e = s.Revise("repo", x.ID, "bob", 0, in); e != ErrConflict {
		t.Fatalf("wanted conflict, got %v", e)
	}
	y, e := s.Revise("repo", x.ID, "bob", 1, in)
	if e != nil || y.CurrentVersion != 2 || len(y.Versions) != 2 {
		t.Fatalf("revision failed: %#v %v", y, e)
	}
}
