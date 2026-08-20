package main

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type releaseBuildStore interface {
	List(string, string) ([]checkruns.Run, error)
	Get(string, string, string) (checkruns.Run, error)
	OpenArtifact(string, string, string, string) (checkruns.Artifact, *os.File, error)
}
type releaseBuildController interface {
	ValidateRelease(string, string) error
	StartRelease(string, string, string, string) ([]checkruns.Run, error)
	Rerun(string, string, string, string) (checkruns.Run, error)
}

type releaseAttestation struct {
	ReleaseID      string          `json:"release_id"`
	RepositoryID   string          `json:"repository_id"`
	SourceCommitID string          `json:"source_commit_id"`
	CreatedByID    string          `json:"created_by_id"`
	Verified       bool            `json:"verified"`
	Attempts       []checkruns.Run `json:"attempts"`
}

type releaseStore interface {
	Create(releases.CreateParams) (releases.Release, error)
	Get(string, string) (releases.Release, error)
	List(string) ([]releases.Release, error)
}

func registerReleasesHTTP(mux *http.ServeMux, store releaseStore, builds releaseBuildStore, controller releaseBuildController, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, security ...securityDeliverySources) {
	mux.HandleFunc("POST /repositories/{repository}/releases", createRelease(store, controller, pulls, repositories, credentials, security...))
	mux.HandleFunc("GET /repositories/{repository}/releases", listReleases(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}", getRelease(store, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}/attestation", getReleaseAttestation(store, builds, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}/builds/{run}/events", getReleaseBuildEvents(store, builds, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/releases/{release}/builds/{run}/artifacts/{artifact}", getReleaseArtifact(store, builds, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/releases/{release}/builds/{run}/rerun", rerunReleaseBuild(store, builds, controller, repositories, credentials))
}

func listReleases(store releaseStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	}
}

func getRelease(store releaseStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		item, err := store.Get(string(repository.ID), r.PathValue("release"))
		if errors.Is(err, releases.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, item)
	}
}

