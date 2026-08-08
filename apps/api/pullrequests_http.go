package main

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type pullRequestStore interface {
	Create(pullrequests.CreateParams) (pullrequests.PullRequest, error)
	Get(string, string) (pullrequests.PullRequest, error)
	List(string) ([]pullrequests.PullRequest, error)
	SynchronizeSource(string, string, string) (pullrequests.PullRequest, error)
	RequestReview(string, string) (pullrequests.PullRequest, error)
	AddComment(string, string, string, string) (pullrequests.Comment, error)
	ListComments(string, string) ([]pullrequests.Comment, error)
	PutReview(string, string, string, pullrequests.ReviewDecision, string) (pullrequests.Review, error)
	DeleteReview(string, string, string) error
	ListReviews(string, string) ([]pullrequests.Review, error)
	MarkMerged(string, string, string, string) (pullrequests.PullRequest, error)
	Close(string, string, string) (pullrequests.PullRequest, error)
	SetMaintainerCanModify(string, string, bool) (pullrequests.PullRequest, error)
}

type pullRequestRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
	LinkObjects(storage.ID, storage.ID) error
}

type checkRunStarter interface {
	Start(string, string, string, string) error
}

type readinessCheckStore interface {
	List(string, string) ([]checkruns.Run, error)
}
type integrationQueueStore interface {
	Enqueue(string, string, string, string, string, string, string, string, []string) (integrationqueue.Entry, error)
	List(string, string) ([]integrationqueue.Entry, error)
	History(string, string) ([]integrationqueue.Entry, error)
	Get(string) (integrationqueue.Entry, error)
	Operate(string, string, string, int) (integrationqueue.Entry, error)
}

