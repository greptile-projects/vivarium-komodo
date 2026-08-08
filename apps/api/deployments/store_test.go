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
