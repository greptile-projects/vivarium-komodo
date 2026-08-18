package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designsystems"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDesignSystemPublicAPIAndAuthorization(t *testing.T) {
	git, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), git)
	credentials, _ := auth.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "product", Visibility: repositories.Public})
	_, _ = catalog.AddCollaborator("owner", repo.ID, "reader")
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	reader := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	store, _ := designsystems.New(t.TempDir())
	mux := http.NewServeMux()
	registerDesignSystemsHTTP(mux, store, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := "/repositories/" + string(repo.ID) + "/design-systems"
	in := designsystems.Input{Name: "Core", Description: "UI language", SourceRevision: "abc", DefinitionPath: "ui/design.json", ReleaseRevision: "r1", Tokens: []designsystems.Token{{Name: "space.1", Category: "spacing", Value: "4px", Description: "Base space"}}, Components: []designsystems.Component{{Name: "Button", Purpose: "Act", Usage: "One primary action"}}, Patterns: []designsystems.Pattern{{Name: "Submit", Trigger: "submit", Behavior: "wait", Feedback: "status"}}, ContentRules: []designsystems.ContentRule{{Name: "Labels", Guidance: "verbs", Example: "Save"}}, ResponsiveRules: []designsystems.ResponsiveRule{{Name: "mobile", MinimumWidth: 0, Behavior: "stack"}}, Themes: []designsystems.Theme{{Name: "light", Purpose: "default", TokenOverrides: map[string]string{}}}, Examples: []designsystems.Example{{Name: "Save", Subject: "Button", Markup: "<button>Save</button>", Theme: "light", Locale: "en", Viewport: "mobile"}}, Accessibility: []designsystems.Constraint{{Subject: "Button", Requirement: "focus visible"}}, Localization: []designsystems.Constraint{{Subject: "Labels", Requirement: "expand"}}, Adoption: designsystems.AdoptionPolicy{Required: true}, ChangeReason: "reviewed"}
	body, _ := json.Marshal(in)
	workflowJSON(t, server.URL, http.MethodPost, base, reader, string(body), http.StatusUnauthorized, nil)
	var created designsystems.System
	workflowJSON(t, server.URL, http.MethodPost, base, owner, string(body), http.StatusCreated, &created)
	if created.Versions[0].AuthorID != "owner" || len(created.Gaps) != 2 {
		t.Fatalf("unexpected system: %#v", created)
	}
	var list designsystems.Catalog
	workflowJSON(t, server.URL, http.MethodGet, base, "", "", http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("public catalog unavailable: %#v", list)
	}
}
