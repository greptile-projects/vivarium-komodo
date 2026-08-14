package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectfunds"
	"net/http"
)

type projectFundStore interface {
	Create(string, string, projectfunds.Terms) (projectfunds.Fund, error)
	Commit(string, string, string, projectfunds.TransferInput) (projectfunds.Fund, error)
	Reconcile(string, string, string, string, projectfunds.ReconcileInput) (projectfunds.Fund, error)
	Get(string, string) (projectfunds.Fund, error)
	List(string) ([]projectfunds.Fund, error)
}

func registerProjectFundsHTTP(mux *http.ServeMux, store projectFundStore, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/funds"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if fundError(w, e) {
			return
		}
		visible := items[:0]
		for _, f := range items {
			if f.Terms.LedgerVisibility == "public" || a.UserID != "" {
				visible = append(visible, f)
			}
		}
		writeJSON(w, 200, map[string]any{"items": visible})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in projectfunds.Terms
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		f, e := store.Create(string(repo.ID), a.UserID, in)
		if fundError(w, e) {
			return
		}
		writeJSON(w, 201, f)
	})
	mux.HandleFunc("GET "+base+"/{fund}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		f, e := store.Get(string(repo.ID), r.PathValue("fund"))
		if fundError(w, e) {
			return
		}
		writeJSON(w, 200, f)
	})
	mux.HandleFunc("POST "+base+"/{fund}/commitments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in projectfunds.TransferInput
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		f, e := store.Commit(string(repo.ID), r.PathValue("fund"), a.UserID, in)
		if fundError(w, e) {
			return
		}
		writeJSON(w, 201, f)
	})
	mux.HandleFunc("POST "+base+"/{fund}/transfers/{transfer}/reconcile", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in projectfunds.ReconcileInput
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		f, e := store.Reconcile(string(repo.ID), r.PathValue("fund"), r.PathValue("transfer"), a.UserID, in)
		if fundError(w, e) {
			return
		}
		writeJSON(w, 200, f)
	})
}
func fundError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status, code := 500, "internal_error"
	switch {
	case errors.Is(e, projectfunds.ErrNotFound):
		status, code = 404, "not_found"
	case errors.Is(e, projectfunds.ErrInvalid):
		status, code = 422, "invalid_fund"
	case errors.Is(e, projectfunds.ErrConflict):
		status, code = 409, "fund_conflict"
	case errors.Is(e, projectfunds.ErrForbidden):
		status, code = 403, "forbidden"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