func registerPullRequestsHTTP(mux *http.ServeMux, store pullRequestStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, extras ...any) {
	var activity activityStore
	var checks checkRunStarter
	var checkResults readinessCheckStore
	var queue integrationQueueStore
	for _, extra := range extras {
		switch value := extra.(type) {
		case activityStore:
			activity = value
		case checkRunStarter:
			checks = value
		case readinessCheckStore:
			checkResults = value
		case integrationQueueStore:
			queue = value
		}
	}
	mux.HandleFunc("POST /repositories/{repository}/pull-requests", createPullRequest(store, proposalStore, repositories, credentials, activity, checks))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests", listPullRequests(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}", getPullRequest(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/synchronize", synchronizePullRequest(store, repositories, credentials, activity, checks))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/request-review", requestPullRequestReview(store, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/commits", listPullRequestCommits(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/files", listPullRequestFiles(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/comments", createPullRequestComment(store, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/comments", listPullRequestComments(store, repositories, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/pull-requests/{pull_request}/reviews/me", putPullRequestReview(store, repositories, credentials, activity))
	mux.HandleFunc("DELETE /repositories/{repository}/pull-requests/{pull_request}/reviews/me", deletePullRequestReview(store, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/reviews", listPullRequestReviews(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/readiness", getPullRequestReadiness(store, repositories, credentials, checkResults))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/merge", mergePullRequest(store, proposalStore, repositories, credentials, activity, checkResults))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/queue", enqueuePullRequest(store, repositories, credentials, activity, checkResults, checks, queue))
	mux.HandleFunc("GET /repositories/{repository}/integration-queue/entries", listIntegrationQueueEntries(repositories, credentials, checkResults, queue))
	mux.HandleFunc("PATCH /repositories/{repository}/integration-queue/entries/{entry}", operateIntegrationQueueEntry(repositories, credentials, activity, checks, queue))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/close", closePullRequest(store, proposalStore, repositories, credentials, activity))
	mux.HandleFunc("PUT /repositories/{repository}/pull-requests/{pull_request}/maintainer-modification", setMaintainerModification(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/source-credential", issuePullRequestSourceCredential(store, repositories, credentials))
}

func requestPullRequestReview(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if item.AuthorID != actor.UserID {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		item, err := store.RequestReview(string(repository.ID), item.ID)
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "review_not_accepted"})
			return
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.review_requested", Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: map[string]string{"source_commit_id": item.SourceCommitID}})
		writeJSON(w, http.StatusOK, item)
	}
}

func enqueuePullRequest(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, checkResults readinessCheckStore, starter checkRunStarter, queue integrationQueueStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		policy, protected := repository.IntegrationQueue[item.TargetBranch]
		if !protected || !policy.Enabled {
			writeJSON(w, 409, map[string]string{"error": "integration_queue_not_required"})
			return
		}
		if queue == nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		recorder := httptest.NewRecorder()
		getPullRequestReadiness(store, repositories, credentials, checkResults).ServeHTTP(recorder, r)
		if recorder.Code != 200 {
			for k, v := range recorder.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		var readiness readinessResponse
		if json.Unmarshal(recorder.Body.Bytes(), &readiness) != nil || !readiness.Ready {
			writeJSON(w, 409, map[string]any{"error": "pull_request_not_ready", "readiness": readiness})
			return
		}
		targetOpened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if item.SourceRepositoryID != item.RepositoryID {
			if err := repositories.LinkObjects(storage.ID(item.SourceRepositoryID), repository.ID); err != nil {
				writeJSON(w, 409, map[string]string{"error": "source_repository_unavailable"})
				return
			}
		}
		target := storage.ObjectID(readiness.TargetBranch.CommitID)
		source := storage.ObjectID(item.SourceCommitID)
		currentTarget, _, found := branchTip(targetOpened, item.TargetBranch)
		if !found || currentTarget != target {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "target_branch_changed"})
			return
		}
		mergeTree, err := materializeMergeTree(r.Context(), targetOpened, target, source)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		now := time.Now().UTC()
		identityTime := fmt.Sprintf("%d +0000", now.Unix())
		message := fmt.Sprintf("Integration candidate for %s\n\nPull-Request: %s\nSource-Repository: %s\nSource-Commit: %s\nTarget-Commit: %s\n", item.Title, item.ID, item.SourceRepositoryID, source, target)
		content := fmt.Sprintf("tree %s\nparent %s\nparent %s\nauthor %s <%s@users.local> %s\ncommitter %s <%s@users.local> %s\n\n%s", mergeTree, target, source, item.AuthorID, item.AuthorID, identityTime, actor.UserID, actor.UserID, identityTime, message)
		candidate, err := targetOpened.WriteObject(storage.CommitObject, []byte(content))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		currentTarget, _, found = branchTip(targetOpened, item.TargetBranch)
		if !found || currentTarget != target {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "target_branch_changed"})
			return
		}
		required := append([]string(nil), repository.RequiredChecks[item.TargetBranch]...)
		entry, err := queue.Enqueue(string(repository.ID), item.ID, item.TargetBranch, item.SourceCommitID, readiness.TargetBranch.CommitID, string(candidate), string(mergeTree), actor.UserID, required)
		if errors.Is(err, integrationqueue.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "pull_request_already_queued"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if starter != nil {
			_ = starter.Start(string(repository.ID), string(repository.ID), item.ID, string(candidate))
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "integration_queue.enqueued", Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: map[string]string{"entry_id": entry.ID, "branch": entry.TargetBranch, "position": strconv.Itoa(entry.Position)}})
		w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/integration-queue/entries")
		writeJSON(w, 201, queueEntryResponse(entry, nil))
	}
}
func listIntegrationQueueEntries(repositories pullRequestRepositoryStore, credentials authStore, checks readinessCheckStore, queue integrationQueueStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		branch := strings.TrimSpace(r.URL.Query().Get("branch"))
		if branch == "" {
			writeJSON(w, 422, map[string]string{"error": "branch_required"})
			return
		}
		if queue == nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items, err := queue.History(string(repository.ID), branch)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		var runs []checkruns.Run
		if checks != nil {
			for _, entry := range items {
				all, runErr := checks.List(string(repository.ID), entry.PullRequestID)
				if runErr != nil {
					writeJSON(w, 500, map[string]string{"error": "internal_error"})
					return
				}
				runs = append(runs, all...)
			}
		}
		responses := make([]integrationQueueEntryResponse, 0, len(items))
		for _, entry := range items {
			responses = append(responses, queueEntryResponse(entry, runs))
		}
		writeJSON(w, 200, map[string]any{"items": responses, "total_count": len(responses), "branch": branch, "policy": integrationPolicyResponse(repository, branch)})
	}
}

