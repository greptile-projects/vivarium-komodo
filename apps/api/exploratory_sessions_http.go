package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/exploratorysessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
)

func registerExploratorySessionsHTTP(mux *http.ServeMux, s *exploratorysessions.Store, repos dataFlowRepositories, credentials authStore, extras ...any) {
	var issueStore *issues.Store
	var plans proposalStore
	for _, extra := range extras {
		switch v := extra.(type) {
		case *issues.Store:
			issueStore = v
		case proposalStore:
			plans = v
		}
	}
	base := "/repositories/{repository}/exploratory-sessions"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		permission := auth.RepositoryRead
		if write {
			permission = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, permission, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Catalog(repo)
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in exploratorysessions.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Create(repo, actor, in)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{session}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("session"))
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/timeline", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.EventInput
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.Append(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.EventInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.FindingInput
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.AddFinding(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.FindingInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("PATCH "+base+"/{session}/findings/{finding}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.FindingUpdate
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateFinding(repo, r.PathValue("session"), r.PathValue("finding"), actor, in.ExpectedRevision, in.FindingUpdate)
		if !explorationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.ControlInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Control(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.ControlInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/candidate-revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.CandidateUpdate
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateCandidate(repo, r.PathValue("session"), actor, in.ExpectedRevision, in.CandidateUpdate)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/findings/{finding}/delivery", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		if issueStore == nil || plans == nil {
			writeJSON(w, 500, map[string]string{"error": "delivery_unavailable"})
			return
		}
		var in struct {
			ExpectedRevision      int64    `json:"expected_revision"`
			ExpectedBehavior      string   `json:"expected_behavior"`
			Severity              string   `json:"severity"`
			OwnerKind             string   `json:"owner_kind"`
			OwnerID               string   `json:"owner_id"`
			AcceptanceCriteria    []string `json:"acceptance_criteria"`
			PermittedEventIDs     []string `json:"permitted_event_ids"`
			MinimizedReproduction []string `json:"minimized_reproduction"`
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		session, e := s.Get(repo, r.PathValue("session"))
		if explorationError(w, e) {
			return
		}
		var finding *exploratorysessions.Finding
		for i := range session.Findings {
			if session.Findings[i].ID == r.PathValue("finding") {
				finding = &session.Findings[i]
			}
		}
		if finding == nil {
			writeJSON(w, 404, map[string]string{"error": "exploratory_finding_not_found"})
			return
		}
		issue, e := issueStore.Create(issues.CreateInput{RepositoryID: repo, ReporterID: actor, Title: finding.Title, ExpectedBehavior: in.ExpectedBehavior, ObservedBehavior: finding.Description, Severity: in.Severity, Environment: session.Access.Environment, AffectedCommitID: finding.CandidateRevision, Visibility: "repository", ReproductionSteps: in.MinimizedReproduction})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		proposal, e := plans.Create(repo, actor, "Repair: "+finding.Title, "Governed repair of exploratory finding "+finding.ID+" from session "+session.ID+" at "+finding.CandidateRevision+".")
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ctx := &reasoning.Context{Kind: "exploratory_finding_repair", IssueID: issue.ID, RepositoryID: repo, CommitID: finding.CandidateRevision, Claim: finding.Description, State: "confirmed", Rationale: strings.Join(in.MinimizedReproduction, "; "), Verification: append([]string{}, in.AcceptanceCriteria...)}
		task, e := plans.CreateTask(repo, proposal.ID, actor, proposals.TaskInput{Title: "Repair " + finding.Title, Outcome: in.ExpectedBehavior, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: append(append([]string{}, in.AcceptanceCriteria...), "demonstrate failure on the exact base and pass on the repaired revision", "publish a reusable regression scenario linked to the quality plan"), BaseRevision: finding.CandidateRevision, ReasoningContext: ctx})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, e = plans.AssignTask(repo, proposal.ID, task.ID, actor, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Repair exploratory finding " + finding.ID + " using only the permitted evidence and ordinary pull-request review.", RepositoryID: repo, BaseRevision: finding.CandidateRevision})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		issue, _ = issueStore.AddRelationship(repo, issue.ID, actor, issues.Relationship{Kind: "investigation", ResourceID: session.ID, Revision: finding.CandidateRevision, Note: "exploratory finding " + finding.ID})
		issue, _ = issueStore.AddRelationship(repo, issue.ID, actor, issues.Relationship{Kind: "proposal", ResourceID: proposal.ID, Revision: finding.CandidateRevision, Note: "governed exploratory repair"})
		x, e := s.LinkDelivery(repo, session.ID, finding.ID, actor, in.ExpectedRevision, exploratorysessions.DeliveryLinkInput{IssueID: issue.ID, ProposalID: proposal.ID, TaskID: task.ID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, AcceptanceCriteria: in.AcceptanceCriteria, PermittedEventIDs: in.PermittedEventIDs, MinimizedReproduction: in.MinimizedReproduction})
		if !explorationError(w, e) {
			writeJSON(w, 201, map[string]any{"session": x, "issue": issue, "proposal": proposal, "task": task, "authority": map[string]bool{"git": false, "credential": false, "review": false, "merge": false}})
		}
	})
	mux.HandleFunc("POST "+base+"/{session}/findings/{finding}/verification", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.VerificationInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.VerifyDelivery(repo, r.PathValue("session"), r.PathValue("finding"), actor, in.ExpectedRevision, in.VerificationInput)
		if explorationError(w, e) {
			return
		}
		if issueStore != nil {
			for _, f := range x.Findings {
				if f.ID == r.PathValue("finding") && f.Delivery != nil {
					d := f.Delivery
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "pull_request", ResourceID: d.PullRequestID, Revision: d.RepairRevision, Note: "exploratory repair pull request"})
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "check_run", ResourceID: d.FailingEvidenceID, Revision: d.BaseRevision, Note: "failure demonstrated against exact base"})
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "check_run", ResourceID: d.PassingEvidenceID, Revision: d.RepairRevision, Note: "regression passes on repaired revision"})
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "review", ResourceID: d.ReviewID, Revision: d.RepairRevision, Note: "repair review"})
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "quality_plan", ResourceID: d.QualityPlanID, Revision: fmt.Sprint(d.QualityPlanVersion), Note: "quality agreement"})
					_, _ = issueStore.AddRelationship(repo, d.IssueID, actor, issues.Relationship{Kind: "test_scenario", ResourceID: d.ScenarioID, Revision: fmt.Sprint(d.ScenarioVersion), Note: "maintainable regression coverage"})
				}
			}
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{session}/findings/{finding}/resolution", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64 `json:"expected_revision"`
			exploratorysessions.ResolutionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ResolveWithoutDelivery(repo, r.PathValue("session"), r.PathValue("finding"), actor, in.ExpectedRevision, in.ResolutionInput)
		if !explorationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}
func explorationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, exploratorysessions.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "exploratory_session_not_found"})
	case errors.Is(e, exploratorysessions.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "exploratory_session_changed_or_inactive"})
	case errors.Is(e, exploratorysessions.ErrScope):
		writeJSON(w, 403, map[string]string{"error": "exploratory_session_scope_exceeded"})
	case errors.Is(e, exploratorysessions.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_exploratory_session"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
