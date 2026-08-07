package main

import (
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
)

type checkRunStore interface {
	List(string, string) ([]checkruns.Run, error)
}

func registerCheckRunsHTTP(mux *http.ServeMux, runs checkRunStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) {
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
}
