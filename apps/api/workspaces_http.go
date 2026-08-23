package main

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type workspaceStore interface {
	Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
	CreateWithPolicy(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string, workspaces.Policy) (workspaces.Workspace, error)
	EffectivePolicy(string, string) (workspaces.Policy, error)
	Get(string, string) (workspaces.Workspace, error)
	List(string) ([]workspaces.Workspace, error)
	Suspend(string, string, string) (workspaces.Workspace, error)
	Resume(string, string, string, string) (workspaces.Workspace, error)
	Observe(string, string, string, string, string) (workspaces.Workspace, error)
	AddMessage(string, string, string, string) (workspaces.Workspace, error)
	Grant(string, string, string, string, string, string, []string) (workspaces.Workspace, error)
	Intervene(string, string, string, string, string, string, int64) (workspaces.Workspace, error)
	AddResolution(string, string, string, workspaces.ResolutionEntry) (workspaces.Workspace, error)
	RecordPublication(string, string, string, workspaces.Publication) (workspaces.Workspace, error)
	LinkPublicationPullRequest(string, string, string, string) (workspaces.Workspace, error)
	SetPolicy(string, string, workspaces.Policy) (workspaces.Policy, error)
	Policy(string, string) (workspaces.Policy, error)
	AnnounceExpiry(string, string, string, time.Time) (workspaces.Workspace, error)
	Stop(string, string, string, string, bool) (workspaces.Workspace, error)
	RequireRebuild(string, string, string) (workspaces.Workspace, error)
}
type workspaceRunner interface {
	Definition(string, string) (workspaces.Definition, string, error)
	Start(workspaces.Workspace)
	Files(workspaces.Workspace, string) ([]workspaces.File, error)
	WriteFile(workspaces.Workspace, string, string, []byte, bool, *string) (workspaces.Workspace, error)
	Search(workspaces.Workspace, string) ([]workspaces.Match, error)
	Command(workspaces.Workspace, string, string, string, int) (workspaces.CommandResult, error)
	Checkpoint(workspaces.Workspace, string, workspaces.CheckpointRequest) (workspaces.Workspace, error)
	Restore(workspaces.Workspace, string, string) (workspaces.Workspace, workspaces.CheckpointStatus, error)
	InspectCheckpoint(workspaces.Workspace, string) (workspaces.Checkpoint, error)
	Publish(workspaces.Workspace, string, string, workspaces.PublishRequest) (storage.ObjectID, []string, error)
	Export(workspaces.Workspace) ([]byte, error)
}
type workspaceOrganizationStore interface {
	Get(string) (organizations.Organization, error)
}

