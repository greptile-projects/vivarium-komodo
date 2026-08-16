package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/recoveryimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryinvestigations"
)

func registerRecoveryImprovementsHTTP(mux *http.ServeMux, store *ri.Store, investigations *recoveryinvestigations.Store, exercises *recoveryexercises.Store, plans *proposals.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/recovery-improvements"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if recoveryImprovementError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs, "total_count": len(xs)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.CreateInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !ri.Valid(in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_recovery_improvement"})
			return
		}
		inv, e := investigations.Get(string(repo.ID), in.InvestigationID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "current_supported_finding_required"})
			return
		}
		supported := false
		for _, f := range inv.Findings {
			supported = supported || (f.ID == in.FindingID && f.Kind == "conclusion" && f.Verdict == "supported")
		}
		ex, xe := exercises.Get(string(repo.ID), inv.ExerciseID)
		if !supported || xe != nil || ex.Revision != inv.ExerciseRevision || !ex.Current {
			writeJSON(w, 422, map[string]string{"error": "current_supported_finding_required"})
			return
		}
		p, e := plans.Create(string(repo.ID), a.UserID, in.Title, "Recovery improvement from exercise "+inv.ExerciseID+" and cited finding "+in.FindingID+". Work remains subject to ordinary repository access, review, checks, integration, release, and approval controls.")
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ids := []string{}
		for i, t := range in.Tasks {
			deps := []string{}
			for _, d := range t.DependsOn {
				deps = append(deps, ids[d-1])
			}
			context := ""
			if t.ContextKind != "" {
				context = "; existing " + t.ContextKind + " " + t.ContextID
			}
			made, er := plans.CreateTask(string(repo.ID), p.ID, a.UserID, proposals.TaskInput{Title: t.Title, Outcome: "Repair recovery gap " + in.FindingID + context + " without protected-state authority", OwnerKind: t.OwnerKind, OwnerID: t.OwnerID, CompletionCriteria: t.AcceptanceCriteria, VerificationPlan: append(append([]string{}, t.AcceptanceCriteria...), "pass a fresh recovery exercise against the repaired protection plan"), BaseRevision: in.BaseRevision, Position: i + 1, Status: proposals.TaskPlanned, DependsOn: deps})
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_recovery_task"})
				return
			}
			ids = append(ids, made.ID)
		}
		v, e := store.Create(string(repo.ID), a.UserID, p.ID, inv.ExerciseID, inv.PlanID, inv.PlanVersion, ids, in)
		if recoveryImprovementError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"improvement": v, "proposal": p, "tasks": ids})
	})
	mux.HandleFunc("GET "+base+"/{improvement}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("improvement"))
		if recoveryImprovementError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{improvement}/links", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.Link
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.Link(string(repo.ID), r.PathValue("improvement"), a.UserID, in)
		if recoveryImprovementError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{improvement}/verification", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExerciseID string `json:"exercise_id"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("improvement"))
		if recoveryImprovementError(w, e) {
			return
		}
		ex, er := exercises.Get(string(repo.ID), in.ExerciseID)
		hasGovernedDelivery := false
		for _, l := range v.Links {
			hasGovernedDelivery = hasGovernedDelivery || map[string]bool{"pull_request": true, "policy_change": true}[l.Kind]
		}
		if er != nil || ex.Result == nil || !ex.Current || ex.PlanID != v.PlanID || ex.PlanVersion <= v.PlanVersion || !hasGovernedDelivery {
			writeJSON(w, 422, map[string]string{"error": "fresh_repaired_plan_exercise_required"})
			return
		}
		v, e = store.Verify(string(repo.ID), v.ID, a.UserID, ex.ID, ex.Status == "passed")
		if recoveryImprovementError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func recoveryImprovementError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, ri.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "recovery_improvement_not_found"})
	} else if errors.Is(e, ri.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_improvement"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
