package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	dw "github.com/greptile-projects/vivarium-komodo/apps/api/debuggingworkspaces"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeinvestigations"
	rp "github.com/greptile-projects/vivarium-komodo/apps/api/runtimeprobes"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerRuntimeInvestigationsHTTP(mux *http.ServeMux, store *ri.Store, workspaces *dw.Store, probes *rp.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/debugging-workspaces/{workspace}/investigations"
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
		xs, e := store.List(string(repo.ID), workspace.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := xs[:0]
		for _, v := range xs {
			if runtimeInvestigationVisible(v, a.UserID) {
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
		workspace, e := workspaces.Get(string(repo.ID), r.PathValue("workspace"))
		if e != nil || !debuggingParticipant(workspace, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in ri.CreateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		in.WorkspaceID = workspace.ID
		in.Revision = workspace.SourceRevision
		allowed := map[string]rp.Probe{}
		xs, _ := probes.List(string(repo.ID), workspace.ID)
		for _, p := range xs {
			allowed[p.ID] = p
		}
		for i := range in.Evidence {
			ev := &in.Evidence[i]
			p, ok := allowed[ev.ProbeID]
			if !ok {
				writeJSON(w, 422, map[string]string{"error": "invalid_runtime_evidence"})
				return
			}
			ev.Kind = p.Kind
			if ev.Audience == "" {
				ev.Audience = p.Preview.Audience
			}
			found := false
			if ev.CaptureID != "" {
				for _, c := range p.Captures {
					found = found || c.ID == ev.CaptureID
				}
				if !found {
					writeJSON(w, 422, map[string]string{"error": "invalid_runtime_evidence"})
					return
				}
			}
			ev.Accessible = found
			if !found {
				ev.Reason = "no selected sanitized capture is accessible"
			} else {
				ev.Reason = ""
			}
			if ev.Audience == "participants" && in.Audience == "repository" {
				writeJSON(w, 422, map[string]string{"error": "evidence_audience_too_narrow"})
				return
			}
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range in.Correlations {
			c := &in.Correlations[i]
			if (c.Kind == "symbol" || c.Kind == "commit") && c.Revision != workspace.SourceRevision {
				writeJSON(w, 422, map[string]string{"error": "correlation_revision_mismatch"})
				return
			}
			if c.Path != "" {
				if !validInvestigationPath(c.Path) {
					writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
					return
				}
				if _, e = blobAtPath(opened, storage.ObjectID(workspace.SourceRevision), c.Path); e != nil {
					writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
					return
				}
			}
		}
		v, e := store.Create(string(repo.ID), a.UserID, in)
		writeRuntimeInvestigation(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{investigation}", runtimeInvestigationGet(store, workspaces, repos, credentials))
	mux.HandleFunc("GET "+base+"/{investigation}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || !runtimeInvestigationVisible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		for _, event := range v.Events {
			if event.Sequence <= after {
				continue
			}
			b, _ := json.Marshal(event)
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", event.Sequence, b)
		}
	})
	mux.HandleFunc("POST "+base+"/{investigation}/claims", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.Claim
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.AddClaim(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRuntimeInvestigation(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/owner-requests", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in ri.OwnerRequest
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := store.RequestOwner(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRuntimeInvestigation(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{investigation}/agents", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			AgentID        string    `json:"agent_id"`
			Mandate        string    `json:"mandate"`
			EvidenceIDs    []string  `json:"evidence_ids"`
			CorrelationIDs []string  `json:"correlation_ids"`
			ExpiresAt      time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, token, e := store.StartAgent(string(repo.ID), r.PathValue("investigation"), a.UserID, in.AgentID, in.Mandate, in.EvidenceIDs, in.CorrelationIDs, in.ExpiresAt)
		if e != nil {
			writeRuntimeInvestigation(w, v, e, 201)
			return
		}
		writeJSON(w, 201, map[string]any{"investigation": v, "credential": token, "authority": []string{"runtime-investigation:read", "runtime-investigation:append"}})
	})
	mux.HandleFunc("POST "+base+"/{investigation}/agents/{session}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Action   string `json:"action"`
			Guidance string `json:"guidance"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := store.ControlAgent(string(repo.ID), r.PathValue("investigation"), r.PathValue("session"), a.UserID, in.Action, in.Guidance)
		writeRuntimeInvestigation(w, v, e, 200)
	})
	mux.HandleFunc("GET /runtime-investigation-agent/context", func(w http.ResponseWriter, r *http.Request) {
		v, a, e := store.AgentContext(agentBearer(r))
		if e != nil {
			writeJSON(w, 403, map[string]string{"error": "agent_session_forbidden"})
			return
		}
		evidence := v.Evidence[:0]
		for _, x := range v.Evidence {
			if containsString(a.EvidenceIDs, x.ID) {
				evidence = append(evidence, x)
			}
		}
		corr := v.Correlations[:0]
		for _, x := range v.Correlations {
			if containsString(a.CorrelationIDs, x.ID) {
				corr = append(corr, x)
			}
		}
		v.Evidence = evidence
		v.Correlations = corr
		v.AgentSessions = []ri.AgentSession{a}
		writeJSON(w, 200, map[string]any{"investigation": v, "session": a, "authority": []string{"runtime-investigation:read", "runtime-investigation:append"}})
	})
	mux.HandleFunc("POST /runtime-investigation-agent/claims", func(w http.ResponseWriter, r *http.Request) {
		var in ri.Claim
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := store.AgentClaim(agentBearer(r), in)
		writeRuntimeInvestigation(w, v, e, 201)
	})
}

func runtimeInvestigationGet(store *ri.Store, workspaces *dw.Store, repos dataFlowRepositories, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := store.Get(string(repo.ID), r.PathValue("investigation"))
		if e != nil || v.WorkspaceID != r.PathValue("workspace") || !runtimeInvestigationVisible(v, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func runtimeInvestigationVisible(v ri.Investigation, actor string) bool {
	if v.Audience == "repository" {
		return true
	}
	return containsString(v.Participants, actor)
}
func agentBearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func writeRuntimeInvestigation(w http.ResponseWriter, v ri.Investigation, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	switch {
	case errors.Is(e, ri.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, ri.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "forbidden"})
	case errors.Is(e, ri.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_runtime_investigation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
