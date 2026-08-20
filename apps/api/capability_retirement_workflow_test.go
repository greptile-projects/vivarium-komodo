package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityinventories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityproofs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityremovals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

// TestCapabilityRetirementWorkflow is the black-box boundary for the complete
// released-capability-to-verified-cleanup loop. It crosses the public APIs used
// by view=capabilities and uses stock Git objects from independently owned
// repositories for exact human- and agent-authored migration provenance.
func TestCapabilityRetirementWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	repos, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	provider, _ := repos.Create("platform-owner", repositories.Metadata{Name: "legacy-checkout", Visibility: repositories.Public})
	storefront, _ := repos.Create("storefront-owner", repositories.Metadata{Name: "storefront", Visibility: repositories.Private})
	partner, _ := repos.Create("partner-owner", repositories.Metadata{Name: "partner-extension", Visibility: repositories.Private})
	_, _ = repos.AddCollaborator("platform-owner", provider.ID, "migration-agent")
	_, _ = repos.AddCollaborator("storefront-owner", storefront.ID, "platform-owner")
	_, _ = repos.AddCollaborator("partner-owner", partner.ID, "migration-agent")

	platformOwner := issueAccess(t, credentials, "platform-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	storefrontOwner := issueAccess(t, credentials, "storefront-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	partnerOwner := issueAccess(t, credentials, "partner-owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "migration-agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	inventories, _ := capabilityinventories.New(t.TempDir())
	plans, _ := capabilityretirements.New(t.TempDir(), inventories)
	proofs, _ := capabilityproofs.New(t.TempDir(), plans)
	removals, _ := capabilityremovals.New(t.TempDir(), proofs, plans)
	mux := http.NewServeMux()
	registerCapabilityInventoriesHTTP(mux, inventories, repos, credentials)
	registerCapabilityRetirementsHTTP(mux, plans, repos, credentials)
	registerCapabilityProofsHTTP(mux, proofs, repos, credentials)
	registerCapabilityRemovalsHTTP(mux, removals, repos, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(provider.ID)
	now := time.Now().UTC()

	providerCommit := capabilityCommit(t, repos, provider.ID, "Platform Owner", "legacy route, schema, flag, configuration, documentation, and collection", "")
	storefrontCommit := capabilityCommit(t, repos, storefront.ID, "Storefront Owner", "human migration from checkout v1 to v2", "")
	partnerCommit := capabilityCommit(t, repos, partner.ID, "Migration Agent", "agent migration from checkout v1 to v2", "")

	legacy := capabilityInventoryInput(string(providerCommit), string(storefront.ID), now)
	var inventory capabilityinventories.Inventory
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-inventories", platformOwner, legacy, http.StatusCreated, &inventory)
	if inventory.RemovalReady || !inventoryHasGap(inventory, "dynamic_consumer") {
		t.Fatalf("runtime-only consumer was treated as absent: %#v", inventory.Gaps)
	}

	firstPlanInput := retirementInput(inventory.ID, inventory.CurrentVersion, string(storefront.ID), "", now)
	var abandoned capabilityretirements.Plan
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements", platformOwner, firstPlanInput, http.StatusCreated, &abandoned)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+abandoned.ID+"/consumer-discoveries", agent, map[string]any{"consumer_id": "partner-extension", "repository_id": string(partner.ID), "revision": string(partnerCommit), "evidence_reference": "scan:extension-manifest", "summary": "runtime extension manifest still calls checkout v1"}, http.StatusCreated, &abandoned)
	if !retirementHasBlocker(abandoned, "new_consumer") {
		t.Fatal("hidden independently owned consumer did not stop the original plan")
	}

	// Correct the inventory rather than erasing the discovery. The original plan
	// remains stale, while the new plan binds the complete consumer set.
	corrected := capabilityInventoryInput(string(providerCommit), string(storefront.ID), now)
	corrected.Consumers = []capabilityinventories.Consumer{
		{ID: "storefront", Kind: "repository", Reference: string(storefront.ID), Revision: string(storefrontCommit), Status: "active", OwnerIDs: []string{"storefront-owner"}, EnvironmentIDs: []string{"production"}, ElementIDs: capabilityElementIDs(), Discovery: "static", Audience: "repository"},
		{ID: "partner-extension", Kind: "extension", Reference: string(partner.ID), Revision: string(partnerCommit), Status: "active", OwnerIDs: []string{"partner-owner"}, EnvironmentIDs: []string{"production"}, ElementIDs: []string{"route", "flag", "documentation"}, Discovery: "observed", Audience: "repository"},
	}
	corrected.UsageEvidence = []capabilityinventories.UsageEvidence{{ID: "complete-scan", Kind: "runtime_discovery", Reference: "scan:released-checkout-v1", Revision: string(providerCommit), ConsumerIDs: []string{"storefront", "partner-extension"}, ElementIDs: capabilityElementIDs(), EnvironmentIDs: []string{"production"}, Status: "current", ObservedAt: now, ExpiresAt: timePointer(now.Add(7 * 24 * time.Hour)), AuthorID: "migration-agent"}}
	corrected.CompatibilityPromises = nil
	corrected.ChangeReason = "add the independently owned runtime extension found during review"
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-inventories/"+inventory.ID+"/versions", platformOwner, struct {
		ExpectedVersion int64 `json:"expected_version"`
		capabilityinventories.Input
	}{inventory.CurrentVersion, corrected}, http.StatusCreated, &inventory)
	workflowValue(t, server.URL, http.MethodGet, base+"/capability-retirements/"+abandoned.ID, platformOwner, nil, http.StatusOK, &abandoned)
	if !retirementHasBlocker(abandoned, "changed_usage") || !inventory.RemovalReady {
		t.Fatalf("inventory correction did not stale only the old contract: inventory=%#v plan=%#v", inventory.Gaps, abandoned.Blockers)
	}

	planInput := retirementInput(inventory.ID, inventory.CurrentVersion, string(storefront.ID), string(partner.ID), now)
	var plan capabilityretirements.Plan
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements", platformOwner, planInput, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/assessments", agent, map[string]any{"author_kind": "read_only_agent", "kind": "impact", "body": "both repositories must ship before warnings become disablement", "evidence_reference": "inventory:" + inventory.ID + ":v2", "audience_ids": []string{"storefront-users", "partner-users"}}, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/approvals", storefrontOwner, map[string]any{"scope": "storefront migration", "decision": "approved", "rationale": "human-owned release window accepted"}, http.StatusCreated, &plan)
	if plan.Ready || !retirementHasBlocker(plan, "owner_approval_pending") {
		t.Fatal("missed partner acknowledgement did not block staged retirement")
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/approvals", partnerOwner, map[string]any{"scope": "partner migration", "decision": "approved", "rationale": "independent owner accepts the agent-authored change"}, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/approvals", platformOwner, map[string]any{"scope": "provider replacement and removal", "decision": "approved", "rationale": "replacement, notices, rollback, and cleanup are governed"}, http.StatusCreated, &plan)
	if !plan.Ready || len(plan.Assessments) != 1 {
		t.Fatalf("acknowledged migration contract did not converge: %#v", plan.Blockers)
	}

	storefrontTask := createCapabilityTask(t, server.URL, base, platformOwner, plan, string(storefront.ID), "human", "storefront-owner", "Migrate storefront checkout", nil)
	partnerTask := createCapabilityTask(t, server.URL, base, platformOwner, plan, string(partner.ID), "agent", "migration-agent", "Migrate partner extension", []string{storefrontTask.ID})
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/tasks/"+partnerTask.ID+"/progress", agent, map[string]any{"state": "in_progress", "summary": "attempted before the provider example was accepted"}, http.StatusConflict, nil)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/tasks/"+storefrontTask.ID+"/progress", storefrontOwner, map[string]any{"state": "completed", "summary": "human-authored migration reviewed and released", "work": []capabilityretirements.WorkLink{{Kind: "pull_request", ID: "pull:storefront-v2", RepositoryID: string(storefront.ID), Revision: string(storefrontCommit)}, {Kind: "workspace", ID: "workspace:storefront-v2", RepositoryID: string(storefront.ID), Revision: string(storefrontCommit)}}}, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/tasks/"+partnerTask.ID+"/progress", agent, map[string]any{"state": "blocked", "summary": "first migration fails dual-support retry checks", "work": []capabilityretirements.WorkLink{{Kind: "session", ID: "agent-session:first-failed", RepositoryID: string(partner.ID), Revision: string(partnerCommit)}}}, http.StatusCreated, &plan)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/tasks/"+partnerTask.ID+"/progress", agent, map[string]any{"state": "completed", "summary": "corrected agent migration reviewed by the independent owner", "work": []capabilityretirements.WorkLink{{Kind: "pull_request", ID: "pull:partner-v2", RepositoryID: string(partner.ID), Revision: string(partnerCommit)}}}, http.StatusCreated, &plan)
	if len(plan.Tasks) != 2 || plan.Tasks[1].Progress[0].State != "blocked" || plan.Tasks[1].Progress[1].State != "completed" {
		t.Fatalf("failed and corrected cross-repository migration trail was lost: %#v", plan.Tasks)
	}

	revisions := capabilityproofs.Revisions{Provider: string(providerCommit), Consumer: string(partnerCommit), Schema: "schema:checkout-v2", Configuration: "config:dual-read-v2", Release: "release:coexistence-v2"}
	checks := []capabilityproofs.Check{
		{ID: "old", Mode: "old_only", Journey: "checkout", Expected: "released v1 remains supported", InputKeys: []string{"provider", "schema", "configuration"}},
		{ID: "dual", Mode: "dual_support", Journey: "checkout", Expected: "v1 and v2 produce one order", InputKeys: []string{"provider", "consumer", "schema", "configuration"}},
		{ID: "replacement", Mode: "replacement", Journey: "checkout", Expected: "v2 fulfills the supported contract", InputKeys: []string{"provider", "consumer", "schema", "configuration"}},
		{ID: "rollback", Mode: "rollback", Journey: "checkout", Expected: "v1 can be restored before deletion", InputKeys: []string{"provider", "schema", "configuration", "release"}},
		{ID: "journey", Mode: "journey", Journey: "checkout", Expected: "storefront and extension checkout remain healthy", InputKeys: []string{"provider", "consumer", "schema", "configuration", "release"}},
	}
	var proof capabilityproofs.Candidate
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs", platformOwner, capabilityproofs.Input{PlanID: plan.ID, StageID: "disable", Revisions: revisions, Environment: capabilityproofs.Environment{Kind: "networkless_workspace", Reference: "workspace:coexistence", Networkless: true, Synthetic: true, CostLimit: 25}, Checks: checks, ConsumerIDs: []string{"storefront", "partner-extension"}, RequiredOwnerIDs: []string{"platform-owner", "storefront-owner", "partner-owner"}}, http.StatusCreated, &proof)
	for _, check := range checks {
		status, summary := "passed", "synthetic coexistence check passed"
		if check.ID == "dual" {
			status, summary = "failed", "first agent migration duplicates a retry"
		}
		workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs/"+proof.ID+"/attempts", agent, capabilityproofs.AttemptInput{CheckID: check.ID, Status: status, Summary: summary, Revisions: revisions, Artifacts: []capabilityproofs.Artifact{{Name: check.ID + ".json", Digest: "sha256:" + check.ID, MediaType: "application/json"}}, Cost: 1, DurationMS: 50}, http.StatusCreated, &proof)
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs/"+proof.ID+"/attempts", agent, capabilityproofs.AttemptInput{CheckID: "dual", Status: "passed", Summary: "corrected agent migration preserves idempotency", Revisions: revisions, Artifacts: []capabilityproofs.Artifact{{Name: "dual-corrected.json", Digest: "sha256:dual-corrected", MediaType: "application/json"}}, Cost: 1, DurationMS: 45}, http.StatusCreated, &proof)
	windowStart := now.Add(-24 * time.Hour)
	staleRevisions := revisions
	staleRevisions.Release = "release:old-sample"
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs/"+proof.ID+"/usage", storefrontOwner, capabilityproofs.UsageInput{ConsumerID: "storefront", Status: "zero", EvidenceReference: "usage:stale-storefront", Revisions: staleRevisions, WindowStart: windowStart, WindowEnd: now}, http.StatusCreated, &proof)
	if proof.RemovalReady || len(proof.Usage[0].StaleInputKeys) == 0 {
		t.Fatal("stale zero-use observation was trusted as current removal evidence")
	}
	for _, consumer := range []struct{ id, token string }{{"storefront", storefrontOwner}, {"partner-extension", partnerOwner}} {
		workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs/"+proof.ID+"/usage", consumer.token, capabilityproofs.UsageInput{ConsumerID: consumer.id, Status: "zero", EvidenceReference: "usage:current:" + consumer.id, Revisions: revisions, WindowStart: windowStart, WindowEnd: now}, http.StatusCreated, &proof)
	}
	for _, owner := range []string{platformOwner, storefrontOwner, partnerOwner} {
		workflowValue(t, server.URL, http.MethodPost, base+"/capability-proofs/"+proof.ID+"/acknowledgements", owner, map[string]any{"decision": "acknowledged", "rationale": "exact coexistence and zero-use evidence accepted"}, http.StatusCreated, &proof)
	}
	if !proof.RemovalReady || !proof.Attempts[1].Superseded || proof.Usage[0].Current || len(proof.Usage[0].StaleInputKeys) == 0 {
		t.Fatalf("corrected proof did not retain failed or stale evidence: %#v %#v", proof.Attempts, proof.Usage)
	}

	removalInput := capabilityremovals.Input{PlanID: plan.ID, ProofID: proof.ID, CandidateRevision: revisions.Provider, OwnerIDs: []string{"platform-owner", "storefront-owner", "partner-owner"}, Stages: []capabilityremovals.Stage{
		{ID: "disable", Name: "Disable old use", PlanStageID: "disable", RequiredEvidence: []string{"merge_queue", "release", "documentation"}, MaxRemainingUse: 0, RollbackBoundary: "reversible"},
		{ID: "remove", Name: "Remove obsolete machinery", PlanStageID: "remove", RequiredEvidence: []string{"merge_queue", "release", "schema_migration", "infrastructure_migration", "documentation", "protected_environment"}, MaxRemainingUse: 0, RollbackBoundary: "irreversible"},
	}}
	var removal capabilityremovals.Removal
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals", platformOwner, removalInput, http.StatusCreated, &removal)
	addRemovalEvidence(t, server.URL, base, platformOwner, &removal, "disable", []string{"merge_queue", "release", "documentation"}, "disable-v1")
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/signals", partnerOwner, map[string]any{"expected_revision": removal.Revision, "stage_id": "disable", "remaining_use": 4, "health": "regressed", "control": "failed", "evidence_reference": "production:late-retry-regression", "environment": "protected-production", "release": "release:disable-v1", "next_action": "restore compatibility and correct the migration"}, http.StatusCreated, &removal)
	if removal.State != "paused" || !removalHasBlocker(removal, "health_regression") {
		t.Fatalf("late post-disable regression was not contained: %#v", removal.Blockers)
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/controls", platformOwner, map[string]any{"expected_revision": removal.Revision, "action": "restore", "rationale": "rollback before the irreversible code and schema deletion boundary"}, http.StatusCreated, &removal)
	if removal.State != "restored" || removal.Compatibility != "restored" {
		t.Fatal("pre-boundary rollback did not restore the legacy behavior")
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/signals", partnerOwner, map[string]any{"expected_revision": removal.Revision, "stage_id": "disable", "remaining_use": 0, "health": "healthy", "control": "passed", "evidence_reference": "production:corrected-disable", "environment": "protected-production", "release": "release:disable-v2", "next_action": "resume and advance to governed cleanup"}, http.StatusCreated, &removal)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/controls", platformOwner, map[string]any{"expected_revision": removal.Revision, "action": "resume", "rationale": "corrected migration and current production signal are healthy"}, http.StatusCreated, &removal)
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/controls", platformOwner, map[string]any{"expected_revision": removal.Revision, "action": "advance", "rationale": "owners accept current zero-use, health, delivery, and rollback evidence"}, http.StatusCreated, &removal)
	addRemovalEvidence(t, server.URL, base, platformOwner, &removal, "remove", []string{"merge_queue", "release", "schema_migration", "infrastructure_migration", "documentation", "protected_environment"}, "cleanup-v1")
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/signals", storefrontOwner, map[string]any{"expected_revision": removal.Revision, "stage_id": "remove", "remaining_use": 0, "health": "healthy", "control": "passed", "evidence_reference": "production:verified-cleanup", "environment": "protected-production", "release": "release:cleanup-v1", "next_action": "record complete cleanup and retain provenance"}, http.StatusCreated, &removal)

	cleanup := []capabilityremovals.CleanupItem{}
	for _, item := range []struct{ category, subject, status string }{
		{"code", "checkout v1 route and symbols", "removed"}, {"flags", "checkout_v1", "removed"}, {"data", "legacy checkout schema and collection", "deleted"}, {"credentials", "legacy integration credential", "revoked"}, {"telemetry", "checkout_v1 usage collection", "removed"}, {"documentation", "v1 guide and collection page", "removed"}, {"policy_exceptions", "temporary compatibility exception", "removed"},
	} {
		cleanup = append(cleanup, capabilityremovals.CleanupItem{Category: item.category, Subject: item.subject, Status: item.status, EvidenceReference: "artifact:cleanup:" + item.category, Revision: "cleanup-v1"})
	}
	workflowValue(t, server.URL, http.MethodPost, base+"/capability-removals/"+removal.ID+"/complete", platformOwner, map[string]any{"expected_revision": removal.Revision, "cleanup": cleanup, "outcome_measures": []string{"supported checkout remains healthy", "legacy use remains zero"}, "historical_evidence": []string{"inventory:" + inventory.ID + ":v1-v2", "plan:" + abandoned.ID, "plan:" + plan.ID, "proof:" + proof.ID}, "completed_revision": "cleanup-v1"}, http.StatusCreated, &removal)
	if removal.State != "completed" || removal.Compatibility != "removed" || removal.Completion == nil || len(removal.Controls) != 3 || len(removal.Signals) != 3 {
		t.Fatalf("verified cleanup lost delivery, rollback, or monitoring provenance: %#v", removal)
	}

	// The same public reads used by the web workspace retain both abandoned and
	// successful plans, superseded proof, independently authored work, and the
	// completed removal without exposing authority over either consumer.
	var publicPlans struct {
		Items []capabilityretirements.Plan `json:"items"`
	}
	var publicProofs struct {
		Items []capabilityproofs.Candidate `json:"items"`
	}
	var publicRemovals struct {
		Items []capabilityremovals.Removal `json:"items"`
	}
	workflowValue(t, server.URL, http.MethodGet, base+"/capability-retirements", platformOwner, nil, http.StatusOK, &publicPlans)
	workflowValue(t, server.URL, http.MethodGet, base+"/capability-proofs", platformOwner, nil, http.StatusOK, &publicProofs)
	workflowValue(t, server.URL, http.MethodGet, base+"/capability-removals", platformOwner, nil, http.StatusOK, &publicRemovals)
	if len(publicPlans.Items) != 2 || len(publicProofs.Items) != 1 || len(publicRemovals.Items) != 1 || !containsString(removal.NonAuthority, "Git write") || !containsString(plan.NonAuthority, "consumer access") {
		t.Fatalf("public lifecycle trail or non-authority boundary is incomplete: plans=%d proofs=%d removals=%d", len(publicPlans.Items), len(publicProofs.Items), len(publicRemovals.Items))
	}
}

func capabilityCommit(t *testing.T, repos *repositories.Store, repository storage.ID, author, content, parent string) storage.ObjectID {
	t.Helper()
	opened, err := repos.Open(repository)
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := opened.WriteObject(storage.BlobObject, []byte(content+"\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "capability.txt", blob))
	parentLine := ""
	if parent != "" {
		parentLine = "parent " + parent + "\n"
	}
	commit, err := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+parentLine+"author "+author+" <actor@example.test> 1 +0000\ncommitter "+author+" <actor@example.test> 1 +0000\n\ncapability migration\n"))
	if err != nil {
		t.Fatal(err)
	}
	return commit
}

func capabilityElementIDs() []string {
	return []string{"route", "symbol", "flag", "schema", "configuration", "documentation", "journey", "release", "collection"}
}

func capabilityInventoryInput(revision, storefront string, now time.Time) capabilityinventories.Input {
	elements := []capabilityinventories.Element{}
	for _, x := range []struct{ id, kind, ref string }{{"route", "interface", "POST /v1/checkout"}, {"symbol", "symbol", "CheckoutV1"}, {"flag", "flag", "checkout_v1"}, {"schema", "schema", "checkout_v1"}, {"configuration", "configuration", "checkout.v1.enabled"}, {"documentation", "documentation", "docs/checkout-v1.md"}, {"journey", "journey", "legacy checkout"}, {"release", "release", "release:v1"}, {"collection", "schema", "checkout_v1_events"}} {
		elements = append(elements, capabilityinventories.Element{ID: x.id, Kind: x.kind, Reference: x.ref, Revision: revision, OwnerIDs: []string{"platform-owner"}, Description: "released legacy " + x.id})
	}
	return capabilityinventories.Input{Name: "Legacy checkout v1", Description: "released legacy checkout behavior and all compatibility machinery", SourceRevision: revision, DefinitionPath: ".komodo/capabilities/checkout-v1.json", Elements: elements, Environments: []capabilityinventories.Environment{{ID: "production", Name: "Production", Revision: "release:v1", OwnerIDs: []string{"platform-owner"}}}, Consumers: []capabilityinventories.Consumer{{ID: "storefront", Kind: "repository", Reference: storefront, Revision: "released-storefront-v1", Status: "active", OwnerIDs: []string{"storefront-owner"}, EnvironmentIDs: []string{"production"}, ElementIDs: capabilityElementIDs(), Discovery: "static", Audience: "repository"}, {ID: "runtime-extensions", Kind: "external", Reference: "runtime extension registry", Status: "dynamic", OwnerIDs: []string{"platform-owner"}, EnvironmentIDs: []string{"production"}, ElementIDs: []string{"route", "flag"}, Discovery: "dynamic", Audience: "repository", Detail: "extension manifests are loaded at runtime"}}, UsageEvidence: []capabilityinventories.UsageEvidence{{ID: "static", Kind: "code_search", Reference: "search:checkout-v1", Revision: revision, ConsumerIDs: []string{"storefront"}, ElementIDs: capabilityElementIDs(), EnvironmentIDs: []string{"production"}, Status: "current", ObservedAt: now, AuthorID: "migration-agent"}}, CompatibilityPromises: []capabilityinventories.CompatibilityPromise{{ID: "support", Scope: "released checkout v1", Revision: "policy:1", ConsumerIDs: []string{"storefront"}, OwnerIDs: []string{"platform-owner"}, Guarantee: "supported until the acknowledged replacement release", Until: timePointer(now.Add(90 * 24 * time.Hour))}}, OwnerIDs: []string{"platform-owner"}, DiscoveryNotes: []string{"runtime extension registry must be inspected"}, ChangeReason: "inventory released legacy behavior before deprecation"}
}

func retirementInput(inventory string, version int64, storefront, partner string, now time.Time) capabilityretirements.Input {
	audiences := []capabilityretirements.Audience{{ID: "storefront-users", Name: "Storefront shoppers", ConsumerIDs: []string{"storefront"}, OwnerIDs: []string{"storefront-owner"}, StopsWorking: "v1 checkout returns unsupported", MigrationPath: "release storefront against checkout v2"}}
	approvals := []capabilityretirements.ApprovalRequirement{{OwnerID: "storefront-owner", Scope: "storefront migration", Deadline: now.Add(14 * 24 * time.Hour)}, {OwnerID: "platform-owner", Scope: "provider replacement and removal", Deadline: now.Add(14 * 24 * time.Hour)}}
	if partner != "" {
		audiences = append(audiences, capabilityretirements.Audience{ID: "partner-users", Name: "Partner extension users", ConsumerIDs: []string{"partner-extension"}, OwnerIDs: []string{"partner-owner"}, StopsWorking: "extension v1 calls fail", MigrationPath: "independent owner reviews and releases the v2 agent migration"})
		approvals = append(approvals, capabilityretirements.ApprovalRequirement{OwnerID: "partner-owner", Scope: "partner migration", Deadline: now.Add(14 * 24 * time.Hour)})
	}
	return capabilityretirements.Input{InventoryID: inventory, InventoryVersion: version, Rationale: "v2 is idempotent and v1 collection is obsolete", Replacements: []capabilityretirements.Replacement{{ID: "checkout-v2", Kind: "interface", Reference: "POST /v2/checkout", Revision: "openapi:v2", MigrationGuide: "docs/migrate-checkout-v2.md", OwnerIDs: []string{"platform-owner"}}}, Audiences: audiences, Stages: []capabilityretirements.Stage{{ID: "disable", Name: "Warn then disable", StartsAt: now.Add(30 * 24 * time.Hour), EndsAt: now.Add(45 * 24 * time.Hour), Compatibility: "v1 remains restorable", ExitCriteria: []string{"all owners acknowledge", "zero current v1 use"}}, {ID: "remove", Name: "Remove compatibility machinery", StartsAt: now.Add(45 * 24 * time.Hour), EndsAt: now.Add(60 * 24 * time.Hour), Compatibility: "v1 code, schema, configuration, documentation, and collection removed", ExitCriteria: []string{"governed cleanup verified"}, RollbackTo: "disable"}}, RemovalDeadline: now.Add(60 * 24 * time.Hour), SuccessCriteria: []string{"supported checkout remains healthy", "legacy use is zero"}, RollbackCriteria: []string{"checkout health regresses", "a consumer remains"}, Communication: capabilityretirements.CommunicationPolicy{OwnerID: "platform-owner", Channels: []string{"release notes", "owner inbox"}, Cadence: "weekly", NoticePeriodDays: 30, Escalation: "pause and restore before deletion"}, RequiredApprovals: approvals, Assumptions: []string{"runtime inventory is complete"}}
}

func createCapabilityTask(t *testing.T, server, base, token string, plan capabilityretirements.Plan, repo, ownerKind, owner, title string, dependencies []string) capabilityretirements.MigrationTask {
	t.Helper()
	var updated capabilityretirements.Plan
	workflowValue(t, server, http.MethodPost, base+"/capability-retirements/"+plan.ID+"/tasks", token, capabilityretirements.MigrationTaskInput{RepositoryID: repo, OwnerKind: ownerKind, OwnerID: owner, Title: title, OldContract: capabilityretirements.ContractReference{Kind: "interface", Reference: "POST /v1/checkout", Revision: "openapi:v1"}, ReplacementContract: capabilityretirements.ContractReference{Kind: "interface", Reference: "POST /v2/checkout", Revision: "openapi:v2"}, AcceptanceCriteria: []string{"dual-support checks pass", "independent owner review and release complete"}, DependsOn: dependencies, DocumentationChanges: []string{"docs/checkout.md"}, RolloutStageID: "disable"}, http.StatusCreated, &updated)
	return updated.Tasks[len(updated.Tasks)-1]
}

func addRemovalEvidence(t *testing.T, server, base, token string, removal *capabilityremovals.Removal, stage string, kinds []string, revision string) {
	t.Helper()
	for _, kind := range kinds {
		workflowValue(t, server, http.MethodPost, base+"/capability-removals/"+removal.ID+"/evidence", token, map[string]any{"expected_revision": removal.Revision, "stage_id": stage, "kind": kind, "resource_id": kind + ":" + revision, "revision": revision, "status": "passed", "reference": "repository-visible:" + kind + ":" + revision}, http.StatusCreated, removal)
	}
}

func inventoryHasGap(x capabilityinventories.Inventory, kind string) bool {
	for _, g := range x.Gaps {
		if g.Kind == kind {
			return true
		}
	}
	return false
}
func retirementHasBlocker(x capabilityretirements.Plan, kind string) bool {
	for _, b := range x.Blockers {
		if b.Kind == kind {
			return true
		}
	}
	return false
}
func removalHasBlocker(x capabilityremovals.Removal, kind string) bool {
	for _, b := range x.Blockers {
		if b.Kind == kind {
			return true
		}
	}
	return false
}
func timePointer(x time.Time) *time.Time { return &x }
