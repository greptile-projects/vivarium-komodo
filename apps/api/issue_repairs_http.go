package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
)

func repairAndVerification(item issues.Issue, repairID, verificationID string) (*issues.Repair, *issues.RepairVerification) {
	for i := range item.Repairs {
		if item.Repairs[i].ID == repairID {
			if verificationID == "" {
				return &item.Repairs[i], nil
			}
			for j := range item.Repairs[i].Verifications {
				if item.Repairs[i].Verifications[j].ID == verificationID {
					return &item.Repairs[i], &item.Repairs[i].Verifications[j]
				}
			}
			return &item.Repairs[i], nil
		}
	}
	return nil, nil
}

func inputDigest(inputs []issues.ReproductionInput) string {
	values := make([]string, 0, len(inputs))
	for _, in := range inputs {
		values = append(values, in.Name+":"+in.SHA256)
	}
	sort.Strings(values)
	sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(sum[:])
}

func verificationEvidence(v issues.RepairVerification, attempt issues.ReproductionAttempt, runs []checkruns.Run, currentRevision string) map[string]any {
	checks := map[string]string{}
	allChecks := true
	for _, name := range v.RequiredChecks {
		state := "missing"
		for _, run := range runs {
			if run.ID != "" && run.Definition.Name == name && run.CommitID == v.Revision {
				state = string(run.State)
				break
			}
		}
		checks[name] = state
		if state != "succeeded" {
			allChecks = false
		}
	}
	reproductionFixed := attempt.ID != "" && attempt.State == "failed" && !attempt.Reproduced && attempt.FailureReason == "reproduction command failed"
	stale := currentRevision != v.Revision || v.InvalidReason != "" || attempt.ID == "" || attempt.Revision != v.Revision || attempt.DefinitionDigest != v.CandidateDefinitionDigest || inputDigest(attempt.Inputs) != v.InputDigest
	state := "running"
	if stale {
		state = "invalid"
	} else if attempt.State == "queued" || attempt.State == "running" {
		state = "running"
	} else if reproductionFixed && allChecks {
		state = "ready_for_reporter"
	} else {
		state = "failed"
	}
	payload := map[string]any{"verification": v, "state": state, "stale": stale, "reproduction_fixed": reproductionFixed, "required_checks_passed": allChecks, "checks": checks, "reproduction": attempt, "acceptance_criteria": v.AcceptanceCriteria}
	raw, _ := json.Marshal([]any{v.Revision, v.CandidateDefinitionDigest, v.InputDigest, v.RequiredChecks, v.AcceptanceCriteria, reproductionFixed, checks})
	sum := sha256.Sum256(raw)
	payload["evidence_digest"] = hex.EncodeToString(sum[:])
	if v.PreviewArtifactPath != "" {
		for _, a := range attempt.Artifacts {
			if a.Path == v.PreviewArtifactPath {
				payload["preview"] = map[string]any{"safe": true, "path": a.Path, "media_type": a.MediaType, "sha256": a.SHA256, "content": a.Content}
			}
		}
	}
	return payload
}

