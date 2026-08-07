package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestAuthenticatedOwnerCreatesDiscoversUsesAndRemovesRepository(t *testing.T) {
	requireGit(t)
	gitStorage, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := repositories.New(t.TempDir(), gitStorage)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	userStore, err := users.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, userStore, credentials)
	registerAuthHTTP(mux, credentials, userStore)
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	response, err := http.Post(server.URL+"/users", "application/json", strings.NewReader(`{"handle":"owner","display_name":"Repository Owner","password":"correct horse battery staple"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("account creation status = %d", response.StatusCode)
	}
	response, err = http.Post(server.URL+"/sessions", "application/json", strings.NewReader(`{"handle":"owner","password":"correct horse battery staple"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("session creation status = %d", response.StatusCode)
	}
	session := response.Cookies()[0]

	request, _ := http.NewRequest(http.MethodPost, server.URL+"/access-grants", strings.NewReader(`{"name":"Git remote","kind":"git","scopes":["git:read","git:write"],"expires_in_hours":1}`))
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("Git grant status = %d", response.StatusCode)
	}
	var gitAccess map[string]any
	if err := json.NewDecoder(response.Body).Decode(&gitAccess); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	request, _ = http.NewRequest(http.MethodPost, server.URL+"/repositories", strings.NewReader(`{"name":"project","description":"Consumer-ready repository"}`))
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", response.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	id := created["id"].(string)
	if created["git_url"] != "/repositories/"+id || response.Header.Get("Location") != "/repositories/"+id {
		t.Fatalf("inconsistent identity: %#v", created)
	}
	if created["name"] != "project" || created["api_url"] != "/repositories/"+id {
		t.Fatalf("incomplete resource: %#v", created)
	}

	request, _ = http.NewRequest(http.MethodPatch, server.URL+"/repositories/"+id, strings.NewReader(`{"name":"renamed","visibility":"public"}`))
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var updated map[string]any
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&updated) != nil {
		t.Fatalf("update status = %d", response.StatusCode)
	}
	response.Body.Close()
	if updated["name"] != "renamed" || updated["visibility"] != "public" || updated["description"] != "Consumer-ready repository" {
		t.Fatalf("partial update = %#v", updated)
	}

	request, _ = http.NewRequest(http.MethodGet, server.URL+"/repositories?page=1&per_page=1", nil)
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var collection struct {
		Items   []map[string]any `json:"items"`
		Page    int              `json:"page"`
		PerPage int              `json:"per_page"`
		Total   int              `json:"total_count"`
	}
	if err := json.NewDecoder(response.Body).Decode(&collection); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if len(collection.Items) != 1 || collection.Page != 1 || collection.PerPage != 1 || collection.Total != 1 {
		t.Fatalf("collection = %#v", collection)
	}
	request, _ = http.NewRequest(http.MethodGet, server.URL+"/repositories?per_page=101", nil)
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("invalid pagination status = %d", response.StatusCode)
	}

	remote, _ := url.Parse(server.URL + "/repositories/" + id)
	remote.User = url.UserPassword("git", gitAccess["token"].(string))
	if output := gitLsRemote(t, remote.String(), "--symref", remote.String(), "HEAD"); output != "" {
		t.Fatalf("empty remote advertised %q", output)
	}

	for _, path := range []string{"/repositories", "/repositories/" + id} {
		request, _ = http.NewRequest(http.MethodGet, server.URL+path, nil)
		request.AddCookie(session)
		response, err = http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, response.StatusCode)
		}
	}

	request, _ = http.NewRequest(http.MethodDelete, server.URL+"/repositories/"+id, nil)
	request.AddCookie(session)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", response.StatusCode)
	}
	if _, err := gitStorage.Open(storage.ID(id)); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("repository remained after delete: %v", err)
	}
}
