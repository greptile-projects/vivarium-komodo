package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestReliabilityInvestigationWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "reliable", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reliability-agent")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "reliability-agent", auth.API, auth.RepositoryRead)
	objectives, _ := serviceobjectives.New(t.TempDir())
	investigations, _ := reliabilityinvestigations.New(t.TempDir())
	mux := http.NewServeMux()
	registerServiceObjectivesHTTP(mux, objectives, catalog, credentials)
	registerReliabilityInvestigationsHTTP(mux, investigations, objectives, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	objectiveBase := "/repositories/" + string(repo.ID) + "/service-objectives"
	var objective serviceobjectives.Objective
	workflowJSON(t, server.URL, http.MethodPost, objectiveBase, owner, objectiveBody("owner", "available", "ratio", time.Now().UTC().Add(60*24*time.Hour)), 201, &objective)

	base := "/repositories/" + string(repo.ID) + "/reliability-investigations"
	body := `{"objective_id":"` + objective.ID + `","objective_version":1,"revision":"commit-a","trigger":{"kind":"objective","resource_id":"` + objective.ID + `","revision":"version:1"},"title":"Review reliability drift","question":"Why may review availability be changing?","journey_ids":["review"],"evidence":[{"kind":"metric","resource_id":"availability","revision":"window-before","window":"prior 28d","summary":"99.95 percent","audience":"repository","baseline":true},{"kind":"metric","resource_id":"availability","revision":"window-current","window":"current 28d","summary":"99.70 percent","audience":"repository","baseline":false,"uncertainty":"low traffic"}]}`
	var investigation reliabilityinvestigations.Investigation
	workflowJSON(t, server.URL, http.MethodPost, base, agent, body, 201, &investigation)
	if investigation.CreatorID != "reliability-agent" || len(investigation.Evidence) != 2 {
		t.Fatalf("read-only agent investigation lost provenance: %+v", investigation)
	}
	evidence := investigation.Evidence[1].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", agent, `{"kind":"hypothesis","body":"The retry change may amplify dependency timeouts.","uncertainty":"No dependency trace yet.","citations":[{"evidence_id":"`+evidence+`"}]}`, 201, &investigation)
	hypothesis := investigation.Entries[0].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", agent, `{"kind":"challenge","body":"Traffic composition could explain the difference.","challenges":"`+hypothesis+`","citations":[{"evidence_id":"`+evidence+`"}]}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/input-requests", agent, `{"owner_id":"database-team","owner_kind":"dependency","question":"Did timeout behavior change?","evidence_needed":["sanitized timeout ratio"]}`, 201, &investigation)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/entries", agent, `{"kind":"conclusion","body":"Current evidence does not distinguish code from traffic effects.","verdict":"inconclusive","citations":[{"evidence_id":"`+evidence+`"}]}`, 201, &investigation)
	conclusion := investigation.Entries[len(investigation.Entries)-1].ID
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+investigation.ID+"/outcomes", owner, `{"kind":"issue","resource_id":"issue-42","rationale":"Collect a comparable dependency trace.","conclusion_entry_id":"`+conclusion+`"}`, 201, &investigation)
	for _, want := range []string{"uncertain_evidence", "disputed_conclusion", "dependency_input_pending", "inconclusive_signals"} {
		if !stringSliceContains(investigation.Blockers, want) {
			t.Fatalf("missing blocker %s: %+v", want, investigation.Blockers)
		}
	}

	revision := objectiveBody("owner", "available", "ratio", time.Now().UTC().Add(90*24*time.Hour))
	workflowJSON(t, server.URL, http.MethodPost, objectiveBase+"/"+objective.ID+"/versions", owner, revision[:1]+`"expected_version":1,`+revision[1:], 201, nil)
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+investigation.ID, agent, "", 200, &investigation)
	if !investigation.Entries[0].Stale || !stringSliceContains(investigation.Blockers, "objective_version_changed") {
		t.Fatalf("changed terms did not stale cited reasoning: %+v", investigation)
	}
}

func stringSliceContains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
