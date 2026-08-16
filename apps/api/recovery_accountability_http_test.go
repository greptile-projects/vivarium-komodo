package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRecoveryWeaknessBecomesAccountableWorkAndFreshExercise(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "continuity-work", Visibility: repositories.Private})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	plans, _ := protectionplans.New(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	planInput := protectionplans.VersionInput{ObjectiveID: "objective", ObjectiveVersion: 1, ResourceIDs: []string{"git"}, EnvironmentID: "production", Mode: "snapshot", Schedule: "hourly", MaximumAgeSeconds: 7200, Encryption: "AES-256-GCM", KeyReference: "kms:key", AccessScope: []string{"recovery"}, Destinations: []protectionplans.Destination{{ID: "vault", Kind: "object_store", Region: "eu", Jurisdiction: "EU", Authorized: true}}, Retention: "1y", ChecksumAlgorithm: "sha256", ValidationCriteria: []string{"refs resolve"}, CostLimit: 20, Currency: "USD", ChangeReason: "initial"}
	plan, _ := plans.Create(string(repo.ID), "owner", planInput)
	plan, _ = plans.Capture(string(repo.ID), plan.ID, "owner", recoveryCapture(now, "capture-1", 1, "commit-a"))
	exercises, _ := recoveryexercises.New(t.TempDir(), plans)
	failed, _ := exercises.Launch(string(repo.ID), "owner", recoveryLaunch("failed", plan, "commit-a"))
	failed, _ = exercises.Record(string(repo.ID), failed.ID, "owner", recoveryResult(now, failed, false))
	investigations, _ := recoveryinvestigations.New(t.TempDir())
	improvements, _ := recoveryimprovements.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	mux := http.NewServeMux()
	registerRecoveryInvestigationsHTTP(mux, investigations, exercises, catalog, credentials)
	registerRecoveryImprovementsHTTP(mux, improvements, investigations, exercises, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	invBase := "/repositories/" + string(repo.ID) + "/recovery-investigations"
	create := recoveryinvestigations.CreateInput{ExerciseID: failed.ID, ExerciseRevision: failed.Revision, Title: "Restore missed packed objects", Question: "Why did clone fail?", ResourceIDs: []string{"git"}, Evidence: []recoveryinvestigations.Evidence{{Kind: "exercise", ResourceID: failed.ID, Revision: "revision:2", Summary: "isolated clone check failed", Audience: "repository"}, {Kind: "code", ResourceID: string(repo.ID), Revision: "commit-a", Path: "backup/config.go", Summary: "pack inclusion excludes recent objects", Audience: "repository"}, {Kind: "dependency", ResourceID: "object-store", Revision: "v2", Summary: "manifest remained available", Audience: "repository", Uncertainty: "eventual consistency window not reproduced"}}}
	b, _ := json.Marshal(create)
	var inv recoveryinvestigations.Investigation
	workflowJSON(t, server.URL, http.MethodPost, invBase, reader, string(b), 201, &inv)
	findingInput := recoveryinvestigations.Finding{Kind: "conclusion", Statement: "The capture policy omitted recent packed objects.", Citations: []recoveryinvestigations.Citation{{EvidenceID: inv.Evidence[0].ID}, {EvidenceID: inv.Evidence[1].ID}}, Uncertainty: "dependency timing remains bounded but unproven", Verdict: "supported"}
	b, _ = json.Marshal(findingInput)
	workflowJSON(t, server.URL, http.MethodPost, invBase+"/"+inv.ID+"/findings", reader, string(b), 201, &inv)
	if len(inv.Findings) != 1 || inv.Findings[0].ActorID != "reader" {
		t.Fatalf("cited reader finding missing: %+v", inv)
	}
	impBase := "/repositories/" + string(repo.ID) + "/recovery-improvements"
	createWork := recoveryimprovements.CreateInput{InvestigationID: inv.ID, FindingID: inv.Findings[0].ID, Title: "Include packed objects in captures", BaseRevision: "commit-a", Tasks: []recoveryimprovements.TaskInput{{Title: "Repair capture selection", OwnerKind: "agent", OwnerID: "approved-agent", ContextKind: "workspace", ContextID: "workspace-7", AcceptanceCriteria: []string{"all reachable objects are captured"}}, {Title: "Review continuity policy", OwnerKind: "human", OwnerID: "owner", DependsOn: []int{1}, AcceptanceCriteria: []string{"policy and check pass ordinary review"}}}}
	b, _ = json.Marshal(createWork)
	workflowJSON(t, server.URL, http.MethodPost, impBase, reader, string(b), 401, nil)
	var created struct {
		Improvement recoveryimprovements.Improvement `json:"improvement"`
		Tasks       []string                         `json:"tasks"`
	}
	workflowJSON(t, server.URL, http.MethodPost, impBase, owner, string(b), 201, &created)
	if len(created.Tasks) != 2 || created.Improvement.State != "planned" {
		t.Fatalf("accountable tasks missing: %+v", created)
	}
	link := recoveryimprovements.Link{Kind: "pull_request", ResourceID: "pull-42", Revision: "commit-b", TaskID: created.Tasks[0], Summary: "ordinary review and checks accepted the repaired capture selection"}
	b, _ = json.Marshal(link)
	workflowJSON(t, server.URL, http.MethodPost, impBase+"/"+created.Improvement.ID+"/links", owner, string(b), 201, &created.Improvement)
	planInput.ChangeReason = "include recent packed objects"
	var err error
	plan, err = plans.Revise(string(repo.ID), plan.ID, "owner", 1, planInput)
	if err != nil {
		t.Fatal(err)
	}
	plan, err = plans.Capture(string(repo.ID), plan.ID, "owner", recoveryCapture(now, "capture-2", 2, "commit-b"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.LatestRecoverableCaptureID == "" {
		t.Fatalf("new capture not recoverable: %+v", plan)
	}
	fresh, err := exercises.Launch(string(repo.ID), "owner", recoveryLaunch("fresh", plan, "commit-b"))
	if err != nil {
		t.Fatal(err)
	}
	fresh, err = exercises.Record(string(repo.ID), fresh.ID, "owner", recoveryResult(now, fresh, true))
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(map[string]string{"exercise_id": fresh.ID})
	workflowJSON(t, server.URL, http.MethodPost, impBase+"/"+created.Improvement.ID+"/verification", owner, string(b), 201, &created.Improvement)
	if created.Improvement.State != "verified" || created.Improvement.VerificationExerciseID != fresh.ID || len(created.Improvement.Blockers) != 0 {
		t.Fatalf("fresh exercise did not verify work: %+v", created.Improvement)
	}
}

func recoveryCapture(now time.Time, key string, version int64, source string) protectionplans.CaptureInput {
	return protectionplans.CaptureInput{IdempotencyKey: key, PlanVersion: version, StartedAt: now.Add(-time.Minute), CapturedAt: now, Resources: []protectionplans.ManifestResource{{ResourceID: "git", SourceVersion: source, Provenance: "main@" + source, DependencyVersions: map[string]string{"object-store": "v2"}, ObjectCount: 4, ByteCount: 100, Checksum: "sha256:data", Complete: true, SourceState: "committed"}}, Validation: protectionplans.Validation{CompletenessVerified: true, ChecksumVerified: true, DecryptionVerified: true, KeyAvailable: true, DestinationAuthorized: true, ValidatedAt: now, EvidenceDigest: "sha256:validation"}, Cost: 2}
}
func recoveryLaunch(key string, plan protectionplans.Plan, source string) recoveryexercises.LaunchInput {
	return recoveryexercises.LaunchInput{IdempotencyKey: key, Scenario: "object loss", FailureModes: []string{"recent pack unavailable"}, PlanID: plan.ID, PlanVersion: plan.CurrentVersion, CaptureID: plan.Captures[len(plan.Captures)-1].ID, Resources: []recoveryexercises.SelectedResource{{ResourceID: "git", SourceVersion: source, DependencyVersions: map[string]string{"object-store": "v2"}}}, EnvironmentID: "isolated", IsolationKind: "ephemeral_networkless", MaximumDurationSeconds: 3600, MaximumCost: 10, Steps: []recoveryexercises.Step{{ID: "restore", Kind: "restore", ResourceID: "git", Command: "restore", Expected: "refs available"}}, Checks: []recoveryexercises.Check{{ID: "clone", Kind: "user_journey", Command: "test-clone", Expected: "clone succeeds", ObjectiveResourceIDs: []string{"git"}}}}
}
func recoveryResult(now time.Time, ex recoveryexercises.Exercise, passed bool) recoveryexercises.ResultInput {
	status, gaps := "failed", []string{"packed objects missing"}
	if passed {
		status, gaps = "passed", nil
	}
	return recoveryexercises.ResultInput{ExpectedRevision: ex.Revision, StartedAt: now, FinishedAt: now.Add(5 * time.Minute), Cost: 3, Summary: "bounded isolated restore", StepResults: []recoveryexercises.StepResult{{StepID: "restore", StartedAt: now, FinishedAt: now.Add(2 * time.Minute), Status: status, Command: "restore", LogDigest: "sha256:restore", Redacted: true, Gaps: gaps}}, CheckResults: []recoveryexercises.CheckResult{{CheckID: "clone", Status: status, StartedAt: now.Add(2 * time.Minute), FinishedAt: now.Add(4 * time.Minute), Command: "test-clone", LogDigest: "sha256:clone", Redacted: true, AchievedObjectiveResourceIDs: func() []string {
		if passed {
			return []string{"git"}
		}
		return nil
	}(), Gaps: gaps}}}
}
