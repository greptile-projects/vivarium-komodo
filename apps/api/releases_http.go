package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type releaseStore interface {
	Create(releases.CreateParams) (releases.Release, error)
	Get(string, string) (releases.Release, error)
	List(string) ([]releases.Release, error)
}

func registerReleasesHTTP(mux *http.ServeMux, store releaseStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/releases", createRelease(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases", listReleases(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}", getRelease(store, repositories, credentials))
}

func listReleases(store releaseStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
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
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	}
}

func getRelease(store releaseStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("release"))
		if errors.Is(err, releases.ErrNotFound) {
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

func createRelease(store releaseStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repository.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repository.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Version        string `json:"version"`
			Notes          string `json:"notes"`
			CommitID       string `json:"commit_id"`
			PriorReleaseID string `json:"prior_release_id"`
		}
		if !readJSON(w, r, &input, 70000) {
			return
		}
		input.CommitID = strings.TrimSpace(input.CommitID)
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		candidate := storage.ObjectID(input.CommitID)
		if _, err = opened.ReadCommit(candidate); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
			return
		}
		priorCommit := ""
		if input.PriorReleaseID != "" {
			prior, err := store.Get(string(repository.ID), input.PriorReleaseID)
			if errors.Is(err, releases.ErrNotFound) {
				writeJSON(w, 422, map[string]string{"error": "invalid_prior_release"})
				return
			}
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			priorCommit = prior.CommitID
		}
		reachable, err := commitSet(opened, candidate)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
			return
		}
		priorReachable := map[storage.ObjectID]bool{}
		if priorCommit != "" {
			if !reachable[storage.ObjectID(priorCommit)] {
				writeJSON(w, 422, map[string]string{"error": "prior_release_not_ancestor"})
				return
			}
			priorReachable, err = commitSet(opened, storage.ObjectID(priorCommit))
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		all, err := pulls.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		links := []releases.PullRequestLink{}
		proposalIDs, taskIDs, contributors := []string{}, []string{}, []string{}
		for _, pull := range all {
			merge := storage.ObjectID(pull.MergeCommitID)
			if pull.Status != pullrequests.Merged || !reachable[merge] || priorReachable[merge] {
				continue
			}
			links = append(links, releases.PullRequestLink{ID: pull.ID, Title: pull.Title, AuthorID: pull.AuthorID, MergeCommitID: pull.MergeCommitID})
			proposalIDs = append(proposalIDs, pull.ProposalID)
			taskIDs = append(taskIDs, pull.TaskID)
			contributors = append(contributors, pull.AuthorID)
		}
		item, err := store.Create(releases.CreateParams{RepositoryID: string(repository.ID), Version: input.Version, Notes: input.Notes, CommitID: input.CommitID, PriorReleaseID: input.PriorReleaseID, PriorCommitID: priorCommit, CreatedByID: actor.UserID, PullRequests: links, ProposalIDs: proposalIDs, TaskIDs: taskIDs, ContributorIDs: contributors})
		switch {
		case errors.Is(err, releases.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_release"})
		case errors.Is(err, releases.ErrVersionConflict):
			writeJSON(w, 409, map[string]string{"error": "version_taken"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		default:
			w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/releases/"+item.ID)
			writeJSON(w, 201, item)
		}
	}
}

func commitSet(repository storage.RepositoryStorage, root storage.ObjectID) (map[storage.ObjectID]bool, error) {
	seen := map[storage.ObjectID]bool{}
	pending := []storage.ObjectID{root}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[current] {
			continue
		}
		commit, err := repository.ReadCommit(current)
		if err != nil {
			return nil, err
		}
		seen[current] = true
		pending = append(pending, commit.Parents...)
	}
	return seen, nil
}
