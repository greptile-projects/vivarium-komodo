package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	rp "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeprobes"
)

func registerRuntimeProbesHTTP(mux *http.ServeMux, probes *rp.Store, workspaces *dw.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/debugging-workspaces/{workspace}/probes"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingVisible(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		xs, e := probes.List(string(repo.ID), workspace.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := []rp.Probe{}
		for _, p := range xs {
			out = append(out, projectProbe(p, workspace, a.UserID))
		}
		writeJSON(w, 200, map[string]any{"items": out})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingParticipant(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in rp.RequestInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		in.WorkspaceID = workspace.ID
		if in.Environment != workspace.Environment || !permittedProbeKind(workspace, in.Kind) {
			writeJSON(w, 422, map[string]string{"error": "probe_outside_workspace_scope"})
			return
		}
		if in.Kind == "dynamic_diagnostic" && in.Diagnostic.Revision != workspace.SourceRevision {
			writeJSON(w, 422, map[string]string{"error": "diagnostic_revision_mismatch"})
			return
		}
		p, e := probes.Request(string(repo.ID), a.UserID, in)
		writeProbe(w, p, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{probe}/decision", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !containsString(workspace.OwnerIDs, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "environment_owner_required"})
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		p, e := probes.Get(string(repo.ID), r.PathValue("probe"))
		if e != nil || p.WorkspaceID != workspace.ID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		p, e = probes.Decide(string(repo.ID), p.ID, a.UserID, in.Decision, in.Reason)
		writeProbe(w, p, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{probe}/captures", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingParticipant(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		p, e := probes.Get(string(repo.ID), r.PathValue("probe"))
		if e != nil || p.WorkspaceID != workspace.ID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in rp.CaptureInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		p, e = probes.Capture(string(repo.ID), p.ID, a.UserID, in)
		writeProbe(w, p, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{probe}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || (!containsString(workspace.OwnerIDs, a.UserID) && !debuggingParticipant(workspace, a.UserID)) {
			writeJSON(w, 403, map[string]string{"error": "probe_control_forbidden"})
			return
		}
		var in struct {
			Kind   string `json:"kind"`
			Detail string `json:"detail"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		p, e := probes.Get(string(repo.ID), r.PathValue("probe"))
		if e != nil || p.WorkspaceID != workspace.ID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if (in.Kind == "revoke" || in.Kind == "overload" || in.Kind == "narrow") && !containsString(workspace.OwnerIDs, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "environment_owner_required"})
			return
		}
		if in.Kind == "consent_revoked" && !containsString(p.ConsentActorIDs, a.UserID) {
			writeJSON(w, 403, map[string]string{"error": "consent_actor_required"})
			return
		}
		p, e = probes.Control(string(repo.ID), p.ID, a.UserID, in.Kind, in.Detail)
		writeProbe(w, p, e, 200)
	})
}
func debuggingParticipant(w dw.Workspace, actor string) bool {
	return containsString(w.ParticipantIDs, actor)
}
func permittedProbeKind(w dw.Workspace, kind string) bool {
	aliases := map[string][]string{"logs": {"logs", "log"}, "traces": {"traces", "trace"}, "profile": {"profile", "profiles"}, "state_snapshot": {"state_snapshot", "state snapshots", "snapshot"}, "dynamic_diagnostic": {"dynamic_diagnostic", "dynamic diagnostics", "diagnostic"}}
	for _, e := range w.PermittedEvidence {
		if e.Access != "permitted" {
			continue
		}
		for _, a := range aliases[kind] {
			if e.Kind == a {
				return true
			}
		}
	}
	return false
}
func projectProbe(p rp.Probe, w dw.Workspace, actor string) rp.Probe {
	if p.Preview.Audience == "participants" && !debuggingParticipant(w, actor) {
		p.Captures = nil
		p.Actions = nil
	}
	return p
}
func writeProbe(w http.ResponseWriter, p rp.Probe, e error, status int) {
	if e == nil {
		writeJSON(w, status, p)
		return
	}
	switch {
	case errors.Is(e, rp.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, rp.ErrStopped):
		writeJSON(w, 409, map[string]string{"error": "probe_stopped"})
	case errors.Is(e, rp.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_runtime_probe"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
