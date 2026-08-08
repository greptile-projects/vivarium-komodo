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
