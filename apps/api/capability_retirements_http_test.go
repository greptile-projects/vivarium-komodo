package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestCapabilityRetirementIsAnAcknowledgedMigrationContract(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := repos.Create("maintainer", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = repos.AddCollaborator("maintainer", repo.ID, "consumer-owner")
	maintainer := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	consumer := issueAccess(t, credentials, "consumer-owner", auth.API, auth.RepositoryRead)
	agent := issueAccess(t, credentials, "impact-agent", auth.API, auth.RepositoryRead)
	inventories, _ := capabilityinventories.New(t.TempDir())
	now := time.Now().UTC()
	inv, _ := inventories.Create(string(repo.ID), "maintainer", capabilityinventories.Input{Name: "Checkout v1", Description: "legacy checkout", SourceRevision: "commit-a", DefinitionPath: ".komodo/capabilities/checkout.json", Elements: []capabilityinventories.Element{{ID: "route", Kind: "interface", Reference: "POST /v1/checkout", Revision: "openapi-1", OwnerIDs: []string{"api-owner"}, Description: "legacy route"}}, Consumers: []capabilityinventories.Consumer{{ID: "mobile", Kind: "application", Reference: "mobile-app", Revision: "commit-mobile", Status: "active", OwnerIDs: []string{"consumer-owner"}, ElementIDs: []string{"route"}, Discovery: "observed", Audience: "repository"}}, UsageEvidence: []capabilityinventories.UsageEvidence{{ID: "usage", Kind: "telemetry", Reference: "usage-7", Revision: "release-7", ConsumerIDs: []string{"mobile"}, ElementIDs: []string{"route"}, Status: "current", ObservedAt: now, ExpiresAt: ptrTime(now.Add(24 * time.Hour)), AuthorID: "analyst"}}, CompatibilityPromises: []capabilityinventories.CompatibilityPromise{{ID: "promise", Scope: "mobile", Revision: "policy-3", ConsumerIDs: []string{"mobile"}, OwnerIDs: []string{"consumer-owner"}, Guarantee: "v1 through September", Until: ptrTime(now.Add(60 * 24 * time.Hour))}}, OwnerIDs: []string{"api-owner"}, ChangeReason: "prepare retirement"})
	store, _ := capabilityretirements.New(t.TempDir(), inventories)
	mux := http.NewServeMux()
	registerCapabilityRetirementsHTTP(mux, store, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/capability-retirements"
	input := capabilityretirements.Input{InventoryID: inv.ID, InventoryVersion: 1, Rationale: "v2 has safer idempotency", Replacements: []capabilityretirements.Replacement{{ID: "v2", Kind: "interface", Reference: "POST /v2/checkout", Revision: "openapi-2", MigrationGuide: "docs/migrate-v2", OwnerIDs: []string{"api-owner"}}}, Audiences: []capabilityretirements.Audience{{ID: "mobile-users", Name: "Mobile checkout", ConsumerIDs: []string{"mobile"}, OwnerIDs: []string{"consumer-owner"}, StopsWorking: "v1 checkout requests return 410", MigrationPath: "ship mobile release using v2", Embargoed: true}}, Stages: []capabilityretirements.Stage{{ID: "warn", Name: "Warnings", StartsAt: now.Add(time.Hour), EndsAt: now.Add(24 * time.Hour), Compatibility: "v1 works with warnings", ExitCriteria: []string{"mobile release available"}}, {ID: "disable", Name: "Disable", StartsAt: now.Add(24 * time.Hour), EndsAt: now.Add(48 * time.Hour), Compatibility: "v1 disabled behind rollback flag", ExitCriteria: []string{"zero v1 traffic"}, RollbackTo: "warn"}}, RemovalDeadline: now.Add(30 * 24 * time.Hour), SuccessCriteria: []string{"zero v1 requests for seven days"}, RollbackCriteria: []string{"checkout success drops below 99%"}, Communication: capabilityretirements.CommunicationPolicy{OwnerID: "api-owner", Channels: []string{"release notes", "owner inbox"}, Cadence: "weekly", NoticePeriodDays: 30, Escalation: "pause removal"}, RequiredApprovals: []capabilityretirements.ApprovalRequirement{{OwnerID: "consumer-owner", Scope: "mobile migration", Deadline: now.Add(-time.Hour)}}, Assumptions: []string{"mobile release review completes"}, Commitments: []capabilityretirements.Commitment{{ID: "support", Reference: "policy-3", Revision: "3", OwnerID: "consumer-owner", Guarantee: "support through September", Until: ptrTime(now.Add(60 * 24 * time.Hour)), Conflicts: true}}, Exceptions: []capabilityretirements.Exception{{ID: "partner", Scope: "partner tenant", Rationale: "cannot update during freeze", OwnerID: "consumer-owner", ExpiresAt: now.Add(7 * 24 * time.Hour)}}}
	b, _ := json.Marshal(input)
	workflowJSON(t, server.URL, http.MethodPost, base, consumer, string(b), http.StatusUnauthorized, nil)
	var plan capabilityretirements.Plan
	workflowJSON(t, server.URL, http.MethodPost, base, maintainer, string(b), http.StatusCreated, &plan)
	assertRetirementBlockers(t, plan, "embargoed_dependency", "conflicting_commitment", "exception_pending", "owner_unresponsive")
	if plan.Ready {
		t.Fatal("unacknowledged removal reported ready")
	}
	assessment := map[string]any{"author_kind": "read_only_agent", "kind": "challenge", "body": "traffic sample excludes old client versions", "evidence_reference": "repo://analysis/usage-gap", "audience_ids": []string{"mobile-users"}}
	b, _ = json.Marshal(assessment)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/assessments", agent, string(b), http.StatusCreated, &plan)
	if len(plan.Assessments) != 1 || plan.Assessments[0].AuthorID != "impact-agent" {
		t.Fatalf("assessment attribution lost: %#v", plan.Assessments)
	}
	approval := map[string]string{"scope": "mobile migration", "decision": "approved", "rationale": "v2 release is scheduled and rollback is adequate"}
	b, _ = json.Marshal(approval)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/approvals", agent, string(b), http.StatusForbidden, nil)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/approvals", consumer, string(b), http.StatusCreated, &plan)
	decision := map[string]any{"kind": "conflicting_commitment", "subject": "support", "decision": "extend", "rationale": "retain v1 until the promised support window closes", "expires_at": now.Add(20 * 24 * time.Hour)}
	b, _ = json.Marshal(decision)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+plan.ID+"/policy-decisions", consumer, string(b), http.StatusCreated, &plan)
	found := false
	for _, x := range plan.Blockers {
		if x.Kind == "conflicting_commitment" && x.ResolvedByDecisionID != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("bounded policy decision did not remain attached to blocker")
	}
	invInput := inv.Versions[0].Input
	invInput.ChangeReason = "new desktop consumer discovered"
	invInput.Consumers = append(invInput.Consumers, capabilityinventories.Consumer{ID: "desktop", Kind: "application", Reference: "desktop", Status: "unknown", OwnerIDs: []string{"desktop-owner"}, ElementIDs: []string{"route"}, Discovery: "dynamic", Audience: "repository"})
	_, _ = inventories.Revise(string(repo.ID), inv.ID, "analyst", 1, invInput)
	workflowJSON(t, server.URL, http.MethodGet, base+"/"+plan.ID, "", "", http.StatusOK, &plan)
	assertRetirementBlockers(t, plan, "changed_usage")
}
func ptrTime(v time.Time) *time.Time { return &v }
func assertRetirementBlockers(t *testing.T, p capabilityretirements.Plan, wants ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, x := range p.Blockers {
		got[x.Kind] = true
	}
	for _, w := range wants {
		if !got[w] {
			t.Errorf("missing blocker %s in %#v", w, p.Blockers)
		}
	}
}
