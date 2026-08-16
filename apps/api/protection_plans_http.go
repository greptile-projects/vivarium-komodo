package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/protectionplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryobjectives"
)

func registerProtectionPlansHTTP(mux *http.ServeMux, s *protectionplans.Store, objectives *recoveryobjectives.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/protection-plans"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if protectionPlanError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in protectionplans.VersionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !protectionObjectiveVersion(w, objectives, string(repo.ID), in) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if protectionPlanError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("plan"))
		if protectionPlanError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{plan}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			protectionplans.VersionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !protectionObjectiveVersion(w, objectives, string(repo.ID), in.VersionInput) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("plan"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if protectionPlanError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{plan}/captures", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in protectionplans.CaptureInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Capture(string(repo.ID), r.PathValue("plan"), a.UserID, in)
		if protectionPlanError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}

func protectionObjectiveVersion(w http.ResponseWriter, s *recoveryobjectives.Store, repo string, in protectionplans.VersionInput) bool {
	x, e := s.Get(repo, in.ObjectiveID)
	if e != nil {
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_objective_reference"})
		return false
	}
	if in.ObjectiveVersion < 1 || in.ObjectiveVersion > int64(len(x.Versions)) {
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_objective_version"})
		return false
	}
	allowed := map[string]bool{}
	for _, r := range x.Versions[in.ObjectiveVersion-1].Resources {
		allowed[r.ID] = true
	}
	for _, id := range in.ResourceIDs {
		if !allowed[id] {
			writeJSON(w, 422, map[string]string{"error": "invalid_protected_resource"})
			return false
		}
	}
	return true
}
func protectionPlanError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, protectionplans.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "protection_plan_not_found"})
	case errors.Is(e, protectionplans.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_protection_plan"})
	case errors.Is(e, protectionplans.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "protection_plan_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
