package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRecoveryExercisesRetainBoundedEvidenceAndBecomeStale(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "rehearsal", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	plans, _ := protectionplans.New(t.TempDir())
	planInput := protectionplans.VersionInput{ObjectiveID: "objective", ObjectiveVersion: 1, ResourceIDs: []string{"git"}, EnvironmentID: "production", Mode: "snapshot", Schedule: "hourly", MaximumAgeSeconds: 7200, Encryption: "AES-256-GCM", KeyReference: "kms:key", AccessScope: []string{"recovery"}, Destinations: []protectionplans.Destination{{ID: "vault", Kind: "object_store", Region: "eu", Jurisdiction: "EU", Authorized: true}}, Retention: "1y", ChecksumAlgorithm: "sha256", ValidationCriteria: []string{"refs resolve"}, CostLimit: 20, Currency: "USD", ChangeReason: "initial"}
	plan, _ := plans.Create(string(repo.ID), "owner", planInput)
	now := time.Now().UTC().Truncate(time.Second)
	plan, _ = plans.Capture(string(repo.ID), plan.ID, "owner", protectionplans.CaptureInput{IdempotencyKey: "capture-1", PlanVersion: 1, StartedAt: now.Add(-time.Minute), CapturedAt: now, Resources: []protectionplans.ManifestResource{{ResourceID: "git", SourceVersion: "commit-abc", Provenance: "main@commit-abc", DependencyVersions: map[string]string{"object-store": "v2"}, ObjectCount: 4, ByteCount: 100, Checksum: "sha256:data", Complete: true, SourceState: "committed"}}, Validation: protectionplans.Validation{CompletenessVerified: true, ChecksumVerified: true, DecryptionVerified: true, KeyAvailable: true, DestinationAuthorized: true, ValidatedAt: now, EvidenceDigest: "sha256:validation"}, Cost: 2})
	exercises, _ := recoveryexercises.New(t.TempDir(), plans)
	mux := http.NewServeMux()
	registerRecoveryExercisesHTTP(mux, exercises, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/recovery-exercises"
	launch := recoveryexercises.LaunchInput{IdempotencyKey: "exercise-1", Scenario: "regional object store loss", FailureModes: []string{"primary unavailable"}, PlanID: plan.ID, PlanVersion: 1, CaptureID: plan.Captures[0].ID, Resources: []recoveryexercises.SelectedResource{{ResourceID: "git", SourceVersion: "commit-abc", DependencyVersions: map[string]string{"object-store": "v2"}}}, EnvironmentID: "isolated-recovery-42", IsolationKind: "ephemeral_networkless", MaximumDurationSeconds: 3600, MaximumCost: 10, Steps: []recoveryexercises.Step{{ID: "restore-git", Kind: "restore", ResourceID: "git", Command: "restore --manifest sha256:data", Expected: "refs available"}, {ID: "start", Kind: "dependency", DependsOn: []string{"restore-git"}, Command: "start --isolated", Expected: "healthy"}}, Checks: []recoveryexercises.Check{{ID: "fsck", Kind: "integrity", Command: "git fsck --full", Expected: "clean", ObjectiveResourceIDs: []string{"git"}}, {ID: "clone", Kind: "user_journey", Command: "test-clone", Expected: "clone succeeds", ObjectiveResourceIDs: []string{"git"}}}}
	b, _ := json.Marshal(launch)
	var exercise recoveryexercises.Exercise
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), 201, &exercise)
	if !exercise.Current || exercise.Status != "planned" || exercise.LaunchedBy != "owner" {
		t.Fatalf("launch projection: %+v", exercise)
	}
	unsafe := launch
	unsafe.IdempotencyKey = "unsafe"
	unsafe.ProductionSecretsAvailable = true
	b, _ = json.Marshal(unsafe)
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(b), 422, nil)
	result := recoveryexercises.ResultInput{ExpectedRevision: 1, StartedAt: now.Add(time.Minute), FinishedAt: now.Add(6 * time.Minute), Cost: 3, Summary: "clean restore passed", StepResults: []recoveryexercises.StepResult{{StepID: "restore-git", StartedAt: now.Add(time.Minute), FinishedAt: now.Add(2 * time.Minute), Status: "passed", Command: "restore --manifest sha256:data", LogExcerpt: "restored four objects", LogDigest: "sha256:restore-log", Redacted: true, Artifacts: []recoveryexercises.Artifact{{Name: "restore-report", Kind: "report", Digest: "sha256:report", SizeBytes: 40}}}, {StepID: "start", StartedAt: now.Add(2 * time.Minute), FinishedAt: now.Add(3 * time.Minute), Status: "passed", Command: "start --isolated", LogDigest: "sha256:start-log", Redacted: true, ManualSteps: []string{"confirmed isolated DNS"}}}, CheckResults: []recoveryexercises.CheckResult{{CheckID: "fsck", Status: "passed", StartedAt: now.Add(3 * time.Minute), FinishedAt: now.Add(4 * time.Minute), Command: "git fsck --full", LogDigest: "sha256:fsck", Redacted: true, AchievedObjectiveResourceIDs: []string{"git"}}, {CheckID: "clone", Status: "passed", StartedAt: now.Add(4 * time.Minute), FinishedAt: now.Add(5 * time.Minute), Command: "test-clone", LogDigest: "sha256:clone", Redacted: true, AchievedObjectiveResourceIDs: []string{"git"}}}}
	b, _ = json.Marshal(result)
	workflowJSON(t, server.URL, http.MethodPost, base+"/"+exercise.ID+"/result", owner, string(b), 201, &exercise)
	if exercise.Status != "passed" || exercise.DurationSeconds != 300 || len(exercise.AchievedObjectiveResourceIDs) != 1 || exercise.Result.ActorID != "owner" {
		t.Fatalf("result projection: %+v", exercise)
	}
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(b), 401, nil)
	planInput.ChangeReason = "rotate destination dependency"
	_, _ = plans.Revise(string(repo.ID), plan.ID, "owner", 1, planInput)
	var listed struct {
		Items []recoveryexercises.Exercise `json:"items"`
	}
	workflowJSON(t, server.URL, http.MethodGet, base, reader, "", 200, &listed)
	if len(listed.Items) != 1 || listed.Items[0].Current || !containsFailure(listed.Items[0].Blockers, "protection_plan_changed") {
		t.Fatalf("changed plan must stale exercise: %+v", listed.Items)
	}
}
