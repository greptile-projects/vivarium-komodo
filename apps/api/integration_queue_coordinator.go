package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/integrationqueue"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type queueRepositoryStore interface {
	Inspect(storage.ID) (repositories.Repository, error)
	Open(storage.ID) (*storage.Repository, error)
	LinkObjects(storage.ID, storage.ID) error
}

// integrationQueueCoordinator serializes decisions per process. The branch
// compare-and-swap remains the cross-process authority, so a restarted or
// concurrent coordinator can never publish evidence for an obsolete base.
type integrationQueueCoordinator struct {
	queue        *integrationqueue.Store
	pulls        pullRequestStore
	repositories queueRepositoryStore
	checks       readinessCheckStore
	starter      checkRunStarter
	activity     activityStore
	proposals    proposalStore
	security     *securityDeliverySources
	mu           sync.Mutex
}

func (c *integrationQueueCoordinator) reconcileAll(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := c.queue.ListActive()
	if err != nil {
		return
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		key := entry.RepositoryID + "\x00" + entry.TargetBranch
		if seen[key] {
			continue
		}
		seen[key] = true
		c.reconcileBranch(ctx, entry.RepositoryID, entry.TargetBranch)
	}
}

func (c *integrationQueueCoordinator) run(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	c.reconcileAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reconcileAll(ctx)
		}
	}
}

func (c *integrationQueueCoordinator) reconcileBranch(ctx context.Context, repositoryID, branch string) {
	repository, err := c.repositories.Inspect(storage.ID(repositoryID))
	if err != nil {
		return
	}
	policy, enabled := repository.IntegrationQueue[branch]
	if !enabled || !policy.Enabled {
		return
	}
	opened, err := c.repositories.Open(repository.ID)
	if err != nil {
		return
	}

	for {
		items, listErr := c.queue.List(repositoryID, branch)
		if listErr != nil || len(items) == 0 {
			return
		}
		entry := items[0]
		if entry.State == "paused" {
			return
		}
		pull, pullErr := c.pulls.Get(repositoryID, entry.PullRequestID)
		if pullErr != nil {
			c.transition(entry, pullrequests.PullRequest{}, "removed", "pull_request_closed", true)
			continue
		}
		liveTarget, _, targetFound := branchTip(opened, branch)
		if !targetFound {
			return
		}
		// Recover the narrow post-CAS window before consulting mutable source
		// state: once this exact candidate is the target, its pull request must
		// be finalized even if the source branch subsequently moves.
		if string(liveTarget) == entry.CandidateCommitID {
			if !c.finishMerged(repository, pull, entry) {
				return
			}
			continue
		}
		if pull.Status != pullrequests.Open {
			c.transition(entry, pull, "removed", "pull_request_closed", true)
			continue
		}
		sourceRepository, sourceErr := c.repositories.Open(storage.ID(pull.SourceRepositoryID))
		if sourceErr != nil {
			c.transition(entry, pull, "removed", "source_unavailable", true)
			continue
		}
		liveSource, _, sourceFound := branchTip(sourceRepository, pull.SourceBranch)
		if !sourceFound || string(liveSource) != entry.SourceCommitID || pull.SourceCommitID != entry.SourceCommitID {
			c.transition(entry, pull, "removed", "source_updated", true)
			continue
		}
		if string(liveTarget) != entry.TargetCommitID {
			c.rebuildAffected(ctx, repository, opened, items, liveTarget, policy)
			updated, updateErr := c.queue.Get(entry.ID)
			if updateErr == nil && updated.CompletedAt == nil && updated.TargetCommitID != string(liveTarget) {
				return
			}
			continue
		}
		reviews, reviewErr := c.pulls.ListReviews(repositoryID, pull.ID)
		if reviewErr != nil {
			return
		}
		ownerApproved, changesRequested := false, false
		for _, review := range reviews {
			if review.CommitID != entry.SourceCommitID {
				continue
			}
			ownerApproved = ownerApproved || (review.ReviewerID == repository.OwnerID && review.Decision == pullrequests.Approve)
			changesRequested = changesRequested || review.Decision == pullrequests.RequestChanges
		}
		if !ownerApproved || changesRequested {
			reason := "approval_withdrawn"
			if changesRequested {
				reason = "changes_requested"
			}
			c.transition(entry, pull, "blocked", reason, false)
			return
		}

		runs, runErr := c.checks.List(repositoryID, pull.ID)
		if runErr != nil {
			return
		}
		result := evaluateRequiredChecks(entry.RequiredChecks, entry.CandidateCommitID, runs)
		if !result.Satisfied {
			pending, failed, canceled := false, false, false
			for _, requirement := range result.Requirements {
				switch requirement.Status {
				case "pending":
					pending = true
				case "failed":
					failed = true
				case "canceled":
					failed, canceled = true, true
				}
			}
			if pending || !failed {
				return
			}
			reason := "checks_failed"
			if canceled {
				reason = "checks_canceled"
			}
			if policy.FailureBehavior == "remove" {
				c.transition(entry, pull, "removed", reason, true)
				continue
			}
			c.transition(entry, pull, "blocked", reason, false)
			return
		}
		if c.security != nil {
			changes, evidenceErr := filesBetweenRepositories(sourceRepository, opened, storage.ObjectID(entry.SourceCommitID), storage.ObjectID(entry.TargetCommitID))
			if evidenceErr != nil {
				return
			}
			paths := make([]string, len(changes))
			risks := []string{}
			for i := range changes {
				paths[i] = changes[i].Path
				if strings.Contains(changes[i].Path, "security") || strings.Contains(changes[i].Path, "auth") {
					risks = append(risks, "critical")
				}
			}
			a, evidenceErr := c.security.assess(repositoryID, repository.OrganizationID, "integration_queue", entry.ID, entry.CandidateCommitID, branch, paths, nil, risks)
			if evidenceErr != nil {
				c.transition(entry, pull, "blocked", "security_evidence_unavailable", false)
				return
			}
			if !a.Ready {
				c.transition(entry, pull, "blocked", "security_requirements_unsatisfied", false)
				return
			}
		}

		err = opened.CompareAndSwapReference(storage.ReferenceName("refs/heads/"+branch), storage.ObjectID(entry.TargetCommitID), storage.ObjectID(entry.CandidateCommitID))
		if errors.Is(err, storage.ErrReferenceChanged) {
			continue
		}
		if err != nil {
			return
		}
		if !c.finishMerged(repository, pull, entry) {
			return
		}
	}
}

