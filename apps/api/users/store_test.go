package users

import (
	"errors"
	"sync"
	"testing"
)

func TestCreateGetUpdateAndReopenUser(t *testing.T) {
	root := t.TempDir()
	store, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.Create(Profile{Handle: " Ada-Lovelace ", DisplayName: " Ada Lovelace "})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Handle != "ada-lovelace" || created.DisplayName != "Ada Lovelace" {
		t.Fatalf("unexpected user: %#v", created)
	}

	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != created {
		t.Fatalf("reopened user = %#v, want %#v", got, created)
	}

	updated, err := reopened.Update(created.ID, Profile{Handle: "ada", DisplayName: "Augusta Ada King"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.CreatedAt != created.CreatedAt {
		t.Fatal("update changed stable identity")
	}
	if updated.Handle != "ada" || updated.DisplayName != "Augusta Ada King" {
		t.Fatalf("unexpected update: %#v", updated)
	}
}

func TestHandleIsUniqueAcrossCreateAndUpdate(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Create(Profile{Handle: "grace", DisplayName: "Grace Hopper"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(Profile{Handle: "katherine", DisplayName: "Katherine Johnson"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(Profile{Handle: "GRACE", DisplayName: "Someone Else"}); !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("duplicate create error = %v", err)
	}
	if _, err := store.Update(second.ID, Profile{Handle: first.Handle, DisplayName: second.DisplayName}); !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("duplicate update error = %v", err)
	}
}

func TestConcurrentCreateClaimsHandleOnce(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errorsSeen := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Create(Profile{Handle: "shared", DisplayName: "Collaborator"})
			errorsSeen <- err
		}()
	}
	wg.Wait()
	close(errorsSeen)
	successes, conflicts := 0, 0
	for err := range errorsSeen {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrHandleTaken) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestRejectsInvalidProfilesAndIDs(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []Profile{{Handle: "", DisplayName: "Name"}, {Handle: "has spaces", DisplayName: "Name"}, {Handle: "valid", DisplayName: ""}} {
		if _, err := store.Create(profile); !errors.Is(err, ErrInvalidProfile) {
			t.Fatalf("profile %#v error = %v", profile, err)
		}
	}
	if _, err := store.Get("../escape"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("invalid ID error = %v", err)
	}
}
