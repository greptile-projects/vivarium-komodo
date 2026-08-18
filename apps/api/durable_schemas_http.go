package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/durableschemas"
)

func registerDurableSchemasHTTP(mux *http.ServeMux, s *durableschemas.Store, repos dataFlowRepositories, c authStore) {
	base := "/repositories/{repository}/durable-schemas"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.ListSchemas(string(repo.ID))
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.VersionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateSchema(string(repo.ID), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{schema}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetSchema(string(repo.ID), r.PathValue("schema"))
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{schema}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			durableschemas.VersionInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ReviseSchema(string(repo.ID), r.PathValue("schema"), a.UserID, in.ExpectedVersion, in.VersionInput)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	migrations := "/repositories/{repository}/schema-migrations"
	mux.HandleFunc("GET "+migrations, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.ListMigrations(string(repo.ID))
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x})
	})
	mux.HandleFunc("POST "+migrations, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.MigrationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateMigration(string(repo.ID), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+migrations+"/{migration}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetMigration(string(repo.ID), r.PathValue("migration"))
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			OwnerID   string `json:"owner_id"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		x, e := s.Approve(string(repo.ID), r.PathValue("migration"), a.UserID, in.OwnerID, in.Decision, in.Rationale)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/work-items", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.WorkItemInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !durableSchemaTargetAccess(w, r, repos, c, in.RepositoryID, auth.RepositoryWrite) {
			return
		}
		x, e := s.AddWorkItem(string(repo.ID), r.PathValue("migration"), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/pull-contracts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.PullContractInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !durableSchemaTargetAccess(w, r, repos, c, in.RepositoryID, auth.RepositoryRead) {
			return
		}
		x, e := s.AddPullContract(string(repo.ID), r.PathValue("migration"), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/rehearsals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.RehearsalInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateRehearsal(string(repo.ID), r.PathValue("migration"), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/rehearsals/{rehearsal}/inputs", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion      int64             `json:"expected_version"`
			ApplicationRevisions map[string]string `json:"application_revisions"`
			Dependencies         map[string]string `json:"dependencies"`
			MigrationRevision    string            `json:"migration_revision"`
			DefinitionDigest     string            `json:"definition_digest"`
			DataShapeDigest      string            `json:"data_shape_digest"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.UpdateRehearsalInputs(string(repo.ID), r.PathValue("migration"), r.PathValue("rehearsal"), a.UserID, in.ExpectedVersion, in.ApplicationRevisions, in.Dependencies, in.MigrationRevision, in.DefinitionDigest, in.DataShapeDigest)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/rehearsals/{rehearsal}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in durableschemas.AttemptInput
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		x, e := s.RecordAttempt(string(repo.ID), r.PathValue("migration"), r.PathValue("rehearsal"), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/rehearsals/{rehearsal}/attestations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			AttemptID string `json:"attempt_id"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<16) {
			return
		}
		x, e := s.AttestRehearsal(string(repo.ID), r.PathValue("migration"), r.PathValue("rehearsal"), a.UserID, in.AttemptID, in.Decision, in.Rationale)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+migrations+"/{migration}/rehearsals/{rehearsal}/investigation", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in durableschemas.InvestigationNote
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddInvestigation(string(repo.ID), r.PathValue("migration"), r.PathValue("rehearsal"), a.UserID, in)
		if durableSchemaError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}

func durableSchemaTargetAccess(w http.ResponseWriter, r *http.Request, repos proposalRepositoryStore, c authStore, target string, scope auth.Scope) bool {
	copy := r.Clone(r.Context())
	copy.SetPathValue("repository", target)
	_, _, ok := proposalRepositoryAccess(w, copy, repos, c, scope, true)
	return ok
}

func durableSchemaError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, durableschemas.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "durable_schema_not_found"})
	case errors.Is(e, durableschemas.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_durable_schema"})
	case errors.Is(e, durableschemas.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "durable_schema_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
