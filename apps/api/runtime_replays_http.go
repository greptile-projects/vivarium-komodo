package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeinvestigations"
	rp "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeprobes"
	rr "github.com/greptile-projects/vivarium-komodo/apps/api/runtimereplays"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

func registerRuntimeReplaysHTTP(mux *http.ServeMux, store *rr.Store, debugging *dw.Store, probes *rp.Store, investigations *ri.Store, isolated *workspaces.Store, previewStore *previews.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/debugging-workspaces/{workspace}/replays"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		workspace, e := debugging.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingVisible(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		xs, e := store.List(string(repo.ID), workspace.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := xs[:0]
		for _, v := range xs {
			if v.Audience == "repository" || containsString(v.ParticipantIDs, a.UserID) {
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
		workspace, e := debugging.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingParticipant(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in rr.CreateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		in.Revision = workspace.SourceRevision
		allowed := map[string]bool{}
		ps, _ := probes.List(string(repo.ID), workspace.ID)
		for _, p := range ps {
			for _, c := range p.Captures {
				allowed[c.ID] = true
			}
		}
		if in.InvestigationID != "" {
			v, x := investigations.Get(string(repo.ID), in.InvestigationID)
			if x != nil || v.WorkspaceID != workspace.ID || !runtimeInvestigationVisible(v, a.UserID) {
				writeJSON(w, 422, map[string]string{"error": "invalid_replay_evidence"})
				return
			}
			for _, ev := range v.Evidence {
				if ev.Accessible {
					allowed[ev.ID] = true
				}
			}
		}
		for _, id := range in.EvidenceIDs {
			if !allowed[id] {
				writeJSON(w, 422, map[string]string{"error": "invalid_replay_evidence"})
				return
			}
		}
		v, e := store.Create(string(repo.ID), workspace.ID, a.UserID, in)
		writeRuntimeReplay(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{replay}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("replay"))
		if e != nil || v.WorkspaceID != r.PathValue("workspace") || (v.Audience != "repository" && !containsString(v.ParticipantIDs, a.UserID)) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{replay}/refinements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Summary string `json:"summary"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := store.Refine(string(repo.ID), r.PathValue("replay"), a.UserID, in.Summary)
		writeRuntimeReplay(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{replay}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in rr.AttemptInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		if in.TargetKind == "workspace" {
			target, err := isolated.Get(string(repo.ID), in.TargetID)
			if err != nil {
				in.Blockers = append(in.Blockers, "missing_dependency")
			} else if target.Revision != in.Revision {
				in.Blockers = append(in.Blockers, "changed_revision")
			}
		} else if in.TargetKind == "preview" {
			target, err := previewStore.GetByID(in.TargetID)
			if err != nil || target.RepositoryID != string(repo.ID) {
				in.Blockers = append(in.Blockers, "missing_dependency")
			} else if target.Revision != in.Revision {
				in.Blockers = append(in.Blockers, "changed_revision")
			}
		}
		v, e := store.Attempt(string(repo.ID), r.PathValue("replay"), a.UserID, in)
		writeRuntimeReplay(w, v, e, 201)
	})
}
func writeRuntimeReplay(w http.ResponseWriter, v rr.Scenario, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	switch {
	case errors.Is(e, rr.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, rr.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
	case errors.Is(e, rr.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_runtime_replay"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
