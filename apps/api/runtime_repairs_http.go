package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeinvestigations"
	rr "github.com/greptile-projects/vivarium-komodo/apps/api/runtimerepairs"
	rp "github.com/greptile-projects/vivarium-komodo/apps/api/runtimereplays"
)

func registerRuntimeRepairsHTTP(mux *http.ServeMux, store *rr.Store, debugging *dw.Store, replays *rp.Store, investigations *ri.Store, plans *proposals.Store, pulls *pullrequests.Store, checks *checkruns.Store, releaseStore *releases.Store, deploymentStore *deployments.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/runtime-repairs"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.List(string(repo.ID))
		if runtimeRepairError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in rr.CreateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		workspace, e := debugging.Get(string(repo.ID), in.WorkspaceID)
		if e != nil || workspace.SourceRevision != in.AffectedRevision {
			writeJSON(w, 422, map[string]string{"error": "invalid_debugging_workspace"})
			return
		}
		replay, e := replays.Get(string(repo.ID), in.ReplayID)
		if e != nil || replay.WorkspaceID != workspace.ID || replay.Revision != in.AffectedRevision || !replay.Reproduced {
			writeJSON(w, 422, map[string]string{"error": "reproduced_runtime_replay_required"})
			return
		}
		investigation, e := investigations.Get(string(repo.ID), in.InvestigationID)
		cause := false
		if e == nil && investigation.WorkspaceID == workspace.ID {
			for _, c := range investigation.Claims {
				cause = cause || (c.ID == in.CauseClaimID && c.Status == "supported" && !c.Stale)
			}
		}
		if !cause {
			writeJSON(w, 422, map[string]string{"error": "current_supported_cause_required"})
			return
		}
		context := "Debugging workspace " + in.WorkspaceID + "; minimized replay " + in.ReplayID + "; supported cause " + in.CauseClaimID + "; affected revision " + in.AffectedRevision + "; regression criteria: " + strings.Join(in.RegressionCriteria, ", ")
		p, e := plans.Create(string(repo.ID), a.UserID, in.Title, context)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		task, e := plans.CreateTask(string(repo.ID), p.ID, a.UserID, proposals.TaskInput{Title: in.Title, Outcome: context, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, BaseRevision: in.AffectedRevision, CompletionCriteria: append(append([]string{}, in.AcceptanceCriteria...), in.RegressionCriteria...), VerificationPlan: append([]string{"Rerun runtime replay " + in.ReplayID + " on every pull revision", "Run ordinary required checks on every pull revision"}, in.RegressionCriteria...), Status: proposals.TaskPlanned, Position: 1})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_runtime_repair_task"})
			return
		}
		v, e := store.Create(string(repo.ID), a.UserID, p.ID, task.ID, in)
		if runtimeRepairError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"repair": v, "proposal": p, "task": task})
	})
	mux.HandleFunc("GET "+base+"/{repair}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("repair"))
		if runtimeRepairError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{repair}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in rr.VerificationInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		repair, e := store.Get(string(repo.ID), r.PathValue("repair"))
		if e != nil {
			runtimeRepairError(w, e)
			return
		}
		pr, e := pulls.Get(string(repo.ID), in.PullRequestID)
		valid := e == nil && pr.TaskID == repair.TaskID && pr.SourceCommitID == in.Revision
		scenario, _ := replays.Get(string(repo.ID), repair.ReplayID)
		replayPassed := false
		for _, attempt := range scenario.Attempts {
			replayPassed = replayPassed || (attempt.ID == in.ReplayAttemptID && attempt.Revision == in.Revision && attempt.Mode == "repair_verification" && attempt.Status == "not_reproduced" && !attempt.Reproduced && len(attempt.Blockers) == 0)
		}
		checksPassed := len(in.RequiredCheckRunIDs) > 0
		for _, id := range in.RequiredCheckRunIDs {
			run, x := checks.Get(string(repo.ID), in.PullRequestID, id)
			checksPassed = checksPassed && x == nil && run.CommitID == in.Revision && run.State == checkruns.Succeeded
		}
		v, e := store.Verify(string(repo.ID), repair.ID, a.UserID, in, valid && replayPassed && checksPassed)
		if runtimeRepairError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{repair}/validations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in rr.ValidationInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		rel, e := releaseStore.Get(string(repo.ID), in.ReleaseID)
		dep, x := deploymentStore.GetDeployment(string(repo.ID), in.DeploymentID)
		if e != nil || x != nil || rel.CommitID != in.Revision || dep.ReleaseID != rel.ID {
			writeJSON(w, 422, map[string]string{"error": "invalid_runtime_repair_delivery"})
			return
		}
		v, e := store.Validate(string(repo.ID), r.PathValue("repair"), a.UserID, in)
		if runtimeRepairError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func runtimeRepairError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, rr.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "runtime_repair_not_found"})
	} else if errors.Is(e, rr.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_runtime_repair"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
