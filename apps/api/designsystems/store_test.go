package designsystems

import "testing"

func design(name, token string) Input {
	return Input{Name: name, Description: "Shared interface", SourceRevision: "reviewed-commit", DefinitionPath: "ui/design.json", ReleaseRevision: "release-1", Tokens: []Token{{Name: "color.action", Category: "color", Value: token, Description: "Primary action"}}, Components: []Component{{Name: "Button", Purpose: "Start an action", Usage: "Use for a single primary action"}}, Patterns: []Pattern{{Name: "Save", Trigger: "Submit", Behavior: "Disable while saving", Feedback: "Show success or error", Keyboard: "Enter submits"}}, ContentRules: []ContentRule{{Name: "Button labels", Guidance: "Use a verb", Example: "Save changes"}}, ResponsiveRules: []ResponsiveRule{{Name: "compact", MinimumWidth: 0, MaximumWidth: 639, Behavior: "Stack actions"}}, Themes: []Theme{{Name: "light", Purpose: "Default", TokenOverrides: map[string]string{}}}, Examples: []Example{{Name: "Primary button", Subject: "Button", Markup: "<button>Save changes</button>", Theme: "light", Locale: "en", Viewport: "compact", Description: "Primary save action"}}, Accessibility: []Constraint{{Subject: "Button", Requirement: "Visible focus and accessible name", Evidence: "axe/button"}}, Localization: []Constraint{{Subject: "Button labels", Requirement: "Allow 200% text expansion"}}, OwnerIDs: []string{"design-team"}, Adoption: AdoptionPolicy{Required: true, Consumers: []string{"web"}, Exceptions: "Owner review", ReviewCadence: "each release"}, Consumers: []Consumer{{Name: "web", ImplementationRevision: "reviewed-commit", ReleaseRevision: "release-1", AdoptedVersion: 1, Status: "current"}}, Provenance: []Provenance{{Kind: "pull_request", Reference: "pull-7", Revision: "reviewed-commit", Rationale: "Reviewed implementation"}}, ChangeReason: "Publish shared decisions"}
}
func TestVersionedCatalogDerivesGapsAndConflicts(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Create("repo", "owner", design("Core", "#06f"))
	if e != nil {
		t.Fatal(e)
	}
	next := design("Core", "#07f")
	next.OwnerIDs = nil
	next.Consumers[0].AdoptedVersion = 1
	next.Consumers[0].Status = "stale"
	next.Consumers = append(next.Consumers, Consumer{Name: "legacy", ImplementationRevision: "old", AdoptedVersion: 1, Status: "unsupported", Notes: "CSS variables unavailable"})
	a, e = s.Revise("repo", a.ID, "owner", 1, next)
	if e != nil {
		t.Fatal(e)
	}
	k := map[string]bool{}
	for _, g := range a.Gaps {
		k[g.Kind] = true
	}
	for _, want := range []string{"missing_owner", "stale_implementation", "unsupported_consumer"} {
		if !k[want] {
			t.Fatalf("missing %s: %#v", want, a.Gaps)
		}
	}
	if _, e = s.Revise("repo", a.ID, "owner", 1, next); e != ErrConflict {
		t.Fatalf("expected optimistic conflict, got %v", e)
	}
	_, e = s.Create("repo", "owner", design("Marketing", "#f60"))
	if e != nil {
		t.Fatal(e)
	}
	catalog, e := s.Catalog("repo")
	if e != nil || len(catalog.Items) != 2 || len(catalog.Conflicts) != 1 || catalog.Conflicts[0].Kind != "token" {
		t.Fatalf("catalog: %#v %v", catalog, e)
	}
}
func TestRejectsBrokenExampleAndThemeOverride(t *testing.T) {
	s, _ := New(t.TempDir())
	in := design("Core", "#06f")
	in.Examples[0].Theme = "missing"
	if _, e := s.Create("repo", "owner", in); e != ErrInvalid {
		t.Fatalf("got %v", e)
	}
	in = design("Core", "#06f")
	in.Themes[0].TokenOverrides["unknown"] = "#000"
	if _, e := s.Create("repo", "owner", in); e != ErrInvalid {
		t.Fatalf("got %v", e)
	}
}
