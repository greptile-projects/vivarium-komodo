package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type previewPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
}
type previewRunner interface {
	Definition(string, string) (previews.Definition, string, error)
	Start(previews.Preview)
}
type previewAudienceSources interface {
	Issue(string, string) (issues.Issue, error)
	Decision(string, string) (decisions.Decision, error)
	Proposal(string, string) (proposals.Proposal, error)
	ProposalComments(string, string) ([]proposals.Comment, error)
}
type previewRepairSessions interface {
	Create(string, string, string, string) (changesessions.Session, error)
}
type previewRepairWorkspaces interface {
	Create(string, string, string, workspaces.SourceContext, workspaces.Access, workspaces.Definition, string) (workspaces.Workspace, error)
}
type previewRepairWorkspaceRunner interface {
	Definition(string, string) (workspaces.Definition, string, error)
	Start(workspaces.Workspace)
}
type previewRepairStores struct {
	plans           proposalStore
	sessions        previewRepairSessions
	workspaces      previewRepairWorkspaces
	workspaceRunner previewRepairWorkspaceRunner
}

type previewSources struct {
	issues    issueStore
	decisions decisionStore
	proposals proposalStore
}

func (s previewSources) Issue(r, id string) (issues.Issue, error) { return s.issues.Get(r, id) }
func (s previewSources) Decision(r, id string) (decisions.Decision, error) {
	return s.decisions.Get(r, id)
}
func (s previewSources) Proposal(r, id string) (proposals.Proposal, error) {
	return s.proposals.Get(r, id)
}
func (s previewSources) ProposalComments(r, id string) ([]proposals.Comment, error) {
	return s.proposals.ListComments(r, id)
}

