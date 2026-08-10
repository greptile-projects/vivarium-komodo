package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type decisionStore interface {
	Create(string, string, string, decisions.Context, decisions.ScopeInput) (decisions.Decision, error)
	Get(string, string) (decisions.Decision, error)
	List(string, string, string) ([]decisions.Decision, error)
	Revise(string, string, string, string, decisions.ScopeInput) (decisions.Decision, error)
	Comment(string, string, string, string) (decisions.Decision, error)
	AddAlternative(string, string, string, string, []decisions.Claim, []decisions.Evidence) (decisions.Decision, error)
	AddClaims(string, string, string, string, []decisions.Claim, []decisions.Evidence) (decisions.Decision, error)
	StartResearch(string, string, string, string) (decisions.Decision, string, error)
	ResearchContext(string) (decisions.Decision, decisions.Alternative, error)
	AddFinding(string, string, string, []decisions.Evidence) (decisions.Decision, error)
	StartExperiment(string, string, string, string, decisions.Experiment) (decisions.Decision, decisions.Experiment, error)
	AddExperimentCheckpoint(string, string, string, string, string, decisions.ExperimentCheckpoint) (decisions.Decision, error)
	AssessExperiment(string, string, string, string, string, string, string, string) (decisions.Decision, error)
	RequestApproval(string, string, string, string, string, string) (decisions.Decision, error)
	RespondApproval(string, string, string, string, string, string) (decisions.Decision, error)
	Publish(string, string, string, string, []string, string, []string, []string, []string, *time.Time, []decisions.Evidence) (decisions.Decision, error)
	AuthorizeException(string, string, string, decisions.Exception) (decisions.Decision, error)
	RevokeException(string, string, string, string) (decisions.Decision, error)
	LinkDelivery(string, string, string, string, string, []string) (decisions.Decision, error)
	RequestRevisit(string, string, string, string, string, string) (decisions.Decision, error)
}

type decisionPlanStore interface {
	Create(string, string, string, string) (proposals.Proposal, error)
	CreateTask(string, string, string, proposals.TaskInput) (proposals.Task, error)
}

