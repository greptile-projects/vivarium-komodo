package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerLearningAssessmentsHTTP(mux *http.ServeMux, s *learningassessments.Store, pathways *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore) {
	base := "/repositories/{repository}/learning-pathways/{pathway}/assessments"
	mux.HandleFunc("POST "+base, publishLearningAssessment(s, pathways, repos, credentials))
	mux.HandleFunc("GET "+base, listLearningAssessments(s, repos, credentials))
	mux.HandleFunc("GET "+base+"/{assessment}", getLearningAssessment(s, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts", startLearningAssessment(s, pathways, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts/{attempt}/evidence", addLearningAssessmentEvidence(s, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts/{attempt}/judgments", judgeLearningAssessment(s, pathways, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts/{attempt}/accommodation", decideLearningAccommodation(s, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts/{attempt}/appeals", appealLearningAssessment(s, repos, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/attempts/{attempt}/appeals/{appeal}", decideLearningAppeal(s, repos, credentials))
}
func learningCurrent(pathways *learningpathways.Store, repos contributorPathwayRepositories, repo, pathway string) (int64, string, error) {
	p, e := pathways.Get(repo, pathway)
	if e != nil {
		return 0, "", e
	}
	r, e := repos.Inspect(repoID(repo))
	if e != nil {
		return 0, "", e
	}
	opened, e := repos.Open(r.ID)
	if e != nil {
		return 0, "", e
	}
	branch, e := opened.DefaultBranch()
	if e != nil {
		return 0, "", e
	}
	ref, e := opened.ReadReference(branch)
	if e != nil {
		return 0, "", e
	}
	return p.CurrentVersion, string(ref.ObjectID), nil
}

func repoID(value string) storage.ID { return storage.ID(value) }
func publishLearningAssessment(s *learningassessments.Store, p *learningpathways.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ID string `json:"id"`
			learningassessments.Definition
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		pv, rev, e := learningCurrent(p, repos, string(repo.ID), r.PathValue("pathway"))
		if e != nil || in.PathwayVersion != pv || in.Revision != rev {
			writeJSON(w, 422, map[string]string{"error": "stale_learning_assessment_inputs"})
			return
		}
		v, e := s.Publish(string(repo.ID), r.PathValue("pathway"), in.ID, a.UserID, in.Definition)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, true))
	}
}
func listLearningAssessments(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.PathValue("pathway"))
		if learningAssessmentError(w, e) {
			return
		}
		out := make([]learningassessments.Assessment, 0, len(xs))
		for _, x := range xs {
			out = append(out, learningassessments.Project(x, a.UserID, false))
		}
		writeJSON(w, 200, map[string]any{"items": out})
	}
}
func getLearningAssessment(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		v, e := s.Get(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"))
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 200, learningassessments.Project(v, a.UserID, false))
	}
}
func startLearningAssessment(s *learningassessments.Store, p *learningpathways.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in learningassessments.AttemptInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		pv, rev, e := learningCurrent(p, repos, string(repo.ID), r.PathValue("pathway"))
		if e != nil {
			learningAssessmentError(w, e)
			return
		}
		v, e := s.Start(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), a.UserID, in, pv, rev)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func addLearningAssessmentEvidence(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in learningassessments.Evidence
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddEvidence(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), r.PathValue("attempt"), a.UserID, in)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func judgeLearningAssessment(s *learningassessments.Store, p *learningpathways.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in learningassessments.Judgment
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		pv, rev, e := learningCurrent(p, repos, string(repo.ID), r.PathValue("pathway"))
		if e != nil {
			learningAssessmentError(w, e)
			return
		}
		v, e := s.Judge(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), r.PathValue("attempt"), a.UserID, in, pv, rev)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func decideLearningAccommodation(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Status    string `json:"status"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.DecideAccommodation(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), r.PathValue("attempt"), a.UserID, in.Status, in.Rationale)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func appealLearningAssessment(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Reason             string   `json:"reason"`
			EvidenceReferences []string `json:"evidence_references"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Appeal(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), r.PathValue("attempt"), a.UserID, in.Reason, in.EvidenceReferences)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func decideLearningAppeal(s *learningassessments.Store, repos contributorPathwayRepositories, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.DecideAppeal(string(repo.ID), r.PathValue("pathway"), r.PathValue("assessment"), r.PathValue("attempt"), r.PathValue("appeal"), a.UserID, in.Decision, in.Rationale)
		if learningAssessmentError(w, e) {
			return
		}
		writeJSON(w, 201, learningassessments.Project(v, a.UserID, false))
	}
}
func learningAssessmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, learningassessments.ErrNotFound), errors.Is(e, learningpathways.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "learning_assessment_not_found"})
	case errors.Is(e, learningassessments.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "learning_assessment_forbidden"})
	case errors.Is(e, learningassessments.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "learning_assessment_conflict"})
	case errors.Is(e, learningassessments.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_learning_assessment"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