func registerPreviewsHTTP(mux *http.ServeMux, store *previews.Store, runner previewRunner, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore, extras ...any) {
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews", createPreview(store, runner, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews", listPreviews(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}", getPreview(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/audience", previewAudience(store, pulls, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings", listPreviewFindings(store, pulls, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings", createPreviewFinding(store, pulls, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings/{finding}/comments", commentPreviewFinding(store, pulls, repositories, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings/{finding}", updatePreviewFinding(store, pulls, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings/{finding}/evidence/{evidence}", getPreviewEvidence(store, pulls, repositories, credentials))
	var sources previewAudienceSources
	var repairs previewRepairStores
	for _, extra := range extras {
		switch v := extra.(type) {
		case previewAudienceSources:
			sources = v
		case previewRepairStores:
			repairs = v
		}
	}
	if repairs.plans != nil {
		mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings/{finding}/work", createPreviewFindingWork(store, pulls, repositories, credentials, repairs))
		mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/findings/{finding}/repairs", publishPreviewFindingRepair(store, runner, pulls, repositories, credentials))
	}
	mux.HandleFunc("/repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/proxy/{path...}", proxyPreview(store, pulls, repositories, credentials))
	if sources != nil {
		mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/invitations", invitePreview(store, pulls, repositories, credentials, sources))
		mux.HandleFunc("DELETE /repositories/{repository}/pull-requests/{pull_request}/previews/{preview}/invitations/{invitation}", revokePreviewInvitation(store, pulls, repositories, credentials))
	}
}

func createPreviewFindingWork(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore, repair previewRepairStores) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, _, participant, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		if !participant {
			writeJSON(w, 403, map[string]string{"error": "repository_participant_required"})
			return
		}
		var in struct {
			Kind               string   `json:"kind"`
			ProposalID         string   `json:"proposal_id"`
			Title              string   `json:"title"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
			EvidenceIDs        []string `json:"evidence_ids"`
			OwnerKind          string   `json:"owner_kind"`
			OwnerID            string   `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		in.Kind, in.ProposalID, in.Title, in.OwnerKind, in.OwnerID = strings.TrimSpace(in.Kind), strings.TrimSpace(in.ProposalID), strings.TrimSpace(in.Title), strings.TrimSpace(in.OwnerKind), strings.TrimSpace(in.OwnerID)
		if in.Kind != "task" && in.Kind != "change_session" && in.Kind != "workspace" {
			writeJSON(w, 422, map[string]string{"error": "invalid_work_kind"})
			return
		}
		if in.ProposalID == "" {
			if pull.SourceRepositoryID == pull.RepositoryID {
				in.ProposalID = pull.ProposalID
			}
		}
		if in.ProposalID == "" || (in.OwnerKind != "human" && in.OwnerKind != "agent") || in.OwnerID == "" || len(in.AcceptanceCriteria) == 0 || len(in.AcceptanceCriteria) > 20 || len(in.EvidenceIDs) > 20 {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_work"})
			return
		}
		source, err := repos.Inspect(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		actorCanWrite := actor == source.OwnerID
		if !actorCanWrite {
			actorCanWrite, _ = repos.IsCollaborator(source.ID, actor)
		}
		if !actorCanWrite {
			writeJSON(w, 403, map[string]string{"error": "source_repository_write_required"})
			return
		}
		if in.OwnerKind == "human" {
			ownerCanWrite := in.OwnerID == source.OwnerID
			if !ownerCanWrite {
				ownerCanWrite, _ = repos.IsCollaborator(source.ID, in.OwnerID)
			}
			if !ownerCanWrite {
				writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
				return
			}
		} else if !availableTaskAgents[in.OwnerID] {
			writeJSON(w, 422, map[string]string{"error": "agent_unavailable"})
			return
		}
		if _, err := repair.plans.Get(pull.SourceRepositoryID, in.ProposalID); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_proposal"})
			return
		}
		preview, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		found, evidence := false, map[string]bool{}
		for _, finding := range preview.Findings {
			if finding.ID == r.PathValue("finding") {
				found = true
				if finding.Work != nil {
					writeJSON(w, 409, map[string]string{"error": "finding_work_conflict"})
					return
				}
				for _, item := range finding.Evidence {
					evidence[item.ID] = true
				}
			}
		}
		if !found {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		for _, id := range in.EvidenceIDs {
			if !evidence[id] {
				writeJSON(w, 422, map[string]string{"error": "invalid_evidence"})
				return
			}
		}
		for i := range in.AcceptanceCriteria {
			in.AcceptanceCriteria[i] = strings.TrimSpace(in.AcceptanceCriteria[i])
			if in.AcceptanceCriteria[i] == "" || len(in.AcceptanceCriteria[i]) > 1000 {
				writeJSON(w, 422, map[string]string{"error": "invalid_acceptance_criteria"})
				return
			}
		}
		if in.Title == "" {
			item, _ := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
			for _, f := range item.Findings {
				if f.ID == r.PathValue("finding") {
					in.Title = "Repair " + f.Title
				}
			}
		}
		task, err := repair.plans.CreateTask(pull.SourceRepositoryID, in.ProposalID, actor, proposals.TaskInput{Title: in.Title, Outcome: strings.Join(in.AcceptanceCriteria, "; "), OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: in.AcceptanceCriteria, BaseRevision: pull.SourceCommitID})
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "task_creation_failed"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		task, err = repair.plans.AssignTask(pull.SourceRepositoryID, in.ProposalID, task.ID, actor, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: task.Outcome, RepositoryID: pull.SourceRepositoryID, BaseRevision: pull.SourceCommitID})
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "task_assignment_failed"})
			return
		}
		work := previews.FindingWork{Kind: in.Kind, ProposalID: in.ProposalID, TaskID: task.ID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, AcceptanceCriteria: in.AcceptanceCriteria, EvidenceIDs: in.EvidenceIDs}
		var resource any = task
		switch in.Kind {
		case "task":
		case "change_session":
			if repair.sessions == nil {
				writeJSON(w, 422, map[string]string{"error": "change_sessions_unavailable"})
				return
			}
			session, e := repair.sessions.Create(pull.SourceRepositoryID, pull.ID, actor, pull.SourceCommitID)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			work.ChangeSessionID, resource = session.ID, session
		case "workspace":
			if repair.workspaces == nil || repair.workspaceRunner == nil {
				writeJSON(w, 422, map[string]string{"error": "workspaces_unavailable"})
				return
			}
			def, digest, e := repair.workspaceRunner.Definition(pull.SourceRepositoryID, pull.SourceCommitID)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_workspace_definition"})
				return
			}
			ctx := workspaces.SourceContext{Type: "preview_finding", ID: r.PathValue("finding"), ParentID: r.PathValue("preview"), Evidence: in.EvidenceIDs, AcceptanceCriteria: in.AcceptanceCriteria}
			ws, e := repair.workspaces.Create(pull.SourceRepositoryID, pull.SourceCommitID, actor, ctx, workspaces.Access{RepositoryID: pull.SourceRepositoryID, ActorID: actor, Permission: "repository:write"}, def, digest)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			repair.workspaceRunner.Start(ws)
			work.WorkspaceID, resource = ws.ID, ws
		}
		finding, err := store.LinkFindingWork(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("finding"), actor, work)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "finding_work_conflict"})
			return
		}
		writeJSON(w, 201, map[string]any{"finding": finding, "task": task, "resource": resource, "authority_notice": "No credential or authority was granted; starting and publishing use the linked resource's ordinary permission boundary."})
	}
}

