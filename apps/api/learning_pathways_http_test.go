package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestLearningPathwayKeepsProjectCurriculumCurrentAndAttributable(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	store, _ := learningpathways.New(t.TempDir())
	decisionStore, _ := decisions.New(t.TempDir())
	issueStore, _ := issues.New(t.TempDir())
	apiStore, _ := apicontracts.New(t.TempDir())
	packageStore, _ := packages.New(t.TempDir())
	contributorStore, _ := contributorpathways.New(t.TempDir())
	repo, _ := catalog.Create("owner", repositories.Metadata{Name: "learnable", Visibility: repositories.Public})
	opened, _ := catalog.Open(repo.ID)
	blob, _ := opened.WriteObject(storage.BlobObject, []byte("# Architecture\n"))
	tree, _ := opened.WriteObject(storage.TreeObject, append([]byte("100644 ARCHITECTURE.md\x00"), objectIDBytes(t, blob)...))
	commit, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nauthor O <o@example.test> 1 +0000\ncommitter O <o@example.test> 1 +0000\n\nTeach architecture\n", tree)))
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	mux := http.NewServeMux()
	registerLearningPathwaysHTTP(mux, store, catalog, credentials, learningPathwaySources{decisions: decisionStore, issues: issueStore, apis: apiStore, packages: packageStore, contributors: contributorStore})
	server := httptest.NewServer(mux)
	defer server.Close()
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	base := server.URL + "/repositories/" + string(repo.ID) + "/learning-pathways/backend/versions"
	body := fmt.Sprintf(`{"expected_version":0,"role":"Backend contributor","outcome":"Safely change request handling","prerequisites":["Read Go"],"objectives":["Trace a request"],"supported_revisions":["%s"],"mentor_ids":["departed-mentor"],"expected_effort_minutes":90,"accessibility_needs":["Keyboard-only instructions"],"localization_needs":["Plain English glossary"],"learner_environments":[{"name":"Windows","requirement":"POSIX shell required","supported":false}],"completion_evidence":["Passing focused test"],"modules":[{"id":"request-flow","title":"Trace requests","why_it_matters":"Every API change crosses this boundary.","objectives":["Find the handler"],"expected_effort_minutes":45,"exercises":[{"title":"Trace one route","instructions":"Follow the public route to storage.","acceptance_criteria":["Name each boundary"]}],"resources":[{"kind":"documentation","label":"Architecture","path":"ARCHITECTURE.md","revision":"%s"},{"kind":"issue","label":"Practice issue","resource_id":"missing","revision":"%s"}]}],"change_reason":"Create backend onboarding"}`, commit, commit, commit)
	req, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ := http.DefaultClient.Do(req)
	var published learningpathways.Pathway
	_ = json.NewDecoder(res.Body).Decode(&published)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated || published.CurrentVersion != 1 || published.Versions[0].Modules[0].Resources[0].Status != "current" || published.Versions[0].Modules[0].Resources[1].Status != "inaccessible" || len(published.Versions[0].Findings) != 3 {
		t.Fatalf("published status=%d pathway=%#v", res.StatusCode, published)
	}
	next, _ := opened.WriteObject(storage.CommitObject, []byte(fmt.Sprintf("tree %s\nparent %s\nauthor O <o@example.test> 2 +0000\ncommitter O <o@example.test> 2 +0000\n\nAdvance project\n", tree, commit)))
	_ = opened.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: next})
	res, _ = http.Get(strings.TrimSuffix(base, "/versions"))
	var inspected learningpathways.Pathway
	_ = json.NewDecoder(res.Body).Decode(&inspected)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || inspected.Versions[0].Modules[0].Resources[0].Status != "stale" {
		t.Fatalf("inspected status=%d pathway=%#v", res.StatusCode, inspected)
	}
	req, _ = http.NewRequest(http.MethodPost, base, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	res, _ = http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("stale publication status=%d", res.StatusCode)
	}
}
