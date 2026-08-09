package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/activities"
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
	AddResource(string, string, securityreports.ResourceLink, func(string) bool) (securityreports.Report, error)
	AddFinding(string, string, securityreports.Finding, func(string) bool) (securityreports.Report, error)
	SetImpact(string, string, securityreports.Impact, func(string) bool) (securityreports.Report, error)
	StartInvestigation(string, string, string, string, []string, func(string) bool) (securityreports.Report, string, error)
	InvestigationContext(string) (securityreports.Report, securityreports.Investigation, error)
	AddInvestigationRecord(string, string, string, string, []string) (securityreports.Report, securityreports.Investigation, error)
	ControlInvestigation(string, string, string, string, string, func(string) bool) (securityreports.Report, error)
	CreateRepair(string, securityreports.RepairInput, func(string) bool) (securityreports.Report, securityreports.RepairTask, error)
	StartRepairSession(string, string, string, string, string, string, string, time.Time, func(string) bool) (securityreports.Report, securityreports.RepairSession, error)
	AddRepairRecord(string, string, string, string, string, string, string, string, func(string) bool) (securityreports.Report, error)
	RevokeRepairSession(string, string, string, string, func(string) bool) (securityreports.Report, securityreports.RepairSession, error)
	UpdateRepairVerification(string, string, securityreports.VerificationAction, func(string) bool) (securityreports.Report, error)
	PrepareDisclosure(string, securityreports.DisclosureInput, func(string) bool) (securityreports.Report, error)
	CompleteDisclosure(string, string, []securityreports.DisclosureBranch, string, func(string) bool) (securityreports.Report, error)
	GetPublicAdvisory(string) (securityreports.Report, error)
}
type securityUserStore interface {
	Get(users.ID) (users.User, error)
}
type securityCredentialStore interface {
	authStore
	IssueRepositoryGit(string, string, string, string, time.Duration) (auth.IssuedGrant, error)
	RevokeRepositoryGit(string, string, string) error
}

