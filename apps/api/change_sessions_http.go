package main

import (
	"errors"
	"net/http"

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
}

func registerChangeSessionsHTTP(mux *http.ServeMux, sessions changeSessionStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, activity activityStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/change-sessions"
	mux.HandleFunc("POST "+base, createChangeSession(sessions, pulls, repositories, credentials, activity))
	mux.HandleFunc("GET "+base, listChangeSessions(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}", getChangeSession(sessions, pulls, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}/events", listChangeSessionEvents(sessions, pulls, repositories, credentials))
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
