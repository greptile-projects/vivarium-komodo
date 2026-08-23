package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
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
