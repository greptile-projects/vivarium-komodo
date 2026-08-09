package relationships

import "testing"

func TestVersionConstraints(t *testing.T) {
	cases := []struct {
		version, constraint string
		want                bool
	}{
		{"1.4.2", "^1.2.0", true}, {"2.0.0", "^1.2.0", false},
		{"1.4.9", "~1.4.0", true}, {"1.5.0", "~1.4.0", false},
		{"2.0.0", ">=1.9.0", true}, {"1.2.3", "1.2.3", true}, {"9.0.0", "*", true},
	}
	for _, test := range cases {
		if got := Satisfies(test.version, test.constraint); got != test.want {
			t.Errorf("Satisfies(%q,%q)=%v", test.version, test.constraint, got)
		}
	}
}

func TestStoreRetainsPublicationsAndDeclarations(t *testing.T) {
	store, _ := New(t.TempDir())
	pub, err := store.Publish(Interface{RepositoryID: "provider", Name: "payments", Version: "1.2.0", CommitID: "abc", ReleaseID: "release", PublishedByID: "owner"})
	if err != nil || pub.ID == "" {
		t.Fatalf("publish = %#v %v", pub, err)
	}
	if _, err = store.Publish(Interface{RepositoryID: "provider", Name: "payments", Version: "1.2.0", CommitID: "def", ReleaseID: "other", PublishedByID: "owner"}); err != ErrConflict {
		t.Fatalf("duplicate = %v", err)
	}
	dep, err := store.Declare(Dependency{RepositoryID: "consumer", CommitID: "def", ProviderRepositoryID: "provider", InterfaceName: "payments", Constraint: "^1.0.0", DeclaredByID: "consumer-owner"})
	if err != nil || dep.ID == "" {
		t.Fatalf("declare = %#v %v", dep, err)
	}
	interfaces, _ := store.Interfaces()
	dependencies, _ := store.Dependencies()
	if len(interfaces) != 1 || len(dependencies) != 1 {
		t.Fatalf("retained %d %d", len(interfaces), len(dependencies))
	}
}
