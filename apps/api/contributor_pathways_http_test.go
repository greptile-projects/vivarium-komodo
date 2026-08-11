package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestContributorPathwayMakesParticipationRequirementsInspectable(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	pathways, _ := contributorpathways.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	proposalStore, _ := proposals.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "welcoming-project", Visibility: repositories.Public})
	opened, _ := catalog.Open(repository.ID)
	doc, _ := opened.WriteObject(storage.BlobObject, []byte("# Contributing"))
	tree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 CONTRIBUTING.md\x00"), objectIDBytes(t, doc)...))
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor Owner <o@example.test> 1 +0000\ncommitter Owner <o@example.test> 1 +0000\n\nDocument contribution\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerContributorPathwaysHTTP(mux, pathways, catalog, credentials, releaseStore, issueStore, proposalStore)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/contributor-pathway"
	body := fmt.Sprintf(`{"expected_version":0,"goals":["Make collaboration approachable"],"prerequisites":["Read the conduct policy"],"conduct_guidance":"Be respectful and challenge ideas, not people.","security_guidance":"Report vulnerabilities privately.","supported_setup":["Use Go 1.22 or later"],"communication_expectations":["Discuss scope before large changes"],"review_policy":["A maintainer reviews every change"],"work_categories":[{"name":"Documentation","description":"Clarify developer workflows.","suitable_for":"human_or_agent","prerequisites":["Run documentation checks"],"review_expectations":"Keep examples revision exact."}],"references":[{"kind":"documentation","label":"Contributor guide","path":"CONTRIBUTING.md","revision":"%s"},{"kind":"ownership","label":"Maintainer","resource_id":"owner"},{"kind":"issue","label":"Starter issue","resource_id":"missing"}],"change_reason":"Initial participation contract"}`, commit)
	request, _ := http.NewRequest(http.MethodPost, base+"/versions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ := http.DefaultClient.Do(request)
	var published contributorpathways.Pathway
	_ = json.NewDecoder(response.Body).Decode(&published)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || published.CurrentVersion != 1 || published.Versions[0].References[0].Status != "current" || published.Versions[0].References[2].Status != "inaccessible" {
		t.Fatalf("published = %#v status %d", published, response.StatusCode)
	}

	next, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor Owner <o@example.test> 2 +0000\ncommitter Owner <o@example.test> 2 +0000\n\nMove main\n", tree, commit)))
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: next})
	response, _ = http.Get(base)
	var inspected contributorpathways.Pathway
	_ = json.NewDecoder(response.Body).Decode(&inspected)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || inspected.Versions[0].References[0].Status != "stale" {
		t.Fatalf("inspected = %#v status %d", inspected, response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodPost, base+"/acknowledgements", strings.NewReader(`{"version":1,"note":"Reviewed before starting."}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	_ = json.NewDecoder(response.Body).Decode(&inspected)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || len(inspected.Acknowledgements) != 1 || inspected.Acknowledgements[0].ActorID != "owner" {
		t.Fatalf("acknowledgements = %#v status %d", inspected.Acknowledgements, response.StatusCode)
	}
	request, _ = http.NewRequest(http.MethodPost, base+"/versions", strings.NewReader(strings.Replace(body, `"expected_version":0`, `"expected_version":0`, 1)))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("stale publication = %d", response.StatusCode)
	}
}
