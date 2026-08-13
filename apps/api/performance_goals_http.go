package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type performanceGoalStore interface {
	Create(string, string, performancegoals.VersionInput) (performancegoals.Goal, error)
	Revise(string, string, string, int64, performancegoals.VersionInput) (performancegoals.Goal, error)
	Measure(string, string, string, performancegoals.MeasurementInput) (performancegoals.Goal, error)
	RecordTrial(string, string, string, performancegoals.TrialInput) (performancegoals.Goal, error)
	Get(string, string) (performancegoals.Goal, error)
	List(string) ([]performancegoals.Goal, error)
}
type performanceReleaseStore interface {
	Get(string, string) (releases.Release, error)
}
type performanceRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerPerformanceGoalsHTTP(mux *http.ServeMux, store performanceGoalStore, repos performanceRepositoryStore, releaseStore performanceReleaseStore, credentials authStore) {
	base := "/repositories/{repository}/performance-goals"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if performanceGoalError(w, e) {
			return
		}
		for i := range items {
			items[i] = performancegoals.Resolve(items[i], time.Now().UTC())
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in performancegoals.VersionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		g, e := store.Create(string(repo.ID), a.UserID, in)
		if performanceGoalError(w, e) {
			return
		}
		writeJSON(w, 201, performancegoals.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("GET "+base+"/{goal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		g, e := store.Get(string(repo.ID), r.PathValue("goal"))
		if performanceGoalError(w, e) {
			return
		}
		writeJSON(w, 200, performancegoals.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("POST "+base+"/{goal}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			performancegoals.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		g, e := store.Revise(string(repo.ID), r.PathValue("goal"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if performanceGoalError(w, e) {
			return
		}
		writeJSON(w, 201, performancegoals.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("POST "+base+"/{goal}/measurements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in performancegoals.MeasurementInput
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		g, e := store.Measure(string(repo.ID), r.PathValue("goal"), a.UserID, in)
		if performanceGoalError(w, e) {
			return
		}
		writeJSON(w, 201, performancegoals.Resolve(g, time.Now().UTC()))
	})
	mux.HandleFunc("POST "+base+"/{goal}/trials", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in performancegoals.TrialInput
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, e = opened.ReadCommit(storage.ObjectID(in.Revision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "unknown_trial_revision"})
			return
		}
		if in.ReleaseID != "" {
			rel, x := releaseStore.Get(string(repo.ID), in.ReleaseID)
			if x != nil || rel.CommitID != in.Revision {
				writeJSON(w, 422, map[string]string{"error": "unattested_trial_release"})
				return
			}
		}
		g, e := store.RecordTrial(string(repo.ID), r.PathValue("goal"), a.UserID, in)
		if performanceGoalError(w, e) {
			return
		}
		writeJSON(w, 201, performancegoals.Resolve(g, time.Now().UTC()))
	})
}
func performanceGoalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, performancegoals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "performance_goal_not_found"})
	case errors.Is(e, performancegoals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_performance_goal"})
	case errors.Is(e, performancegoals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "performance_goal_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
