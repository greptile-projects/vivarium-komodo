package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestReleaseCandidateDerivesInclusionsSincePriorRelease(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	pulls, _ := pullrequests.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "project", Visibility: repositories.Public})
	opened, _ := catalog.Open(repository.ID)
	manifest, _ := opened.WriteObject(storage.BlobObject, []byte(`{"version":1,"builds":[{"name":"compile","command":"printf compiled"},{"name":"package","command":"mkdir -p dist; printf package > dist/app","dependencies":["compile"],"artifacts":["dist/app"],"timeout_seconds":5}]}`))
	komodoTree, _ := opened.WriteObject(storage.TreeObject, treeEntry("100644", "releases.json", manifest))
	tree, _ := opened.WriteObject(storage.TreeObject, treeEntry("40000", ".komodo", komodoTree))
	commit := func(parent storage.ObjectID, message string) storage.ObjectID {
		body := "tree " + string(tree) + "\n"
		if parent != "" {
			body += "parent " + string(parent) + "\n"
		}
		id, _ := opened.WriteObject(storage.CommitObject, []byte(body+"author A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\n"+message+"\n"))
		return id
	}
	base := commit("", "base")
	first := commit(base, "first merge")
	second := commit(first, "second merge")
	makeMerged := func(title, author string, merge storage.ObjectID, proposal, task string) {
		p, _ := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: author, Title: title, SourceBranch: title, TargetBranch: "main", SourceCommitID: string(merge), TargetCommitID: string(base), ProposalID: proposal, TaskID: task})
		_, _ = pulls.MarkMerged(string(repository.ID), p.ID, "owner", string(merge))
	}
	makeMerged("first", "alice", first, "proposal-1", "task-1")
	makeMerged("second", "bob", second, "proposal-2", "task-2")
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	checkStore, _ := checkruns.New(t.TempDir())
	runner := checkruns.NewRunner(checkStore, catalog)
	registerReleasesHTTP(mux, releaseStore, checkStore, runner, pulls, catalog, credentials)
	create := func(body map[string]string) releases.Release {
		data, _ := json.Marshal(body)
		request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/releases", bytes.NewReader(data))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != 201 {
			t.Fatalf("create: %d %s", response.Code, response.Body.String())
		}
		var item releases.Release
		_ = json.Unmarshal(response.Body.Bytes(), &item)
		return item
	}
	prior := create(map[string]string{"version": "v1", "commit_id": string(first), "notes": "baseline"})
	candidate := create(map[string]string{"version": "v2", "commit_id": string(second), "prior_release_id": prior.ID, "notes": "next"})
	if candidate.PriorCommitID != string(first) || len(candidate.PullRequests) != 1 || candidate.PullRequests[0].Title != "second" || len(candidate.ContributorIDs) != 1 || candidate.ContributorIDs[0] != "bob" || len(candidate.ProposalIDs) != 1 || candidate.ProposalIDs[0] != "proposal-2" || len(candidate.TaskIDs) != 1 || candidate.TaskIDs[0] != "task-2" {
		t.Fatalf("candidate = %#v", candidate)
	}
	var attestation releaseAttestation
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		request := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/releases/"+candidate.ID+"/attestation", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		_ = json.Unmarshal(response.Body.Bytes(), &attestation)
		if attestation.Verified {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !attestation.Verified || attestation.SourceCommitID != string(second) || attestation.CreatedByID != "owner" || len(attestation.Attempts) != 2 || len(attestation.Attempts[0].Definition.Dependencies) != 1 {
		t.Fatalf("attestation = %#v", attestation)
	}
	var artifact *checkruns.Artifact
	for _, event := range attestation.Attempts[0].Events {
		if event.Artifact != nil {
			artifact = event.Artifact
		}
	}
	if artifact == nil || artifact.SHA256 == "" {
		t.Fatalf("artifact evidence = %#v", attestation.Attempts)
	}
	download := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/releases/"+candidate.ID+"/builds/"+attestation.Attempts[0].ID+"/artifacts/"+artifact.ID, nil)
	download.Header.Set("Authorization", "Bearer "+token)
	downloadResponse := httptest.NewRecorder()
	mux.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != 200 || downloadResponse.Body.String() != "package" {
		t.Fatalf("artifact download = %d %q", downloadResponse.Code, downloadResponse.Body.String())
	}
}