func publishPreviewFindingRepair(store *previews.Store, runner previewRunner, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if pull.Status != pullrequests.Open {
			writeJSON(w, 409, map[string]string{"error": "pull_request_not_open"})
			return
		}
		var in struct {
			Revision        string            `json:"revision"`
			CommitIDs       []string          `json:"commit_ids"`
			Commands        []string          `json:"commands"`
			Checks          []string          `json:"checks"`
			AuthorIDs       []string          `json:"author_ids"`
			ChangeSessionID string            `json:"change_session_id"`
			WorkspaceID     string            `json:"workspace_id"`
			Configuration   map[string]string `json:"configuration"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		if in.Revision != pull.SourceCommitID || len(in.CommitIDs) == 0 || len(in.CommitIDs) > 100 || len(in.AuthorIDs) == 0 || len(in.AuthorIDs) > 100 || len(in.Commands) > 50 || len(in.Checks) > 50 {
			writeJSON(w, 409, map[string]string{"error": "repair_revision_not_current"})
			return
		}
		origin, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		workLinked := false
		for _, finding := range origin.Findings {
			if finding.ID == r.PathValue("finding") {
				workLinked = finding.Work != nil
			}
		}
		if !workLinked {
			writeJSON(w, 422, map[string]string{"error": "finding_work_required"})
			return
		}
		opened, err := repos.Open(storage.ID(pull.SourceRepositoryID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		seenRevision := false
		for _, id := range in.CommitIDs {
			if id == in.Revision {
				seenRevision = true
			}
			if len(id) != 40 {
				writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
				return
			}
			if _, e := opened.ReadCommit(storage.ObjectID(id)); e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
				return
			}
		}
		if !seenRevision {
			writeJSON(w, 422, map[string]string{"error": "repair_revision_commit_required"})
			return
		}
		for _, values := range [][]string{in.Commands, in.Checks, in.AuthorIDs} {
			for _, value := range values {
				if strings.TrimSpace(value) == "" || len(value) > 2000 {
					writeJSON(w, 422, map[string]string{"error": "invalid_repair_evidence"})
					return
				}
			}
		}
		definition, digest, err := runner.Definition(pull.SourceRepositoryID, pull.SourceCommitID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_preview_definition"})
			return
		}
		if in.Configuration == nil {
			in.Configuration = map[string]string{}
		}
		allowed := map[string]bool{}
		for _, key := range definition.Configuration {
			allowed[key] = true
		}
		for key, value := range in.Configuration {
			if !allowed[key] || len(value) > 2000 {
				writeJSON(w, 422, map[string]string{"error": "invalid_preview_configuration"})
				return
			}
		}
		for _, key := range definition.Configuration {
			if _, ok := in.Configuration[key]; !ok {
				writeJSON(w, 422, map[string]string{"error": "missing_preview_configuration", "name": key})
				return
			}
		}
		keys := make([]string, 0, len(in.Configuration))
		for key := range in.Configuration {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		hash := sha256.New()
		for _, key := range keys {
			_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", key, in.Configuration[key])
		}
		next, err := store.Create(previews.Preview{RepositoryID: pull.RepositoryID, SourceRepositoryID: pull.SourceRepositoryID, PullRequestID: pull.ID, Revision: pull.SourceCommitID, CreatorID: actor, Definition: definition, Configuration: in.Configuration, Attestation: previews.Attestation{CommitID: pull.SourceCommitID, DefinitionDigest: digest, ConfigurationDigest: fmt.Sprintf("%x", hash.Sum(nil))}})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		finding, err := store.RecordRepair(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("finding"), actor, previews.RepairPublication{Revision: in.Revision, CommitIDs: in.CommitIDs, Commands: in.Commands, Checks: in.Checks, AuthorIDs: in.AuthorIDs, ChangeSessionID: in.ChangeSessionID, WorkspaceID: in.WorkspaceID, PreviewID: next.ID})
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_publication"})
			return
		}
		runner.Start(next)
		writeJSON(w, 201, map[string]any{"finding": finding, "preview": next, "authority_notice": "The pull request revision was already current; this action recorded provenance and launched ordinary preview verification."})
	}
}

func previewFindingAccess(w http.ResponseWriter, r *http.Request, store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) (pullrequests.PullRequest, string, previews.Invitation, bool, bool) {
	grant, ok := authenticateRequest(w, r, c, auth.ProfileRead)
	if !ok {
		return pullrequests.PullRequest{}, "", previews.Invitation{}, false, false
	}
	pull, e := pulls.Get(r.PathValue("repository"), r.PathValue("pull_request"))
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return pull, "", previews.Invitation{}, false, false
	}
	repo, e := repos.Inspect(storage.ID(pull.RepositoryID))
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return pull, "", previews.Invitation{}, false, false
	}
	participant := grant.UserID == repo.OwnerID
	if !participant {
		participant, _ = repos.IsCollaborator(repo.ID, grant.UserID)
	}
	if participant {
		return pull, grant.UserID, previews.Invitation{}, true, true
	}
	_, inv, e := store.Authorize(pull.RepositoryID, pull.ID, r.PathValue("preview"), grant.UserID)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return pull, "", previews.Invitation{}, false, false
	}
	return pull, grant.UserID, inv, false, true
}
func listPreviewFindings(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, _, _, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		p, e := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": p.Findings, "revision": p.Revision})
	}
}
func createPreviewFinding(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, inv, participant, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		if !participant && inv.Role != "feedback" && !store.HasRole(pull.RepositoryID, pull.ID, r.PathValue("preview"), actor, "feedback") {
			writeJSON(w, 403, map[string]string{"error": "feedback_role_required"})
			return
		}
		var in previews.Finding
		if !readJSON(w, r, &in, 8<<20) {
			return
		}
		p, e := store.AddFinding(pull.RepositoryID, pull.ID, r.PathValue("preview"), actor, in)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding"})
			return
		}
		writeJSON(w, 201, p.Findings[len(p.Findings)-1])
	}
}
func commentPreviewFinding(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, _, _, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		p, e := store.CommentFinding(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("finding"), actor, in.Body)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_comment"})
			return
		}
		writeJSON(w, 201, p)
	}
}
func updatePreviewFinding(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, _, participant, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		if !participant {
			writeJSON(w, 403, map[string]string{"error": "repository_participant_required"})
			return
		}
		var in struct {
			Classification string   `json:"classification"`
			Status         string   `json:"status"`
			DuplicateOf    string   `json:"duplicate_of"`
			Related        []string `json:"related_finding_ids"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		p, e := store.UpdateFinding(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("finding"), actor, in.Classification, in.Status, in.DuplicateOf, in.Related)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_update"})
			return
		}
		writeJSON(w, 200, p)
	}
}
func getPreviewEvidence(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, _, _, ok := previewFindingAccess(w, r, store, pulls, repos, c)
		if !ok {
			return
		}
		item, e := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		a, b, e := store.ReadEvidence(pull.RepositoryID, pull.ID, item.ID, r.PathValue("finding"), r.PathValue("evidence"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		w.Header().Set("Content-Type", a.MediaType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", a.Name))
		w.Header().Set("X-Komodo-Evidence-Revision", item.Revision)
		w.WriteHeader(200)
		_, _ = w.Write(b)
	}
}

func previewAudience(store *previews.Store, pulls previewPullStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, c, auth.ProfileRead)
		if !ok {
			return
		}
		pull, e := pulls.Get(r.PathValue("repository"), r.PathValue("pull_request"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item, in, e := store.Authorize(pull.RepositoryID, pull.ID, r.PathValue("preview"), grant.UserID)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, map[string]any{"preview_id": item.ID, "revision": item.Revision, "state": item.State, "stale": item.Revision != pull.SourceCommitID, "role": in.Role, "expires_at": in.ExpiresAt, "audience": item.Definition.Audience, "effective_access": map[string]bool{"preview": true, "repository": false, "git": false, "workspace": false, "deployment": false, "production": false}, "configuration_values_exposed": false})
	}
}

func invitePreview(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore, s previewAudienceSources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID     string    `json:"user_id"`
			Role       string    `json:"role"`
			SourceKind string    `json:"source_kind"`
			SourceID   string    `json:"source_id"`
			ExpiresAt  time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		valid := in.SourceKind == "user" && in.SourceID == ""
		switch in.SourceKind {
		case "issue":
			if v, e := s.Issue(pull.RepositoryID, in.SourceID); e == nil {
				valid = v.ReporterID == in.UserID
				for _, x := range v.Triage.AssigneeIDs {
					valid = valid || x == in.UserID
				}
				for _, x := range v.Comments {
					valid = valid || x.AuthorID == in.UserID
				}
			}
		case "decision":
			if v, e := s.Decision(pull.RepositoryID, in.SourceID); e == nil {
				for _, x := range v.Scope.ParticipantIDs {
					valid = valid || x == in.UserID
				}
			}
		case "proposal":
			if v, e := s.Proposal(pull.RepositoryID, in.SourceID); e == nil {
				valid = v.AuthorID == in.UserID
				if xs, e := s.ProposalComments(pull.RepositoryID, in.SourceID); e == nil {
					for _, x := range xs {
						valid = valid || x.AuthorID == in.UserID
					}
				}
			}
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "user_not_source_participant"})
			return
		}
		v, e := store.Invite(pull.RepositoryID, pull.ID, r.PathValue("preview"), actor, previews.Invitation{UserID: in.UserID, Role: in.Role, SourceKind: in.SourceKind, SourceID: in.SourceID, ExpiresAt: in.ExpiresAt})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_invitation"})
			return
		}
		writeJSON(w, 201, v)
	}
}
func revokePreviewInvitation(store *previews.Store, pulls previewPullStore, repos pullRequestRepositoryStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, e := store.Revoke(pull.RepositoryID, pull.ID, r.PathValue("preview"), r.PathValue("invitation"), actor)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}

