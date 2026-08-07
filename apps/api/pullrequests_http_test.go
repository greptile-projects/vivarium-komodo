package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestPullRequestExposesSnapshottedCommitsFilesAndDiscussion(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	if _, err := catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID)); err != nil {
		t.Fatal(err)
	}
	opened, _ := catalog.Open(repository.ID)
	oldReadme, _ := opened.WriteObject(storage.BlobObject, []byte("old setup\n"))
	oldTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"README.md": oldReadme}))
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(oldTree)+"\nauthor Base Author <base@example.com> 1 +0000\ncommitter Base Author <base@example.com> 1 +0000\n\nbase\n"))
	newReadme, _ := opened.WriteObject(storage.BlobObject, []byte("new setup\nwith details\n"))
	script, _ := opened.WriteObject(storage.BlobObject, []byte("#!/bin/sh\necho ready\n"))
	newTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"README.md": newReadme, "setup.sh": script}))
	change, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(newTree)+"\nparent "+string(base)+"\nauthor Contributor <c@example.com> 2 +0000\ncommitter Contributor <c@example.com> 2 +0000\n\nexplain setup\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: change})
	created, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: string(contributor.ID), Title: "Improve setup", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(change), TargetCommitID: string(base)})
	token := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	baseURL := server.URL + "/repositories/" + string(repository.ID) + "/pull-requests/" + created.ID

	response, _ := http.Get(baseURL + "/commits")
	var commits struct {
		Items []struct {
			ID, Message, Author string
			ParentIDs           []string `json:"parent_ids"`
		}
		Total int `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&commits)
	response.Body.Close()
	if response.StatusCode != 200 || commits.Total != 1 || commits.Items[0].ID != string(change) || commits.Items[0].Message != "explain setup" || commits.Items[0].Author == "" {
		t.Fatalf("commits = %#v, status %d", commits, response.StatusCode)
	}
	response, _ = http.Get(baseURL + "/files")
	var files struct {
		Items []struct {
			Path, Status, Patch  string
			Additions, Deletions int
			Binary               bool
		}
		Total int `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&files)
	response.Body.Close()
	if response.StatusCode != 200 || files.Total != 2 || files.Items[0].Path != "README.md" || files.Items[0].Status != "modified" || files.Items[0].Additions != 2 || files.Items[0].Deletions != 1 || !strings.Contains(files.Items[0].Patch, "+with details") || files.Items[1].Status != "added" {
		t.Fatalf("files = %#v, status %d", files, response.StatusCode)
	}
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/comments", strings.NewReader(`{"body":"The extra detail is important."}`))
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var comment pullrequests.Comment
	json.NewDecoder(response.Body).Decode(&comment)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || comment.AuthorID != string(contributor.ID) || comment.PullRequestID != created.ID {
		t.Fatalf("comment = %#v, status %d", comment, response.StatusCode)
	}
	response, _ = http.Get(baseURL + "/comments")
	var comments struct {
		Items []pullrequests.Comment `json:"items"`
		Total int                    `json:"total_count"`
	}
	json.NewDecoder(response.Body).Decode(&comments)
	response.Body.Close()
	if response.StatusCode != 200 || comments.Total != 1 || comments.Items[0] != comment {
		t.Fatalf("comments = %#v, status %d", comments, response.StatusCode)
	}
}

func testTree(t *testing.T, entries map[string]storage.ObjectID) []byte {
	t.Helper()
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	slices.Sort(names)
	var content []byte
	for _, name := range names {
		objectID, err := hex.DecodeString(string(entries[name]))
		if err != nil {
			t.Fatal(err)
		}
		mode := "100644"
		if strings.HasSuffix(name, ".sh") {
			mode = "100755"
		}
		content = append(content, []byte(mode+" "+name)...)
		content = append(content, 0)
		content = append(content, objectID...)
	}
	return content
}

