package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapdelivery"
)

func registerRoadmapDeliveryHTTP(mux *http.ServeMux, s *roadmapdelivery.Store, roadmaps *productroadmaps.Store, plans *proposals.Store, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/product-roadmaps/{roadmap}/outcomes/{outcome}/delivery"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		p := auth.RepositoryRead
		if write {
			p = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, p, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	resolve := func(repo, rid, oid string) (productroadmaps.Roadmap, productroadmaps.Outcome, bool) {
		v, e := roadmaps.Get(repo, rid)
		if e != nil {
			return v, productroadmaps.Outcome{}, false
		}
		for _, o := range v.Versions[len(v.Versions)-1].Outcomes {
			if o.ID == oid && o.Decision == "accepted" {
				return v, o, true
			}
		}
		return v, productroadmaps.Outcome{}, false
	}
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in roadmapdelivery.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		rm, o, yes := resolve(repo, r.PathValue("roadmap"), r.PathValue("outcome"))
		if !yes || in.OutcomeID != o.ID || !roadmapdelivery.Validate(in, o.SuccessMeasures) {
			writeJSON(w, 422, map[string]string{"error": "invalid_roadmap_delivery"})
			return
		}
		p, e := plans.Create(repo, actor, "Deliver roadmap outcome: "+o.Title, "Traces implementation and measured value to roadmap outcome "+o.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ids := []string{}
		evidence := []string{}
		for i, t := range in.Tasks {
			deps := []string{}
			for _, d := range t.DependsOn {
				deps = append(deps, ids[d-1])
			}
			made, er := plans.CreateTask(repo, p.ID, actor, proposals.TaskInput{Title: t.Title, Outcome: o.Title, OwnerKind: t.OwnerKind, OwnerID: t.OwnerID, CompletionCriteria: t.AcceptanceCriteria, VerificationPlan: t.SuccessMeasures, BaseRevision: in.BaseRevision, Position: i + 1, Status: proposals.TaskPlanned, DependsOn: deps})
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_roadmap_delivery_task"})
				return
			}
			ids = append(ids, made.ID)
			evidence = append(evidence, t.EvidenceIDs...)
		}
		v, e := s.Create(repo, rm.ID, rm.CurrentVersion, o.ID, o.OpportunityID, o.OpportunityVersion, p.ID, actor, in.BaseRevision, ids, o.SuccessMeasures, evidence)
		if deliveryError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"delivery": v, "proposal": p, "tasks": ids})
	})
	mux.HandleFunc("GET "+base+"/{delivery}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("delivery"))
		if deliveryError(w, e) {
			return
		}
		if v.RoadmapID != r.PathValue("roadmap") || v.OutcomeID != r.PathValue("outcome") {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{delivery}/links", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in roadmapdelivery.Link
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Report(repo, r.PathValue("delivery"), a, in)
		if deliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{delivery}/revisit-requests", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Reason      string   `json:"reason"`
			EvidenceIDs []string `json:"evidence_ids"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Revisit(repo, r.PathValue("delivery"), a, in.Reason, in.EvidenceIDs)
		if deliveryError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func deliveryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, roadmapdelivery.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "roadmap_delivery_not_found"})
	case errors.Is(e, roadmapdelivery.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "roadmap_delivery_conflict"})
	case errors.Is(e, roadmapdelivery.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_roadmap_delivery"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
