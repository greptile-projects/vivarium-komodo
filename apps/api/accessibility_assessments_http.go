package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitycommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type accessibilityAssessmentSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	runs interface {
		Get(string, string, string) (checkruns.Run, error)
	}
	previews interface {
		GetByID(string) (previews.Preview, error)
	}
	barriers interface {
		Get(string, string) (accessibilitybarriers.Barrier, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
	commitments interface {
		Get(string, string) (accessibilitycommitments.Commitment, error)
	}
	plans    proposalStore
	sessions interface {
		CreateForTask(string, string, string, string, changesessions.TaskContext) (changesessions.Session, error)
	}
	workspaces interface {
		Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
	}
	workspaceRunner interface {
		Definition(string, string) (workspaces.Definition, string, error)
		Start(workspaces.Workspace)
	}
}

func registerAccessibilityAssessmentsHTTP(mux *http.ServeMux, s *accessibilityassessments.Store, repos proposalRepositoryStore, c authStore, src accessibilityAssessmentSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/accessibility-assessments"
	read := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo, pull string, a *accessibilityassessments.Assessment) {
		p, e := src.pulls.Get(repo, pull)
		if e != nil {
			return
		}
		blobs := assessmentBlobs(src.repositories, repo, p.SourceCommitID, a)
		accessibilityassessments.Derive(a, p.SourceCommitID, blobs)
	}
	mux.HandleFunc("GET /repositories/{repository}/accessibility-assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		items, e := s.ListRepository(repo)
		if assessmentError(w, e) {
			return
		}
		for i := range items {
			project(repo, items[i].PullRequestID, &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo, r.PathValue("pull"))
		if assessmentError(w, e) {
			return
		}
		for i := range items {
			project(repo, r.PathValue("pull"), &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		var in accessibilityassessments.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if in.Revision != p.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		if !captureAssessmentLocations(src.repositories, repo, in.Revision, &in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_source_location"})
			return
		}
		a, e := s.Create(repo, p.ID, actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, p.ID, &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("GET "+base+"/{assessment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/automation", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		var in struct {
			RunID string `json:"run_id"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		run, e := src.runs.Get(repo, r.PathValue("pull"), in.RunID)
		if e != nil || run.Definition.Accessibility == nil {
			writeJSON(w, 422, map[string]string{"error": "accessibility_check_run_required"})
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		if run.CommitID != a.Revision {
			writeJSON(w, 409, map[string]string{"error": "check_revision_mismatch"})
			return
		}
		spec := run.Definition.Accessibility
		inputs := make([]accessibilityassessments.Location, 0, len(spec.Inputs))
		rr, openErr := src.repositories.Open(storage.ID(repo))
		if openErr != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for _, path := range spec.Inputs {
			oid, exists := assessmentBlob(rr, storage.ObjectID(a.Revision), path)
			if !exists {
				writeJSON(w, 422, map[string]string{"error": "accessibility_check_input_missing"})
				return
			}
			inputs = append(inputs, accessibilityassessments.Location{Path: path, BlobID: string(oid)})
		}
		a, e = s.AddAutomation(repo, r.PathValue("pull"), a.ID, actor, accessibilityassessments.Automation{RunID: run.ID, Name: run.Definition.Name, ScenarioIDs: spec.ScenarioIDs, Evaluations: spec.Evaluations, Status: string(run.State), RequiresHumanEvaluation: spec.RequiresHumanEvaluation, Inputs: inputs})
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		var in accessibilityassessments.FindingInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !captureFindingLocations(src.repositories, repo, a.Revision, &in) || !validAssessmentCitation(repo, a.Revision, in.Citation, src) {
			writeJSON(w, 422, map[string]string{"error": "permitted_revision_exact_evidence_required"})
			return
		}
		a, e = s.AddFinding(repo, r.PathValue("pull"), a.ID, actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		if _, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true); !ok {
			return
		}
		var in accessibilityassessments.DecisionInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		a, e := s.Decide(repo, r.PathValue("pull"), r.PathValue("assessment"), r.PathValue("finding"), actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/repairs", accessibilityRepairWork(s, repos, c, src))
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/repairs/{repair}/progress", accessibilityRepairProgress(s, repos, c))
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/repairs/{repair}/delivery", accessibilityRepairDelivery(s, repos, c, src))
}

func accessibilityRepairWork(s *accessibilityassessments.Store, repos proposalRepositoryStore, c authStore, src accessibilityAssessmentSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		a, e := s.Get(string(repo.ID), r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		pull, e := src.pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil || pull.SourceCommitID != a.Revision {
			writeJSON(w, 409, map[string]string{"error": "current_finding_revision_required"})
			return
		}
		var in struct {
			Kind               string   `json:"kind"`
			ProposalID         string   `json:"proposal_id"`
			Title              string   `json:"title"`
			OwnerKind          string   `json:"owner_kind"`
			OwnerID            string   `json:"owner_id"`
			CommitmentID       string   `json:"commitment_id"`
			CommitmentVersion  int64    `json:"commitment_version"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
			EvidenceIDs        []string `json:"evidence_ids"`
			ComponentGuidance  []string `json:"component_guidance"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if src.plans == nil || src.commitments == nil || !map[string]bool{"task": true, "change_session": true, "workspace": true}[in.Kind] || !map[string]bool{"human": true, "agent": true}[in.OwnerKind] {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_repair_work"})
			return
		}
		if in.OwnerKind == "agent" && !availableTaskAgents[in.OwnerID] {
			writeJSON(w, 422, map[string]string{"error": "agent_unavailable"})
			return
		}
		if in.OwnerKind == "human" {
			assigned := in.OwnerID == repo.OwnerID
			if !assigned {
				assigned, _ = repos.IsCollaborator(repo.ID, in.OwnerID)
			}
			if !assigned {
				writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
				return
			}
		}
		commitment, e := src.commitments.Get(string(repo.ID), in.CommitmentID)
		if e != nil || in.CommitmentVersion < 1 || in.CommitmentVersion > int64(len(commitment.Versions)) || (a.CommitmentID != "" && (a.CommitmentID != in.CommitmentID || a.CommitmentVersion != in.CommitmentVersion)) {
			writeJSON(w, 422, map[string]string{"error": "exact_accessibility_commitment_required"})
			return
		}
		found := false
		confirmed := false
		evidence := map[string]bool{}
		summary := ""
		for _, f := range a.Findings {
			if f.ID == r.PathValue("finding") {
				found = true
				confirmed = !f.Stale && f.Repair == nil && len(f.Decisions) > 0 && f.Decisions[len(f.Decisions)-1].Outcome == "confirmed"
				summary = f.Summary
				if f.Citation.Kind == "reproduction" {
					b, _ := src.barriers.Get(string(repo.ID), f.Citation.ResourceID)
					for _, x := range b.Evidence {
						evidence[x.ID] = true
					}
					for _, at := range b.Attempts {
						if at.Revision == a.Revision {
							for _, x := range at.Evidence {
								evidence[x.ID] = true
							}
						}
					}
				}
			}
		}
		if !found {
			writeJSON(w, 404, map[string]string{"error": "accessibility_finding_not_found"})
			return
		}
		if !confirmed || len(in.AcceptanceCriteria) == 0 || len(in.AcceptanceCriteria) > 50 || len(in.ComponentGuidance) == 0 || len(in.ComponentGuidance) > 50 {
			writeJSON(w, 422, map[string]string{"error": "confirmed_current_finding_required"})
			return
		}
		for _, values := range [][]string{in.AcceptanceCriteria, in.ComponentGuidance} {
			for _, value := range values {
				if strings.TrimSpace(value) == "" || len(value) > 4000 {
					writeJSON(w, 422, map[string]string{"error": "invalid_repair_context"})
					return
				}
			}
		}
		for _, id := range in.EvidenceIDs {
			if !evidence[id] {
				writeJSON(w, 422, map[string]string{"error": "unpermitted_reproduction_evidence"})
				return
			}
		}
		proposal, e := src.plans.Get(string(repo.ID), in.ProposalID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
			return
		}
		context := &reasoning.Context{Kind: "accessibility_repair", AssessmentID: a.ID, RepositoryID: string(repo.ID), CommitID: a.Revision, Claim: summary, State: "confirmed", Rationale: "Component guidance: " + strings.Join(in.ComponentGuidance, "; ") + ". Preserve the affected user's evidence without treating the implementer as their representative.", Verification: in.AcceptanceCriteria, Evidence: []reasoning.Evidence{{RepositoryID: string(repo.ID), CommitID: a.Revision, Kind: "accessibility_finding", ResourceID: r.PathValue("finding"), Label: summary}, {RepositoryID: string(repo.ID), CommitID: a.Revision, Kind: "accessibility_commitment", ResourceID: in.CommitmentID, Label: commitment.Versions[in.CommitmentVersion-1].Title}}}
		for _, evidenceID := range in.EvidenceIDs {
			context.Evidence = append(context.Evidence, reasoning.Evidence{RepositoryID: string(repo.ID), CommitID: a.Revision, Kind: "accessibility_reproduction_evidence", ResourceID: evidenceID, Label: "Permitted evidence body remains in the barrier store."})
		}
		if strings.TrimSpace(in.Title) == "" {
			in.Title = "Repair confirmed accessibility barrier"
		}
		task, e := src.plans.CreateTask(string(repo.ID), in.ProposalID, actor.UserID, proposals.TaskInput{Title: in.Title, Outcome: strings.Join(in.AcceptanceCriteria, "; "), OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: in.AcceptanceCriteria, BaseRevision: a.Revision, ReasoningContext: context})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "task_creation_failed"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, e = src.plans.AssignTask(string(repo.ID), in.ProposalID, task.ID, actor.UserID, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Implement the confirmed accessibility repair within the retained criteria and ordinary review policy.", RepositoryID: string(repo.ID), BaseRevision: a.Revision})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "task_assignment_failed"})
			return
		}
		repair := accessibilityassessments.Repair{Revision: a.Revision, AcceptanceCriteria: in.AcceptanceCriteria, EvidenceIDs: in.EvidenceIDs, CommitmentID: in.CommitmentID, CommitmentVersion: in.CommitmentVersion, ComponentGuidance: in.ComponentGuidance, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, ProposalID: in.ProposalID, TaskID: task.ID}
		var resource any = task
		if in.Kind == "change_session" {
			if src.sessions == nil {
				writeJSON(w, 422, map[string]string{"error": "change_sessions_unavailable"})
				return
			}
			taskContext := changesessions.TaskContext{ProposalID: proposal.ID, ProposalTitle: proposal.Title, ProposalDescription: proposal.Body, TaskID: task.ID, TaskTitle: task.Title, TaskOutcome: task.Outcome, Mandate: task.Assignment.Mandate, Repository: changesessions.RepositoryContext{ID: string(repo.ID), Name: repo.Name, Description: repo.Description, DefaultBranch: "main", BaseRevision: a.Revision}, ReasoningContext: context}
			x, er := src.sessions.CreateForTask(string(repo.ID), pull.ID, actor.UserID, a.Revision, taskContext)
			if er != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			repair.ChangeSessionID = x.ID
			resource = x
		}
		if in.Kind == "workspace" {
			if src.workspaces == nil || src.workspaceRunner == nil {
				writeJSON(w, 422, map[string]string{"error": "workspaces_unavailable"})
				return
			}
			def, digest, er := src.workspaceRunner.Definition(string(repo.ID), a.Revision)
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_workspace_definition"})
				return
			}
			ctx := workspaces.SourceContext{Type: "accessibility_finding", ID: r.PathValue("finding"), ParentID: a.ID, GuidanceVersion: in.CommitmentVersion, Guidance: in.ComponentGuidance, Evidence: in.EvidenceIDs, AcceptanceCriteria: in.AcceptanceCriteria}
			x, er := src.workspaces.Create(string(repo.ID), a.Revision, actor.UserID, ctx, workspaces.Access{RepositoryID: string(repo.ID), ActorID: actor.UserID, Permission: "repository:write"}, def, digest)
			if er != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			src.workspaceRunner.Start(x)
			repair.WorkspaceID = x.ID
			resource = x
		}
		updated, made, e := s.CreateRepair(string(repo.ID), pull.ID, a.ID, r.PathValue("finding"), actor.UserID, repair)
		if assessmentError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"assessment": updated, "repair": made, "task": task, "resource": resource, "authority_notice": "The finding grants no repository, agent, credential, review, or merge authority."})
	}
}

func accessibilityRepairProgress(s *accessibilityassessments.Store, repos proposalRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Status  string `json:"status"`
			Summary string `json:"summary"`
		}
		if !readJSON(w, r, &in, 72<<10) {
			return
		}
		a, e := s.AddRepairProgress(string(repo.ID), r.PathValue("pull"), r.PathValue("assessment"), r.PathValue("finding"), r.PathValue("repair"), actor.UserID, in.Status, in.Summary)
		if assessmentError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	}
}

func accessibilityRepairDelivery(s *accessibilityassessments.Store, repos proposalRepositoryStore, c authStore, src accessibilityAssessmentSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in accessibilityassessments.RepairDelivery
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		a, e := s.Get(string(repo.ID), r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		var repair *accessibilityassessments.Repair
		for _, f := range a.Findings {
			if f.ID == r.PathValue("finding") {
				repair = f.Repair
			}
		}
		if repair == nil || repair.ID != r.PathValue("repair") {
			writeJSON(w, 404, map[string]string{"error": "accessibility_repair_not_found"})
			return
		}
		pull, e := src.pulls.Get(string(repo.ID), in.PullRequestID)
		if e != nil || pull.ProposalID != repair.ProposalID || pull.TaskID != repair.TaskID || pull.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "pull_request_not_from_accessibility_repair"})
			return
		}
		preview, e := src.previews.GetByID(in.PreviewID)
		if e != nil || preview.RepositoryID != string(repo.ID) || preview.PullRequestID != pull.ID || preview.Revision != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "revision_exact_preview_required"})
			return
		}
		updated, e := s.LinkRepairDelivery(string(repo.ID), r.PathValue("pull"), a.ID, r.PathValue("finding"), repair.ID, actor.UserID, in)
		if assessmentError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"assessment": updated, "pull_request": pull, "preview": preview, "authority_notice": "Delivery remains subject to ordinary repository review and merge permissions."})
	}
}

func captureAssessmentLocations(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, in *accessibilityassessments.Input) bool {
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for i := range in.Scenarios {
		h := sha256.New()
		for j := range in.Scenarios[i].Locations {
			oid, ok := assessmentBlob(r, storage.ObjectID(revision), in.Scenarios[i].Locations[j].Path)
			if !ok {
				return false
			}
			in.Scenarios[i].Locations[j].BlobID = string(oid)
			fmt.Fprintf(h, "%s\x00%s\x00", in.Scenarios[i].Locations[j].Path, oid)
		}
		in.Scenarios[i].Digest = fmt.Sprintf("%x", h.Sum(nil))
	}
	return true
}
func captureFindingLocations(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, in *accessibilityassessments.FindingInput) bool {
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for i := range in.Locations {
		oid, ok := assessmentBlob(r, storage.ObjectID(revision), in.Locations[i].Path)
		if !ok {
			return false
		}
		in.Locations[i].BlobID = string(oid)
	}
	return true
}
func assessmentBlob(r *storage.Repository, revision storage.ObjectID, path string) (storage.ObjectID, bool) {
	commit, e := r.ReadCommit(revision)
	if e != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var walk func(storage.ObjectID, int) (storage.ObjectID, bool)
	walk = func(tree storage.ObjectID, i int) (storage.ObjectID, bool) {
		t, e := r.ReadTree(tree)
		if e != nil {
			return "", false
		}
		for _, x := range t.Entries {
			if x.Name == parts[i] {
				if i == len(parts)-1 {
					return x.ObjectID, x.Type == storage.BlobObject
				}
				if x.Type != storage.TreeObject {
					return "", false
				}
				return walk(x.ObjectID, i+1)
			}
		}
		return "", false
	}
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	return walk(commit.Tree, 0)
}
func assessmentBlobs(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, a *accessibilityassessments.Assessment) map[string]string {
	out := map[string]string{}
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return out
	}
	for _, sc := range a.Scenarios {
		for _, x := range sc.Locations {
			if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
				out[x.Path] = string(oid)
			}
		}
	}
	for _, f := range a.Findings {
		for _, x := range f.Locations {
			if _, exists := out[x.Path]; !exists {
				if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
					out[x.Path] = string(oid)
				}
			}
		}
	}
	for _, automation := range a.Automation {
		for _, x := range automation.Inputs {
			if _, exists := out[x.Path]; !exists {
				if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
					out[x.Path] = string(oid)
				}
			}
		}
	}
	return out
}
func validAssessmentCitation(repo, revision string, c accessibilityassessments.Citation, s accessibilityAssessmentSources) bool {
	if c.Kind == "preview" {
		p, e := s.previews.GetByID(c.ResourceID)
		return e == nil && p.RepositoryID == repo && p.Revision == revision
	}
	if c.Kind == "reproduction" {
		b, e := s.barriers.Get(repo, c.ResourceID)
		if e != nil {
			return false
		}
		for _, a := range b.Attempts {
			if a.Revision == revision {
				return true
			}
		}
	}
	return false
}
func assessmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, accessibilityassessments.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "accessibility_assessment_not_found"})
	} else if errors.Is(e, accessibilityassessments.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_assessment"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