func startIssueRepairVerification(store issueStore, pulls pullRequestStore, repos issueRepositoryStore, credentials authStore, runner *issues.ReproductionRunner, checks readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		repair, _ := repairAndVerification(item, r.PathValue("repair"), "")
		if repair == nil || repair.PullRequestID == "" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(string(repo.ID), repair.PullRequestID)
		if err != nil || pull.Status != "open" {
			writeJSON(w, 409, map[string]string{"error": "repair_pull_request_unavailable"})
			return
		}
		original, err := store.GetReproduction(string(repo.ID), item.ID, repair.ReproductionID)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "retained_reproduction_unavailable"})
			return
		}
		definition, digest, err := runner.Definition(pull.SourceRepositoryID, pull.SourceCommitID)
		invalid := ""
		if err != nil || digest != original.DefinitionDigest {
			invalid = "reproduction definition or environment changed"
		}
		var in struct {
			PreviewArtifactPath string `json:"preview_artifact_path"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		runs, _ := checks.List(string(repo.ID), pull.ID)
		required := append([]string{}, repo.RequiredChecks[pull.TargetBranch]...)
		ids := []string{}
		for _, run := range runs {
			if run.CommitID == pull.SourceCommitID {
				for _, name := range required {
					if run.Definition.Name == name {
						ids = append(ids, run.ID)
					}
				}
			}
		}
		v := issues.RepairVerification{Revision: pull.SourceCommitID, PullRequestID: pull.ID, OriginalDefinitionDigest: original.DefinitionDigest, CandidateDefinitionDigest: digest, InputDigest: inputDigest(original.Inputs), RequiredChecks: required, CheckRunIDs: ids, AcceptanceCriteria: append([]string{}, repair.AcceptanceCriteria...), InvalidReason: invalid, PreviewArtifactPath: strings.TrimSpace(in.PreviewArtifactPath)}
		if invalid == "" {
			attempt, e := store.CreateReproduction(item, pull.SourceCommitID, "", "", actor.UserID, original.ID, definition, digest, original.Command, original.Inputs)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			v.ReproductionAttemptID = attempt.ID
			runner.Start(attempt)
		}
		_, created, err := store.AddRepairVerification(string(repo.ID), item.ID, repair.ID, actor.UserID, v)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, created)
	}
}

func getIssueRepairVerification(store issueStore, pulls pullRequestStore, repos issueRepositoryStore, credentials authStore, runner *issues.ReproductionRunner, checks readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, item, ok := reproductionIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		repair, v := repairAndVerification(item, r.PathValue("repair"), r.PathValue("verification"))
		if repair == nil || v == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(string(repo.ID), v.PullRequestID)
		current := ""
		if err == nil {
			current = pull.SourceCommitID
		}
		attempt, _ := store.GetReproduction(string(repo.ID), item.ID, v.ReproductionAttemptID)
		runs, _ := checks.List(string(repo.ID), v.PullRequestID)
		writeJSON(w, 200, verificationEvidence(*v, attempt, runs, current))
	}
}

func decideIssueRepairVerification(store issueStore, pulls pullRequestStore, repos issueRepositoryStore, credentials authStore, runner *issues.ReproductionRunner, checks readinessCheckStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		repair, v := repairAndVerification(item, r.PathValue("repair"), r.PathValue("verification"))
		if repair == nil || v == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Kind           string `json:"kind"`
			Reason         string `json:"reason"`
			EvidenceDigest string `json:"evidence_digest"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		if in.Kind == "override" {
			if actor.UserID != repo.OwnerID {
				writeJSON(w, 403, map[string]string{"error": "owner_required"})
				return
			}
		} else if actor.UserID != item.ReporterID {
			writeJSON(w, 403, map[string]string{"error": "reporter_required"})
			return
		}
		pull, _ := pulls.Get(string(repo.ID), v.PullRequestID)
		attempt, _ := store.GetReproduction(string(repo.ID), item.ID, v.ReproductionAttemptID)
		runs, _ := checks.List(string(repo.ID), v.PullRequestID)
		evidence := verificationEvidence(*v, attempt, runs, pull.SourceCommitID)
		digest, _ := evidence["evidence_digest"].(string)
		state, _ := evidence["state"].(string)
		if digest == "" || digest != in.EvidenceDigest || state == "invalid" || ((in.Kind == "confirmed" || in.Kind == "override") && state != "ready_for_reporter") {
			writeJSON(w, 409, map[string]string{"error": "verification_evidence_changed"})
			return
		}
		_, updated, err := store.DecideRepairVerification(string(repo.ID), item.ID, repair.ID, v.ID, actor.UserID, in.Kind, in.Reason, v.Revision, digest)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_decision"})
			return
		}
		writeJSON(w, 200, updated)
	}
}

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