func operateIntegrationQueueEntry(repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, starter checkRunStarter, queue integrationQueueStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		entry, err := queue.Get(r.PathValue("entry"))
		if err != nil || entry.RepositoryID != string(repository.ID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Action   string `json:"action"`
			Position int    `json:"position"`
		}
		if !readJSON(w, r, &input, 1024) {
			return
		}
		if input.Action != "pause" && input.Action != "resume" && input.Action != "retry" && input.Action != "remove" && input.Action != "reprioritize" {
			writeJSON(w, 422, map[string]string{"error": "invalid_queue_action"})
			return
		}
		updated, err := queue.Operate(entry.ID, input.Action, actor.UserID, input.Position)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "queue_entry_not_active"})
			return
		}
		if input.Action == "retry" && starter != nil {
			_ = starter.Start(updated.RepositoryID, updated.RepositoryID, updated.PullRequestID, updated.CandidateCommitID)
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: updated.RepositoryID, ActorID: actor.UserID, Type: "integration_queue." + input.Action, Resource: activities.Resource{Type: "pull_request", ID: updated.PullRequestID}, Metadata: map[string]string{"entry_id": updated.ID, "branch": updated.TargetBranch, "position": strconv.Itoa(updated.Position)}})
		writeJSON(w, 200, queueEntryResponse(updated, nil))
	}
}

type integrationQueueEntryResponse struct {
	integrationqueue.Entry
	Checks     readinessChecks          `json:"checks"`
	History    []queueCandidateResponse `json:"attempt_history"`
	Blocker    string                   `json:"blocker,omitempty"`
	NextAction string                   `json:"next_action"`
}
type queueCandidateResponse struct {
	Generation        int             `json:"generation"`
	TargetCommitID    string          `json:"target_commit_id"`
	CandidateCommitID string          `json:"candidate_commit_id"`
	CreatedAt         time.Time       `json:"created_at"`
	Checks            readinessChecks `json:"checks"`
}

func queueEntryResponse(entry integrationqueue.Entry, runs []checkruns.Run) integrationQueueEntryResponse {
	matching := make([]checkruns.Run, 0)
	for _, run := range runs {
		if run.PullRequestID == entry.PullRequestID && run.CommitID == entry.CandidateCommitID {
			matching = append(matching, run)
		}
	}
	checks := evaluateRequiredChecks(entry.RequiredChecks, entry.CandidateCommitID, matching)
	checks.TargetBranch = entry.TargetBranch
	history := make([]queueCandidateResponse, 0, len(entry.Candidates))
	for _, candidate := range entry.Candidates {
		candidateRuns := make([]checkruns.Run, 0)
		for _, run := range runs {
			if run.PullRequestID == entry.PullRequestID && run.CommitID == candidate.CandidateCommitID {
				candidateRuns = append(candidateRuns, run)
			}
		}
		evaluated := evaluateRequiredChecks(entry.RequiredChecks, candidate.CandidateCommitID, candidateRuns)
		evaluated.TargetBranch = entry.TargetBranch
		history = append(history, queueCandidateResponse{Generation: candidate.Generation, TargetCommitID: candidate.TargetCommitID, CandidateCommitID: candidate.CandidateCommitID, CreatedAt: candidate.CreatedAt, Checks: evaluated})
	}
	next := "Waiting for earlier entries to finish."
	if entry.CompletedAt != nil {
		next = "No further automation is scheduled for this entry."
	} else if entry.State == "paused" {
		next = "A maintainer can resume or remove this entry."
	} else if entry.State == "blocked" {
		next = "A maintainer can retry or remove this entry."
	} else if entry.Position == 1 && checks.Satisfied {
		next = "The coordinator will atomically advance the target branch."
	} else if entry.Position == 1 {
		next = "Candidate checks must finish before publication."
	}
	response := integrationQueueEntryResponse{Entry: entry, Checks: checks, History: history, Blocker: entry.Reason, NextAction: next}
	if runs == nil {
		return response
	}
	if entry.CompletedAt != nil || entry.State == "paused" {
		return response
	}
	state := "passed"
	if !checks.Satisfied {
		state = "blocked"
		for _, requirement := range checks.Requirements {
			if requirement.Status == "pending" {
				state = "verifying"
				break
			}
		}
	}
	entry.State = state
	response.Entry = entry
	return response
}

type repositoryGitIssuer interface {
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
	RevokeRepositoryGit(string, string, string) error
}

