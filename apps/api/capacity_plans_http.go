package main

import (
	"encoding/json"
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityplans"
	"net/http"
)

type capacityPlanStore interface {
	Create(string, string, capacityplans.Input) (capacityplans.Plan, error)
	Approve(string, string, string, capacityplans.ApprovalInput) (capacityplans.Plan, error)
	AddWork(string, string, string, capacityplans.WorkInput) (capacityplans.Plan, error)
	Decide(string, string, string, string, capacityplans.DecisionInput) (capacityplans.Plan, error)
	Get(string, string) (capacityplans.Plan, error)
	List(string) ([]capacityplans.Plan, error)
}

func registerCapacityPlansHTTP(mux *http.ServeMux, store capacityPlanStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/capacity-plans"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if capacityPlanError(w, e) {
			return
		}
		for i := range xs {
			xs[i] = capacityplans.Resolve(xs[i])
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in capacityplans.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		x, e := store.Create(string(repo.ID), a.UserID, in)
		if capacityPlanError(w, e) {
			return
		}
		writeJSON(w, 201, capacityplans.Resolve(x))
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := store.Get(string(repo.ID), r.PathValue("plan"))
		if capacityPlanError(w, e) {
			return
		}
		writeJSON(w, 200, capacityplans.Resolve(x))
	})
	post := func(suffix string, decode func(*http.Request, string, string, string) (capacityplans.Plan, error)) {
		mux.HandleFunc("POST "+base+"/{plan}"+suffix, func(w http.ResponseWriter, r *http.Request) {
			repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
			if !ok {
				return
			}
			x, e := decode(r, string(repo.ID), r.PathValue("plan"), a.UserID)
			if capacityPlanError(w, e) {
				return
			}
			writeJSON(w, 201, capacityplans.Resolve(x))
		})
	}
	post("/approvals", func(r *http.Request, repo, pid, actor string) (capacityplans.Plan, error) {
		var in capacityplans.ApprovalInput
		if e := jsonBody(r, &in); e != nil {
			return capacityplans.Plan{}, capacityplans.ErrInvalid
		}
		return store.Approve(repo, pid, actor, in)
	})
	post("/work", func(r *http.Request, repo, pid, actor string) (capacityplans.Plan, error) {
		var in capacityplans.WorkInput
		if e := jsonBody(r, &in); e != nil {
			return capacityplans.Plan{}, capacityplans.ErrInvalid
		}
		return store.AddWork(repo, pid, actor, in)
	})
	post("/decisions/{decision}", func(r *http.Request, repo, pid, actor string) (capacityplans.Plan, error) {
		var in capacityplans.DecisionInput
		if e := jsonBody(r, &in); e != nil {
			return capacityplans.Plan{}, capacityplans.ErrInvalid
		}
		return store.Decide(repo, pid, r.PathValue("decision"), actor, in)
	})
}

// These helpers let the compact route adapter share the service's bounded decoder.
type noopWriter struct{}

func (noopWriter) Header() http.Header       { return http.Header{} }
func (noopWriter) Write([]byte) (int, error) { return 0, nil }
func (noopWriter) WriteHeader(int)           {}
func jsonBody(r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(noopWriter{}, r.Body, 512<<10))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func capacityPlanError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capacityplans.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capacity_plan_not_found"})
	case errors.Is(e, capacityplans.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capacity_plan"})
	case errors.Is(e, capacityplans.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capacity_plan_changed"})
	case errors.Is(e, capacityplans.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capacity_plan_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
