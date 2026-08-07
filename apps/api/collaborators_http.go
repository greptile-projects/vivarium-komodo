package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type collaboratorRepositoryStore interface {
	Get(string, storage.ID) (repositories.Repository, error)
	AddCollaborator(string, storage.ID, string) (repositories.Repository, error)
	RemoveCollaborator(string, storage.ID, string) error
}

type collaboratorUserStore interface {
	Get(users.ID) (users.User, error)
}

func registerCollaboratorsHTTP(mux *http.ServeMux, repositories collaboratorRepositoryStore, userStore collaboratorUserStore, credentials authStore) {
	mux.HandleFunc("GET /repositories/{repository}/collaborators", listCollaborators(repositories, userStore, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/collaborators/{user}", addCollaborator(repositories, userStore, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}/collaborators/{user}", removeCollaborator(repositories, credentials))
}

func listCollaborators(store collaboratorRepositoryStore, userStore collaboratorUserStore, credentials authStore) http.HandlerFunc {
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
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		ids := item.CollaboratorIDs
		total := len(ids)
		ids = paginate(ids, page, perPage)
		items := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			user, err := userStore.Get(users.ID(id))
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			items = append(items, collaboratorResponse(user))
		}
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func addCollaborator(store collaboratorRepositoryStore, userStore collaboratorUserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		user, err := userStore.Get(users.ID(r.PathValue("user")))
		if errors.Is(err, users.ErrNotFound) || errors.Is(err, users.ErrInvalidID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		_, err = store.AddCollaborator(actor.UserID, storage.ID(r.PathValue("repository")), string(user.ID))
		if errors.Is(err, repositories.ErrInvalidRepository) {
			writeJSON(w, 422, map[string]string{"error": "invalid_collaborator"})
			return
		}
		if err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.Header().Set("Location", "/repositories/"+r.PathValue("repository")+"/collaborators/"+string(user.ID))
		writeJSON(w, 200, collaboratorResponse(user))
	}
}

func removeCollaborator(store collaboratorRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		if err := store.RemoveCollaborator(actor.UserID, storage.ID(r.PathValue("repository")), r.PathValue("user")); err != nil {
			writeRepositoryError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func collaboratorResponse(user users.User) map[string]any {
	return map[string]any{"user_id": user.ID, "handle": user.Handle, "display_name": user.DisplayName, "role": "contributor"}
}
