package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalcontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalimplementations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalrollouts"
)

type signalRolloutSources struct {
	contracts interface {
		Get(string, string) (signalcontracts.Contract, error)
	}
	runs interface {
		GetRun(string, string, string) (signalimplementations.Run, error)
	}
}

func registerSignalRolloutsHTTP(mux *http.ServeMux, s *signalrollouts.Store, repos performanceRepositoryStore, credentials authStore, src signalRolloutSources) {
	base := "/repositories/{repository}/signal-contracts/{contract}/rollouts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.PathValue("contract"))
		if signalRolloutError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalrollouts.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if in.ContractID != r.PathValue("contract") {
			writeJSON(w, 422, map[string]string{"error": "invalid_signal_rollout"})
			return
		}
		c, e := src.contracts.Get(string(repo.ID), in.ContractID)
		if e != nil || c.CurrentVersion != in.ContractVersion || !signalcontracts.Resolve(c).Complete || signalcontracts.Resolve(c).Blocked {
			writeJSON(w, 409, map[string]string{"error": "signal_rollout_contract_not_current"})
			return
		}
		run, e := src.runs.GetRun(string(repo.ID), in.PullRequestID, in.ImplementationRunID)
		if e != nil || !run.Passed || run.ContractID != in.ContractID || run.ContractVersion != in.ContractVersion || run.CandidateRevision != in.DeployedRevision {
			writeJSON(w, 409, map[string]string{"error": "passing_reviewed_instrumentation_required"})
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if signalRolloutError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{rollout}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("rollout"))
		if signalRolloutError(w, e) {
			return
		}
		if x.ContractID != r.PathValue("contract") {
			writeJSON(w, 404, map[string]string{"error": "signal_rollout_not_found"})
			return
		}
		writeJSON(w, 200, x)
	})
	post := func(suffix string, fn func(string, string, string, string, *http.Request) (signalrollouts.Rollout, error)) {
		mux.HandleFunc("POST "+base+"/{rollout}"+suffix, func(w http.ResponseWriter, r *http.Request) {
			repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
			if !ok {
				return
			}
			kind := r.Header.Get("X-Actor-Kind")
			if kind == "" {
				kind = "human"
			}
			x, e := fn(string(repo.ID), r.PathValue("rollout"), kind, a.UserID, r)
			if signalRolloutError(w, e) {
				return
			}
			if x.ContractID != r.PathValue("contract") {
				writeJSON(w, 404, map[string]string{"error": "signal_rollout_not_found"})
				return
			}
			writeJSON(w, 201, x)
		})
	}
	post("/observations", func(repo, rid, kind, actor string, r *http.Request) (signalrollouts.Rollout, error) {
		var in signalrollouts.ObservationInput
		if e := jsonBody(r, &in); e != nil {
			return signalrollouts.Rollout{}, signalrollouts.ErrInvalid
		}
		return s.Observe(repo, rid, kind, actor, in)
	})
	post("/controls", func(repo, rid, kind, actor string, r *http.Request) (signalrollouts.Rollout, error) {
		var in signalrollouts.ControlInput
		if e := jsonBody(r, &in); e != nil {
			return signalrollouts.Rollout{}, signalrollouts.ErrInvalid
		}
		return s.Control(repo, rid, kind, actor, in)
	})
}
func signalRolloutError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, signalrollouts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "signal_rollout_not_found"})
	case errors.Is(e, signalrollouts.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_signal_rollout"})
	case errors.Is(e, signalrollouts.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "signal_rollout_changed"})
	case errors.Is(e, signalrollouts.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "signal_rollout_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
