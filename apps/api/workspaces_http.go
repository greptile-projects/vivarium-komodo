package main

import (
	"errors"
	"net/http"
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
}

func registerWorkspacesHTTP(mux *http.ServeMux, store workspaceStore, runner workspaceRunner, repositories taskSessionRepositoryStore, credentials authStore, plans proposalStore, pulls pullRequestStore, incidentStore incidentStore) {
	base := "/repositories/{repository}/workspaces"
	mux.HandleFunc("POST "+base, createWorkspace(store, runner, repositories, credentials, plans, pulls, incidentStore))
	mux.HandleFunc("GET "+base, listWorkspaces(store, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{workspace}", getWorkspace(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/suspend", suspendWorkspace(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{workspace}/resume", resumeWorkspace(store, runner, repositories, credentials))
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
