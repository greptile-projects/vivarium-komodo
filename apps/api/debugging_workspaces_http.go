package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type debuggingReleaseStore interface {
	Get(string, string) (releases.Release, error)
}

func registerDebuggingWorkspacesHTTP(mux *http.ServeMux, store *dw.Store, repos dataFlowRepositories, credentials authStore, releaseStore debuggingReleaseStore) {
	base := "/repositories/{repository}/debugging-workspaces"
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
		out := []dw.Workspace{}
		for _, v := range xs {
			if debuggingVisible(v, a.UserID) {
				out = append(out, debuggingProject(v, a.UserID))
			}
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in dw.CreateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_revision"})
			return
		}
		if _, e = opened.ReadCommit(storage.ObjectID(in.SourceRevision)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_revision"})
			return
		}
		release, e := releaseStore.Get(string(repo.ID), in.ReleaseID)
		if e != nil || release.CommitID != in.ReleaseRevision || in.ReleaseRevision != in.SourceRevision {
			writeJSON(w, 422, map[string]string{"error": "release_source_mismatch"})
			return
		}
		allowed := map[string]bool{repo.OwnerID: true}
		for _, id := range repo.CollaboratorIDs {
			allowed[id] = true
		}
		for _, id := range append(in.OwnerIDs, in.ParticipantIDs...) {
			if !allowed[id] {
				writeJSON(w, 422, map[string]string{"error": "repository_participant_required"})
				return
			}
		}
		v, e := store.Create(string(repo.ID), a.UserID, in)
		writeDebugging(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{workspace}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingVisible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, debuggingProject(v, a.UserID))
	})
	mux.HandleFunc("POST "+base+"/{workspace}/hypotheses", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in dw.Hypothesis
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := store.AddHypothesis(string(repo.ID), r.PathValue("workspace"), a.UserID, in)
		writeDebugging(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{workspace}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Status         string   `json:"status"`
			ParticipantIDs []string `json:"participant_ids"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		allowed := map[string]bool{repo.OwnerID: true}
		for _, id := range repo.CollaboratorIDs {
			allowed[id] = true
		}
		for _, id := range in.ParticipantIDs {
			if !allowed[id] {
				writeJSON(w, 422, map[string]string{"error": "repository_participant_required"})
				return
			}
		}
		v, e := store.Control(string(repo.ID), r.PathValue("workspace"), a.UserID, in.Status, in.ParticipantIDs)
		writeDebugging(w, v, e, 200)
	})
}
func debuggingVisible(v dw.Workspace, actor string) bool {
	if v.Audience == "repository" {
		return true
	}
	for _, p := range v.ParticipantIDs {
		if p == actor {
			return true
		}
	}
	return false
}
func debuggingProject(v dw.Workspace, actor string) dw.Workspace {
	participant := false
	for _, p := range v.ParticipantIDs {
		participant = participant || p == actor
	}
	if participant {
		return v
	}
	e := v.PermittedEvidence[:0]
	for _, x := range v.PermittedEvidence {
		if x.Audience == "repository" {
			e = append(e, x)
		}
	}
	v.PermittedEvidence = e
	return v
}
func writeDebugging(w http.ResponseWriter, v dw.Workspace, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, dw.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else if errors.Is(e, dw.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_debugging_workspace"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