func TestContributorOpensPullRequestAtExactBranchState(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	if _, err := catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID)); err != nil {
		t.Fatal(err)
	}
	opened, _ := catalog.Open(repository.ID)
	treeID, _ := opened.WriteObject(storage.TreeObject, nil)
	targetID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nauthor A <a@example.com> 1 +0000\ncommitter A <a@example.com> 1 +0000\n\nbase\n"))
	sourceID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nparent "+string(targetID)+"\nauthor A <a@example.com> 2 +0000\ncommitter A <a@example.com> 2 +0000\n\nchange\n"))
	if err := opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: targetID}); err != nil {
		t.Fatal(err)
	}
	if err := opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: sourceID}); err != nil {
		t.Fatal(err)
	}
	proposal, _ := proposalStore.Create(string(repository.ID), string(contributor.ID), "Improve setup", "Make setup reproducible.")
	token := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()
	base := server.URL + "/repositories/" + string(repository.ID) + "/pull-requests"
	body := `{"proposal_id":"` + proposal.ID + `","title":"Automate setup","body":"Adds one reproducible command.","source_branch":"candidate","target_branch":"main"}`
	request, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var created pullrequests.PullRequest
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || created.AuthorID != string(contributor.ID) || created.ProposalID != proposal.ID || created.SourceCommitID != string(sourceID) || created.TargetCommitID != string(targetID) || created.Status != pullrequests.Open {
		t.Fatalf("created pull request = %#v, status %d", created, response.StatusCode)
	}
	if response.Header.Get("Location") != "/repositories/"+string(repository.ID)+"/pull-requests/"+created.ID {
		t.Fatalf("location = %q", response.Header.Get("Location"))
	}

	// Later branch movement does not rewrite the repository state represented by the request.
	newSourceID, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(treeID)+"\nparent "+string(sourceID)+"\nauthor A <a@example.com> 3 +0000\ncommitter A <a@example.com> 3 +0000\n\nmore\n"))
	if err := opened.UpdateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: newSourceID}); err != nil {
		t.Fatal(err)
	}
	request, _ = http.NewRequest(http.MethodGet, base+"/"+created.ID, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, _ = http.DefaultClient.Do(request)
	var inspected pullrequests.PullRequest
	json.NewDecoder(response.Body).Decode(&inspected)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || inspected.SourceCommitID != string(sourceID) {
		t.Fatalf("inspected pull request = %#v, status %d", inspected, response.StatusCode)
	}
}

func TestPullRequestRejectsMissingOrNonCommitBranches(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	token := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	request := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/pull-requests", strings.NewReader(`{"title":"Change","source_branch":"missing","target_branch":"main"}`))
	request.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, request)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "invalid_branches") {
		t.Fatalf("response = %d %s", w.Code, w.Body.String())
	}
}

func TestCollaboratorReviewTracksEvaluatedCommitAndStaleness(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	outsider, _ := userStore.Create(users.Profile{Handle: "outsider", DisplayName: "Outsider"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID))
	opened, _ := catalog.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n"))
	change, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nparent "+string(base)+"\nauthor A <a@x> 2 +0000\ncommitter A <a@x> 2 +0000\n\nchange\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: change})
	pr, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: string(contributor.ID), Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(change), TargetCommitID: string(base)})
	ownerToken := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	outsiderToken := issueAccess(t, credentials, string(outsider.ID), auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	baseURL := "/repositories/" + string(repository.ID) + "/pull-requests/" + pr.ID + "/reviews"
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	w := do(http.MethodPut, baseURL+"/me", ownerToken, `{"decision":"approve"}`)
	var approved reviewResponse
	json.NewDecoder(w.Body).Decode(&approved)
	if w.Code != 200 || approved.ReviewerID != string(owner.ID) || approved.CommitID != string(change) || approved.Stale {
		t.Fatalf("approval = %d %#v", w.Code, approved)
	}
	newChange, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nparent "+string(change)+"\nauthor A <a@x> 3 +0000\ncommitter A <a@x> 3 +0000\n\nmore\n"))
	opened.UpdateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: newChange})
	w = do(http.MethodGet, baseURL, ownerToken, "")
	var listed struct {
		Items []reviewResponse `json:"items"`
		Total int              `json:"total_count"`
	}
	json.NewDecoder(w.Body).Decode(&listed)
	if w.Code != 200 || listed.Total != 1 || !listed.Items[0].Stale || listed.Items[0].CommitID != string(change) {
		t.Fatalf("stale reviews = %d %#v", w.Code, listed)
	}
	w = do(http.MethodPut, baseURL+"/me", ownerToken, `{"decision":"request_changes"}`)
	var replaced reviewResponse
	json.NewDecoder(w.Body).Decode(&replaced)
	if w.Code != 200 || replaced.Decision != pullrequests.RequestChanges || replaced.CommitID != string(newChange) || replaced.Stale {
		t.Fatalf("replacement = %d %#v", w.Code, replaced)
	}
	if w := do(http.MethodPut, baseURL+"/me", outsiderToken, `{"decision":"approve"}`); w.Code != http.StatusNotFound {
		t.Fatalf("outsider review status = %d", w.Code)
	}
	if w := do(http.MethodDelete, baseURL+"/me", ownerToken, ""); w.Code != http.StatusNoContent {
		t.Fatalf("withdraw status = %d", w.Code)
	}
	w = do(http.MethodGet, baseURL, ownerToken, "")
	json.NewDecoder(w.Body).Decode(&listed)
	if listed.Total != 0 {
		t.Fatalf("reviews after withdrawal = %#v", listed)
	}
}

