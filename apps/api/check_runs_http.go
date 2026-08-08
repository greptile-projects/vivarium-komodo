package main

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

type checkRunStore interface {
	List(string, string) ([]checkruns.Run, error)
	Get(string, string, string) (checkruns.Run, error)
	OpenArtifact(string, string, string, string) (checkruns.Artifact, *os.File, error)
}

type checkRunController interface {
	Rerun(string, string, string, string) (checkruns.Run, error)
	Cancel(string, string, string, string) (checkruns.Run, error)
}

func registerCheckRunsHTTP(mux *http.ServeMux, runs checkRunStore, controller checkRunController, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore, sessions changeSessionStore, activity activityStore) {
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/check-runs", func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		if _, ok := readPullRequest(w, pulls, string(repository.ID), pullID); !ok {
			return
		}
		items, err := runs.List(string(repository.ID), pullID)
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
	})

	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}", func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		if _, ok := readPullRequest(w, pulls, string(repository.ID), pullID); !ok {
			return
		}
		run, err := runs.Get(string(repository.ID), pullID, r.PathValue("run"))
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, run)
	})

	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}/events", func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		if _, ok := readPullRequest(w, pulls, string(repository.ID), pullID); !ok {
			return
		}
		run, err := runs.Get(string(repository.ID), pullID, r.PathValue("run"))
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		after := int64(0)
		if raw := r.URL.Query().Get("after"); raw != "" {
			value, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr != nil || value < 0 {
				writeJSON(w, 422, map[string]string{"error": "invalid_after"})
				return
			}
			after = value
		}
		events := make([]checkruns.Event, 0)
		for _, event := range run.Events {
			if event.Sequence > after {
				events = append(events, event)
			}
		}
		writeJSON(w, 200, map[string]any{"items": events, "last_sequence": int64(len(run.Events)), "state": run.State})
	})

	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}/artifacts/{artifact}", func(w http.ResponseWriter, r *http.Request) {
		repository, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		if _, ok := readPullRequest(w, pulls, string(repository.ID), pullID); !ok {
			return
		}
		artifact, file, err := runs.OpenArtifact(string(repository.ID), pullID, r.PathValue("run"), r.PathValue("artifact"))
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", artifact.MediaType)
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(path.Base(artifact.Path)))
		http.ServeContent(w, r, path.Base(artifact.Path), time.Time{}, file)
	})

	control := func(w http.ResponseWriter, r *http.Request, rerun bool) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		if _, ok := readPullRequest(w, pulls, string(repository.ID), pullID); !ok {
			return
		}
		var run checkruns.Run
		var err error
		if rerun {
			run, err = controller.Rerun(string(repository.ID), pullID, r.PathValue("run"), actor.UserID)
		} else {
			run, err = controller.Cancel(string(repository.ID), pullID, r.PathValue("run"), actor.UserID)
		}
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if errors.Is(err, checkruns.ErrInvalidTransition) {
			writeJSON(w, 409, map[string]string{"error": "invalid_check_state"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if rerun {
			w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/pull-requests/"+pullID+"/check-runs/"+run.ID)
			writeJSON(w, 201, run)
			return
		}
		writeJSON(w, 200, run)
	}
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}/rerun", func(w http.ResponseWriter, r *http.Request) { control(w, r, true) })
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}/cancel", func(w http.ResponseWriter, r *http.Request) { control(w, r, false) })
	mux.HandleFunc("POST /repositories/{repository}/pull-requests/{pull_request}/check-runs/{run}/change-session", func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		pullID := r.PathValue("pull_request")
		pull, ok := readPullRequest(w, pulls, string(repository.ID), pullID)
		if !ok {
			return
		}
		if pull.Status != "open" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "pull_request_not_open"})
			return
		}
		run, err := runs.Get(string(repository.ID), pullID, r.PathValue("run"))
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if run.State != checkruns.Failed {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "check_not_failed"})
			return
		}
		if run.CommitID != pull.SourceCommitID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "check_revision_not_current"})
			return
		}
		failure := changesessions.CheckFailure{RunID: run.ID, CommitID: run.CommitID, Name: run.Definition.Name, Command: run.Definition.Command, WorkingDirectory: run.Definition.WorkingDirectory, TimeoutSeconds: run.Definition.TimeoutSeconds, Environment: run.Definition.Environment, DeclaredArtifacts: run.Definition.Artifacts, Error: run.Error}
		for _, event := range run.Events {
			if event.Type == "log" {
				failure.Logs = append(failure.Logs, changesessions.CheckLog{Sequence: event.Sequence, Stream: event.Stream, Message: event.Message})
			}
			if event.Artifact != nil {
				failure.Artifacts = append(failure.Artifacts, changesessions.CheckArtifact{ID: event.Artifact.ID, Path: event.Artifact.Path, Size: event.Artifact.Size, SHA256: event.Artifact.SHA256, MediaType: event.Artifact.MediaType})
			}
			if event.Outcome != nil {
				failure.ExitCode, failure.TimedOut = event.Outcome.ExitCode, event.Outcome.TimedOut
			}
		}
		item, err := sessions.CreateWithCheckFailure(string(repository.ID), pullID, actor.UserID, run.CommitID, &failure)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if err := recordActivity(activity, activities.Input{RepositoryID: string(repository.ID), ActorID: actor.UserID, Type: "change_session.started", Resource: activities.Resource{Type: "pull_request", ID: pullID}, Metadata: map[string]string{"session_id": item.ID, "source_commit_id": item.SourceCommitID, "check_run_id": run.ID}}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/pull-requests/"+pullID+"/change-sessions/"+item.ID)
		writeJSON(w, http.StatusCreated, item)
	})
}
