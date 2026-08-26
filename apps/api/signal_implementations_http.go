package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalcontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/signalimplementations"
)

type signalImplementationSources struct {
	contracts interface {
		Get(string, string) (signalcontracts.Contract, error)
	}
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
}

func registerSignalImplementationsHTTP(mux *http.ServeMux, s *signalimplementations.Store, repos performanceRepositoryStore, credentials authStore, src signalImplementationSources) {
	contractBase := "/repositories/{repository}/signal-contracts/{contract}/implementations"
	mux.HandleFunc("POST "+contractBase, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ContractVersion int64                        `json:"contract_version"`
			BaseRevision    string                       `json:"base_revision"`
			Work            []signalimplementations.Work `json:"work"`
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		c, e := src.contracts.Get(string(repo.ID), r.PathValue("contract"))
		if e != nil {
			signalImplementationError(w, e)
			return
		}
		resolved := signalcontracts.Resolve(c)
		if resolved.CurrentVersion != in.ContractVersion || !resolved.Complete || resolved.Blocked {
			writeJSON(w, 422, map[string]string{"error": "accepted_current_signal_contract_required"})
			return
		}
		p, e := s.CreatePlan(string(repo.ID), c.ID, in.ContractVersion, in.BaseRevision, a.UserID, in.Work)
		if signalImplementationError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	pullBase := "/repositories/{repository}/pull-requests/{pull}/telemetry-checks"
	mux.HandleFunc("GET "+pullBase, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.ListRuns(string(repo.ID), r.PathValue("pull"))
		if signalImplementationError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+pullBase+"/runs", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in signalimplementations.Run
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		p, e := src.pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.CandidateRevision {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		plan, e := s.GetPlan(string(repo.ID), in.PlanID)
		if e != nil || plan.ContractID != in.ContractID || plan.ContractVersion != in.ContractVersion {
			writeJSON(w, 422, map[string]string{"error": "accepted_implementation_plan_required"})
			return
		}
		c, e := src.contracts.Get(string(repo.ID), in.ContractID)
		if e != nil || c.CurrentVersion != in.ContractVersion || !signalcontracts.Resolve(c).Complete {
			writeJSON(w, 422, map[string]string{"error": "current_signal_contract_required"})
			return
		}
		in.RepositoryID = string(repo.ID)
		in.PullRequestID = p.ID
		in.CreatedByID = a.UserID
		run, e := s.CreateRun(in)
		if signalImplementationError(w, e) {
			return
		}
		writeJSON(w, 201, run)
	})
}

func signalImplementationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, signalimplementations.ErrNotFound), errors.Is(e, signalcontracts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "signal_implementation_not_found"})
	case errors.Is(e, signalimplementations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_signal_implementation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
