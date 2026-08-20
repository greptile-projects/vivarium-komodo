package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceevidence"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/independentassessments"
)

func registerIndependentAssessmentsHTTP(mux *http.ServeMux, s *independentassessments.Store, evidence *assuranceevidence.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/independent-assessments"
	owner := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if ok && repo.OwnerID != a.UserID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "repository_owner_required"})
			return "", "", false
		}
		return string(repo.ID), a.UserID, ok
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		xs, e := s.List(repo)
		if e == nil {
			out := xs[:0]
			for _, x := range xs {
				if x.OwnerID == a {
					out = append(out, x)
				}
			}
			xs = out
		}
		if !independentError(w, e) {
			for i := range xs {
				xs[i] = independentassessments.Redact(xs[i])
			}
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		var in independentassessments.OpenInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Open(repo, a, in)
		if !independentError(w, e) {
			writeJSON(w, 201, independentassessments.Redact(x))
		}
	})
	mux.HandleFunc("GET "+base+"/{assessment}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("assessment"))
		if e == nil && x.OwnerID != a {
			e = independentassessments.ErrForbidden
		}
		if !independentError(w, e) {
			writeJSON(w, 200, independentassessments.Redact(x))
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/invitations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		var in independentassessments.InvitationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, c, e := s.Invite(repo, r.PathValue("assessment"), a, in)
		if !independentError(w, e) {
			writeJSON(w, 201, map[string]any{"assessment": independentassessments.Redact(x), "credential": c})
		}
	})
	mux.HandleFunc("DELETE "+base+"/{assessment}/invitations/{invitation}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		x, e := s.Revoke(repo, r.PathValue("assessment"), a, r.PathValue("invitation"))
		if !independentError(w, e) {
			writeJSON(w, 200, independentassessments.Redact(x))
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		var in independentassessments.EventInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Add(repo, r.PathValue("assessment"), a, "owner", in)
		if !independentError(w, e) {
			writeJSON(w, 201, independentassessments.Redact(x))
		}
	})
	mux.HandleFunc("POST "+base+"/{assessment}/scope", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := owner(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int64                        `json:"expected_revision"`
			Scope            independentassessments.Scope `json:"scope"`
			Reason           string                       `json:"reason"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ChangeScope(repo, r.PathValue("assessment"), a, in.ExpectedRevision, in.Scope, in.Reason)
		if !independentError(w, e) {
			writeJSON(w, 201, independentassessments.Redact(x))
		}
	})
	assessor := func(w http.ResponseWriter, r *http.Request) (independentassessments.Assessment, independentassessments.Invitation, bool) {
		v := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		a, i, e := s.Authenticate(v)
		if independentError(w, e) {
			return a, i, false
		}
		return a, i, true
	}
	mux.HandleFunc("GET /independent-assessor/context", func(w http.ResponseWriter, r *http.Request) {
		a, i, ok := assessor(w, r)
		if !ok {
			return
		}
		ctx := independentassessments.Context{Assessment: independentassessments.Redact(a), Assessor: independentassessments.RedactInvitation(i), Evidence: []independentassessments.Evidence{}, UnavailableEvidenceIDs: []string{}}
		for _, pid := range a.Scope.EvidencePackageIDs {
			p, e := evidence.Package(a.RepositoryID, a.Scope.ProgramID, pid, "repository")
			if e != nil || p.ControlVersion != a.Scope.ProgramVersion || !hasString(a.Scope.ControlIDs, p.ControlID) {
				ctx.UnavailableEvidenceIDs = append(ctx.UnavailableEvidenceIDs, pid)
				continue
			}
			ctx.Evidence = append(ctx.Evidence, independentassessments.Evidence{ID: p.ID, ControlID: p.ControlID, PeriodStart: p.PeriodStart, PeriodEnd: p.PeriodEnd, PackageHash: p.PackageHash, Attestation: p.Attestation, Fresh: p.Fresh, Coverage: p.Coverage, Gaps: p.Gaps, Records: p.Records})
		}
		writeJSON(w, 200, ctx)
	})
	mux.HandleFunc("POST /independent-assessor/events", func(w http.ResponseWriter, r *http.Request) {
		a, i, ok := assessor(w, r)
		if !ok {
			return
		}
		var in independentassessments.EventInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Add(a.RepositoryID, a.ID, i.AssessorID, "assessor", in)
		if !independentError(w, e) {
			writeJSON(w, 201, independentassessments.Redact(x))
		}
	})
}
func hasString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func independentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status, code := 500, "internal_error"
	switch {
	case errors.Is(e, independentassessments.ErrNotFound):
		status, code = 404, "assessment_not_found"
	case errors.Is(e, independentassessments.ErrForbidden):
		status, code = 403, "assessment_forbidden"
	case errors.Is(e, independentassessments.ErrExpired):
		status, code = 401, "assessment_access_expired"
	case errors.Is(e, independentassessments.ErrConflict):
		status, code = 409, "assessment_changed"
	case errors.Is(e, independentassessments.ErrInvalid):
		status, code = 422, "invalid_independent_assessment"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
