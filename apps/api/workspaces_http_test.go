package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func TestCollaboratorLaunchesSuspendsAndResumesExactWorkspace(t *testing.T) {
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("bubblewrap unavailable")
	}
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	credentials, _ := auth.New(t.TempDir())
	plans, _ := proposals.New(t.TempDir())
	pulls, _ := pullrequests.New(t.TempDir())
	incidentStore, _ := incidents.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	runner := workspaces.NewRunner(workspaceStore, catalog)
	repository, err := catalog.Create("owner", repositories.Metadata{Name: "shared-project", Visibility: repositories.Private})
	if err != nil {
		t.Fatal(err)
	}
	opened, _ := catalog.Open(repository.ID)
	manifest := `{"version":1,"tools":[{"name":"go","version":"1.25"}],"dependencies":["go modules"],"setup":["printf ready > .workspace-ready"],"resources":{"cpu_seconds":30,"memory_mb":256,"disk_mb":256,"setup_timeout_seconds":30}}`
	manifestBlob := writeObject(t, opened, storage.BlobObject, []byte(manifest))
	komodoTree := writeObject(t, opened, storage.TreeObject, treeEntry("100644", "workspaces.json", manifestBlob))
	readme := writeObject(t, opened, storage.BlobObject, []byte("shared state\n"))
	rootTree := writeObject(t, opened, storage.TreeObject, append(treeEntry("040000", ".komodo", komodoTree), treeEntry("100644", "README.md", readme)...))
	commit := writeCommit(t, opened, rootTree, nil, "define workspace")
	if err = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit}); err != nil {
		t.Fatal(err)
	}
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerWorkspacesHTTP(mux, workspaceStore, runner, catalog, credentials, plans, pulls, incidentStore)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/workspaces"
	request, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(`{"revision":"`+string(commit)+`","source_context":{"type":"repository"}}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusCreated {
		t.Fatalf("launch status = %d error = %v", response.StatusCode, err)
	}
	var created workspaces.Workspace
	if err = json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if created.Revision != string(commit) || created.CreatorID != "owner" || created.Access.Permission != "repository:write" || created.DefinitionDigest == "" {
		t.Fatalf("created = %#v", created)
	}
	var current workspaces.Workspace
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		request, _ = http.NewRequest(http.MethodGet, base+"/"+created.ID, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response, _ = http.DefaultClient.Do(request)
		_ = json.NewDecoder(response.Body).Decode(&current)
		response.Body.Close()
		if current.State == workspaces.Ready || current.State == workspaces.Failed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if current.State != workspaces.Ready || len(current.Events) < 4 {
		t.Fatalf("setup = %#v", current)
	}
	for _, action := range []string{"suspend", "resume"} {
		request, _ = http.NewRequest(http.MethodPost, base+"/"+created.ID+"/"+action, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer "+token)
		response, _ = http.DefaultClient.Do(request)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", action, response.StatusCode)
		}
		_ = json.NewDecoder(response.Body).Decode(&current)
		response.Body.Close()
	}
	if current.State != workspaces.Ready || current.Revision != string(commit) || current.DefinitionDigest != created.DefinitionDigest {
		t.Fatalf("resumed = %#v", current)
	}
}