func (c *integrationQueueCoordinator) transition(entry integrationqueue.Entry, pull pullrequests.PullRequest, state, reason string, terminal bool) {
	if entry.State == state && entry.Reason == reason {
		return
	}
	updated, err := c.queue.Transition(entry.ID, state, reason, terminal)
	if err != nil || c.activity == nil || pull.ID == "" {
		return
	}
	eventType := "integration_queue." + state
	_, _ = c.activity.Record(activities.Input{RepositoryID: entry.RepositoryID, ActorID: "integration-queue", Type: eventType, Resource: activities.Resource{Type: "pull_request", ID: pull.ID}, TargetUserID: pull.AuthorID, Metadata: map[string]string{"entry_id": updated.ID, "branch": updated.TargetBranch, "reason": reason}})
}

func (c *integrationQueueCoordinator) rebuildAffected(ctx context.Context, repository repositories.Repository, opened *storage.Repository, entries []integrationqueue.Entry, target storage.ObjectID, policy repositories.IntegrationQueuePolicy) {
	limit := policy.Concurrency
	if limit < 1 || limit > len(entries) {
		limit = len(entries)
	}
	for _, entry := range entries[:limit] {
		if entry.TargetCommitID == string(target) {
			continue
		}
		pull, err := c.pulls.Get(string(repository.ID), entry.PullRequestID)
		if err != nil || pull.Status != pullrequests.Open {
			c.transition(entry, pull, "removed", "pull_request_closed", true)
			continue
		}
		sourceRepository, err := c.repositories.Open(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			c.transition(entry, pull, "removed", "source_unavailable", true)
			continue
		}
		liveSource, _, found := branchTip(sourceRepository, pull.SourceBranch)
		if !found || string(liveSource) != entry.SourceCommitID || pull.SourceCommitID != entry.SourceCommitID {
			c.transition(entry, pull, "removed", "source_updated", true)
			continue
		}
		_ = c.rebuild(ctx, repository, opened, pull, entry, target, policy)
	}
}

