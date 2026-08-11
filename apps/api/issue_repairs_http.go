package main

import (
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
)

func createIssueRepair(store issueStore, plans proposalStore, repos issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, err := store.Get(string(repo.ID), r.PathValue("issue"))
		if err != nil || !issueVisible(repos, repo, item, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			ReproductionID     string   `json:"reproduction_id"`
			InvestigationID    string   `json:"investigation_id"`
			ConclusionEntryID  string   `json:"conclusion_entry_id"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
			OwnerKind          string   `json:"owner_kind"`
			OwnerID            string   `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		if item.Triage.UpdatedAt == nil || (in.OwnerKind != "human" && in.OwnerKind != "agent") || strings.TrimSpace(in.OwnerID) == "" || len(in.AcceptanceCriteria) == 0 {
			writeJSON(w, 422, map[string]string{"error": "triaged_repair_required"})
			return
		}
		if in.OwnerKind == "agent" && in.OwnerID != "codex" {
			writeJSON(w, 422, map[string]string{"error": "unsupported_agent"})
			return
		}
		if in.OwnerKind == "human" {
			participant, _ := repos.IsCollaborator(repo.ID, in.OwnerID)
			if in.OwnerID != repo.OwnerID && !participant {
				writeJSON(w, 422, map[string]string{"error": "assignee_not_participant"})
				return
			}
		}
		attempt, err := store.GetReproduction(string(repo.ID), item.ID, in.ReproductionID)
		if err != nil || !attempt.Reproduced || attempt.State != "completed" {
			writeJSON(w, 422, map[string]string{"error": "confirmed_reproduction_required"})
			return
		}
		var inv *issues.Investigation
		var conclusion *issues.InvestigationEntry
		for i := range item.Investigations {
			if item.Investigations[i].ID == in.InvestigationID {
				inv = &item.Investigations[i]
				break
			}
		}
		if inv != nil && inv.ReproductionID == attempt.ID && inv.Revision == attempt.Revision {
			for i := range inv.Entries {
				e := &inv.Entries[i]
				if e.ID == in.ConclusionEntryID && e.Kind == "conclusion" && !e.Stale && !e.Disputed {
					conclusion = e
				}
			}
		}
		if conclusion == nil {
			writeJSON(w, 422, map[string]string{"error": "current_diagnosis_required"})
			return
		}
		for i := range in.AcceptanceCriteria {
			in.AcceptanceCriteria[i] = strings.TrimSpace(in.AcceptanceCriteria[i])
			if in.AcceptanceCriteria[i] == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_acceptance_criteria"})
				return
			}
		}
		proposal, err := plans.Create(string(repo.ID), actor.UserID, "Repair: "+item.Title, "Governed repair for issue "+item.ID+". The confirmed reproduction and diagnosis remain pinned to "+attempt.Revision+".")
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		context := &reasoning.Context{Kind: "issue_repair", IssueID: item.ID, ReproductionID: attempt.ID, InvestigationID: inv.ID, ConclusionID: conclusion.ID, RepositoryID: string(repo.ID), CommitID: attempt.Revision, Claim: conclusion.Body, State: "confirmed", Rationale: item.ObservedBehavior, Verification: append([]string{}, in.AcceptanceCriteria...), Evidence: []reasoning.Evidence{{RepositoryID: string(repo.ID), CommitID: attempt.Revision, Kind: "issue_reproduction", ResourceID: attempt.ID, Label: attempt.ObservedResult}, {RepositoryID: string(repo.ID), CommitID: attempt.Revision, Kind: "issue_diagnosis", ResourceID: conclusion.ID, Label: conclusion.Body}}}
		task, err := plans.CreateTask(string(repo.ID), proposal.ID, actor.UserID, proposals.TaskInput{Title: "Repair " + item.Title, Outcome: item.ExpectedBehavior, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: in.AcceptanceCriteria, BaseRevision: attempt.Revision, ReasoningContext: context})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, err = plans.AssignTask(string(repo.ID), proposal.ID, task.ID, actor.UserID, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Repair issue " + item.ID + " from the retained reproduction and diagnosis; satisfy every acceptance criterion and publish through ordinary pull-request review.", RepositoryID: string(repo.ID), BaseRevision: attempt.Revision})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		updated, repair, err := store.CreateRepair(string(repo.ID), item.ID, actor.UserID, issues.Repair{ReproductionID: attempt.ID, InvestigationID: inv.ID, ConclusionEntryID: conclusion.ID, Revision: attempt.Revision, AcceptanceCriteria: in.AcceptanceCriteria, ProposalID: proposal.ID, TaskID: task.ID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"issue": updated, "repair": repair, "proposal": proposal, "task": task, "authority": map[string]bool{"granted_by_issue": false, "credential_issued": false, "merge": false}})
	}
}

func linkIssueRepairPullRequest(store issueStore, plans proposalStore, pulls pullRequestStore, repos issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		item, err := store.Get(string(repo.ID), r.PathValue("issue"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			PullRequestID string `json:"pull_request_id"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		var repair *issues.Repair
		for i := range item.Repairs {
			if item.Repairs[i].ID == r.PathValue("repair") {
				repair = &item.Repairs[i]
			}
		}
		if repair == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(string(repo.ID), in.PullRequestID)
		if err != nil || pull.ProposalID != repair.ProposalID || pull.TaskID != repair.TaskID {
			writeJSON(w, 422, map[string]string{"error": "pull_request_not_from_repair_task"})
			return
		}
		updated, linked, err := store.LinkRepairPullRequest(string(repo.ID), item.ID, repair.ID, pull.ID, actor.UserID)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "repair_link_conflict"})
			return
		}
		writeJSON(w, 200, map[string]any{"issue": updated, "repair": linked, "pull_request": pull, "progress_url": "/repositories/" + string(repo.ID) + "/pull-requests/" + pull.ID})
	}
}