func requestDecisionApproval(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Kind    string `json:"kind"`
			ActorID string `json:"actor_id"`
			Policy  string `json:"policy"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := s.RequestApproval(string(repo.ID), r.PathValue("decision"), a.UserID, in.Kind, in.ActorID, in.Policy)
		writeDecision(w, v, e, 201)
	}
}
func respondDecisionApproval(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Response string `json:"response"`
			Note     string `json:"note"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := s.RespondApproval(string(repo.ID), r.PathValue("decision"), r.PathValue("requirement"), a.UserID, in.Response, in.Note)
		writeDecision(w, v, e, 200)
	}
}
func publishDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			SelectedAlternativeID  string               `json:"selected_alternative_id"`
			RejectedAlternativeIDs []string             `json:"rejected_alternative_ids"`
			Rationale              string               `json:"rationale"`
			AcceptedTradeoffs      []string             `json:"accepted_tradeoffs"`
			Dissent                []string             `json:"dissent"`
			Conditions             []string             `json:"conditions"`
			ReviewDate             *time.Time           `json:"review_date"`
			Evidence               []decisions.Evidence `json:"evidence_considered"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Publish(string(repo.ID), r.PathValue("decision"), a.UserID, in.SelectedAlternativeID, in.RejectedAlternativeIDs, in.Rationale, in.AcceptedTradeoffs, in.Dissent, in.Conditions, in.ReviewDate, in.Evidence)
		writeDecision(w, v, e, 201)
	}
}
func authorizeDecisionException(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in decisions.Exception
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.AuthorizeException(string(repo.ID), r.PathValue("decision"), a.UserID, in)
		writeDecision(w, v, e, 201)
	}
}
func revokeDecisionException(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, e := s.RevokeException(string(repo.ID), r.PathValue("decision"), r.PathValue("exception"), a.UserID)
		writeDecision(w, v, e, 200)
	}
}

func registerDecisionsHTTP(mux *http.ServeMux, s decisionStore, repos taskSessionRepositoryStore, c authStore, ws workspaceStore, runner workspaceRunner, plans decisionPlanStore) {
	base := "/repositories/{repository}/decisions"
	mux.HandleFunc("POST "+base, createDecision(s, repos, c))
	mux.HandleFunc("GET "+base, listDecisions(s, repos, c))
	mux.HandleFunc("GET "+base+"/{decision}", getDecision(s, repos, c))
	mux.HandleFunc("PATCH "+base+"/{decision}", reviseDecision(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/comments", commentDecision(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives", addDecisionAlternative(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/approval-requirements", requestDecisionApproval(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/approval-requirements/{requirement}/responses", respondDecisionApproval(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/commitments", publishDecision(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/exceptions", authorizeDecisionException(s, repos, c))
	mux.HandleFunc("DELETE "+base+"/{decision}/exceptions/{exception}", revokeDecisionException(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/delivery", createDecisionDelivery(s, plans, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/revisit-requests", requestDecisionRevisit(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives/{alternative}/claims", addDecisionClaims(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives/{alternative}/agent-runs", startDecisionResearch(s, repos, c))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives/{alternative}/experiments", startDecisionExperiment(s, repos, c, ws, runner))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives/{alternative}/experiments/{experiment}/checkpoints", checkpointDecisionExperiment(s, repos, c, ws, runner))
	mux.HandleFunc("POST "+base+"/{decision}/alternatives/{alternative}/experiments/{experiment}/validity", assessDecisionExperiment(s, repos, c))
	mux.HandleFunc("GET /decision-research-agent/context", decisionResearchContext(s))
	mux.HandleFunc("POST /decision-research-agent/findings", decisionResearchFinding(s))
}

func createDecisionDelivery(s decisionStore, plans decisionPlanStore, repos taskSessionRepositoryStore, c authStore) http.HandlerFunc {
	type taskInput struct {
		Title              string   `json:"title"`
		Outcome            string   `json:"outcome"`
		OwnerKind          string   `json:"owner_kind"`
		OwnerID            string   `json:"owner_id"`
		CompletionCriteria []string `json:"completion_criteria"`
		VerificationPlan   []string `json:"verification_plan"`
		DependsOn          []int    `json:"depends_on"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title        string      `json:"title"`
			Body         string      `json:"body"`
			BaseRevision string      `json:"base_revision"`
			Tasks        []taskInput `json:"tasks"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		d, err := s.Get(string(repo.ID), r.PathValue("decision"))
		if err != nil || d.State != "published" || !containsDecisionActor(d.Scope.ParticipantIDs, actor.UserID) {
			writeJSON(w, 409, map[string]string{"error": "decision_not_accepted"})
			return
		}
		if len(in.BaseRevision) != 40 || len(in.Tasks) == 0 {
			writeJSON(w, 422, map[string]string{"error": "exact_revision_and_tasks_required"})
			return
		}
		opened, err := repos.Open(repo.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, err = opened.ReadCommit(storage.ObjectID(in.BaseRevision)); err != nil {
			writeJSON(w, 422, map[string]string{"error": "exact_commit_required"})
			return
		}
		commitment := d.Commitments[len(d.Commitments)-1]
		required := append(append(append([]string{}, d.Scope.Constraints...), d.Scope.SuccessMeasures...), commitment.Conditions...)
		covered := map[string]bool{}
		allowed := map[string]bool{}
		for _, criterion := range required {
			allowed[criterion] = true
		}
		for _, task := range in.Tasks {
			if task.OwnerKind != "human" && task.OwnerKind != "codex" {
				writeJSON(w, 422, map[string]string{"error": "invalid_task_owner"})
				return
			}
			if len(task.CompletionCriteria) == 0 {
				writeJSON(w, 422, map[string]string{"error": "completion_criteria_required"})
				return
			}
			for _, criterion := range task.CompletionCriteria {
				if !allowed[criterion] {
					writeJSON(w, 422, map[string]string{"error": "criteria_must_derive_from_commitment"})
					return
				}
				covered[criterion] = true
			}
		}
		for _, criterion := range required {
			if !covered[criterion] {
				writeJSON(w, 422, map[string]string{"error": "decision_coverage_incomplete"})
				return
			}
		}
		proposal, err := plans.Create(string(repo.ID), actor.UserID, in.Title, in.Body)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_delivery"})
			return
		}
		tasks := []proposals.Task{}
		evidence := make([]reasoning.Evidence, 0, len(commitment.Evidence))
		for _, item := range commitment.Evidence {
			evidence = append(evidence, reasoning.Evidence{RepositoryID: item.RepositoryID, CommitID: item.Revision, Kind: item.Kind, Path: item.Path, ResourceID: item.ResourceID, Label: item.Summary})
		}
		for index, input := range in.Tasks {
			dependencies := []string{}
			for _, position := range input.DependsOn {
				if position < 1 || position > len(tasks) {
					writeJSON(w, 422, map[string]string{"error": "invalid_task_dependency"})
					return
				}
				dependencies = append(dependencies, tasks[position-1].ID)
			}
			context := &reasoning.Context{Kind: "decision", DecisionID: d.ID, DecisionVersion: commitment.Version, RepositoryID: string(repo.ID), CommitID: in.BaseRevision, Claim: d.Scope.Question, State: "accepted", Rationale: commitment.Rationale, Verification: append([]string{}, input.VerificationPlan...), Evidence: evidence}
			ownerKind := input.OwnerKind
			if ownerKind == "codex" {
				ownerKind = "agent"
			}
			made, createErr := plans.CreateTask(string(repo.ID), proposal.ID, actor.UserID, proposals.TaskInput{Title: input.Title, Outcome: input.Outcome, Position: index + 1, Status: proposals.TaskPlanned, DependsOn: dependencies, OwnerKind: ownerKind, OwnerID: input.OwnerID, CompletionCriteria: input.CompletionCriteria, VerificationPlan: input.VerificationPlan, BaseRevision: in.BaseRevision, ReasoningContext: context})
			if createErr != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_task"})
				return
			}
			tasks = append(tasks, made)
		}
		ids := make([]string, len(tasks))
		for i := range tasks {
			ids[i] = tasks[i].ID
		}
		updated, err := s.LinkDelivery(string(repo.ID), d.ID, actor.UserID, proposal.ID, in.BaseRevision, ids)
		if err != nil {
			writeDecision(w, decisions.Decision{}, err, 201)
			return
		}
		writeJSON(w, 201, map[string]any{"decision": updated, "proposal": proposal, "tasks": tasks, "coverage": map[string]any{"commitment_version": commitment.Version, "required": required, "covered": required, "state": "planned"}})
	}
}

func requestDecisionRevisit(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Kind        string `json:"kind"`
			Summary     string `json:"summary"`
			EvidenceURL string `json:"evidence_url"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, err := s.RequestRevisit(string(repo.ID), r.PathValue("decision"), actor.UserID, in.Kind, in.Summary, in.EvidenceURL)
		writeDecision(w, v, err, 201)
	}
}
func startDecisionExperiment(s decisionStore, repos proposalRepositoryStore, c authStore, ws workspaceStore, runner workspaceRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision               string `json:"revision"`
			CommandName            string `json:"command_name"`
			DependencyDigest       string `json:"dependency_digest"`
			ReproducesExperimentID string `json:"reproduces_experiment_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		d, err := s.Get(string(repo.ID), r.PathValue("decision"))
		if err != nil || !containsDecisionActor(d.Scope.ParticipantIDs, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if len(in.Revision) != 40 {
			writeJSON(w, 422, map[string]string{"error": "exact_revision_required"})
			return
		}
		if strings.TrimSpace(in.DependencyDigest) == "" {
			writeJSON(w, 422, map[string]string{"error": "dependency_digest_required"})
			return
		}
		if in.ReproducesExperimentID != "" {
			validReproduction := false
			for _, alternative := range d.Alternatives {
				if alternative.ID != r.PathValue("alternative") {
					continue
				}
				for _, prior := range alternative.Experiments {
					if prior.ID == in.ReproducesExperimentID && prior.Revision == in.Revision && prior.CommandName == strings.TrimSpace(in.CommandName) {
						validReproduction = true
					}
				}
			}
			if !validReproduction {
				writeJSON(w, 422, map[string]string{"error": "invalid_reproduction"})
				return
			}
		}
		def, digest, err := runner.Definition(string(repo.ID), in.Revision)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_workspace_definition"})
			return
		}
		found := false
		for _, cmd := range def.Commands {
			if cmd.Name == strings.TrimSpace(in.CommandName) {
				found = true
			}
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "undeclared_experiment_command"})
			return
		}
		policy, _ := ws.EffectivePolicy(string(repo.ID), repo.OrganizationID)
		item, err := ws.CreateWithPolicy(string(repo.ID), in.Revision, actor.UserID, workspaces.SourceContext{Type: "decision", ID: d.ID, ParentID: r.PathValue("alternative")}, workspaces.Access{RepositoryID: string(repo.ID), ActorID: actor.UserID, Permission: "experiment:execute"}, def, digest, policy)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		_, x, err := s.StartExperiment(string(repo.ID), d.ID, r.PathValue("alternative"), actor.UserID, decisions.Experiment{WorkspaceID: item.ID, Revision: in.Revision, CommandName: strings.TrimSpace(in.CommandName), DefinitionDigest: digest, DependencyDigest: strings.TrimSpace(in.DependencyDigest), ReproducesExperimentID: strings.TrimSpace(in.ReproducesExperimentID)})
		if err != nil {
			writeDecision(w, decisions.Decision{}, err, 201)
			return
		}
		if named, ok := runner.(interface {
			StartNamed(workspaces.Workspace, string, string)
		}); ok {
			named.StartNamed(item, actor.UserID, strings.TrimSpace(in.CommandName))
		} else {
			runner.Start(item)
		}
		writeJSON(w, 201, map[string]any{"experiment": x, "workspace": item, "publication_authority": false})
	}
}
func checkpointDecisionExperiment(s decisionStore, repos proposalRepositoryStore, c authStore, ws workspaceStore, runner workspaceRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		d, err := s.Get(string(repo.ID), r.PathValue("decision"))
		if err != nil || !containsDecisionActor(d.Scope.ParticipantIDs, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in decisions.ExperimentCheckpoint
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		var x *decisions.Experiment
		for i := range d.Alternatives {
			if d.Alternatives[i].ID == r.PathValue("alternative") {
				for j := range d.Alternatives[i].Experiments {
					if d.Alternatives[i].Experiments[j].ID == r.PathValue("experiment") {
						x = &d.Alternatives[i].Experiments[j]
					}
				}
			}
		}
		if x == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		workspace, err := ws.Get(string(repo.ID), x.WorkspaceID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		cp, err := runner.InspectCheckpoint(workspace, in.WorkspaceCheckpointID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_workspace_checkpoint"})
			return
		}
		validSeq := map[int64]bool{}
		for _, e := range workspace.Activity {
			validSeq[e.Sequence] = true
		}
		for _, seq := range in.LogSequences {
			if !validSeq[seq] {
				writeJSON(w, 422, map[string]string{"error": "invalid_log_sequence"})
				return
			}
		}
		for _, p := range in.ArtifactPaths {
			found := false
			for _, ch := range cp.Changes {
				if ch.Path == p {
					found = true
				}
			}
			if !found {
				writeJSON(w, 422, map[string]string{"error": "invalid_artifact_path"})
				return
			}
		}
		in.ResourceUse = map[string]int64{}
		for _, use := range workspace.Consumption {
			in.ResourceUse[use.Kind+"_"+use.Unit] += use.Quantity
		}
		v, err := s.AddExperimentCheckpoint(string(repo.ID), d.ID, r.PathValue("alternative"), x.ID, actor.UserID, in)
		writeDecision(w, v, err, 201)
	}
}
func assessDecisionExperiment(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision          string `json:"revision"`
			DependencyDigest  string `json:"dependency_digest"`
			EnvironmentDigest string `json:"environment_digest"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := s.AssessExperiment(string(repo.ID), r.PathValue("decision"), r.PathValue("alternative"), r.PathValue("experiment"), actor.UserID, in.Revision, in.DependencyDigest, in.EnvironmentDigest)
		writeDecision(w, v, e, 200)
	}
}
func addDecisionAlternative(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title    string               `json:"title"`
			Claims   []decisions.Claim    `json:"claims"`
			Evidence []decisions.Evidence `json:"evidence"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddAlternative(string(repo.ID), r.PathValue("decision"), a.UserID, in.Title, in.Claims, in.Evidence)
		writeDecision(w, v, e, 201)
	}
}
func addDecisionClaims(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Claims   []decisions.Claim    `json:"claims"`
			Evidence []decisions.Evidence `json:"evidence"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddClaims(string(repo.ID), r.PathValue("decision"), r.PathValue("alternative"), a.UserID, in.Claims, in.Evidence)
		writeDecision(w, v, e, 201)
	}
}
func startDecisionResearch(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, t, e := s.StartResearch(string(repo.ID), r.PathValue("decision"), r.PathValue("alternative"), a.UserID)
		if e != nil {
			writeDecision(w, v, e, 201)
			return
		}
		writeJSON(w, 201, map[string]any{"decision": v, "worker_credential": t, "credential_notice": "shown once; selected alternative context and cited finding publication only; no Git or repository-write authority"})
	}
}
func decisionResearchContext(s decisionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, a, e := s.ResearchContext(bearer(r))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"decision_id": v.ID, "repository_id": v.RepositoryID, "scope": v.Scope, "alternative": a})
	}
}
func decisionResearchFinding(s decisionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Body        string               `json:"body"`
			Uncertainty string               `json:"uncertainty"`
			Evidence    []decisions.Evidence `json:"evidence"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddFinding(bearer(r), in.Body, in.Uncertainty, in.Evidence)
		writeDecision(w, v, e, 201)
	}
}
func createDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title   string            `json:"title"`
			Context decisions.Context `json:"context"`
			decisions.ScopeInput
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if !decisionActors(repo.OwnerID, repo.CollaboratorIDs, in.ParticipantIDs, in.OwnerID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_participants"})
			return
		}
		v, e := s.Create(string(repo.ID), a.UserID, in.Title, in.Context, in.ScopeInput)
		writeDecision(w, v, e, 201)
	}
}
func listDecisions(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID), strings.TrimSpace(r.URL.Query().Get("context_kind")), strings.TrimSpace(r.URL.Query().Get("context_id")))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := items[:0]
		for _, v := range items {
			if a.UserID == "" || containsDecisionActor(v.Scope.ParticipantIDs, a.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func getDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("repository"), r.PathValue("decision"))
		if e != nil || (a.UserID != "" && !containsDecisionActor(v.Scope.ParticipantIDs, a.UserID)) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func reviseDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title string `json:"title"`
			decisions.ScopeInput
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if !decisionActors(repo.OwnerID, repo.CollaboratorIDs, in.ParticipantIDs, in.OwnerID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_participants"})
			return
		}
		v, e := s.Revise(string(repo.ID), r.PathValue("decision"), a.UserID, in.Title, in.ScopeInput)
		writeDecision(w, v, e, 200)
	}
}
func commentDecision(s decisionStore, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Comment(string(repo.ID), r.PathValue("decision"), a.UserID, in.Body)
		writeDecision(w, v, e, 201)
	}
}
func decisionActors(owner string, collabs, participants []string, decisionOwner string) bool {
	if len(participants) == 0 {
		return false
	}
	allowed := map[string]bool{owner: true}
	for _, x := range collabs {
		allowed[x] = true
	}
	for _, x := range participants {
		if !allowed[x] {
			return false
		}
	}
	return allowed[decisionOwner]
}
func containsDecisionActor(v []string, x string) bool {
	for _, a := range v {
		if a == x {
			return true
		}
	}
	return false
}
func writeDecision(w http.ResponseWriter, v decisions.Decision, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
	} else if errors.Is(e, decisions.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_decision"})
	} else if errors.Is(e, decisions.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
