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
)

type changeSessionStore interface {
	Create(string, string, string, string) (changesessions.Session, error)
	Get(string, string, string) (changesessions.Session, error)
	List(string, string) ([]changesessions.Session, error)
	Events(string, string, string) ([]changesessions.Event, error)
	Delegate(string, string, string, changesessions.DelegateParams) (changesessions.Run, error)
	RevokeRunCredential(string, string, string, string, time.Time) (changesessions.Run, error)
	AppendRunEvent(string, string, string, string, string, map[string]string) (changesessions.Event, error)
}

type changeSessionCredentialStore interface {
	authStore
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
}

func registerChangeSessionsHTTP(mux *http.ServeMux, sessions changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials changeSessionCredentialStore, activity activityStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/change-sessions"
	mux.HandleFunc("POST "+base, createChangeSession(sessions, pulls, repositories, credentials, activity))
	mux.HandleFunc("GET "+base, listChangeSessions(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}", getChangeSession(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}/events", listChangeSessionEvents(sessions, pulls, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs", delegateChangeSession(sessions, pulls, repositories, credentials))
	mux.HandleFunc("DELETE "+base+"/{session}/runs/{run}/credential", revokeRunCredential(sessions, pulls, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/events", appendRunEvent(sessions, credentials))
}

func appendRunEvent(store changeSessionStore, credentials changeSessionCredentialStore) http.HandlerFunc {
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
		if run == nil || grant.ID != run.CredentialGrantID || grant.RepositoryID != repositoryID || grant.Branch != "refs/heads/"+run.WorkingBranch {
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
		if !workingBranchPattern.MatchString(input.WorkingBranch) || strings.Contains(input.WorkingBranch, "..") || input.WorkingBranch == pull.TargetBranch || len(input.ContextPaths) > 50 {
			writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
			return
		}
		for _, path := range input.ContextPaths {
			if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || len(path) > 500 {
				writeJSON(w, 422, map[string]string{"error": "invalid_delegation"})
				return
			}
		}
		issued, err := credentials.IssueRepositoryGit(actorID, "Agent run "+session.ID, pull.RepositoryID, "refs/heads/"+input.WorkingBranch, 24*time.Hour)
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
		writeJSON(w, 201, map[string]any{"run": run, "credential": map[string]any{"token": issued.Token, "username": "agent", "expires_at": issued.ExpiresAt, "repository_id": pull.RepositoryID, "branch": "refs/heads/" + input.WorkingBranch}})
	}
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
