package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/localeplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
)

func registerLocalePlansHTTP(mux *http.ServeMux, s *localeplans.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/locale-plans"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if localePlanError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in localeplans.VersionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !localeRevisionsExist(repos, string(repo.ID), in.Resources) {
			writeJSON(w, 422, map[string]string{"error": "invalid_locale_resource_revision"})
			return
		}
		p, e := s.Create(string(repo.ID), a.UserID, in)
		if localePlanError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		p, e := s.Get(string(repo.ID), r.PathValue("plan"))
		if localePlanError(w, e) {
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST "+base+"/{plan}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			localeplans.VersionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !localeRevisionsExist(repos, string(repo.ID), in.Resources) {
			writeJSON(w, 422, map[string]string{"error": "invalid_locale_resource_revision"})
			return
		}
		p, e := s.Revise(string(repo.ID), r.PathValue("plan"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if localePlanError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("POST "+base+"/{plan}/coverage", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in localeplans.CoverageInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		opened, e := repos.Open(storage.ID(repo.ID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_locale_coverage_revision"})
			return
		}
		if _, e = opened.ReadCommit(storage.ObjectID(in.SourceRevision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_locale_coverage_revision"})
			return
		}
		p, e := s.RecordCoverage(string(repo.ID), r.PathValue("plan"), a.UserID, in)
		if localePlanError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
}
func localeRevisionsExist(repos dataFlowRepositories, repo string, resources []localeplans.Resource) bool {
	r, e := repos.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for _, x := range resources {
		if _, e = r.ReadCommit(storage.ObjectID(x.SourceRevision)); e != nil {
			return false
		}
	}
	return true
}
func localePlanError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, localeplans.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "locale_plan_not_found"})
	case errors.Is(e, localeplans.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_locale_plan"})
	case errors.Is(e, localeplans.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "locale_plan_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
