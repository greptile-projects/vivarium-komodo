package main

import (
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
)

type connectedTaskInput struct {
	Title      string                 `json:"title"`
	Outcome    string                 `json:"outcome"`
	SourceKind string                 `json:"source_kind"`
	SourceID   string                 `json:"source_id"`
	DependsOn  []int                  `json:"depends_on"`
	OwnerKind  proposals.AssigneeKind `json:"owner_kind"`
	OwnerID    string                 `json:"owner_id"`
	Mandate    string                 `json:"mandate"`
}

// registerReasoningWorkHTTP is the explicit boundary from analysis into
// accountable delivery. Source records are snapshotted before any proposal is
// written, preventing partial or branch-following work context.
func registerReasoningWorkHTTP(mux *http.ServeMux, canvases investigationStore, impacts impactStore, plans proposalStore, repositories taskSessionRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/connected-work", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title           string               `json:"title"`
			Body            string               `json:"body"`
			InvestigationID string               `json:"investigation_id"`
			AssessmentID    string               `json:"assessment_id"`
			Tasks           []connectedTaskInput `json:"tasks"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		in.Title, in.Body = strings.TrimSpace(in.Title), strings.TrimSpace(in.Body)
		if in.Title == "" || len(in.Tasks) == 0 || len(in.Tasks) > 100 {
			writeJSON(w, 422, map[string]string{"error": "invalid_connected_work"})
			return
		}
		var canvas investigations.Investigation
		var assessment impactassessments.Assessment
		var err error
		if in.InvestigationID != "" {
			canvas, err = canvases.Get(string(repo.ID), in.InvestigationID)
			if err != nil || !investigationParticipant(canvas, actor.UserID) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		if in.AssessmentID != "" {
			assessment, err = impacts.Get(string(repo.ID), in.AssessmentID)
			if err != nil || !impactParticipant(assessment, actor.UserID) {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		contexts := make([]*reasoning.Context, len(in.Tasks))
		for i, task := range in.Tasks {
			if strings.TrimSpace(task.Title) == "" || strings.TrimSpace(task.Outcome) == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_task"})
				return
			}
			for _, dependency := range task.DependsOn {
				if dependency < 0 || dependency >= i {
					writeJSON(w, 422, map[string]string{"error": "invalid_dependency"})
					return
				}
			}
			// Existing plan policy assigns only ready work. A blocked task keeps
			// its reasoning context and can be assigned when dependencies merge.
			if task.OwnerID != "" && len(task.DependsOn) != 0 {
				writeJSON(w, 422, map[string]string{"error": "blocked_task_cannot_be_assigned"})
				return
			}
			if task.OwnerID != "" {
				mandate := strings.TrimSpace(task.Mandate)
				if mandate == "" || len(mandate) > 4096 {
					writeJSON(w, 422, map[string]string{"error": "invalid_assignment"})
					return
				}
				switch task.OwnerKind {
				case proposals.AgentAssignee:
					if !availableTaskAgents[task.OwnerID] {
						writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
						return
					}
				case proposals.HumanAssignee:
					collaborator, _ := repositories.IsCollaborator(repo.ID, task.OwnerID)
					if task.OwnerID != repo.OwnerID && !collaborator {
						writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
						return
					}
				default:
					writeJSON(w, 422, map[string]string{"error": "invalid_assignment"})
					return
				}
			}
			switch task.SourceKind {
			case "investigation_conclusion":
				for _, entry := range canvas.Entries {
					if entry.ID == task.SourceID && entry.Type == "conclusion" && !entry.Stale {
						evidence := make([]reasoning.Evidence, 0, len(entry.Citations))
						for _, c := range entry.Citations {
							evidence = append(evidence, reasoning.Evidence{RepositoryID: c.RepositoryID, CommitID: c.CommitID, Kind: c.Kind, Path: c.Path, ResourceID: c.ObjectID, Label: c.Label})
						}
						contexts[i] = &reasoning.Context{Kind: task.SourceKind, InvestigationID: canvas.ID, ConclusionID: entry.ID, RepositoryID: canvas.RepositoryID, CommitID: entry.CommitID, Claim: entry.Body, Evidence: evidence}
					}
				}
			case "impact_item":
				for _, impact := range assessment.Impacts {
					if impact.ID == task.SourceID {
						evidence := make([]reasoning.Evidence, 0, len(impact.Evidence))
						for _, e := range impact.Evidence {
							evidence = append(evidence, reasoning.Evidence{RepositoryID: e.RepositoryID, CommitID: e.CommitID, Kind: e.Kind, Path: e.Path, Line: e.Line, ResourceID: e.ResourceID, Label: e.Label})
						}
						acks := make([]reasoning.Acknowledgement, 0, len(impact.Acknowledgements))
						for _, a := range impact.Acknowledgements {
							acks = append(acks, reasoning.Acknowledgement{OwnerID: a.OwnerID, State: a.State, Note: a.Note, DecidedByID: a.DecidedByID})
						}
						contexts[i] = &reasoning.Context{Kind: task.SourceKind, AssessmentID: assessment.ID, ImpactID: impact.ID, RepositoryID: assessment.RepositoryID, CommitID: assessment.CommitID, Claim: impact.Summary, Risk: impact.Category, State: impact.State, Rationale: impact.Rationale, Verification: append([]string{}, impact.Verification...), Evidence: evidence, Acknowledgements: acks}
					}
				}
			}
			if contexts[i] == nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_reasoning_source"})
				return
			}
		}
		proposal, err := plans.Create(string(repo.ID), actor.UserID, in.Title, in.Body)
		if err != nil {
			writeProposalError(w, err)
			return
		}
		created := make([]proposals.Task, 0, len(in.Tasks))
		for i, input := range in.Tasks {
			dependencies := make([]string, 0, len(input.DependsOn))
			for _, d := range input.DependsOn {
				dependencies = append(dependencies, created[d].ID)
			}
			task, createErr := plans.CreateTask(string(repo.ID), proposal.ID, actor.UserID, proposals.TaskInput{Title: input.Title, Outcome: input.Outcome, Position: i + 1, Status: proposals.TaskPlanned, DependsOn: dependencies, ReasoningContext: contexts[i]})
			if createErr != nil {
				writeProposalTaskError(w, createErr)
				return
			}
			if input.OwnerID != "" {
				task, createErr = plans.AssignTask(string(repo.ID), proposal.ID, task.ID, actor.UserID, "", proposals.AssignmentInput{Kind: input.OwnerKind, AssigneeID: input.OwnerID, Mandate: input.Mandate, RepositoryID: string(repo.ID), BaseRevision: contexts[i].CommitID})
				if createErr != nil {
					writeProposalTaskError(w, createErr)
					return
				}
			}
			created = append(created, task)
		}
		w.Header().Set("Location", "/repositories/"+string(repo.ID)+"/proposals/"+proposal.ID)
		writeJSON(w, http.StatusCreated, map[string]any{"proposal": proposal, "tasks": created})
	})
}