func createRelease(store releaseStore, controller releaseBuildController, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, security ...securityDeliverySources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		participant := actor.UserID == repository.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repository.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Version        string   `json:"version"`
			Notes          string   `json:"notes"`
			CommitID       string   `json:"commit_id"`
			PriorReleaseID string   `json:"prior_release_id"`
			Components     []string `json:"security_components"`
			Assets         []string `json:"security_assets"`
			RiskClasses    []string `json:"security_risk_classes"`
		}
		if !readJSON(w, r, &input, 70000) {
			return
		}
		input.CommitID = strings.TrimSpace(input.CommitID)
		opened, err := repositories.Open(repository.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		candidate := storage.ObjectID(input.CommitID)
		if _, err = opened.ReadCommit(candidate); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
			return
		}
		if err := controller.ValidateRelease(string(repository.ID), input.CommitID); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_release_manifest"})
			return
		}
		if len(security) > 0 {
			a, e := security[0].assess(string(repository.ID), repository.OrganizationID, "release", input.CommitID, input.CommitID, "main", input.Components, input.Assets, input.RiskClasses)
			if e != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			if !a.Ready {
				writeJSON(w, http.StatusConflict, map[string]any{"error": "security_requirements_unsatisfied", "security": a})
				return
			}
		}
		priorCommit := ""
		if input.PriorReleaseID != "" {
			prior, err := store.Get(string(repository.ID), input.PriorReleaseID)
			if errors.Is(err, releases.ErrNotFound) {
				writeJSON(w, 422, map[string]string{"error": "invalid_prior_release"})
				return
			}
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
			priorCommit = prior.CommitID
		}
		reachable, err := commitSet(opened, candidate)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_commit"})
			return
		}
		priorReachable := map[storage.ObjectID]bool{}
		if priorCommit != "" {
			if !reachable[storage.ObjectID(priorCommit)] {
				writeJSON(w, 422, map[string]string{"error": "prior_release_not_ancestor"})
				return
			}
			priorReachable, err = commitSet(opened, storage.ObjectID(priorCommit))
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		all, err := pulls.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		links := []releases.PullRequestLink{}
		proposalIDs, taskIDs, contributors := []string{}, []string{}, []string{}
		for _, pull := range all {
			merge := storage.ObjectID(pull.MergeCommitID)
			if pull.Status != pullrequests.Merged || !reachable[merge] || priorReachable[merge] {
				continue
			}
			link := releases.PullRequestLink{ID: pull.ID, Title: pull.Title, AuthorID: pull.AuthorID, MergeCommitID: pull.MergeCommitID}
			if pull.ReasoningContext != nil && pull.ReasoningContext.Kind == "decision" {
				link.DecisionID, link.DecisionVersion = pull.ReasoningContext.DecisionID, pull.ReasoningContext.DecisionVersion
			}
			links = append(links, link)
			proposalIDs = append(proposalIDs, pull.ProposalID)
			taskIDs = append(taskIDs, pull.TaskID)
			contributors = append(contributors, pull.AuthorID)
		}
		item, err := store.Create(releases.CreateParams{RepositoryID: string(repository.ID), Version: input.Version, Notes: input.Notes, CommitID: input.CommitID, PriorReleaseID: input.PriorReleaseID, PriorCommitID: priorCommit, CreatedByID: actor.UserID, PullRequests: links, ProposalIDs: proposalIDs, TaskIDs: taskIDs, ContributorIDs: contributors})
		switch {
		case errors.Is(err, releases.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_release"})
		case errors.Is(err, releases.ErrVersionConflict):
			writeJSON(w, 409, map[string]string{"error": "version_taken"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		default:
			if _, buildErr := controller.StartRelease(string(repository.ID), item.ID, item.CommitID, actor.UserID); buildErr != nil {
				writeJSON(w, 500, map[string]string{"error": "build_start_failed"})
				return
			}
			w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/releases/"+item.ID)
			writeJSON(w, 201, item)
		}
	}
}

func releaseEvidenceAccess(w http.ResponseWriter, r *http.Request, store releaseStore, repositories pullRequestRepositoryStore, credentials authStore, write bool) (releases.Release, auth.Grant, bool) {
	scope := auth.RepositoryRead
	if write {
		scope = auth.RepositoryWrite
	}
	repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, write)
	if !ok {
		return releases.Release{}, actor, false
	}
	item, err := store.Get(string(repository.ID), r.PathValue("release"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return releases.Release{}, actor, false
	}
	return item, actor, true
}

func getReleaseAttestation(store releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := releaseEvidenceAccess(w, r, store, repositories, credentials, false)
		if !ok {
			return
		}
		attempts, err := builds.List(item.RepositoryID, "release:"+item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		latest := map[string]checkruns.Run{}
		for i := len(attempts) - 1; i >= 0; i-- {
			latest[attempts[i].Definition.Name] = attempts[i]
		}
		verified := len(latest) > 0
		for _, run := range latest {
			if run.State != checkruns.Succeeded {
				verified = false
			}
		}
		writeJSON(w, 200, releaseAttestation{ReleaseID: item.ID, RepositoryID: item.RepositoryID, SourceCommitID: item.CommitID, CreatedByID: item.CreatedByID, Verified: verified, Attempts: attempts})
	}
}

func getReleaseBuildEvents(store releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := releaseEvidenceAccess(w, r, store, repositories, credentials, false)
		if !ok {
			return
		}
		run, err := builds.Get(item.RepositoryID, "release:"+item.ID, r.PathValue("run"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		events := []checkruns.Event{}
		for _, event := range run.Events {
			if event.Sequence > after {
				events = append(events, event)
			}
		}
		writeJSON(w, 200, map[string]any{"items": events})
	}
}

func getReleaseArtifact(store releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, _, ok := releaseEvidenceAccess(w, r, store, repositories, credentials, false)
		if !ok {
			return
		}
		artifact, file, err := builds.OpenArtifact(item.RepositoryID, "release:"+item.ID, r.PathValue("run"), r.PathValue("artifact"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", artifact.MediaType)
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(path.Base(artifact.Path)))
		http.ServeContent(w, r, path.Base(artifact.Path), time.Time{}, file)
	}
}

func rerunReleaseBuild(store releaseStore, builds releaseBuildStore, controller releaseBuildController, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := releaseEvidenceAccess(w, r, store, repositories, credentials, true)
		if !ok {
			return
		}
		repository, _ := repositories.Inspect(storage.ID(item.RepositoryID))
		participant := actor.UserID == repository.OwnerID
		if !participant {
			participant, _ = repositories.IsCollaborator(repository.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		previous, err := builds.Get(item.RepositoryID, "release:"+item.ID, r.PathValue("run"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if previous.State == checkruns.Queued || previous.State == checkruns.Running {
			writeJSON(w, 409, map[string]string{"error": "build_not_terminal"})
			return
		}
		run, err := controller.Rerun(item.RepositoryID, "release:"+item.ID, previous.ID, actor.UserID)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": "rerun_failed"})
			return
		}
		writeJSON(w, 201, run)
	}
}

func commitSet(repository storage.RepositoryStorage, root storage.ObjectID) (map[storage.ObjectID]bool, error) {
	seen := map[storage.ObjectID]bool{}
	pending := []storage.ObjectID{root}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[current] {
			continue
		}
		commit, err := repository.ReadCommit(current)
		if err != nil {
			return nil, err
		}
		seen[current] = true
		pending = append(pending, commit.Parents...)
	}
	return seen, nil
}
