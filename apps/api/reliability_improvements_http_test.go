package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestReliabilityImprovementCarriesHarmToVerifiedRecovery(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "recovery", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "agent")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead)
	objectives, _ := serviceobjectives.New(t.TempDir())
	investigations, _ := reliabilityinvestigations.New(t.TempDir())
	improvements, _ := reliabilityimprovements.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerServiceObjectivesHTTP(mux, objectives, catalog, credentials)
	registerReliabilityInvestigationsHTTP(mux, investigations, objectives, catalog, credentials)
	registerReliabilityImprovementsHTTP(mux, improvements, investigations, objectives, plans, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	root := "/repositories/" + string(repo.ID)
	var objective serviceobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, root+"/service-objectives", owner, objectiveBody("owner", "available", "ratio", time.Now().UTC().Add(30*24*time.Hour)), 201, &objective)
	var inv reliabilityinvestigations.Investigation
	workflowJSON(t, server.URL, http.MethodPost, root+"/reliability-investigations", agent, `{"objective_id":"`+objective.ID+`","objective_version":1,"revision":"bad-commit","trigger":{"kind":"budget_consumption","resource_id":"budget-window-1","revision":"window:current"},"title":"Review exhausted budget","question":"Why did review availability fall?","journey_ids":["review"],"evidence":[{"kind":"metric","resource_id":"availability","revision":"before","window":"prior 28d","summary":"99.95","audience":"repository","baseline":true},{"kind":"metric","resource_id":"availability","revision":"current","window":"current 28d","summary":"99.70","audience":"repository","baseline":false}]}`, 201, &inv)
	evidence := inv.Evidence[1].ID
	workflowJSON(t, server.URL, http.MethodPost, root+"/reliability-investigations/"+inv.ID+"/entries", agent, `{"kind":"conclusion","body":"Retry fanout depleted the review journey budget.","verdict":"supported","citations":[{"evidence_id":"`+evidence+`"}]}`, 201, &inv)
	conclusion := inv.Entries[0].ID
	body := `{"objective_id":"` + objective.ID + `","objective_version":1,"source":{"kind":"finding","resource_id":"` + inv.ID + `","entry_id":"` + conclusion + `"},"base_revision":"bad-commit","title":"Restore review availability","affected_revisions":["bad-commit","deployment-bad"],"journey_ids":["review"],"evidence_ids":["` + evidence + `"],"dependency_context":["database timeout budget"],"acceptance_criteria":["availability exceeds 99.9 percent","error budget is positive"],"baseline":{"indicator":"available","window":"prior 28d","value":99.95,"unit":"percent","evidence_id":"baseline-metric"},"tasks":[{"title":"Bound retry fanout","owner_kind":"agent","owner_id":"agent","acceptance_criteria":["ordinary checks pass"]},{"title":"Review dependency behavior","owner_kind":"human","owner_id":"owner","depends_on":[1],"acceptance_criteria":["ordinary review approves exact revision"]}]}`
	var made struct {
		Improvement reliabilityimprovements.Improvement `json:"improvement"`
		Proposal    proposals.Proposal                  `json:"proposal"`
		Tasks       []string                            `json:"tasks"`
	}
	workflowJSON(t, server.URL, http.MethodPost, root+"/reliability-improvements", owner, body, 201, &made)
	plan, _ := plans.GetPlan(string(repo.ID), made.Proposal.ID)
	if len(plan.Tasks) != 2 || plan.Tasks[0].OwnerID != "agent" || plan.Tasks[0].OwnerKind != "agent" || len(plan.Tasks[1].DependsOn) != 1 || plan.Tasks[1].DependsOn[0] != plan.Tasks[0].ID {
		t.Fatalf("ordered accountable work lost context: %+v", plan)
	}
	base := root + "/reliability-improvements/" + made.Improvement.ID
	var current reliabilityimprovements.Improvement
	workflowJSON(t, server.URL, http.MethodPost, base+"/delivery-links", owner, `{"kind":"pull_request","resource_id":"pr-1","revision":"repair-commit","task_id":"`+made.Tasks[0]+`","summary":"Reviewed repair with ordinary checks"}`, 201, &current)
	workflowJSON(t, server.URL, http.MethodPost, base+"/rollouts", owner, `{"deployment_id":"deploy-canary","release_id":"release-2","revision":"repair-commit","environment":"production","stage":"canary","rationale":"Contain and rollback if the measure fails.","measurements":[{"indicator":"available","window":"canary 30m","value":99.8,"unit":"percent","evidence_id":"metric-canary","passed":false}]}`, 201, &current)
	if current.State != "contained" || current.Rollouts[0].RequiredAction != "rollback" || current.BudgetState != "depleted" {
		t.Fatalf("failed rollout was not contained: %+v", current)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/rollouts", owner, `{"deployment_id":"deploy-progressive","release_id":"release-2","revision":"repair-commit","environment":"production","stage":"complete","rationale":"Current comparable window exceeds the recorded baseline target.","measurements":[{"indicator":"available","window":"current 28d","value":99.97,"unit":"percent","evidence_id":"metric-recovered","passed":true}]}`, 201, &current)
	if current.State != "verified" || current.BudgetState != "restored" || !current.PriorImpactRetained || len(current.Rollouts) != 2 {
		t.Fatalf("recovery erased impact or failed to restore budget: %+v", current)
	}
}
