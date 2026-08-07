package repositories

import (
	"errors"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestOwnedRepositoryLifecycle(t *testing.T) {
	gitStorage, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(t.TempDir(), gitStorage)
	if err != nil {
		t.Fatal(err)
	}

	first, err := store.Create("owner-one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create("owner-two")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || !first.Empty || first.OwnerID != "owner-one" {
		t.Fatalf("unexpected repository: %#v", first)
	}

	items, err := store.List("owner-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("owner list = %#v", items)
	}
	if _, err := store.Get("owner-two", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get error = %v", err)
	}
	if err := store.Delete("owner-two", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete error = %v", err)
	}

	if err := store.Delete("owner-one", first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := gitStorage.Open(first.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("deleted Git repository error = %v", err)
	}
	if _, err := store.Get("owner-one", first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted catalog error = %v", err)
	}
}
