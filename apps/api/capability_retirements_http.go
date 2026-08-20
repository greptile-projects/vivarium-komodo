package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/capabilityretirements"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerCapabilityRetirementsHTTP(mux *http.ServeMux, s *capabilityretirements.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/capability-retirements"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, scope, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.List(repo)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityretirements.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(repo, actor, in)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{plan}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("plan"))
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			AuthorKind        string   `json:"author_kind"`
			Kind              string   `json:"kind"`
			Body              string   `json:"body"`
			EvidenceReference string   `json:"evidence_reference"`
			AudienceIDs       []string `json:"audience_ids"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Assess(repo, r.PathValue("plan"), actor, in.AuthorKind, in.Kind, in.Body, in.EvidenceReference, in.AudienceIDs)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Scope     string `json:"scope"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.DecideApproval(repo, r.PathValue("plan"), actor, in.Scope, in.Decision, in.Rationale)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/policy-decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			Kind      string    `json:"kind"`
			Subject   string    `json:"subject"`
			Decision  string    `json:"decision"`
			Rationale string    `json:"rationale"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.AddPolicyDecision(repo, r.PathValue("plan"), actor, in.Kind, in.Subject, in.Decision, in.Rationale, in.ExpiresAt)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/tasks", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in capabilityretirements.MigrationTaskInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		if _, e := repos.Inspect(storage.ID(in.RepositoryID)); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_task_repository"})
			return
		}
		v, e := s.CreateMigrationTask(repo, r.PathValue("plan"), actor, in)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/tasks/{task}/progress", func(w http.ResponseWriter, r *http.Request) {
		anchor, e := repos.Inspect(storage.ID(r.PathValue("repository")))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "capability_retirement_not_found"})
			return
		}
		p, e := s.Get(string(anchor.ID), r.PathValue("plan"))
		if capabilityRetirementError(w, e) {
			return
		}
		var task *capabilityretirements.MigrationTask
		for i := range p.Tasks {
			if p.Tasks[i].ID == r.PathValue("task") {
				task = &p.Tasks[i]
				break
			}
		}
		if task == nil {
			writeJSON(w, 404, map[string]string{"error": "capability_retirement_not_found"})
			return
		}
		_, actor, ok := proposalRepositoryAccessPath(w, r, repos, credentials, task.Input.RepositoryID, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			State   string                           `json:"state"`
			Summary string                           `json:"summary"`
			Work    []capabilityretirements.WorkLink `json:"work"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		for _, x := range in.Work {
			if x.RepositoryID != task.Input.RepositoryID {
				writeJSON(w, 422, map[string]string{"error": "work_repository_mismatch"})
				return
			}
		}
		v, e := s.ReportTask(string(anchor.ID), p.ID, task.ID, actor.UserID, in.State, in.Summary, in.Work)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{plan}/consumer-discoveries", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			ConsumerID        string `json:"consumer_id"`
			RepositoryID      string `json:"repository_id"`
			Revision          string `json:"revision"`
			EvidenceReference string `json:"evidence_reference"`
			Summary           string `json:"summary"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.DiscoverConsumer(repo, r.PathValue("plan"), actor, in.ConsumerID, in.RepositoryID, in.Revision, in.EvidenceReference, in.Summary)
		if !capabilityRetirementError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func capabilityRetirementError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, capabilityretirements.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "capability_retirement_not_found"})
	case errors.Is(e, capabilityretirements.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "capability_retirement_owner_required"})
	case errors.Is(e, capabilityretirements.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "capability_retirement_conflict"})
	case errors.Is(e, capabilityretirements.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_capability_retirement"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
