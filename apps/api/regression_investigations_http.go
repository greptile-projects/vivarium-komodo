package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	ri "github.com/greptile-projects/vivarium-komodo/apps/api/regressioninvestigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type regressionReleaseStore interface {
	Get(string, string) (releases.Release, error)
}

func registerRegressionInvestigationsHTTP(m *http.ServeMux, s *ri.Store, repos codeIntelligenceStore, credentials authStore, releaseStore regressionReleaseStore, builds releaseBuildStore) {
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
	m.HandleFunc("POST "+base+"/{investigation}/scenarios", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Derived    bool                  `json:"derived"`
			Definition ri.ScenarioDefinition `json:"definition"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.CreateScenario(string(repo.ID), r.PathValue("investigation"), a.UserID, in.Derived, in.Definition)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/scenarios/{scenario}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.AttemptInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil || !resolveRegressionTarget(string(repo.ID), opened, releaseStore, builds, &in.Target) {
			writeJSON(w, 422, map[string]string{"error": "invalid_attempt_target"})
			return
		}
		v, e := s.AddAttempt(string(repo.ID), r.PathValue("investigation"), r.PathValue("scenario"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/searches", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.SearchInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil || !validateSearchGraph(string(repo.ID), opened, in.Revisions) {
			writeJSON(w, 422, map[string]string{"error": "invalid_search_graph"})
			return
		}
		v, e := s.CreateSearch(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/searches/{search}/classifications", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.CandidateClassification
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.ClassifyCandidate(string(repo.ID), r.PathValue("investigation"), r.PathValue("search"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/searches/{search}/hypotheses", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in ri.CausalHypothesis
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddHypothesis(string(repo.ID), r.PathValue("investigation"), r.PathValue("search"), a.UserID, in)
		writeRegression(w, v, e, 201)
	})
	m.HandleFunc("POST "+base+"/{investigation}/responses", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner_required"})
			return
		}
		var in ri.ResponsePlanInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.CreateResponse(string(repo.ID), r.PathValue("investigation"), a.UserID, in)
		writeRegression(w, v, e, http.StatusCreated)
	})
	m.HandleFunc("POST "+base+"/{investigation}/responses/{response}/work", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner_required"})
			return
		}
		var in ri.ResponseWork
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.AddResponseWork(string(repo.ID), r.PathValue("investigation"), r.PathValue("response"), a.UserID, in)
		writeRegression(w, v, e, http.StatusCreated)
	})
}

func validateSearchGraph(repoID string, repo *storage.Repository, revisions []ri.SearchRevision) bool {
	for _, x := range revisions {
		if x.Kind != "commit" {
			if x.Kind == "repository_revision" && x.RepositoryID != "" && x.RepositoryID != repoID || x.Kind == "package_revision" && x.Package != "" {
				continue
			}
			return false
		}
		c, e := repo.ReadCommit(storage.ObjectID(x.Revision))
		if e != nil {
			return false
		}
		actual := make([]string, len(c.Parents))
		for i, p := range c.Parents {
			actual[i] = string(p)
		}
		if len(actual) != len(x.Parents) {
			return false
		}
		for i, p := range actual {
			if x.Parents[i] != p {
				return false
			}
		}
	}
	return true
}

func resolveRegressionTarget(repoID string, repo *storage.Repository, rs regressionReleaseStore, builds releaseBuildStore, t *ri.Target) bool {
	switch t.Kind {
	case "revision":
		b := ri.Boundary{Kind: "revision", Reference: t.Reference, CommitID: t.CommitID}
		if !resolveRegressionBoundary(repoID, repo, rs, &b) {
			return false
		}
		t.Reference, t.CommitID, t.ReleaseID = b.Reference, b.CommitID, ""
		return true
	case "release":
		b := ri.Boundary{Kind: "release", Reference: t.Reference, CommitID: t.CommitID, ReleaseID: t.ReleaseID}
		if !resolveRegressionBoundary(repoID, repo, rs, &b) {
			return false
		}
		digest, ok := regressionReleaseAttestation(repoID, b.ReleaseID, builds)
		if !ok {
			return false
		}
		t.Reference, t.CommitID, t.ReleaseID = b.Reference, b.CommitID, b.ReleaseID
		t.AttestationDigest = digest
		return true
	case "dependency_combination":
		if len(t.Dependencies) == 0 || strings.TrimSpace(t.Reference) == "" {
			return false
		}
		b := ri.Boundary{Kind: "revision", Reference: t.Reference}
		if !resolveRegressionBoundary(repoID, repo, rs, &b) {
			return false
		}
		t.Reference, t.CommitID = b.Reference, b.CommitID
		return true
	}
	return false
}

func regressionReleaseAttestation(repoID, releaseID string, builds releaseBuildStore) (string, bool) {
	if builds == nil {
		return "", false
	}
	attempts, err := builds.List(repoID, "release:"+releaseID)
	if err != nil || len(attempts) == 0 {
		return "", false
	}
	latest := map[string]checkruns.Run{}
	for i := len(attempts) - 1; i >= 0; i-- {
		latest[attempts[i].Definition.Name] = attempts[i]
	}
	for _, run := range latest {
		if run.State != checkruns.Succeeded {
			return "", false
		}
	}
	b, err := json.Marshal(attempts)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), true
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
