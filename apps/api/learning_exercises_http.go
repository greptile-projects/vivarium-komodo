package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerLearningExercisesHTTP(mux *http.ServeMux, attempts *learningexercises.Store, pathways *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore) {
	base := "/repositories/{repository}/learning-pathways/{pathway}/attempts"
	mux.HandleFunc("POST "+base, launchLearningExercise(attempts, pathways, repos, credentials))
	mux.HandleFunc("GET "+base, listLearningExercises(attempts, repos, credentials))
	mux.HandleFunc("GET "+base+"/{attempt}", getLearningExercise(attempts, repos, credentials))
	mux.HandleFunc("POST "+base+"/{attempt}/events", appendLearningExerciseEvent(attempts, repos, credentials))
}

func launchLearningExercise(attempts *learningexercises.Store, pathways *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			PathwayVersion int64  `json:"pathway_version"`
			ModuleID       string `json:"module_id"`
			ExerciseIndex  int    `json:"exercise_index"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := pathways.Get(string(repo.ID), r.PathValue("pathway"))
		if e != nil {
			learningExerciseError(w, e)
			return
		}
		var version *learningpathways.Version
		for i := range p.Versions {
			if p.Versions[i].Number == in.PathwayVersion {
				version = &p.Versions[i]
				break
			}
		}
		if version == nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_learning_exercise"})
			return
		}
		var module *learningpathways.Module
		for i := range version.Modules {
			if version.Modules[i].ID == in.ModuleID {
				module = &version.Modules[i]
				break
			}
		}
		if module == nil || in.ExerciseIndex < 0 || in.ExerciseIndex >= len(module.Exercises) {
			writeJSON(w, 422, map[string]string{"error": "invalid_learning_exercise"})
			return
		}
		revision := ""
		for _, resource := range module.Resources {
			if resource.Revision != "" {
				if revision == "" {
					revision = resource.Revision
				}
				if resource.Revision != revision {
					writeJSON(w, 422, map[string]string{"error": "mixed_exercise_revisions"})
					return
				}
			}
		}
		supported := false
		for _, v := range version.SupportedRevisions {
			supported = supported || v == revision
		}
		opened, e := repos.Open(repo.ID)
		if e != nil || !supported {
			writeJSON(w, 422, map[string]string{"error": "unsupported_exercise_revision"})
			return
		}
		if _, e = opened.ReadObject(storage.ObjectID(revision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "unavailable_exercise_revision"})
			return
		}
		a, e := attempts.Create(string(repo.ID), p.ID, version.Number, module.ID, in.ExerciseIndex, actor.UserID, revision, module.Exercises[in.ExerciseIndex])
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, a)
	}
}
func listLearningExercises(s *learningexercises.Store, repos contributorPathwayRepositories, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID), r.PathValue("pathway"), actor.UserID)
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}
}
func getLearningExercise(s *learningexercises.Store, repos contributorPathwayRepositories, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		a, e := s.Get(string(repo.ID), r.PathValue("pathway"), r.PathValue("attempt"))
		if e == nil && a.LearnerID != actor.UserID {
			e = learningexercises.ErrNotFound
		}
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, 200, a)
	}
}
func appendLearningExerciseEvent(s *learningexercises.Store, repos contributorPathwayRepositories, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in learningexercises.EventInput
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		a, e := s.Append(string(repo.ID), r.PathValue("pathway"), r.PathValue("attempt"), actor.UserID, in)
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	}
}
func learningExerciseError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, learningexercises.ErrNotFound), errors.Is(e, learningpathways.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "learning_exercise_not_found"})
	case errors.Is(e, learningexercises.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "learning_exercise_forbidden"})
	case errors.Is(e, learningexercises.ErrTerminal):
		writeJSON(w, 409, map[string]string{"error": "learning_exercise_terminal"})
	case errors.Is(e, learningexercises.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_learning_exercise"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
