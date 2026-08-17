package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apiconsumers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
)

func registerAPIConsumersHTTP(mux *http.ServeMux, s *apiconsumers.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/api-consumers"
	migrationBase := "/repositories/{repository}/api-contract-migrations"
	mux.HandleFunc("GET "+migrationBase, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.ListMigrations(string(repo.ID), a.UserID, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+migrationBase, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in apiconsumers.MigrationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateMigration(string(repo.ID), a.UserID, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("GET "+migrationBase+"/{migration}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.GetMigration(string(repo.ID), r.PathValue("migration"), a.UserID, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusOK, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ApplicationID string                       `json:"application_id"`
			Decision      string                       `json:"decision"`
			Reason        string                       `json:"reason"`
			Work          []apiconsumers.WorkReference `json:"work"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AcknowledgeMigration(string(repo.ID), r.PathValue("migration"), in.ApplicationID, a.UserID, in.Decision, in.Reason, in.Work)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/dual-version-tests", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ApplicationID string `json:"application_id"`
			apiconsumers.DualVersionTest
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.RecordDualVersionTest(string(repo.ID), r.PathValue("migration"), in.ApplicationID, a.UserID, in.DualVersionTest)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/exceptions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ApplicationID string    `json:"application_id"`
			Reason        string    `json:"reason"`
			Scope         string    `json:"scope"`
			ExpiresAt     time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RequestMigrationException(string(repo.ID), r.PathValue("migration"), in.ApplicationID, a.UserID, in.Reason, in.Scope, in.ExpiresAt)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/exceptions/{exception}/decision", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ApplicationID string `json:"application_id"`
			Decision      string `json:"decision"`
			Reason        string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.DecideMigrationException(string(repo.ID), r.PathValue("migration"), a.UserID, in.ApplicationID, r.PathValue("exception"), in.Decision, in.Reason)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusOK, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/attestations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ApplicationID string `json:"application_id"`
			apiconsumers.MigrationAttestation
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.AttestMigration(string(repo.ID), r.PathValue("migration"), in.ApplicationID, a.UserID, in.MigrationAttestation)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+migrationBase+"/{migration}/advance", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		x, e := s.AdvanceMigration(string(repo.ID), r.PathValue("migration"), a.UserID, in.ExpectedVersion)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusOK, x)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.List(string(repo.ID), a.UserID, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.RegistrationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Register(string(repo.ID), a.UserID, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{application}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.Get(string(repo.ID), r.PathValue("application"), a.UserID, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/decision", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in apiconsumers.ApprovalInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Decide(string(repo.ID), r.PathValue("application"), a.UserID, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/credentials", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.Rotate(string(repo.ID), r.PathValue("application"), a.UserID, in.Reason, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/consent", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Reason          string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		x, e := s.Consent(string(repo.ID), r.PathValue("application"), a.UserID, in.Reason, in.ExpectedVersion)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.Control(string(repo.ID), r.PathValue("application"), a.UserID, in.Action, in.Reason, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/ownership", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			TargetID string `json:"target_id"`
			Action   string `json:"action"`
			Reason   string `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.Transfer(string(repo.ID), r.PathValue("application"), a.UserID, in.TargetID, in.Reason, in.Action == "accept", writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/integration-work", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.WorkInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.CreateWork(string(repo.ID), r.PathValue("application"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.VerificationInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.RecordVerification(string(repo.ID), r.PathValue("application"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/observations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.ObservationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.RecordObservation(string(repo.ID), r.PathValue("application"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/investigations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.InvestigationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.OpenInvestigation(string(repo.ID), r.PathValue("application"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("GET "+base+"/{application}/investigations/{investigation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.GetInvestigation(string(repo.ID), r.PathValue("application"), r.PathValue("investigation"), a.UserID, writer)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusOK, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/investigations/{investigation}/agents", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			AgentID string `json:"agent_id"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.InviteAgent(string(repo.ID), r.PathValue("application"), r.PathValue("investigation"), a.UserID, writer, in.AgentID)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/investigations/{investigation}/entries", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.EntryInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.AddEntry(string(repo.ID), r.PathValue("application"), r.PathValue("investigation"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/investigations/{investigation}/reproductions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.SandboxInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.Reproduce(string(repo.ID), r.PathValue("application"), r.PathValue("investigation"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST "+base+"/{application}/investigations/{investigation}/change-work", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in apiconsumers.RouteInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		_, writer := repositoryPermission(a, auth.RepositoryWrite)
		x, e := s.RouteChange(string(repo.ID), r.PathValue("application"), r.PathValue("investigation"), a.UserID, writer, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, x)
	})
	mux.HandleFunc("POST /api-sandbox/{application}/requests", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		var in apiconsumers.SandboxInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Sandbox(r.PathValue("application"), token, in)
		if apiConsumerError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
}

func repositoryPermission(a auth.Grant, permission auth.Scope) (auth.Scope, bool) {
	for _, p := range a.Scopes {
		if p == permission {
			return p, true
		}
	}
	return "", false
}
func apiConsumerError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, apiconsumers.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "api_consumer_not_found"})
	case errors.Is(e, apiconsumers.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "api_consumer_changed"})
	case errors.Is(e, apiconsumers.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "api_consumer_forbidden"})
	case errors.Is(e, apiconsumers.ErrQuota):
		writeJSON(w, 429, map[string]string{"error": "sandbox_quota_exceeded"})
	case errors.Is(e, apiconsumers.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_api_consumer"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
