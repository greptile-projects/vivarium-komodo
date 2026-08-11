package previews

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAttemptRetainsImmutableAttestationAndLifecycle(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	definition := Definition{Version: 1, Start: "serve", Port: 3000, Resources: Resources{CPUSeconds: 60, MemoryMB: 256, DiskMB: 512, BuildTimeoutSeconds: 30, LifetimeMinutes: 15}}
	item, err := store.Create(Preview{RepositoryID: "repo", SourceRepositoryID: "fork", PullRequestID: "pull", Revision: "revision-one", CreatorID: "collaborator", Definition: definition, Configuration: map[string]string{"MODE": "secret-review-value"}, Attestation: Attestation{CommitID: "revision-one", DefinitionDigest: "definition", ConfigurationDigest: "configuration"}})
	if err != nil {
		t.Fatal(err)
	}
	if item.State != "setting_up" || len(item.Events) != 1 || item.ExpiresAt.IsZero() {
		t.Fatalf("created preview = %#v", item)
	}
	if _, err = store.Transition(item.ID, "ready", "/preview", "", 4000); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("repo", "pull", item.ID)
	if err != nil || got.Revision != "revision-one" || got.Attestation.DefinitionDigest != "definition" || got.State != "ready" || got.LocalPort != 4000 {
		t.Fatalf("stored preview = %#v, %v", got, err)
	}
	raw, _ := json.Marshal(got)
	if !json.Valid(raw) || strings.Contains(string(raw), "secret-review-value") {
		t.Fatal("configuration value leaked into durable/public preview")
	}
}