func registerWorkspacesHTTP(mux *http.ServeMux, store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, plans proposalStore, pulls pullRequestStore, incidentStore incidentStore, extras ...any) {
	base := "/repositories/{repository}/workspaces"
	mux.HandleFunc("POST "+base, createWorkspace(store, runner, repositories, credentials, plans, pulls, incidentStore))
	mux.HandleFunc("GET "+base, listWorkspaces(store, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}", getWorkspace(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/suspend", suspendWorkspace(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/resume", resumeWorkspace(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/expiry", announceWorkspaceExpiry(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/stop", stopWorkspace(store, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/export", exportWorkspace(store, runner, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/workspace-policy", repositoryWorkspacePolicy(store, repositories, credentials, false))
	mux.HandleFunc("PUT /repositories/{repository}/workspace-policy", repositoryWorkspacePolicy(store, repositories, credentials, true))
	mux.HandleFunc("GET "+base+"/{workspace}/files", workspaceFiles(store, runner, repositories, credentials))
	mux.HandleFunc("PUT "+base+"/{workspace}/files", workspaceWriteFile(store, runner, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/search", workspaceSearch(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/commands", workspaceCommand(store, runner, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/preview/{port}/{path...}", workspacePreview(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/presence", workspacePresence(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/messages", workspaceMessage(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/resolutions", workspaceResolution(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/checkpoints", createWorkspaceCheckpoint(store, runner, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/checkpoints/{checkpoint}", getWorkspaceCheckpoint(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/checkpoints/{checkpoint}/restore", restoreWorkspaceCheckpoint(store, runner, repositories, credentials))
	var checks checkRunStarter
	var organizationStore workspaceOrganizationStore
	for _, extra := range extras {
		if value, ok := extra.(checkRunStarter); ok {
			checks = value
		}
		if value, ok := extra.(workspaceOrganizationStore); ok {
			organizationStore = value
		}
	}
	if organizationStore != nil {
		mux.HandleFunc("GET /organizations/{organization}/workspace-policy", organizationWorkspacePolicy(store, credentials, organizationStore, false))
		mux.HandleFunc("PUT /organizations/{organization}/workspace-policy", organizationWorkspacePolicy(store, credentials, organizationStore, true))
	}
	mux.HandleFunc("POST "+base+"/{workspace}/controls", workspaceGrantControl(store, repositories, credentials, organizationStore))
	mux.HandleFunc("POST "+base+"/{workspace}/controls/{control}/interventions", workspaceIntervention(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/checkpoints/{checkpoint}/publication", publishWorkspaceCheckpoint(store, runner, repositories, credentials, plans, pulls, checks))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/conflicts/workspace", createConflictWorkspace(store, runner, repositories, credentials, pulls))
}

func workspaceResolution(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in workspaces.ResolutionEntry
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		in.Kind, in.Summary, in.Uncertainty = strings.TrimSpace(in.Kind), strings.TrimSpace(in.Summary), strings.TrimSpace(in.Uncertainty)
		allowedKind := map[string]bool{"question": true, "answer": true, "proposal": true, "applied": true, "undone": true}
		allowedImpact := map[string]bool{"acceptance_criterion": true, "design_decision": true, "migration": true, "user_behavior": true}
		allowedDisposition := map[string]bool{"preserved": true, "changed": true, "unknown": true}
		if !allowedKind[in.Kind] || in.Summary == "" || len(in.Evidence) == 0 || (in.Kind != "question" && len(in.Impacts) == 0) {
			writeJSON(w, 422, map[string]string{"error": "invalid_resolution_entry"})
			return
		}
		current, err := store.Get(string(repository.ID), r.PathValue("workspace"))
		if err != nil || current.Context.Conflict == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		validRevision := map[string]bool{current.Context.Conflict.BaseCommitID: true, current.Context.Conflict.Source.CommitID: true, current.Context.Conflict.Target.CommitID: true, current.Revision: true}
		for _, e := range in.Evidence {
			if strings.TrimSpace(e.Reference) == "" || !validRevision[e.Revision] {
				writeJSON(w, 422, map[string]string{"error": "evidence_revision_not_frozen"})
				return
			}
		}
		for _, impact := range in.Impacts {
			if !allowedImpact[impact.Kind] || !allowedDisposition[impact.Disposition] || strings.TrimSpace(impact.Outcome) == "" || strings.TrimSpace(impact.Rationale) == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_outcome_impact"})
				return
			}
		}
		if in.ActorKind == "" {
			in.ActorKind = "human"
		}
		if in.ActorKind != "human" && in.ActorKind != "agent" {
			writeJSON(w, 422, map[string]string{"error": "invalid_actor_kind"})
			return
		}
		item, err := store.AddResolution(string(repository.ID), current.ID, actor.UserID, in)
		if err != nil {
			writeWorkspaceResult(w, item, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

func createConflictWorkspace(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, pulls pullRequestStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		pull, err := pulls.Get(string(repository.ID), r.PathValue("pull_request"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		sourceRepo, err := repositories.Open(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "source_repository_unavailable"})
			return
		}
		targetRepo, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		analysis, err := analyzePullConflict(r.Context(), pull, pulls, sourceRepo, targetRepo, nil)
		if err != nil || !analysis.Complete || analysis.Stale || len(analysis.Conflicts) == 0 {
			writeJSON(w, 409, map[string]string{"error": "conflict_evidence_not_current"})
			return
		}
		definition, digest, err := runner.Definition(string(repository.ID), pull.TargetCommitID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_workspace_definition"})
			return
		}
		history := func(commits []pullRequestCommit) []string {
			out := make([]string, 0, len(commits))
			for _, c := range commits {
				out = append(out, c.ID)
			}
			return out
		}
		evidence := make([]workspaces.ConflictEvidence, 0, len(analysis.Conflicts))
		for _, item := range analysis.Conflicts {
			evidence = append(evidence, workspaces.ConflictEvidence{Kind: item.Kind, Path: item.Path, Symbol: item.Symbol, Detail: item.Detail})
		}
		owners := []string{pull.AuthorID, repository.OwnerID}
		if pull.AuthorID == repository.OwnerID {
			owners = owners[:1]
		}
		context := workspaces.SourceContext{Type: "pull_request_conflict", ID: pull.ID, Evidence: []string{"conflict-analysis:" + pull.ID}, Conflict: &workspaces.ConflictContext{PullRequestID: pull.ID, BaseCommitID: analysis.BaseCommitID, Source: workspaces.ConflictRevision{RepositoryID: pull.SourceRepositoryID, Branch: pull.SourceBranch, CommitID: pull.SourceCommitID}, Target: workspaces.ConflictRevision{RepositoryID: pull.RepositoryID, Branch: pull.TargetBranch, CommitID: pull.TargetCommitID}, SourceHistory: history(analysis.Source.Commits), TargetHistory: history(analysis.Target.Commits), Evidence: evidence, OwnerIDs: owners, PublishRepositoryID: string(repository.ID), PublishPermission: "repository:write"}}
		policy, _ := store.EffectivePolicy(string(repository.ID), repository.OrganizationID)
		item, err := store.CreateWithPolicy(string(repository.ID), pull.TargetCommitID, actor.UserID, context, workspaces.Access{RepositoryID: string(repository.ID), ActorID: actor.UserID, Permission: "repository:write"}, definition, digest, policy)
		if err != nil {
			writeWorkspaceResult(w, item, err)
			return
		}
		runner.Start(item)
		w.Header().Set("Location", baseWorkspaceURL(item.RepositoryID, item.ID))
		writeJSON(w, http.StatusCreated, item)
	}
}

func organizationWorkspacePolicy(store workspaceStore, credentials authStore, orgs workspaceOrganizationStore, update bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[update])
		if !ok {
			return
		}
		org, err := orgs.Get(r.PathValue("organization"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		member := false
		owner := false
		for _, m := range org.Members {
			if m.UserID == actor.UserID && !m.AcceptedAt.IsZero() {
				member = true
				owner = m.Role == "owner"
			}
		}
		if !member || update && !owner {
			writeJSON(w, 403, map[string]string{"error": "organization_owner_required"})
			return
		}
		if update {
			var p workspaces.Policy
			if !readJSON(w, r, &p, 16<<10) {
				return
			}
			saved, e := store.SetPolicy("organization", org.ID, p)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_workspace_policy"})
				return
			}
			writeJSON(w, 200, saved)
			return
		}
		p, e := store.Policy("organization", org.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, p)
	}
}

func repositoryWorkspacePolicy(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore, update bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[update], update)
		if !ok {
			return
		}
		if update && actor.UserID != repository.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		if update {
			var p workspaces.Policy
			if !readJSON(w, r, &p, 16<<10) {
				return
			}
			saved, err := store.SetPolicy("repository", string(repository.ID), p)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_workspace_policy"})
				return
			}
			writeJSON(w, 200, saved)
			return
		}
		p, err := store.EffectivePolicy(string(repository.ID), repository.OrganizationID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, p)
	}
}

func announceWorkspaceExpiry(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			ExpiresAt time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		item, err := store.AnnounceExpiry(string(repository.ID), r.PathValue("workspace"), actor.UserID, in.ExpiresAt.UTC())
		writeWorkspaceResult(w, item, err)
	}
}
func stopWorkspace(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		var in struct {
			Reason string `json:"reason"`
			Expire bool   `json:"expire"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		item, err := store.Stop(string(repository.ID), r.PathValue("workspace"), actor.UserID, in.Reason, in.Expire)
		writeWorkspaceResult(w, item, err)
	}
}
func exportWorkspace(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("workspace"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if item.State != workspaces.Ready || item.ExpiresAt == nil || (actor.UserID != item.CreatorID && actor.UserID != repository.OwnerID) {
			writeJSON(w, 409, map[string]string{"error": "export_unavailable"})
			return
		}
		data, err := runner.Export(item)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "unsafe_export"})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=workspace-"+item.ID+".zip")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}
}

func publishWorkspaceCheckpoint(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, plans proposalStore, pulls pullRequestStore, checks checkRunStarter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Branch            string `json:"branch"`
			TargetBranch      string `json:"target_branch"`
			Title             string `json:"title"`
			Message           string `json:"message"`
			CreatePullRequest bool   `json:"create_pull_request"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		var target storage.Reference
		if in.CreatePullRequest {
			repo, e := repositories.Open(storage.ID(item.RepositoryID))
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			target, e = repo.ReadReference(storage.ReferenceName("refs/heads/" + strings.TrimSpace(in.TargetBranch)))
			if e != nil || strings.TrimSpace(in.Title) == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
				return
			}
		}
		commit, contributors, err := runner.Publish(item, actor.UserID, r.PathValue("checkpoint"), workspaces.PublishRequest{Branch: in.Branch, Message: in.Message})
		if errors.Is(err, workspaces.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "checkpoint_publication_conflict"})
			return
		}
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "checkpoint_publication_failed"})
			return
		}
		var pull *pullrequests.PullRequest
		if in.CreatePullRequest {
			targetName := strings.TrimSpace(in.TargetBranch)
			proposalID, taskID, sessionID := "", "", ""
			if item.Context.Type == "proposal_task" {
				proposalID, taskID = item.Context.ParentID, item.Context.ID
				if plan, e := plans.GetPlan(item.RepositoryID, proposalID); e == nil {
					for _, task := range plan.Tasks {
						if task.ID == taskID && task.Assignment != nil {
							sessionID = task.Assignment.SessionID
						}
					}
				}
			}
			originPullID := ""
			if item.Context.Type == "pull_request" {
				originPullID = item.Context.ID
			}
			body := "Published from workspace `" + item.ID + "` checkpoint `" + r.PathValue("checkpoint") + "`.\n\nChanges: " + in.Message
			for _, cp := range item.Checkpoints {
				if cp.ID == r.PathValue("checkpoint") && len(cp.Reproducibility.Commands) > 0 {
					body += "\n\nCommands performed:\n- `" + strings.Join(cp.Reproducibility.Commands, "`\n- `") + "`"
				}
			}
			var reasoningContext *reasoning.Context
			if proposalID != "" && taskID != "" {
				if plan, planErr := plans.GetPlan(item.RepositoryID, proposalID); planErr == nil {
					for _, task := range plan.Tasks {
						if task.ID == taskID {
							reasoningContext = task.ReasoningContext
						}
					}
				}
			}
			created, e := pulls.Create(pullrequests.CreateParams{RepositoryID: item.RepositoryID, SourceRepositoryID: item.RepositoryID, ProposalID: proposalID, TaskID: taskID, ChangeSessionID: sessionID, OriginPullRequestID: originPullID, AuthorID: actor.UserID, Title: in.Title, Body: body, SourceBranch: strings.TrimSpace(in.Branch), TargetBranch: targetName, SourceCommitID: string(commit), TargetCommitID: string(target.ObjectID), WorkspaceID: item.ID, CheckpointID: r.PathValue("checkpoint"), ContributorIDs: contributors, ReasoningContext: reasoningContext})
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
				return
			}
			if proposalID != "" && taskID != "" {
				assignmentID := ""
				if plan, planErr := plans.GetPlan(item.RepositoryID, proposalID); planErr == nil {
					for _, task := range plan.Tasks {
						if task.ID == taskID && task.Assignment != nil {
							assignmentID = task.Assignment.ID
							break
						}
					}
				}
				if _, contributionErr := plans.PublishTaskContribution(item.RepositoryID, proposalID, taskID, actor.UserID, proposals.TaskContribution{PullRequestID: created.ID, SessionID: sessionID, AssignmentID: assignmentID, SourceCommitID: string(commit), TargetCommitID: string(target.ObjectID), Status: proposals.ContributionReview}); contributionErr != nil {
					writeJSON(w, 422, map[string]string{"error": "task_contribution_failed"})
					return
				}
			}
			pull = &created
			if checks != nil {
				_ = checks.Start(item.RepositoryID, item.RepositoryID, created.ID, string(commit))
			}
		}
		pullID := ""
		if pull != nil {
			pullID = pull.ID
		}
		publication := workspaces.Publication{CommitID: string(commit), Branch: strings.TrimSpace(in.Branch), PullRequestID: pullID, PublisherID: actor.UserID, ContributorIDs: contributors, PublishedAt: time.Now().UTC()}
		updated, err := store.Get(item.RepositoryID, item.ID)
		if pullID != "" {
			updated, err = store.LinkPublicationPullRequest(item.RepositoryID, item.ID, r.PathValue("checkpoint"), pullID)
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "publication_link_failed"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"workspace": updated, "publication": publication, "pull_request": pull})
	}
}

