package main

import (
	"errors"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
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

func registerCheckRunsHTTP(mux *http.ServeMux, runs checkRunStore, controller checkRunController, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) {
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
}
