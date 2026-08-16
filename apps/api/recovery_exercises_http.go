package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	"net/http"
)

func registerRecoveryExercisesHTTP(mux *http.ServeMux, s *recoveryexercises.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/recovery-exercises"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if recoveryExerciseError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryexercises.LaunchInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Launch(string(repo.ID), a.UserID, in)
		if recoveryExerciseError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{exercise}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("exercise"))
		if recoveryExerciseError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{exercise}/result", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in recoveryexercises.ResultInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Record(string(repo.ID), r.PathValue("exercise"), a.UserID, in)
		if recoveryExerciseError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}
func recoveryExerciseError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, recoveryexercises.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "recovery_exercise_not_found"})
	case errors.Is(e, recoveryexercises.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_exercise"})
	case errors.Is(e, recoveryexercises.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "recovery_exercise_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
