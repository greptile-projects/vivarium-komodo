package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/historyremediations"
	"net/http"
)

func registerHistoryRemediationsHTTP(mux *http.ServeMux, s *historyremediations.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/history-remediations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Catalog(string(repo.ID), a.UserID)
		if !historyRemediationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in historyremediations.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !historyRemediationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{remediation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("remediation"), a.UserID)
		if !historyRemediationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/reachability", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in historyremediations.ReachabilityInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddReachability(string(repo.ID), r.PathValue("remediation"), a.UserID, in)
		if !historyRemediationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/rewrite-rules", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in historyremediations.RewriteRuleInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddRewriteRule(string(repo.ID), r.PathValue("remediation"), a.UserID, in)
		if !historyRemediationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/rewrite-candidates", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in historyremediations.RewriteCandidateInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddCandidate(string(repo.ID), r.PathValue("remediation"), a.UserID, in)
		if !historyRemediationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/rewrite-rehearsals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in historyremediations.RehearsalInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddRehearsal(string(repo.ID), r.PathValue("remediation"), a.UserID, in)
		if !historyRemediationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func historyRemediationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, historyremediations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "history_remediation_not_found"})
	case errors.Is(e, historyremediations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_history_remediation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
