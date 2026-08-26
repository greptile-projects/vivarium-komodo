package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
)

func registerResponseRotationsHTTP(mux *http.ServeMux, s *responserotations.Store, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/response-rotations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if !responseRotationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responserotations.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !responseRotationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{rotation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("rotation"))
		if !responseRotationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{rotation}/transfers", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responserotations.TransferInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Propose(string(repo.ID), r.PathValue("rotation"), a.UserID, in)
		if !responseRotationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{rotation}/transfers/{transfer}/accept", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Accept(string(repo.ID), r.PathValue("rotation"), r.PathValue("transfer"), a.UserID, in.ExpectedRevision)
		if !responseRotationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{rotation}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responserotations.EventInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Record(string(repo.ID), r.PathValue("rotation"), a.UserID, in)
		if !responseRotationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}

func responseRotationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, responserotations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "response_rotation_not_found"})
	case errors.Is(e, responserotations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_response_rotation"})
	case errors.Is(e, responserotations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "response_rotation_changed"})
	case errors.Is(e, responserotations.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "response_rotation_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