func TestPullRequestReadinessReportsEveryExistingBlockerWithoutMutation(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Public})
	catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID))
	opened, _ := catalog.Open(repository.ID)
	baseBlob, _ := opened.WriteObject(storage.BlobObject, []byte("base\n"))
	baseTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"file.txt": baseBlob}))
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(baseTree)+"\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n"))
	sourceBlob, _ := opened.WriteObject(storage.BlobObject, []byte("source\n"))
	sourceTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"file.txt": sourceBlob}))
	source, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(sourceTree)+"\nparent "+string(base)+"\nauthor A <a@x> 2 +0000\ncommitter A <a@x> 2 +0000\n\nsource\n"))
	targetBlob, _ := opened.WriteObject(storage.BlobObject, []byte("target\n"))
	targetTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"file.txt": targetBlob}))
	target, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(targetTree)+"\nparent "+string(base)+"\nauthor A <a@x> 3 +0000\ncommitter A <a@x> 3 +0000\n\ntarget\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: target})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: source})
	pr, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: string(contributor.ID), Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(source), TargetCommitID: string(base)})
	pullRequestStore.PutReview(string(repository.ID), pr.ID, string(owner.ID), pullrequests.Approve, string(source))
	pullRequestStore.PutReview(string(repository.ID), pr.ID, string(contributor.ID), pullrequests.RequestChanges, string(source))
	ownerToken := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead)
	contributorToken := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	path := "/repositories/" + string(repository.ID) + "/pull-requests/" + pr.ID + "/readiness"
	objectsBefore, _ := opened.ListObjects()
	refsBefore, _ := opened.ListReferences()

	read := func(token string) readinessResponse {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		var result readinessResponse
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK {
			t.Fatalf("readiness status = %d, body = %s", w.Code, w.Body.String())
		}
		return result
	}
	result := read(ownerToken)
	if result.Ready || !result.CanMerge || result.HasConflicts == nil || !*result.HasConflicts || result.TargetBranch.CommitID != string(target) || result.TargetBranch.MatchesPullRequest || result.Reviews.CurrentOwnerApprovals != 1 || result.Reviews.CurrentChangeRequests != 1 {
		t.Fatalf("owner readiness = %#v", result)
	}
	codes := make([]string, len(result.Blockers))
	for i, blocker := range result.Blockers {
		codes[i] = blocker.Code
	}
	if !slices.Equal(codes, []string{"changes_requested", "merge_conflicts"}) {
		t.Fatalf("blockers = %#v", result.Blockers)
	}
	contributorResult := read(contributorToken)
	if contributorResult.CanMerge || contributorResult.Blockers[len(contributorResult.Blockers)-1].Code != "insufficient_permissions" {
		t.Fatalf("contributor readiness = %#v", contributorResult)
	}
	objectsAfter, _ := opened.ListObjects()
	refsAfter, _ := opened.ListReferences()
	if len(objectsAfter) != len(objectsBefore) || !slices.Equal(refsAfter, refsBefore) {
		t.Fatalf("readiness mutated repository: objects %d -> %d, refs %#v -> %#v", len(objectsBefore), len(objectsAfter), refsBefore, refsAfter)
	}

	// When the source moves, conflict analysis is intentionally unavailable and
	// current reviews become stale; the response explains all three consequences.
	newSource, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(sourceTree)+"\nparent "+string(source)+"\nauthor A <a@x> 4 +0000\ncommitter A <a@x> 4 +0000\n\nmore\n"))
	opened.UpdateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: newSource})
	result = read(ownerToken)
	if result.HasConflicts != nil || result.SourceBranch.MatchesPullRequest || result.Reviews.StaleReviews != 2 || result.Blockers[0].Code != "source_branch_changed" || result.Blockers[1].Code != "owner_approval_required" {
		t.Fatalf("changed-source readiness = %#v", result)
	}
}

func TestPullRequestReadinessIsReadyForOwnerAfterCurrentApproval(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	opened, _ := catalog.Open(repository.ID)
	tree, _ := opened.WriteObject(storage.TreeObject, nil)
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n"))
	change, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(tree)+"\nparent "+string(base)+"\nauthor A <a@x> 2 +0000\ncommitter A <a@x> 2 +0000\n\nchange\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: change})
	pr, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), AuthorID: string(owner.ID), Title: "Change", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(change), TargetCommitID: string(base)})
	pullRequestStore.PutReview(string(repository.ID), pr.ID, string(owner.ID), pullrequests.Approve, string(change))
	token := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	req := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/pull-requests/"+pr.ID+"/readiness", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var result readinessResponse
	json.NewDecoder(w.Body).Decode(&result)
	if w.Code != http.StatusOK || !result.Ready || !result.CanMerge || result.HasConflicts == nil || *result.HasConflicts || len(result.Blockers) != 0 {
		t.Fatalf("readiness = %d %#v", w.Code, result)
	}
}

