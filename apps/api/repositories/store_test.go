package repositories

import (
	"errors"
	"slices"
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

	first, err := store.Create("owner-one", Metadata{Name: "first", Visibility: Private})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create("owner-two", Metadata{Name: "second", Visibility: Public})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || !first.Empty || first.OwnerID != "owner-one" || first.Visibility != Private || second.Visibility != Public {
		t.Fatalf("unexpected repository: %#v", first)
	}
	if _, err := store.Create("owner-one", Metadata{Name: "first", Visibility: Private}); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate name error = %v", err)
	}
	if _, err := store.Create("owner-one", Metadata{Name: "bad name", Visibility: Private}); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid metadata error = %v", err)
	}
	updated, err := store.Update("owner-one", first.ID, Metadata{Name: "renamed", Description: "A repository", Visibility: Public})
	if err != nil || updated.Name != "renamed" || updated.Description != "A repository" || updated.Visibility != Public || updated.UpdatedAt.Before(updated.CreatedAt) {
		t.Fatalf("updated repository = %#v, %v", updated, err)
	}
	protected, err := store.SetRequiredChecks("owner-one", first.ID, "main", []string{"test", "lint"})
	if err != nil || !slices.Equal(protected.RequiredChecks["main"], []string{"lint", "test"}) {
		t.Fatalf("required checks = %#v, %v", protected.RequiredChecks, err)
	}
	if _, err := store.SetRequiredChecks("owner-two", first.ID, "main", []string{"lint"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner required checks error = %v", err)
	}
	if _, err := store.SetRequiredChecks("owner-one", first.ID, "bad..branch", []string{"lint"}); !errors.Is(err, ErrInvalidRepository) {
		t.Fatalf("invalid branch error = %v", err)
	}
	reopened, _ := store.Inspect(first.ID)
	if !slices.Equal(reopened.RequiredChecks["main"], []string{"lint", "test"}) {
		t.Fatalf("reopened required checks = %#v", reopened.RequiredChecks)
	}

	items, err := store.List("owner-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("owner list = %#v", items)
	}
	if _, err := store.AddCollaborator("owner-two", second.ID, "owner-one"); err != nil {
		t.Fatal(err)
	}
	accessible, err := store.ListAccessible("owner-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(accessible) != 2 || accessible[0].ID != first.ID || accessible[1].ID != second.ID {
		t.Fatalf("accessible list = %#v", accessible)
	}
	public, err := store.ListPublic("second")
	if err != nil || len(public) != 1 || public[0].ID != second.ID {
		t.Fatalf("public discovery = %#v, %v", public, err)
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
