package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookexecutions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbookrehearsals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/runbooks"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
		// The server, never the caller, freezes the executable path from the exact
		// published version so later runbook edits cannot change a live procedure.
		in.ActivePath = nil
		version := book.Versions[in.RunbookVersion-1]
		in.RunbookUseState, in.ApprovedFallbackID, in.ApprovedFallbackVersion, _ = s.RunbookStatus(string(repo.ID), in.RunbookID, in.RunbookVersion)
		in.OutcomeCriteria = nil
		criteria := map[string][]string{"health": version.HealthCriteria, "containment": version.ContainmentCriteria, "recovery": version.RecoveryCriteria, "communication": version.CommunicationCriteria, "rollback": version.RollbackCriteria}
		for _, kind := range []string{"health", "containment", "recovery", "communication", "rollback"} {
			if len(criteria[kind]) > 0 {
				in.OutcomeCriteria = append(in.OutcomeCriteria, runbookexecutions.OutcomeCriterion{Kind: kind, Description: strings.Join(criteria[kind], "; ")})
			}
		}
		for _, step := range version.Steps {
			humanDecision := step.Decision != nil && step.Decision.HumanRequired
			in.ActivePath = append(in.ActivePath, runbookexecutions.ProcedureStep{ID: step.ID, Kind: step.Kind, Title: step.Title, DependsOn: append([]string{}, step.DependsOn...), ExpectedEvidence: append([]string{}, step.ExpectedEvidence...), RequiredAuthority: append([]string{}, step.RequiredAuthority...), OwnerIDs: append([]string{}, step.OwnerIDs...), RollbackCriteria: append([]string{}, step.RollbackCriteria...), HumanDecision: humanDecision, Optional: step.Optional, PolicyPermitsSkip: step.PolicyPermitsSkip})
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
	mux.HandleFunc("POST "+base+"/{execution}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in runbookexecutions.ControlInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Control(string(repo.ID), r.PathValue("execution"), a.UserID, in)
		if !runbookExecutionError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{execution}/evaluation", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in runbookexecutions.EvaluationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Evaluate(string(repo.ID), r.PathValue("execution"), a.UserID, in)
		if !runbookExecutionError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{execution}/learning", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("execution"))
		if e != nil {
			runbookExecutionError(w, e)
			return
		}
		book, e := rb.Get(string(repo.ID), x.RunbookID)
		if e != nil {
			runbookExecutionError(w, runbookexecutions.ErrInvalid)
			return
		}
		version := book.Versions[x.RunbookVersion-1]
		owner := false
		for _, id := range version.OwnerIDs {
			if id == a.UserID {
				owner = true
			}
		}
		if !owner {
			runbookExecutionError(w, runbookexecutions.ErrForbidden)
			return
		}
		var in runbookexecutions.LearningInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if in.Action == "record_revision" {
			if in.ReviewedRunbookVersion != book.CurrentVersion || in.ReviewedRunbookVersion <= x.RunbookVersion {
				runbookExecutionError(w, runbookexecutions.ErrInvalid)
				return
			}
		}
		if in.Action == "record_fresh_rehearsal" {
			proof, err := rh.Get(string(repo.ID), in.FreshRehearsalID)
			proof = runbookrehearsals.Resolve(proof)
			if err != nil || proof.RunbookID != x.RunbookID || proof.RunbookVersion != x.ReviewedRunbookVersion || proof.Revision != in.FreshRehearsalRevision || !proof.Ready {
				runbookExecutionError(w, runbookexecutions.ErrInvalid)
				return
			}
		}
		if in.Action == "suspend" {
			fallback, err := rb.Get(string(repo.ID), in.FallbackRunbookID)
			if err != nil || in.FallbackRunbookVersion != fallback.CurrentVersion || len(fallback.Findings) > 0 {
				runbookExecutionError(w, runbookexecutions.ErrInvalid)
				return
			}
		}
		x, e = s.Learn(string(repo.ID), x.ID, a.UserID, in)
		if !runbookExecutionError(w, e) {
			writeJSON(w, 200, x)
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
			use, fallback, fallbackVersion, _ := s.RunbookStatus(string(repo.ID), b.ID, v.Number)
			if use == "suspended" {
				c.Blockers = append(c.Blockers, runbookexecutions.Blocker{Kind: "runbook_suspended", Subject: b.ID, Detail: "current use is suspended; approved fallback is " + fallback + " version " + strconv.FormatInt(fallbackVersion, 10), Choices: []string{"select approved fallback", "inspect corrected revision and fresh rehearsal"}})
			}
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
	case errors.Is(e, runbookexecutions.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "runbook_execution_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