func registerSecurityReportsHTTP(mux *http.ServeMux, store securityReportStore, catalog pullRequestRepositoryStore, userStore securityUserStore, credentials securityCredentialStore, extras ...activityStore) {
	var activity activityStore
	if len(extras) > 0 {
		activity = extras[0]
	}
	mux.HandleFunc("POST /security-reports", createSecurityReport(store, catalog, credentials))
	mux.HandleFunc("GET /security-reports", listSecurityReports(store, catalog, credentials))
	mux.HandleFunc("GET /security-reports/{report}", getSecurityReport(store, catalog, credentials))
	mux.HandleFunc("PATCH /security-reports/{report}/triage", triageSecurityReport(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/team", addSecurityReportMember(store, catalog, userStore, credentials))
	mux.HandleFunc("DELETE /security-reports/{report}/team/{user}", removeSecurityReportMember(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/messages", addSecurityReportMessage(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/resources", addSecurityReportResource(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/findings", addSecurityReportFinding(store, catalog, credentials))
	mux.HandleFunc("PUT /security-reports/{report}/impact", setSecurityReportImpact(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/investigations", startSecurityInvestigation(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/investigations/{session}/control", controlSecurityInvestigation(store, catalog, credentials))
	mux.HandleFunc("GET /security-investigations/context", securityInvestigationContext(store))
	mux.HandleFunc("POST /security-investigations/records", addSecurityInvestigationRecord(store))
	mux.HandleFunc("POST /security-reports/{report}/repairs", createSecurityRepair(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/repairs/{repair}/sessions", startSecurityRepairSession(store, catalog, userStore, credentials))
	mux.HandleFunc("POST /security-reports/{report}/repairs/{repair}/sessions/{session}/records", addSecurityRepairRecord(store, catalog, credentials))
	mux.HandleFunc("DELETE /security-reports/{report}/repairs/{repair}/sessions/{session}", revokeSecurityRepairSession(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/repairs/{repair}/verification", updateSecurityRepairVerification(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/disclosure", prepareSecurityDisclosure(store, catalog, credentials))
	mux.HandleFunc("POST /security-reports/{report}/disclosure/publish", publishSecurityDisclosure(store, catalog, credentials, activity))
	mux.HandleFunc("GET /security-advisories/{advisory}", getPublicSecurityAdvisory(store))
}

func prepareSecurityDisclosure(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			AdvisoryID      string            `json:"advisory_id"`
			Summary         string            `json:"summary"`
			UpgradeGuidance string            `json:"upgrade_guidance"`
			Credits         []string          `json:"credits"`
			ScheduledAt     *time.Time        `json:"scheduled_at"`
			PublishedRefs   map[string]string `json:"published_refs"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.PrepareDisclosure(r.PathValue("report"), securityreports.DisclosureInput{ActorID: actor, AdvisoryID: in.AdvisoryID, Summary: in.Summary, UpgradeGuidance: in.UpgradeGuidance, Credits: in.Credits, ScheduledAt: in.ScheduledAt, PublishedRefs: in.PublishedRefs}, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}

func publishSecurityDisclosure(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore, activity activityStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		item, err := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		if err != nil {
			writeSecurityReport(w, item, err, 200)
			return
		}
		if item.Disclosure == nil || (item.Disclosure.State != "ready" && item.Disclosure.State != "scheduled" && item.Disclosure.State != "paused") {
			writeJSON(w, 409, map[string]string{"error": "disclosure_not_ready"})
			return
		}
		if item.Disclosure.ScheduledAt != nil && time.Now().UTC().Before(*item.Disclosure.ScheduledAt) {
			writeJSON(w, 409, map[string]string{"error": "disclosure_scheduled"})
			return
		}
		branches := append([]securityreports.DisclosureBranch{}, item.Disclosure.Branches...)
		created := []struct {
			repo *storage.Repository
			ref  storage.ReferenceName
		}{}
		failure := ""
		for i := range branches {
			repo, openErr := catalog.Open(storage.ID(branches[i].RepositoryID))
			if openErr != nil {
				failure = "repository unavailable: " + branches[i].RepositoryID
				branches[i].State, branches[i].Error = "failed", failure
				break
			}
			ref := storage.Reference{Name: storage.ReferenceName(branches[i].PublishedRef), ObjectID: storage.ObjectID(branches[i].Revision)}
			if createErr := repo.CreateReference(ref); createErr != nil {
				existing, readErr := repo.ReadReference(ref.Name)
				if readErr != nil || existing.ObjectID != ref.ObjectID {
					failure = "could not publish " + branches[i].PublishedRef
					branches[i].State, branches[i].Error = "failed", failure
					break
				}
			} else {
				created = append(created, struct {
					repo *storage.Repository
					ref  storage.ReferenceName
				}{repo, ref.Name})
			}
			branches[i].State = "published"
		}
		if failure != "" {
			for _, made := range created {
				_ = made.repo.DeleteReference(made.ref)
			}
			for i := range branches {
				if branches[i].State == "published" {
					branches[i].State = "rolled_back"
				}
			}
			updated, completeErr := store.CompleteDisclosure(item.ID, actor, branches, failure, ownerCheck(catalog, actor))
			if completeErr != nil {
				writeSecurityReport(w, updated, completeErr, 200)
				return
			}
			scrubSecurityCredentials(&updated)
			writeJSON(w, 409, updated)
			return
		}
		updated, err := store.CompleteDisclosure(item.ID, actor, branches, "", ownerCheck(catalog, actor))
		if err == nil {
			for _, affected := range updated.Affected {
				repository, inspectErr := catalog.Inspect(storage.ID(affected.RepositoryID))
				if inspectErr != nil {
					continue
				}
				recipients := append([]string{repository.OwnerID}, repository.CollaboratorIDs...)
				for _, recipient := range recipients {
					_ = recordActivity(activity, activities.Input{RepositoryID: affected.RepositoryID, ActorID: actor, Type: "security_advisory.published", Resource: activities.Resource{Type: "security_advisory", ID: updated.Disclosure.AdvisoryID}, TargetUserID: recipient, Metadata: map[string]string{"severity": updated.Severity, "upgrade_guidance": updated.Disclosure.UpgradeGuidance}})
				}
			}
		}
		writeSecurityReport(w, updated, err, 200)
	}
}

func getPublicSecurityAdvisory(store securityReportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := store.GetPublicAdvisory(r.PathValue("advisory"))
		if err != nil {
			writeSecurityReport(w, item, err, 200)
			return
		}
		writeJSON(w, 200, item)
	}
}

func updateSecurityRepairVerification(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in securityreports.VerificationAction
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		report, err := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		if err != nil {
			writeSecurityReport(w, report, err, 200)
			return
		}
		var task *securityreports.RepairTask
		for x := range report.Repairs {
			if report.Repairs[x].ID == r.PathValue("repair") {
				task = &report.Repairs[x]
			}
		}
		if task == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		repository, err := catalog.Open(storage.ID(task.RepositoryID))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		ref, err := repository.ReadReference(storage.ReferenceName(task.Branch))
		if err != nil || string(ref.ObjectID) != in.Revision {
			writeJSON(w, 409, map[string]string{"error": "repair_revision_changed"})
			return
		}
		in.ActorID = actor
		updated, err := store.UpdateRepairVerification(report.ID, task.ID, in, ownerCheck(catalog, actor))
		writeSecurityReport(w, updated, err, 200)
	}
}

func createSecurityRepair(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			RepositoryID  string   `json:"repository_id"`
			Version       string   `json:"version"`
			Outcome       string   `json:"outcome"`
			BaseRevision  string   `json:"base_revision"`
			DependencyIDs []string `json:"dependency_ids"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		report, accessErr := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		if accessErr != nil {
			writeSecurityReport(w, report, accessErr, 201)
			return
		}
		if !repositoryParticipant(catalog, storage.ID(in.RepositoryID), actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		repository, err := catalog.Open(storage.ID(in.RepositoryID))
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_base"})
			return
		}
		base, err := repository.ReadObject(storage.ObjectID(in.BaseRevision))
		if err != nil || base.Type != storage.CommitObject {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_base"})
			return
		}
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		branch := "refs/heads/embargo/" + hex.EncodeToString(nonce[:])
		if err = repository.CreateReference(storage.Reference{Name: storage.ReferenceName(branch), ObjectID: storage.ObjectID(in.BaseRevision)}); err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		item, task, err := store.CreateRepair(r.PathValue("report"), securityreports.RepairInput{ActorID: actor, RepositoryID: in.RepositoryID, Version: in.Version, Outcome: in.Outcome, BaseRevision: in.BaseRevision, Branch: branch, DependencyIDs: in.DependencyIDs}, ownerCheck(catalog, actor))
		if err != nil {
			_ = repository.DeleteReference(storage.ReferenceName(branch))
			writeSecurityReport(w, item, err, 201)
			return
		}
		scrubSecurityCredentials(&item)
		writeJSON(w, 201, map[string]any{"report": item, "repair": task})
	}
}

func startSecurityRepairSession(store securityReportStore, catalog pullRequestRepositoryStore, users securityUserStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Kind       string `json:"kind"`
			AssigneeID string `json:"assignee_id"`
			Mandate    string `json:"mandate"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		report, err := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		if err != nil {
			writeSecurityReport(w, report, err, 201)
			return
		}
		var task *securityreports.RepairTask
		for x := range report.Repairs {
			if report.Repairs[x].ID == r.PathValue("repair") {
				task = &report.Repairs[x]
			}
		}
		if task == nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if !repositoryParticipant(catalog, storage.ID(task.RepositoryID), actor) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		grantUser := actor
		if strings.ToLower(strings.TrimSpace(in.Kind)) == "human" {
			if _, er := users.Get(userspkgID(in.AssigneeID)); er != nil || !securityTeamMember(report, in.AssigneeID) || !repositoryParticipant(catalog, storage.ID(task.RepositoryID), in.AssigneeID) {
				writeJSON(w, 422, map[string]string{"error": "invalid_repair_assignee"})
				return
			}
			grantUser = in.AssigneeID
		} else if strings.ToLower(strings.TrimSpace(in.Kind)) != "agent" || strings.TrimSpace(in.AssigneeID) != "codex" {
			writeJSON(w, 422, map[string]string{"error": "invalid_repair_assignee"})
			return
		}
		var credentialNonce [8]byte
		if _, er := rand.Read(credentialNonce[:]); er != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		name := "Security repair " + report.ID + "/" + task.ID + "/" + hex.EncodeToString(credentialNonce[:])
		issued, er := credentials.IssueRepositoryGit(grantUser, name, task.RepositoryID, task.Branch, 24*time.Hour)
		if er != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		item, session, er := store.StartRepairSession(report.ID, task.ID, actor, in.Kind, in.AssigneeID, in.Mandate, name, issued.ExpiresAt, ownerCheck(catalog, actor))
		if er != nil {
			_ = credentials.RevokeRepositoryGit(task.RepositoryID, task.Branch, name)
			writeSecurityReport(w, item, er, 201)
			return
		}
		scrubSecurityCredentials(&item)
		writeJSON(w, 201, map[string]any{"report": item, "session": session, "git_credential": issued.Token, "credential_notice": "shown once; exact embargoed repository branch only"})
	}
}

func userspkgID(id string) users.ID { return users.ID(id) }
func securityTeamMember(r securityreports.Report, id string) bool {
	if r.ReporterID == id {
		return true
	}
	for _, m := range r.Team {
		if m.UserID == id {
			return true
		}
	}
	return false
}
func repositoryParticipant(c pullRequestRepositoryStore, id storage.ID, actor string) bool {
	r, e := c.Inspect(id)
	if e != nil {
		return false
	}
	if r.OwnerID == actor {
		return true
	}
	ok, _ := c.IsCollaborator(id, actor)
	return ok
}

func addSecurityRepairRecord(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Type     string `json:"type"`
			Body     string `json:"body"`
			Revision string `json:"revision"`
			Decision string `json:"decision"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		if in.Type == "branch_update" || in.Type == "review" {
			report, er := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
			if er != nil {
				writeSecurityReport(w, report, er, 201)
				return
			}
			var task *securityreports.RepairTask
			for x := range report.Repairs {
				if report.Repairs[x].ID == r.PathValue("repair") {
					task = &report.Repairs[x]
				}
			}
			if task == nil {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
			repository, er := catalog.Open(storage.ID(task.RepositoryID))
			if er != nil {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
			ref, er := repository.ReadReference(storage.ReferenceName(task.Branch))
			if er != nil || string(ref.ObjectID) != in.Revision {
				writeJSON(w, 409, map[string]string{"error": "repair_revision_changed"})
				return
			}
		}
		item, err := store.AddRepairRecord(r.PathValue("report"), r.PathValue("repair"), r.PathValue("session"), actor, in.Type, in.Body, in.Revision, in.Decision, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}
func revokeSecurityRepairSession(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		item, session, err := store.RevokeRepairSession(r.PathValue("report"), r.PathValue("repair"), r.PathValue("session"), actor, ownerCheck(catalog, actor))
		if err == nil {
			for _, task := range item.Repairs {
				if task.ID == r.PathValue("repair") {
					_ = credentials.RevokeRepositoryGit(task.RepositoryID, task.Branch, session.CredentialName)
				}
			}
		}
		writeSecurityReport(w, item, err, 200)
	}
}

func addSecurityReportResource(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in securityreports.ResourceLink
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.AddResource(r.PathValue("report"), actor, in, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}
func addSecurityReportFinding(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in securityreports.Finding
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.AddFinding(r.PathValue("report"), actor, in, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 201)
	}
}
func setSecurityReportImpact(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in securityreports.Impact
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.SetImpact(r.PathValue("report"), actor, in, ownerCheck(catalog, actor))
		writeSecurityReport(w, item, err, 200)
	}
}
func startSecurityInvestigation(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Agent       string   `json:"agent"`
			Mandate     string   `json:"mandate"`
			EvidenceIDs []string `json:"evidence_ids"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, token, err := store.StartInvestigation(r.PathValue("report"), actor, in.Agent, in.Mandate, in.EvidenceIDs, ownerCheck(catalog, actor))
		if err != nil {
			writeSecurityReport(w, item, err, 201)
			return
		}
		scrubSecurityCredentials(&item)
		writeJSON(w, 201, map[string]any{"report": item, "worker_credential": token, "credential_notice": "shown once; selected advisory evidence and investigation record access only"})
	}
}
func controlSecurityInvestigation(store securityReportStore, catalog pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		var in struct {
			Action  string `json:"action"`
			Message string `json:"message"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		item, err := store.ControlInvestigation(r.PathValue("report"), r.PathValue("session"), actor, in.Action, in.Message, ownerCheck(catalog, actor))
		scrubSecurityCredentials(&item)
		writeSecurityReport(w, item, err, 200)
	}
}
func securityInvestigationContext(store securityReportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, inv, err := store.InvestigationContext(investigationBearer(r))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		allowed := map[string]bool{}
		for _, id := range inv.EvidenceIDs {
			allowed[id] = true
		}
		evidence := []securityreports.Evidence{}
		links := []securityreports.ResourceLink{}
		for _, e := range report.Evidence {
			if allowed[e.ID] {
				evidence = append(evidence, e)
			}
		}
		for _, e := range report.ResourceLinks {
			if allowed[e.ID] {
				links = append(links, e)
			}
		}
		inv.CredentialDigest = ""
		writeJSON(w, 200, map[string]any{"advisory": map[string]any{"id": report.ID, "title": report.Title, "summary": report.Summary, "severity": report.Severity}, "investigation": inv, "submitted_evidence": evidence, "resource_links": links})
	}
}
func addSecurityInvestigationRecord(store securityReportStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Type        string   `json:"type"`
			Body        string   `json:"body"`
			Uncertainty string   `json:"uncertainty"`
			EvidenceIDs []string `json:"evidence_ids"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		_, inv, err := store.AddInvestigationRecord(investigationBearer(r), in.Type, in.Body, in.Uncertainty, in.EvidenceIDs)
		inv.CredentialDigest = ""
		switch {
		case errors.Is(err, securityreports.ErrNotFound), errors.Is(err, securityreports.ErrConflict):
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
		case errors.Is(err, securityreports.ErrTransition):
			writeJSON(w, 409, map[string]string{"error": "investigation_not_running"})
		case errors.Is(err, securityreports.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_record"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
		default:
			writeJSON(w, 201, inv)
		}
	}
}
func scrubSecurityCredentials(r *securityreports.Report) {
	for x := range r.Investigations {
		r.Investigations[x].CredentialDigest = ""
	}
	for x := range r.Repairs {
		for y := range r.Repairs[x].Sessions {
			r.Repairs[x].Sessions[y].CredentialName = ""
		}
	}
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
func removeSecurityReportMember(store securityReportStore, catalog pullRequestRepositoryStore, credentials securityCredentialStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := securityActor(w, r, credentials)
		if !ok {
			return
		}
		before, _ := store.Get(r.PathValue("report"), actor, ownerCheck(catalog, actor))
		item, err := store.SetMember(r.PathValue("report"), actor, r.PathValue("user"), false, ownerCheck(catalog, actor))
		if err == nil {
			for _, task := range before.Repairs {
				for _, session := range task.Sessions {
					if session.AssigneeID == r.PathValue("user") && session.State == "active" {
						_ = credentials.RevokeRepositoryGit(task.RepositoryID, task.Branch, session.CredentialName)
						_, _, _ = store.RevokeRepairSession(before.ID, task.ID, session.ID, actor, ownerCheck(catalog, actor))
					}
				}
			}
		}
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
	case errors.Is(err, securityreports.ErrTransition):
		writeJSON(w, 409, map[string]string{"error": "invalid_investigation_transition"})
	case err != nil:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	default:
		scrubSecurityCredentials(&item)
		writeJSON(w, success, item)
	}
}
