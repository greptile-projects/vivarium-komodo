package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositoryrestructuring"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerRepositoryRestructuringHTTP(mux *http.ServeMux, s *repositoryrestructuring.Store, repos codeIntelligenceStore, credentials authStore) {
	base := "/repositories/{repository}/restructuring-plans"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in repositoryrestructuring.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		// Verify exact provenance for the anchoring source. Other selected sources
		// remain declared with their access state; this plan never grants access.
		found := false
		opened, err := repos.Open(repo.ID)
		for _, source := range in.Sources {
			if source.RepositoryID == string(repo.ID) {
				found = true
				if err != nil {
					writeJSON(w, 422, map[string]string{"error": "invalid_source_revision"})
					return
				}
				if _, readErr := opened.ReadCommit(storage.ObjectID(source.Revision)); readErr != nil {
					writeJSON(w, 422, map[string]string{"error": "invalid_source_revision"})
					return
				}
			}
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "anchoring_source_required"})
			return
		}
		plan, err := s.Create(string(repo.ID), actor.UserID, in)
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := s.List(string(repo.ID))
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		plan, err := s.Get(string(repo.ID), r.PathValue("plan"))
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, 200, plan)
	})
	mux.HandleFunc("POST "+base+"/{plan}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in repositoryrestructuring.FindingInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		plan, err := s.AddFinding(string(repo.ID), r.PathValue("plan"), actor.UserID, in)
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	})
	mux.HandleFunc("POST "+base+"/{plan}/candidates", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in repositoryrestructuring.CandidateInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		plan, err := s.AddCandidate(string(repo.ID), r.PathValue("plan"), actor.UserID, in)
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	})
	mux.HandleFunc("POST "+base+"/{plan}/rehearsals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in repositoryrestructuring.RehearsalInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		plan, err := s.AddRehearsal(string(repo.ID), r.PathValue("plan"), actor.UserID, in)
		if restructuringError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, plan)
	})
}

func restructuringError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, repositoryrestructuring.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "restructuring_plan_not_found"})
	case errors.Is(err, repositoryrestructuring.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_restructuring_plan"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
