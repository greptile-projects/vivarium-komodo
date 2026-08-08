package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type taskSessionRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerProposalTaskSessionsHTTP(mux *http.ServeMux, plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials changeSessionCredentialStore, activity activityStore) {
	base := "/repositories/{repository}/proposals/{proposal}/plan/tasks/{task}/change-sessions"
	mux.HandleFunc("POST "+base, startProposalTask(plans, sessions, repositories, credentials, activity))
	mux.HandleFunc("GET "+base, listProposalTaskSessions(plans, sessions, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}", getProposalTaskSession(plans, sessions, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}/events", getProposalTaskSessionEvents(plans, sessions, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/interventions", interveneProposalTaskRun(plans, sessions, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{session}/runs/{run}/control", controlProposalTaskRun(sessions, credentials))
	mux.HandleFunc("POST "+base+"/{session}/runs/{run}/events", appendProposalTaskRunEvent(sessions, credentials))
}

func taskSessionScope(taskID string) string { return "task-" + taskID }

func proposalTaskContext(w http.ResponseWriter, r *http.Request, plans proposalStore, repositoryStore taskSessionRepositoryStore, credentials authStore, scope auth.Scope, requireActor bool) (repositories.Repository, proposals.Proposal, proposals.Task, string, bool) {
	repository, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, scope, requireActor)
	if !ok {
		return repositories.Repository{}, proposals.Proposal{}, proposals.Task{}, "", false
	}
	proposal, err := plans.Get(string(repository.ID), r.PathValue("proposal"))
	if err != nil {
		writeProposalError(w, err)
		return repositories.Repository{}, proposals.Proposal{}, proposals.Task{}, "", false
	}
	plan, err := plans.GetPlan(string(repository.ID), proposal.ID)
	if err != nil {
		writeProposalError(w, err)
		return repositories.Repository{}, proposals.Proposal{}, proposals.Task{}, "", false
	}
	for _, task := range plan.Tasks {
		if task.ID == r.PathValue("task") {
			return repository, proposal, task, actor.UserID, true
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	return repositories.Repository{}, proposals.Proposal{}, proposals.Task{}, "", false
}

func startProposalTask(plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials changeSessionCredentialStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, proposal, task, actorID, ok := proposalTaskContext(w, r, plans, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			ExpectedAssignmentID string `json:"expected_assignment_id"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		if task.Assignment == nil || task.Assignment.ID != input.ExpectedAssignmentID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "assignment_changed"})
			return
		}
		if task.Assignment.Kind != proposals.AgentAssignee || !availableTaskAgents[task.Assignment.AssigneeID] {
			writeJSON(w, 422, map[string]string{"error": "assignment_not_agent"})
			return
		}
		if task.Assignment.SessionID != "" || task.Status != proposals.TaskPlanned || !task.Ready {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "task_already_started"})
			return
		}
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err := opened.ReadCommit(storage.ObjectID(task.Assignment.BaseRevision)); err != nil {
			writeJSON(w, 409, map[string]string{"error": "base_revision_unavailable"})
			return
		}
		branch := "codex/task-" + task.ID[:12] + "-" + task.Assignment.ID[:8]
		refName := storage.ReferenceName("refs/heads/" + branch)
		if err := opened.CreateReference(storage.Reference{Name: refName, ObjectID: storage.ObjectID(task.Assignment.BaseRevision)}); err != nil {
			if errors.Is(err, storage.ErrReferenceExists) {
				writeJSON(w, 409, map[string]string{"error": "task_already_started"})
			} else {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
			}
			return
		}
		rollbackBranch := true
		defer func() {
			if rollbackBranch {
				_ = opened.DeleteReference(refName)
			}
		}()
		issued, err := credentials.IssueRepositoryGit(actorID, "Proposal task "+task.ID, string(repository.ID), string(refName), 24*time.Hour)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		rollbackGrant := true
		defer func() {
			if rollbackGrant {
				_, _ = credentials.Revoke(actorID, issued.ID)
			}
		}()
		plan, _ := plans.GetPlan(string(repository.ID), proposal.ID)
		dependencies := []changesessions.TaskDependency{}
		for _, dependencyID := range task.DependsOn {
			for _, candidate := range plan.Tasks {
				if candidate.ID == dependencyID {
					dependencies = append(dependencies, changesessions.TaskDependency{ID: candidate.ID, Title: candidate.Title, Outcome: candidate.Outcome, Status: string(candidate.Status)})
				}
			}
		}
		defaultBranch := ""
		if head, err := opened.DefaultBranch(); err == nil {
			defaultBranch = strings.TrimPrefix(string(head), "refs/heads/")
		}
		context := changesessions.TaskContext{ProposalID: proposal.ID, ProposalTitle: proposal.Title, ProposalDescription: proposal.Body, TaskID: task.ID, TaskTitle: task.Title, TaskOutcome: task.Outcome, Mandate: task.Assignment.Mandate, Dependencies: dependencies, Repository: changesessions.RepositoryContext{ID: string(repository.ID), Name: repository.Name, Description: repository.Description, DefaultBranch: defaultBranch, BaseRevision: task.Assignment.BaseRevision, WorkingBranch: branch}}
		session, err := sessions.CreateForTask(string(repository.ID), taskSessionScope(task.ID), actorID, task.Assignment.BaseRevision, context)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		instructions := task.Assignment.Mandate + "\n\nOutcome: " + task.Outcome
		run, err := sessions.Delegate(string(repository.ID), taskSessionScope(task.ID), session.ID, changesessions.DelegateParams{InitiatorID: actorID, Agent: task.Assignment.AssigneeID, Instructions: instructions, RevisionID: task.Assignment.BaseRevision, WorkingBranch: branch, CredentialGrantID: issued.ID, CredentialExpiresAt: issued.ExpiresAt})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		started, err := plans.StartAssignedTask(string(repository.ID), proposal.ID, task.ID, actorID, input.ExpectedAssignmentID, session.ID, branch)
		if err != nil {
			writeProposalTaskError(w, err)
			return
		}
		rollbackBranch, rollbackGrant = false, false
		_ = recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actorID, Type: "proposal.task.started", Resource: activities.Resource{Type: "proposal", ID: proposal.ID}, Metadata: map[string]string{"task_id": task.ID, "session_id": session.ID, "run_id": run.ID, "working_branch": branch}})
		location := "/repositories/" + string(repository.ID) + "/proposals/" + proposal.ID + "/plan/tasks/" + task.ID + "/change-sessions/" + session.ID
		w.Header().Set("Location", location)
		writeJSON(w, http.StatusCreated, map[string]any{"task": started, "session": session, "run": run, "credential": map[string]any{"token": issued.Token, "username": "agent", "expires_at": issued.ExpiresAt, "repository_id": string(repository.ID), "branch": string(refName)}})
	}
}

func listProposalTaskSessions(plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, task, _, ok := proposalTaskContext(w, r, plans, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := sessions.List(r.PathValue("repository"), taskSessionScope(task.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	}
}

func getProposalTaskSession(plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, task, _, ok := proposalTaskContext(w, r, plans, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := sessions.Get(r.PathValue("repository"), taskSessionScope(task.ID), r.PathValue("session"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, item)
	}
}

func getProposalTaskSessionEvents(plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, task, _, ok := proposalTaskContext(w, r, plans, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := sessions.Events(r.PathValue("repository"), taskSessionScope(task.ID), r.PathValue("session"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	}
}

func taskRun(session changesessions.Session, id string) *changesessions.Run {
	for i := range session.Runs {
		if session.Runs[i].ID == id {
			return &session.Runs[i]
		}
	}
	return nil
}

func validateTaskWorker(w http.ResponseWriter, r *http.Request, sessions changeSessionStore, credentials changeSessionCredentialStore) (auth.Grant, changesessions.Session, *changesessions.Run, bool) {
	grant, ok := authenticateRequest(w, r, credentials, auth.GitWrite)
	if !ok {
		return auth.Grant{}, changesessions.Session{}, nil, false
	}
	session, err := sessions.Get(r.PathValue("repository"), taskSessionScope(r.PathValue("task")), r.PathValue("session"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return auth.Grant{}, changesessions.Session{}, nil, false
	}
	run := taskRun(session, r.PathValue("run"))
	if run == nil || grant.ID != run.CredentialGrantID || grant.RepositoryID != r.PathValue("repository") || grant.Branch != "refs/heads/"+run.WorkingBranch {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return auth.Grant{}, changesessions.Session{}, nil, false
	}
	return grant, session, run, true
}

func controlProposalTaskRun(sessions changeSessionStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, session, run, ok := validateTaskWorker(w, r, sessions, credentials)
		if !ok {
			return
		}
		events := []changesessions.Event{}
		for _, event := range session.Events {
			if event.RunID == run.ID && event.Type == "run.intervention" {
				events = append(events, event)
			}
		}
		writeJSON(w, 200, map[string]any{"state": run.State, "interventions": events})
	}
}

func appendProposalTaskRunEvent(sessions changeSessionStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, run, ok := validateTaskWorker(w, r, sessions, credentials)
		if !ok {
			return
		}
		var input struct {
			Type     string            `json:"type"`
			Metadata map[string]string `json:"metadata"`
		}
		if !readJSON(w, r, &input, 16384) {
			return
		}
		event, err := sessions.AppendRunEvent(r.PathValue("repository"), taskSessionScope(r.PathValue("task")), r.PathValue("session"), run.ID, input.Type, input.Metadata)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_run_event"})
			return
		}
		if input.Type == "run.failed" {
			_, _ = credentials.Revoke(run.InitiatorID, run.CredentialGrantID)
			_, _ = sessions.RevokeRunCredential(r.PathValue("repository"), taskSessionScope(r.PathValue("task")), r.PathValue("session"), run.ID, time.Now())
		}
		writeJSON(w, 201, event)
	}
}

func interveneProposalTaskRun(plans proposalStore, sessions changeSessionStore, repositories taskSessionRepositoryStore, credentials changeSessionCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, task, actorID, ok := proposalTaskContext(w, r, plans, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		session, err := sessions.Get(r.PathValue("repository"), taskSessionScope(task.ID), r.PathValue("session"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		run := taskRun(session, r.PathValue("run"))
		if run == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if !readJSON(w, r, &input, 16384) {
			return
		}
		event, updated, err := sessions.Intervene(r.PathValue("repository"), taskSessionScope(task.ID), session.ID, run.ID, actorID, input.Type, input.Message)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "invalid_run_transition"})
			return
		}
		if input.Type == "cancel" {
			_, _ = credentials.Revoke(run.InitiatorID, run.CredentialGrantID)
			_, _ = sessions.RevokeRunCredential(r.PathValue("repository"), taskSessionScope(task.ID), session.ID, run.ID, time.Now())
		}
		writeJSON(w, 201, map[string]any{"event": event, "run": updated})
	}
}
