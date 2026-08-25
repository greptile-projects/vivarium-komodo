package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerReviewPlansHTTP(mux *http.ServeMux, plans *reviewplans.Store, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/review-plans"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		p, e := plans.Get(string(repo.ID), pull.ID)
		if errors.Is(e, reviewplans.ErrNotFound) {
			writeJSON(w, 200, map[string]any{"current_version": 0, "versions": []any{}, "blockers": []any{}, "stale": false})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, reviewplans.Derive(p, pull.SourceCommitID, pull.TargetCommitID))
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		if actor.UserID != pull.AuthorID && actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "pull_author_or_maintainer_required"})
			return
		}
		var body struct {
			ExpectedVersion int64             `json:"expected_version"`
			Plan            reviewplans.Input `json:"plan"`
		}
		if !readJSON(w, r, &body, 256<<10) {
			return
		}
		source, e := repos.Open(storage.ID(pull.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "source_context_inaccessible"})
			return
		}
		target, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		files, e := filesBetweenRepositories(source, target, storage.ObjectID(pull.SourceCommitID), storage.ObjectID(pull.TargetCommitID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "changed_code_inaccessible"})
			return
		}
		paths := make([]string, 0, len(files))
		for _, f := range files {
			paths = append(paths, f.Path)
		}
		p, e := plans.Publish(string(repo.ID), pull.ID, pull.SourceCommitID, pull.TargetCommitID, actor.UserID, paths, body.ExpectedVersion, body.Plan)
		if errors.Is(e, reviewplans.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "review_plan_version_conflict"})
			return
		}
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_review_plan"})
			return
		}
		writeJSON(w, 201, map[string]any{"plan": p, "authority_notice": "A review plan describes required expertise and evidence; it grants no repository, review, approval, merge, policy, secret, or operational authority."})
	})
}