func getWorkspaceCheckpoint(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("workspace"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		checkpoint, err := runner.InspectCheckpoint(item, r.PathValue("checkpoint"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, checkpoint)
	}
}

func createWorkspaceCheckpoint(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Summary         string                     `json:"summary"`
			Paths           []string                   `json:"paths"`
			ParentID        string                     `json:"parent_id"`
			Reproducibility workspaces.Reproducibility `json:"reproducibility"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		updated, err := runner.Checkpoint(item, actor.UserID, workspaces.CheckpointRequest{Summary: in.Summary, Paths: in.Paths, ParentID: in.ParentID, Reproducibility: in.Reproducibility})
		if errors.Is(err, workspaces.ErrUnsafeCheckpoint) {
			writeJSON(w, 422, map[string]string{"error": "unsafe_checkpoint_content"})
			return
		}
		if err != nil {
			writeWorkspaceResult(w, updated, err)
			return
		}
		checkpoint := updated.Checkpoints[len(updated.Checkpoints)-1]
		w.Header().Set("Location", baseWorkspaceURL(item.RepositoryID, item.ID)+"/checkpoints/"+checkpoint.ID)
		writeJSON(w, http.StatusCreated, checkpoint)
	}
}

func restoreWorkspaceCheckpoint(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		updated, status, err := runner.Restore(item, actor.UserID, r.PathValue("checkpoint"))
		if errors.Is(err, workspaces.ErrConflict) {
			writeJSON(w, http.StatusConflict, map[string]any{"error": "checkpoint_conflict", "status": status})
			return
		}
		if errors.Is(err, workspaces.ErrUnsafeCheckpoint) {
			writeJSON(w, 422, map[string]string{"error": "checkpoint_evidence_invalid"})
			return
		}
		writeWorkspaceResult(w, updated, err)
	}
}

func workspacePresence(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in struct {
			Surface string `json:"surface"`
			Path    string `json:"path"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		if in.Surface != "files" && in.Surface != "terminal" && in.Surface != "commands" && in.Surface != "preview" && in.Surface != "discussion" {
			writeJSON(w, 422, map[string]string{"error": "invalid_presence"})
			return
		}
		item, err := store.Observe(string(repository.ID), r.PathValue("workspace"), actor.UserID, in.Surface, strings.TrimSpace(in.Path))
		writeWorkspaceResult(w, item, err)
	}
}
func workspaceMessage(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Message string `json:"message"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		item, err := store.AddMessage(string(repository.ID), r.PathValue("workspace"), actor.UserID, in.Message)
		writeWorkspaceResult(w, item, err)
	}
}
func workspaceGrantControl(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore, orgs workspaceOrganizationStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			SubjectID   string   `json:"subject_id"`
			SubjectKind string   `json:"subject_kind"`
			Mode        string   `json:"mode"`
			Scopes      []string `json:"scopes"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		if in.SubjectKind == "human" {
			participant := in.SubjectID == repository.OwnerID
			if !participant {
				participant, _ = repositories.IsCollaborator(repository.ID, in.SubjectID)
			}
			if !participant {
				writeJSON(w, 422, map[string]string{"error": "subject_not_authorized"})
				return
			}
		} else if in.SubjectKind == "approved_agent" {
			approved := false
			if orgs != nil && repository.OrganizationID != "" {
				if org, err := orgs.Get(repository.OrganizationID); err == nil {
					for _, agent := range org.Agents {
						if agent.ID == in.SubjectID {
							approved = true
							break
						}
					}
				}
			}
			if !approved {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "agent_not_approved"})
				return
			}
		}
		item, err := store.Grant(string(repository.ID), r.PathValue("workspace"), actor.UserID, in.SubjectID, in.SubjectKind, in.Mode, in.Scopes)
		writeWorkspaceResult(w, item, err)
	}
}
func workspaceIntervention(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Action  string `json:"action"`
			Message string `json:"message"`
			Version int64  `json:"version"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		item, err := store.Intervene(string(repository.ID), r.PathValue("workspace"), actor.UserID, r.PathValue("control"), in.Action, in.Message, in.Version)
		writeWorkspaceResult(w, item, err)
	}
}

func readyWorkspace(w http.ResponseWriter, r *http.Request, store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore, scope auth.Scope, write bool) (workspaces.Workspace, auth.Grant, bool) {
	repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, write)
	if !ok {
		return workspaces.Workspace{}, actor, false
	}
	item, err := store.Get(string(repository.ID), r.PathValue("workspace"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return item, actor, false
	}
	if item.State != workspaces.Ready {
		writeJSON(w, 409, map[string]string{"error": "workspace_state_conflict"})
		return item, actor, false
	}
	return item, actor, true
}

func workspaceFiles(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		files, err := runner.Files(item, r.URL.Query().Get("path"))
		if errors.Is(err, workspaces.ErrUnsafePath) || errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "path_not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"revision": item.Revision, "items": files})
	}
}
func workspaceWriteFile(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Path       string  `json:"path"`
			Content    string  `json:"content"`
			Deleted    bool    `json:"deleted"`
			BaseDigest *string `json:"base_digest"`
		}
		if !readJSON(w, r, &input, 2<<20) {
			return
		}
		updated, err := runner.WriteFile(item, actor.UserID, input.Path, []byte(input.Content), input.Deleted, input.BaseDigest)
		writeWorkspaceResult(w, updated, err)
	}
}
func workspaceSearch(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		matches, err := runner.Search(item, r.URL.Query().Get("q"))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_search"})
			return
		}
		writeJSON(w, 200, map[string]any{"revision": item.Revision, "items": matches, "total_count": len(matches)})
	}
}
func workspaceCommand(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Command        string `json:"command"`
			Directory      string `json:"directory"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		}
		if !readJSON(w, r, &input, 8<<10) {
			return
		}
		result, err := runner.Command(item, actor.UserID, input.Command, input.Directory, input.TimeoutSeconds)
		if err != nil {
			writeJSON(w, 422, map[string]any{"error": "command_failed", "result": result})
			return
		}
		writeJSON(w, 200, map[string]any{"revision": item.Revision, "result": result})
	}
}
func workspacePreview(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := readyWorkspace(w, r, store, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		port, err := strconv.Atoi(r.PathValue("port"))
		if err != nil {
			return
		}
		root := ""
		for _, declared := range item.Definition.Ports {
			if declared.Number == port {
				root = declared.Path
				break
			}
		}
		if root == "" {
			writeJSON(w, 404, map[string]string{"error": "preview_not_found"})
			return
		}
		path := filepath.Join(root, r.PathValue("path"))
		files, err := runner.Files(item, path)
		if err != nil || len(files) != 1 || files[0].Directory || files[0].Binary {
			writeJSON(w, 404, map[string]string{"error": "preview_not_found"})
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'none'; frame-ancestors 'self'; form-action 'none'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Workspace-Revision", item.Revision)
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
		_, _ = w.Write([]byte(files[0].Content))
	}
}

func createWorkspace(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, plans proposalStore, pulls pullRequestStore, incidentStore incidentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Revision      string                   `json:"revision"`
			SourceContext workspaces.SourceContext `json:"source_context"`
		}
		if !readJSON(w, r, &input, 8<<10) {
			return
		}
		input.Revision = strings.TrimSpace(input.Revision)
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if len(input.Revision) != 40 {
			writeJSON(w, 422, map[string]string{"error": "exact_revision_required"})
			return
		}
		if _, err = opened.ReadCommit(storage.ObjectID(input.Revision)); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_revision"})
			return
		}
		if !workspaceContextMatches(string(repository.ID), input.Revision, input.SourceContext, plans, pulls, incidentStore) {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_context"})
			return
		}
		definition, digest, err := runner.Definition(string(repository.ID), input.Revision)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_workspace_definition"})
			return
		}
		policy, _ := store.EffectivePolicy(string(repository.ID), repository.OrganizationID)
		item, err := store.CreateWithPolicy(string(repository.ID), input.Revision, actor.UserID, input.SourceContext, workspaces.Access{RepositoryID: string(repository.ID), ActorID: actor.UserID, Permission: "repository:write"}, definition, digest, policy)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(item)
		w.Header().Set("Location", baseWorkspaceURL(string(repository.ID), item.ID))
		writeJSON(w, http.StatusCreated, item)
	}
}

