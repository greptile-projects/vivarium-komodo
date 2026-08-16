package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryresponses"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestControlledRecoveryResponseWorkflow(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("commander", repositories.Metadata{Name: "live-recovery", Visibility: repositories.Private})
	for _, id := range []string{"approver", "operator", "agent"} {
		_, _ = catalog.AddCollaborator("commander", repo.ID, id)
	}
	commander := issueAccess(t, credentials, "commander", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	approver := issueAccess(t, credentials, "approver", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	operator := issueAccess(t, credentials, "operator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	agent := issueAccess(t, credentials, "agent", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	plans, _ := protectionplans.New(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	plan, _ := plans.Create(string(repo.ID), "commander", protectionplans.VersionInput{ObjectiveID: "objective", ObjectiveVersion: 1, ResourceIDs: []string{"database", "git"}, EnvironmentID: "production", Mode: "snapshot", Schedule: "hourly", MaximumAgeSeconds: 7200, Encryption: "AES-256-GCM", KeyReference: "kms:recovery", AccessScope: []string{"response"}, Destinations: []protectionplans.Destination{{ID: "vault", Kind: "object_store", Region: "eu", Jurisdiction: "EU", Authorized: true}}, Retention: "1y", ChecksumAlgorithm: "sha256", ValidationCriteria: []string{"coherent state"}, CostLimit: 20, Currency: "USD", ChangeReason: "live recovery"})
	plan, _ = plans.Capture(string(repo.ID), plan.ID, "operator", protectionplans.CaptureInput{IdempotencyKey: "capture", PlanVersion: 1, StartedAt: now.Add(-time.Minute), CapturedAt: now, Resources: []protectionplans.ManifestResource{{ResourceID: "database", SourceVersion: "lsn-42", Provenance: "primary@lsn-42", DependencyVersions: map[string]string{"keys": "v3"}, ObjectCount: 10, ByteCount: 1000, Checksum: "sha256:db", Complete: true, SourceState: "committed"}, {ResourceID: "git", SourceVersion: "commit-a", Provenance: "main@commit-a", DependencyVersions: map[string]string{"database": "lsn-42"}, ObjectCount: 20, ByteCount: 2000, Checksum: "sha256:git", Complete: true, SourceState: "committed"}}, Validation: protectionplans.Validation{CompletenessVerified: true, ChecksumVerified: true, DecryptionVerified: true, KeyAvailable: true, DestinationAuthorized: true, ValidatedAt: now, EvidenceDigest: "sha256:verified"}, Cost: 2})
	responses, _ := recoveryresponses.New(t.TempDir(), plans)
	mux := http.NewServeMux()
	registerRecoveryResponsesHTTP(mux, responses, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/recovery-responses"
	activate := recoveryresponses.ActivationInput{IdempotencyKey: "incident-77-response", TriggerKind: "incident", TriggerID: "incident-77", LossConfirmed: true, Summary: "primary and collaboration writes are incoherent", PlanID: plan.ID, PlanVersion: 1, CaptureID: plan.LatestRecoverableCaptureID, WorkspaceID: "workspace-response-77", EnvironmentID: "production", EstimatedLoss: "up to 4 minutes after lsn-42", ApproverIDs: []string{"approver"}, CommunicationChannels: []string{"status-page", "incident-room"}, RollbackOptions: []string{"isolate restored database and return traffic to maintenance mode"}, Steps: []recoveryresponses.Step{{ID: "database", Title: "restore database", ResourceID: "database", ExecutorKind: "agent", ExecutorID: "agent", Command: "restore-db --to lsn-42", Expected: "checksums and lsn match"}, {ID: "cutover", Title: "cut over authoritative writes", ResourceID: "git", DependsOn: []string{"database"}, ExecutorKind: "human", ExecutorID: "operator", Command: "enable-writes --expected lsn-42", Expected: "one authoritative writer", Destructive: true}}}
	b, _ := json.Marshal(activate)
	var response recoveryresponses.Response
	workflowJSON(t, server.URL, http.MethodPost, base, commander, string(b), 201, &response)
	if response.State != "awaiting_approval" || response.WorkspaceID != "workspace-response-77" || response.EstimatedLoss == "" {
		t.Fatalf("public control record incomplete: %+v", response)
	}
	b, _ = json.Marshal(recoveryresponses.DecisionInput{ExpectedRevision: response.Revision, Kind: "approve", Rationale: "verified point is within declared loss tolerance"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/approvals", approver, string(b), 201, &response)
	if response.State != "active" || len(response.NextStepIDs) != 1 || response.NextStepIDs[0] != "database" {
		t.Fatalf("approval did not open dependency order: %+v", response)
	}
	b, _ = json.Marshal(recoveryresponses.StepUpdate{ExpectedRevision: response.Revision, Status: "succeeded", Summary: "isolated restore matched", EvidenceDigest: "sha256:db-result", KeyAvailable: true, ReplicaCurrent: true})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/steps/database", agent, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.StepUpdate{ExpectedRevision: response.Revision, Status: "succeeded", Summary: "write owner switched", EvidenceDigest: "sha256:cutover", KeyAvailable: true, ReplicaCurrent: true})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/steps/cutover", operator, string(b), 409, nil)
	b, _ = json.Marshal(recoveryresponses.DecisionInput{ExpectedRevision: response.Revision, Kind: "approve_cutover", Rationale: "dependencies match and conflicting writes are fenced"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/decisions", commander, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.StepUpdate{ExpectedRevision: response.Revision, Status: "succeeded", Summary: "write owner switched", EvidenceDigest: "sha256:cutover", KeyAvailable: true, ReplicaCurrent: true})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/steps/cutover", operator, string(b), 201, &response)
	if response.State != "validating" || !containsResponseBlocker(response.Blockers, "validation_required") {
		t.Fatalf("cutover returned users before validation: %+v", response)
	}
	b, _ = json.Marshal(recoveryresponses.CommunicationInput{ExpectedRevision: response.Revision, Audience: "public", Message: "Restoration complete; validation remains in progress."})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/communications", commander, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.ValidationInput{ExpectedRevision: response.Revision, Passed: true, EvidenceDigest: "sha256:journeys", Summary: "reads, writes, and Git collaboration are coherent"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/validations", approver, string(b), 201, &response)
	if response.State != "restored" || response.ProgressCompleted != 2 || len(response.Communications) != 1 || len(response.Decisions) != 1 {
		t.Fatalf("trusted return not attributable: %+v", response)
	}

	unsafe := activate
	unsafe.IdempotencyKey = "incident-78-response"
	unsafe.TriggerID = "incident-78"
	unsafe.Steps = []recoveryresponses.Step{{ID: "database", Title: "restore database", ResourceID: "database", ExecutorKind: "agent", ExecutorID: "agent", Command: "restore", Expected: "current replica"}}
	b, _ = json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, commander, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.DecisionInput{ExpectedRevision: response.Revision, Kind: "approve", Rationale: "begin isolated restore"})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/approvals", approver, string(b), 201, &response)
	b, _ = json.Marshal(recoveryresponses.StepUpdate{ExpectedRevision: response.Revision, Status: "succeeded", Summary: "replica lag detected", EvidenceDigest: "sha256:lag", KeyAvailable: true, ReplicaCurrent: false})
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+response.ID+"/steps/database", agent, string(b), 201, &response)
	if response.State != "paused" || !containsResponseBlocker(response.Blockers, "restoration_step_failed") {
		t.Fatalf("stale replica did not pause safely: %+v", response)
	}
}

func containsResponseBlocker(v []string, want string) bool {
	for _, x := range v {
		if x == want {
			return true
		}
	}
	return false
}
