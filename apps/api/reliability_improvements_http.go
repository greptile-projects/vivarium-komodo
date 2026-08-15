package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityimprovements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
)

func registerReliabilityImprovementsHTTP(mux *http.ServeMux, store *ri.Store, investigations *reliabilityinvestigations.Store, objectives *serviceobjectives.Store, plans *proposals.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/reliability-improvements"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.CreateInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		o, e := objectives.Get(string(repo.ID), in.ObjectiveID)
		if e != nil || o.CurrentVersion != in.ObjectiveVersion || !ri.Valid(in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_reliability_improvement"})
			return
		}
		if in.Source.Kind == "finding" {
			inv, er := investigations.Get(string(repo.ID), in.Source.ResourceID)
			valid := er == nil && inv.ObjectiveID == in.ObjectiveID && inv.ObjectiveVersion == in.ObjectiveVersion
			found := false
			for _, entry := range inv.Entries {
				found = found || (entry.ID == in.Source.EntryID && entry.Kind == "conclusion" && entry.Verdict == "supported" && !entry.Stale)
			}
			if !valid || !found {
				writeJSON(w, 422, map[string]string{"error": "current_supported_finding_required"})
				return
			}
		}
		p, e := plans.Create(string(repo.ID), actor.UserID, in.Title, "Reliability improvement for objective "+in.ObjectiveID+" from "+in.Source.Kind+":"+in.Source.ResourceID+"; affected revisions: "+strings.Join(in.AffectedRevisions, ", ")+"; evidence: "+strings.Join(in.EvidenceIDs, ", ")+"; dependencies: "+strings.Join(in.DependencyContext, ", "))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ids := []string{}
		for i, t := range in.Tasks {
			deps := []string{}
			for _, position := range t.DependsOn {
				deps = append(deps, ids[position-1])
			}
			criteria := append([]string{}, t.AcceptanceCriteria...)
			criteria = append(criteria, in.AcceptanceCriteria...)
			made, er := plans.CreateTask(string(repo.ID), p.ID, actor.UserID, proposals.TaskInput{Title: t.Title, Outcome: "Improve " + strings.Join(in.JourneyIDs, ", ") + " against objective " + in.ObjectiveID + "; evidence " + strings.Join(append(in.EvidenceIDs, t.EvidenceIDs...), ", ") + "; dependencies " + strings.Join(append(in.DependencyContext, t.DependencyContext...), ", "), OwnerKind: t.OwnerKind, OwnerID: t.OwnerID, CompletionCriteria: criteria, VerificationPlan: criteria, Risk: t.Risk, BaseRevision: in.BaseRevision, Position: i + 1, Status: proposals.TaskPlanned, DependsOn: deps})
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_reliability_task"})
				return
			}
			ids = append(ids, made.ID)
		}
		v, e := store.Create(string(repo.ID), actor.UserID, p.ID, ids, in)
		if reliabilityImprovementError(w, e) {
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
		if reliabilityImprovementError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{improvement}/delivery-links", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.DeliveryLink
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.Link(string(repo.ID), r.PathValue("improvement"), a.UserID, in)
		if reliabilityImprovementError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{improvement}/rollouts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.RolloutInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := store.Rollout(string(repo.ID), r.PathValue("improvement"), a.UserID, in)
		if reliabilityImprovementError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func reliabilityImprovementError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, ri.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "reliability_improvement_not_found"})
	} else if errors.Is(e, ri.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_reliability_improvement"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
