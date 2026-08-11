package main

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type issueStore interface {
	Create(issues.CreateInput) (issues.Issue, error)
	Get(string, string) (issues.Issue, error)
	List(string) ([]issues.Issue, error)
	AddComment(string, string, string, string) (issues.Issue, error)
	SetStatus(string, string, string, string) (issues.Issue, error)
	SetTriage(string, string, string, int64, issues.Triage) (issues.Issue, error)
	AddRelationship(string, string, string, issues.Relationship) (issues.Issue, error)
	CreateInvestigation(string, string, string, string, string) (issues.Issue, issues.Investigation, error)
	AddInvestigationEntry(string, string, string, string, string, issues.InvestigationEntry) (issues.Issue, issues.InvestigationEntry, error)
	StartAgentRun(string, string, string, string, string) (issues.Issue, string, error)
	RevokeAgentRun(string, string, string, string, string) (issues.Issue, error)
	AgentContext(string) (issues.Issue, issues.Investigation, issues.AgentRun, error)
	CreateReproduction(issues.Issue, string, string, string, string, string, issues.ReproductionDefinition, string, issues.ReproductionCommand, []issues.ReproductionInput) (issues.ReproductionAttempt, error)
	GetReproduction(string, string, string) (issues.ReproductionAttempt, error)
	ListReproductions(string, string) ([]issues.ReproductionAttempt, error)
}
type issueReleaseStore interface {
	Get(string, string) (releases.Release, error)
}
type issueRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerIssuesHTTP(mux *http.ServeMux, store issueStore, releaseStore issueReleaseStore, repositories issueRepositoryStore, credentials authStore, reproduction ...*issues.ReproductionRunner) {
	var runner *issues.ReproductionRunner
	if len(reproduction) > 0 {
		runner = reproduction[0]
	}
	mux.HandleFunc("GET /repositories/{repository}/issue-templates", listIssueTemplates(repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/issues/suggestions", suggestIssues(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues", createIssue(store, releaseStore, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/issues", listIssues(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/issues/{issue}", getIssue(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/comments", commentIssue(store, repositories, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}/issues/{issue}", updateIssue(store, repositories, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/issues/{issue}/triage", triageIssue(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/relationships", relateIssue(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/investigations", createIssueInvestigation(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/investigations/{investigation}/entries", addIssueInvestigationEntry(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/investigations/{investigation}/agent-runs", startIssueAgent(store, repositories, credentials))
	mux.HandleFunc("DELETE /repositories/{repository}/issues/{issue}/investigations/{investigation}/agent-runs/{run}", revokeIssueAgent(store, repositories, credentials))
	mux.HandleFunc("GET /issue-investigation-agent/context", issueAgentContext(store))
	mux.HandleFunc("POST /issue-investigation-agent/entries", issueAgentEntry(store))
	if runner != nil {
		mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/reproductions", createIssueReproduction(store, releaseStore, repositories, credentials, runner))
		mux.HandleFunc("GET /repositories/{repository}/issues/{issue}/reproductions", listIssueReproductions(store, repositories, credentials))
		mux.HandleFunc("GET /repositories/{repository}/issues/{issue}/reproductions/{attempt}", getIssueReproduction(store, repositories, credentials))
		mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/reproductions/{attempt}/reruns", rerunIssueReproduction(store, repositories, credentials, runner))
	}
}

func ownerIssue(w http.ResponseWriter, r *http.Request, store issueStore, repos proposalRepositoryStore, credentials authStore) (repositories.Repository, auth.Grant, issues.Issue, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
	if !ok {
		return repo, actor, issues.Issue{}, false
	}
	item, err := store.Get(string(repo.ID), r.PathValue("issue"))
	if err != nil || !issueVisible(repos, repo, item, actor.UserID) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return repo, actor, item, false
	}
	if actor.UserID != repo.OwnerID {
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
		return repo, actor, item, false
	}
	return repo, actor, item, true
}
func triageIssue(store issueStore, repos proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, _, ok := ownerIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64    `json:"expected_version"`
			Classification  string   `json:"classification"`
			Priority        string   `json:"priority"`
			Assignees       []string `json:"assignee_ids"`
			Labels          []string `json:"labels"`
			DuplicateOf     string   `json:"duplicate_of"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		for _, id := range in.Assignees {
			participant := id == repo.OwnerID
			if !participant {
				participant, _ = repos.IsCollaborator(repo.ID, id)
			}
			if !participant {
				writeJSON(w, 422, map[string]string{"error": "invalid_assignee"})
				return
			}
		}
		if in.DuplicateOf != "" {
			other, e := store.Get(string(repo.ID), in.DuplicateOf)
			if e != nil || !issueVisible(repos, repo, other, actor.UserID) || other.ID == r.PathValue("issue") {
				writeJSON(w, 422, map[string]string{"error": "invalid_duplicate"})
				return
			}
		}
		v, e := store.SetTriage(string(repo.ID), r.PathValue("issue"), actor.UserID, in.ExpectedVersion, issues.Triage{Classification: in.Classification, Priority: in.Priority, AssigneeIDs: in.Assignees, Labels: in.Labels, DuplicateOf: in.DuplicateOf})
		writeIssue(w, v, e)
	}
}
func relateIssue(store issueStore, repos issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, _, ok := ownerIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		var link issues.Relationship
		if !readJSON(w, r, &link, 32<<10) {
			return
		}
		if link.Kind == "code" {
			if link.RepositoryID == "" {
				link.RepositoryID = string(repo.ID)
			}
			if link.RepositoryID != string(repo.ID) || link.Revision == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_relationship"})
				return
			}
			opened, e := repos.Open(repo.ID)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_relationship"})
				return
			}
			if _, e = opened.ReadCommit(storage.ObjectID(link.Revision)); e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_relationship"})
				return
			}
		}
		v, e := store.AddRelationship(string(repo.ID), r.PathValue("issue"), actor.UserID, link)
		writeIssue(w, v, e)
	}
}
func createIssueInvestigation(store issueStore, repos issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ReproductionID string `json:"reproduction_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		attempt, e := store.GetReproduction(string(repo.ID), item.ID, in.ReproductionID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_reproduction"})
			return
		}
		v, inv, e := store.CreateInvestigation(string(repo.ID), item.ID, attempt.ID, attempt.Revision, actor.UserID)
		if e != nil {
			writeIssue(w, v, e)
			return
		}
		writeJSON(w, 201, map[string]any{"issue": v, "investigation": inv})
	}
}
func addIssueInvestigationEntry(store issueStore, repos issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		var entry issues.InvestigationEntry
		if !readJSON(w, r, &entry, 128<<10) {
			return
		}
		selected := ""
		for _, inv := range item.Investigations {
			if inv.ID == r.PathValue("investigation") {
				selected = inv.ReproductionID
			}
		}
		if selected == "" || !validIssueCitations(store, string(repo.ID), item, selected, entry.Citations) {
			writeJSON(w, 422, map[string]string{"error": "invalid_citation"})
			return
		}
		v, e2, e := store.AddInvestigationEntry(string(repo.ID), item.ID, r.PathValue("investigation"), "human", actor.UserID, entry)
		if e != nil {
			writeIssue(w, v, e)
			return
		}
		writeJSON(w, 201, map[string]any{"issue": v, "entry": e2})
	}
}
func validIssueCitations(store issueStore, repo string, item issues.Issue, selectedReproduction string, citations []issues.Citation) bool {
	for _, c := range citations {
		if c.Kind == "reproduction_event" || c.Kind == "reproduction_artifact" {
			if c.ResourceID != selectedReproduction {
				return false
			}
			a, e := store.GetReproduction(repo, item.ID, c.ResourceID)
			if e != nil {
				return false
			}
			if c.Kind == "reproduction_event" {
				found := false
				for _, ev := range a.Events {
					if ev.Sequence == c.EventSequence {
						found = true
					}
				}
				if !found {
					return false
				}
			} else {
				found := false
				for _, artifact := range a.Artifacts {
					if artifact.Path == c.ArtifactPath {
						found = true
					}
				}
				if !found {
					return false
				}
			}
		} else if c.Kind == "code" {
			if c.Revision == "" || c.ResourceID != repo {
				return false
			}
		} else if c.Kind == "relationship" {
			found := false
			for _, link := range item.Relationships {
				if link.ID == c.ResourceID {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
func revokeIssueAgent(store issueStore, repos proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, actor, item, ok := ownerIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		v, e := store.RevokeAgentRun(item.RepositoryID, item.ID, r.PathValue("investigation"), r.PathValue("run"), actor.UserID)
		writeIssue(w, v, e)
	}
}
func startIssueAgent(store issueStore, repos proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, actor, item, ok := ownerIssue(w, r, store, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			AgentID string `json:"agent_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, token, e := store.StartAgentRun(item.RepositoryID, item.ID, r.PathValue("investigation"), in.AgentID, actor.UserID)
		if e != nil {
			writeIssue(w, v, e)
			return
		}
		writeJSON(w, 201, map[string]any{"issue": v, "worker_credential": token, "credential_notice": "shown once; selected reproduction and cited investigation publication only; no Git or repository-write authority"})
	}
}
func issueAgentToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
}
func issueAgentContext(store issueStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, inv, run, e := store.AgentContext(issueAgentToken(r))
		if e != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		for i := range item.Attachments {
			item.Attachments[i].Content = ""
		}
		attempt, e := store.GetReproduction(item.RepositoryID, item.ID, inv.ReproductionID)
		if e != nil {
			writeJSON(w, 409, map[string]string{"error": "selected_reproduction_unavailable"})
			return
		}
		writeJSON(w, 200, map[string]any{"issue": item, "investigation": inv, "reproduction": attempt, "agent_run": run, "authority": map[string]bool{"repository_read": false, "repository_write": false, "git": false, "publish_cited_entries": true}})
	}
}
func issueAgentEntry(store issueStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, inv, run, e := store.AgentContext(issueAgentToken(r))
		if e != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		var entry issues.InvestigationEntry
		if !readJSON(w, r, &entry, 128<<10) {
			return
		}
		if !validIssueCitations(store, item.RepositoryID, item, inv.ReproductionID, entry.Citations) {
			writeJSON(w, 422, map[string]string{"error": "invalid_citation"})
			return
		}
		v, e2, e := store.AddInvestigationEntry(item.RepositoryID, item.ID, inv.ID, "agent", run.AgentID, entry)
		if e != nil {
			writeIssue(w, v, e)
			return
		}
		writeJSON(w, 201, map[string]any{"issue": v, "entry": e2})
	}
}
func listIssueTemplates(repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false); !ok {
			return
		}
		writeJSON(w, 200, map[string]any{"items": []map[string]any{{"id": "unexpected-behavior", "name": "Unexpected behavior", "description": "Capture what you expected, what happened, and how to reproduce it.", "required_fields": []string{"title", "expected_behavior", "observed_behavior", "severity", "environment", "reproduction_steps"}}}})
	}
}

func createIssue(store issueStore, releaseStore issueReleaseStore, repositories issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Title       string              `json:"title"`
			Expected    string              `json:"expected_behavior"`
			Observed    string              `json:"observed_behavior"`
			Severity    string              `json:"severity"`
			Environment string              `json:"environment"`
			Steps       []string            `json:"reproduction_steps"`
			ReleaseID   string              `json:"affected_release_id"`
			CommitID    string              `json:"affected_commit_id"`
			Visibility  string              `json:"visibility"`
			Attachments []issues.Attachment `json:"attachments"`
		}
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		version, commitID := "", strings.TrimSpace(in.CommitID)
		if in.ReleaseID != "" {
			rel, err := releaseStore.Get(string(repo.ID), in.ReleaseID)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_affected_release"})
				return
			}
			version = rel.Version
			commitID = rel.CommitID
		} else if commitID != "" {
			opened, err := repositories.Open(repo.ID)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_affected_commit"})
				return
			}
			if _, err = opened.ReadCommit(storage.ObjectID(commitID)); err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_affected_commit"})
				return
			}
		}
		item, err := store.Create(issues.CreateInput{RepositoryID: string(repo.ID), ReporterID: actor.UserID, Title: in.Title, ExpectedBehavior: in.Expected, ObservedBehavior: in.Observed, Severity: in.Severity, Environment: in.Environment, ReproductionSteps: in.Steps, AffectedReleaseID: in.ReleaseID, AffectedVersion: version, AffectedCommitID: commitID, Visibility: in.Visibility, Attachments: in.Attachments})
		if errors.Is(err, issues.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_issue"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, item)
	}
}

func visibleIssue(item issues.Issue, repoPublic bool, actor string, participant bool) bool {
	return item.Visibility == "public" && repoPublic || actor == item.ReporterID || participant
}

func issueVisible(store proposalRepositoryStore, repo repositories.Repository, item issues.Issue, actor string) bool {
	participant := actor != "" && actor == repo.OwnerID
	if actor != "" && !participant {
		participant, _ = store.IsCollaborator(repo.ID, actor)
	}
	return visibleIssue(item, string(repo.Visibility) == "public", actor, participant)
}
func listIssues(store issueStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := []issues.Issue{}
		for _, v := range items {
			if issueVisible(repositories, repo, v, actor.UserID) {
				out = append(out, summaryIssue(v))
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func getIssue(store issueStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, err := store.Get(string(repo.ID), r.PathValue("issue"))
		if err != nil || !issueVisible(repositories, repo, v, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func commentIssue(store issueStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, err := store.Get(string(repo.ID), r.PathValue("issue"))
		if err != nil || !issueVisible(repositories, repo, v, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		v, err = store.AddComment(string(repo.ID), v.ID, actor.UserID, in.Body)
		writeIssue(w, v, err)
	}
}
func updateIssue(store issueStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Status string `json:"status"`
		}
		if !readJSON(w, r, &in, 4<<10) {
			return
		}
		v, err := store.Get(string(repo.ID), r.PathValue("issue"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if actor.UserID != v.ReporterID && actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		v, err = store.SetStatus(string(repo.ID), v.ID, actor.UserID, in.Status)
		writeIssue(w, v, err)
	}
}
func writeIssue(w http.ResponseWriter, v issues.Issue, err error) {
	if errors.Is(err, issues.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_issue"})
	} else if errors.Is(err, issues.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "issue_version_conflict"})
	} else if errors.Is(err, issues.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else if err != nil {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	} else {
		writeJSON(w, 200, v)
	}
}
func summaryIssue(v issues.Issue) issues.Issue {
	for i := range v.Attachments {
		v.Attachments[i].Content = ""
	}
	v.Comments = nil
	v.History = nil
	return v
}
func suggestIssues(store issueStore, repositories proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(query) < 3 {
			writeJSON(w, 200, map[string]any{"items": []issues.Issue{}})
			return
		}
		wanted := words(query)
		items, _ := store.List(string(repo.ID))
		type scored struct {
			v issues.Issue
			n int
		}
		matches := []scored{}
		for _, v := range items {
			if !issueVisible(repositories, repo, v, actor.UserID) {
				continue
			}
			n := 0
			for word := range words(v.Title + " " + v.ObservedBehavior) {
				if wanted[word] {
					n++
				}
			}
			if n > 0 {
				matches = append(matches, scored{v, n})
			}
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].n > matches[j].n })
		out := []issues.Issue{}
		for i, m := range matches {
			if i == 5 {
				break
			}
			out = append(out, summaryIssue(m.v))
		}
		writeJSON(w, 200, map[string]any{"items": out})
	}
}
func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, v := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(v) > 2 {
			out[v] = true
		}
	}
	return out
}

func reproductionIssue(w http.ResponseWriter, r *http.Request, store issueStore, repositoryStore issueRepositoryStore, credentials authStore) (repositories.Repository, auth.Grant, issues.Issue, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryRead, true)
	if !ok {
		return repositories.Repository{}, auth.Grant{}, issues.Issue{}, false
	}
	item, err := store.Get(string(repo.ID), r.PathValue("issue"))
	if err != nil || !issueVisible(repositoryStore, repo, item, actor.UserID) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return repositories.Repository{}, auth.Grant{}, issues.Issue{}, false
	}
	return repo, actor, item, true
}

func createIssueReproduction(store issueStore, releases issueReleaseStore, repositories issueRepositoryStore, credentials authStore, runner *issues.ReproductionRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repositories, credentials)
		if !ok {
			return
		}
		if actor.UserID != item.ReporterID && actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var input struct {
			Name   string                     `json:"name"`
			Inputs []issues.ReproductionInput `json:"inputs"`
		}
		if !readJSON(w, r, &input, 6<<20) {
			return
		}
		revision, releaseID, releaseVersion := item.AffectedCommitID, item.AffectedReleaseID, item.AffectedVersion
		if releaseID != "" {
			release, err := releases.Get(string(repo.ID), releaseID)
			if err != nil || release.CommitID != revision {
				writeJSON(w, 409, map[string]string{"error": "affected_release_unavailable"})
				return
			}
			releaseVersion = release.Version
		}
		if revision == "" {
			writeJSON(w, 422, map[string]string{"error": "issue_has_no_affected_revision"})
			return
		}
		definition, digest, err := runner.Definition(string(repo.ID), revision)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_reproduction_definition"})
			return
		}
		var command *issues.ReproductionCommand
		for i := range definition.Reproductions {
			if definition.Reproductions[i].Name == input.Name {
				command = &definition.Reproductions[i]
				break
			}
		}
		if command == nil {
			writeJSON(w, 422, map[string]string{"error": "unknown_reproduction_command"})
			return
		}
		attempt, err := store.CreateReproduction(item, revision, releaseID, releaseVersion, actor.UserID, "", definition, digest, *command, input.Inputs)
		if errors.Is(err, issues.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "unsafe_reproduction_input"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(attempt)
		writeJSON(w, 202, attempt)
	}
}

func listIssueReproductions(store issueStore, repositories issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, item, ok := reproductionIssue(w, r, store, repositories, credentials)
		if !ok {
			return
		}
		attempts, err := store.ListReproductions(string(repo.ID), item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": attempts, "total_count": len(attempts)})
	}
}
func getIssueReproduction(store issueStore, repositories issueRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, item, ok := reproductionIssue(w, r, store, repositories, credentials)
		if !ok {
			return
		}
		attempt, err := store.GetReproduction(string(repo.ID), item.ID, r.PathValue("attempt"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, attempt)
	}
}
func rerunIssueReproduction(store issueStore, repositories issueRepositoryStore, credentials authStore, runner *issues.ReproductionRunner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, item, ok := reproductionIssue(w, r, store, repositories, credentials)
		if !ok {
			return
		}
		previous, err := store.GetReproduction(string(repo.ID), item.ID, r.PathValue("attempt"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if previous.State == "queued" || previous.State == "running" {
			writeJSON(w, 409, map[string]string{"error": "attempt_not_terminal"})
			return
		}
		attempt, err := store.CreateReproduction(item, previous.Revision, previous.ReleaseID, previous.ReleaseVersion, actor.UserID, previous.ID, previous.Definition, previous.DefinitionDigest, previous.Command, previous.Inputs)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		runner.Start(attempt)
		writeJSON(w, 202, attempt)
	}
}