func TestOwnerMergesReadyPullRequestAndClosesLinkedCollaboration(t *testing.T) {
	gitStorage, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStorage)
	proposalStore, _ := proposals.New(t.TempDir())
	pullRequestStore, _ := pullrequests.New(t.TempDir())
	userStore, _ := users.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner, _ := userStore.Create(users.Profile{Handle: "owner", DisplayName: "Owner"})
	contributor, _ := userStore.Create(users.Profile{Handle: "contributor", DisplayName: "Contributor"})
	repository, _ := catalog.Create(string(owner.ID), repositories.Metadata{Name: "project", Visibility: repositories.Private})
	catalog.AddCollaborator(string(owner.ID), repository.ID, string(contributor.ID))
	opened, _ := catalog.Open(repository.ID)
	baseTree, _ := opened.WriteObject(storage.TreeObject, nil)
	base, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(baseTree)+"\nauthor A <a@x> 1 +0000\ncommitter A <a@x> 1 +0000\n\nbase\n"))
	changeBlob, _ := opened.WriteObject(storage.BlobObject, []byte("accepted\n"))
	changeTree, _ := opened.WriteObject(storage.TreeObject, testTree(t, map[string]storage.ObjectID{"change.txt": changeBlob}))
	change, _ := opened.WriteObject(storage.CommitObject, []byte("tree "+string(changeTree)+"\nparent "+string(base)+"\nauthor Contributor <c@x> 2 +0000\ncommitter Contributor <c@x> 2 +0000\n\nchange\n"))
	opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: base})
	opened.CreateReference(storage.Reference{Name: "refs/heads/candidate", ObjectID: change})
	proposal, _ := proposalStore.Create(string(repository.ID), string(contributor.ID), "Improve project", "Shared rationale.")
	pr, _ := pullRequestStore.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), ProposalID: proposal.ID, AuthorID: string(contributor.ID), Title: "Apply accepted change", Body: "Preserve the collaboration context.", SourceBranch: "candidate", TargetBranch: "main", SourceCommitID: string(change), TargetCommitID: string(base)})
	pullRequestStore.PutReview(string(repository.ID), pr.ID, string(owner.ID), pullrequests.Approve, string(change))
	ownerToken := issueAccess(t, credentials, string(owner.ID), auth.API, auth.RepositoryWrite)
	contributorToken := issueAccess(t, credentials, string(contributor.ID), auth.API, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPullRequestsHTTP(mux, pullRequestStore, proposalStore, catalog, credentials)
	path := "/repositories/" + string(repository.ID) + "/pull-requests/" + pr.ID + "/merge"
	do := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	if w := do(contributorToken); w.Code != http.StatusNotFound {
		t.Fatalf("contributor merge = %d %s", w.Code, w.Body.String())
	}
	w := do(ownerToken)
	var merged pullrequests.PullRequest
	json.NewDecoder(w.Body).Decode(&merged)
	if w.Code != http.StatusOK || merged.Status != pullrequests.Merged || merged.MergedByID != string(owner.ID) || merged.MergeCommitID == "" || merged.MergedAt == nil {
		t.Fatalf("merge = %d %#v body=%s", w.Code, merged, w.Body.String())
	}
	tip, _, found := branchTip(opened, "main")
	commit, err := opened.ReadCommit(tip)
	if err != nil || !found || string(tip) != merged.MergeCommitID || commit.Tree != changeTree || !slices.Equal(commit.Parents, []storage.ObjectID{base, change}) || !strings.Contains(string(commit.Content), "Pull-Request: "+pr.ID) || !strings.Contains(string(commit.Content), "Proposal: "+proposal.ID) || !strings.Contains(string(commit.Content), "Author-ID: "+string(contributor.ID)) || !strings.Contains(string(commit.Content), "Merged-By: "+string(owner.ID)) {
		t.Fatalf("merge commit = %#v, %v", commit, err)
	}
	closed, _ := proposalStore.Get(string(repository.ID), proposal.ID)
	comments, _ := pullRequestStore.ListComments(string(repository.ID), pr.ID)
	if closed.State != proposals.Closed || closed.ClosedByID != string(owner.ID) || len(comments) != 1 || comments[0].AuthorID != string(owner.ID) || !strings.Contains(comments[0].Body, merged.MergeCommitID) {
		t.Fatalf("closed collaboration = proposal %#v comments %#v", closed, comments)
	}
}
