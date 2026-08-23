package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func TestPullConflictEvidenceExplainsExactIntentWithoutChangingBranches(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "intent", Visibility: repositories.Public})
	repo, _ := catalog.Open(repository.ID)
	commit := func(parent storage.ObjectID, contents, message string) storage.ObjectID {
		blob, _ := repo.WriteObject(storage.BlobObject, []byte(contents))
		tree, _ := repo.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"schema.json": blob}))
		parentLine := ""
		if parent != "" {
			parentLine = "parent " + string(parent) + "\n"
		}
		id, _ := repo.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+parentLine+"author A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\n"+message+"\n"))
		return id
	}
	base := commit("", `{"type":"string"}`+"\n", "base")
	source := commit(base, `{"type":"number","title":"source"}`+"\n", "source contract")
	target := commit(base, `{"type":"boolean","title":"target"}`+"\n", "target contract")
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: target})
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/change", ObjectID: source})
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), SourceRepositoryID: string(repository.ID), AuthorID: "author", Title: "Change response", Body: "Clients must receive numbers.", SourceBranch: "change", TargetBranch: "main", SourceCommitID: string(source), TargetCommitID: string(target), DeliveryEvidence: &pullrequests.DeliveryEvidence{CompletionCriteria: []pullrequests.CriterionStatus{{Criterion: "Numeric responses remain accepted", Status: "passed"}}}})
	if _, err := analyzePullConflict(t.Context(), pull, pulls, repo, repo, nil); err != nil {
		t.Fatalf("direct analysis: %v", err)
	}
	token := issueAccess(t, credentials, "reader", auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pulls, nil, catalog, credentials)
	request := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID+"/conflicts", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var analysis conflictAnalysis
	_ = json.Unmarshal(response.Body.Bytes(), &analysis)
	if response.Code != http.StatusOK || analysis.BaseCommitID != string(base) || len(analysis.Conflicts) != 1 || analysis.Conflicts[0].Kind != "textual" || analysis.Source.Intent.OwnerID != "author" || len(analysis.Source.Intent.AcceptanceCriteria) != 1 || analysis.MutatesBranches {
		t.Fatalf("analysis = %#v, status=%d body=%s", analysis, response.Code, response.Body.String())
	}
	mainAfter, _ := repo.ReadReference("refs/heads/main")
	sourceAfter, _ := repo.ReadReference("refs/heads/change")
	if mainAfter.ObjectID != target || sourceAfter.ObjectID != source {
		t.Fatalf("analysis changed branches: main=%s source=%s", mainAfter.ObjectID, sourceAfter.ObjectID)
	}
	// Moving a selected branch preserves the old analysis inputs and reports them stale.
	_ = repo.UpdateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request.Clone(request.Context()))
	analysis = conflictAnalysis{}
	_ = json.Unmarshal(response.Body.Bytes(), &analysis)
	if !analysis.Stale || analysis.Target.Revision.LiveCommitID != string(base) {
		t.Fatalf("stale analysis = %#v", analysis)
	}
}

func TestCurrentConflictLaunchesAuthorityBoundSharedWorkspace(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	workspaceStore, _ := workspaces.New(t.TempDir())
	repository, _ := catalog.Create("maintainer", repositories.Metadata{Name: "reconcile", Visibility: repositories.Public})
	repo, _ := catalog.Open(repository.ID)
	commit := func(parent storage.ObjectID, value, message string, manifest bool) storage.ObjectID {
		file, _ := repo.WriteObject(storage.BlobObject, []byte(value))
		entries := treeEntry("100644", "contract.go", file)
		if manifest {
			raw := `{"version":1,"tools":[{"name":"go","version":"1.25"}],"dependencies":[],"setup":["true"],"resources":{"cpu_seconds":10,"memory_mb":128,"disk_mb":128,"setup_timeout_seconds":10}}`
			blob, _ := repo.WriteObject(storage.BlobObject, []byte(raw))
			komodo, _ := repo.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"workspaces.json": blob}))
			entries = append(treeEntry("040000", ".komodo", komodo), entries...)
		}
		tree, _ := repo.WriteObject(storage.TreeObject, entries)
		parents := ""
		if parent != "" {
			parents = "parent " + string(parent) + "\n"
		}
		id, _ := repo.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\n"+parents+"author A <a@example.test> 1 +0000\ncommitter A <a@example.test> 1 +0000\n\n"+message+"\n"))
		return id
	}
	base := commit("", "package p\nconst Value = 1\n", "base", false)
	source := commit(base, "package p\nconst Value = 2\n", "source", false)
	target := commit(base, "package p\nconst Value = 3\n", "target", true)
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: target})
	_ = repo.CreateReference(storage.Reference{Name: "refs/heads/change", ObjectID: source})
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), SourceRepositoryID: string(repository.ID), AuthorID: "contributor", Title: "overlap", SourceBranch: "change", TargetBranch: "main", SourceCommitID: string(source), TargetCommitID: string(target)})
	runner := workspaces.NewRunner(workspaceStore, catalog)
	mux := http.NewServeMux()
	registerWorkspacesHTTP(mux, workspaceStore, runner, catalog, credentials, nil, pulls, nil)
	token := issueAccess(t, credentials, "maintainer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID+"/conflicts/workspace", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var created workspaces.Workspace
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	if response.Code != http.StatusCreated || created.Context.Conflict == nil || created.Context.Conflict.Source.CommitID != string(source) || created.Context.Conflict.Target.CommitID != string(target) || created.Context.Conflict.PublishRepositoryID != string(repository.ID) || len(created.Context.Conflict.OwnerIDs) != 2 {
		t.Fatalf("workspace = %#v status=%d body=%s", created, response.Code, response.Body.String())
	}
	for deadline := time.Now().Add(time.Second); created.State != workspaces.Ready && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
		created, _ = workspaceStore.Get(string(repository.ID), created.ID)
	}
	// Questions and proposed edits must cite one of the workspace's frozen
	// revisions and make their effect on intended outcomes inspectable.
	resolutionURL := "/repositories/" + string(repository.ID) + "/workspaces/" + created.ID + "/resolutions"
	request = httptest.NewRequest(http.MethodPost, resolutionURL, strings.NewReader(`{"kind":"proposal","summary":"Keep the source value while retaining the target setup contract","paths":["contract.go"],"evidence":[{"kind":"source_change","reference":"contract.go:2","revision":"`+string(source)+`","path":"contract.go"},{"kind":"target_change","reference":"contract.go:2","revision":"`+string(target)+`","path":"contract.go"}],"impacts":[{"kind":"acceptance_criterion","outcome":"source callers receive Value 2","disposition":"preserved","rationale":"the proposed constant matches the exact source revision"},{"kind":"design_decision","outcome":"target workspace setup remains available","disposition":"preserved","rationale":"the target .komodo tree is not changed"}],"assumptions":["no caller depends on Value 3"],"uncertainty":"combined checks have not run","actor_kind":"agent"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	_ = json.Unmarshal(response.Body.Bytes(), &created)
	if response.Code != http.StatusCreated || len(created.Resolutions) != 1 || created.Resolutions[0].ActorID != "maintainer" || created.Resolutions[0].Impacts[0].Disposition != "preserved" || created.Resolutions[0].Uncertainty == "" {
		t.Fatalf("resolution ledger = %#v status=%d body=%s", created.Resolutions, response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, resolutionURL, strings.NewReader(`{"kind":"question","summary":"Was this checked against a newer branch?","evidence":[{"kind":"branch","reference":"moving tip","revision":"not-frozen"}]}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unfrozen evidence status=%d body=%s", response.Code, response.Body.String())
	}
}
