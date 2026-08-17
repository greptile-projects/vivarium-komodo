package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentdiscovery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAgentDiscoveryHTTP(m *http.ServeMux, s *agentdiscovery.Store, profiles *agentprofiles.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/agent-discovery"
	m.HandleFunc("POST "+base+"/evidence", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentdiscovery.EvidenceInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		p, e := profiles.Get(in.ProfileID)
		if e != nil {
			agentDiscoveryError(w, agentdiscovery.ErrInvalid)
			return
		}
		x, e := s.AddEvidence(string(repo.ID), a.UserID, in, p)
		if !agentDiscoveryError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/searches", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in agentdiscovery.SearchInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		ps, e := profiles.List()
		if e != nil {
			agentDiscoveryError(w, e)
			return
		}
		x, e := s.Search(string(repo.ID), a.UserID, in, ps)
		if !agentDiscoveryError(w, e) {
			w.Header().Set("Location", "/agent-searches/"+x.ID)
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/searches/{search}", func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(r.PathValue("search"), true)
		if !agentDiscoveryError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("GET /agent-searches/{search}", func(w http.ResponseWriter, r *http.Request) {
		x, e := s.Get(r.PathValue("search"), false)
		if !agentDiscoveryError(w, e) {
			writeJSON(w, 200, x)
		}
	})
}
func agentDiscoveryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentdiscovery.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_search_not_found"})
	case errors.Is(e, agentdiscovery.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_discovery"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
