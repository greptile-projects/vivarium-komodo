package main

import (
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type inboxActivityStore interface {
	ListAll() ([]activities.Event, error)
}
type inboxStateStore interface {
	Cleared(string) (map[string]bool, error)
	Clear(string, string) error
}
type inboxRepositoryStore interface {
	ListAccessible(string) ([]repositories.Repository, error)
}
type inboxProposalStore interface {
	Get(string, string) (proposals.Proposal, error)
}
type inboxPullRequestStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
}
type inboxUserStore interface {
	Get(users.ID) (users.User, error)
}

type inboxItem struct {
	ID             string `json:"id"`
	Classification string `json:"classification"`
	EventType      string `json:"event_type"`
	RepositoryID   string `json:"repository_id"`
	RepositoryName string `json:"repository_name"`
	ActorID        string `json:"actor_id"`
	ActorHandle    string `json:"actor_handle"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Href           string `json:"href"`
	CreatedAt      any    `json:"created_at"`
}

func registerInboxHTTP(mux *http.ServeMux, activity inboxActivityStore, state inboxStateStore, repositoryStore inboxRepositoryStore, proposalStore inboxProposalStore, pullStore inboxPullRequestStore, userStore inboxUserStore, credentials authStore) {
	build := func(userID string) ([]inboxItem, error) {
		accessibleRepositories, err := repositoryStore.ListAccessible(userID)
		if err != nil {
			return nil, err
		}
		repoByID := map[string]repositories.Repository{}
		for _, repository := range accessibleRepositories {
			repoByID[string(repository.ID)] = repository
		}
		events, err := activity.ListAll()
		if err != nil {
			return nil, err
		}
		cleared, err := state.Cleared(userID)
		if err != nil {
			return nil, err
		}
		items := []inboxItem{}
		mentionedSources := map[string]bool{}
		for _, event := range events {
			if event.Type == "mention.created" && event.TargetUserID == userID {
				mentionedSources[event.Metadata["source_event_id"]] = true
			}
		}
		for _, event := range events {
			repository, accessible := repoByID[event.RepositoryID]
			if !accessible || event.ActorID == userID || cleared[event.ID] {
				continue
			}
			classification, title, summary, href := "", "", "", ""
			switch event.Type {
			case "mention.created":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "You were mentioned", "Open the conversation and respond if needed."
					if event.Resource.Type == "pull_request" {
						if pull, getErr := pullStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && pull.Status != pullrequests.Open {
							classification, summary = "awareness", "Review the completed conversation."
						}
					}
					if event.Resource.Type == "proposal" {
						if proposal, getErr := proposalStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && proposal.State != proposals.Open {
							classification, summary = "awareness", "Review the completed conversation."
						}
					}
				}
			case "access.granted":
				if event.TargetUserID == userID {
					classification, title, summary = "awareness", "Repository access granted", "You can now contribute to this repository."
				}
			case "pull_request.created":
				if pull, getErr := pullStore.Get(event.RepositoryID, event.Resource.ID); repository.OwnerID == userID && getErr == nil && pull.Status == pullrequests.Open {
					classification, title, summary = "review", "Pull request ready for review", "Review the proposed change and leave a decision."
				}
			case "review.submitted", "review.replaced":
				if pull, getErr := pullStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && pull.AuthorID == userID && pull.Status == pullrequests.Open {
					if event.Metadata["decision"] == "request_changes" {
						classification, title, summary = "response", "Changes requested", "Respond to the review and publish follow-up work."
					} else if event.Metadata["decision"] == "approve" {
						classification, title, summary = "awareness", "Pull request approved", "Your change received an approval."
					}
				}
			case "pull_request.commented":
				if pull, getErr := pullStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && pull.AuthorID == userID && pull.Status == pullrequests.Open && !mentionedSources[event.ID] {
					classification, title, summary = "response", "New pull request reply", "Read the discussion and respond if needed."
				}
			case "pull_request.merged":
				if pull, getErr := pullStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && pull.AuthorID == userID {
					classification, title, summary = "awareness", "Pull request merged", "Your change is now part of the target branch."
				}
			case "integration_queue.blocked":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Integration is blocked", "Inspect the candidate evidence and coordinate a retry or update."
				}
			case "integration_queue.removed":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Change left the integration queue", "Review the recorded reason before synchronizing or asking for readmission."
				}
			case "integration_queue.merged":
				if event.TargetUserID == userID {
					classification, title, summary = "awareness", "Change landed from the queue", "The verified candidate is now on the target branch."
				}
			case "proposal.commented":
				if proposal, getErr := proposalStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && proposal.AuthorID == userID && proposal.State == proposals.Open && !mentionedSources[event.ID] {
					classification, title, summary = "response", "New proposal reply", "Read the discussion and continue the decision."
				}
			case "proposal.created":
				if proposal, getErr := proposalStore.Get(event.RepositoryID, event.Resource.ID); repository.OwnerID == userID && getErr == nil && proposal.State == proposals.Open {
					classification, title, summary = "awareness", "New repository proposal", "See what a collaborator is proposing."
				}
			case "proposal.closed":
				if proposal, getErr := proposalStore.Get(event.RepositoryID, event.Resource.ID); getErr == nil && proposal.AuthorID == userID {
					classification, title, summary = "awareness", "Proposal closed", "Review the recorded outcome."
				}
			case "proposal.task.ready":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Assigned task is ready", "Its dependencies are complete; begin from the assigned revision."
				}
			case "proposal.task.blocked":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Assigned task is blocked", "A dependency changed; pause work and inspect the plan."
				}
			case "proposal.task.changed", "proposal.task.context_changed":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Assigned task changed", "Review the revised outcome, dependencies, or starting revision."
				}
			case "proposal.task.obsolete":
				if event.TargetUserID == userID {
					classification, title, summary = "awareness", "Assigned task is obsolete", "The plan no longer expects this work to continue."
				}
			case "deployment.unhealthy", "deployment.failed":
				classification, title, summary = "response", "Deployment needs intervention", "Inspect retained health evidence and pause, cancel, or mark the rollout unsuccessful."
			case "deployment.pause", "deployment.cancel", "deployment.fail":
				if event.TargetUserID == userID {
					classification, title, summary = "response", "Deployment rollout changed", "Review the attributed rollout decision and coordinate the next action."
				}
			case "deployment.resume", "deployment.succeeded":
				if event.TargetUserID == "" || event.TargetUserID == userID {
					classification, title, summary = "awareness", "Deployment rollout updated", "Review the latest environment and health evidence."
				}
			}
			if classification == "" {
				continue
			}
			if event.Resource.Type == "pull_request" {
				href = "/repositories/" + event.RepositoryID + "?view=pulls&pull=" + event.Resource.ID
			}
			if event.Resource.Type == "proposal" {
				href = "/repositories/" + event.RepositoryID + "?view=proposals&proposal=" + event.Resource.ID
			}
			if event.Resource.Type == "repository" {
				href = "/repositories/" + event.RepositoryID
			}
			if event.Resource.Type == "deployment" {
				href = "/repositories/" + event.RepositoryID + "?view=releases&release=" + event.Metadata["release_id"] + "#deployment-" + event.Resource.ID
			}
			actorHandle := event.ActorID
			if actor, getErr := userStore.Get(users.ID(event.ActorID)); getErr == nil {
				actorHandle = actor.Handle
			}
			items = append(items, inboxItem{ID: event.ID, Classification: classification, EventType: event.Type, RepositoryID: event.RepositoryID, RepositoryName: repository.Name, ActorID: event.ActorID, ActorHandle: actorHandle, Title: title, Summary: summary, Href: href, CreatedAt: event.CreatedAt})
		}
		return items, nil
	}
	mux.HandleFunc("GET /inbox", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		items, err := build(actor.UserID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		classification := r.URL.Query().Get("classification")
		if classification != "" && classification != "review" && classification != "response" && classification != "awareness" {
			writeJSON(w, 422, map[string]string{"error": "invalid_classification"})
			return
		}
		if classification != "" {
			filtered := items[:0]
			for _, item := range items {
				if item.Classification == classification {
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
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	})
	mux.HandleFunc("DELETE /inbox/{item}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		items, err := build(actor.UserID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		found := false
		for _, item := range items {
			if item.ID == r.PathValue("item") {
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err := state.Clear(actor.UserID, r.PathValue("item")); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
