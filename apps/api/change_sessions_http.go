package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type changeSessionStore interface {
	Create(string, string, string, string) (changesessions.Session, error)
	CreateWithCheckFailure(string, string, string, string, *changesessions.CheckFailure) (changesessions.Session, error)
	CreateWithDeploymentFailure(string, string, string, string, *changesessions.DeploymentFailure) (changesessions.Session, error)
	CreateForTask(string, string, string, string, changesessions.TaskContext) (changesessions.Session, error)
	Get(string, string, string) (changesessions.Session, error)
	List(string, string) ([]changesessions.Session, error)
	Events(string, string, string) ([]changesessions.Event, error)
	Delegate(string, string, string, changesessions.DelegateParams) (changesessions.Run, error)
	RevokeRunCredential(string, string, string, string, time.Time) (changesessions.Run, error)
	AppendRunEvent(string, string, string, string, string, map[string]string) (changesessions.Event, error)
	Intervene(string, string, string, string, string, string, string) (changesessions.Event, changesessions.Run, error)
	Publish(string, string, string, string, changesessions.Publication) (changesessions.Event, changesessions.Run, error)
	LinkTaskContribution(string, string, string, string) (changesessions.Session, error)
}

type changeSessionCredentialStore interface {
	authStore
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
}

func registerChangeSessionsHTTP(mux *http.ServeMux, sessions changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore, activity activityStore, checkStarters ...checkRunStarter) {
	var checks checkRunStarter
	if len(checkStarters) > 0 {
		checks = checkStarters[0]
	}
	base := "/repositories/{repository}/pull-requests/{pull_request}/change-sessions"
	mux.HandleFunc("POST "+base, createChangeSession(sessions, pulls, repositories, credentials, activity))
	mux.HandleFunc("GET "+base, listChangeSessions(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}", getChangeSession(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}/events", listChangeSessionEvents(sessions, pulls, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs", delegateChangeSession(sessions, pulls, repositories, credentials))
	mux.HandleFunc("DELETE "+base+"/{session}/runs/{run}/credential", revokeRunCredential(sessions, pulls, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/events", appendRunEvent(sessions, pulls, credentials))
	mux.HandleFunc("GET "+base+"/{session}/runs/{run}/control", getRunControl(sessions, pulls, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/publication", publishRun(sessions, pulls, repositories, credentials, activity, checks))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/interventions", interveneRun(sessions, pulls, repositories, credentials))
}

func publishRun(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore, activity activityStore, checks checkRunStarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
		if !ok {
			return
		}
		repositoryID, pullID, sessionID, runID := r.PathValue("repository"), r.PathValue("pull_request"), r.PathValue("session"), r.PathValue("run")
		session, err := store.Get(repositoryID, pullID, sessionID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var run *changesessions.Run
		for i := range session.Runs {
			if session.Runs[i].ID == runID {
				run = &session.Runs[i]
				break
			}
		}
		pull, err := pulls.Get(repositoryID, pullID)
		if run == nil || err != nil || grant.ID != run.CredentialGrantID || grant.RepositoryID != pull.SourceRepositoryID || grant.Branch != "refs/heads/"+run.WorkingBranch || run.WorkingBranch != pull.SourceBranch {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if pull.Status != pullrequests.Open {
			writeJSON(w, 409, map[string]string{"error": "pull_request_not_open"})
			return
		}
		var input struct {
			Summary  string   `json:"summary"`
			Checks   []string `json:"checks"`
			Concerns []string `json:"concerns"`
		}
		if !readJSON(w, r, &input, 128<<10) {
			return
		}
		opened, err := repositories.Open(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		tip, _, found := branchTip(opened, pull.SourceBranch)
		if !found || string(tip) == run.RevisionID {
			writeJSON(w, 409, map[string]string{"error": "source_branch_not_updated"})
			return
		}
		commits, err := commitsBetween(opened, tip, storage.ObjectID(run.RevisionID))
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "source_history_diverged"})
			return
		}
		reachable := map[storage.ObjectID]bool{}
		if err := walkCommits(opened, tip, reachable, nil); err != nil || !reachable[storage.ObjectID(run.RevisionID)] {
			writeJSON(w, 409, map[string]string{"error": "source_history_diverged"})
			return
		}
		files, err := filesBetween(opened, tip, storage.ObjectID(run.RevisionID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commitIDs := make([]string, len(commits))
		for i := range commits {
			commitIDs[i] = commits[i].ID
		}
		paths := make([]string, len(files))
		for i := range files {
			paths[i] = files[i].Path
		}
		publication := changesessions.Publication{Summary: input.Summary, CommitIDs: commitIDs, ChangedFiles: paths, Checks: input.Checks, Concerns: input.Concerns, SourceCommitID: string(tip)}
		event, publishedRun, err := store.Publish(repositoryID, pullID, sessionID, runID, publication)
		if errors.Is(err, changesessions.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_publication"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		updated, err := pulls.SynchronizeSource(repositoryID, pullID, string(tip))
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "pull_request_not_open"})
			return
		}
		if _, err := credentials.Revoke(run.InitiatorID, run.CredentialGrantID); err == nil {
			_, _ = store.RevokeRunCredential(repositoryID, pullID, sessionID, runID, time.Now())
		}
		_ = recordActivity(activity, activities.Input{RepositoryID: repositoryID, ActorID: run.InitiatorID, Type: "pull_request.synchronized", Resource: activities.Resource{Type: "pull_request", ID: pullID}, Metadata: map[string]string{"previous_commit_id": pull.SourceCommitID, "commit_id": string(tip), "session_id": sessionID, "run_id": runID, "agent": run.Agent}})
		if checks != nil {
			_ = checks.Start(repositoryID, pull.SourceRepositoryID, pullID, string(tip))
		}
		writeJSON(w, http.StatusCreated, map[string]any{"event": event, "run": publishedRun, "pull_request": updated})
	}
}

func getRunControl(store changeSessionStore, pulls pullRequestStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
		if !ok {
			return
		}
		repositoryID, pullID, sessionID, runID := r.PathValue("repository"), r.PathValue("pull_request"), r.PathValue("session"), r.PathValue("run")
		session, err := store.Get(repositoryID, pullID, sessionID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var run *changesessions.Run
		for i := range session.Runs {
			if session.Runs[i].ID == runID {
				run = &session.Runs[i]
				break
			}
		}
		pull, pullErr := pulls.Get(repositoryID, pullID)
		if run == nil || pullErr != nil || grant.ID != run.CredentialGrantID || grant.RepositoryID != pull.SourceRepositoryID || grant.Branch != "refs/heads/"+run.WorkingBranch {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		items := []changesessions.Event{}
		for _, event := range session.Events {
			if event.RunID == runID && event.Type == "run.intervention" {
				items = append(items, event)
			}
		}
		writeJSON(w, 200, map[string]any{"run_id": run.ID, "state": run.State, "interventions": items})
	}
}

func interveneRun(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actorID, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if !readJSON(w, r, &input, 12288) {
			return
		}
		event, run, err := store.Intervene(pull.RepositoryID, pull.ID, r.PathValue("session"), r.PathValue("run"), actorID, input.Type, input.Message)
		if errors.Is(err, changesessions.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if errors.Is(err, changesessions.ErrInvalid) {
			writeJSON(w, 409, map[string]string{"error": "invalid_run_transition"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if input.Type == "cancel" {
			if _, err := credentials.Revoke(run.InitiatorID, run.CredentialGrantID); err != nil && !errors.Is(err, auth.ErrNotFound) {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			_, _ = store.RevokeRunCredential(pull.RepositoryID, pull.ID, r.PathValue("session"), run.ID, time.Now())
		}
		writeJSON(w, http.StatusCreated, map[string]any{"event": event, "run": run})
	}
}

func appendRunEvent(store changeSessionStore, pulls pullRequestStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
		if !ok {
			return
		}
		repositoryID, pullID, sessionID, runID := r.PathValue("repository"), r.PathValue("pull_request"), r.PathValue("session"), r.PathValue("run")
		session, err := store.Get(repositoryID, pullID, sessionID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var run *changesessions.Run
		for i := range session.Runs {
			if session.Runs[i].ID == runID {
				run = &session.Runs[i]
				break
			}
		}
		pull, pullErr := pulls.Get(repositoryID, pullID)
		if run == nil || pullErr != nil || grant.ID != run.CredentialGrantID || grant.RepositoryID != pull.SourceRepositoryID || grant.Branch != "refs/heads/"+run.WorkingBranch {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Type     string            `json:"type"`
			Metadata map[string]string `json:"metadata"`
		}
		if !readJSON(w, r, &input, 16384) {
			return
		}
		event, err := store.AppendRunEvent(repositoryID, pullID, sessionID, runID, input.Type, input.Metadata)
		if errors.Is(err, changesessions.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_run_event"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		// A failed run is terminal. Revoke its branch credential immediately so a
		// crashed or abandoned worker cannot continue publishing after clients
		// have observed the failure.
		if input.Type == "run.failed" {
			if _, err := credentials.Revoke(run.InitiatorID, run.CredentialGrantID); err != nil && !errors.Is(err, auth.ErrNotFound) {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			_, _ = store.RevokeRunCredential(repositoryID, pullID, sessionID, runID, time.Now())
		}
		writeJSON(w, http.StatusCreated, event)
	}
}

var workingBranchPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,99}$`)

func delegateChangeSession(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actorID, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		session, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("session"))
		if errors.Is(err, changesessions.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Instructions  string   `json:"instructions"`
			RevisionID    string   `json:"revision_id"`
			ContextPaths  []string `json:"context_paths"`
			WorkingBranch string   `json:"working_branch"`
			Agent         string   `json:"agent"`
		}
		if !readJSON(w, r, &input, 16384) {
			return
		}
		input.WorkingBranch = strings.TrimPrefix(strings.TrimSpace(input.WorkingBranch), "refs/heads/")
		if input.Agent == "" {
			input.Agent = "codex"
		}
		if !workingBranchPattern.MatchString(input.WorkingBranch) || strings.Contains(input.WorkingBranch, "..") || input.WorkingBranch != pull.SourceBranch || len(input.ContextPaths) > 50 {
			writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
			return
		}
		for _, path := range input.ContextPaths {
			if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || len(path) > 500 {
				writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
				return
			}
		}
		if pull.SourceRepositoryID != pull.RepositoryID && actorID != pull.AuthorID {
			if !pull.MaintainerCanModify || !pullRequestParticipant(pulls, pull, repositories, actorID) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		if _, err := repositories.Inspect(storage.ID(pull.SourceRepositoryID)); err != nil {
			writeJSON(w, 409, map[string]string{"error": "source_repository_unavailable"})
			return
		}
		grantName := "Agent run " + session.ID
		if pull.SourceRepositoryID != pull.RepositoryID {
			// Share the pull-request grant name so disabling maintainer modification
			// or closing the request revokes human and agent branch access together.
			grantName = "pull request " + pull.ID
		}
		issued, err := credentials.IssueRepositoryGit(actorID, grantName, pull.SourceRepositoryID, "refs/heads/"+input.WorkingBranch, 24*time.Hour)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		run, err := store.Delegate(pull.RepositoryID, pull.ID, session.ID, changesessions.DelegateParams{InitiatorID: actorID, Agent: input.Agent, Instructions: input.Instructions, RevisionID: input.RevisionID, ContextPaths: input.ContextPaths, WorkingBranch: input.WorkingBranch, CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt})
		if err != nil {
			_, _ = credentials.Revoke(actorID, issued.ID)
			writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
			return
		}
		writeJSON(w, 201, map[string]any{"run": run, "credential": map[string]any{"token": issued.Token, "username": "agent", "expires_at": issued.ExpiresAt, "repository_id": pull.SourceRepositoryID, "branch": "refs/heads/" + input.WorkingBranch}})
	}
}

// pullRequestParticipant applies the established cross-repository delegated-write
// policy: the target owner or a collaborator already present in review/discussion.
func pullRequestParticipant(pulls pullRequestStore, pull pullrequests.PullRequest, repositories pullRequestRepositoryStore, actorID string) bool {
	repository, err := repositories.Inspect(storage.ID(pull.RepositoryID))
	if err != nil {
		return false
	}
	participant := actorID == repository.OwnerID
	if !participant {
		if reviews, err := pulls.ListReviews(pull.RepositoryID, pull.ID); err == nil {
			for _, review := range reviews {
				participant = participant || review.ReviewerID == actorID
			}
		}
		if comments, err := pulls.ListComments(pull.RepositoryID, pull.ID); err == nil {
			for _, comment := range comments {
				participant = participant || comment.AuthorID == actorID
			}
		}
	}
	return participant
}

func revokeRunCredential(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actorID, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		session, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("session"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var found *changesessions.Run
		for i := range session.Runs {
			if session.Runs[i].ID == r.PathValue("run") {
				found = &session.Runs[i]
				break
			}
		}
		if found == nil || found.InitiatorID != actorID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if _, err := credentials.Revoke(actorID, found.CredentialGrantID); err != nil && !errors.Is(err, auth.ErrNotFound) {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		_, err = store.RevokeRunCredential(pull.RepositoryID, pull.ID, session.ID, found.ID, time.Now())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.WriteHeader(204)
	}
}

func changeSessionContext(w http.ResponseWriter, r *http.Request, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, scope auth.Scope, requireActor bool) (pullrequests.PullRequest, string, bool) {
	repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, requireActor)
	if !ok {
		return pullrequests.PullRequest{}, "", false
	}
	pull, ok := readPullRequest(w, pulls, string(repository.ID), r.PathValue("pull_request"))
	if !ok {
		return pullrequests.PullRequest{}, "", false
	}
	return pull, actor.UserID, true
}

func createChangeSession(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actorID, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if pull.Status != pullrequests.Open {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_open"})
			return
		}
		item, err := store.Create(pull.RepositoryID, pull.ID, actorID, pull.SourceCommitID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: pull.RepositoryID, ActorID: actorID, Type: "change_session.started", Resource: activities.Resource{Type: "pull_request", ID: pull.ID}, Metadata: map[string]string{"session_id": item.ID, "source_commit_id": item.SourceCommitID}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		location := "/repositories/" + pull.RepositoryID + "/pull-requests/" + pull.ID + "/change-sessions/" + item.ID
		w.Header().Set("Location", location)
		writeJSON(w, http.StatusCreated, item)
	}
}

func listChangeSessions(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(pull.RepositoryID, pull.ID)
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

func getChangeSession(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("session"))
		if errors.Is(err, changesessions.ErrNotFound) {
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

func listChangeSessionEvents(store changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := changeSessionContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.Events(pull.RepositoryID, pull.ID, r.PathValue("session"))
		if errors.Is(err, changesessions.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
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