func (c *integrationQueueCoordinator) finishMerged(repository repositories.Repository, pull pullrequests.PullRequest, entry integrationqueue.Entry) bool {
	if pull.Status == pullrequests.Open {
		_, err := c.pulls.MarkMerged(string(repository.ID), pull.ID, repository.OwnerID, entry.CandidateCommitID)
		if err != nil {
			return false
		}
		_, _ = c.pulls.AddComment(string(repository.ID), pull.ID, repository.OwnerID, "Merged from the integration queue as "+entry.CandidateCommitID+".")
	} else if pull.Status != pullrequests.Merged || pull.MergeCommitID != entry.CandidateCommitID {
		return false
	}
	if c.proposals != nil && pull.ProposalID != "" && pull.TaskID != "" {
		before, _ := c.proposals.GetPlan(pull.RepositoryID, pull.ProposalID)
		if _, err := c.proposals.UpdateTaskContribution(pull.RepositoryID, pull.ProposalID, pull.TaskID, pull.ID, repository.OwnerID, proposals.ContributionMerged); err != nil {
			return false
		}
		after, _ := c.proposals.GetPlan(pull.RepositoryID, pull.ProposalID)
		recordTaskCoordinationChanges(c.activity, pull.RepositoryID, repository.OwnerID, pull.ProposalID, before, after)
	}
	_, err := c.queue.Transition(entry.ID, "merged", "", true)
	if err == nil && c.activity != nil {
		_, _ = c.activity.Record(activities.Input{RepositoryID: entry.RepositoryID, ActorID: repository.OwnerID, Type: "integration_queue.merged", Resource: activities.Resource{Type: "pull_request", ID: pull.ID}, TargetUserID: pull.AuthorID, Metadata: map[string]string{"entry_id": entry.ID, "branch": entry.TargetBranch, "candidate_commit_id": entry.CandidateCommitID}})
	}
	return err == nil
}

func (c *integrationQueueCoordinator) rebuild(ctx context.Context, repository repositories.Repository, opened *storage.Repository, pull pullrequests.PullRequest, entry integrationqueue.Entry, target storage.ObjectID, policy repositories.IntegrationQueuePolicy) bool {
	if pull.SourceRepositoryID != pull.RepositoryID {
		if err := c.repositories.LinkObjects(storage.ID(pull.SourceRepositoryID), repository.ID); err != nil {
			return false
		}
	}
	source := storage.ObjectID(entry.SourceCommitID)
	conflicts, err := mergeHasConflicts(ctx, opened, target, source)
	if err != nil {
		return false
	}
	if conflicts {
		terminal := policy.FailureBehavior == "remove"
		state := "blocked"
		if terminal {
			state = "removed"
		}
		c.transition(entry, pull, state, "merge_conflict", terminal)
		return false
	}
	tree, err := materializeMergeTree(ctx, opened, target, source)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	stamp := fmt.Sprintf("%d +0000", now.Unix())
	message := fmt.Sprintf("Integration candidate for %s\n\nPull-Request: %s\nSource-Repository: %s\nSource-Commit: %s\nTarget-Commit: %s\n", pull.Title, pull.ID, pull.SourceRepositoryID, source, target)
	content := fmt.Sprintf("tree %s\nparent %s\nparent %s\nauthor %s <%s@users.local> %s\ncommitter %s <%s@users.local> %s\n\n%s", tree, target, source, pull.AuthorID, pull.AuthorID, stamp, repository.OwnerID, repository.OwnerID, stamp, message)
	candidate, err := opened.WriteObject(storage.CommitObject, []byte(content))
	if err != nil {
		return false
	}
	updated, err := c.queue.ReplaceCandidate(entry.ID, string(target), string(candidate), string(tree), repository.RequiredChecks[pull.TargetBranch])
	if err != nil {
		return false
	}
	if c.starter != nil {
		_ = c.starter.Start(string(repository.ID), string(repository.ID), pull.ID, updated.CandidateCommitID)
	}
	return true
}
