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
	if runner != nil {
		mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/reproductions", createIssueReproduction(store, releaseStore, repositories, credentials, runner))
		mux.HandleFunc("GET /repositories/{repository}/issues/{issue}/reproductions", listIssueReproductions(store, repositories, credentials))
		mux.HandleFunc("GET /repositories/{repository}/issues/{issue}/reproductions/{attempt}", getIssueReproduction(store, repositories, credentials))
		mux.HandleFunc("POST /repositories/{repository}/issues/{issue}/reproductions/{attempt}/reruns", rerunIssueReproduction(store, repositories, credentials, runner))
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
