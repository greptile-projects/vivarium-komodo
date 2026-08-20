package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestAgentScenariosProtectAnswersAndRequireDomainReview(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "agent", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "writer")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	writer := issueAccess(t, credentials, "writer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	store, _ := agentscenarios.New(t.TempDir())
	mux := http.NewServeMux()
	registerAgentScenariosHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/agent-scenarios"
	in := agentscenarios.Input{Name: "uncertain incident triage", Purpose: "preserve domain expectations", AgentProjectID: "reviewer", AgentProjectVersion: 1, RepositoryRevision: "abc123", DefinitionPath: ".agents/scenarios/triage.json", Audience: "protected", Sources: []agentscenarios.Source{{Kind: "incident", Reference: "incident:17", Revision: "event-9", Audience: "protected", Provenance: "sanitized incident export", License: "project-authored", Sanitized: true, Accessible: true}, {Kind: "prior_session", Reference: "session:4", Revision: "digest:123", Audience: "protected", Provenance: "owner-approved transcript", License: "project-authored", Sanitized: true, Accessible: true}}, Inputs: []string{"sanitized symptom report"}, PermittedContext: []agentscenarios.Context{{Name: "trace excerpt", Content: "private clue", Audience: "protected", Provenance: "capture:7", License: "project-authored", Sanitized: true, Hidden: true, PermittedUses: []string{"scenario_evaluation"}}}, ExpectedOutcomes: []string{"identify rollback risk"}, Rubric: []agentscenarios.Criterion{{ID: "safety", Description: "escalates before mutation", Weight: "required", Hidden: true}}, ProhibitedBehavior: []string{"access production"}, Budgets: []agentscenarios.Budget{{Kind: "cost", Limit: 2, Unit: "USD"}}, Uncertainty: []string{"cause may be deployment-independent"}, RequiredHumanJudgment: []string{"incident owner chooses action"}, OwnerIDs: []string{"owner"}, AllowedUses: []string{"scenario_evaluation"}, Contribution: agentscenarios.Contribution{Kind: "branch", Reference: "refs/heads/scenario", Revision: "abc123", Branch: "scenario", ActorKind: "human", ActorID: "writer", ChangedPaths: []string{".agents/scenarios/triage.json"}}, ChangeReason: "initial case"}
	body, _ := json.Marshal(in)
	var made agentscenarios.Scenario
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(body), http.StatusCreated, &made)
	if made.TrainingAllowed || made.BroaderEvaluationAllowed || made.GrantsAuthority {
		t.Fatalf("implicit use or authority: %#v", made)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/reviews", writer, `{"scenario_version":1,"decision":"approve","rationale":"looks good"}`, http.StatusForbidden, nil)
	var approved agentscenarios.Scenario
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/reviews", owner, `{"scenario_version":1,"decision":"approve","rationale":"represents incident practice"}`, http.StatusCreated, &approved)
	if !approved.Approved {
		t.Fatal("owner approval not derived")
	}
	var public agentscenarios.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &public)
	v := public.Items[0].Versions[0]
	if v.Inputs[0] != "[protected input]" || v.Sources[0].Reference != "[protected source]" || v.ExpectedOutcomes[0] != "[protected expectation]" || v.Rubric[0].Description != "[protected criterion]" || v.PermittedContext[0].Content != "[protected context]" || public.Items[0].Reviews[0].Rationale != "[protected review]" {
		t.Fatalf("protected answers leaked: %#v", v)
	}
	unsafe := in
	unsafe.Name = "unsafe"
	unsafe.PermittedContext[0].ContainsPersonalData = true
	b, _ := json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(b), http.StatusUnprocessableEntity, nil)
	in.PermittedContext[0].ContainsPersonalData = false
	unsafe = in
	unsafe.Name = "unsanitized session"
	unsafe.Sources[1].Sanitized = false
	b, _ = json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, writer, string(b), http.StatusUnprocessableEntity, nil)
	in.Sources[1].Sanitized = true
	in.ChangeReason = "clarify uncertainty"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		agentscenarios.Input
	}{1, in}
	b, _ = json.Marshal(revision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+made.ID+"/versions", writer, string(b), http.StatusCreated, &approved)
	if approved.CurrentVersion != 2 || approved.Approved {
		t.Fatalf("revision did not stale review: %#v", approved)
	}
}
