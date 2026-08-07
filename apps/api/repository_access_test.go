package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestRepositoryAccessIsConsistentAcrossAPIAndGit(t *testing.T) {
	gitStorage, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := repositories.New(t.TempDir(), gitStorage)
	if err != nil {
		t.Fatal(err)
	}
	privateRepository, err := catalog.Create("owner", repositories.Metadata{Name: "private", Visibility: repositories.Private})
	if err != nil {
		t.Fatal(err)
	}
	publicRepository, err := catalog.Create("owner", repositories.Metadata{Name: "public", Visibility: repositories.Public})
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	ownerAPI := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	otherAPI := issueAccess(t, credentials, "other", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	ownerGit := issueAccess(t, credentials, "owner", auth.Git, auth.GitRead, auth.GitWrite)
	otherGit := issueAccess(t, credentials, "other", auth.Git, auth.GitRead, auth.GitWrite)

	mux := http.NewServeMux()
	registerRepositoriesHTTP(mux, catalog, credentials)
	registerGitHTTP(mux, catalog, credentials)

	apiGet := func(repository repositories.Repository, token string) int {
		r := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID), nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	gitRequest := func(repository repositories.Repository, service, token string) int {
		r := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/info/refs?service="+service, nil)
		if token != "" {
			r.SetBasicAuth("git", token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}

	for _, test := range []struct {
		name           string
		api, git, want int
	}{
		{"anonymous private read", apiGet(privateRepository, ""), gitRequest(privateRepository, uploadPackService, ""), http.StatusUnauthorized},
		{"anonymous public read", apiGet(publicRepository, ""), gitRequest(publicRepository, uploadPackService, ""), http.StatusOK},
		{"owner private read", apiGet(privateRepository, ownerAPI), gitRequest(privateRepository, uploadPackService, ownerGit), http.StatusOK},
		{"non-owner private read", apiGet(privateRepository, otherAPI), gitRequest(privateRepository, uploadPackService, otherGit), http.StatusNotFound},
		{"non-owner public read", apiGet(publicRepository, otherAPI), gitRequest(publicRepository, uploadPackService, otherGit), http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.api != test.want || test.git != test.want {
				t.Fatalf("API status %d and Git status %d, want %d", test.api, test.git, test.want)
			}
		})
	}

	if got := gitRequest(publicRepository, receivePackService, ""); got != http.StatusUnauthorized {
		t.Fatalf("anonymous public Git write = %d", got)
	}
	if got := gitRequest(publicRepository, receivePackService, otherGit); got != http.StatusNotFound {
		t.Fatalf("non-owner public Git write = %d", got)
	}
	if got := gitRequest(publicRepository, receivePackService, ownerGit); got != http.StatusOK {
		t.Fatalf("owner public Git write = %d", got)
	}

	patch := func(token, visibility string) int {
		r := httptest.NewRequest(http.MethodPatch, "/repositories/"+string(privateRepository.ID), strings.NewReader(`{"visibility":"`+visibility+`"}`))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	if got := patch(otherAPI, "public"); got != http.StatusNotFound {
		t.Fatalf("non-owner administration = %d", got)
	}
	if got := patch(ownerAPI, "public"); got != http.StatusOK {
		t.Fatalf("owner administration = %d", got)
	}
	if got := apiGet(privateRepository, ""); got != http.StatusOK {
		t.Fatalf("published API read = %d", got)
	}
	if got := gitRequest(privateRepository, uploadPackService, ""); got != http.StatusOK {
		t.Fatalf("published Git read = %d", got)
	}
}

func issueAccess(t *testing.T, store *auth.Store, userID string, kind auth.Kind, scopes ...auth.Scope) string {
	t.Helper()
	issued, err := store.Issue(userID, "test access", kind, scopes, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return issued.Token
}
