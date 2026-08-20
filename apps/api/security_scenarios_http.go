package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
)

func registerSecurityScenariosHTTP(mux *http.ServeMux, s *securityscenarios.Store, threats *threatmodels.Store, repos proposalRepositoryStore, c authStore, pulls pullRequestStore, previewStore *previews.Store) {
	base := "/repositories/{repository}/security-scenarios"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		return string(repo.ID), a.UserID, ok
	}
	validate := func(repo string, in securityscenarios.Input) bool {
		m, e := threats.Get(repo, in.ThreatModelID)
		if e != nil || m.Origin.Revision != in.ThreatModelRevision {
			return false
		}
		for _, p := range m.AbusePaths {
			if p.ID == in.AbusePathID {
				return true
			}
		}
		return false
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		x, e := s.List(repo)
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": x, "total_count": len(x)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in securityscenarios.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		if !validate(repo, in) {
			writeJSON(w, 422, map[string]string{"error": "exact_threat_model_path_required"})
			return
		}
		x, e := s.Create(repo, actor, in)
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base+"/{scenario}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("scenario"))
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/{scenario}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in securityscenarios.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		if !validate(repo, in) {
			writeJSON(w, 422, map[string]string{"error": "exact_threat_model_path_required"})
			return
		}
		x, e := s.Revise(repo, r.PathValue("scenario"), actor, in)
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{scenario}/reviews", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			ScenarioVersion int64  `json:"scenario_version"`
			Decision        string `json:"decision"`
			Rationale       string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := s.Review(repo, r.PathValue("scenario"), actor, in.Decision, in.Rationale, in.ScenarioVersion)
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{scenario}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in securityscenarios.AttemptInput
		if !readJSON(w, r, &in, 4<<20) {
			return
		}
		p, e := pulls.Get(repo, in.PullRequestID)
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_candidate_revision_required"})
			return
		}
		if in.TargetKind == "preview" {
			pv, e := previewStore.Get(repo, in.PullRequestID, in.PreviewID)
			if e != nil || pv.Revision != in.Revision {
				writeJSON(w, 422, map[string]string{"error": "exact_isolated_preview_required"})
				return
			}
		}
		x, e := s.AddAttempt(repo, r.PathValue("scenario"), actor, in)
		if securityScenarioError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
}

func securityScenarioError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, securityscenarios.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "security_scenario_not_found"})
	} else if errors.Is(e, securityscenarios.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_or_unsafe_security_scenario"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
