package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	PutReview(string, string, string, pullrequests.ReviewDecision, string) (pullrequests.Review, error)
	DeleteReview(string, string, string) error
	ListReviews(string, string) ([]pullrequests.Review, error)
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
	mux.HandleFunc("PUT /repositories/{repository}/pull-requests/{pull_request}/reviews/me", putPullRequestReview(store, repositories, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}/pull-requests/{pull_request}/reviews/me", deletePullRequestReview(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/reviews", listPullRequestReviews(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/readiness", getPullRequestReadiness(store, repositories, credentials))
}

type readinessBranch struct {
	Name               string `json:"name"`
	Exists             bool   `json:"exists"`
	CommitID           string `json:"commit_id,omitempty"`
	SnapshotCommitID   string `json:"snapshot_commit_id"`
	MatchesPullRequest bool   `json:"matches_pull_request"`
}

type readinessReviews struct {
	RequiredOwnerApprovals int `json:"required_owner_approvals"`
	CurrentOwnerApprovals  int `json:"current_owner_approvals"`
	CurrentChangeRequests  int `json:"current_change_requests"`
	StaleReviews           int `json:"stale_reviews"`
}

type readinessBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type readinessResponse struct {
	Ready        bool               `json:"ready"`
	CanMerge     bool               `json:"can_merge"`
	HasConflicts *bool              `json:"has_conflicts"`
	SourceBranch readinessBranch    `json:"source_branch"`
	TargetBranch readinessBranch    `json:"target_branch"`
	Reviews      readinessReviews   `json:"reviews"`
	Blockers     []readinessBlocker `json:"blockers"`
}

func getPullRequestReadiness(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
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

		response := readinessResponse{
			CanMerge:     actor.UserID != "" && actor.UserID == repository.OwnerID,
			HasConflicts: nil,
			SourceBranch: inspectReadinessBranch(opened, item.SourceBranch, item.SourceCommitID),
			TargetBranch: inspectReadinessBranch(opened, item.TargetBranch, item.TargetCommitID),
			Reviews:      readinessReviews{RequiredOwnerApprovals: 1},
			Blockers:     []readinessBlocker{},
		}
		addBlocker := func(code, message string) {
			response.Blockers = append(response.Blockers, readinessBlocker{Code: code, Message: message})
		}
		if item.Status != pullrequests.Open {
			addBlocker("pull_request_not_open", "The pull request is not open.")
		}
		if !response.SourceBranch.Exists {
			addBlocker("source_branch_missing", "The source branch no longer exists.")
		} else if !response.SourceBranch.MatchesPullRequest {
			addBlocker("source_branch_changed", "The source branch no longer points to the commit represented by the pull request.")
		}
		if !response.TargetBranch.Exists {
			addBlocker("target_branch_missing", "The target branch no longer exists.")
		}

		reviews, err := store.ListReviews(string(repository.ID), item.ID)
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		for _, review := range reviews {
			if !response.SourceBranch.Exists || review.CommitID != response.SourceBranch.CommitID {
				response.Reviews.StaleReviews++
				continue
			}
			if review.Decision == pullrequests.RequestChanges {
				response.Reviews.CurrentChangeRequests++
			}
			if review.ReviewerID == repository.OwnerID && review.Decision == pullrequests.Approve {
				response.Reviews.CurrentOwnerApprovals++
			}
		}
		if response.Reviews.CurrentOwnerApprovals < response.Reviews.RequiredOwnerApprovals {
			addBlocker("owner_approval_required", "A current approval from the repository owner is required.")
		}
		if response.Reviews.CurrentChangeRequests > 0 {
			addBlocker("changes_requested", "A current review requests changes.")
		}

		if response.SourceBranch.Exists && response.SourceBranch.MatchesPullRequest && response.TargetBranch.Exists {
			hasConflicts, err := mergeHasConflicts(r.Context(), opened, storage.ObjectID(response.TargetBranch.CommitID), storage.ObjectID(response.SourceBranch.CommitID))
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			response.HasConflicts = &hasConflicts
			if hasConflicts {
				addBlocker("merge_conflicts", "The source and target commits have merge conflicts.")
			}
		}
		if !response.CanMerge {
			addBlocker("insufficient_permissions", "Only the repository owner can merge this pull request.")
		}
		response.Ready = len(response.Blockers) == 0
		writeJSON(w, 200, response)
	}
}

func inspectReadinessBranch(repository *storage.Repository, name, snapshotCommitID string) readinessBranch {
	branch := readinessBranch{Name: name, SnapshotCommitID: snapshotCommitID}
	if id, _, found := branchTip(repository, name); found {
		branch.Exists = true
		branch.CommitID = string(id)
		branch.MatchesPullRequest = branch.CommitID == snapshotCommitID
	}
	return branch
}

// mergeHasConflicts asks stock Git to perform its normal recursive merge while
// redirecting every object it might create to a disposable object directory.
// Existing repository objects are available only as alternates, so readiness
// inspection cannot mutate the repository even when Git produces a result tree.
func mergeHasConflicts(ctx context.Context, repository *storage.Repository, target, source storage.ObjectID) (bool, error) {
	objectDirectory, err := os.MkdirTemp("", "pull-request-readiness-objects-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(objectDirectory)
	if err := os.MkdirAll(filepath.Join(objectDirectory, "pack"), 0o750); err != nil {
		return false, err
	}
	command := exec.CommandContext(ctx, "git", "--git-dir="+repository.GitDir(), "merge-tree", "--write-tree", "--quiet", "--allow-unrelated-histories", string(target), string(source))
	command.Env = append(os.Environ(), "GIT_OBJECT_DIRECTORY="+objectDirectory, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(repository.GitDir(), "objects"))
	if output, err := command.CombinedOutput(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("inspect merge conflicts: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return false, nil
}

type reviewResponse struct {
	pullrequests.Review
	Stale bool `json:"stale"`
}

func putPullRequestReview(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		var input struct {
			Decision pullrequests.ReviewDecision `json:"decision"`
		}
		if !readJSON(w, r, &input, 4<<10) {
			return
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitID, _, found := branchTip(opened, item.SourceBranch)
		if !found {
			writeJSON(w, 409, map[string]string{"error": "source_branch_unavailable"})
			return
		}
		review, err := store.PutReview(string(repository.ID), item.ID, actor.UserID, input.Decision, string(commitID))
		if errors.Is(err, pullrequests.ErrInvalidReview) {
			writeJSON(w, 422, map[string]string{"error": "invalid_review"})
			return
		}
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		writeJSON(w, 200, reviewResponse{Review: review, Stale: false})
	}
}

func deletePullRequestReview(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		err := store.DeleteReview(string(repository.ID), r.PathValue("pull_request"), actor.UserID)
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listPullRequestReviews(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		reviews, err := store.ListReviews(string(repository.ID), item.ID)
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		current := ""
		if opened, err := repositories.Open(repository.ID); err == nil {
			if id, _, found := branchTip(opened, item.SourceBranch); found {
				current = string(id)
			}
		} else {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items := make([]reviewResponse, len(reviews))
		for i, review := range reviews {
			items[i] = reviewResponse{Review: review, Stale: current == "" || review.CommitID != current}
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
