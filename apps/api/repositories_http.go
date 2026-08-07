package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type ownedRepositoryStore interface {
	Create(string, repositories.Metadata) (repositories.Repository, error)
	Get(string, storage.ID) (repositories.Repository, error)
	Inspect(storage.ID) (repositories.Repository, error)
	List(string) ([]repositories.Repository, error)
	ListAccessible(string) ([]repositories.Repository, error)
	Update(string, storage.ID, repositories.Metadata) (repositories.Repository, error)
	Delete(string, storage.ID) error
	IsCollaborator(storage.ID, string) (bool, error)
}

func registerRepositoriesHTTP(mux *http.ServeMux, store ownedRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories", createRepository(store, credentials))
	mux.HandleFunc("GET /repositories", listRepositories(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}", getRepository(store, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}", updateRepository(store, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}", deleteRepository(store, credentials))
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
	return map[string]any{"id": item.ID, "owner_id": item.OwnerID, "name": item.Name, "description": item.Description, "visibility": item.Visibility, "empty": item.Empty, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt, "api_url": "/repositories/" + string(item.ID), "git_url": "/repositories/" + string(item.ID)}
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
