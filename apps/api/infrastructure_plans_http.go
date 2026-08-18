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
func (p infrastructurePlanPulls) MergedRevision(repo, pull string) (string, string, bool, error) {
	v, e := p.store.Get(repo, pull)
	return v.SourceCommitID, v.MergeCommitID, v.Status == pullrequests.Merged, e
}

type infrastructureExecutionEnvironments struct{ store deploymentStore }

func (e infrastructureExecutionEnvironments) ExecutionEnvironment(repo, environment string) (int, bool) {
	v, err := e.store.GetEnvironment(repo, environment)
	return v.RequiredApprovals, err == nil
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
	mux.HandleFunc("POST "+base+"/{plan}/rehearsals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in infrastructureplans.RehearsalInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.CreateRehearsal(repo, r.PathValue("pull"), r.PathValue("plan"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/rehearsals/{rehearsal}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in infrastructureplans.AttemptInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		v, e := s.RecordRehearsalAttempt(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("rehearsal"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions", func(w http.ResponseWriter, r *http.Request) {
		catalog, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		repo, actor := string(catalog.ID), a.UserID
		if catalog.OwnerID != actor {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in infrastructureplans.ExecutionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.StartExecution(repo, r.PathValue("pull"), r.PathValue("plan"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		v, e := s.ApproveExecution(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), actor)
		if !infraPlanError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/control", func(w http.ResponseWriter, r *http.Request) {
		catalog, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		repo, actor := string(catalog.ID), a.UserID
		if catalog.OwnerID != actor {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.ControlExecution(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), actor, in.Action, in.Reason)
		if !infraPlanError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/steps/{step}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in infrastructureplans.StepUpdate
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.UpdateExecutionStep(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), r.PathValue("step"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in infrastructureplans.VerificationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.VerifyExecution(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/monitoring", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in infrastructureplans.DriftInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.MonitorExecution(repo, r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), actor, in)
		if !infraPlanError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/executions/{execution}/monitoring/{assessment}/actions", func(w http.ResponseWriter, r *http.Request) {
		catalog, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if catalog.OwnerID != a.UserID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in infrastructureplans.DriftActionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.OpenDriftAction(string(catalog.ID), r.PathValue("pull"), r.PathValue("plan"), r.PathValue("execution"), r.PathValue("assessment"), a.UserID, in)
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