func workspaceContextMatches(repositoryID, revision string, c workspaces.SourceContext, plans proposalStore, pulls pullRequestStore, incidentStore incidentStore) bool {
	switch c.Type {
	case "repository":
		return c.ID == "" && c.ParentID == ""
	case "contribution_opportunity":
		return c.ID != "" && c.UpstreamRepositoryID != "" && len(c.AcceptanceCriteria) > 0
	case "decision":
		return c.ID != "" && c.ParentID != ""
	case "proposal_task":
		proposal, err := plans.Get(repositoryID, c.ParentID)
		if err != nil {
			return false
		}
		plan, err := plans.GetPlan(repositoryID, proposal.ID)
		if err != nil {
			return false
		}
		for _, task := range plan.Tasks {
			if task.ID != c.ID {
				continue
			}
			if task.Assignment != nil && task.Assignment.BaseRevision == revision {
				return true
			}
			for _, contribution := range task.Contributions {
				if contribution.SourceCommitID == revision {
					return true
				}
			}
		}
		return false
	case "pull_request":
		item, err := pulls.Get(repositoryID, c.ID)
		return err == nil && item.SourceRepositoryID == repositoryID && item.SourceCommitID == revision
	case "incident_repair":
		item, err := incidentStore.Get(repositoryID, c.ParentID)
		if err != nil {
			return false
		}
		for _, mitigation := range item.Mitigations {
			if mitigation.ID != c.ID || mitigation.Kind != "emergency_repair" {
				continue
			}
			for _, attempt := range mitigation.Attempts {
				if attempt.ResourceType == "pull_request" {
					pull, e := pulls.Get(repositoryID, attempt.ResourceID)
					if e == nil && pull.SourceRepositoryID == repositoryID && pull.SourceCommitID == revision {
						return true
					}
				}
			}
		}
		return false
	default:
		return false
	}
}
func listWorkspaces(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func getWorkspace(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("workspace"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, item)
	}
}
func suspendWorkspace(store workspaceStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, err := store.Suspend(string(repository.ID), r.PathValue("workspace"), actor.UserID)
		writeWorkspaceResult(w, item, err)
	}
}
func resumeWorkspace(store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		current, err := store.Get(string(repository.ID), r.PathValue("workspace"))
		if err != nil {
			writeWorkspaceResult(w, current, err)
			return
		}
		_, digest, err := runner.Definition(string(repository.ID), current.Revision)
		if err != nil {
			_, _ = store.RequireRebuild(string(repository.ID), current.ID, "base revision or environment definition is unavailable")
			writeJSON(w, 409, map[string]string{"error": "workspace_foundation_unavailable"})
			return
		}
		if digest != current.DefinitionDigest {
			_, _ = store.RequireRebuild(string(repository.ID), current.ID, "environment definition changed")
			writeJSON(w, 409, map[string]string{"error": "workspace_rebuild_required"})
			return
		}
		item, err := store.Resume(string(repository.ID), current.ID, actor.UserID, digest)
		writeWorkspaceResult(w, item, err)
	}
}
func writeWorkspaceResult(w http.ResponseWriter, item workspaces.Workspace, err error) {
	if errors.Is(err, workspaces.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if errors.Is(err, workspaces.ErrInvalidTransition) {
		writeJSON(w, 409, map[string]string{"error": "workspace_state_conflict"})
		return
	}
	if errors.Is(err, workspaces.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "workspace_version_conflict"})
		return
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, 200, item)
}
func baseWorkspaceURL(repository, id string) string {
	return "/repositories/" + repository + "/workspaces/" + id
}
