package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningexercises"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type learningAgentApprovals interface {
	GetOnboarding(string, string, string) (agentevaluations.Onboarding, error)
}

func registerLearningExercisesHTTP(mux *http.ServeMux, attempts *learningexercises.Store, pathways *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, agents learningAgentApprovals) {
	base := "/repositories/{repository}/learning-pathways/{pathway}/attempts"
	mux.HandleFunc("POST "+base, launchLearningExercise(attempts, pathways, repos, credentials))
	mux.HandleFunc("GET "+base, listLearningExercises(attempts, repos, credentials))
	mux.HandleFunc("GET "+base+"/{attempt}", getLearningExercise(attempts, repos, credentials))
	mux.HandleFunc("POST "+base+"/{attempt}/events", appendLearningExerciseEvent(attempts, repos, credentials))
	mux.HandleFunc("POST "+base+"/{attempt}/help", appendLearningHelp(attempts, pathways, repos, credentials, agents))
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
		a, e := attempts.Create(string(repo.ID), p.ID, version.Number, module.ID, in.ExerciseIndex, actor.UserID, revision, module.Exercises[in.ExerciseIndex], module.Resources)
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, a)
	}
}
func appendLearningHelp(s *learningexercises.Store, pathways *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, agents learningAgentApprovals) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in learningexercises.HelpInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if in.Kind == "question" && in.RecipientKind == "agent" {
			if agents == nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_agent_approval"})
				return
			}
			approval, err := agents.GetOnboarding("repository", string(repo.ID), in.AgentApprovalID)
			if err != nil || approval.State != "active" || approval.Identity != in.RecipientID {
				writeJSON(w, 422, map[string]string{"error": "invalid_agent_approval"})
				return
			}
		}
		if in.Kind == "guidance" {
			attempt, _ := s.Get(string(repo.ID), r.PathValue("pathway"), r.PathValue("attempt"))
			if attempt.HelpParticipants[actor.UserID] == "agent" {
				if agents == nil {
					writeJSON(w, 403, map[string]string{"error": "learning_agent_revoked"})
					return
				}
				approvalID := ""
				for _, entry := range attempt.HelpTimeline {
					if entry.Kind == "question" && entry.RecipientID == actor.UserID {
						approvalID = entry.AgentApprovalID
					}
				}
				approval, err := agents.GetOnboarding("repository", string(repo.ID), approvalID)
				if err != nil || approval.State != "active" || approval.Identity != actor.UserID {
					writeJSON(w, 403, map[string]string{"error": "learning_agent_revoked"})
					return
				}
			}
			opened, err := repos.Open(repo.ID)
			for _, citation := range in.Citations {
				if err != nil || citation.Path == "" || !repositoryPathExists(opened, citation.Revision, citation.Path) {
					writeJSON(w, 422, map[string]string{"error": "inaccessible_learning_evidence"})
					return
				}
			}
		}
		p, e := pathways.Get(string(repo.ID), r.PathValue("pathway"))
		if e != nil {
			learningExerciseError(w, e)
			return
		}
		attempt, e := s.Get(string(repo.ID), p.ID, r.PathValue("attempt"))
		if e != nil {
			learningExerciseError(w, e)
			return
		}
		mentors := map[string]bool{}
		for _, v := range p.Versions {
			if v.Number == attempt.PathwayVersion {
				for _, m := range v.MentorIDs {
					mentors[m] = true
				}
			}
		}
		a, e := s.Help(string(repo.ID), p.ID, r.PathValue("attempt"), actor.UserID, mentors, in)
		if learningExerciseError(w, e) {
			return
		}
		writeJSON(w, 201, a)
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
		a, e := s.View(string(repo.ID), r.PathValue("pathway"), r.PathValue("attempt"), actor.UserID)
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
