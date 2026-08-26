package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
)

type runbookRehearsalStore interface {
	Create(string, string, runbookrehearsals.Input) (runbookrehearsals.Rehearsal, error)
	AppendAttempt(string, string, string, runbookrehearsals.AttemptInput) (runbookrehearsals.Rehearsal, error)
	Observe(string, string, string, runbookrehearsals.ObservationInput) (runbookrehearsals.Rehearsal, error)
	Get(string, string) (runbookrehearsals.Rehearsal, error)
	List(string, string) ([]runbookrehearsals.Rehearsal, error)
}
type selectedRunbookStore interface {
	Get(string, string) (runbooks.Runbook, error)
}

func registerRunbookRehearsalsHTTP(mux *http.ServeMux, s runbookRehearsalStore, books selectedRunbookStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/runbooks/{runbook}/rehearsals"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.PathValue("runbook"))
		if runbookRehearsalError(w, e) {
			return
		}
		for i := range xs {
			xs[i] = runbookrehearsals.Resolve(xs[i])
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in runbookrehearsals.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		in.RunbookID = r.PathValue("runbook")
		book, e := books.Get(string(repo.ID), in.RunbookID)
		if e != nil {
			runbookError(w, e)
			return
		}
		found := false
		for _, v := range book.Versions {
			found = found || v.Number == in.RunbookVersion
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "invalid_runbook_revision"})
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !runbookRehearsalError(w, e) {
			writeJSON(w, 201, runbookrehearsals.Resolve(x))
		}
	})
	mux.HandleFunc("GET "+base+"/{rehearsal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("rehearsal"))
		if e == nil && x.RunbookID != r.PathValue("runbook") {
			e = runbookrehearsals.ErrNotFound
		}
		if !runbookRehearsalError(w, e) {
			writeJSON(w, 200, runbookrehearsals.Resolve(x))
		}
	})
	mux.HandleFunc("POST "+base+"/{rehearsal}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in runbookrehearsals.AttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		current, e := s.Get(string(repo.ID), r.PathValue("rehearsal"))
		if e == nil && current.RunbookID != r.PathValue("runbook") {
			e = runbookrehearsals.ErrNotFound
		}
		if runbookRehearsalError(w, e) {
			return
		}
		x, e := s.AppendAttempt(string(repo.ID), r.PathValue("rehearsal"), a.UserID, in)
		if !runbookRehearsalError(w, e) {
			writeJSON(w, 201, runbookrehearsals.Resolve(x))
		}
	})
	mux.HandleFunc("POST "+base+"/{rehearsal}/observations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in runbookrehearsals.ObservationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		current, e := s.Get(string(repo.ID), r.PathValue("rehearsal"))
		if e == nil && current.RunbookID != r.PathValue("runbook") {
			e = runbookrehearsals.ErrNotFound
		}
		if runbookRehearsalError(w, e) {
			return
		}
		x, e := s.Observe(string(repo.ID), r.PathValue("rehearsal"), a.UserID, in)
		if !runbookRehearsalError(w, e) {
			writeJSON(w, 201, runbookrehearsals.Resolve(x))
		}
	})
}
func runbookRehearsalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, runbookrehearsals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "runbook_rehearsal_not_found"})
	case errors.Is(e, runbookrehearsals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_runbook_rehearsal"})
	case errors.Is(e, runbookrehearsals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "runbook_rehearsal_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
