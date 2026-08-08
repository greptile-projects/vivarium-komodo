package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type unusedCheckController struct{}

func (unusedCheckController) Rerun(string, string, string, string) (checkruns.Run, error) {
	return checkruns.Run{}, nil
}
func (unusedCheckController) Cancel(string, string, string, string) (checkruns.Run, error) {
	return checkruns.Run{}, nil
}

func TestFailedCheckStartsEvidenceBackedChangeSession(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	pulls, _ := pullrequests.New(t.TempDir())
	runs, _ := checkruns.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "owner", Title: "Repair", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "failed-revision", TargetCommitID: "base"})
	run, _ := runs.Create(string(repository.ID), pull.ID, "failed-revision", checkruns.Definition{Name: "api", Command: "go test ./...", WorkingDirectory: "apps/api", TimeoutSeconds: 90, Environment: map[string]string{"MODE": "test"}, Artifacts: []string{"report.json"}})
	run, _ = runs.Start(run.ID)
	_ = runs.AppendLog(run.ID, "stderr", "expected 200, got 500\n")
	artifact, _ := runs.AddArtifact(run.ID, "report.json", "application/json", []byte(`{"failed":true}`))
	run, _ = runs.Complete(run.ID, 1, false, "command exited with status 1")
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	mux := http.NewServeMux()
	registerCheckRunsHTTP(mux, runs, unusedCheckController{}, pulls, catalog, credentials, sessions, nil)
	path := "/repositories/" + string(repository.ID) + "/pull-requests/" + pull.ID + "/check-runs/" + run.ID + "/change-session"
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("start repair: %d %s", response.Code, response.Body.String())
	}
	var session changesessions.Session
	_ = json.NewDecoder(response.Body).Decode(&session)
	failure := session.CheckFailure
	if session.SourceCommitID != run.CommitID || failure == nil || failure.RunID != run.ID || failure.Command != "go test ./..." || failure.WorkingDirectory != "apps/api" || failure.ExitCode != 1 || len(failure.Logs) != 1 || failure.Logs[0].Message != "expected 200, got 500\n" || len(failure.Artifacts) != 1 || failure.Artifacts[0].ID != artifact.ID {
		t.Fatalf("evidence snapshot: %#v", session)
	}
	restored, err := sessions.Get(string(repository.ID), pull.ID, session.ID)
	if err != nil || restored.CheckFailure == nil || restored.CheckFailure.Environment["MODE"] != "test" || restored.Events[0].Metadata["check_run_id"] != run.ID {
		t.Fatalf("restored evidence: %#v, %v", restored, err)
	}
}

func TestRepairSessionRequiresFailedCheck(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	pulls, _ := pullrequests.New(t.TempDir())
	runs, _ := checkruns.New(t.TempDir())
	sessions, _ := changesessions.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Private})
	pull, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: "owner", Title: "Repair", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: "revision", TargetCommitID: "base"})
	run, _ := runs.Create(string(repository.ID), pull.ID, "revision", checkruns.Definition{Name: "api", Command: "go test ./..."})
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerCheckRunsHTTP(mux, runs, unusedCheckController{}, pulls, catalog, credentials, sessions, nil)
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests/"+pull.ID+"/check-runs/"+run.ID+"/change-session", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("queued check repair: %d %s", response.Code, response.Body.String())
	}
}
