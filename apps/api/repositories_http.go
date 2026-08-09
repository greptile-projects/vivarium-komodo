package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type ownedRepositoryStore interface {
	Create(string, repositories.Metadata) (repositories.Repository, error)
	Fork(string, storage.ID, repositories.Metadata) (repositories.Repository, error)
	SyncForkBranch(string, storage.ID, string) (repositories.SyncResult, error)
	Get(string, storage.ID) (repositories.Repository, error)
	Inspect(storage.ID) (repositories.Repository, error)
	List(string) ([]repositories.Repository, error)
	ListAccessible(string) ([]repositories.Repository, error)
	ListPublic(string) ([]repositories.Repository, error)
	Update(string, storage.ID, repositories.Metadata) (repositories.Repository, error)
	Delete(string, storage.ID) error
	IsCollaborator(storage.ID, string) (bool, error)
	SetRequiredChecks(string, storage.ID, string, []string) (repositories.Repository, error)
	SetIntegrationQueue(string, storage.ID, string, repositories.IntegrationQueuePolicy) (repositories.Repository, error)
	TransferOwner(storage.ID, string, string, string, string, string) (repositories.Repository, error)
	ListOrganization(string) ([]repositories.Repository, error)
}

func registerRepositoriesHTTP(mux *http.ServeMux, store ownedRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories", createRepository(store, credentials))
	mux.HandleFunc("POST /repositories/{repository}/forks", forkRepository(store, credentials))
	mux.HandleFunc("POST /repositories/{repository}/sync", syncForkBranch(store, credentials))
	mux.HandleFunc("GET /repositories", listRepositories(store, credentials))
	mux.HandleFunc("GET /repositories/public", listPublicRepositories(store))
	mux.HandleFunc("GET /repositories/{repository}", getRepository(store, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}", updateRepository(store, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}", deleteRepository(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/required-checks", getRequiredChecks(store, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/required-checks", putRequiredChecks(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}/integration-queue", getIntegrationQueue(store, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/integration-queue", putIntegrationQueue(store, credentials))
}

func integrationPolicyResponse(item repositories.Repository, branch string) map[string]any {
	policy, enabled := item.IntegrationQueue[branch]
	if !enabled {
		policy = repositories.IntegrationQueuePolicy{Concurrency: 1, FailureBehavior: "pause"}
	}
	return map[string]any{"branch": branch, "enabled": enabled, "concurrency": policy.Concurrency, "failure_behavior": policy.FailureBehavior, "required_checks": item.RequiredChecks[branch], "required_owner_approvals": 1}
}

func getIntegrationQueue(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := proposalRepositoryAccess(w, r, store, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		writeJSON(w, 200, integrationPolicyResponse(item, strings.TrimSpace(r.URL.Query().Get("branch"))))
	}
}

func putIntegrationQueue(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var input struct {
			Branch          string `json:"branch"`
			Enabled         bool   `json:"enabled"`
			Concurrency     int    `json:"concurrency"`
			FailureBehavior string `json:"failure_behavior"`
		}
		if !readJSON(w, r, &input, 2048) {
			return
		}
		item, err := store.SetIntegrationQueue(actor.UserID, storage.ID(r.PathValue("repository")), input.Branch, repositories.IntegrationQueuePolicy{Enabled: input.Enabled, Concurrency: input.Concurrency, FailureBehavior: input.FailureBehavior})
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_integration_queue_policy"})
			return
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, 200, integrationPolicyResponse(item, strings.TrimSpace(input.Branch)))
	}
}

