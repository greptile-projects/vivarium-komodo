package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestNewcomerProposesAndDiscussesPublicRepositoryWork(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	newcomer, _ := userStore.Create(users.Profile{Handle: "newcomer", DisplayName: "Newcomer"})
	outsider, _ := userStore.Create(users.Profile{Handle: "outsider", DisplayName: "Outsider"})
	repository, err := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	if err != nil {
		t.Fatal(err)
	}
	ownerToken := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	newcomerToken := issueAccess(t, credentials, string(newcomer.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	outsiderToken := issueAccess(t, credentials, string(outsider.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerProposalsHTTP(mux, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/proposals"

	request, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(`{"title":"Document agent setup","body":"New contributors need a shared setup path."}`))
	request.Header.Set("Authorization", "Bearer "+newcomerToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, error = %v", response.StatusCode, err)
	}
	var created proposals.Proposal
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if created.AuthorID != string(newcomer.ID) || created.State != proposals.Open || response.Header.Get("Location") != "/repositories/"+string(repository.ID)+"/proposals/"+created.ID {
		t.Fatalf("created = %#v", created)
	}

	response, _ = http.Get(base + "/" + created.ID)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("anonymous inspect = %d", response.StatusCode)
	}
	response.Body.Close()
	request, _ = http.NewRequest(http.MethodPost, base+"/"+created.ID+"/comments", strings.NewReader(`{"body":"Please include credential setup."}`))
	request.Header.Set("Authorization", "Bearer "+ownerToken)
	response, _ = http.DefaultClient.Do(request)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("comment status = %d", response.StatusCode)
	}
	var comment proposals.Comment
	json.NewDecoder(response.Body).Decode(&comment)
	response.Body.Close()
	if comment.AuthorID != string(owner.ID) {
		t.Fatalf("comment author = %q", comment.AuthorID)
	}

	request, _ = http.NewRequest(http.MethodPatch, base+"/"+created.ID, strings.NewReader(`{"title":"Document complete agent setup","state":"closed"}`))
	request.Header.Set("Authorization", "Bearer "+newcomerToken)
	response, _ = http.DefaultClient.Do(request)
	var closed proposals.Proposal
	json.NewDecoder(response.Body).Decode(&closed)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || closed.Title != "Document complete agent setup" || closed.State != proposals.Closed || closed.ClosedByID != string(newcomer.ID) {
		t.Fatalf("closed = %#v status %d", closed, response.StatusCode)
	}

	request, _ = http.NewRequest(http.MethodPatch, base+"/"+created.ID, strings.NewReader(`{"body":"hijacked"}`))
	request.Header.Set("Authorization", "Bearer "+outsiderToken)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider update = %d", response.StatusCode)
	}
	response, _ = http.Get(base + "?state=closed&per_page=1")
	var listed struct {
		Items []proposals.Proposal `json:"items"`
		Total int                  `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&listed)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || listed.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("list = %#v status %d", listed, response.StatusCode)
	}
	response, _ = http.Get(base + "/" + created.ID + "/comments")
	var discussion struct {
		Items []proposals.Comment `json:"items"`
		Total int                 `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&discussion)
	response.Body.Close()
	if discussion.Total != 1 || discussion.Items[0].AuthorID != string(owner.ID) {
		t.Fatalf("discussion = %#v", discussion)
	}
}

func TestPrivateProposalAccessFollowsRepositoryMembership(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	outsider, _ := userStore.Create(users.Profile{Handle: "outsider", DisplayName: "Outsider"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "private", Visibility: repositories.Private})
	token := issueAccess(t, credentials, string(outsider.ID), auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerProposalsHTTP(mux, proposalStore, catalog, credentials)
	path := "/repositories/" + string(repository.ID) + "/proposals"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous private list = %d", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("outsider private list = %d", w.Code)
	}
}
