package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/exploratorysessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestExploratoryFindingBecomesGovernedRepairAndRegression(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("lead", repositories.Metadata{Name: "quality", Visibility: repositories.Private})
	token := issueAccess(t, credentials, "lead", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	sessions, _ := exploratorysessions.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	in := exploratorysessions.Input{Title: "Explore retry", OriginKind: "pull_request_preview", OriginReference: "preview", Candidate: exploratorysessions.Candidate{Kind: "pull_request", Reference: "pull-candidate", Revision: "base-commit"}, QualityPlanID: "quality-plan", Access: exploratorysessions.Access{ExpiresAt: time.Now().Add(time.Hour), Environment: "isolated-preview", Network: "preview", AllowedRoutes: []string{"/checkout"}}, TestData: exploratorysessions.TestData{Description: "synthetic cart", PrivacyClassification: "internal", Synthetic: true}, Budget: exploratorysessions.Budget{MaxMinutes: 60, MaxCost: 1}, Participants: []exploratorysessions.Participant{{ID: "lead", Kind: "human", Approved: true, Role: "lead"}}, Charters: []exploratorysessions.Charter{{ID: "retry", Title: "Retry", Risk: "duplicate", RiskLevel: "high", Mission: "retry once", OwnerID: "lead", Routes: []string{"/checkout"}, BehaviorIDs: []string{"one-order"}}}}
	session, _ := sessions.Create(string(repo.ID), "lead", in)
	session, _ = sessions.Append(string(repo.ID), session.ID, "lead", session.Revision, exploratorysessions.EventInput{Kind: "observation", CharterID: "retry", Route: "/checkout", BehaviorIDs: []string{"one-order"}, Observation: "two orders"})
	eventID := session.Events[0].ID
	session, _ = sessions.AddFinding(string(repo.ID), session.ID, "lead", session.Revision, exploratorysessions.FindingInput{CharterID: "retry", Title: "Retry duplicates order", Description: "two durable orders are created", EventIDs: []string{eventID}, ReproductionSteps: []string{"submit synthetic cart", "retry once"}})
	findingID := session.Findings[0].ID
	session, _ = sessions.UpdateFinding(string(repo.ID), session.ID, findingID, "lead", session.Revision, exploratorysessions.FindingUpdate{Classification: "defect", Reproduction: "reproduced", Rationale: "three clean attempts"})
	mux := http.NewServeMux()
	registerExploratorySessionsHTTP(mux, sessions, repos, credentials, issueStore, plans)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/exploratory-sessions/" + session.ID + "/findings/" + findingID
	var delivered struct {
		Session   exploratorysessions.Session `json:"session"`
		Issue     issues.Issue                `json:"issue"`
		Task      proposals.Task              `json:"task"`
		Authority map[string]bool             `json:"authority"`
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/delivery", token, `{"expected_revision":4,"expected_behavior":"exactly one order exists","severity":"high","owner_kind":"agent","owner_id":"agent:quality","acceptance_criteria":["one order exists","ordinary checks pass"],"permitted_event_ids":["`+eventID+`"],"minimized_reproduction":["submit synthetic cart","retry once"]}`, http.StatusCreated, &delivered)
	if delivered.Issue.AffectedCommitID != "base-commit" || delivered.Task.BaseRevision != "base-commit" || delivered.Authority["git"] || len(delivered.Issue.Relationships) != 2 {
		t.Fatalf("governed preload lost: %#v", delivered)
	}
	var verified exploratorysessions.Session
	workflowJSON(t, server.URL, http.MethodPost, base+"/verification", token, `{"expected_revision":5,"pull_request_id":"pull-9","base_revision":"base-commit","repair_revision":"repair-commit","failing_evidence_id":"run-base-failed","passing_evidence_id":"run-repair-passed","review_id":"review-accepted","quality_plan_id":"quality-plan","quality_plan_version":2,"regression_scenario_id":"scenario-retry","regression_scenario_version":1}`, http.StatusCreated, &verified)
	d := verified.Findings[0].Delivery
	if d == nil || d.PullRequestID != "pull-9" || d.FailingEvidenceID == d.PassingEvidenceID || d.ScenarioID != "scenario-retry" || verified.Findings[0].Status != "resolved" {
		t.Fatalf("durable regression linkage lost: %#v", verified)
	}
	linkedIssue, _ := issueStore.Get(string(repo.ID), delivered.Issue.ID)
	if len(linkedIssue.Relationships) != 8 {
		t.Fatalf("issue does not link back through delivery evidence: %#v", linkedIssue.Relationships)
	}
}
