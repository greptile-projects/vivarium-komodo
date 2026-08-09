package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityreports"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type securityReportStore interface {
	Create(securityreports.CreateInput) (securityreports.Report, error)
	ListVisible(string, func(string) bool) ([]securityreports.Report, error)
	Get(string, string, func(string) bool) (securityreports.Report, error)
	Triage(string, securityreports.TriageInput, func(string) bool) (securityreports.Report, error)
	SetMember(string, string, string, bool, func(string) bool) (securityreports.Report, error)
	AddMessage(string, string, string, func(string) bool) (securityreports.Report, error)
}
type securityUserStore interface {
	Get(users.ID) (users.User, error)
}

func registerSecurityReportsHTTP(mux *http.ServeMux, store securityReportStore, catalog pullRequestRepositoryStore, userStore securityUserStore, credentials authStore) {
	mux.HandleFunc("POST /security-reports", createSecurityReport(store, catalog, credentials))
	mux.HandleFunc("GET /security-reports", listSecurityReports(store, catalog, credentials))
	mux.HandleFunc("GET /security-reports/{report}", getSecurityReport(store, catalog, credentials))
	mux.HandleFunc("PATCH /security-reports/{report}/triage", triageSecurityReport(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/team", addSecurityReportMember(store, catalog, userStore, credentials))
	mux.HandleFunc("DELETE /security-reports/{report}/team/{user}", removeSecurityReportMember(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/messages", addSecurityReportMessage(store, catalog, credentials))
}

func securityActor(w http.ResponseWriter, r *http.Request, credentials authStore) (string, bool) {
	grant, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
	if !ok {
		return "", false
	}
	return grant.UserID, true
}
func ownerCheck(catalog pullRequestRepositoryStore, actor string) func(string) bool {
	return func(id string) bool {
		repo, err := catalog.Inspect(storage.ID(id))
		return err == nil && repo.OwnerID == actor
	}
}
func canReportRepository(catalog pullRequestRepositoryStore, id, actor string) bool {
	repo, err := catalog.Inspect(storage.ID(id))
	if err != nil {
		return false
	}
	if repo.Visibility == repositories.Public || repo.OwnerID == actor {
		return true
	}
	ok, _ := catalog.IsCollaborator(repo.ID, actor)
	return ok
}

func createSecurityReport(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Title    string                               `json:"title"`
			Summary  string                               `json:"summary"`
			Contact  securityreports.Contact              `json:"contact"`
			Affected []securityreports.AffectedRepository `json:"affected_repositories"`
			Evidence []securityreports.Evidence           `json:"evidence"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		for _, a := range in.Affected {
			if !canReportRepository(catalog, a.RepositoryID, actor) {
				writeJSON(w, 422, map[string]string{"error": "invalid_affected_repository"})
				return
			}
		}
		item, err := store.Create(securityreports.CreateInput{ActorID: actor, Title: in.Title, Summary: in.Summary, Contact: in.Contact, Affected: in.Affected, Evidence: in.Evidence})
		writeSecurityReport(w, item, err, 201)
	}
}
func listSecurityReports(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		items, err := store.ListVisible(actor, ownerCheck(catalog, actor))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(items)
		writeJSON(w, 200, map[string]any{"items": paginate(items, page, perPage), "page": page, "per_page": perPage, "total_count": total})
	}
}
func getSecurityReport(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		item, err := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 200)
	}
}
func triageSecurityReport(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Severity     string `json:"severity"`
			EmbargoState string `json:"embargo_state"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		item, err := store.Triage(r.PathValue("report"), securityreports.TriageInput{ActorID: actor, Severity: in.Severity, EmbargoState: in.EmbargoState}, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 200)
	}
}
func addSecurityReportMember(store securityReportStore, catalog pullRequestRepositoryStore, userStore securityUserStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			UserID string `json:"user_id"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		if _, err := userStore.Get(users.ID(in.UserID)); err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_team_member"})
			return
		}
		item, err := store.SetMember(r.PathValue("report"), actor, in.UserID, true, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}
func removeSecurityReportMember(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		item, err := store.SetMember(r.PathValue("report"), actor, r.PathValue("user"), false, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 200)
	}
}
func addSecurityReportMessage(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.AddMessage(r.PathValue("report"), actor, in.Body, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}
func writeSecurityReport(w http.ResponseWriter, item securityreports.Report, err error, success int) {
	switch {
	case errors.Is(err, securityreports.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(err, securityreports.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_security_report"})
	case errors.Is(err, securityreports.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "security_report_conflict"})
	case err != nil:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	default:
		writeJSON(w, success, item)
	}
}
