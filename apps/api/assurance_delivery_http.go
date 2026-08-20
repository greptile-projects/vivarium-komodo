package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assurancedelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/independentassessments"
)

type assuranceFindingSource struct{ assessments *independentassessments.Store }

func (s assuranceFindingSource) Finding(repo, assessment, finding string) (assurancedelivery.FindingSource, error) {
	a, err := s.assessments.Get(repo, assessment)
	if err != nil {
		return assurancedelivery.FindingSource{}, err
	}
	for _, event := range a.Events {
		if event.ID == finding && event.Kind == "finding" {
			return assurancedelivery.FindingSource{AssessmentID: a.ID, FindingID: event.ID, ControlID: event.ControlID, FindingBody: event.Body, ActorID: event.ActorID, OwnerID: a.OwnerID, Scope: assurancedelivery.AssessmentScope{ProgramID: a.Scope.ProgramID, ProgramVersion: a.Scope.ProgramVersion, ControlIDs: a.Scope.ControlIDs, Releases: a.Scope.Releases, EvidencePackageIDs: a.Scope.EvidencePackageIDs, PeriodStart: a.Scope.PeriodStart, PeriodEnd: a.Scope.PeriodEnd}}, nil
		}
	}
	return assurancedelivery.FindingSource{}, independentassessments.ErrNotFound
}

func registerAssuranceDeliveryHTTP(mux *http.ServeMux, s *assurancedelivery.Store, assessments *independentassessments.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/assurance-remediations"
	access := func(w http.ResponseWriter, r *http.Request, scope auth.Scope, required bool) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, scope, required)
		return string(repo.ID), a.UserID, ok
	}
	ownerAccess := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if ok && repo.OwnerID != a.UserID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "repository_owner_required"})
			return "", "", false
		}
		return string(repo.ID), a.UserID, ok
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead, true)
		if !ok {
			return
		}
		v, e := s.ListRemediations(repo)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("POST /repositories/{repository}/independent-assessments/{assessment}/findings/{finding}/remediations", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assurancedelivery.RemediationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.CreateRemediation(repo, r.PathValue("assessment"), r.PathValue("finding"), actor, in)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+base+"/{remediation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, auth.RepositoryRead, true)
		if !ok {
			return
		}
		v, e := s.GetRemediation(repo, r.PathValue("remediation"))
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/work/{work}/progress", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in assurancedelivery.ProgressInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		v, e := s.Progress(repo, r.PathValue("remediation"), r.PathValue("work"), actor, in)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/verifications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in assurancedelivery.VerificationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Verify(repo, r.PathValue("remediation"), actor, in)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/dispositions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Disposition(repo, r.PathValue("remediation"), actor, "owner", in.Decision, in.Rationale)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST "+base+"/{remediation}/drift", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision string `json:"revision"`
			Reason   string `json:"reason"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Drift(repo, r.PathValue("remediation"), actor, in.Revision, in.Reason)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("POST /independent-assessor/remediations/{remediation}/dispositions", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		a, i, e := assessments.Authenticate(token)
		if independentError(w, e) {
			return
		}
		v, e := s.GetRemediation(a.RepositoryID, r.PathValue("remediation"))
		if e == nil && v.AssessmentID != a.ID {
			e = assurancedelivery.ErrForbidden
		}
		if e != nil {
			assuranceDeliveryError(w, e)
			return
		}
		var in struct {
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e = s.Disposition(a.RepositoryID, v.ID, i.AssessorID, "assessor", in.Decision, in.Rationale)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})

	statements := "/repositories/{repository}/assurance-statements"
	mux.HandleFunc("POST "+statements, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := ownerAccess(w, r)
		if !ok {
			return
		}
		var in assurancedelivery.StatementInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Publish(repo, actor, in)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	mux.HandleFunc("GET "+statements, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead, false)
		if !ok {
			return
		}
		audience := "public"
		if actor != "" {
			audience = "repository"
		}
		v, e := s.ListStatements(repo, audience)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 200, map[string]any{"items": v})
		}
	})
	mux.HandleFunc("GET "+statements+"/{statement}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryRead, false)
		if !ok {
			return
		}
		audience := "public"
		if actor != "" {
			audience = "repository"
		}
		v, e := s.Statement(repo, r.PathValue("statement"), audience)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	mux.HandleFunc("POST "+statements+"/{statement}/revocation", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.RevokeStatement(repo, r.PathValue("statement"), actor, in.Reason)
		if !assuranceDeliveryError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}

func assuranceDeliveryError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status, code := 500, "internal_error"
	switch {
	case errors.Is(e, assurancedelivery.ErrNotFound):
		status, code = 404, "assurance_delivery_not_found"
	case errors.Is(e, assurancedelivery.ErrForbidden):
		status, code = 403, "assurance_delivery_forbidden"
	case errors.Is(e, assurancedelivery.ErrConflict):
		status, code = 409, "assurance_delivery_not_current"
	case errors.Is(e, assurancedelivery.ErrInvalid):
		status, code = 422, "invalid_assurance_delivery"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