func previewContext(w http.ResponseWriter, r *http.Request, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore, scope auth.Scope, requireActor bool) (pullrequests.PullRequest, string, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, requireActor)
	if !ok {
		return pullrequests.PullRequest{}, "", false
	}
	pull, err := pulls.Get(string(repo.ID), r.PathValue("pull_request"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return pull, "", false
	}
	return pull, actor.UserID, true
}

func createPreview(store *previews.Store, runner previewRunner, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, actor, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if pull.Status != pullrequests.Open {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_open"})
			return
		}
		var input struct {
			Configuration map[string]string `json:"configuration"`
		}
		if !readJSON(w, r, &input, 64<<10) {
			return
		}
		definition, definitionDigest, err := runner.Definition(pull.SourceRepositoryID, pull.SourceCommitID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_preview_definition", "message": err.Error()})
			return
		}
		if input.Configuration == nil {
			input.Configuration = map[string]string{}
		}
		allowed := map[string]bool{}
		for _, key := range definition.Configuration {
			allowed[key] = true
		}
		for key, value := range input.Configuration {
			if !allowed[key] || len(value) > 2000 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_preview_configuration"})
				return
			}
		}
		for _, key := range definition.Configuration {
			if _, found := input.Configuration[key]; !found {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "missing_preview_configuration", "name": key})
				return
			}
		}
		keys := make([]string, 0, len(input.Configuration))
		for key := range input.Configuration {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		hash := sha256.New()
		for _, key := range keys {
			_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", key, input.Configuration[key])
		}
		item, err := store.Create(previews.Preview{RepositoryID: pull.RepositoryID, SourceRepositoryID: pull.SourceRepositoryID, PullRequestID: pull.ID, Revision: pull.SourceCommitID, CreatorID: actor, Definition: definition, Configuration: input.Configuration, Attestation: previews.Attestation{CommitID: pull.SourceCommitID, DefinitionDigest: definitionDigest, ConfigurationDigest: fmt.Sprintf("%x", hash.Sum(nil))}})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(item)
		writeJSON(w, http.StatusCreated, item)
	}
}

