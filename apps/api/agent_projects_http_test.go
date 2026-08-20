package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprojects"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAgentProjectsExposeRevisionExactIntentAndGaps(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "assistant", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := agentprojects.New(t.TempDir())
	mux := http.NewServeMux()
	registerAgentProjectsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/agent-projects"
	in := agentprojects.Input{
		Name: "review assistant", Purpose: "Review changes and stop for human judgment", RepositoryRevision: "abc123", DefinitionPath: ".agents/reviewer.json", ChangeReason: "initial reviewed intent",
		Prompts:          []agentprojects.ReviewedText{{ID: "system", Kind: "prompt", Content: "Never publish changes", Revision: "p1"}},
		Instructions:     []agentprojects.ReviewedText{{ID: "policy", Kind: "instruction", Content: "Never publish changes", Revision: "i1"}},
		Tools:            []agentprojects.Tool{{Name: "repository-reader", Revision: "tool-v2", Capabilities: []string{"repository:read", "pull:comment"}, Boundary: "one repository"}},
		Models:           []agentprojects.Model{{Provider: "example", Name: "reasoner", Revision: "2026-08", Guarantees: []agentprojects.Guarantee{{Claim: "always correct", UnsupportedReason: "no such guarantee"}}}},
		KnowledgeSources: []agentprojects.KnowledgeSource{{Reference: "handbook", Revision: "h7", Audience: "repository", DataUse: "inference only", Accessible: false, InaccessibleReason: "reader lacks access"}},
		Dependencies:     []agentprojects.Dependency{{Kind: "package", Reference: "review-rules", Revision: "sha256:deadbeef", Accessible: false, InaccessibleReason: "artifact unavailable"}},
		MemoryPolicy:     agentprojects.MemoryPolicy{Scope: "one task", Retention: "session", DeletionRule: "delete at session end"}, SupportedTasks: []string{"pull request review"}, ExpectedOutputs: []string{"cited review"}, ProhibitedActions: []string{"merge", "publish"}, DataUseTerms: []string{"no training"}, Budgets: []agentprojects.Budget{{Kind: "cost", Limit: 2, Unit: "USD", Period: "task"}}, HumanEscalations: []agentprojects.Escalation{{Trigger: "uncertain security impact", Action: "stop and ask", BlocksWork: true}}, DeploymentBoundaries: []string{"networkless preview"},
	}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created agentprojects.Project
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	if created.GrantsAuthority || len(created.EffectiveCapabilities) != 2 || created.Versions[0].RepositoryRevision != "abc123" {
		t.Fatalf("intent projection wrong: %#v", created)
	}
	kinds := map[string]bool{}
	for _, gap := range created.Gaps {
		kinds[gap.Kind] = true
		if gap.AttributedTo != "owner" {
			t.Fatalf("gap lost attribution: %#v", gap)
		}
	}
	for _, want := range []string{"missing_owner", "inaccessible_dependency", "conflicting_instruction", "unsupported_guarantee"} {
		if !kinds[want] {
			t.Errorf("missing %s: %#v", want, created.Gaps)
		}
	}
	var public agentprojects.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &public)
	if len(public.Items) != 1 {
		t.Fatalf("public project unavailable: %#v", public)
	}
	in.ChangeReason = "reviewed revision"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		agentprojects.Input
	}{1, in}
	body, _ = json.Marshal(revision)
	var revised agentprojects.Project
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 {
		t.Fatalf("history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusConflict, nil)
}
