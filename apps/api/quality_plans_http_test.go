package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/qualityplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestQualityPlanPublicAPIAndDerivedGaps(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := qualityplans.New(t.TempDir())
	mux := http.NewServeMux()
	registerQualityPlansHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/quality-plans"
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	in := qualityplans.Input{
		Name: "Release quality", Description: "Protected checkout behavior", ChangeReason: "initial agreement", OwnerIDs: []string{"qa"},
		Scopes:       []qualityplans.Scope{{Kind: "repository", Reference: string(repo.ID)}, {Kind: "release", Reference: "v2"}},
		Risks:        []qualityplans.Risk{{ID: "risk-loss", Description: "orders can be lost", Severity: "critical"}},
		Requirements: []qualityplans.Requirement{{ID: "req-1", Kind: "issue", Reference: "issue-42", Rationale: "customer reports"}},
		Behaviors: []qualityplans.Behavior{
			{ID: "checkout", Subject: "order creation", Description: "checkout completes once", Expected: "exactly one order", RequirementIDs: []string{"req-1"}, RiskIDs: []string{"risk-loss"}, TestLevels: []string{"unit", "journey"}, EnvironmentIDs: []string{"prod-like"}, OwnerIDs: []string{"qa"}, JudgeIDs: []string{"release-owner"}, Testable: true},
			{ID: "legacy", Subject: "legacy gateway", Description: "legacy gateway remains safe", Expected: "request is rejected safely", TestLevels: []string{"exploratory"}, UntestableReason: "gateway is unavailable"},
		},
		Environments:       []qualityplans.Environment{{ID: "prod-like", Name: "Production-like", Description: "supported browser and payment sandbox", Supported: true}},
		RepresentativeData: []qualityplans.RepresentativeData{{ID: "orders", Description: "boundary order shapes", Source: "synthetic generator", PrivacyClassification: "internal", Synthetic: true}},
		CoverageGoals:      []qualityplans.CoverageGoal{{Subject: "checkout", Metric: "journey coverage", Target: 100, TestLevel: "journey"}},
		Schedules:          []qualityplans.Schedule{{Cadence: "weekly", OwnerIDs: []string{"qa"}}},
		ReleaseThresholds:  []qualityplans.Threshold{{ID: "checkout-pass", Subject: "checkout", Metric: "pass rate", Operator: "gte", Value: 100, Required: true}},
		Evidence:           []qualityplans.Evidence{{ID: "check-1", Kind: "check", Reference: "run-1", BehaviorIDs: []string{"checkout"}, Status: "passing", ObservedAt: time.Now().UTC(), AuthorID: "ci"}},
		Exceptions:         []qualityplans.Exception{{ID: "legacy-waiver", Subject: "legacy", Rationale: "replacement underway", OwnerID: "release-owner", ExpiresAt: expires}},
	}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created qualityplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	kinds := map[string]bool{}
	for _, gap := range created.Gaps {
		kinds[gap.Kind] = true
	}
	for _, want := range []string{"missing_owner", "missing_judge", "untestable_claim", "missing_evidence", "expiring_exception"} {
		if !kinds[want] {
			t.Errorf("missing derived gap %s: %#v", want, created.Gaps)
		}
	}
	var list qualityplans.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatalf("public catalog unavailable: %#v", list)
	}
	in.ChangeReason = "reviewed again"
	revision := struct {
		ExpectedVersion int64 `json:"expected_version"`
		qualityplans.Input
	}{1, in}
	body, _ = json.Marshal(revision)
	var revised qualityplans.Plan
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusCreated, &revised)
	if revised.CurrentVersion != 2 || len(revised.Versions) != 2 {
		t.Fatalf("history lost: %#v", revised)
	}
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+created.ID+"/versions", owner, string(body), http.StatusConflict, nil)
}
