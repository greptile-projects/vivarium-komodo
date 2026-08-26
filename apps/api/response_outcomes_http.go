package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responseoutcomes"
)

func registerResponseOutcomesHTTP(mux *http.ServeMux, s *responseoutcomes.Store, alerts *responsealerts.Store, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/response-outcomes"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, summary, e := s.List(string(repo.ID), a.UserID, a.UserID == string(repo.OwnerID))
		if !responseOutcomeError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs, "summary": summary})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responseoutcomes.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		alert, e := alerts.Get(string(repo.ID), in.AlertID)
		if e != nil {
			responseOutcomeError(w, responseoutcomes.ErrInvalid)
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in, alert)
		if !responseOutcomeError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{outcome}/reviews", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64  `json:"expected_revision"`
			Decision         string `json:"decision"`
			Rationale        string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Review(string(repo.ID), r.PathValue("outcome"), a.UserID, in.ExpectedRevision, in.Decision, in.Rationale)
		if !responseOutcomeError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{outcome}/corrections", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			responseoutcomes.Correction
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Correct(string(repo.ID), r.PathValue("outcome"), a.UserID, in.ExpectedRevision, in.Correction)
		if !responseOutcomeError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{outcome}/corrections/{correction}/approval", func(w http.ResponseWriter, r *http.Request) {
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
		x, e := s.Approve(string(repo.ID), r.PathValue("outcome"), r.PathValue("correction"), a.UserID, in.ExpectedRevision)
		if !responseOutcomeError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{outcome}/work", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			responseoutcomes.Work
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddWork(string(repo.ID), r.PathValue("outcome"), a.UserID, in.ExpectedRevision, in.Work)
		if !responseOutcomeError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func responseOutcomeError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, responseoutcomes.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "response_outcome_not_found"})
	case errors.Is(e, responseoutcomes.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "response_outcome_changed"})
	case errors.Is(e, responseoutcomes.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_response_outcome"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
