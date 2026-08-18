package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type designWorkStore interface {
	Create(string, string, string, string) (proposals.Proposal, error)
	CreateTask(string, string, string, proposals.TaskInput) (proposals.Task, error)
}
type designPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
}

func registerDesignProposalsHTTP(mux *http.ServeMux, s *designproposals.Store, repos dataFlowRepositories, credentials authStore, work designWorkStore, pulls designPullStore) {
	base := "/repositories/{repository}/design-proposals"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, scope, write)
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
		x, e := s.List(repo)
		if !designProposalError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in designproposals.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(repo, a, in)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{proposal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("proposal"))
		if !designProposalError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			designproposals.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(repo, r.PathValue("proposal"), a, in.ExpectedVersion, in.Input)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion     int64    `json:"expected_version"`
			SubjectID           string   `json:"subject_id"`
			Kind                string   `json:"kind"`
			Role                string   `json:"role"`
			GroundedEvidenceIDs []string `json:"grounded_evidence_ids"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Invite(repo, r.PathValue("proposal"), a, in.SubjectID, in.Kind, in.Role, in.GroundedEvidenceIDs, in.ExpectedVersion)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in designproposals.ArtifactInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddArtifact(repo, r.PathValue("proposal"), a, in)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/artifacts/{artifact}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			designproposals.ArtifactInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ReviseArtifact(repo, r.PathValue("proposal"), r.PathValue("artifact"), a, in.ExpectedVersion, in.ArtifactInput)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			SubjectKind     string   `json:"subject_kind"`
			SubjectID       string   `json:"subject_id"`
			SubjectRevision int64    `json:"subject_revision"`
			Body            string   `json:"body"`
			Stance          string   `json:"stance"`
			EvidenceIDs     []string `json:"evidence_ids"`
			Uncertainty     string   `json:"uncertainty"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Comment(repo, r.PathValue("proposal"), a, in.SubjectKind, in.SubjectID, in.Body, in.Stance, in.Uncertainty, in.SubjectRevision, in.EvidenceIDs)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			OwnerID         string `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RequestAcknowledgement(repo, r.PathValue("proposal"), a, in.OwnerID, in.ExpectedVersion)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/acknowledgements/{ack}/response", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Status    string `json:"status"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Respond(repo, r.PathValue("proposal"), r.PathValue("ack"), a, in.Status, in.Rationale)
		if !designProposalError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/implementations", func(w http.ResponseWriter, r *http.Request) {
		repoID, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		if work == nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		var in struct {
			BaseRevision string `json:"base_revision"`
			Title        string `json:"title"`
			Tasks        []struct {
				Title              string   `json:"title"`
				Outcome            string   `json:"outcome"`
				OwnerKind          string   `json:"owner_kind"`
				OwnerID            string   `json:"owner_id"`
				DependsOn          []int    `json:"depends_on"`
				AcceptanceCriteria []string `json:"acceptance_criteria"`
				ChangedPaths       []string `json:"changed_paths"`
				RenderedSurfaces   []string `json:"rendered_surfaces"`
			} `json:"tasks"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		in.BaseRevision = strings.TrimSpace(in.BaseRevision)
		opener, ok := any(repos).(interface {
			Open(storage.ID) (*storage.Repository, error)
		})
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		repository, e := opener.Open(storage.ID(repoID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if _, e = repository.ReadCommit(storage.ObjectID(in.BaseRevision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_base_revision"})
			return
		}
		design, e := s.Get(repoID, r.PathValue("proposal"))
		if designProposalError(w, e) {
			return
		}
		if len(in.Tasks) == 0 {
			writeJSON(w, 422, map[string]string{"error": "invalid_design_proposal"})
			return
		}
		accepted := false
		for _, ack := range design.Acknowledgements {
			if ack.ProposalRevision == design.CurrentRevision {
				accepted = true
				if ack.Status != "acknowledged" {
					writeJSON(w, 422, map[string]string{"error": "design_not_accepted"})
					return
				}
			}
		}
		if !accepted {
			writeJSON(w, 422, map[string]string{"error": "design_not_accepted"})
			return
		}
		for i, task := range in.Tasks {
			if strings.TrimSpace(task.Title) == "" || strings.TrimSpace(task.Outcome) == "" || (task.OwnerKind != "human" && task.OwnerKind != "agent") || strings.TrimSpace(task.OwnerID) == "" || len(task.AcceptanceCriteria) == 0 || len(task.ChangedPaths) == 0 || len(task.RenderedSurfaces) == 0 {
				writeJSON(w, 422, map[string]string{"error": "invalid_implementation_task"})
				return
			}
			for _, position := range task.DependsOn {
				if position < 1 || position > i {
					writeJSON(w, 422, map[string]string{"error": "invalid_implementation_task"})
					return
				}
			}
		}
		rev := design.Revisions[design.CurrentRevision-1]
		title := strings.TrimSpace(in.Title)
		if title == "" {
			title = "Implement " + rev.Title
		}
		body := "Implements accepted design proposal `" + design.ID + "` revision " + int64String(design.CurrentRevision) + " at base `" + in.BaseRevision + "`."
		workProposal, e := work.Create(repoID, actor, title, body)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_implementation"})
			return
		}
		made := []proposals.Task{}
		links := []designproposals.ImplementationTask{}
		for i, t := range in.Tasks {
			deps := []string{}
			valid := true
			for _, pos := range t.DependsOn {
				if pos < 1 || pos > len(made) {
					valid = false
					break
				}
				deps = append(deps, made[pos-1].ID)
			}
			if !valid || len(t.AcceptanceCriteria) == 0 || len(t.ChangedPaths) == 0 || len(t.RenderedSurfaces) == 0 {
				writeJSON(w, 422, map[string]string{"error": "invalid_implementation_task"})
				return
			}
			ctx := designReasoning(repoID, in.BaseRevision, design)
			task, e := work.CreateTask(repoID, workProposal.ID, actor, proposals.TaskInput{Title: t.Title, Outcome: t.Outcome, OwnerKind: t.OwnerKind, OwnerID: t.OwnerID, CompletionCriteria: t.AcceptanceCriteria, VerificationPlan: append(append([]string{}, t.AcceptanceCriteria...), "Map changed paths and rendered surfaces to every design requirement; request approval for each deviation"), BaseRevision: in.BaseRevision, Position: i + 1, Status: proposals.TaskPlanned, DependsOn: deps, ReasoningContext: ctx})
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_implementation_task"})
				return
			}
			made = append(made, task)
			links = append(links, designproposals.ImplementationTask{TaskID: task.ID, Title: task.Title, OwnerKind: t.OwnerKind, OwnerID: t.OwnerID, Position: i + 1, AcceptanceCriteria: t.AcceptanceCriteria, ExpectedPaths: t.ChangedPaths, RenderedSurfaces: t.RenderedSurfaces})
		}
		design, e = s.AddImplementation(repoID, design.ID, actor, in.BaseRevision, workProposal.ID, links)
		if designProposalError(w, e) {
			return
		}
		writeJSON(w, 201, map[string]any{"design_proposal": design, "work_proposal": workProposal, "tasks": made})
	})
	mux.HandleFunc("POST "+base+"/{proposal}/implementations/{implementation}/mappings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in designproposals.Mapping
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		in.ImplementationID = r.PathValue("implementation")
		if pulls == nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		pull, e := pulls.Get(repo, in.PullRequestID)
		if e != nil || pull.TaskID != in.TaskID || pull.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "invalid_implementation_mapping"})
			return
		}
		x, e := s.AddMapping(repo, r.PathValue("proposal"), actor, in)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/implementation-mappings/{mapping}/deviations/{requirement}/approval", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		inspected, e := repos.Inspect(storage.ID(repo))
		if e != nil || inspected.OwnerID != actor {
			writeJSON(w, 403, map[string]string{"error": "owner_required"})
			return
		}
		x, e := s.ApproveDeviation(repo, r.PathValue("proposal"), r.PathValue("mapping"), actor, r.PathValue("requirement"))
		if !designProposalError(w, e) {
			writeJSON(w, 200, x)
		}
	})
}

func int64String(n int64) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return int64String(n/10) + string(rune('0'+n%10))
}
func designReasoning(repo, commit string, p designproposals.Proposal) *reasoning.Context {
	imp := reasoning.DesignContract{ProposalID: p.ID, ProposalRevision: p.CurrentRevision, ArtifactVersions: map[string]int64{}}
	for _, a := range p.Artifacts {
		imp.ArtifactVersions[a.ID] = a.CurrentVersion
		ar := a.Revisions[a.CurrentVersion-1]
		if ar.ProposalRevision == p.CurrentRevision {
			for _, x := range ar.Assets {
				imp.Assets = append(imp.Assets, reasoning.DesignAsset{ID: x.ID, Name: x.Name, Source: x.Source, AuthorID: x.AuthorID, License: x.License, Transformations: x.Transformations})
			}
		}
	}
	r := p.Revisions[p.CurrentRevision-1]
	n := 0
	add := func(k, s, e string) {
		n++
		imp.Requirements = append(imp.Requirements, reasoning.DesignRequirement{ID: k + "-" + int64String(int64(n)), Kind: k, Subject: s, Expected: e})
	}
	for _, x := range r.Journeys {
		add("journey", x.Name, strings.Join(x.Steps, " -> ")+" => "+x.Outcome)
	}
	for _, x := range r.States {
		add("state", x.Name, x.Trigger+" => "+x.Behavior+"; "+x.Content)
	}
	for _, x := range r.Constraints {
		add("constraint", x.Kind, x.Requirement)
	}
	for _, x := range r.ComponentContracts {
		add("component", x.Name, x.Contract)
	}
	for _, x := range r.Breakpoints {
		add("breakpoint", x.Name, int64String(int64(x.MinimumWidth))+".."+int64String(int64(x.MaximumWidth))+": "+x.Behavior)
	}
	for _, x := range r.SuccessMeasures {
		add("acceptance", x.Name, x.Target)
	}
	for _, a := range p.Artifacts {
		ar := a.Revisions[a.CurrentVersion-1]
		if ar.ProposalRevision == p.CurrentRevision {
			for _, f := range ar.Frames {
				add("prototype", ar.Title+"/"+f.Name, f.Description+" ["+f.Format+"] "+f.Body)
			}
			for _, x := range ar.Interactions {
				add("interaction", ar.Title, x.Trigger+" => "+x.Action+" => "+x.Result)
			}
		}
	}
	acks := []reasoning.Acknowledgement{}
	for _, a := range p.Acknowledgements {
		if a.ProposalRevision == p.CurrentRevision {
			acks = append(acks, reasoning.Acknowledgement{OwnerID: a.OwnerID, State: a.Status, Note: a.Rationale, DecidedByID: a.OwnerID})
		}
	}
	return &reasoning.Context{Kind: "design_implementation", RepositoryID: repo, CommitID: commit, Claim: r.UserGoal, Rationale: r.ChangeReason, Verification: r.Content, Acknowledgements: acks, Design: &imp}
}

func designProposalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, designproposals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "design_proposal_not_found"})
	case errors.Is(e, designproposals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "design_proposal_changed"})
	case errors.Is(e, designproposals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_design_proposal"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
