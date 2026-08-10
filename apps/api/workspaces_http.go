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
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type workspaceStore interface {
	Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
	Get(string, string) (workspaces.Workspace, error)
	List(string) ([]workspaces.Workspace, error)
	Suspend(string, string, string) (workspaces.Workspace, error)
	Resume(string, string, string, string) (workspaces.Workspace, error)
	Observe(string, string, string, string, string) (workspaces.Workspace, error)
	AddMessage(string, string, string, string) (workspaces.Workspace, error)
	Grant(string, string, string, string, string, string, []string) (workspaces.Workspace, error)
	Intervene(string, string, string, string, string, string, int64) (workspaces.Workspace, error)
	RecordPublication(string, string, string, workspaces.Publication) (workspaces.Workspace, error)
	LinkPublicationPullRequest(string, string, string, string) (workspaces.Workspace, error)
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
	mux.HandleFunc("GET "+base+"/{workspace}/files", workspaceFiles(store, runner, repositories, credentials))
	mux.HandleFunc("PUT "+base+"/{workspace}/files", workspaceWriteFile(store, runner, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/search", workspaceSearch(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/commands", workspaceCommand(store, runner, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}/preview/{port}/{path...}", workspacePreview(store, runner, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/presence", workspacePresence(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/messages", workspaceMessage(store, repositories, credentials))
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
	mux.HandleFunc("POST "+base+"/{workspace}/controls", workspaceGrantControl(store, repositories, credentials, organizationStore))
	mux.HandleFunc("POST "+base+"/{workspace}/controls/{control}/interventions", workspaceIntervention(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/checkpoints/{checkpoint}/publication", publishWorkspaceCheckpoint(store, runner, repositories, credentials, plans, pulls, checks))
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
			created, e := pulls.Create(pullrequests.CreateParams{RepositoryID: item.RepositoryID, SourceRepositoryID: item.RepositoryID, ProposalID: proposalID, TaskID: taskID, ChangeSessionID: sessionID, OriginPullRequestID: originPullID, AuthorID: actor.UserID, Title: in.Title, Body: body, SourceBranch: strings.TrimSpace(in.Branch), TargetBranch: targetName, SourceCommitID: string(commit), TargetCommitID: string(target.ObjectID), WorkspaceID: item.ID, CheckpointID: r.PathValue("checkpoint"), ContributorIDs: contributors})
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
				return
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
			if orgs != nil {
				if org, err := orgs.Get(repository.OwnerID); err == nil {
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
		item, err := store.Create(string(repository.ID), input.Revision, actor.UserID, input.SourceContext, workspaces.Access{RepositoryID: string(repository.ID), ActorID: actor.UserID, Permission: "repository:write"}, definition, digest)
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
			writeJSON(w, 409, map[string]string{"error": "workspace_foundation_unavailable"})
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
