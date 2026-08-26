package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsealerts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responsepolicies"
	"github.com/greptile-projects/vivarium-komodo/apps/api/responserotations"
)

func registerResponseAlertsHTTP(mux *http.ServeMux, s *responsealerts.Store, policies *responsepolicies.Store, rotations *responserotations.Store, incidentStore incidentStore, repos performanceRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/response-alerts"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.List(string(repo.ID), r.URL.Query().Get("recipient"))
		if !responseAlertError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID string `json:"policy_id"`
			responsealerts.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		p, e := policies.Get(string(repo.ID), in.PolicyID)
		if responseAlertDependencyError(w, e) {
			return
		}
		rs, e := rotations.List(string(repo.ID))
		if responseAlertDependencyError(w, e) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in.Input, p, rs)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{alert}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("alert"))
		if !responseAlertError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/routing-attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.AttemptInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RecordAttempt(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/workspace", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.WorkspaceInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.OpenWorkspace(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/workspace/actions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.WorkspaceActionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Act(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/workspace/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.DiagnosticInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RunDiagnostic(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{alert}/workspace/agents", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in responsealerts.AgentInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, token, e := s.StartAgent(string(repo.ID), r.PathValue("alert"), a.UserID, in)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, map[string]any{"alert": x, "credential": token})
		}
	})
	mux.HandleFunc("GET /response-alert-investigations/context", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		a, x, e := s.AgentContext(token)
		if e != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_investigation_credential"})
			return
		}
		permitted := []responsealerts.ContextReference{}
		for _, c := range a.Workspace.Context {
			for _, id := range x.ContextReferences {
				if c.ResourceID == id && c.Permitted {
					permitted = append(permitted, c)
				}
			}
		}
		writeJSON(w, 200, map[string]any{"signal": a.Signal, "investigation": x, "context": permitted, "authority": []string{"read selected context", "publish findings, questions, and uncertainty"}})
	})
	mux.HandleFunc("POST /response-alert-investigations/records", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		var in struct {
			Kind               string   `json:"kind"`
			Body               string   `json:"body"`
			EvidenceReferences []string `json:"evidence_references"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		a, x, e := s.AddAgentRecord(token, in.Kind, in.Body, in.EvidenceReferences)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_investigation_record"})
			return
		}
		writeJSON(w, 201, map[string]any{"alert": a, "investigation": x})
	})
	mux.HandleFunc("POST "+base+"/{alert}/workspace/incident", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64                           `json:"expected_revision"`
			Title            string                          `json:"title"`
			Summary          string                          `json:"summary"`
			Severity         string                          `json:"severity"`
			Roles            map[string]string               `json:"roles"`
			Affected         []incidents.AffectedEnvironment `json:"affected"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		alert, e := s.Get(string(repo.ID), r.PathValue("alert"))
		if e != nil || alert.Workspace == nil || !containsString(alert.Workspace.Participants, a.UserID) || alert.Revision != in.ExpectedRevision {
			responseAlertError(w, responsealerts.ErrInvalid)
			return
		}
		incident, e := incidentStore.Create(incidents.CreateInput{RepositoryID: string(repo.ID), ActorID: a.UserID, Title: in.Title, Summary: in.Summary + "\n\nPromoted from response alert " + alert.ID + " at revision " + alert.Signal.Revision, Severity: in.Severity, Roles: in.Roles, Affected: in.Affected})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_incident"})
			return
		}
		linked, e := s.LinkIncident(string(repo.ID), alert.ID, a.UserID, incident.ID, in.ExpectedRevision)
		if !responseAlertError(w, e) {
			writeJSON(w, 201, map[string]any{"alert": linked, "incident": incident})
		}
	})
}
func responseAlertDependencyError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, responsepolicies.ErrNotFound) {
		writeJSON(w, 422, map[string]string{"error": "active_response_policy_not_found"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
func responseAlertError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, responsealerts.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "response_alert_not_found"})
	case errors.Is(e, responsealerts.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_response_alert"})
	case errors.Is(e, responsealerts.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "response_alert_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
