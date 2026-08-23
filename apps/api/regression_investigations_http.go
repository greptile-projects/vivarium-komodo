package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/regressioninvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type regressionReleaseStore interface {
	Get(string, string) (releases.Release, error)
}

func registerRegressionInvestigationsHTTP(m *http.ServeMux, s *ri.Store, repos codeIntelligenceStore, credentials authStore, releaseStore regressionReleaseStore) {
	base := "/repositories/{repository}/regression-investigations"
	m.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.Input
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if strings.TrimSpace(in.Title) == "" || len(in.Title) > 200 || !validRegressionSource(in.Source) {
			writeJSON(w, 422, map[string]string{"error": "invalid_regression_investigation"})
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		if !resolveRegressionScope(string(repo.ID), opened, releaseStore, &in.Scope) {
			writeJSON(w, 422, map[string]string{"error": "invalid_or_incomparable_boundary"})
			return
		}
		if !validRegressionEvidence(in.Evidence) {
			writeJSON(w, 422, map[string]string{"error": "invalid_evidence"})
			return
		}
		v, e := s.Create(string(repo.ID), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
	})
	m.HandleFunc("GET "+base+"/{investigation}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.Get(string(repo.ID), r.PathValue("investigation"))
		if e == nil {
			opened, openErr := repos.Open(repo.ID)
			if openErr == nil {
				v.StaleInputs = regressionStaleInputs(opened, v.Scope)
			}
		}
		writeRegression(w, v, e, 200)
	})
	m.HandleFunc("PUT "+base+"/{investigation}/scope", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64    `json:"expected_version"`
			Reason          string   `json:"reason"`
			Scope           ri.Scope `json:"scope"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil || !resolveRegressionScope(string(repo.ID), opened, releaseStore, &in.Scope) {
			writeJSON(w, 422, map[string]string{"error": "invalid_or_incomparable_boundary"})
			return
		}
		v, e := s.ChangeScope(string(repo.ID), r.PathValue("investigation"), a.UserID, in.Reason, in.ExpectedVersion, in.Scope)
		writeRegression(w, v, e, 200)
	})
	m.HandleFunc("POST "+base+"/{investigation}/evidence", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.Evidence
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		if !validRegressionEvidence([]ri.Evidence{in}) {
			writeJSON(w, 422, map[string]string{"error": "invalid_evidence"})
			return
		}
		v, e := s.AddEvidence(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/entries", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.Entry
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := s.AddEntry(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/status", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct{ Status, Reason string }
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.SetStatus(string(repo.ID), r.PathValue("investigation"), a.UserID, in.Status, in.Reason)
		writeRegression(w, v, e, 200)
	})
}

func regressionStaleInputs(repo *storage.Repository, s ri.Scope) []string {
	out := []string{}
	for label, b := range map[string]ri.Boundary{"known_good": s.KnownGood, "known_bad": s.KnownBad} {
		if b.Kind != "revision" || b.Reference == "" || len(b.Reference) == 40 {
			continue
		}
		commit, _, err := resolveRevision(repo, b.Reference)
		if err != nil || string(commit) != b.CommitID {
			out = append(out, label+"_revision_changed")
		}
	}
	return out
}
func resolveRegressionScope(repoID string, repo *storage.Repository, rs regressionReleaseStore, s *ri.Scope) bool {
	return resolveRegressionBoundary(repoID, repo, rs, &s.KnownGood) && resolveRegressionBoundary(repoID, repo, rs, &s.KnownBad) && (s.KnownGood.CommitID == "" || s.KnownBad.CommitID == "" || regressionAncestor(repo, storage.ObjectID(s.KnownGood.CommitID), storage.ObjectID(s.KnownBad.CommitID)))
}
func resolveRegressionBoundary(repoID string, repo *storage.Repository, rs regressionReleaseStore, b *ri.Boundary) bool {
	if strings.TrimSpace(b.Reference) == "" {
		return b.CommitID == ""
	}
	switch b.Kind {
	case "revision":
		c, ref, e := resolveRevision(repo, b.Reference)
		if e != nil {
			return false
		}
		b.CommitID = string(c)
		b.Reference = ref
		b.ReleaseID = ""
		return true
	case "release":
		if rs == nil {
			return false
		}
		x, e := rs.Get(repoID, b.Reference)
		if e != nil {
			return false
		}
		if _, e = repo.ReadCommit(storage.ObjectID(x.CommitID)); e != nil {
			return false
		}
		b.ReleaseID = x.ID
		b.Reference = x.Version
		b.CommitID = x.CommitID
		return true
	}
	return false
}
func regressionAncestor(repo *storage.Repository, ancestor, descendant storage.ObjectID) bool {
	pending := []storage.ObjectID{descendant}
	seen := map[storage.ObjectID]bool{}
	for len(pending) > 0 {
		x := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if x == ancestor {
			return true
		}
		if seen[x] {
			continue
		}
		seen[x] = true
		c, e := repo.ReadCommit(x)
		if e != nil {
			return false
		}
		pending = append(pending, c.Parents...)
	}
	return false
}
func validRegressionSource(s ri.Source) bool {
	switch s.Kind {
	case "issue", "support_thread", "failed_check", "release", "deployment", "reproduction":
		return strings.TrimSpace(s.ResourceID) != ""
	}
	return false
}
func validRegressionEvidence(es []ri.Evidence) bool {
	if len(es) > 50 {
		return false
	}
	for _, e := range es {
		if !validRegressionSource(ri.Source{Kind: e.Kind, ResourceID: e.ResourceID}) || strings.TrimSpace(e.Summary) == "" || len(e.Summary) > 4000 || (e.Audience != "" && e.Audience != "public" && e.Audience != "repository") {
			return false
		}
	}
	return true
}
func writeRegression(w http.ResponseWriter, v ri.Investigation, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, ri.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if errors.Is(e, ri.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "conflict"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
