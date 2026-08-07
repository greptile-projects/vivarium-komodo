package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type ownedRepositoryStore interface {
	Create(string) (repositories.Repository, error)
	Get(string, storage.ID) (repositories.Repository, error)
	List(string) ([]repositories.Repository, error)
	Delete(string, storage.ID) error
}

func registerRepositoriesHTTP(mux *http.ServeMux, store ownedRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories", createRepository(store, credentials))
	mux.HandleFunc("GET /repositories", listRepositories(store, credentials))
	mux.HandleFunc("GET /repositories/{repository}", getRepository(store, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}", deleteRepository(store, credentials))
}

func createRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		item, err := store.Create(actor.UserID)
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
		items, err := store.List(actor.UserID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		output := make([]map[string]any, len(items))
		for i, item := range items {
			output[i] = repositoryResponse(item)
		}
		writeJSON(w, 200, output)
	}
}

func getRepository(store ownedRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		item, err := store.Get(actor.UserID, storage.ID(r.PathValue("repository")))
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		writeJSON(w, 200, repositoryResponse(item))
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
	return map[string]any{"id": item.ID, "owner_id": item.OwnerID, "empty": item.Empty, "created_at": item.CreatedAt, "git_url": "/repositories/" + string(item.ID)}
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
