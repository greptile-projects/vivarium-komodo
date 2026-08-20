package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceevidence"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAssuranceEvidenceHTTP(mux *http.ServeMux, s *assuranceevidence.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/assurance-programs/{program}/evidence"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		audience := "public"
		if actor.UserID != "" {
			audience = "repository"
		}
		x, e := s.Catalog(string(repo.ID), r.PathValue("program"), audience)
		if !assuranceEvidenceError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/queries", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assuranceevidence.QueryInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateQuery(string(repo.ID), r.PathValue("program"), a.UserID, in)
		if !assuranceEvidenceError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/packages", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assuranceevidence.CollectInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Collect(string(repo.ID), r.PathValue("program"), a.UserID, in)
		if !assuranceEvidenceError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func assuranceEvidenceError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, assuranceevidence.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "assurance_program_not_found"})
	case errors.Is(e, assuranceevidence.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "assurance_evidence_conflict"})
	case errors.Is(e, assuranceevidence.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_assurance_evidence"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
