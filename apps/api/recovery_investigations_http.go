package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/recoveryexercises"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/recoveryinvestigations"
)

func registerRecoveryInvestigationsHTTP(mux *http.ServeMux, store *ri.Store, exercises *recoveryexercises.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/recovery-investigations"
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
			if recoveryInvestigationVisible(v, a.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.CreateInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		ex, e := exercises.Get(string(repo.ID), in.ExerciseID)
		if e != nil || ex.Revision != in.ExerciseRevision || ex.Result == nil {
			writeJSON(w, 422, map[string]string{"error": "completed_exercise_required"})
			return
		}
		valid := map[string]bool{}
		for _, x := range ex.Resources {
			valid[x.ResourceID] = true
		}
		for _, x := range in.ResourceIDs {
			if !valid[x] {
				writeJSON(w, 422, map[string]string{"error": "exercise_resource_required"})
				return
			}
		}
		v, e := store.Create(string(repo.ID), a.UserID, ex.PlanID, ex.PlanVersion, ex.Status, in)
		writeRecoveryInvestigation(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{investigation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || !recoveryInvestigationVisible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "recovery_investigation_not_found"})
			return
		}
		if ex, er := exercises.Get(string(repo.ID), v.ExerciseID); er == nil {
			v = ri.Resolve(v, ex.Revision, ex.Current)
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
		writeRecoveryInvestigation(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.Finding
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.AddFinding(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRecoveryInvestigation(w, v, e, 201)
	})
}
func recoveryInvestigationVisible(v ri.Investigation, actor string) bool {
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
func writeRecoveryInvestigation(w http.ResponseWriter, v ri.Investigation, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, ri.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "recovery_investigation_not_found"})
	} else if errors.Is(e, ri.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_recovery_investigation"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
