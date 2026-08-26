package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacitydeliveries"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capacityplans"
)

type capacityDeliveryStore interface {
	Create(string, string, capacitydeliveries.Input) (capacitydeliveries.Delivery, error)
	Observe(string, string, string, string, capacitydeliveries.ObservationInput) (capacitydeliveries.Delivery, error)
	Control(string, string, string, string, capacitydeliveries.ControlInput) (capacitydeliveries.Delivery, error)
	Get(string, string) (capacitydeliveries.Delivery, error)
	List(string, string) ([]capacitydeliveries.Delivery, error)
}

func registerCapacityDeliveriesHTTP(mux *http.ServeMux, store capacityDeliveryStore, plans capacityPlanStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/capacity-plans/{plan}/deliveries"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID), r.PathValue("plan"))
		if capacityDeliveryError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": xs})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in capacitydeliveries.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if in.PlanID != r.PathValue("plan") {
			writeJSON(w, 422, map[string]string{"error": "invalid_capacity_delivery"})
			return
		}
		plan, e := plans.Get(string(repo.ID), in.PlanID)
		if e != nil || plan.Revision != in.PlanRevision || plan.ObjectiveID != in.ObjectiveID || plan.ObjectiveVersion != in.ObjectiveVersion || plan.ModelID != in.ModelID || plan.ModelRevision != in.ModelRevision || capacityplans.Resolve(plan).Status != "approved" {
			writeJSON(w, 409, map[string]string{"error": "capacity_delivery_plan_not_current"})
			return
		}
		x, e := store.Create(string(repo.ID), a.UserID, in)
		if capacityDeliveryError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{delivery}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := store.Get(string(repo.ID), r.PathValue("delivery"))
		if capacityDeliveryError(w, e) {
			return
		}
		if x.PlanID != r.PathValue("plan") {
			writeJSON(w, 404, map[string]string{"error": "capacity_delivery_not_found"})
			return
		}
		writeJSON(w, 200, x)
	})
	post := func(suffix string, fn func(string, string, string, string, *http.Request) (capacitydeliveries.Delivery, error)) {
		mux.HandleFunc("POST "+base+"/{delivery}"+suffix, func(w http.ResponseWriter, r *http.Request) {
			repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
			if !ok {
				return
			}
			kind := r.Header.Get("X-Actor-Kind")
			if kind == "" {
				kind = "human"
			}
			x, e := fn(string(repo.ID), r.PathValue("delivery"), kind, a.UserID, r)
			if capacityDeliveryError(w, e) {
				return
			}
			if x.PlanID != r.PathValue("plan") {
				writeJSON(w, 404, map[string]string{"error": "capacity_delivery_not_found"})
				return
			}
			writeJSON(w, 201, x)
		})
	}
	post("/observations", func(repo, did, kind, actor string, r *http.Request) (capacitydeliveries.Delivery, error) {
		var in capacitydeliveries.ObservationInput
		if e := jsonBody(r, &in); e != nil {
			return capacitydeliveries.Delivery{}, capacitydeliveries.ErrInvalid
		}
		return store.Observe(repo, did, kind, actor, in)
	})
	post("/controls", func(repo, did, kind, actor string, r *http.Request) (capacitydeliveries.Delivery, error) {
		var in capacitydeliveries.ControlInput
		if e := jsonBody(r, &in); e != nil {
			return capacitydeliveries.Delivery{}, capacitydeliveries.ErrInvalid
		}
		return store.Control(repo, did, kind, actor, in)
	})
}
func capacityDeliveryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capacitydeliveries.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capacity_delivery_not_found"})
	case errors.Is(e, capacitydeliveries.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capacity_delivery"})
	case errors.Is(e, capacitydeliveries.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capacity_delivery_changed"})
	case errors.Is(e, capacitydeliveries.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capacity_delivery_action_forbidden"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
