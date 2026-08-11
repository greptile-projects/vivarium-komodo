package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestIssueReportingRetainsActionableEvidenceWithoutLeakingPrivateDuplicates(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	reporter, _ := userStore.Create(users.Profile{Handle: "reporter", DisplayName: "Reporter"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "public-project", Visibility: repositories.Public})
	token := issueAccess(t, credentials, string(reporter.ID), auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerIssuesHTTP(mux, issueStore, releaseStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/issues"

	created := createIssueRequest(t, base, token, `{"title":"CLI crashes while cloning","expected_behavior":"Clone completes","observed_behavior":"CLI exits with code 2","severity":"high","environment":"Ubuntu 24.04, git 2.45","reproduction_steps":["Create an empty folder","Run git clone"],"visibility":"public","attachments":[{"kind":"log","name":"clone.log","media_type":"text/plain","content":"c2FuaXRpemVkIGxvZw=="}]}`)
	if created.ReporterID != string(reporter.ID) || created.Status != "open" || len(created.History) != 1 || len(created.Attachments) != 1 {
		t.Fatalf("created issue = %#v", created)
	}
	response, _ := http.Get(base + "/" + created.ID)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("anonymous public issue = %d", response.StatusCode)
	}
	response.Body.Close()

	private := createIssueRequest(t, base, token, `{"title":"CLI crashes with private customer input","expected_behavior":"Clone completes","observed_behavior":"CLI exits with code 2","severity":"high","environment":"customer staging","reproduction_steps":["Use sanitized fixture"],"visibility":"repository","attachments":[{"kind":"sample_input","name":"fixture.json","media_type":"application/json","content":"e30="}]}`)
	response, _ = http.Get(base + "/suggestions?q=CLI%20crashes%20private")
	var suggestions struct {
		Items []issues.Issue `json:"items"`
	}
	json.NewDecoder(response.Body).Decode(&suggestions)
	response.Body.Close()
	for _, item := range suggestions.Items {
		if item.ID == private.ID {
			t.Fatal("private issue leaked through anonymous duplicate suggestions")
		}
	}

	request, _ := http.NewRequest(http.MethodPost, base+"/"+created.ID+"/comments", strings.NewReader(`{"body":"I can reproduce this on a clean machine."}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var discussed issues.Issue
	json.NewDecoder(response.Body).Decode(&discussed)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || len(discussed.Comments) != 1 || len(discussed.History) != 2 {
		t.Fatalf("discussion = %#v status=%d", discussed, response.StatusCode)
	}
}

func createIssueRequest(t *testing.T, url, token, body string) issues.Issue {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create issue status = %d", response.StatusCode)
	}
	var item issues.Issue
	if err := json.NewDecoder(response.Body).Decode(&item); err != nil {
		t.Fatal(err)
	}
	return item
}
