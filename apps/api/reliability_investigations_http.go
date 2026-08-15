package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/reliabilityinvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/serviceobjectives"
)

type reliabilityObjectiveStore interface {
	Get(string, string) (serviceobjectives.Objective, error)
}

func registerReliabilityInvestigationsHTTP(mux *http.ServeMux, store *ri.Store, objectives reliabilityObjectiveStore, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/reliability-investigations"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := store.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := xs[:0]
		for _, v := range xs {
			if reliabilityVisible(v, a.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.CreateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		o, e := objectives.Get(string(repo.ID), in.ObjectiveID)
		if e != nil || o.CurrentVersion != in.ObjectiveVersion {
			writeJSON(w, 422, map[string]string{"error": "invalid_objective_version"})
			return
		}
		validJourney := map[string]bool{}
		for _, j := range o.Versions[in.ObjectiveVersion-1].Journeys {
			validJourney[j.ID] = true
		}
		for _, j := range in.JourneyIDs {
			if !validJourney[j] {
				writeJSON(w, 422, map[string]string{"error": "invalid_journey"})
				return
			}
		}
		if in.Trigger.Kind == "objective" && (in.Trigger.ResourceID != o.ID || in.Trigger.Revision != "version:"+reliabilityVersion(in.ObjectiveVersion)) {
			writeJSON(w, 422, map[string]string{"error": "invalid_trigger"})
			return
		}
		v, e := store.Create(string(repo.ID), a.UserID, in)
		if e == nil {
			for _, owner := range o.Versions[in.ObjectiveVersion-1].OwnerIDs {
				v, e = store.Invite(string(repo.ID), v.ID, a.UserID, owner)
				if e != nil {
					break
				}
			}
		}
		writeRI(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{investigation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || !reliabilityVisible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if o, e := objectives.Get(string(repo.ID), v.ObjectiveID); e == nil {
			v = ri.Resolve(v, o.CurrentVersion, v.Revision)
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID string `json:"user_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		allowed := in.UserID == repo.OwnerID
		for _, x := range repo.CollaboratorIDs {
			allowed = allowed || x == in.UserID
		}
		if !allowed {
			writeJSON(w, 422, map[string]string{"error": "repository_participant_required"})
			return
		}
		v, e := store.Invite(string(repo.ID), r.PathValue("investigation"), a.UserID, in.UserID)
		writeRI(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/entries", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.Entry
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.Add(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRI(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/input-requests", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.InputRequest
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := store.Request(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRI(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/outcomes", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.Outcome
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := store.AddOutcome(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRI(w, v, e, 201)
	})
}
func reliabilityVisible(v ri.Investigation, actor string) bool {
	for _, p := range v.Participants {
		if p == actor {
			return true
		}
	}
	for _, e := range v.Evidence {
		if e.Audience == "participants" {
			return false
		}
	}
	return true
}
func writeRI(w http.ResponseWriter, v ri.Investigation, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, ri.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else if errors.Is(e, ri.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_reliability_investigation"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
func reliabilityVersion(v int64) string { return strconv.FormatInt(v, 10) }