func listPreviews(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(pull.RepositoryID, pull.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range items {
			items[i].Stale = items[i].Revision != pull.SourceCommitID
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}
}
func getPreview(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pull, _, ok := previewContext(w, r, pulls, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		item.Stale = item.Revision != pull.SourceCommitID
		writeJSON(w, 200, item)
	}
}
func proxyPreview(store *previews.Store, pulls previewPullStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		pull, err := pulls.Get(string(repo.ID), r.PathValue("pull_request"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		grant, ok := authenticateRequest(w, r, credentials, auth.ProfileRead)
		if !ok {
			return
		}
		participant := grant.UserID == repo.OwnerID || string(repo.Visibility) == "public"
		if !participant {
			participant, _ = repositories.IsCollaborator(repo.ID, grant.UserID)
		}
		var invitation previews.Invitation
		if !participant {
			if _, invitation, err = store.Authorize(pull.RepositoryID, pull.ID, r.PathValue("preview"), grant.UserID); err != nil {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		item, err := store.Get(pull.RepositoryID, pull.ID, r.PathValue("preview"))
		if err != nil || item.State != "ready" || item.LocalPort == 0 {
			writeJSON(w, 409, map[string]string{"error": "preview_not_ready"})
			return
		}
		allowedAction := r.Method == http.MethodGet || r.Method == http.MethodHead
		for _, action := range item.Definition.Audience.Actions {
			if action == "submit_test_data" && invitation.Role == "test" {
				allowedAction = true
			}
		}
		if !allowedAction {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "preview_action_restricted"})
			return
		}
		w.Header().Set("X-Komodo-Preview-Network", item.Definition.Audience.Network)
		w.Header().Set("X-Komodo-Preview-Data", item.Definition.Audience.Data)
		w.Header().Set("X-Komodo-Preview-Identity", item.Definition.Audience.Identity)
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; form-action 'self'; frame-ancestors 'self'")
		target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", item.LocalPort))
		prefix := "/repositories/" + pull.RepositoryID + "/pull-requests/" + pull.ID + "/previews/" + item.ID + "/proxy"
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			req.Header.Del("X-Forwarded-User")
		}
		proxy.ServeHTTP(w, r)
	}
}
