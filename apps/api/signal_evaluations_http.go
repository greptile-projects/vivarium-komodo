package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/observabilitygaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalevaluations"
)

type signalEvaluationGapSource interface {
	Get(string, string) (observabilitygaps.Gap, error)
}

func registerSignalEvaluationsHTTP(mux *http.ServeMux, s *signalevaluations.Store, gaps signalEvaluationGapSource, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/observability-gaps/{gap}/signal-evaluations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.PathValue("gap"))
		if signalEvaluationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalevaluations.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		g, e := gaps.Get(string(repo.ID), r.PathValue("gap"))
		if e != nil || g.CurrentVersion != in.GapVersion {
			writeJSON(w, 409, map[string]string{"error": "current_observability_gap_required"})
			return
		}
		x, e := s.Create(string(repo.ID), g.ID, a.UserID, in)
		if signalEvaluationError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{evaluation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("evaluation"))
		if signalEvaluationError(w, e) {
			return
		}
		if x.GapID != r.PathValue("gap") {
			writeJSON(w, 404, map[string]string{"error": "signal_evaluation_not_found"})
			return
		}
		writeJSON(w, 200, x)
	})
	// Findings are evidence contributions: repository readers and bounded read-only agents may publish them.
	mux.HandleFunc("POST "+base+"/{evaluation}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in signalevaluations.FindingInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		kind := r.Header.Get("X-Actor-Kind")
		if kind == "" {
			kind = "human"
		}
		x, e := s.AddFinding(string(repo.ID), r.PathValue("gap"), r.PathValue("evaluation"), a.UserID, kind, in)
		if signalEvaluationError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{evaluation}/resolutions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalevaluations.ResolutionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, e := s.Resolve(string(repo.ID), r.PathValue("gap"), r.PathValue("evaluation"), a.UserID, in)
		if signalEvaluationError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{evaluation}/lifecycle-decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalevaluations.LifecycleInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, e := s.Lifecycle(string(repo.ID), r.PathValue("gap"), r.PathValue("evaluation"), a.UserID, in)
		if signalEvaluationError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}
func signalEvaluationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, signalevaluations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "signal_evaluation_not_found"})
	case errors.Is(e, signalevaluations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_signal_evaluation"})
	case errors.Is(e, signalevaluations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "signal_evaluation_changed"})
	case errors.Is(e, signalevaluations.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "signal_evaluation_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
