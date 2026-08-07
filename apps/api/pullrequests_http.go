package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type pullRequestStore interface {
	Create(pullrequests.CreateParams) (pullrequests.PullRequest, error)
	Get(string, string) (pullrequests.PullRequest, error)
	List(string) ([]pullrequests.PullRequest, error)
	AddComment(string, string, string, string) (pullrequests.Comment, error)
	ListComments(string, string) ([]pullrequests.Comment, error)
}

type pullRequestRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerPullRequestsHTTP(mux *http.ServeMux, store pullRequestStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests", createPullRequest(store, proposalStore, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests", listPullRequests(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}", getPullRequest(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/commits", listPullRequestCommits(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/files", listPullRequestFiles(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/comments", createPullRequestComment(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/comments", listPullRequestComments(store, repositories, credentials))
}

func createPullRequest(store pullRequestStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			ProposalID   string `json:"proposal_id"`
			Title        string `json:"title"`
			Body         string `json:"body"`
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
		}
		if !readJSON(w, r, &input, 70<<10) {
			return
		}
		if input.ProposalID != "" {
			if _, err := proposalStore.Get(string(repository.ID), input.ProposalID); errors.Is(err, proposals.ErrNotFound) {
				writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
				return
			} else if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		source, sourceName, sourceOK := branchTip(opened, input.SourceBranch)
		target, targetName, targetOK := branchTip(opened, input.TargetBranch)
		if !sourceOK || !targetOK || sourceName == targetName {
			writeJSON(w, 422, map[string]string{"error": "invalid_branches"})
			return
		}
		item, err := store.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), ProposalID: input.ProposalID, AuthorID: actor.UserID, Title: input.Title, Body: input.Body, SourceBranch: sourceName, TargetBranch: targetName, SourceCommitID: string(source), TargetCommitID: string(target)})
		if errors.Is(err, pullrequests.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		location := "/repositories/" + string(repository.ID) + "/pull-requests/" + item.ID
		w.Header().Set("Location", location)
		writeJSON(w, http.StatusCreated, item)
	}
}

func branchTip(repository *storage.Repository, branch string) (storage.ObjectID, string, bool) {
	branch = strings.TrimSpace(branch)
	if branch == "" || strings.HasPrefix(branch, "refs/") {
		return "", "", false
	}
	reference, err := repository.ReadReference(storage.ReferenceName("refs/heads/" + branch))
	if err != nil || reference.ObjectID == "" {
		return "", "", false
	}
	object, err := repository.ReadObject(reference.ObjectID)
	if err != nil || object.Type != storage.CommitObject {
		return "", "", false
	}
	return reference.ObjectID, branch, true
}

func listPullRequests(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func getPullRequest(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("pull_request"))
		if errors.Is(err, pullrequests.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, item)
	}
}

func listPullRequestCommits(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items, err := commitsBetween(opened, storage.ObjectID(item.SourceCommitID), storage.ObjectID(item.TargetCommitID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func listPullRequestFiles(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items, err := filesBetween(opened, storage.ObjectID(item.SourceCommitID), storage.ObjectID(item.TargetCommitID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func createPullRequestComment(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &input, 70<<10) {
			return
		}
		item, err := store.AddComment(string(repository.ID), r.PathValue("pull_request"), actor.UserID, input.Body)
		if errors.Is(err, pullrequests.ErrInvalidComment) {
			writeJSON(w, 422, map[string]string{"error": "invalid_comment"})
			return
		}
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

func listPullRequestComments(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.ListComments(string(repository.ID), r.PathValue("pull_request"))
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func readPullRequest(w http.ResponseWriter, store pullRequestStore, repositoryID, id string) (pullrequests.PullRequest, bool) {
	item, err := store.Get(repositoryID, id)
	if err != nil {
		writePullRequestError(w, err)
		return pullrequests.PullRequest{}, false
	}
	return item, true
}

func writePullRequestError(w http.ResponseWriter, err error) {
	if errors.Is(err, pullrequests.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
