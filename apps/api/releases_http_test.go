package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
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
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
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
	registerReleasesHTTP(mux, releaseStore, pulls, catalog, credentials)
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
}
