package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/incidents"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type incidentStore interface {
	Create(incidents.CreateInput) (incidents.Incident, error)
	Get(string, string) (incidents.Incident, error)
	List(string) ([]incidents.Incident, error)
	Update(string, string, incidents.UpdateInput) (incidents.Incident, error)
	AddUpdate(string, string, string, string, string) (incidents.Incident, error)
	Follow(string, string, string, bool) (incidents.Incident, error)
	Acknowledge(string, string, string, int64) (incidents.Incident, error)
	AddEvidence(string, string, string, incidents.Evidence) (incidents.Incident, error)
	AddFinding(string, string, string, incidents.Finding) (incidents.Incident, error)
	StartInvestigation(string, string, incidents.InvestigationInput) (incidents.Incident, string, error)
	InvestigationContext(string) (incidents.Incident, incidents.Investigation, error)
	AddInvestigationRecord(string, string, string, string, []string) (incidents.Incident, incidents.Investigation, error)
	ControlInvestigation(string, string, string, string, string, string) (incidents.Incident, error)
	ProposeMitigation(string, string, incidents.MitigationInput) (incidents.Incident, error)
	CommentMitigation(string, string, string, string, string) (incidents.Incident, error)
	DecideMitigation(string, string, string, string, string, string, bool) (incidents.Incident, error)
	RecordMitigationAttempt(string, string, string, string, string, string, string, string) (incidents.Incident, error)
	VerifyMitigation(string, string, string, string, []incidents.RecoveryCriterion) (incidents.Incident, error)
}

