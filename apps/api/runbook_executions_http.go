package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookexecutions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"net/http"
	"sort"
)

func registerRunbookExecutionsHTTP(mux *http.ServeMux, s *runbookexecutions.Store, rb *runbooks.Store, rh *runbookrehearsals.Store, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/runbook-executions"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.List(string(repo.ID))
		if !runbookExecutionError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("GET "+base+"/{execution}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("execution"))
		if !runbookExecutionError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in runbookexecutions.LaunchInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		book, e := rb.Get(string(repo.ID), in.RunbookID)
		if e != nil || in.RunbookVersion > book.CurrentVersion {
			runbookExecutionError(w, runbookexecutions.ErrInvalid)
			return
		}
		if in.RunbookVersion != book.CurrentVersion {
			in.RehearsalReady = false
		}
		in.RunbookFindings = nil
		for _, finding := range book.Findings {
			in.RunbookFindings = append(in.RunbookFindings, finding.Kind+": "+finding.Detail)
		}
		validatedRehearsal := false
		if in.RehearsalID != "" {
			proof, x := rh.Get(string(repo.ID), in.RehearsalID)
			proof = runbookrehearsals.Resolve(proof)
			validatedRehearsal = x == nil && proof.RunbookID == in.RunbookID && proof.RunbookVersion == in.RunbookVersion && proof.Revision == in.RehearsalRevision && proof.Ready
		}
		in.RehearsalReady = in.RunbookVersion == book.CurrentVersion && validatedRehearsal
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !runbookExecutionError(w, e) {
			status := 201
			if x.State == "blocked" {
				status = 422
			}
			writeJSON(w, status, x)
		}
	})
	mux.HandleFunc("POST "+base+"/recommendations", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var q struct {
			Origin         runbookexecutions.Origin `json:"origin"`
			ResourceKinds  []string                 `json:"resource_kinds"`
			ResourceIDs    []string                 `json:"resource_ids"`
			RequiredSkills []string                 `json:"required_skills"`
		}
		if !readJSON(w, r, &q, 1<<20) {
			return
		}
		books, e := rb.List(string(repo.ID))
		if e != nil {
			runbookExecutionError(w, e)
			return
		}
		out := []runbookexecutions.Candidate{}
		for _, b := range books {
			v := b.Versions[len(b.Versions)-1]
			c := runbookexecutions.Candidate{RunbookID: b.ID, RunbookVersion: v.Number, Name: v.Name}
			if runbookExecutionContains(q.ResourceKinds, v.Scope.Kind) {
				c.Score += 2
				c.MatchExplanation = append(c.MatchExplanation, "scope kind matches affected context")
			}
			if runbookExecutionContains(q.ResourceIDs, v.Scope.ResourceID) {
				c.Score += 4
				c.MatchExplanation = append(c.MatchExplanation, "exact affected resource matches runbook scope")
			}
			for _, skill := range q.RequiredSkills {
				if runbookExecutionContains(v.RequiredSkills, skill) {
					c.Score++
					c.MatchExplanation = append(c.MatchExplanation, "required responder skill matches: "+skill)
				}
			}
			rehearsals, _ := rh.List(string(repo.ID), b.ID)
			ready := false
			for _, proof := range rehearsals {
				proof = runbookrehearsals.Resolve(proof)
				if proof.RunbookVersion == v.Number && proof.Ready {
					ready = true
				}
			}
			if !ready {
				c.Blockers = append(c.Blockers, runbookexecutions.Blocker{Kind: "stale_or_missing_rehearsal", Subject: b.ID, Detail: "current runbook version lacks current rehearsal proof", Choices: []string{"rehearse current version", "select another runbook"}})
			}
			if len(b.Findings) > 0 {
				c.Blockers = append(c.Blockers, runbookexecutions.Blocker{Kind: "runbook_findings", Subject: b.ID, Detail: "current procedure has unresolved safety or accessibility findings", Choices: []string{"inspect findings", "select another runbook"}})
			}
			c.Eligible = c.Score > 0 && len(c.Blockers) == 0
			out = append(out, c)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
		if len(out) > 1 && out[0].Score == out[1].Score && out[0].Score > 0 {
			out[0].Choices = []string{"select this exact revision", "compare equally matched procedures"}
			out[1].Choices = []string{"select this exact revision", "compare equally matched procedures"}
		}
		writeJSON(w, 200, map[string]any{"origin": q.Origin, "items": out, "automatic_selection": false})
	})
}
func runbookExecutionContains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func runbookExecutionError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, runbookexecutions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "runbook_execution_not_found"})
	case errors.Is(e, runbookexecutions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_runbook_execution"})
	case errors.Is(e, runbookexecutions.ErrBlocked):
		writeJSON(w, 422, map[string]string{"error": "runbook_execution_blocked"})
	case errors.Is(e, runbookexecutions.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "duplicate_runbook_execution"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
