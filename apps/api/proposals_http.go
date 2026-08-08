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
	GetPlan(string, string) (proposals.Plan, error)
	CreateTask(string, string, string, proposals.TaskInput) (proposals.Task, error)
	UpdateTask(string, string, string, string, proposals.TaskInput) (proposals.Task, error)
	AssignTask(string, string, string, string, string, proposals.AssignmentInput) (proposals.Task, error)
	RevokeTaskAssignment(string, string, string, string, string) (proposals.Task, error)
	StartAssignedTask(string, string, string, string, string, string, string) (proposals.Task, error)
	PublishTaskContribution(string, string, string, string, proposals.TaskContribution) (proposals.Task, error)
	UpdateTaskContribution(string, string, string, string, string, proposals.ContributionStatus) (proposals.Task, error)
	RebaseTaskAssignment(string, string, string, string, string, string) (proposals.Task, error)
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
	mux.HandleFunc("GET /repositories/{repository}/proposals/{proposal}/plan", getProposalPlan(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/proposals/{proposal}/plan/tasks", createProposalTask(store, repositories, credentials, activity))
	mux.HandleFunc("PATCH /repositories/{repository}/proposals/{proposal}/plan/tasks/{task}", updateProposalTask(store, repositories, credentials, activity))
	mux.HandleFunc("PUT /repositories/{repository}/proposals/{proposal}/plan/tasks/{task}/assignment", assignProposalTask(store, repositories, credentials, activity))
	mux.HandleFunc("DELETE /repositories/{repository}/proposals/{proposal}/plan/tasks/{task}/assignment", revokeProposalTaskAssignment(store, repositories, credentials, activity))
	mux.HandleFunc("PATCH /repositories/{repository}/proposals/{proposal}/plan/tasks/{task}/assignment/base", rebaseProposalTaskAssignment(store, repositories, credentials, activity))
}

func rebaseProposalTaskAssignment(store proposalStore, repositoryStore proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			ExpectedAssignmentID string `json:"expected_assignment_id"`
			BaseRevision         string `json:"base_revision"`
		}
		if !readJSON(w, r, &input, 4<<10) {
			return
		}
		opener, ok := repositoryStore.(interface {
			Open(storage.ID) (*storage.Repository, error)
		})
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		opened, err := opener.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err = opened.ReadCommit(storage.ObjectID(input.BaseRevision)); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_base_revision"})
			return
		}
		task, err := store.RebaseTaskAssignment(string(repository.ID), r.PathValue("proposal"), r.PathValue("task"), actor.UserID, input.ExpectedAssignmentID, input.BaseRevision)
		if writeProposalTaskError(w, err) {
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.task.context_changed", Resource: activities.Resource{Type: "proposal", ID: task.ProposalID}, TargetUserID: task.Assignment.AssigneeID, Metadata: map[string]string{"task_id": task.ID, "base_revision": task.Assignment.BaseRevision}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, task)
	}
}

var availableTaskAgents = map[string]bool{"codex": true}

func assignProposalTask(store proposalStore, repositoryStore proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Kind                 proposals.AssigneeKind `json:"kind"`
			AssigneeID           string                 `json:"assignee_id"`
			Mandate              string                 `json:"mandate"`
			RepositoryID         string                 `json:"repository_id"`
			BaseRevision         string                 `json:"base_revision"`
			ExpectedAssignmentID string                 `json:"expected_assignment_id"`
		}
		if !readJSON(w, r, &input, 16<<10) {
			return
		}
		if input.RepositoryID == "" {
			input.RepositoryID = string(repository.ID)
		}
		if input.RepositoryID != string(repository.ID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_assignment_repository"})
			return
		}
		if input.Kind == proposals.HumanAssignee {
			participant := input.AssigneeID == repository.OwnerID
			if !participant {
				participant, _ = repositoryStore.IsCollaborator(repository.ID, input.AssigneeID)
			}
			if !participant {
				writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
				return
			}
		} else if input.Kind == proposals.AgentAssignee {
			if !availableTaskAgents[input.AssigneeID] {
				writeJSON(w, 422, map[string]string{"error": "agent_unavailable"})
				return
			}
		}
		opener, ok := repositoryStore.(interface {
			Open(storage.ID) (*storage.Repository, error)
		})
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		opened, err := opener.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err = opened.ReadCommit(storage.ObjectID(input.BaseRevision)); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_base_revision"})
			return
		}
		task, err := store.AssignTask(string(repository.ID), r.PathValue("proposal"), r.PathValue("task"), actor.UserID, input.ExpectedAssignmentID, proposals.AssignmentInput{Kind: input.Kind, AssigneeID: input.AssigneeID, Mandate: input.Mandate, RepositoryID: input.RepositoryID, BaseRevision: input.BaseRevision})
		if writeProposalTaskError(w, err) {
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.task.assigned", Resource: activities.Resource{Type: "proposal", ID: task.ProposalID}, Metadata: map[string]string{"task_id": task.ID, "assignee_id": input.AssigneeID, "kind": string(input.Kind)}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, task)
	}
}

func revokeProposalTaskAssignment(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		task, err := store.RevokeTaskAssignment(string(repository.ID), r.PathValue("proposal"), r.PathValue("task"), actor.UserID, r.URL.Query().Get("expected_assignment_id"))
		if writeProposalTaskError(w, err) {
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.task.assignment_revoked", Resource: activities.Resource{Type: "proposal", ID: task.ProposalID}, Metadata: map[string]string{"task_id": task.ID}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, task)
	}
}

