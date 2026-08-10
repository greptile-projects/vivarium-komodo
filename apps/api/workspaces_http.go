package main

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type workspaceStore interface {
	Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
	Get(string, string) (workspaces.Workspace, error)
	List(string) ([]workspaces.Workspace, error)
	Suspend(string, string, string) (workspaces.Workspace, error)
	Resume(string, string, string, string) (workspaces.Workspace, error)
}
type workspaceRunner interface {
	Definition(string, string) (workspaces.Definition, string, error)
	Start(workspaces.Workspace)
	Files(workspaces.Workspace, string) ([]workspaces.File, error)
	WriteFile(workspaces.Workspace, string, string, []byte, bool) (workspaces.Workspace, error)
	Search(workspaces.Workspace, string) ([]workspaces.Match, error)
	Command(workspaces.Workspace, string, string, string, int) (workspaces.CommandResult, error)
}

func registerWorkspacesHTTP(mux *http.ServeMux, store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, plans proposalStore, pulls pullRequestStore, incidentStore incidentStore) {
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
			Path    string `json:"path"`
			Content string `json:"content"`
			Deleted bool   `json:"deleted"`
		}
		if !readJSON(w, r, &input, 2<<20) {
			return
		}
		updated, err := runner.WriteFile(item, actor.UserID, input.Path, []byte(input.Content), input.Deleted)
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
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(w, 200, item)
}
func baseWorkspaceURL(repository, id string) string {
	return "/repositories/" + repository + "/workspaces/" + id
}
