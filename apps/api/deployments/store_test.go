package deployments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGovernedEnvironmentKeepsSecretsProtectedAndAttributesPromotion(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	environment, err := store.PutEnvironment("repo", "", "owner", EnvironmentInput{Name: "Production", Position: 2, Command: "deploy", Configuration: map[string]string{"REGION": "east"}, Secrets: map[string]string{"TOKEN": "super-secret"}, RequiredApprovals: 2, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(environment.SecretNames) != 1 || environment.SecretNames[0] != "TOKEN" {
		t.Fatalf("public secret metadata = %#v", environment)
	}
	public, _ := os.ReadFile(filepath.Join(root, "repo", "environments", environment.ID+".json"))
	encrypted, _ := os.ReadFile(filepath.Join(root, "repo", "credentials", environment.ID+".json"))
	if strings.Contains(string(public), "super-secret") || strings.Contains(string(encrypted), "super-secret") {
		t.Fatal("protected credential was persisted in plaintext")
	}
	d, err := store.Create(CreateDeployment{RepositoryID: "repo", EnvironmentID: environment.ID, ReleaseID: "release", BuildRunID: "run", ArtifactID: "artifact", ArtifactSHA256: "sha", ActorID: "alice"})
	if err != nil || d.State != "pending" || d.Events[0].ActorID != "alice" {
		t.Fatalf("deployment = %#v, %v", d, err)
	}
	if _, err = store.Create(CreateDeployment{RepositoryID: "repo", EnvironmentID: environment.ID, ReleaseID: "other", ActorID: "bob"}); err != ErrConflict {
		t.Fatalf("concurrency error = %v", err)
	}
	d, _ = store.Approve("repo", d.ID, "bob")
	if d.State != "pending" {
		t.Fatalf("one approval state = %s", d.State)
	}
	d, _ = store.Approve("repo", d.ID, "carol")
	if d.State != "queued" || len(d.Approvals) != 2 {
		t.Fatalf("approved deployment = %#v", d)
	}
	d, _ = store.Start("repo", d.ID)
	_ = store.Log("repo", d.ID, "stdout", "deployed\n")
	d, _ = store.Complete("repo", d.ID, true, "done")
	if d.State != "succeeded" || len(d.Events) != 7 || d.Events[1].ActorID != "bob" || d.Events[2].ActorID != "carol" {
		t.Fatalf("timeline = %#v", d)
	}
}

func TestRecoveryDeploymentRetainsFailedDeploymentLink(t *testing.T) {
	store, _ := New(t.TempDir())
	environment, _ := store.PutEnvironment("repo", "", "owner", EnvironmentInput{Name: "production", Position: 1, Command: "deploy", Concurrency: 2})
	item, err := store.Create(CreateDeployment{RepositoryID: "repo", EnvironmentID: environment.ID, ReleaseID: "known-good", BuildRunID: "run", ArtifactID: "artifact", ArtifactSHA256: "sha", SourceCommitID: "commit", ActorID: "collaborator", RecoveryOfID: "failed-deployment", RecoveryAction: "rollback"})
	if err != nil || item.RecoveryOfID != "failed-deployment" || item.RecoveryAction != "rollback" {
		t.Fatalf("recovery deployment = %#v, %v", item, err)
	}
	restored, _ := store.GetDeployment("repo", item.ID)
	if restored.RecoveryOfID != "failed-deployment" || restored.ReleaseID != "known-good" {
		t.Fatalf("restored recovery = %#v", restored)
	}
}

func TestRolloutManifestAndAttributedControlsRetainEvidence(t *testing.T) {
	stages, err := ParseManifest([]byte(`{"version":1,"environments":[{"name":"Production","stages":[{"name":"canary","command":"rollout 10","health":[{"name":"error-rate","command":"check-errors"}]},{"name":"complete","health":[{"name":"availability","command":"check-up","timeout_seconds":30}]}]}]}`), "production")
	if err != nil || len(stages) != 2 || stages[0].Health[0].TimeoutSeconds != 60 {
		t.Fatalf("parsed stages = %#v, %v", stages, err)
	}
	if _, err = ParseManifest([]byte(`{"version":1,"environments":[{"name":"Production","stages":[{"name":"canary","health":[]}]}]}`), "Production"); err != ErrInvalid {
		t.Fatalf("empty health policy error = %v", err)
	}
	store, _ := New(t.TempDir())
	environment, _ := store.PutEnvironment("repo", "", "owner", EnvironmentInput{Name: "Production", Position: 1, Command: "deploy", Concurrency: 1})
	deployment, _ := store.Create(CreateDeployment{RepositoryID: "repo", EnvironmentID: environment.ID, ReleaseID: "release", ActorID: "alice"})
	deployment, _ = store.Start("repo", deployment.ID)
	deployment, _ = store.Stage("repo", deployment.ID, "canary", "health.completed", "error-rate", "passed", "within threshold")
	deployment, _ = store.Control("repo", deployment.ID, "bob", "pause", "Investigating elevated latency")
	if deployment.State != "paused" || deployment.DecisionByID != "bob" || deployment.Events[len(deployment.Events)-1].Message == "" {
		t.Fatalf("paused deployment = %#v", deployment)
	}
	if _, err = store.Create(CreateDeployment{RepositoryID: "repo", EnvironmentID: environment.ID, ReleaseID: "other", ActorID: "carol"}); err != ErrConflict {
		t.Fatalf("paused rollout must retain concurrency slot: %v", err)
	}
	deployment, _ = store.Control("repo", deployment.ID, "carol", "resume", "Signal recovered")
	deployment, _ = store.Control("repo", deployment.ID, "carol", "fail", "Manual verification still failed")
	if deployment.State != "failed" || deployment.CompletedAt == nil || deployment.DecisionReason != "Manual verification still failed" {
		t.Fatalf("failed deployment = %#v", deployment)
	}
}