func listPublicRepositories(store ownedRepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.ListPublic(r.URL.Query().Get("q"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		output := make([]map[string]any, len(items))
		for i, item := range items {
			output[i] = repositoryResponse(item)
		}
		writeJSON(w, 200, map[string]any{"items": output, "page": page, "per_page": perPage, "total_count": total})
	}
}

func forkRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		upstreamID := storage.ID(r.PathValue("repository"))
		upstream, reader, ok := proposalRepositoryAccess(w, r, store, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		if reader.UserID != actor.UserID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Name        string                  `json:"name"`
			Description string                  `json:"description"`
			Visibility  repositories.Visibility `json:"visibility"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			input.Name = upstream.Name
		}
		if strings.TrimSpace(input.Description) == "" {
			input.Description = upstream.Description
		}
		if input.Visibility == "" {
			input.Visibility = repositories.Private
		}
		item, err := store.Fork(actor.UserID, upstreamID, repositories.Metadata{Name: input.Name, Description: input.Description, Visibility: input.Visibility})
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_repository"})
			return
		}
		if errors.Is(err, repositories.ErrNameTaken) {
			writeJSON(w, 409, map[string]string{"error": "name_taken"})
			return
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.Header().Set("Location", "/repositories/"+string(item.ID))
		writeJSON(w, http.StatusCreated, repositoryResponse(item))
	}
}

func syncForkBranch(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var input struct {
			Branch string `json:"branch"`
		}
		if !readJSON(w, r, &input, 1024) {
			return
		}
		result, err := store.SyncForkBranch(actor.UserID, storage.ID(r.PathValue("repository")), strings.TrimSpace(input.Branch))
		switch {
		case errors.Is(err, repositories.ErrNotFork):
			writeJSON(w, 422, map[string]string{"error": "not_a_fork"})
			return
		case errors.Is(err, repositories.ErrUpstreamBranch):
			writeJSON(w, 422, map[string]string{"error": "upstream_branch_not_found"})
			return
		case errors.Is(err, repositories.ErrForkConflict):
			writeJSON(w, 409, map[string]string{"error": "fork_branch_diverged"})
			return
		case err != nil:
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"repository": repositoryResponse(result.Repository), "branch": result.Branch, "before_commit_id": result.Before, "after_commit_id": result.After, "updated": result.Updated})
	}
}

func getRequiredChecks(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := proposalRepositoryAccess(w, r, store, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		branch := r.URL.Query().Get("branch")
		writeJSON(w, 200, map[string]any{"branch": branch, "checks": item.RequiredChecks[branch]})
	}
}

func putRequiredChecks(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var input struct {
			Branch string   `json:"branch"`
			Checks []string `json:"checks"`
		}
		if !readJSON(w, r, &input, 8192) {
			return
		}
		item, err := store.SetRequiredChecks(actor.UserID, storage.ID(r.PathValue("repository")), input.Branch, input.Checks)
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_required_checks"})
			return
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"branch": input.Branch, "checks": item.RequiredChecks[input.Branch]})
	}
}

func createRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		var input struct {
			Name        string                  `json:"name"`
			Description string                  `json:"description"`
			Visibility  repositories.Visibility `json:"visibility"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		if input.Visibility == "" {
			input.Visibility = repositories.Private
		}
		item, err := store.Create(actor.UserID, repositories.Metadata{Name: input.Name, Description: input.Description, Visibility: input.Visibility})
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_repository"})
			return
		}
		if errors.Is(err, repositories.ErrNameTaken) {
			writeJSON(w, 409, map[string]string{"error": "name_taken"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.Header().Set("Location", "/repositories/"+string(item.ID))
		writeJSON(w, http.StatusCreated, repositoryResponse(item))
	}
}

func listRepositories(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		var items []repositories.Repository
		var err error
		if r.URL.Query().Get("affiliation") == "all" {
			items, err = store.ListAccessible(actor.UserID)
		} else {
			items, err = store.List(actor.UserID)
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		output := make([]map[string]any, len(items))
		for i, item := range items {
			output[i] = repositoryResponse(item)
		}
		writeJSON(w, 200, map[string]any{"items": output, "page": page, "per_page": perPage, "total_count": total})
	}
}

func getRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := store.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		actor, authenticated, ok := authenticateOptionalRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		if item.Visibility != repositories.Public && !authenticated {
			writeUnauthenticated(w, "Bearer", "komodo")
			return
		}
		collaborator := false
		if authenticated && actor.UserID != item.OwnerID {
			collaborator, err = store.IsCollaborator(item.ID, actor.UserID)
			if err != nil {
				writeRepositoryError(w, err)
				return
			}
		}
		if item.Visibility != repositories.Public && authenticated && actor.UserID != item.OwnerID && !collaborator {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, repositoryResponse(item))
	}
}

func updateRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		current, err := store.Get(actor.UserID, storage.ID(r.PathValue("repository")))
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		var input struct {
			Name        *string                  `json:"name"`
			Description *string                  `json:"description"`
			Visibility  *repositories.Visibility `json:"visibility"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		metadata := repositories.Metadata{Name: current.Name, Description: current.Description, Visibility: current.Visibility}
		if input.Name != nil {
			metadata.Name = *input.Name
		}
		if input.Description != nil {
			metadata.Description = *input.Description
		}
		if input.Visibility != nil {
			metadata.Visibility = *input.Visibility
		}
		item, err := store.Update(actor.UserID, storage.ID(r.PathValue("repository")), metadata)
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_repository"})
			return
		}
		if errors.Is(err, repositories.ErrNameTaken) {
			writeJSON(w, 409, map[string]string{"error": "name_taken"})
			return
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, repositoryResponse(item))
	}
}

func deleteRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		if err := store.Delete(actor.UserID, storage.ID(r.PathValue("repository"))); err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func repositoryResponse(item repositories.Repository) map[string]any {
	response := map[string]any{"id": item.ID, "owner_id": item.OwnerID, "name": item.Name, "description": item.Description, "visibility": item.Visibility, "empty": item.Empty, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt, "api_url": "/repositories/" + string(item.ID), "git_url": "/repositories/" + string(item.ID)}
	if item.OrganizationID != "" {
		response["organization_id"] = item.OrganizationID
		response["owner_kind"] = "organization"
		response["administrator_id"] = item.OwnerID
		response["owner_id"] = item.OrganizationID
	} else {
		response["owner_kind"] = "user"
	}
	if item.UpstreamID != "" {
		response["upstream_repository_id"] = item.UpstreamID
		response["upstream_api_url"] = "/repositories/" + string(item.UpstreamID)
	}
	return response
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
