package main

import (
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacydrift"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
)

func registerPrivacyDriftHTTP(mux *http.ServeMux, s *privacydrift.Store, repos proposalRepositoryStore, credentials authStore, commitments interface {
	Get(string, string) (datacommitments.Commitment, error)
}, plans proposalStore) {
	base := "/repositories/{repository}/privacy-drift"
	mux.HandleFunc("POST "+base+"/monitors", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in privacydrift.MonitorInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		c, e := commitments.Get(string(repo.ID), in.CommitmentID)
		if e != nil || in.CommitmentVersion < 1 || in.CommitmentVersion > int64(len(c.Versions)) {
			writeJSON(w, 422, map[string]string{"error": "exact_data_commitment_required"})
			return
		}
		for _, u := range in.DataUseIDs {
			found := false
			for _, x := range c.Versions[in.CommitmentVersion-1].DataUses {
				found = found || x.ID == u
			}
			if !found {
				writeJSON(w, 422, map[string]string{"error": "declared_data_use_required"})
				return
			}
		}
		x, e := s.CreateMonitor(string(repo.ID), actor.UserID, in)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_privacy_drift_monitor"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/signals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in privacydrift.SignalInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := s.Report(string(repo.ID), actor.UserID, in)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_or_unsanitized_privacy_signal"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, false)
		if !ok {
			return
		}
		m, d, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"monitors": m, "signals": d, "total_count": len(d), "projection": "authorized_collaborators", "raw_personal_data_retained": false})
	})
	mux.HandleFunc("POST "+base+"/signals/{signal}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Kind         string   `json:"kind"`
			Summary      string   `json:"summary"`
			TargetIDs    []string `json:"target_ids"`
			ResourceKind string   `json:"resource_kind"`
			ResourceID   string   `json:"resource_id"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, event, e := s.AddEvent(string(repo.ID), r.PathValue("signal"), actor.UserID, in.Kind, in.Summary, in.ResourceKind, in.ResourceID, in.TargetIDs)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_privacy_drift_event"})
			return
		}
		writeJSON(w, 201, map[string]any{"signal": x, "event": event, "authority": map[string]bool{"data": false, "environment": false, "extension": false}})
	})
	mux.HandleFunc("POST "+base+"/signals/{signal}/repairs", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title              string   `json:"title"`
			OwnerKind          string   `json:"owner_kind"`
			OwnerID            string   `json:"owner_id"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		_, signals, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		var drift *privacydrift.Signal
		for i := range signals {
			if signals[i].ID == r.PathValue("signal") {
				drift = &signals[i]
			}
		}
		if drift == nil {
			writeJSON(w, 404, map[string]string{"error": "privacy_drift_not_found"})
			return
		}
		if in.OwnerKind == "human" {
			participant, _ := repos.IsCollaborator(repo.ID, in.OwnerID)
			if in.OwnerID != repo.OwnerID && !participant {
				writeJSON(w, 422, map[string]string{"error": "assignee_not_participant"})
				return
			}
		} else if in.OwnerKind != "agent" || in.OwnerID != "codex" {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_owner"})
			return
		}
		if strings.TrimSpace(in.Title) == "" || len(in.AcceptanceCriteria) == 0 {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair"})
			return
		}
		p, e := plans.Create(string(repo.ID), actor.UserID, "Privacy repair: "+in.Title, "Correct sanitized production drift "+drift.ID+" for release "+drift.ReleaseID+" in "+drift.EnvironmentID+" through ordinary review, privacy verification, release, and deployment.")
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		task, e := plans.CreateTask(string(repo.ID), p.ID, actor.UserID, proposals.TaskInput{Title: in.Title, Outcome: drift.Expected, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: in.AcceptanceCriteria, BaseRevision: drift.ReleaseRevision})
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, e = plans.AssignTask(string(repo.ID), p.ID, task.ID, actor.UserID, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Correct privacy drift using only the retained sanitized aggregate evidence; return the change through ordinary review and verification.", RepositoryID: string(repo.ID), BaseRevision: drift.ReleaseRevision})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "repair_assignment_failed"})
			return
		}
		updated, e := s.LinkRepair(string(repo.ID), drift.ID, actor.UserID, privacydrift.Repair{OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, ProposalID: p.ID, TaskID: task.ID, BaseRevision: drift.ReleaseRevision, AcceptanceCriteria: in.AcceptanceCriteria})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "privacy_repair_exists"})
			return
		}
		writeJSON(w, 201, map[string]any{"signal": updated, "proposal": p, "task": task, "preloaded_evidence": drift.Evidence, "authority": map[string]bool{"data": false, "environment": false, "credential": false, "review": false, "merge": false}})
	})
}
