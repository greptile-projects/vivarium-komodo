package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/infrastructureplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
)

type infrastructurePlanPulls struct{ store *pullrequests.Store }

func (p infrastructurePlanPulls) CurrentRevision(repo, pull string) (string, error) {
	v, e := p.store.Get(repo, pull)
	return v.SourceCommitID, e
}

func registerInfrastructurePlansHTTP(mux *http.ServeMux, s *infrastructureplans.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/pull-requests/{pull}/infrastructure-plans"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, scope, write)
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
		v, e := s.List(repo, r.PathValue("pull"))
		if !infraPlanError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in infrastructureplans.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(repo, r.PathValue("pull"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("pull"), r.PathValue("plan"))
		if !infraPlanError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/annotations", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Kind              string   `json:"kind"`
			Body              string   `json:"body"`
			EvidenceReference string   `json:"evidence_reference"`
			ResourceIDs       []string `json:"resource_ids"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Annotate(repo, r.PathValue("pull"), r.PathValue("plan"), actor, in.Kind, in.Body, in.EvidenceReference, in.ResourceIDs)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			OwnerID     string   `json:"owner_id"`
			ResourceIDs []string `json:"resource_ids"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Request(repo, r.PathValue("pull"), r.PathValue("plan"), actor, in.OwnerID, in.ResourceIDs)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/acknowledgements/{ack}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Decide(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("ack"), actor, in.Decision, in.Rationale)
		if !infraPlanError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/invalidations", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Kind      string `json:"kind"`
			Reference string `json:"reference"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Invalidate(repo, r.PathValue("pull"), r.PathValue("plan"), actor, in.Kind, in.Reference)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func infraPlanError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, infrastructureplans.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "infrastructure_plan_not_found"})
	} else if errors.Is(e, infrastructureplans.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_infrastructure_plan"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