type proposalTaskInput struct {
	Title                string               `json:"title"`
	Outcome              string               `json:"outcome"`
	Position             int                  `json:"position"`
	Status               proposals.TaskStatus `json:"status"`
	DependsOn            []string             `json:"depends_on"`
	DiscussionCommentIDs []string             `json:"discussion_comment_ids"`
}

func (input proposalTaskInput) storeInput() proposals.TaskInput {
	return proposals.TaskInput{Title: input.Title, Outcome: input.Outcome, Position: input.Position, Status: input.Status, DependsOn: input.DependsOn, DiscussionCommentIDs: input.DiscussionCommentIDs}
}

func getProposalPlan(store proposalStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		plan, err := store.GetPlan(string(repository.ID), r.PathValue("proposal"))
		if err != nil {
			writeProposalError(w, err)
			return
		}
		writeJSON(w, 200, plan)
	}
}

func createProposalTask(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input proposalTaskInput
		if !readJSON(w, r, &input, 16<<10) {
			return
		}
		if !validDiscussionLinks(store, string(repository.ID), r.PathValue("proposal"), input.DiscussionCommentIDs) {
			writeJSON(w, 422, map[string]string{"error": "invalid_discussion_link"})
			return
		}
		task, err := store.CreateTask(string(repository.ID), r.PathValue("proposal"), actor.UserID, input.storeInput())
		if writeProposalTaskError(w, err) {
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.task.created", Resource: activities.Resource{Type: "proposal", ID: task.ProposalID}, Metadata: map[string]string{"task_id": task.ID, "title": task.Title}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusCreated, task)
	}
}

func updateProposalTask(store proposalStore, repositories proposalRepositoryStore, credentials authStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input proposalTaskInput
		if !readJSON(w, r, &input, 16<<10) {
			return
		}
		if !validDiscussionLinks(store, string(repository.ID), r.PathValue("proposal"), input.DiscussionCommentIDs) {
			writeJSON(w, 422, map[string]string{"error": "invalid_discussion_link"})
			return
		}
		before, _ := store.GetPlan(string(repository.ID), r.PathValue("proposal"))
		task, err := store.UpdateTask(string(repository.ID), r.PathValue("proposal"), r.PathValue("task"), actor.UserID, input.storeInput())
		if writeProposalTaskError(w, err) {
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "proposal.task.updated", Resource: activities.Resource{Type: "proposal", ID: task.ProposalID}, Metadata: map[string]string{"task_id": task.ID, "title": task.Title, "status": string(task.Status)}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		after, _ := store.GetPlan(string(repository.ID), task.ProposalID)
		recordTaskCoordinationChanges(activity, string(repository.ID), actor.UserID, task.ProposalID, before, after)
		writeJSON(w, 200, task)
	}
}

func recordTaskCoordinationChanges(activity activityStore, repositoryID, actorID, proposalID string, before, after proposals.Plan) {
	if activity == nil {
		return
	}
	previous := map[string]proposals.Task{}
	for _, task := range before.Tasks {
		previous[task.ID] = task
	}
	for _, task := range after.Tasks {
		if task.Assignment == nil || task.Assignment.AssigneeID == actorID {
			continue
		}
		old, exists := previous[task.ID]
		typeName := ""
		if exists && !old.Ready && task.Ready {
			typeName = "proposal.task.ready"
		}
		if exists && old.Ready && !task.Ready {
			typeName = "proposal.task.blocked"
		}
		if exists && (old.Outcome != task.Outcome || !sameTaskDependencies(old.DependsOn, task.DependsOn)) {
			typeName = "proposal.task.changed"
		}
		if task.Status == proposals.TaskCanceled || task.Status == proposals.TaskSuperseded {
			typeName = "proposal.task.obsolete"
		}
		if typeName != "" {
			_, _ = activity.Record(activities.Input{RepositoryID: repositoryID, ActorID: actorID, Type: typeName, Resource: activities.Resource{Type: "proposal", ID: proposalID}, TargetUserID: task.Assignment.AssigneeID, Metadata: map[string]string{"task_id": task.ID}})
		}
	}
}

func sameTaskDependencies(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validDiscussionLinks(store proposalStore, repositoryID, proposalID string, ids []string) bool {
	if len(ids) == 0 {
		return true
	}
	comments, err := store.ListComments(repositoryID, proposalID)
	if err != nil {
		return false
	}
	known := map[string]bool{}
	for _, comment := range comments {
		known[comment.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			return false
		}
	}
	return true
}

func writeProposalTaskError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, proposals.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else if errors.Is(err, proposals.ErrInvalidDependency) {
		writeJSON(w, 422, map[string]string{"error": "invalid_dependency"})
	} else if errors.Is(err, proposals.ErrInvalidTask) {
		writeJSON(w, 422, map[string]string{"error": "invalid_task"})
	} else if errors.Is(err, proposals.ErrTaskNotReady) {
		writeJSON(w, 409, map[string]string{"error": "task_not_ready"})
	} else if errors.Is(err, proposals.ErrTaskAssigned) {
		writeJSON(w, 409, map[string]string{"error": "task_already_assigned"})
	} else if errors.Is(err, proposals.ErrAssignmentConflict) {
		writeJSON(w, 409, map[string]string{"error": "assignment_changed"})
	} else if errors.Is(err, proposals.ErrActiveTaskConflict) {
		writeJSON(w, 409, map[string]string{"error": "task_has_active_work"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
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