func registerIncidentsHTTP(mux *http.ServeMux, store incidentStore, deployments deploymentStore, releases releaseStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("GET /repositories/{repository}/incidents", listIncidents(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents", createIncident(store, deployments, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/incidents/{incident}", getIncident(store, repositories, credentials))
	mux.HandleFunc("PATCH /repositories/{repository}/incidents/{incident}", updateIncident(store, deployments, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/updates", addIncidentUpdate(store, repositories, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/incidents/{incident}/follow", followIncident(store, repositories, credentials, true))
	mux.HandleFunc("DELETE /repositories/{repository}/incidents/{incident}/follow", followIncident(store, repositories, credentials, false))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/acknowledgements", acknowledgeIncident(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/evidence", addIncidentEvidence(store, deployments, releases, pulls, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/findings", addIncidentFinding(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/investigations", startIncidentInvestigation(store, deployments, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/investigations/{session}/control", controlIncidentInvestigation(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/mitigations", proposeIncidentMitigation(store, deployments, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/mitigations/{mitigation}/comments", commentIncidentMitigation(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/mitigations/{mitigation}/decision", decideIncidentMitigation(store, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/mitigations/{mitigation}/execution", executeIncidentMitigation(store, deployments, pulls, repositories, credentials))
	mux.HandleFunc("POST /repositories/{repository}/incidents/{incident}/mitigations/{mitigation}/verification", verifyIncidentMitigation(store, deployments, repositories, credentials))
	mux.HandleFunc("GET /incident-investigations/context", incidentInvestigationContext(store))
	mux.HandleFunc("GET /incident-investigations/operational/{resource}", incidentInvestigationOperational(store, deployments))
	mux.HandleFunc("POST /incident-investigations/records", addIncidentInvestigationRecord(store))
}
func proposeIncidentMitigation(store incidentStore, deployments deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, err := store.Get(repo, r.PathValue("incident"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Kind             string                        `json:"kind"`
			Title            string                        `json:"title"`
			Description      string                        `json:"description"`
			RepositoryID     string                        `json:"repository_id"`
			EnvironmentID    string                        `json:"environment_id"`
			DeploymentID     string                        `json:"deployment_id"`
			EvidenceIDs      []string                      `json:"evidence_ids"`
			RecoveryCriteria []incidents.RecoveryCriterion `json:"recovery_criteria"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if in.RepositoryID == "" {
			in.RepositoryID = repo
		}
		affected := false
		for _, a := range current.Affected {
			if a.RepositoryID == in.RepositoryID && a.EnvironmentID == in.EnvironmentID {
				affected = true
			}
		}
		if !affected {
			writeJSON(w, 422, map[string]string{"error": "invalid_mitigation_scope"})
			return
		}
		if in.DeploymentID != "" {
			d, e := deployments.GetDeployment(in.RepositoryID, in.DeploymentID)
			if e != nil || d.EnvironmentID != in.EnvironmentID {
				writeJSON(w, 422, map[string]string{"error": "invalid_mitigation_target"})
				return
			}
		}
		item, e := store.ProposeMitigation(repo, current.ID, incidents.MitigationInput{ActorID: actor, Kind: in.Kind, Title: in.Title, Description: in.Description, RepositoryID: in.RepositoryID, EnvironmentID: in.EnvironmentID, DeploymentID: in.DeploymentID, EvidenceIDs: in.EvidenceIDs, RecoveryCriteria: in.RecoveryCriteria})
		writeIncidentResult(w, item, e, 201)
	}
}
func commentIncidentMitigation(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, e := store.CommentMitigation(repo, r.PathValue("incident"), r.PathValue("mitigation"), actor, in.Body)
		writeIncidentResult(w, item, e, 201)
	}
}
func decideIncidentMitigation(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, e := store.Get(repo, r.PathValue("incident"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
			Override bool   `json:"override"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		for _, m := range current.Mitigations {
			if m.ID == r.PathValue("mitigation") && m.ProposedByID == actor && !in.Override {
				writeJSON(w, 409, map[string]string{"error": "independent_authorization_required"})
				return
			}
		}
		if in.Override && current.Roles["commander"] != actor {
			writeJSON(w, 403, map[string]string{"error": "commander_override_required"})
			return
		}
		item, e := store.DecideMitigation(repo, current.ID, r.PathValue("mitigation"), actor, in.Decision, in.Reason, in.Override)
		writeIncidentResult(w, item, e, 200)
	}
}
func executeIncidentMitigation(store incidentStore, deployments deploymentStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, e := store.Get(repo, r.PathValue("incident"))
		if e != nil || !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Outcome      string `json:"outcome"`
			ResourceType string `json:"resource_type"`
			ResourceID   string `json:"resource_id"`
			Message      string `json:"message"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		var target *incidents.Mitigation
		for x := range current.Mitigations {
			if current.Mitigations[x].ID == r.PathValue("mitigation") {
				target = &current.Mitigations[x]
			}
		}
		if target == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if in.Outcome == "started" && target.Kind == "pause_rollout" {
			d, er := deployments.Control(target.RepositoryID, target.DeploymentID, actor, "pause", in.Message)
			if er != nil {
				item, _ := store.RecordMitigationAttempt(repo, current.ID, target.ID, actor, "failed", "deployment", target.DeploymentID, "Deployment pause was rejected: "+er.Error())
				writeJSON(w, 409, item)
				return
			}
			in.ResourceType, in.ResourceID = "deployment", d.ID
		}
		if in.ResourceType == "deployment" {
			d, er := deployments.GetDeployment(target.RepositoryID, in.ResourceID)
			if er != nil || (target.Kind == "restore_release" && (d.RecoveryOfID != target.DeploymentID || d.RecoveryAction != "rollback")) {
				writeJSON(w, 422, map[string]string{"error": "invalid_execution_resource"})
				return
			}
		} else if in.ResourceType == "pull_request" {
			p, er := pulls.Get(target.RepositoryID, in.ResourceID)
			if er != nil || target.Kind != "emergency_repair" || !p.Draft || p.SourceCommitID == "" {
				writeJSON(w, 422, map[string]string{"error": "invalid_execution_resource"})
				return
			}
		} else {
			writeJSON(w, 422, map[string]string{"error": "invalid_execution_resource"})
			return
		}
		item, e := store.RecordMitigationAttempt(repo, current.ID, target.ID, actor, in.Outcome, in.ResourceType, in.ResourceID, in.Message)
		writeIncidentResult(w, item, e, 200)
	}
}
func verifyIncidentMitigation(store incidentStore, deployments deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, e := store.Get(repo, r.PathValue("incident"))
		if e != nil || !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Results []incidents.RecoveryCriterion `json:"results"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		for x := range in.Results {
			result := &in.Results[x]
			d, er := deployments.GetDeployment(repo, result.DeploymentID)
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_recovery_evidence"})
				return
			}
			found := false
			for _, event := range d.Events {
				if event.Sequence == result.EventSequence && event.Signal != "" {
					result.Outcome = event.Outcome
					found = true
				}
			}
			if !found {
				writeJSON(w, 422, map[string]string{"error": "invalid_recovery_evidence"})
				return
			}
		}
		item, e := store.VerifyMitigation(repo, current.ID, r.PathValue("mitigation"), actor, in.Results)
		writeIncidentResult(w, item, e, 200)
	}
}
func incidentAccess(w http.ResponseWriter, r *http.Request, repositories pullRequestRepositoryStore, credentials authStore, write bool) (string, string, bool) {
	scope := auth.RepositoryRead
	if write {
		scope = auth.RepositoryWrite
	}
	repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, scope, write)
	if !ok {
		return "", "", false
	}
	if write && actor.UserID != repo.OwnerID {
		participant, _ := repositories.IsCollaborator(repo.ID, actor.UserID)
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return "", "", false
		}
	}
	return string(repo.ID), actor.UserID, true
}
func listIncidents(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, false)
		if !ok {
			return
		}
		items, err := store.List(repo)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		state := r.URL.Query().Get("status")
		if state != "" {
			filtered := items[:0]
			for _, i := range items {
				if i.Status == state {
					filtered = append(filtered, i)
				}
			}
			items = filtered
		}
		participant := incidentParticipant(repositories, repo, actor)
		for x := range items {
			items[x] = visibleIncident(items[x], participant)
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func getIncident(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, false)
		if !ok {
			return
		}
		item, err := store.Get(repo, r.PathValue("incident"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, visibleIncident(item, incidentParticipant(repositories, repo, actor)))
	}
}
func addIncidentEvidence(store incidentStore, deployments deploymentStore, releases releaseStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in incidents.Evidence
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		if in.RepositoryID == "" {
			in.RepositoryID = repo
		}
		if !validateEvidenceSource(in, actor, deployments, releases, pulls, repositories, store) {
			writeJSON(w, 422, map[string]string{"error": "invalid_evidence_source"})
			return
		}
		item, err := store.AddEvidence(repo, r.PathValue("incident"), actor, in)
		writeIncidentResult(w, item, err, 201)
	}
}
func addIncidentFinding(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in incidents.Finding
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.AddFinding(repo, r.PathValue("incident"), actor, in)
		writeIncidentResult(w, item, err, 201)
	}
}
func startIncidentInvestigation(store incidentStore, deployments deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, err := store.Get(repo, r.PathValue("incident"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Agent             string                        `json:"agent"`
			Mandate           string                        `json:"mandate"`
			EvidenceIDs       []string                      `json:"evidence_ids"`
			Revisions         []incidents.Revision          `json:"revisions"`
			OperationalAccess []incidents.OperationalAccess `json:"operational_access"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		for _, rev := range in.Revisions {
			if !incidentAffected(current, rev.RepositoryID) {
				writeJSON(w, 422, map[string]string{"error": "invalid_revision"})
				return
			}
			repository, er := repositories.Open(storage.ID(rev.RepositoryID))
			if er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_revision"})
				return
			}
			if _, er = repository.ReadCommit(storage.ObjectID(rev.CommitID)); er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_revision"})
				return
			}
		}
		for _, access := range in.OperationalAccess {
			if _, er := deployments.GetDeployment(access.RepositoryID, access.ResourceID); er != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_operational_access"})
				return
			}
			if !incidentAffected(current, access.RepositoryID) {
				writeJSON(w, 422, map[string]string{"error": "invalid_operational_access"})
				return
			}
		}
		item, token, er := store.StartInvestigation(repo, current.ID, incidents.InvestigationInput{ActorID: actor, Agent: in.Agent, Mandate: in.Mandate, EvidenceIDs: in.EvidenceIDs, Revisions: in.Revisions, OperationalAccess: in.OperationalAccess})
		if er != nil {
			writeIncidentResult(w, item, er, 201)
			return
		}
		scrubInvestigationCredentials(&item)
		writeJSON(w, 201, map[string]any{"incident": item, "worker_credential": token, "credential_notice": "shown once; read-only incident investigation access; no deployment, secret, repository-write, or Git authority"})
	}
}
func controlIncidentInvestigation(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		current, err := store.Get(repo, r.PathValue("incident"))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !incidentResponder(current, actor) {
			writeJSON(w, 403, map[string]string{"error": "responder_required"})
			return
		}
		var in struct {
			Action  string `json:"action"`
			Message string `json:"message"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.ControlInvestigation(repo, current.ID, r.PathValue("session"), actor, in.Action, in.Message)
		writeIncidentResult(w, item, err, 200)
	}
}
func incidentInvestigationContext(store incidentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := investigationBearer(r)
		if token == "" {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		item, inv, err := store.InvestigationContext(token)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		selected := []incidents.Evidence{}
		allowed := map[string]bool{}
		for _, id := range inv.EvidenceIDs {
			allowed[id] = true
		}
		for _, e := range item.Evidence {
			if allowed[e.ID] {
				selected = append(selected, e)
			}
		}
		inv.CredentialDigest = ""
		writeJSON(w, 200, map[string]any{"incident": map[string]any{"id": item.ID, "repository_id": item.RepositoryID, "title": item.Title, "summary": item.Summary, "severity": item.Severity, "status": item.Status}, "investigation": inv, "evidence": selected})
	}
}
func incidentInvestigationOperational(store incidentStore, deployments deploymentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := investigationBearer(r)
		if token == "" {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		_, inv, err := store.InvestigationContext(token)
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		if inv.State != "running" {
			writeJSON(w, 409, map[string]string{"error": "investigation_not_running"})
			return
		}
		var allowed *incidents.OperationalAccess
		for x := range inv.OperationalAccess {
			if inv.OperationalAccess[x].ResourceID == r.PathValue("resource") {
				allowed = &inv.OperationalAccess[x]
				break
			}
		}
		if allowed == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		deployment, err := deployments.GetDeployment(allowed.RepositoryID, allowed.ResourceID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		// Deployment resources contain redacted execution evidence and identifiers,
		// never environment secret values. The worker route deliberately has no
		// mutation counterpart.
		writeJSON(w, 200, map[string]any{"access": allowed, "deployment": deployment})
	}
}
func addIncidentInvestigationRecord(store incidentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := investigationBearer(r)
		if token == "" {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		var in struct {
			Type        string   `json:"type"`
			Message     string   `json:"message"`
			Uncertainty string   `json:"uncertainty"`
			EvidenceIDs []string `json:"evidence_ids"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		_, inv, err := store.AddInvestigationRecord(token, in.Type, in.Message, in.Uncertainty, in.EvidenceIDs)
		inv.CredentialDigest = ""
		switch {
		case errors.Is(err, incidents.ErrNotFound):
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
		case errors.Is(err, incidents.ErrTransition):
			writeJSON(w, 409, map[string]string{"error": "investigation_not_running"})
		case errors.Is(err, incidents.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_record"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		default:
			writeJSON(w, 201, inv)
		}
	}
}
func investigationBearer(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(v) > 7 && strings.EqualFold(v[:7], "Bearer ") {
		return strings.TrimSpace(v[7:])
	}
	return ""
}
func incidentResponder(i incidents.Incident, actor string) bool {
	if i.DeclaredByID == actor {
		return true
	}
	for _, id := range i.Roles {
		if id == actor {
			return true
		}
	}
	return false
}
func incidentAffected(i incidents.Incident, repo string) bool {
	for _, a := range i.Affected {
		if a.RepositoryID == repo {
			return true
		}
	}
	return false
}
func validateEvidenceSource(e incidents.Evidence, actor string, deployments deploymentStore, releases releaseStore, pulls pullRequestStore, repositories pullRequestRepositoryStore, incidentStore incidentStore) bool {
	repository, err := repositories.Inspect(storage.ID(e.RepositoryID))
	if err != nil {
		return false
	}
	if repository.Visibility != "public" && repository.OwnerID != actor {
		allowed, _ := repositories.IsCollaborator(repository.ID, actor)
		if !allowed {
			return false
		}
	}
	switch e.Kind {
	case "logs", "health_signal", "deployment":
		d, err := deployments.GetDeployment(e.RepositoryID, e.ResourceID)
		if err != nil {
			return false
		}
		if e.Kind == "health_signal" {
			for _, event := range d.Events {
				if event.Sequence == e.EventSequence && event.Signal != "" {
					return true
				}
			}
			return false
		}
		return e.Kind != "logs" || (e.StartAt != nil && e.EndAt != nil)
	case "release":
		_, err := releases.Get(e.RepositoryID, e.ResourceID)
		return err == nil
	case "pull_request":
		_, err := pulls.Get(e.RepositoryID, e.ResourceID)
		return err == nil
	case "incident":
		_, err := incidentStore.Get(e.RepositoryID, e.ResourceID)
		return err == nil
	case "commit":
		repository, err := repositories.Open(storage.ID(e.RepositoryID))
		if err != nil {
			return false
		}
		_, err = repository.ReadCommit(storage.ObjectID(e.ResourceID))
		return err == nil
	default:
		return false
	}
}
func incidentParticipant(repositories pullRequestRepositoryStore, repositoryID, actor string) bool {
	if actor == "" {
		return false
	}
	repo, err := repositories.Inspect(storage.ID(repositoryID))
	if err != nil {
		return false
	}
	if repo.OwnerID == actor {
		return true
	}
	ok, _ := repositories.IsCollaborator(repo.ID, actor)
	return ok
}
func visibleIncident(i incidents.Incident, participant bool) incidents.Incident {
	scrubInvestigationCredentials(&i)
	if participant {
		return i
	}
	evidence := i.Evidence[:0]
	for _, item := range i.Evidence {
		if item.Audience == "public" {
			evidence = append(evidence, item)
		}
	}
	i.Evidence = evidence
	findings := i.Findings[:0]
	for _, item := range i.Findings {
		if item.Audience == "public" {
			findings = append(findings, item)
		}
	}
	i.Findings = findings
	i.Investigations = nil
	timeline := i.Timeline[:0]
	for _, item := range i.Timeline {
		if item.Audience == "" || item.Audience == "public" {
			timeline = append(timeline, item)
		}
	}
	i.Timeline = timeline
	i.Followers = nil
	i.Acknowledgements = nil
	return i
}
func scrubInvestigationCredentials(i *incidents.Incident) {
	for x := range i.Investigations {
		i.Investigations[x].CredentialDigest = ""
	}
}
func createIncident(store incidentStore, deploymentStore deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in struct {
			Title        string                          `json:"title"`
			Summary      string                          `json:"summary"`
			Severity     string                          `json:"severity"`
			Roles        map[string]string               `json:"roles"`
			Affected     []incidents.AffectedEnvironment `json:"affected"`
			SourceSignal *incidents.SourceSignal         `json:"source_signal"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !validateIncidentContext(w, repo, actor, in.Roles, in.Affected, in.SourceSignal, deploymentStore, repositories) {
			return
		}
		item, err := store.Create(incidents.CreateInput{RepositoryID: repo, ActorID: actor, Title: in.Title, Summary: in.Summary, Severity: in.Severity, Roles: in.Roles, Affected: in.Affected, SourceSignal: in.SourceSignal})
		writeIncidentResult(w, item, err, 201)
	}
}
func updateIncident(store incidentStore, deploymentStore deploymentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in struct {
			Summary  string                          `json:"summary"`
			Severity string                          `json:"severity"`
			Status   string                          `json:"status"`
			Roles    map[string]string               `json:"roles"`
			Affected []incidents.AffectedEnvironment `json:"affected"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if in.Roles != nil || in.Affected != nil {
			current, err := store.Get(repo, r.PathValue("incident"))
			if err != nil {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
			roles, affected := in.Roles, in.Affected
			if roles == nil {
				roles = current.Roles
			}
			if affected == nil {
				affected = current.Affected
			}
			if !validateIncidentContext(w, repo, actor, roles, affected, nil, deploymentStore, repositories) {
				return
			}
		}
		item, err := store.Update(repo, r.PathValue("incident"), incidents.UpdateInput{ActorID: actor, Summary: in.Summary, Severity: in.Severity, Status: in.Status, Roles: in.Roles, Affected: in.Affected})
		writeIncidentResult(w, item, err, 200)
	}
}
func addIncidentUpdate(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in struct {
			Audience string `json:"audience"`
			Message  string `json:"message"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.AddUpdate(repo, r.PathValue("incident"), actor, in.Audience, in.Message)
		writeIncidentResult(w, item, err, 201)
	}
}
func followIncident(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore, following bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		item, err := store.Follow(repo, r.PathValue("incident"), actor, following)
		writeIncidentResult(w, item, err, 200)
	}
}
func acknowledgeIncident(store incidentStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := incidentAccess(w, r, repositories, credentials, true)
		if !ok {
			return
		}
		var in struct {
			UpdateSequence int64 `json:"update_sequence"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		item, err := store.Acknowledge(repo, r.PathValue("incident"), actor, in.UpdateSequence)
		writeIncidentResult(w, item, err, 201)
	}
}
func writeIncidentResult(w http.ResponseWriter, item incidents.Incident, err error, status int) {
	scrubInvestigationCredentials(&item)
	switch {
	case errors.Is(err, incidents.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, incidents.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_incident"})
	case errors.Is(err, incidents.ErrTransition):
		writeJSON(w, 409, map[string]string{"error": "incident_transition_conflict"})
	case err != nil:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	default:
		writeJSON(w, status, item)
	}
}
func validateIncidentContext(w http.ResponseWriter, anchor, actor string, roles map[string]string, affected []incidents.AffectedEnvironment, source *incidents.SourceSignal, deploymentStore deploymentStore, repositories pullRequestRepositoryStore) bool {
	participants := map[string]bool{}
	for _, a := range affected {
		repo, err := repositories.Inspect(storage.ID(a.RepositoryID))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_affected_scope"})
			return false
		}
		allowed := repo.OwnerID == actor
		if !allowed {
			allowed, _ = repositories.IsCollaborator(repo.ID, actor)
		}
		if !allowed {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return false
		}
		participants[a.RepositoryID] = true
		if a.EnvironmentID != "" {
			if _, err = deploymentStore.GetEnvironment(a.RepositoryID, a.EnvironmentID); err != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_affected_scope"})
				return false
			}
		}
	}
	if len(affected) > 0 && !participants[anchor] {
		writeJSON(w, 422, map[string]string{"error": "anchor_repository_required"})
		return false
	}
	for _, user := range roles {
		if strings.TrimSpace(user) == "" {
			continue
		}
		valid := false
		for repositoryID := range participants {
			repo, _ := repositories.Inspect(storage.ID(repositoryID))
			if repo.OwnerID == user {
				valid = true
				break
			}
			valid, _ = repositories.IsCollaborator(repo.ID, user)
			if valid {
				break
			}
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_response_role"})
			return false
		}
	}
	if source != nil {
		if source.RepositoryID != anchor || !participants[source.RepositoryID] || source.EventSequence < 1 {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_signal"})
			return false
		}
		d, err := deploymentStore.GetDeployment(source.RepositoryID, source.DeploymentID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_signal"})
			return false
		}
		found := false
		for _, e := range d.Events {
			if e.Sequence == source.EventSequence && e.Signal != "" && (e.Outcome == "failed" || e.Outcome == "unhealthy") {
				source.Stage, source.Signal, source.Outcome = e.Stage, e.Signal, e.Outcome
				found = true
			}
		}
		if !found {
			writeJSON(w, 422, map[string]string{"error": "invalid_source_signal"})
			return false
		}
	}
	return true
}
