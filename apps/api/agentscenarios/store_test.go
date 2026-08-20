package agentscenarios

import "testing"

func TestScopedAgentMayReviewOnlyItsAllowlistedCase(t *testing.T) {
	s, _ := New(t.TempDir())
	in := Input{Name: "support reply", Purpose: "domain case", AgentProjectID: "p", AgentProjectVersion: 1, RepositoryRevision: "rev", DefinitionPath: ".agents/case.json", Audience: "protected", Sources: []Source{{Kind: "support_thread", Reference: "support:1", Revision: "r1", Audience: "protected", Provenance: "sanitized export", License: "project-authored", Sanitized: true, Accessible: true}}, Inputs: []string{"question"}, PermittedContext: []Context{{Name: "docs", Content: "excerpt", Audience: "protected", Provenance: "docs:r1", License: "project-authored", Sanitized: true, PermittedUses: []string{"scenario_evaluation"}}}, ExpectedOutcomes: []string{"safe answer"}, Rubric: []Criterion{{ID: "safe", Description: "escalates", Weight: "required", Hidden: true}}, ProhibitedBehavior: []string{"guess"}, Uncertainty: []string{"unknown account state"}, RequiredHumanJudgment: []string{"owner checks policy"}, OwnerIDs: []string{"domain-owner"}, AllowedUses: []string{"scenario_evaluation"}, Contribution: Contribution{Kind: "workspace", Reference: "workspace:1", Revision: "rev", Workspace: "workspace:1", ActorKind: "agent", ActorID: "codex", ChangedPaths: []string{".agents/case.json"}, Scope: []string{".agents/case.json"}}, ChangeReason: "initial"}
	x, e := s.Create("repo", "domain-owner", in)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Review("repo", x.ID, "other-agent", "agent", 1, "comment", "outside scope"); e != ErrForbidden {
		t.Fatalf("unscoped agent review: %v", e)
	}
	x, e = s.Review("repo", x.ID, "codex", "agent", 1, "comment", "bounded alternative")
	if e != nil || len(x.Reviews) != 1 {
		t.Fatalf("scoped review failed: %#v %v", x, e)
	}
}
