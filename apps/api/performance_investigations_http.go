package main

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/performancegoals"
	pi "github.com/greptile-projects/vivarium-komodo/apps/api/performanceinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type performanceInvestigationStore interface {
	Create(string, string, pi.CreateInput) (pi.Investigation, error)
	Get(string, string) (pi.Investigation, error)
	List(string) ([]pi.Investigation, error)
	Invite(string, string, string, string) (pi.Investigation, error)
	Add(string, string, string, pi.Entry) (pi.Investigation, error)
	AddChange(string, string, string, pi.Change) (pi.Investigation, error)
}

func registerPerformanceInvestigationsHTTP(mux *http.ServeMux, store performanceInvestigationStore, goals performanceGoalStore, repos performanceRepositoryStore, credentials authStore, extras ...any) {
	var plans proposalStore
	for _, extra := range extras {
		if v, ok := extra.(proposalStore); ok {
			plans = v
		}
	}
	base := "/repositories/{repository}/performance-investigations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := xs[:0]
		for _, v := range xs {
			if visible(v, a.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in pi.CreateInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		g, e := goals.Get(string(repo.ID), in.GoalID)
		if e != nil || g.CurrentVersion != in.GoalVersion {
			writeJSON(w, 422, map[string]string{"error": "invalid_goal_version"})
			return
		}
		trials := trialMap(g)
		for i := range in.Evidence {
			live, exists := trials[in.Evidence[i].TrialID]
			if !exists || live.Revision != in.Evidence[i].Revision || live.WorkloadSource != in.Evidence[i].WorkloadSource || live.EnvironmentDigest != in.Evidence[i].EnvironmentDigest {
				writeJSON(w, 422, map[string]string{"error": "invalid_trial_evidence"})
				return
			}
			if in.Evidence[i].Visibility == "" {
				in.Evidence[i].Visibility = "repository"
			}
		}
		v, e := store.Create(string(repo.ID), a.UserID, in)
		writePI(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{investigation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || !visible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		g, e := goals.Get(string(repo.ID), v.GoalID)
		if e == nil {
			v = pi.Resolve(v, g.CurrentVersion, trialMap(g))
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID string `json:"user_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		allowed := in.UserID == repo.OwnerID
		for _, x := range repo.CollaboratorIDs {
			allowed = allowed || x == in.UserID
		}
		if !allowed {
			writeJSON(w, 422, map[string]string{"error": "repository_participant_required"})
			return
		}
		v, e := store.Invite(string(repo.ID), r.PathValue("investigation"), a.UserID, in.UserID)
		writePI(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/entries", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || !participant(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in pi.Entry
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		g, e := goals.Get(string(repo.ID), v.GoalID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "goal_unavailable"})
			return
		}
		tm := trialMap(g)
		opened, _ := repos.Open(repo.ID)
		for i := range in.Citations {
			c := &in.Citations[i]
			switch c.Kind {
			case "trial", "profile", "trace", "operational_evidence":
				live, yes := tm[c.TrialID]
				captured, selected := selected(v, c.TrialID)
				if !yes || !selected {
					writeJSON(w, 422, map[string]string{"error": "unselected_evidence"})
					return
				}
				c.Revision = live.Revision
				if captured.Visibility == "participants" && in.Audience == "repository" {
					writeJSON(w, 422, map[string]string{"error": "restricted_evidence_cannot_propagate"})
					return
				}
			case "symbol", "code", "runtime_path":
				if c.Revision == "" {
					c.Revision = v.Evidence[0].Revision
				}
				if c.Revision != v.Evidence[0].Revision || c.Path != "" && (!safePath(c.Path) || !blobExists(opened, c.Revision, c.Path)) {
					writeJSON(w, 422, map[string]string{"error": "invalid_revision_citation"})
					return
				}
			case "dependency", "commit", "release":
				if c.Revision == "" && c.CommitID == "" && c.ReleaseID == "" {
					writeJSON(w, 422, map[string]string{"error": "invalid_project_citation"})
					return
				}
			default:
				writeJSON(w, 422, map[string]string{"error": "invalid_citation_kind"})
				return
			}
		}
		in.ActorID = ""
		in.Stale = false
		in.StaleReasons = nil
		v, e = store.Add(string(repo.ID), v.ID, a.UserID, in)
		writePI(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/changes", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in pi.Change
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if plans == nil || (in.OwnerKind != "human" && in.OwnerKind != "agent") {
			writeJSON(w, 422, map[string]string{"error": "invalid_change"})
			return
		}
		inv, err := store.Get(string(repo.ID), r.PathValue("investigation"))
		if err != nil || !participant(inv, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		g, err := goals.Get(string(repo.ID), inv.GoalID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "goal_unavailable"})
			return
		}
		validBase := false
		for _, t := range g.Trials {
			if t.ID == in.BaselineTrialID && t.Version == inv.GoalVersion {
				validBase = true
			}
		}
		if !validBase {
			writeJSON(w, 422, map[string]string{"error": "invalid_baseline"})
			return
		}
		proposal, err := plans.Create(string(repo.ID), a.UserID, in.Title, "Performance change from diagnosis "+in.DiagnosisEntryID)
		if err != nil {
			writeProposalError(w, err)
			return
		}
		task, err := plans.CreateTask(string(repo.ID), proposal.ID, a.UserID, proposals.TaskInput{Title: in.Title, Outcome: "Demonstrate a measured candidate improvement without violating constraints.", Position: 1, Status: proposals.TaskPlanned})
		if err != nil {
			writeProposalTaskError(w, err)
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, err = plans.AssignTask(string(repo.ID), proposal.ID, task.ID, a.UserID, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Optimize the captured workload and publish exact-revision comparison evidence.", RepositoryID: string(repo.ID), BaseRevision: inv.Evidence[0].Revision})
		if err != nil {
			writeProposalTaskError(w, err)
			return
		}
		in.ProposalID, in.TaskID = proposal.ID, task.ID
		inv, err = store.AddChange(string(repo.ID), inv.ID, a.UserID, in)
		writePI(w, inv, err, 201)
	})
}
func trialMap(g performancegoals.Goal) map[string]pi.EvidenceRef {
	m := map[string]pi.EvidenceRef{}
	for _, t := range g.Trials {
		m[t.ID] = pi.EvidenceRef{TrialID: t.ID, Revision: t.Revision, WorkloadSource: t.WorkloadSource, EnvironmentDigest: t.Environment.Digest, Visibility: "repository"}
	}
	return m
}
func selected(v pi.Investigation, id string) (pi.EvidenceRef, bool) {
	for _, x := range v.Evidence {
		if x.TrialID == id {
			return x, true
		}
	}
	return pi.EvidenceRef{}, false
}
func participant(v pi.Investigation, id string) bool {
	for _, x := range v.Participants {
		if x == id {
			return true
		}
	}
	return false
}
func visible(v pi.Investigation, id string) bool {
	if participant(v, id) {
		return true
	}
	for _, e := range v.Evidence {
		if e.Visibility == "participants" {
			return false
		}
	}
	return true
}
func safePath(p string) bool {
	return p != "" && len(p) < 1000 && path.Clean(p) == p && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "../")
}
func blobExists(r *storage.Repository, rev, p string) bool {
	if r == nil {
		return false
	}
	_, e := blobAtPath(r, storage.ObjectID(rev), p)
	return e == nil
}
func writePI(w http.ResponseWriter, v pi.Investigation, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, pi.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else if errors.Is(e, pi.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_performance_investigation"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
