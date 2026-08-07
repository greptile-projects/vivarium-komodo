package main

import (
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

type activityStore interface {
	Record(activities.Input) (activities.Event, error)
	List(string) ([]activities.Event, error)
}

func registerActivitiesHTTP(mux *http.ServeMux, store activityStore, repositories proposalRepositoryStore, credentials authStore) {
	mux.HandleFunc("GET /repositories/{repository}/activity", func(w http.ResponseWriter, r *http.Request) {
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
		items = paginate(items, page, perPage)
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "per_page": perPage, "total_count": total})
	})
}

func recordActivity(store activityStore, input activities.Input) error {
	if store == nil {
		return nil
	}
	_, err := store.Record(input)
	return err
}
