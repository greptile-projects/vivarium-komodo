package previews

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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

func TestInvitationExpiresRevokesAndAuditsWithoutAuthority(t *testing.T) {
	store, _ := New(t.TempDir())
	store.now = func() time.Time { return time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC) }
	item, _ := store.Create(Preview{RepositoryID: "repo", PullRequestID: "pull", Definition: Definition{Resources: Resources{LifetimeMinutes: 60}}})
	invited, err := store.Invite("repo", "pull", item.ID, "owner", Invitation{UserID: "customer", Role: "test", SourceKind: "issue", SourceID: "issue-1", ExpiresAt: store.now().Add(30 * time.Minute)})
	if err != nil || len(invited.Invitations) != 1 || len(invited.AccessEvents) != 1 {
		t.Fatalf("invite = %#v, %v", invited, err)
	}
	_, grant, err := store.Authorize("repo", "pull", item.ID, "customer")
	if err != nil || grant.Role != "test" {
		t.Fatalf("authorize = %#v, %v", grant, err)
	}
	revoked, err := store.Revoke("repo", "pull", item.ID, grant.ID, "owner")
	if err != nil || revoked.Invitations[0].RevokedAt == nil || len(revoked.AccessEvents) != 3 {
		t.Fatalf("revoke = %#v, %v", revoked, err)
	}
	if _, _, err = store.Authorize("repo", "pull", item.ID, "customer"); err == nil {
		t.Fatal("revoked invitation authorized")
	}
}
