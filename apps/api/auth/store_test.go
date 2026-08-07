package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGrantLifecycleIsScopedExpiringAndRevocable(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	issued, err := store.Issue("actor-1", "automation", API, []Scope{ProfileRead}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(issued.Token, TokenPrefix) {
		t.Fatalf("token = %q", issued.Token)
	}
	if _, err := store.Authenticate(issued.Token, ProfileWrite); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("write authentication error = %v", err)
	}
	grant, err := store.Authenticate(issued.Token, ProfileRead)
	if err != nil || grant.LastUsedAt == nil {
		t.Fatalf("read authentication = %#v, %v", grant, err)
	}
	data, err := os.ReadFile(filepath.Join(store.root, "grants", grant.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), issued.Token) {
		t.Fatal("stored grant contains bearer secret")
	}
	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["digest"] == nil {
		t.Fatal("stored grant has no digest")
	}
	now = now.Add(2 * time.Hour)
	if _, err := store.Authenticate(issued.Token, ProfileRead); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired authentication error = %v", err)
	}

	now = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	second, err := store.Issue("actor-1", "replacement", API, []Scope{ProfileRead}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Revoke("actor-1", second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(second.Token, ProfileRead); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked authentication error = %v", err)
	}
}

func TestKindPoliciesLimitAuthorityAndLifetime(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		kind     Kind
		scopes   []Scope
		lifetime time.Duration
	}{
		{"git cannot manage access", Git, []Scope{AccessManage}, time.Hour},
		{"api cannot read git", API, []Scope{GitRead}, time.Hour},
		{"git limited to thirty days", Git, []Scope{GitRead}, 31 * 24 * time.Hour},
		{"api limited to ninety days", API, []Scope{ProfileRead}, 91 * 24 * time.Hour},
		{"duplicate scopes", API, []Scope{ProfileRead, ProfileRead}, time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.Issue("actor", "test", test.kind, test.scopes, test.lifetime); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPasswordsAreHashedAndVerified(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	password := "correct horse battery staple"
	if err := store.SetPassword("actor", password); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.root, "passwords", "actor.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), password) {
		t.Fatal("stored password contains plaintext")
	}
	if err := store.VerifyPassword("actor", password); err != nil {
		t.Fatal(err)
	}
	if err := store.VerifyPassword("actor", "incorrect password"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("incorrect password error = %v", err)
	}
}
