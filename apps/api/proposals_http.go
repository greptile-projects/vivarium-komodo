package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type proposalStore interface {
	Create(string, string, string, string) (proposals.Proposal, error)
	Get(string, string) (proposals.Proposal, error)
	List(string) ([]proposals.Proposal, error)
	Update(string, string, string, string) (proposals.Proposal, error)
	Close(string, string, string) (proposals.Proposal, error)
	AddComment(string, string, string, string) (proposals.Comment, error)
	ListComments(string, string) ([]proposals.Comment, error)
}

type proposalRepositoryStore interface {
	Inspect(storage.ID) (repositories.Repository, error)
	IsCollaborator(storage.ID, string) (bool, error)
}

func registerProposalsHTTP(mux *http.ServeMux, store proposalStore, repositories proposalRepositoryStore, credentials authStore, activityStores ...activityStore) {
	var activity activityStore
	if len(activityStores) > 0 {
		activity = activityStores[0]
	}
	mux.HandleFunc("POST /repositories/{repository}/proposals", createProposal(store, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/proposals", listProposals(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/proposals/{proposal}", getProposal(store, repositories, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}/proposals/{proposal}", updateProposal(store, repositories, credentials, activity))
	mux.HandleFunc("POST /repositories/{repository}/proposals/{proposal}/comments", createProposalComment(store, repositories, credentials, activity))
	mux.HandleFunc("GET /repositories/{repository}/proposals/{proposal}/comments", listProposalComments(store, repositories, credentials))
}

func proposalRepositoryAccess(w http.ResponseWriter, r *http.Request, store proposalRepositoryStore, credentials authStore, scope auth.Scope, requireAuthentication bool) (repositories.Repository, auth.Grant, bool) {
	repository, err := store.Inspect(storage.ID(r.PathValue("repository")))
	if err != nil {
		writeRepositoryError(w, err)
		return repositories.Repository{}, auth.Grant{}, false
	}
	actor, authenticated, ok := authenticateOptionalRequest(w, r, credentials, scope)
	if !ok {
		return repositories.Repository{}, auth.Grant{}, false
	}
	if requireAuthentication && !authenticated {
		writeUnauthenticated(w, "Bearer", "komodo")
		return repositories.Repository{}, auth.Grant{}, false
	}
	if repository.Visibility == repositories.Public {
		return repository, actor, true
	}
	if !authenticated {
		writeUnauthenticated(w, "Bearer", "komodo")
		return repositories.Repository{}, auth.Grant{}, false
	}
	if actor.UserID == repository.OwnerID {
		return repository, actor, true
	}
	collaborator, err := store.IsCollaborator(repository.ID, actor.UserID)
	if err != nil {
		writeRepositoryError(w, err)
		return repositories.Repository{}, auth.Grant{}, false
	}
	if !collaborator {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return repositories.Repository{}, auth.Grant{}, false
	}
	return repository, actor, true
}

func createProposal(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if !readJSON(w, r, &input, 70<<10) {
			return
		}
		item, err := store.Create(string(repository.ID), actor.UserID, input.Title, input.Body)
		if errors.Is(err, proposals.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.created", Resource: activities.Resource{Type: "proposal", ID: item.ID}, Metadata: map[string]string{"title": item.Title}, MentionText: item.Title + "\n" + item.Body}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		location := "/repositories/" + string(repository.ID) + "/proposals/" + item.ID
		w.Header().Set("Location", location)
		writeJSON(w, http.StatusCreated, item)
	}
}

func listProposals(store proposalStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
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
		state := r.URL.Query().Get("state")
		if state != "" && state != string(proposals.Open) && state != string(proposals.Closed) {
			writeJSON(w, 422, map[string]string{"error": "invalid_state"})
			return
		}
		if state != "" {
			filtered := []proposals.Proposal{}
			for _, item := range items {
				if string(item.State) == state {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func getProposal(store proposalStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("proposal"))
		if err != nil {
			writeProposalError(w, err)
			return
		}
		writeJSON(w, 200, item)
	}
}

func updateProposal(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		current, err := store.Get(string(repository.ID), r.PathValue("proposal"))
		if err != nil {
			writeProposalError(w, err)
			return
		}
		if actor.UserID != current.AuthorID && actor.UserID != repository.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Title *string          `json:"title"`
			Body  *string          `json:"body"`
			State *proposals.State `json:"state"`
		}
		if !readJSON(w, r, &input, 70<<10) {
			return
		}
		if input.Title == nil && input.Body == nil && input.State == nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
			return
		}
		if input.State != nil && *input.State != proposals.Closed {
			writeJSON(w, 422, map[string]string{"error": "invalid_state"})
			return
		}
		item := current
		if input.Title != nil || input.Body != nil {
			title, body := current.Title, current.Body
			if input.Title != nil {
				title = *input.Title
			}
			if input.Body != nil {
				body = *input.Body
			}
			item, err = store.Update(string(repository.ID), current.ID, title, body)
		}
		if err == nil && input.State != nil {
			item, err = store.Close(string(repository.ID), current.ID, actor.UserID)
		}
		if errors.Is(err, proposals.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
			return
		}
		if err != nil {
			writeProposalError(w, err)
			return
		}
		resource := activities.Resource{Type: "proposal", ID: item.ID}
		if input.Title != nil || input.Body != nil {
			if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.updated", Resource: resource, Metadata: map[string]string{"title": item.Title}, MentionText: item.Title + "\n" + item.Body}); err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		if input.State != nil {
			if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.closed", Resource: resource, Metadata: map[string]string{"title": item.Title}}); err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		writeJSON(w, 200, item)
	}
}

func createProposalComment(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &input, 70<<10) {
			return
		}
		item, err := store.AddComment(string(repository.ID), r.PathValue("proposal"), actor.UserID, input.Body)
		if errors.Is(err, proposals.ErrInvalidComment) {
			writeJSON(w, 422, map[string]string{"error": "invalid_comment"})
			return
		}
		if err != nil {
			writeProposalError(w, err)
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.commented", Resource: activities.Resource{Type: "proposal", ID: item.ProposalID}, Metadata: map[string]string{"comment_id": item.ID}, MentionText: item.Body}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusCreated, item)
	}
}

func listProposalComments(store proposalStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.ListComments(string(repository.ID), r.PathValue("proposal"))
		if err != nil {
			writeProposalError(w, err)
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	}
}

func writeProposalError(w http.ResponseWriter, err error) {
	if errors.Is(err, proposals.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
