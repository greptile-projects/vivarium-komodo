package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestConnectedWorkSnapshotsReasoningIntoOrderedOwnedTasks(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repositoriesStore, _ := repositories.New(t.TempDir(), git)
	plans, _ := proposals.New(t.TempDir())
	canvases, _ := investigations.New(t.TempDir())
	impacts, _ := impactassessments.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := repositoriesStore.Create("owner", repositories.Metadata{Name: "reasoned", Visibility: repositories.Private})
	canvas, _ := canvases.Create(string(repository.ID), "Routing", "What must remain stable?", "main", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "owner")
	canvas, _ = canvases.Add(string(repository.ID), canvas.ID, "owner", investigations.Entry{Type: "conclusion", Body: "Callers require stable routing.", Citations: []investigations.Citation{{RepositoryID: string(repository.ID), CommitID: canvas.CommitID, Kind: "source", Path: "route.go", LineStart: 4}}})
	assessment, _ := impacts.Create(impactassessments.Assessment{RepositoryID: string(repository.ID), Title: "Routing risk", Revision: "main", CommitID: canvas.CommitID, CreatorID: "owner", Sources: []impactassessments.Source{{Kind: "investigation_conclusion", InvestigationID: canvas.ID, ConclusionID: canvas.Entries[0].ID}}, Impacts: []impactassessments.Impact{{Category: "test", Summary: "Compatibility needs a regression check.", Verification: []string{"go test ./..."}, Evidence: []impactassessments.Evidence{{RepositoryID: string(repository.ID), CommitID: canvas.CommitID, Kind: "test", Path: "route_test.go", Label: "routing contract"}}}}})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerReasoningWorkHTTP(mux, canvases, impacts, plans, repositoriesStore, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	body, _ := json.Marshal(map[string]any{
		"title": "Preserve routing", "body": "Implement the agreed routing contract.", "investigation_id": canvas.ID, "assessment_id": assessment.ID,
		"tasks": []map[string]any{
			{"title": "Implement routing", "outcome": "Routing follows the conclusion.", "source_kind": "investigation_conclusion", "source_id": canvas.Entries[0].ID, "owner_kind": "human", "owner_id": "owner", "mandate": "Implement only the cited conclusion."},
			{"title": "Verify compatibility", "outcome": "The identified risk is checked.", "source_kind": "impact_item", "source_id": assessment.Impacts[0].ID, "owner_kind": "agent", "owner_id": "codex", "mandate": "Run the retained verification."},
			{"title": "Document the result", "outcome": "Both changes are explained.", "source_kind": "investigation_conclusion", "source_id": canvas.Entries[0].ID, "depends_on": []int{0, 1}},
		},
	})
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/repositories/"+string(repository.ID)+"/connected-work", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result struct {
		Proposal proposals.Proposal `json:"proposal"`
		Tasks    []proposals.Task   `json:"tasks"`
	}
	_ = json.NewDecoder(response.Body).Decode(&result)
	if response.StatusCode != http.StatusCreated || len(result.Tasks) != 3 {
		t.Fatalf("status=%d result=%#v", response.StatusCode, result)
	}
	if result.Tasks[0].ReasoningContext == nil || result.Tasks[0].ReasoningContext.ConclusionID != canvas.Entries[0].ID || result.Tasks[1].ReasoningContext == nil || result.Tasks[1].ReasoningContext.ImpactID != assessment.Impacts[0].ID {
		t.Fatalf("reasoning = %#v", result.Tasks)
	}
	if len(result.Tasks[2].DependsOn) != 2 || result.Tasks[2].DependsOn[0] != result.Tasks[0].ID || result.Tasks[1].Assignment == nil || result.Tasks[1].Assignment.BaseRevision != canvas.CommitID {
		t.Fatalf("ordering = %#v", result.Tasks)
	}
	_, _ = canvases.Rerun(string(repository.ID), canvas.ID, "owner", "main", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "Code changed")
	plan, _ := plans.GetPlan(string(repository.ID), result.Proposal.ID)
	if plan.Tasks[0].ReasoningContext.CommitID != canvas.CommitID || plan.Tasks[0].ReasoningContext.Claim != "Callers require stable routing." {
		t.Fatalf("history was rewritten: %#v", plan.Tasks[0].ReasoningContext)
	}
}