func setMaintainerModification(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if actor.UserID != item.AuthorID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Allowed bool `json:"allowed"`
		}
		if !readJSON(w, r, &input, 1024) {
			return
		}
		updated, err := store.SetMaintainerCanModify(string(repository.ID), item.ID, input.Allowed)
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		if !input.Allowed {
			if issuer, ok := credentials.(repositoryGitIssuer); ok {
				_ = issuer.RevokeRepositoryGit(item.SourceRepositoryID, "refs/heads/"+item.SourceBranch, "pull request "+item.ID)
			}
		}
		writeJSON(w, 200, updated)
	}
}

func issuePullRequestSourceCredential(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if item.Status != pullrequests.Open || !item.MaintainerCanModify || actor.UserID == item.AuthorID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		participant := actor.UserID == repository.OwnerID
		if !participant {
			if reviews, err := store.ListReviews(string(repository.ID), item.ID); err == nil {
				for _, review := range reviews {
					participant = participant || review.ReviewerID == actor.UserID
				}
			}
			if comments, err := store.ListComments(string(repository.ID), item.ID); err == nil {
				for _, comment := range comments {
					participant = participant || comment.AuthorID == actor.UserID
				}
			}
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		issuer, ok := credentials.(repositoryGitIssuer)
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err := repositories.Inspect(storage.ID(item.SourceRepositoryID)); err != nil {
			writeJSON(w, 409, map[string]string{"error": "source_repository_unavailable"})
			return
		}
		issued, err := issuer.IssueRepositoryGit(actor.UserID, "pull request "+item.ID, item.SourceRepositoryID, "refs/heads/"+item.SourceBranch, 24*time.Hour)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, issued)
	}
}

func closePullRequest(store pullRequestStore, proposalsStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if actor.UserID != item.AuthorID && actor.UserID != repository.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		closed, err := store.Close(string(repository.ID), item.ID, actor.UserID)
		if errors.Is(err, pullrequests.ErrInvalid) {
			writeJSON(w, 409, map[string]string{"error": "pull_request_not_open"})
			return
		}
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		if issuer, ok := credentials.(repositoryGitIssuer); ok {
			_ = issuer.RevokeRepositoryGit(item.SourceRepositoryID, "refs/heads/"+item.SourceBranch, "pull request "+item.ID)
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.closed", Resource: activities.Resource{Type: "pull_request", ID: item.ID}})
		if item.ProposalID != "" && item.TaskID != "" && proposalsStore != nil {
			before, _ := proposalsStore.GetPlan(item.RepositoryID, item.ProposalID)
			_, _ = proposalsStore.UpdateTaskContribution(item.RepositoryID, item.ProposalID, item.TaskID, item.ID, actor.UserID, proposals.ContributionClosed)
			after, _ := proposalsStore.GetPlan(item.RepositoryID, item.ProposalID)
			recordTaskCoordinationChanges(activity, item.RepositoryID, actor.UserID, item.ProposalID, before, after)
		}
		writeJSON(w, 200, closed)
	}
}

func synchronizePullRequest(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, checks checkRunStarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, false)
		if !ok {
			return
		}
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if actor.UserID != item.AuthorID {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		sourceRepository, err := repositories.Inspect(storage.ID(item.SourceRepositoryID))
		if err != nil || (sourceRepository.ID != repository.ID && (sourceRepository.OwnerID != actor.UserID || sourceRepository.UpstreamID != repository.ID)) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		opened, err := repositories.Open(sourceRepository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitID, _, found := branchTip(opened, item.SourceBranch)
		if !found {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "source_branch_unavailable"})
			return
		}
		updated, err := store.SynchronizeSource(string(repository.ID), item.ID, string(commitID))
		if errors.Is(err, pullrequests.ErrInvalid) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_open"})
			return
		}
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		if updated.SourceCommitID != item.SourceCommitID {
			if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.synchronized", Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: map[string]string{"previous_commit_id": item.SourceCommitID, "commit_id": updated.SourceCommitID}}); err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			if checks != nil {
				_ = checks.Start(string(repository.ID), updated.SourceRepositoryID, item.ID, updated.SourceCommitID)
			}
		}
		writeJSON(w, http.StatusOK, updated)
	}
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
	Checks       readinessChecks    `json:"checks"`
	Blockers     []readinessBlocker `json:"blockers"`
}

type readinessCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	RunID    string `json:"run_id,omitempty"`
	CommitID string `json:"commit_id,omitempty"`
}
type readinessChecks struct {
	TargetBranch string           `json:"target_branch"`
	CommitID     string           `json:"commit_id"`
	Requirements []readinessCheck `json:"requirements"`
	Satisfied    bool             `json:"satisfied"`
}

func getPullRequestReadiness(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, checkStore readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		sourceOpened, err := repositories.Open(storage.ID(item.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 200, readinessResponse{CanMerge: actor.UserID != "" && actor.UserID == repository.OwnerID, SourceBranch: readinessBranch{Name: item.SourceBranch, SnapshotCommitID: item.SourceCommitID}, TargetBranch: readinessBranch{Name: item.TargetBranch, SnapshotCommitID: item.TargetCommitID}, Reviews: readinessReviews{RequiredOwnerApprovals: 1}, Checks: readinessChecks{TargetBranch: item.TargetBranch, CommitID: item.SourceCommitID, Requirements: []readinessCheck{}, Satisfied: false}, Blockers: []readinessBlocker{{Code: "source_repository_unavailable", Message: "The source repository is no longer available."}}})
			return
		}
		targetOpened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}

		response := readinessResponse{
			CanMerge:     actor.UserID != "" && actor.UserID == repository.OwnerID,
			HasConflicts: nil,
			SourceBranch: inspectReadinessBranch(sourceOpened, item.SourceBranch, item.SourceCommitID),
			TargetBranch: inspectReadinessBranch(targetOpened, item.TargetBranch, item.TargetCommitID),
			Reviews:      readinessReviews{RequiredOwnerApprovals: 1},
			Checks:       readinessChecks{TargetBranch: item.TargetBranch, CommitID: item.SourceCommitID, Requirements: []readinessCheck{}, Satisfied: true},
			Blockers:     []readinessBlocker{},
		}
		addBlocker := func(code, message string) {
			response.Blockers = append(response.Blockers, readinessBlocker{Code: code, Message: message})
		}
		if item.Status != pullrequests.Open {
			addBlocker("pull_request_not_open", "The pull request is not open.")
		}
		if item.Draft {
			addBlocker("pull_request_draft", "The task contribution is still a draft.")
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
		runs := []checkruns.Run{}
		if checkStore != nil {
			var err error
			runs, err = checkStore.List(string(repository.ID), item.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		response.Checks = evaluateRequiredChecks(repository.RequiredChecks[item.TargetBranch], item.SourceCommitID, runs)
		response.Checks.TargetBranch = item.TargetBranch
		for _, requirement := range response.Checks.Requirements {
			if requirement.Status != "succeeded" {
				addBlocker("required_check_"+requirement.Status, "Required check ‘"+requirement.Name+"’ is "+requirement.Status+" for revision "+item.SourceCommitID+".")
			}
		}

		if response.SourceBranch.Exists && response.SourceBranch.MatchesPullRequest && response.TargetBranch.Exists {
			hasConflicts, err := mergeHasConflictsAcross(r.Context(), targetOpened, sourceOpened, storage.ObjectID(response.TargetBranch.CommitID), storage.ObjectID(response.SourceBranch.CommitID))
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

func mergeHasConflictsAcross(ctx context.Context, targetRepository, sourceRepository *storage.Repository, target, source storage.ObjectID) (bool, error) {
	if targetRepository.ID() == sourceRepository.ID() {
		return mergeHasConflicts(ctx, targetRepository, target, source)
	}
	objectDirectory, err := os.MkdirTemp("", "pull-request-cross-readiness-objects-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(objectDirectory)
	if err := os.MkdirAll(filepath.Join(objectDirectory, "pack"), 0o750); err != nil {
		return false, err
	}
	command := exec.CommandContext(ctx, "git", "--git-dir="+targetRepository.GitDir(), "merge-tree", "--write-tree", "--quiet", "--allow-unrelated-histories", string(target), string(source))
	command.Env = append(os.Environ(), "GIT_OBJECT_DIRECTORY="+objectDirectory, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(targetRepository.GitDir(), "objects")+string(os.PathListSeparator)+filepath.Join(sourceRepository.GitDir(), "objects"))
	output, err := command.CombinedOutput()
	if err == nil {
		return false, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("inspect cross-repository merge conflicts: %w: %s", err, strings.TrimSpace(string(output)))
}

func evaluateRequiredChecks(required []string, commitID string, runs []checkruns.Run) readinessChecks {
	result := readinessChecks{CommitID: commitID, Requirements: []readinessCheck{}, Satisfied: true}
	for _, name := range required {
		entry := readinessCheck{Name: name, Status: "missing"}
		var stale *checkruns.Run
		for i := range runs {
			if runs[i].Definition.Name != name {
				continue
			}
			if stale == nil {
				stale = &runs[i]
			}
			if runs[i].CommitID == commitID {
				entry.RunID, entry.CommitID, entry.Status = runs[i].ID, runs[i].CommitID, string(runs[i].State)
				break
			}
		}
		if entry.Status == "missing" && stale != nil {
			entry.RunID, entry.CommitID, entry.Status = stale.ID, stale.CommitID, "stale"
		}
		if entry.Status == string(checkruns.Queued) || entry.Status == string(checkruns.Running) {
			entry.Status = "pending"
		}
		if entry.Status != "succeeded" {
			result.Satisfied = false
		}
		result.Requirements = append(result.Requirements, entry)
	}
	return result
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

func mergePullRequest(store pullRequestStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, checkStore readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if policy, protected := repository.IntegrationQueue[item.TargetBranch]; protected && policy.Enabled {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "integration_queue_required"})
			return
		}
		if item.Status != pullrequests.Open {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
			return
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		sourceOpened, err := repositories.Open(storage.ID(item.SourceRepositoryID))
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "source_repository_unavailable"})
			return
		}
		source, _, sourceOK := branchTip(sourceOpened, item.SourceBranch)
		target, _, targetOK := branchTip(opened, item.TargetBranch)
		if !sourceOK || !targetOK || string(source) != item.SourceCommitID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
			return
		}
		if item.SourceRepositoryID != item.RepositoryID {
			if err := repositories.LinkObjects(storage.ID(item.SourceRepositoryID), repository.ID); err != nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "source_repository_unavailable"})
				return
			}
		}
		reviews, err := store.ListReviews(string(repository.ID), item.ID)
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		ownerApproved := false
		for _, review := range reviews {
			if review.CommitID != string(source) {
				continue
			}
			if review.Decision == pullrequests.RequestChanges {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
				return
			}
			ownerApproved = ownerApproved || (review.ReviewerID == repository.OwnerID && review.Decision == pullrequests.Approve)
		}
		if !ownerApproved {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
			return
		}
		if required := repository.RequiredChecks[item.TargetBranch]; len(required) > 0 {
			if checkStore == nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
				return
			}
			runs, err := checkStore.List(string(repository.ID), item.ID)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			if !evaluateRequiredChecks(required, item.SourceCommitID, runs).Satisfied {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
				return
			}
		}
		hasConflicts, err := mergeHasConflicts(r.Context(), opened, target, source)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if hasConflicts {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_ready"})
			return
		}
		mergeTree, err := materializeMergeTree(r.Context(), opened, target, source)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		now := time.Now().UTC()
		message := item.Title
		if item.Body != "" {
			message += "\n\n" + item.Body
		}
		message += "\n\nPull-Request: " + item.ID + "\nAuthor-ID: " + item.AuthorID + "\nSource-Repository: " + item.SourceRepositoryID + "\nSource-Branch: " + item.SourceBranch + "\nSource-Commit: " + item.SourceCommitID + "\nMerged-By: " + actor.UserID
		if item.ProposalID != "" {
			message += "\nProposal: " + item.ProposalID
		}
		if item.TaskID != "" {
			message += "\nProposal-Task: " + item.TaskID
		}
		if item.ChangeSessionID != "" {
			message += "\nChange-Session: " + item.ChangeSessionID
		}
		identityTime := fmt.Sprintf("%d +0000", now.Unix())
		commitContent := fmt.Sprintf("tree %s\nparent %s\nparent %s\nauthor %s <%s@users.local> %s\ncommitter %s <%s@users.local> %s\n\n%s\n", mergeTree, target, source, item.AuthorID, item.AuthorID, identityTime, actor.UserID, actor.UserID, identityTime, message)
		mergeCommit, err := opened.WriteObject(storage.CommitObject, []byte(commitContent))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		// Recheck immediately before publishing so a concurrent target update is
		// never silently overwritten by the merge operation.
		currentTarget, _, found := branchTip(opened, item.TargetBranch)
		if !found || currentTarget != target {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "target_branch_changed"})
			return
		}
		if err := opened.UpdateReference(storage.Reference{Name: storage.ReferenceName("refs/heads/" + item.TargetBranch), ObjectID: mergeCommit}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		merged, err := store.MarkMerged(string(repository.ID), item.ID, actor.UserID, string(mergeCommit))
		if err != nil {
			writePullRequestError(w, err)
			return
		}
		if issuer, ok := credentials.(repositoryGitIssuer); ok {
			_ = issuer.RevokeRepositoryGit(item.SourceRepositoryID, "refs/heads/"+item.SourceBranch, "pull request "+item.ID)
		}
		if _, err := store.AddComment(string(repository.ID), item.ID, actor.UserID, "Merged into "+item.TargetBranch+" as "+string(mergeCommit)+"."); err != nil {
			writePullRequestError(w, err)
			return
		}
		if item.ProposalID != "" {
			if item.TaskID != "" {
				before, _ := proposalStore.GetPlan(item.RepositoryID, item.ProposalID)
				_, _ = proposalStore.UpdateTaskContribution(item.RepositoryID, item.ProposalID, item.TaskID, item.ID, actor.UserID, proposals.ContributionMerged)
				after, _ := proposalStore.GetPlan(item.RepositoryID, item.ProposalID)
				recordTaskCoordinationChanges(activity, item.RepositoryID, actor.UserID, item.ProposalID, before, after)
			} else if _, err := proposalStore.Close(string(repository.ID), item.ProposalID, actor.UserID); err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		metadata := map[string]string{"merge_commit_id": merged.MergeCommitID, "source_branch": item.SourceBranch, "target_branch": item.TargetBranch}
		if item.ProposalID != "" {
			metadata["proposal_id"] = item.ProposalID
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.merged", Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: metadata}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if item.ProposalID != "" {
			if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.closed", Resource: activities.Resource{Type: "proposal", ID: item.ProposalID}, Metadata: map[string]string{"pull_request_id": item.ID, "merge_commit_id": merged.MergeCommitID}}); err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		writeJSON(w, http.StatusOK, merged)
	}
}

// materializeMergeTree lets stock Git calculate a recursive merge in a
// disposable object directory, then imports every generated object through the
// repository's ObjectStore boundary.
func materializeMergeTree(ctx context.Context, repository *storage.Repository, target, source storage.ObjectID) (storage.ObjectID, error) {
	dir, err := os.MkdirTemp("", "pull-request-merge-objects-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Join(dir, "pack"), 0o750); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "git", "--git-dir="+repository.GitDir(), "merge-tree", "--write-tree", "--allow-unrelated-histories", string(target), string(source))
	command.Env = append(os.Environ(), "GIT_OBJECT_DIRECTORY="+dir, "GIT_ALTERNATE_OBJECT_DIRECTORIES="+filepath.Join(repository.GitDir(), "objects"))
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("create merge tree: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", errors.New("git did not return a merge tree")
	}
	treeID := storage.ObjectID(fields[0])
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Base(filepath.Dir(path)) == "pack" {
			return walkErr
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		reader, err := zlib.NewReader(file)
		if err != nil {
			file.Close()
			return err
		}
		canonical, err := io.ReadAll(reader)
		reader.Close()
		file.Close()
		if err != nil {
			return err
		}
		nul := bytes.IndexByte(canonical, 0)
		if nul < 0 {
			return errors.New("invalid generated git object")
		}
		header := string(canonical[:nul])
		typeName, sizeText, found := strings.Cut(header, " ")
		size, sizeErr := strconv.Atoi(sizeText)
		if !found || sizeErr != nil || size != len(canonical[nul+1:]) {
			return errors.New("invalid generated git object")
		}
		_, err = repository.WriteObject(storage.ObjectType(typeName), canonical[nul+1:])
		return err
	})
	if err != nil {
		return "", err
	}
	if _, err := repository.ReadTree(treeID); err != nil {
		return "", err
	}
	return treeID, nil
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

func putPullRequestReview(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, ok := readPullRequest(w, store, string(repository.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		wasReplacement := false
		if reviews, err := store.ListReviews(string(repository.ID), item.ID); err == nil {
			for _, existing := range reviews {
				if existing.ReviewerID == actor.UserID {
					wasReplacement = true
					break
				}
			}
		} else {
			writePullRequestError(w, err)
			return
		}
		var input struct {
			Decision pullrequests.ReviewDecision `json:"decision"`
		}
		if !readJSON(w, r, &input, 4<<10) {
			return
		}
		sourceOpened, err := repositories.Open(storage.ID(item.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitID, _, found := branchTip(sourceOpened, item.SourceBranch)
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
		eventType := "review.submitted"
		if wasReplacement {
			eventType = "review.replaced"
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: eventType, Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: map[string]string{"decision": string(review.Decision), "commit_id": review.CommitID}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, reviewResponse{Review: review, Stale: false})
	}
}

func deletePullRequestReview(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
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
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "review.withdrawn", Resource: activities.Resource{Type: "pull_request", ID: r.PathValue("pull_request")}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
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
		if opened, err := repositories.Open(storage.ID(item.SourceRepositoryID)); err == nil {
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

func createPullRequest(store pullRequestStore, proposalStore proposalStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore, checks checkRunStarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, false)
		if !ok {
			return
		}
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var input struct {
			ProposalID         string `json:"proposal_id"`
			Title              string `json:"title"`
			Body               string `json:"body"`
			SourceBranch       string `json:"source_branch"`
			TargetBranch       string `json:"target_branch"`
			SourceRepositoryID string `json:"source_repository_id"`
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
		sourceRepositoryID := repository.ID
		if input.SourceRepositoryID != "" {
			sourceRepositoryID = storage.ID(input.SourceRepositoryID)
		}
		sourceRepository, err := repositories.Inspect(sourceRepositoryID)
		if err != nil || (sourceRepositoryID != repository.ID && (sourceRepository.OwnerID != actor.UserID || sourceRepository.UpstreamID != repository.ID)) {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_repository"})
			return
		}
		if sourceRepositoryID == repository.ID {
			collaborator, collaboratorErr := repositories.IsCollaborator(repository.ID, actor.UserID)
			if actor.UserID != repository.OwnerID && (collaboratorErr != nil || !collaborator) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		sourceOpened, err := repositories.Open(sourceRepositoryID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		targetOpened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		source, sourceName, sourceOK := branchTip(sourceOpened, input.SourceBranch)
		target, targetName, targetOK := branchTip(targetOpened, input.TargetBranch)
		if !sourceOK || !targetOK || (sourceRepositoryID == repository.ID && sourceName == targetName) {
			writeJSON(w, 422, map[string]string{"error": "invalid_branches"})
			return
		}
		item, err := store.Create(pullrequests.CreateParams{RepositoryID: string(repository.ID), SourceRepositoryID: string(sourceRepositoryID), ProposalID: input.ProposalID, AuthorID: actor.UserID, Title: input.Title, Body: input.Body, SourceBranch: sourceName, TargetBranch: targetName, SourceCommitID: string(source), TargetCommitID: string(target)})
		if errors.Is(err, pullrequests.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		metadata := map[string]string{"title": item.Title, "source_repository_id": item.SourceRepositoryID, "source_branch": item.SourceBranch, "target_branch": item.TargetBranch}
		if item.ProposalID != "" {
			metadata["proposal_id"] = item.ProposalID
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.created", Resource: activities.Resource{Type: "pull_request", ID: item.ID}, Metadata: metadata, MentionText: item.Title + "\n" + item.Body}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if checks != nil {
			_ = checks.Start(string(repository.ID), item.SourceRepositoryID, item.ID, item.SourceCommitID)
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
		sourceOpened, err := repositories.Open(storage.ID(item.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		targetOpened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items, err := commitsBetweenRepositories(sourceOpened, targetOpened, storage.ObjectID(item.SourceCommitID), storage.ObjectID(item.TargetCommitID))
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
		sourceOpened, err := repositories.Open(storage.ID(item.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		targetOpened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		items, err := filesBetweenRepositories(sourceOpened, targetOpened, storage.ObjectID(item.SourceCommitID), storage.ObjectID(item.TargetCommitID))
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

func createPullRequestComment(store pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
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
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "pull_request.commented", Resource: activities.Resource{Type: "pull_request", ID: item.PullRequestID}, Metadata: map[string]string{"comment_id": item.ID}, MentionText: item.Body}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
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
