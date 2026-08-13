package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"net/http"
)

func registerProductExperimentsHTTP(mux *http.ServeMux, s *productexperiments.Store, repos proposalRepositoryStore, c authStore) {
	base := "/repositories/{repository}/product-experiments"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		perm := auth.RepositoryRead
		if write {
			perm = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, perm, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.PlanInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Create(repo, a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET "+base+"/{experiment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("experiment"))
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productexperiments.PlanInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Revise(repo, r.PathValue("experiment"), a, in.ExpectedVersion, in.PlanInput)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Comment(repo, r.PathValue("experiment"), a, in.Body)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Approve(repo, r.PathValue("experiment"), a, in.Decision, in.Note)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/assumption-changes", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Assumption string `json:"assumption"`
			Detail     string `json:"detail"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.ChangeAssumption(repo, r.PathValue("experiment"), a, in.Assumption, in.Detail)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	sig := base + "/signals"
	mux.HandleFunc("GET "+sig, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Signals(repo)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("POST "+sig, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.SignalVersion
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.CreateSignal(repo, a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+sig+"/{signal}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productexperiments.SignalVersion
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.ReviseSignal(repo, r.PathValue("signal"), a, in.ExpectedVersion, in.SignalVersion)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func experimentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	code, key := 500, "internal_error"
	if errors.Is(e, productexperiments.ErrInvalid) {
		code, key = 422, "invalid_product_experiment"
	}
	if errors.Is(e, productexperiments.ErrNotFound) {
		code, key = 404, "product_experiment_not_found"
	}
	if errors.Is(e, productexperiments.ErrConflict) {
		code, key = 409, "product_experiment_version_conflict"
	}
	writeJSON(w, code, map[string]string{"error": key})
	return true
}
